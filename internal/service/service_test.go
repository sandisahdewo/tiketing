package service_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"tiket/internal/applog"
	"tiket/internal/db"
	"tiket/internal/model"
	"tiket/internal/server"
	. "tiket/internal/service"
	"tiket/internal/store"
)

const testVersion = "v1.2.0"

var logoB64 = func() string {
	img := image.NewGray(image.Rect(0, 0, 40, 20))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}()

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newLocal(t *testing.T) (*Local, *clock) {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "tiket.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(conn, filepath.Join(dir, "backups")); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}
	c := &clock{t: time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)}
	st := store.New(conn)
	st.Now = c.now
	return NewLocal(st, testVersion), c
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func setup(t *testing.T, svc Service) (admin, kasir string) {
	t.Helper()
	st := must(svc.Status())
	if st.HasUsers {
		t.Fatal("fresh db should have no users")
	}
	if err := svc.CreateFirstAdmin(model.UserInput{Username: "Admin", FullName: "Pak Admin", Password: "rahasia"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateFirstAdmin(model.UserInput{Username: "admin2", FullName: "X", Password: "rahasia"}); err == nil {
		t.Fatal("second first-admin should fail")
	}
	admin = must(svc.Login("admin", "rahasia")).Token
	must(svc.SaveUser(admin, model.UserInput{Username: "budi", FullName: "Budi", Role: model.RoleKasir, Password: "1234"}))
	kasir = must(svc.Login("budi", "1234")).Token
	useCounter(t, svc, admin, testDevice, "LOKET 1", "1")
	return admin, kasir
}

// testDevice is the PC the tests sell from; setup gives it counter LOKET 1
// with invoice code 1.
const testDevice = "pc-test"

// useCounter creates a counter and puts the PC device on it.
func useCounter(t *testing.T, svc Service, admin, device, name, code string) model.Counter {
	t.Helper()
	c := must(svc.SaveCounter(admin, model.CounterInput{Name: name, InvoiceCode: code}))
	if err := svc.AssignCounter(admin, model.AssignCounterInput{CounterID: c.ID, DeviceID: device, DeviceName: "PC " + name}); err != nil {
		t.Fatal(err)
	}
	return c
}

func exercise(t *testing.T, svc Service, c *clock) {
	admin, kasir := setup(t, svc)

	if _, err := svc.Login("budi", "salah"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := svc.Me("token-palsu"); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatalf("want unauthorized, got %v", err)
	}

	if _, err := svc.SaveTicket(kasir, model.TicketInput{Name: "X", Price: 100}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("kasir must not manage tickets, got %v", err)
	}
	if _, err := svc.ListUsers(kasir); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("kasir must not list users, got %v", err)
	}
	if err := svc.SaveSettings(kasir, model.Settings{BusinessName: "x"}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("kasir must not save settings, got %v", err)
	}

	weekday := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry Weekday", Price: 3500000}))
	couple := must(svc.SaveTicket(admin, model.TicketInput{Name: "Couple", Price: 5000050}))
	elder := must(svc.SaveTicket(admin, model.TicketInput{Name: "Elder", Price: 2000000}))
	if _, err := svc.SaveTicket(admin, model.TicketInput{Name: "  ", Price: 1}); err == nil {
		t.Fatal("empty ticket name accepted")
	}

	if got := must(svc.ListTickets(kasir, "week", false, false)); len(got) != 1 || got[0].ID != weekday.ID {
		t.Fatalf("search failed: %+v", got)
	}

	must(svc.SaveTicket(admin, model.TicketInput{ID: elder.ID, Name: "Lansia", Price: 1500000}))
	if err := svc.DeleteTicket(admin, elder.ID); err != nil {
		t.Fatal(err)
	}
	if got := must(svc.ListTickets(kasir, "", false, false)); len(got) != 2 {
		t.Fatalf("deleted ticket still listed: %+v", got)
	}
	if got := must(svc.ListTickets(admin, "", true, false)); len(got) != 1 || got[0].DeletedAt == nil {
		t.Fatalf("deleted list wrong: %+v", got)
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: elder.ID, Qty: 1}}, Paid: 9999999}); err == nil {
		t.Fatal("sold a deleted ticket")
	}
	if err := svc.RestoreTicket(admin, elder.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: weekday.ID, Qty: 2}}, Paid: 100}); err == nil {
		t.Fatal("underpaid sale accepted")
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice}); err == nil {
		t.Fatal("empty sale accepted")
	}

	sale1 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice,
		Items: []model.SaleItemInput{
			{TicketID: weekday.ID, Qty: 2},
			{TicketID: couple.ID, Qty: 1},
			{TicketID: weekday.ID, Qty: 1},
		},
		Paid: 20000000,
	}))
	if sale1.InvoiceNo != "TKT1202610050001" {
		t.Errorf("invoice = %s", sale1.InvoiceNo)
	}
	if sale1.Total != 3*3500000+5000050 || sale1.ChangeAmount != 20000000-sale1.Total {
		t.Errorf("totals wrong: %+v", sale1)
	}
	if len(sale1.Items) != 2 || sale1.Items[0].Qty != 3 || sale1.CashierName != "Budi" {
		t.Errorf("items/cashier wrong: %+v", sale1)
	}

	must(svc.SaveTicket(admin, model.TicketInput{ID: weekday.ID, Name: "Entry Weekday Baru", Price: 9900000}))
	again := must(svc.GetSale(kasir, sale1.ID))
	if again.Items[0].Price != 3500000 || again.Total != sale1.Total {
		t.Errorf("price change rewrote history: %+v", again)
	}
	if again.Items[0].TicketName != "Entry Weekday Baru" {
		t.Errorf("ticket name should come from relation: %s", again.Items[0].TicketName)
	}

	sale2 := must(svc.CreateSale(admin, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: elder.ID, Qty: 4}}, Paid: 6000000}))
	if sale2.InvoiceNo != "TKT1202610050002" {
		t.Errorf("invoice2 = %s", sale2.InvoiceNo)
	}
	if _, err := svc.GetSale(kasir, sale2.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir read another user's sale: %v", err)
	}

	c.t = time.Date(2026, 10, 14, 10, 0, 0, 0, time.Local)
	sale3 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	if sale3.InvoiceNo != "TKT1202610140001" {
		t.Errorf("invoice3 = %s", sale3.InvoiceNo)
	}
	c.t = time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local)
	must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 2}}, Paid: 10000100}))

	if got := must(svc.ListSales(kasir, model.SaleQuery{From: "2026-10-01", To: "2026-10-31"})); len(got) != 1 || got[0].CreatedAt[:10] != "2026-11-01" {
		t.Errorf("kasir should see only own sales of today, got %+v", got)
	}
	if got := must(svc.ListSales(admin, model.SaleQuery{From: "2026-10-01", To: "2026-10-31"})); len(got) != 3 {
		t.Errorf("admin should see all sales in range, got %d", len(got))
	}

	daily := must(svc.Report(admin, model.ReportQuery{Period: "daily", From: "2026-10-01", To: "2026-11-30"}))
	if len(daily.Rows) != 3 || daily.Rows[0].Period != "2026-10-05" || daily.Rows[0].Transactions != 2 || daily.Rows[0].Qty != 8 {
		t.Errorf("daily report wrong: %+v", daily.Rows)
	}
	weekly := must(svc.Report(admin, model.ReportQuery{Period: "weekly", From: "2026-10-01", To: "2026-11-30"}))
	if len(weekly.Rows) != 3 || weekly.Rows[0].Period != "2026-10-05" || weekly.Rows[1].Period != "2026-10-12" || weekly.Rows[2].Period != "2026-10-26" {
		t.Errorf("weekly report should group by Monday: %+v", weekly.Rows)
	}
	monthly := must(svc.Report(admin, model.ReportQuery{Period: "monthly", From: "2026-10-01", To: "2026-11-30"}))
	wantTotal := sale1.Total + sale2.Total + sale3.Total + 2*5000050
	if len(monthly.Rows) != 2 || monthly.Rows[0].Period != "2026-10" || monthly.Total != wantTotal || monthly.Transactions != 4 {
		t.Errorf("monthly report wrong: %+v total=%d want=%d", monthly.Rows, monthly.Total, wantTotal)
	}
	if len(monthly.Tickets) != 3 {
		t.Errorf("ticket breakdown wrong: %+v", monthly.Tickets)
	}

	if s := must(svc.GetSettings(kasir)); s.InvoicePrefix != "TKT" || s.PurposeWeekday != "WEEKDAY REGULER" || s.HasLogo {
		t.Errorf("default settings = %+v", s)
	}
	if err := svc.SaveSettings(admin, model.Settings{BusinessName: "x", InvoicePrefix: "PN-Y", PurposeWeekday: "a", PurposeWeekend: "b"}); err == nil {
		t.Error("invalid invoice prefix accepted")
	}
	if err := svc.SaveSettings(admin, model.Settings{
		BusinessName: " Pony Park ", Footer: "Terima kasih",
		InvoicePrefix: "pny", PurposeWeekday: "WEEKDAY REGULER", PurposeWeekend: "WEEKEND REGULER",
	}); err != nil {
		t.Fatal(err)
	}
	if s := must(svc.GetSettings(kasir)); s.BusinessName != "Pony Park" || s.InvoicePrefix != "PNY" {
		t.Errorf("settings = %+v", s)
	}

	c.t = time.Date(2026, 11, 3, 10, 18, 0, 0, time.Local)
	useCounter(t, svc, admin, "pc-25", "LOKET 25", "25")
	useCounter(t, svc, admin, "pc-26", "LOKET 26", "26")
	pony1 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: "pc-25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	pony2 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: "pc-25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	other := must(svc.CreateSale(admin, model.SaleInput{DeviceID: "pc-26", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	if pony1.InvoiceNo != "PNY25202611030001" || pony2.InvoiceNo != "PNY25202611030002" || other.InvoiceNo != "PNY26202611030001" {
		t.Errorf("invoice format: %s %s %s", pony1.InvoiceNo, pony2.InvoiceNo, other.InvoiceNo)
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: "pc-none", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}); err == nil {
		t.Error("a PC without a counter must not sell")
	}
	if _, err := svc.SaveCounter(admin, model.CounterInput{Name: "LOKET X", InvoiceCode: "12345"}); err == nil {
		t.Error("too long invoice code accepted")
	}

	if pony1.PrintCount != 0 {
		t.Errorf("new sale print count = %d", pony1.PrintCount)
	}
	for i := 0; i < 3; i++ {
		if err := svc.RecordPrint(kasir, model.PrintInput{SaleID: pony1.ID, DeviceID: testDevice}); err != nil {
			t.Fatal(err)
		}
	}
	if got := must(svc.GetSale(kasir, pony1.ID)); got.PrintCount != 3 {
		t.Errorf("print count = %d, want 3", got.PrintCount)
	}
	if err := svc.RecordPrint(kasir, model.PrintInput{SaleID: other.ID}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir recorded print of another user's sale: %v", err)
	}

	if err := svc.SaveLogo(kasir, logoB64); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir saved logo: %v", err)
	}
	if err := svc.SaveLogo(admin, "bm90IGFuIGltYWdl"); err == nil {
		t.Error("non-image logo accepted")
	}
	if err := svc.SaveLogo(admin, logoB64); err != nil {
		t.Fatal(err)
	}
	if got := must(svc.GetLogo(kasir)); got != logoB64 {
		t.Error("logo roundtrip mismatch")
	}
	if s := must(svc.GetSettings(kasir)); !s.HasLogo {
		t.Error("HasLogo should be true")
	}
	if err := svc.SaveLogo(admin, ""); err != nil {
		t.Fatal(err)
	}
	if s := must(svc.GetSettings(kasir)); s.HasLogo {
		t.Error("logo not removed")
	}

	users := must(svc.ListUsers(admin))
	var adminID, budiID int64
	for _, u := range users {
		if u.Username == "admin" {
			adminID = u.ID
		} else {
			budiID = u.ID
		}
	}
	if err := svc.DeleteUser(admin, adminID); err == nil {
		t.Error("admin deleted self")
	}
	if _, err := svc.SaveUser(admin, model.UserInput{ID: adminID, Username: "admin", FullName: "A", Role: model.RoleKasir}); err == nil {
		t.Error("last admin demoted")
	}
	if _, err := svc.SaveUser(admin, model.UserInput{Username: "budi", FullName: "Dup", Role: model.RoleKasir, Password: "1234"}); err == nil {
		t.Error("duplicate username accepted")
	}
	if err := svc.DeleteUser(admin, budiID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Me(kasir); !errors.Is(err, model.ErrUnauthorized) {
		t.Errorf("deleted user's session still valid: %v", err)
	}
	if got := must(svc.GetSale(admin, sale1.ID)); got.CashierName != "Budi" {
		t.Errorf("sale of deleted user lost cashier name: %+v", got)
	}
	must(svc.SaveUser(admin, model.UserInput{Username: "budi", FullName: "Budi Baru", Role: model.RoleKasir, Password: "1234"}))

	if err := svc.Logout(admin); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Me(admin); !errors.Is(err, model.ErrUnauthorized) {
		t.Errorf("logout did not end session: %v", err)
	}
}

func TestLocal(t *testing.T) {
	local, c := newLocal(t)
	exercise(t, local, c)
}

func TestRemoteRoundTrip(t *testing.T) {
	local, c := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	exercise(t, NewRemote(srv.URL, "1.2.5"), c)
}

func TestRemoteVersionMismatch(t *testing.T) {
	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	remote := NewRemote(srv.URL, "1.3.0")
	if st := must(remote.Status()); st.Version != testVersion {
		t.Errorf("status version = %s", st.Version)
	}
	err := remote.CreateFirstAdmin(model.UserInput{Username: "admin", FullName: "A", Password: "rahasia"})
	if !errors.Is(err, model.ErrVersionMismatch) {
		t.Fatalf("want version mismatch, got %v", err)
	}
}

func TestRemoteUnreachable(t *testing.T) {
	remote := NewRemote("127.0.0.1:1", testVersion)
	if _, err := remote.Status(); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestTopupCategory(t *testing.T) {
	svc, _ := newLocal(t)
	admin, kasir := setup(t, svc)

	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry Weekday", Price: 3500000}))
	if entry.Category != model.CategoryTicket || entry.Color != model.DefaultColor {
		t.Errorf("defaults wrong: %+v", entry)
	}
	topup50 := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup 50.000", Price: 5000000, Category: model.CategoryTopup, Color: "emerald"}))
	topup100 := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup 100.000", Price: 10000000, Category: model.CategoryTopup, Color: "blue"}))
	if topup50.Category != model.CategoryTopup || topup50.Color != "emerald" {
		t.Errorf("topup saved wrong: %+v", topup50)
	}
	if _, err := svc.SaveTicket(admin, model.TicketInput{Name: "X", Price: 1, Category: "voucher"}); err == nil {
		t.Error("invalid category accepted")
	}
	if _, err := svc.SaveTicket(admin, model.TicketInput{Name: "X", Price: 1, Color: "#ff0000"}); err == nil {
		t.Error("invalid color accepted")
	}
	entry = must(svc.SaveTicket(admin, model.TicketInput{ID: entry.ID, Name: entry.Name, Price: entry.Price, Color: "red"}))
	if entry.Color != "red" || entry.Category != model.CategoryTicket {
		t.Errorf("update wrong: %+v", entry)
	}

	sale := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice,
		Items: []model.SaleItemInput{
			{TicketID: entry.ID, Qty: 2},
			{TicketID: topup100.ID, Qty: 1},
			{TicketID: topup50.ID, Qty: 2},
		},
		Paid: 30000000,
	}))
	if sale.Total != 2*3500000+10000000+2*5000000 || sale.Items[1].Category != model.CategoryTopup {
		t.Errorf("mixed sale wrong: %+v", sale)
	}

	r := must(svc.Report(admin, model.ReportQuery{Period: "daily", From: "2026-10-01", To: "2026-10-31"}))
	if r.TicketQty != 2 || r.TicketTotal != 7000000 || r.TopupQty != 3 || r.TopupTotal != 20000000 || r.Total != sale.Total {
		t.Errorf("report totals wrong: %+v", r)
	}
	if len(r.Rows) != 1 || r.Rows[0].TicketTotal != 7000000 || r.Rows[0].TopupTotal != 20000000 {
		t.Errorf("report rows wrong: %+v", r.Rows)
	}
	if len(r.Tickets) != 3 || r.Tickets[0].Category != model.CategoryTicket || r.Tickets[1].TicketID != topup100.ID {
		t.Errorf("report items wrong: %+v", r.Tickets)
	}
}

