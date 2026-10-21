package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tiket/internal/config"
	"tiket/internal/db"
	"tiket/internal/model"
)

func TestRestoreDatabase(t *testing.T) {
	dir := t.TempDir()
	a := NewApp(dir)
	a.cfg = config.Config{Mode: config.ModeServer, Port: 0}
	if err := a.activate(); err != nil {
		t.Fatal(err)
	}
	defer a.deactivate()

	if err := a.svc.CreateFirstAdmin(model.UserInput{Username: "admin", FullName: "Admin", Password: "rahasia"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Login("admin", "rahasia"); err != nil {
		t.Fatal(err)
	}
	tk, err := a.SaveTicket(model.TicketInput{Name: "Entry", Price: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	sell := func() {
		t.Helper()
		if _, err := a.CreateSale([]model.SaleItemInput{{TicketID: tk.ID, Qty: 1}}, 1000000); err != nil {
			t.Fatal(err)
		}
	}
	sales := func() int64 {
		t.Helper()
		info, err := db.Inspect(a.dbPath())
		if err != nil {
			t.Fatal(err)
		}
		return info.Sales
	}

	sell()
	backup := filepath.Join(dir, "backup.db")
	if err := db.BackupTo(a.conn, backup); err != nil {
		t.Fatal(err)
	}
	sell()
	a.conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if n := sales(); n != 2 {
		t.Fatalf("before restore: %d sales", n)
	}

	// A file that is not a backup is refused and nothing changes.
	junk := filepath.Join(dir, "foto.db")
	os.WriteFile(junk, []byte("bukan database"), 0o644)
	if _, err := a.prepareRestore(junk); err == nil {
		t.Fatal("junk file accepted")
	}
	if err := a.RestoreDatabase(); err == nil {
		t.Fatal("restore without a checked backup")
	}
	if a.CurrentUser() == nil || sales() != 2 {
		t.Fatal("a refused restore must not touch the data or the login")
	}

	info, err := a.prepareRestore(backup)
	if err != nil {
		t.Fatal(err)
	}
	if info.Sales != 1 || info.Users != 1 {
		t.Errorf("restore info: %+v", info)
	}
	if err := a.RestoreDatabase(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(a.restoreWorkDir()); len(entries) != 0 {
		t.Errorf("checked copy left behind: %d files", len(entries))
	}
	if a.CurrentUser() != nil {
		t.Error("restore should log everyone out")
	}
	if n := sales(); n != 1 {
		t.Errorf("after restore: %d sales, want 1", n)
	}

	// The data from just before the restore was saved.
	entries, _ := os.ReadDir(a.backupDir())
	var safety string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-pre-restore.db") {
			safety = filepath.Join(a.backupDir(), e.Name())
		}
	}
	if safety == "" {
		t.Fatal("no safety backup")
	}
	if info, err := db.Inspect(safety); err != nil || info.Sales != 2 {
		t.Errorf("safety backup: %+v %v", info, err)
	}

	// The server runs again on the restored data.
	if _, err := a.Login("admin", "rahasia"); err != nil {
		t.Fatalf("login after restore: %v", err)
	}
	sell()
	if n := sales(); n != 2 {
		t.Errorf("selling after restore: %d sales", n)
	}

	// Cashiers may not restore.
	if _, err := a.SaveUser(model.UserInput{Username: "budi", FullName: "Budi", Role: model.RoleKasir, Password: "1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Login("budi", "1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.prepareRestore(backup); err != model.ErrForbidden {
		t.Errorf("kasir prepare: %v", err)
	}
	if err := a.RestoreDatabase(); err != model.ErrForbidden {
		t.Errorf("kasir restore: %v", err)
	}
}
