package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Schema changes are applied automatically when the app opens a database:
// PRAGMA user_version counts the migration files already applied, and every
// newer file runs once, in name order, after a backup of the database. A
// released migration file is never edited; a change to the structure is a new
// file (002_..., 003_...). TestReleasedMigrations guards this.
//
//go:embed migrations/*.sql
var embedded embed.FS

// migrationFiles holds the migrations/ folder; tests swap it.
var migrationFiles fs.FS = embedded

const keepBackups = 14

func Open(path string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func migrations() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func Migrate(conn *sql.DB, backupDir string) error {
	var current int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	names, err := migrations()
	if err != nil {
		return err
	}
	if current > len(names) {
		return fmt.Errorf("Database ini dari versi aplikasi yang lebih baru (struktur v%d, aplikasi ini v%d). Pasang versi aplikasi terbaru.", current, len(names))
	}
	if current == len(names) {
		return nil
	}
	if current > 0 {
		if _, err := Backup(conn, backupDir, fmt.Sprintf("pre-migrate-v%d", current)); err != nil {
			return fmt.Errorf("backup sebelum migrasi gagal: %w", err)
		}
	}
	for i := current; i < len(names); i++ {
		body, err := fs.ReadFile(migrationFiles, "migrations/"+names[i])
		if err != nil {
			return err
		}
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrasi %s: %w", names[i], err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func Backup(conn *sql.DB, dir, label string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("tiket-%s-%s.db", time.Now().Format("20060102-150405"), label)
	target := filepath.Join(dir, name)
	if _, err := conn.Exec("VACUUM INTO ?", target); err != nil {
		return "", err
	}
	return target, prune(dir)
}

// BackupTo writes a complete copy of the database to path, replacing any
// file there. It is written next to path first, so a failed backup never
// leaves a half-written file under that name.
func BackupTo(conn *sql.DB, path string) error {
	tmp := path + ".tmp"
	os.Remove(tmp)
	if _, err := conn.Exec("VACUUM INTO ?", tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// BackupInfo describes a database file picked for a restore.
type BackupInfo struct {
	Version  int    // schema version (applied migrations)
	Sales    int64  // non-deleted sales
	LastSale string // time of the newest sale, "" for none
	Users    int64  // active users
}

// ErrNotBackup means the file is not a database of this app.
var ErrNotBackup = errors.New("File ini bukan backup database aplikasi Tiket")

// OpenReadOnly opens a database file without being able to change it.
func OpenReadOnly(path string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
}

// Inspect opens a database file read-only and checks that it is a database
// of this app that this version can open (same or older schema).
func Inspect(path string) (BackupInfo, error) {
	var info BackupInfo
	conn, err := OpenReadOnly(path)
	if err != nil {
		return info, err
	}
	defer conn.Close()
	if err := conn.QueryRow("PRAGMA user_version").Scan(&info.Version); err != nil {
		return info, ErrNotBackup
	}
	var tables int
	err = conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('sales', 'sale_items', 'tickets', 'users', 'settings')`).Scan(&tables)
	if err != nil || tables != 5 || info.Version < 1 {
		return info, ErrNotBackup
	}
	names, err := migrations()
	if err != nil {
		return info, err
	}
	if info.Version > len(names) {
		return info, fmt.Errorf("Backup ini dari versi aplikasi yang lebih baru. Update aplikasi dulu, lalu restore lagi.")
	}
	var last sql.NullString
	if err := conn.QueryRow("SELECT COUNT(*), MAX(created_at) FROM sales WHERE deleted_at IS NULL").Scan(&info.Sales, &last); err != nil {
		return info, ErrNotBackup
	}
	info.LastSale = last.String
	if err := conn.QueryRow("SELECT COUNT(*) FROM users WHERE deleted_at IS NULL").Scan(&info.Users); err != nil {
		return info, ErrNotBackup
	}
	return info, nil
}

// Prepare checks a backup for a restore and returns a ready copy of it in
// workDir: the file must be intact, it is migrated to this version, its
// structure must match the live database exactly, and it must have an active
// admin. The live database is never touched. The caller removes the copy.
func Prepare(src, workDir string, live *sql.DB) (BackupInfo, string, error) {
	var info BackupInfo
	if _, err := Inspect(src); err != nil {
		return info, "", err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return info, "", err
	}
	f, err := os.CreateTemp(workDir, "restore-*.db")
	if err != nil {
		return info, "", err
	}
	path := f.Name()
	f.Close()
	if err := copyFile(path, src); err != nil {
		os.Remove(path)
		return info, "", err
	}
	fail := func(err error) (BackupInfo, string, error) {
		os.Remove(path)
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
		return info, "", err
	}

	conn, err := Open(path)
	if err != nil {
		return fail(ErrNotBackup)
	}
	var check string
	if err := conn.QueryRow("PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		conn.Close()
		return fail(errors.New("File backup rusak atau tidak utuh"))
	}
	// Bring an older backup up to this version; its own pre-migration copy is
	// not needed, so it goes to a scratch folder.
	scratch := filepath.Join(workDir, "migrate-check")
	err = Migrate(conn, scratch)
	os.RemoveAll(scratch)
	if err != nil {
		conn.Close()
		return fail(fmt.Errorf("Backup tidak bisa disesuaikan ke versi aplikasi ini: %w", err))
	}
	if diffs, err := schemaDiff(live, conn); err != nil {
		conn.Close()
		return fail(ErrNotBackup)
	} else if len(diffs) > 0 {
		conn.Close()
		return fail(fmt.Errorf("Struktur data backup tidak cocok dengan aplikasi ini: %s", diffs))
	}
	var admins int
	if err := conn.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND deleted_at IS NULL").Scan(&admins); err != nil || admins == 0 {
		conn.Close()
		return fail(errors.New("Backup ini tidak punya akun admin aktif, sehingga tidak ada yang bisa login untuk mengelola"))
	}
	// One self-contained file; Open turns WAL back on when it becomes live.
	if _, err := conn.Exec("PRAGMA journal_mode = DELETE"); err != nil {
		conn.Close()
		return fail(err)
	}
	conn.Close()
	if info, err = Inspect(path); err != nil {
		return fail(err)
	}
	return info, path, nil
}

// schema lists every table column and index of a database as comparable lines.
func schema(conn *sql.DB) (map[string]string, error) {
	out := map[string]string{}
	rows, err := conn.Query(`SELECT type, name, tbl_name FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	type object struct{ kind, name, table string }
	var objects []object
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.kind, &o.name, &o.table); err != nil {
			rows.Close()
			return nil, err
		}
		objects = append(objects, o)
	}
	rows.Close()
	for _, o := range objects {
		if o.kind == "index" {
			out["indeks "+o.name] = "pada tabel " + o.table
			continue
		}
		cols, err := conn.Query(`SELECT name, type, "notnull", COALESCE(dflt_value, ''), pk FROM pragma_table_info(?)`, o.name)
		if err != nil {
			return nil, err
		}
		out["tabel "+o.name] = "ada"
		for cols.Next() {
			var name, typ, dflt string
			var notNull, pk int
			if err := cols.Scan(&name, &typ, &notNull, &dflt, &pk); err != nil {
				cols.Close()
				return nil, err
			}
			out["kolom "+o.name+"."+name] = fmt.Sprintf("%s notnull=%d default=%s pk=%d", typ, notNull, dflt, pk)
		}
		cols.Close()
	}
	return out, nil
}

// Verify checks that a migrated database has exactly the structure the
// migrations of this version create. A database that differs, such as one
// from a trial build whose migrations were later rewritten, would fail in
// odd places, so the app refuses to open it instead.
func Verify(conn *sql.DB) error {
	want, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer want.Close()
	want.SetMaxOpenConns(1)
	if err := Migrate(want, ""); err != nil {
		return err
	}
	diffs, err := schemaDiff(want, conn)
	if err != nil {
		return err
	}
	if len(diffs) > 0 {
		return fmt.Errorf("Struktur database tidak cocok dengan versi aplikasi ini: %s", diffs)
	}
	return nil
}

// schemaDiffs lists how one database structure differs from another.
type schemaDiffs []string

func (d schemaDiffs) String() string {
	if len(d) > 5 {
		d = append(d[:5:5], fmt.Sprintf("dan %d lainnya", len(d)-5))
	}
	return strings.Join(d, "; ")
}

// schemaDiff reports how the structure of got differs from want.
func schemaDiff(want, got *sql.DB) (schemaDiffs, error) {
	w, err := schema(want)
	if err != nil {
		return nil, err
	}
	g, err := schema(got)
	if err != nil {
		return nil, err
	}
	// A missing table is reported once, not with each of its columns and indexes.
	missing := map[string]bool{}
	for k := range w {
		if t, ok := strings.CutPrefix(k, "tabel "); ok && g[k] == "" {
			missing[t] = true
		}
	}
	partOfMissing := func(k, v string) bool {
		if c, ok := strings.CutPrefix(k, "kolom "); ok {
			return missing[strings.SplitN(c, ".", 2)[0]]
		}
		t, ok := strings.CutPrefix(v, "pada tabel ")
		return ok && missing[t]
	}
	var diffs schemaDiffs
	for k, v := range w {
		if gv, ok := g[k]; !ok {
			if !partOfMissing(k, v) || strings.HasPrefix(k, "tabel ") {
				diffs = append(diffs, k+" tidak ada")
			}
		} else if gv != v {
			diffs = append(diffs, k+" berbeda")
		}
	}
	for k := range g {
		if _, ok := w[k]; !ok {
			diffs = append(diffs, k+" tidak dikenal")
		}
	}
	sort.Strings(diffs)
	return diffs, nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// Replace puts a copy of src at dst (the closed database file). The copy is
// written next to dst and renamed into place, so a failure leaves dst as it
// was; leftover WAL files of the old database are removed.
func Replace(dst, src string) error {
	tmp := dst + ".restore"
	if err := copyFile(tmp, src); err != nil {
		return err
	}
	os.Remove(dst + "-wal")
	os.Remove(dst + "-shm")
	return os.Rename(tmp, dst)
}

func BackupDaily(conn *sql.DB, dir string) error {
	today := "tiket-" + time.Now().Format("20060102")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), today) && strings.HasSuffix(e.Name(), "-harian.db") {
			return nil
		}
	}
	_, err := Backup(conn, dir, "harian")
	return err
}

func prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tiket-") && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keepBackups {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