func TestKasirReportAndDeleteSale(t *testing.T) {
	local, c := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	// Goes through the HTTP server so the new route and remote client are covered too.
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)
	tk := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	sell := func(token string, qty int64) model.Sale {
		return must(svc.CreateSale(token, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: tk.ID, Qty: qty}}, Paid: qty * 1000000}))
	}

	c.t = time.Date(2026, 10, 4, 9, 0, 0, 0, time.Local)
	sell(kasir, 5) // yesterday
	c.t = time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)
	mine := sell(kasir, 2)
	victim := sell(kasir, 1)
	sell(admin, 3) // someone else's sale today

	r := must(svc.Report(kasir, model.ReportQuery{Period: "monthly", From: "2026-01-01", To: "2026-12-31"}))
	if r.From != "2026-10-05" || r.To != "2026-10-05" || r.Period != "daily" || r.Transactions != 2 || r.Qty != 3 {
		t.Errorf("kasir report should be own sales today: %+v", r)
	}
	list := must(svc.ListSales(kasir, model.SaleQuery{From: "2026-01-01", To: "2026-12-31"}))
	if len(list) != 2 || len(list[1].Items) != 1 || list[1].Items[0].Qty != mine.Items[0].Qty || list[1].Items[0].TicketName != "Entry" {
		t.Errorf("kasir list should hold today's own sales with items: %+v", list)
	}
	if _, err := svc.ListSales(kasir, model.SaleQuery{From: "2026-10-05", To: "2026-10-05", Deleted: true}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not list deleted sales, got %v", err)
	}

	if err := svc.DeleteSale(kasir, model.DeleteSaleInput{ID: victim.ID, Reason: "salah"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not delete sales, got %v", err)
	}
	if err := svc.DeleteSale(admin, model.DeleteSaleInput{ID: victim.ID, Reason: "  "}); err == nil {
		t.Error("empty reason accepted")
	}
	if err := svc.DeleteSale(admin, model.DeleteSaleInput{ID: 999, Reason: "x"}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown sale: %v", err)
	}
	if err := svc.DeleteSale(admin, model.DeleteSaleInput{ID: victim.ID, Reason: "Salah input"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSale(admin, model.DeleteSaleInput{ID: victim.ID, Reason: "lagi"}); err == nil {
		t.Error("double delete accepted")
	}

	all := must(svc.Report(admin, model.ReportQuery{Period: "daily", From: "2026-10-01", To: "2026-10-31"}))
	if all.Transactions != 3 || all.Qty != 10 {
		t.Errorf("deleted sale still counted: %+v", all)
	}
	if got := must(svc.ListSales(admin, model.SaleQuery{From: "2026-10-05", To: "2026-10-05"})); len(got) != 2 {
		t.Errorf("deleted sale still listed: %d", len(got))
	}
	deleted := must(svc.ListSales(admin, model.SaleQuery{From: "2026-10-05", To: "2026-10-05", Deleted: true}))
	if len(deleted) != 1 || deleted[0].ID != victim.ID || deleted[0].DeletedAt == nil || deleted[0].DeletedByName != "Pak Admin" || deleted[0].DeleteReason != "Salah input" {
		t.Errorf("deleted list wrong: %+v", deleted)
	}
	if got := must(svc.GetSale(kasir, victim.ID)); got.DeletedAt == nil {
		t.Error("GetSale should expose deletion")
	}
	if err := svc.RecordPrint(kasir, model.PrintInput{SaleID: victim.ID}); err == nil {
		t.Error("printing a deleted sale accepted")
	}

	next := sell(kasir, 1)
	if next.InvoiceNo != "TKT1202610050004" {
		t.Errorf("invoice number reused after delete: %s", next.InvoiceNo)
	}
}

func TestLoggedOpSessionAndPayload(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	defer slog.SetDefault(prev)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewLogged(NewRemote(srv.URL, testVersion))
	admin, _ := setup(t, svc)

	buf.Reset()
	if _, err := svc.SaveUser(admin, model.UserInput{Username: "x", FullName: "X", Role: model.RoleKasir, Password: "rahasia"}); err == nil {
		t.Fatal("short username accepted")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one server and one client line, got:\n%s", buf.String())
	}
	server, client := lines[0], lines[1]
	op := regexp.MustCompile(`op=(\w+)`).FindStringSubmatch(client)
	if op == nil || !strings.Contains(server, "op="+op[1]) {
		t.Errorf("op not shared:\nserver: %s\nclient: %s", server, client)
	}
	sess := "sess=" + applog.SessionID(admin)
	if !strings.Contains(client, sess) || !strings.Contains(server, sess) || strings.Contains(buf.String(), admin) {
		t.Errorf("both sides should log the session id but never the token:\n%s", buf.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, "level=WARN") || !strings.Contains(l, `\"username\":\"x\"`) || !strings.Contains(l, `\"password\":\"***\"`) || strings.Contains(l, "rahasia") {
			t.Errorf("failed action should log masked input: %s", l)
		}
	}

	buf.Reset()
	must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	if strings.Contains(buf.String(), "input=") || !strings.Contains(buf.String(), "msg=remote:saveTicket") || !strings.Contains(buf.String(), "name=Entry") {
		t.Errorf("successful action should log a summary only:\n%s", buf.String())
	}

	// All steps of one action share its op, on this PC and on the server.
	buf.Reset()
	scoped := svc.WithOp("klik123")
	scoped.ListSales(admin, model.SaleQuery{From: "2026-10-05", To: "2026-10-05"})
	scoped.Report(admin, model.ReportQuery{Period: "daily", From: "2026-10-05", To: "2026-10-05"})
	if n := strings.Count(buf.String(), "op=klik123"); n != 4 {
		t.Errorf("want 4 lines (2 steps x client+server) under one op, got %d:\n%s", n, buf.String())
	}
}

func TestTicketActive(t *testing.T) {
	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)

	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup", Price: 5000000, Category: model.CategoryTopup}))
	if !entry.Active {
		t.Fatal("new ticket should be active")
	}

	if err := svc.SetTicketActive(kasir, topup.ID, false); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not change active, got %v", err)
	}
	if err := svc.SetTicketActive(admin, 999, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown ticket: %v", err)
	}
	if err := svc.SetTicketActive(admin, topup.ID, false); err != nil {
		t.Fatal(err)
	}

	// The cashier screen never sees inactive tickets, even when it asks for them.
	for _, tc := range []struct {
		token      string
		activeOnly bool
		want       int
	}{{kasir, false, 1}, {kasir, true, 1}, {admin, true, 1}, {admin, false, 2}} {
		if got := must(svc.ListTickets(tc.token, "", false, tc.activeOnly)); len(got) != tc.want {
			t.Errorf("activeOnly=%v admin=%v: got %d tickets, want %d", tc.activeOnly, tc.token == admin, len(got), tc.want)
		}
	}
	if got := must(svc.ListTickets(admin, "Topup", false, false)); len(got) != 1 || got[0].Active {
		t.Errorf("admin should see the inactive ticket flagged: %+v", got)
	}
	if _, err := svc.ListTickets(kasir, "", true, false); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not list deleted tickets, got %v", err)
	}

	_, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: entry.ID, Qty: 1}, {TicketID: topup.ID, Qty: 1}}, Paid: 6000000})
	var ve *model.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Message, "Topup") {
		t.Errorf("selling an inactive ticket should fail with its name, got %v", err)
	}

	must(svc.SaveTicket(admin, model.TicketInput{ID: topup.ID, Name: "Topup", Price: 5000000, Category: model.CategoryTopup}))
	if got := must(svc.ListTickets(admin, "Topup", false, false)); got[0].Active {
		t.Error("editing a ticket must not reactivate it")
	}
	if err := svc.SetTicketActive(admin, topup.ID, true); err != nil {
		t.Fatal(err)
	}
	must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: topup.ID, Qty: 1}}, Paid: 5000000}))
}

