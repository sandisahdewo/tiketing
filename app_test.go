package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tiket/internal/config"
	"tiket/internal/db"
	"tiket/internal/export"
	"tiket/internal/model"
	"tiket/internal/receipt"
)

func TestReportPrint(t *testing.T) {
	a := NewApp(t.TempDir())
	a.cfg = config.Config{Mode: config.ModeServer, Port: 0, DeviceID: "server-pc"}
	if err := a.activate(); err != nil {
		t.Fatal(err)
	}
	defer a.deactivate()
	c, err := a.local.Store.CreateCounter(model.CounterInput{Name: "LOKET 1", InvoiceCode: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.local.Store.AssignCounter(c.ID, a.cfg.DeviceID, "SERVER", false); err != nil {
		t.Fatal(err)
	}
	if err := a.svc.CreateFirstAdmin(model.UserInput{Username: "admin", FullName: "Pak Admin", Password: "rahasia"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Login("admin", "rahasia"); err != nil {
		t.Fatal(err)
	}
	tk, err := a.SaveTicket(model.TicketInput{Name: "Entry", Price: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	st, err := a.ShiftStatus()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenShift(st.SuggestedTypeID, 0); err != nil {
		t.Fatal(err)
	}
	sell := func() model.Sale {
		t.Helper()
		sale, err := a.CreateSale([]model.SaleItemInput{{TicketID: tk.ID, Qty: 1}}, 1000000)
		if err != nil {
			t.Fatal(err)
		}
		return sale
	}
	first, second := sell(), sell()
	if err := a.DeleteSale(second.ID, "salah input"); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	request := func(kind string) ReportTab {
		return ReportTab{Kind: kind, Query: model.ReportQuery{Period: "daily", From: today, To: today}}
	}
	preview := func(p ReportTab) string {
		t.Helper()
		v, err := a.ReportPrintPreview(p)
		if err != nil {
			t.Fatalf("%+v: %v", p, err)
		}
		return receipt.Text(v.Lines)
	}
	has := func(text string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q in\n%s", want, text)
			}
		}
	}

	has(preview(request(export.KindPeriod)), "PENJUALAN PER HARI", "Transaksi")
	has(preview(request(export.KindItems)), "PENJUALAN PER ITEM", "Entry")
	list := preview(request(export.KindSales))
	has(list, "DAFTAR TRANSAKSI", first.InvoiceNo)
	if strings.Contains(list, second.InvoiceNo) {
		t.Errorf("a deleted sale on the list:\n%s", list)
	}

	gone := request(export.KindSales)
	gone.Deleted = true
	has(preview(gone), "NOTA DIHAPUS", second.InvoiceNo, "salah input")

	found := request(export.KindSales)
	found.Search = "tidak ada yang cocok"
	has(preview(found), "Cari: tidak ada yang cocok", "Tidak ada transaksi.")

	for _, kind := range []string{"lain", export.KindAll} {
		if _, err := a.ReportPrintPreview(request(kind)); err == nil {
			t.Errorf("printing kind %q was accepted", kind)
		}
	}

	// An export holds what its tab holds. The whole report never takes the
	// search or the deleted sales of the list's tab.
	exported := func(p ReportTab, format string) export.Data {
		t.Helper()
		svc, token, err := a.service()
		if err != nil {
			t.Fatal(err)
		}
		d, err := a.exportData(svc, token, p, format)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := exported(request(export.KindItems), "pdf"); len(d.Sales) != 0 || d.SalesCount != 0 || d.PrintedBy != "Pak Admin" {
		t.Errorf("items export: %+v", d)
	}
	if d := exported(gone, "xlsx"); !d.Deleted || len(d.Sales) != 1 || d.Sales[0].ID != second.ID {
		t.Errorf("deleted export: %+v", d)
	}
	if d := exported(found, "xlsx"); len(d.Sales) != 0 || d.Filter != "Cari: tidak ada yang cocok" {
		t.Errorf("searched export: %+v", d)
	}
	whole := request(export.KindAll)
	whole.Deleted, whole.Search = true, "tidak ada yang cocok"
	if d := exported(whole, "xlsx"); d.Deleted || d.Filter != "" || len(d.Sales) != 1 || d.Sales[0].ID != first.ID {
		t.Errorf("whole report: %+v", d)
	}
	svc, token, _ := a.service()
	if _, err := a.exportData(svc, token, request("lain"), "pdf"); err == nil {
		t.Error("an unknown export kind was accepted")
	}

	// A list too long for the paper is refused, with a way out; the other
	// reports of the same sales still print.
	for i := 0; i < receipt.MaxSalesList; i++ {
		sell()
	}
	if _, err := a.ReportPrintPreview(request(export.KindSales)); err == nil || !strings.Contains(err.Error(), "PDF/Excel") {
		t.Errorf("long list: %v", err)
	}
	has(preview(request(export.KindItems)), "PENJUALAN PER ITEM")
}

func TestRestoreDatabase(t *testing.T) {
	dir := t.TempDir()
	a := NewApp(dir)
	a.cfg = config.Config{Mode: config.ModeServer, Port: 0, DeviceID: "server-pc"}
	if err := a.activate(); err != nil {
		t.Fatal(err)
	}
	defer a.deactivate()
	c, err := a.local.Store.CreateCounter(model.CounterInput{Name: "LOKET 1", InvoiceCode: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.local.Store.AssignCounter(c.ID, a.cfg.DeviceID, "SERVER", false); err != nil {
		t.Fatal(err)
	}

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

	// Sales need an open shift; the backup below keeps it open.
	st, err := a.ShiftStatus()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenShift(st.SuggestedTypeID, 0); err != nil {
		t.Fatal(err)
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
