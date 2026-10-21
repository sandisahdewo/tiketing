package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		conn.Exec("INSERT INTO sales (invoice_no, user_id, terminal, total, paid, change_amount, created_at) VALUES ('A1', 1, 'L1', 100, 100, 0, '2026-10-02 09:00:00')")
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

// migrated returns a database at path with the first n migrations applied
// (all when n < 0), with an admin and one sale.
func migrated(t *testing.T, path string, n int) {
	t.Helper()
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	names, _ := migrations()
	if n < 0 {
		n = len(names)
	}
	for i := 0; i < n; i++ {
		body, _ := migrationFiles.ReadFile("migrations/" + names[i])
		if _, err := conn.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", n))
	conn.Exec("INSERT INTO users (username, full_name, password_hash, role, created_at, updated_at) VALUES ('admin', 'A', 'x', 'admin', '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
	conn.Exec("INSERT INTO tickets (name, price, created_at, updated_at) VALUES ('Entry', 100, '2026-10-01 08:00:00', '2026-10-01 08:00:00')")
	conn.Exec("INSERT INTO sales (invoice_no, user_id, terminal, total, paid, change_amount, created_at) VALUES ('A1', 1, 'L1', 100, 100, 0, '2026-10-02 09:00:00')")
	conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
}

func TestPrepare(t *testing.T) {
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")
	migrated(t, livePath, -1)
	live, err := Open(livePath)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	work := filepath.Join(dir, "work")

	backup := func(name string, n int, change string) string {
		path := filepath.Join(dir, name)
		migrated(t, path, n)
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

	// Same version, and an older one that gets migrated on the copy.
	for _, src := range []string{backup("same.db", -1, ""), backup("old.db", 3, "")} {
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
	if old, _ := Inspect(filepath.Join(dir, "old.db")); old.Version != 3 {
		t.Error("the backup file itself must not be changed")
	}

	corrupt := backup("corrupt.db", -1, "")
	body, _ := os.ReadFile(corrupt)
	os.WriteFile(corrupt, body[:len(body)/2], 0o644)

	for _, tc := range []struct {
		name, src, want string
	}{
		{"extra column", backup("extra.db", -1, "ALTER TABLE sales ADD COLUMN note TEXT"), "kolom sales.note tidak dikenal"},
		{"missing index", backup("noindex.db", -1, "DROP INDEX sales_terminal"), "indeks sales_terminal tidak ada"},
		{"no admin", backup("noadmin.db", -1, "UPDATE users SET role = 'kasir'"), "admin"},
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
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("refused backups must leave no copies, work holds %v", names)
	}
}