func TestSearchSales(t *testing.T) {
	local, c := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)
	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup 50%", Price: 5000000, Category: model.CategoryTopup}))

	c.t = time.Date(2026, 10, 4, 9, 0, 0, 0, time.Local)
	must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: entry.ID, Qty: 1}}, Paid: 1000000}))
	c.t = time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)
	var sales []model.Sale
	for i := 0; i < 7; i++ {
		sales = append(sales, must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: entry.ID, Qty: 1}}, Paid: 1000000})))
	}
	withTopup := must(svc.CreateSale(admin, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: topup.ID, Qty: 1}}, Paid: 5000000}))

	month := model.SaleQuery{From: "2026-10-01", To: "2026-10-31"}
	p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: month, Page: 2, PageSize: 3}))
	if p.Count != 9 || p.Amount != 8*1000000+5000000 || p.Page != 2 || len(p.Sales) != 3 || p.Sales[0].ID != sales[4].ID || len(p.Sales[0].Items) != 1 {
		t.Errorf("page 2 wrong: count=%d amount=%d page=%d ids=%v", p.Count, p.Amount, p.Page, len(p.Sales))
	}
	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: month, Page: 9, PageSize: 3})); p.Count != 9 || len(p.Sales) != 0 {
		t.Errorf("page past the end: %+v", p)
	}
	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: month})); p.PageSize != 100 || p.Page != 1 {
		t.Errorf("defaults: page=%d size=%d", p.Page, p.PageSize)
	}
	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: month, PageSize: 10000})); p.PageSize != 500 {
		t.Errorf("page size not capped: %d", p.PageSize)
	}

	for search, want := range map[string]int64{
		"topup 50%":             1, // item name, case-insensitive; % is literal
		withTopup.InvoiceNo[3:]: 1, // part of the invoice number
		"budi":                  8, // cashier
		"loket 1":               9, // every test sale is at counter LOKET 1
		"tidak ada":             0,
	} {
		q := month
		q.Search = search
		if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: q})); p.Count != want {
			t.Errorf("search %q: got %d, want %d", search, p.Count, want)
		}
	}

	// Cashiers get their own sales of today only.
	if p := must(svc.SearchSales(kasir, model.SalePageQuery{SaleQuery: month})); p.Count != 7 {
		t.Errorf("kasir should see own sales of today: %d", p.Count)
	}
	if _, err := svc.SearchSales(kasir, model.SalePageQuery{SaleQuery: model.SaleQuery{From: "2026-10-05", To: "2026-10-05", Deleted: true}}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not search deleted sales: %v", err)
	}
}

