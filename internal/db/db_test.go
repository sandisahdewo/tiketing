package db

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBackupTo(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "tiket.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(conn, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO settings (key, value) VALUES ('business_name', 'Uji')"); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "salinan.db")
	os.WriteFile(target, []byte("file lama"), 0o644) // an existing file is replaced
	if err := BackupTo(conn, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Error("temporary file left behind")
	}

	copy, err := Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var name string
	if err := copy.QueryRow("SELECT value FROM settings WHERE key = 'business_name'").Scan(&name); err != nil || name != "Uji" {
		t.Errorf("backup content: %q, %v", name, err)
	}
}

func TestInspectAndReplace(t *testing.T) {
	dir := t.TempDir()
	newDB := func(name string) string {
		path := filepath.Join(dir, name)
		conn, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err := Migrate(conn, filepath.Join(dir, "backups")); err != nil {
			t.Fatal(err)
		}
		conn.Exec("INSERT INTO users (username, full_name, password_hash, role, created_at, updated_at) VALUES ('admin', 'A', 'x', 'admin', '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
		conn.Exec("INSERT INTO tickets (name, price, created_at, updated_at) VALUES ('Entry', 100, '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
		conn.Exec("INSERT INTO counters (name, invoice_code, created_at, updated_at) VALUES ('L1', '1', '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
		conn.Exec("INSERT INTO sales (invoice_no, user_id, counter_id, terminal, total, paid, change_amount, created_at) VALUES ('A1', 1, 1, 'L1', 100, 100, 0, '2026-10-02 09:00:00')")
		conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return path
	}

	backup := newDB("backup.db")
	info, err := Inspect(backup)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := migrations()
	if info.Version != len(names) || info.Sales != 1 || info.LastSale != "2026-10-02 09:00:00" || info.Users != 1 {
		t.Errorf("info = %+v", info)
	}

	notDB := filepath.Join(dir, "foto.db")
	os.WriteFile(notDB, []byte("bukan database"), 0o644)
	if _, err := Inspect(notDB); err == nil {
		t.Error("non-database accepted")
	}

	newer := newDB("newer.db")
	conn, _ := Open(newer)
	conn.Exec("PRAGMA user_version = 999")
	conn.Close()
	if _, err := Inspect(newer); err == nil || !strings.Contains(err.Error(), "lebih baru") {
		t.Errorf("newer schema: %v", err)
	}

	live := filepath.Join(dir, "tiket.db")
	os.WriteFile(live, []byte("lama"), 0o644)
	os.WriteFile(live+"-wal", []byte("wal lama"), 0o644)
	if err := Replace(live, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(live + "-wal"); !os.IsNotExist(err) {
		t.Error("old WAL left behind")
	}
	if info, err := Inspect(live); err != nil || info.Sales != 1 {
		t.Errorf("replaced database: %+v %v", info, err)
	}
}

// migrated creates a database at path with an admin, a counter and one sale.
func migrated(t *testing.T, path string) {
	t.Helper()
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(conn, filepath.Join(filepath.Dir(path), "backups")); err != nil {
		t.Fatal(err)
	}
	conn.Exec("INSERT INTO counters (name, invoice_code, created_at, updated_at) VALUES ('L1', '1', '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
	conn.Exec("INSERT INTO users (username, full_name, password_hash, role, created_at, updated_at) VALUES ('admin', 'A', 'x', 'admin', '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
	conn.Exec("INSERT INTO tickets (name, price, created_at, updated_at) VALUES ('Entry', 100, '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
	conn.Exec("INSERT INTO sales (invoice_no, user_id, counter_id, terminal, total, paid, change_amount, created_at) VALUES ('A1', 1, 1, 'L1', 100, 100, 0, '2026-10-02 09:00:00')")
	conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
}

func TestPrepare(t *testing.T) {
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")
	migrated(t, livePath)
	live, err := Open(livePath)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	work := filepath.Join(dir, "work")

	backup := func(name, change string) string {
		path := filepath.Join(dir, name)
		migrated(t, path)
		if change != "" {
			conn, _ := Open(path)
			if _, err := conn.Exec(change); err != nil {
				t.Fatal(err)
			}
			conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
			conn.Close()
		}
		return path
	}

	for _, src := range []string{backup("same.db", "")} {
		info, copyPath, err := Prepare(src, work, live)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(src), err)
		}
		names, _ := migrations()
		if info.Version != len(names) || info.Sales != 1 {
			t.Errorf("%s: info %+v", filepath.Base(src), info)
		}
		if _, err := os.Stat(copyPath); err != nil {
			t.Errorf("%s: no prepared copy", filepath.Base(src))
		}
	}

	corrupt := backup("corrupt.db", "")
	body, _ := os.ReadFile(corrupt)
	os.WriteFile(corrupt, body[:len(body)/2], 0o644)

	for _, tc := range []struct {
		name, src, want string
	}{
		{"extra column", backup("extra.db", "ALTER TABLE sales ADD COLUMN note TEXT"), "kolom sales.note tidak dikenal"},
		{"missing index", backup("noindex.db", "DROP INDEX sales_counter_created"), "indeks sales_counter_created tidak ada"},
		{"missing table", backup("notable.db", "DROP TABLE sale_prints"), "cocok dengan aplikasi ini: tabel sale_prints tidak ada"},
		{"no admin", backup("noadmin.db", "UPDATE users SET role = 'kasir'"), "admin"},
		{"corrupt", corrupt, ""},
	} {
		_, copyPath, err := Prepare(tc.src, work, live)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error with %q", tc.name, err, tc.want)
		}
		if copyPath != "" {
			t.Errorf("%s: copy returned for a refused backup", tc.name)
		}
	}
	entries, _ := os.ReadDir(work)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("refused backups must leave no copies, work holds %v", names)
	}
}

// Migration files that are already on users' PCs. They must never change:
// a change to the structure goes in a new file, which is then added here
// once released.
var releasedMigrations = map[string]string{
	"001_init.sql": "364a244e63669c6482c41cb3253e141a4365126a0e735c4e252ea229cd1b40b2",
}

func TestReleasedMigrations(t *testing.T) {
	for name, want := range releasedMigrations {
		body, err := fs.ReadFile(migrationFiles, "migrations/"+name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != want {
			t.Errorf("%s sudah dirilis dan tidak boleh diubah; buat file migrasi baru", name)
		}
	}
}

// An installed app keeps its data when a new version adds a migration: only
// the new file runs, after a backup, and Verify accepts the result.
func TestMigrateUpgrade(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "tiket.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	backups := filepath.Join(dir, "backups")
	if err := Migrate(conn, backups); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO settings (key, value) VALUES ('business_name', 'Uji')"); err != nil {
		t.Fatal(err)
	}

	init, _ := fs.ReadFile(embedded, "migrations/001_init.sql")
	migrationFiles = fstest.MapFS{
		"migrations/001_init.sql":  {Data: init},
		"migrations/002_notes.sql": {Data: []byte("ALTER TABLE sales ADD COLUMN note TEXT NOT NULL DEFAULT '';")},
	}
	defer func() { migrationFiles = embedded }()

	if err := Migrate(conn, backups); err != nil {
		t.Fatal(err)
	}
	var version int
	var name string
	conn.QueryRow("PRAGMA user_version").Scan(&version)
	conn.QueryRow("SELECT value FROM settings WHERE key = 'business_name'").Scan(&name)
	if version != 2 || name != "Uji" {
		t.Errorf("after upgrade: version %d, data %q", version, name)
	}
	if _, err := conn.Exec("UPDATE sales SET note = 'x'"); err != nil {
		t.Errorf("new column: %v", err)
	}
	if files, _ := filepath.Glob(filepath.Join(backups, "*pre-migrate-v1.db")); len(files) != 1 {
		t.Errorf("backup before migration: %v", files)
	}
	if err := Verify(conn); err != nil {
		t.Errorf("Verify after upgrade: %v", err)
	}
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	conn, err := Open(filepath.Join(dir, "tiket.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(conn, dir); err != nil {
		t.Fatal(err)
	}
	if err := Verify(conn); err != nil {
		t.Fatalf("fresh database: %v", err)
	}

	// A trial database whose structure no migration of this version made.
	conn.Exec("DROP INDEX sales_counter_created")
	if err := Verify(conn); err == nil || !strings.Contains(err.Error(), "indeks sales_counter_created tidak ada") {
		t.Errorf("changed structure: %v", err)
	}

	// A database from a newer app is never opened by an older one.
	conn.Exec("PRAGMA user_version = 99")
	if err := Migrate(conn, dir); err == nil || !strings.Contains(err.Error(), "lebih baru") {
		t.Errorf("newer database: %v", err)
	}
}