func TestSearchTickets(t *testing.T) {
	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)

	var ids []int64
	for i := 0; i < 12; i++ {
		cat := model.CategoryTicket
		if i >= 8 {
			cat = model.CategoryTopup
		}
		ids = append(ids, must(svc.SaveTicket(admin, model.TicketInput{Name: fmt.Sprintf("Item %02d", i), Price: 100000, Category: cat})).ID)
	}
	must(0, svc.SetTicketActive(admin, ids[1], false))
	must(0, svc.SetTicketActive(admin, ids[9], false))
	must(0, svc.DeleteTicket(admin, ids[2]))

	count := func(token string, q model.TicketQuery) int64 {
		t.Helper()
		return must(svc.SearchTickets(token, q)).Count
	}
	for _, tc := range []struct {
		q    model.TicketQuery
		want int64
	}{
		{model.TicketQuery{}, 11},
		{model.TicketQuery{Status: model.TicketActive}, 9},
		{model.TicketQuery{Status: model.TicketInactive}, 2},
		{model.TicketQuery{Status: model.TicketDeleted}, 1},
		{model.TicketQuery{Category: model.CategoryTopup}, 4},
		{model.TicketQuery{Category: model.CategoryTopup, Status: model.TicketActive}, 3},
		{model.TicketQuery{Search: "item 1"}, 2},
	} {
		if got := count(admin, tc.q); got != tc.want {
			t.Errorf("admin %+v: got %d, want %d", tc.q, got, tc.want)
		}
	}

	// Pages follow the cashier screen order: tickets first, then topups.
	p := must(svc.SearchTickets(admin, model.TicketQuery{Page: 3, PageSize: 4}))
	if p.Count != 11 || len(p.Tickets) != 3 || p.Tickets[0].Name != "Item 09" || p.Tickets[0].Category != model.CategoryTopup {
		t.Errorf("page 3 wrong: %+v", p)
	}

	// Cashiers only get active tickets, whatever they ask for.
	if got := count(kasir, model.TicketQuery{Status: model.TicketInactive}); got != 9 {
		t.Errorf("kasir should only see active tickets, got %d", got)
	}
	if _, err := svc.SearchTickets(kasir, model.TicketQuery{Status: model.TicketDeleted}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not see deleted tickets: %v", err)
	}
	if _, err := svc.SearchTickets(admin, model.TicketQuery{Status: "x"}); err == nil {
		t.Error("invalid status accepted")
	}
}

func TestSorting(t *testing.T) {
	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)

	cheap := must(svc.SaveTicket(admin, model.TicketInput{Name: "b murah", Price: 100000}))
	pricey := must(svc.SaveTicket(admin, model.TicketInput{Name: "A mahal", Price: 900000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "C topup", Price: 500000, Category: model.CategoryTopup}))

	names := func(q model.TicketQuery) string {
		var out []string
		for _, tk := range must(svc.SearchTickets(admin, q)).Tickets {
			out = append(out, tk.Name)
		}
		return strings.Join(out, ",")
	}
	for _, tc := range []struct {
		q    model.TicketQuery
		want string
	}{
		{model.TicketQuery{}, "A mahal,b murah,C topup"}, // default: category, then name
		{model.TicketQuery{Sort: "price", Dir: "desc"}, "A mahal,C topup,b murah"},
		{model.TicketQuery{Sort: "price"}, "b murah,C topup,A mahal"},
		{model.TicketQuery{Sort: "name", Dir: "desc"}, "C topup,b murah,A mahal"}, // case-insensitive
	} {
		if got := names(tc.q); got != tc.want {
			t.Errorf("tickets %+v: got %s, want %s", tc.q, got, tc.want)
		}
	}

	sell := func(token string, id int64, qty int64, price int64) model.Sale {
		return must(svc.CreateSale(token, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: id, Qty: qty}}, Paid: qty * price}))
	}
	s1 := sell(kasir, cheap.ID, 1, 100000)  // 1.000, Budi
	s2 := sell(admin, pricey.ID, 1, 900000) // 9.000, Pak Admin
	s3 := sell(kasir, topup.ID, 1, 500000)  // 5.000, Budi
	must(0, svc.RecordPrint(admin, model.PrintInput{SaleID: s1.ID}))
	must(0, svc.RecordPrint(admin, model.PrintInput{SaleID: s1.ID}))
	must(0, svc.RecordPrint(admin, model.PrintInput{SaleID: s3.ID}))

	ids := func(sort, dir string) []int64 {
		q := model.SalePageQuery{SaleQuery: model.SaleQuery{From: "2026-10-05", To: "2026-10-05"}, Sort: sort, Dir: dir}
		var out []int64
		for _, s := range must(svc.SearchSales(admin, q)).Sales {
			out = append(out, s.ID)
		}
		return out
	}
	for _, tc := range []struct {
		sort, dir string
		want      []int64
	}{
		{"", "", []int64{s3.ID, s2.ID, s1.ID}}, // newest first (same second: by id)
		{"total", "asc", []int64{s1.ID, s3.ID, s2.ID}},
		{"total", "desc", []int64{s2.ID, s3.ID, s1.ID}},
		{"prints", "desc", []int64{s1.ID, s3.ID, s2.ID}},
		{"cashier", "asc", []int64{s1.ID, s3.ID, s2.ID}}, // Budi before Pak Admin, ties by id
		{"invoice", "desc", []int64{s3.ID, s2.ID, s1.ID}},
	} {
		if got := ids(tc.sort, tc.dir); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("sales sort %s %s: got %v, want %v", tc.sort, tc.dir, got, tc.want)
		}
	}

	for _, q := range []model.SalePageQuery{
		{SaleQuery: model.SaleQuery{From: "2026-10-05", To: "2026-10-05"}, Sort: "s.total; DROP TABLE sales"},
		{SaleQuery: model.SaleQuery{From: "2026-10-05", To: "2026-10-05"}, Sort: "total", Dir: "sideways"},
	} {
		if _, err := svc.SearchSales(admin, q); err == nil {
			t.Errorf("invalid sort %+v accepted", q)
		}
	}
	if _, err := svc.SearchTickets(admin, model.TicketQuery{Sort: "password"}); err == nil {
		t.Error("invalid ticket sort accepted")
	}
}

func TestReportFilter(t *testing.T) {
	local, _ := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)
	siti := must(svc.SaveUser(admin, model.UserInput{Username: "siti", FullName: "Siti", Role: model.RoleKasir, Password: "1234"}))
	sitiToken := must(svc.Login("siti", "1234")).Token

	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup", Price: 5000000, Category: model.CategoryTopup}))
	loket2 := useCounter(t, svc, admin, "pc-2", "LOKET 2", "2")
	sell := func(token, device string, items ...model.SaleItemInput) {
		must(svc.CreateSale(token, model.SaleInput{DeviceID: device, Items: items, Paid: 1e10}))
	}
	sell(kasir, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 2})                                              // Budi: 20.000
	sell(kasir, "pc-2", model.SaleItemInput{TicketID: entry.ID, Qty: 1}, model.SaleItemInput{TicketID: topup.ID, Qty: 1}) // Budi: 10.000 + 50.000
	sell(sitiToken, "pc-2", model.SaleItemInput{TicketID: topup.ID, Qty: 2})                                              // Siti: 100.000

	day := model.ReportQuery{Period: "daily", From: "2026-10-05", To: "2026-10-05"}
	for _, tc := range []struct {
		f            model.ReportFilter
		transactions int64
		total        int64
		listed       int64
	}{
		{model.ReportFilter{}, 3, 18000000, 3},
		{model.ReportFilter{Category: model.CategoryTopup}, 2, 15000000, 2},
		{model.ReportFilter{Category: model.CategoryTicket}, 2, 3000000, 2},
		{model.ReportFilter{UserID: siti.ID}, 1, 10000000, 1},
		{model.ReportFilter{CounterID: loket2.ID}, 2, 16000000, 2},
		{model.ReportFilter{CounterID: loket2.ID, Category: model.CategoryTicket}, 1, 1000000, 1},
		{model.ReportFilter{CounterID: 999}, 0, 0, 0},
	} {
		q := day
		q.ReportFilter = tc.f
		r := must(svc.Report(admin, q))
		if r.Transactions != tc.transactions || r.Total != tc.total {
			t.Errorf("report %+v: %d transaksi, total %d; want %d, %d", tc.f, r.Transactions, r.Total, tc.transactions, tc.total)
		}
		p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: model.SaleQuery{From: day.From, To: day.To, ReportFilter: tc.f}}))
		if p.Count != tc.listed {
			t.Errorf("sales %+v: %d listed, want %d", tc.f, p.Count, tc.listed)
		}
	}

	// A cashier's own report ignores filters that would show other sales.
	if r := must(svc.Report(kasir, model.ReportQuery{ReportFilter: model.ReportFilter{UserID: siti.ID}})); r.Transactions != 2 {
		t.Errorf("kasir report should stay their own: %+v", r)
	}
	if p := must(svc.SearchSales(kasir, model.SalePageQuery{SaleQuery: model.SaleQuery{ReportFilter: model.ReportFilter{UserID: siti.ID}}})); p.Count != 2 {
		t.Errorf("kasir list should stay their own: %d", p.Count)
	}
	if _, err := svc.Report(admin, model.ReportQuery{Period: "daily", From: day.From, To: day.To, ReportFilter: model.ReportFilter{Category: "voucher"}}); err == nil {
		t.Error("invalid category accepted")
	}

	must(0, svc.DeleteUser(admin, siti.ID))
	f := must(svc.ReportFilters(admin))
	var names []string
	for _, c := range f.Cashiers {
		names = append(names, fmt.Sprintf("%s/%v", c.FullName, c.Deleted))
	}
	var counters []string
	for _, c := range f.Counters {
		counters = append(counters, c.Name)
	}
	if strings.Join(names, ",") != "Budi/false,Pak Admin/false,Siti/true" || strings.Join(counters, ",") != "LOKET 1,LOKET 2" {
		t.Errorf("filter choices: %v %v", names, counters)
	}
	if _, err := svc.ReportFilters(kasir); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not list filter choices: %v", err)
	}
}

func TestCounters(t *testing.T) {
	local, c := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc) // pc-test has LOKET 1 (code 1)
	tk := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	sell := func(device string) (model.Sale, error) {
		return svc.CreateSale(kasir, model.SaleInput{DeviceID: device, Items: []model.SaleItemInput{{TicketID: tk.ID, Qty: 1}}, Paid: 1000000})
	}

	// Master data rules.
	for _, in := range []model.CounterInput{
		{Name: " ", InvoiceCode: "9"},
		{Name: "loket 1", InvoiceCode: "9"}, // name taken, any case
		{Name: "LOKET 9", InvoiceCode: "1"}, // code taken
		{Name: "LOKET 9", InvoiceCode: ""},
		{Name: "LOKET 9", InvoiceCode: "A-1"},
	} {
		if _, err := svc.SaveCounter(admin, in); err == nil {
			t.Errorf("counter %+v accepted", in)
		}
	}
	if _, err := svc.SaveCounter(kasir, model.CounterInput{Name: "LOKET 9", InvoiceCode: "9"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not save counters: %v", err)
	}
	two := must(svc.SaveCounter(admin, model.CounterInput{Name: "LOKET 2", InvoiceCode: "b2"}))
	if two.InvoiceCode != "B2" || !two.Active || two.DeviceID != "" {
		t.Errorf("new counter: %+v", two)
	}

	// One PC per counter, one counter per PC.
	if err := svc.AssignCounter(admin, model.AssignCounterInput{CounterID: two.ID, DeviceID: "pc-a", DeviceName: "KASIR-A"}); err != nil {
		t.Fatal(err)
	}
	err := svc.AssignCounter(admin, model.AssignCounterInput{CounterID: two.ID, DeviceID: "pc-b", DeviceName: "KASIR-B"})
	if err == nil || !strings.Contains(err.Error(), "KASIR-A") {
		t.Errorf("taking a used counter without force: %v", err)
	}
	if err := svc.AssignCounter(admin, model.AssignCounterInput{CounterID: two.ID, DeviceID: "pc-b", DeviceName: "KASIR-B", Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeviceCounter("pc-a"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("pc-a should have lost its counter: %v", err)
	}
	if got := must(svc.DeviceCounter("pc-b")); got.ID != two.ID || got.DeviceName != "KASIR-B" {
		t.Errorf("pc-b counter: %+v", got)
	}
	if err := svc.AssignCounter(kasir, model.AssignCounterInput{CounterID: two.ID, DeviceID: "pc-a"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir must not assign counters: %v", err)
	}

	// The server takes the counter from the PC, not from the request.
	c.t = time.Date(2026, 10, 5, 10, 0, 0, 0, time.Local)
	sale := must(sell("pc-b"))
	if sale.Terminal != "LOKET 2" || sale.CounterID != two.ID || sale.InvoiceNo != "TKTB2202610050001" {
		t.Errorf("sale at pc-b: %s %d %s", sale.Terminal, sale.CounterID, sale.InvoiceNo)
	}
	if _, err := sell("pc-a"); err == nil || !strings.Contains(err.Error(), "belum punya loket") {
		t.Errorf("PC without counter: %v", err)
	}

	// Renaming shows everywhere at once; the receipt keeps the printed name.
	must(svc.SaveCounter(admin, model.CounterInput{ID: two.ID, Name: "LOKET DEPAN", InvoiceCode: "B2"}))
	got := must(svc.GetSale(admin, sale.ID))
	if got.Terminal != "LOKET 2" || got.CounterName != "LOKET DEPAN" {
		t.Errorf("after rename: terminal %q counter %q", got.Terminal, got.CounterName)
	}
	q := model.SaleQuery{From: "2026-10-05", To: "2026-10-05", ReportFilter: model.ReportFilter{CounterID: two.ID}}
	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: q})); p.Count != 1 || p.Sales[0].CounterName != "LOKET DEPAN" {
		t.Errorf("list by counter after rename: %+v", p)
	}
	q.ReportFilter, q.Search = model.ReportFilter{}, "depan"
	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: q})); p.Count != 1 {
		t.Errorf("search by new counter name: %d", p.Count)
	}

	// Turning a counter off frees its PC, which then cannot sell.
	if err := svc.SetCounterActive(admin, two.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := sell("pc-b"); err == nil {
		t.Error("PC of an inactive counter sold")
	}
	if err := svc.AssignCounter(admin, model.AssignCounterInput{CounterID: two.ID, DeviceID: "pc-b"}); err == nil {
		t.Error("inactive counter assigned")
	}

	useCounter(t, svc, admin, "pc-old", "LOKET 3", "3")

	// Prints record the printing PC's counter.
	if err := svc.RecordPrint(kasir, model.PrintInput{SaleID: sale.ID, DeviceID: "pc-old"}); err != nil {
		t.Fatal(err)
	}
	var printed string
	local.Store.DB.QueryRow("SELECT terminal FROM sale_prints WHERE sale_id = ?", sale.ID).Scan(&printed)
	if printed != "LOKET 3" {
		t.Errorf("print terminal %q", printed)
	}
}

func TestSetupCounter(t *testing.T) {
	local, _ := newLocal(t)
	if err := local.SetupCounter("server-pc", "SERVER", "LOKET UTAMA"); err != nil {
		t.Fatal(err)
	}
	c := must(local.Store.CounterByDevice("server-pc"))
	if c.Name != "LOKET UTAMA" || c.InvoiceCode != "1" {
		t.Errorf("first counter: %+v", c)
	}
	// Running setup again keeps the counter; another PC gets the next code.
	must(0, local.SetupCounter("server-pc", "SERVER", "LOKET LAIN"))
	must(0, local.SetupCounter("pc-2", "PC2", "LOKET DUA"))
	if c := must(local.Store.CounterByDevice("server-pc")); c.Name != "LOKET UTAMA" {
		t.Errorf("setup changed the counter: %+v", c)
	}
	if c := must(local.Store.CounterByDevice("pc-2")); c.InvoiceCode != "2" {
		t.Errorf("second counter code: %+v", c)
	}
}
