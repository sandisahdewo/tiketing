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

// onShift lets token sell at the counter of device: it closes the user's
// shift open elsewhere and (as admin) another user's shift holding that
// counter, then opens a shift of the suggested type unless one is open there.
func onShift(t *testing.T, svc Service, admin, token, device string) model.Shift {
	t.Helper()
	st := must(svc.ShiftStatus(token, device))
	if st.Own != nil && st.Own.Shift.CounterID == st.CounterID {
		return st.Own.Shift
	}
	if st.Own != nil {
		closeShift(t, svc, token, st.Own.Shift.ID)
	}
	if st.Blocking != nil {
		closeShift(t, svc, admin, st.Blocking.ID)
	}
	return must(svc.OpenShift(token, model.OpenShiftInput{DeviceID: device, TypeID: st.SuggestedTypeID}))
}

// closeShift closes an open shift with exactly the cash it should have.
func closeShift(t *testing.T, svc Service, token string, id int64) model.ShiftClosing {
	t.Helper()
	r := must(svc.GetShift(token, id))
	return must(svc.CloseShift(token, model.CloseShiftInput{ShiftID: id, ActualCash: r.ExpectedCash}))
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
	if _, err := svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: weekday.ID, Qty: 1}}, Paid: 9999999}); err == nil || !strings.Contains(err.Error(), "Buka shift") {
		t.Fatalf("sold without a shift: %v", err)
	}
	onShift(t, svc, admin, kasir, testDevice)
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

	onShift(t, svc, admin, admin, testDevice)
	sale2 := must(svc.CreateSale(admin, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: elder.ID, Qty: 4}}, Paid: 6000000}))
	if sale2.InvoiceNo != "TKT1202610050002" {
		t.Errorf("invoice2 = %s", sale2.InvoiceNo)
	}
	if _, err := svc.GetSale(kasir, sale2.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir read another user's sale: %v", err)
	}

	c.t = time.Date(2026, 10, 14, 10, 0, 0, 0, time.Local)
	onShift(t, svc, admin, kasir, testDevice)
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
	onShift(t, svc, admin, kasir, "pc-25")
	pony1 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: "pc-25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	pony2 := must(svc.CreateSale(kasir, model.SaleInput{DeviceID: "pc-25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	onShift(t, svc, admin, admin, "pc-26")
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
	if err := svc.DeleteUser(admin, budiID); err == nil || !strings.Contains(err.Error(), "shift Siang yang terbuka di LOKET 25") {
		t.Errorf("deleted a user with an open shift: %v", err)
	}
	closeShift(t, svc, admin, must(svc.ShiftStatus(kasir, "pc-25")).Own.Shift.ID)
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

	onShift(t, svc, admin, kasir, testDevice)
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
		onShift(t, svc, admin, token, testDevice)
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

	onShift(t, svc, admin, kasir, testDevice)
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
	onShift(t, svc, admin, kasir, testDevice)
	must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: entry.ID, Qty: 1}}, Paid: 1000000}))
	c.t = time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)
	var sales []model.Sale
	for i := 0; i < 7; i++ {
		sales = append(sales, must(svc.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Items: []model.SaleItemInput{{TicketID: entry.ID, Qty: 1}}, Paid: 1000000})))
	}
	onShift(t, svc, admin, admin, testDevice)
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
		onShift(t, svc, admin, token, testDevice)
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
		onShift(t, svc, admin, token, device)
		must(svc.CreateSale(token, model.SaleInput{DeviceID: device, Items: items, Paid: 1e10}))
	}
	types := map[string]int64{}
	for _, st := range must(svc.ListShiftTypes(admin)) {
		types[st.Name] = st.ID
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
		{model.ReportFilter{ShiftTypeID: types["Siang"]}, 3, 18000000, 3}, // every sale is at 09:00
		{model.ReportFilter{ShiftTypeID: types["Siang"], UserID: siti.ID}, 1, 10000000, 1},
		{model.ReportFilter{ShiftTypeID: types["Sore"]}, 0, 0, 0},
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

	if p := must(svc.SearchSales(admin, model.SalePageQuery{SaleQuery: model.SaleQuery{From: day.From, To: day.To}, Sort: "shift"})); p.Count != 3 || p.Sales[0].ShiftName != "Siang" {
		t.Errorf("sales should carry their shift: %+v", p.Sales)
	}

	if err := svc.DeleteUser(admin, siti.ID); err == nil {
		t.Error("deleted a cashier with an open shift")
	}
	closeShift(t, svc, admin, must(svc.ShiftStatus(sitiToken, "pc-2")).Own.Shift.ID)
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
	var shifts []string
	for _, st := range f.ShiftTypes {
		shifts = append(shifts, st.Name)
	}
	if strings.Join(names, ",") != "Budi/false,Pak Admin/false,Siti/true" || strings.Join(counters, ",") != "LOKET 1,LOKET 2" ||
		strings.Join(shifts, ",") != "Siang,Sore,Malam" {
		t.Errorf("filter choices: %v %v %v", names, counters, shifts)
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
	onShift(t, svc, admin, kasir, "pc-b")
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

func TestShifts(t *testing.T) {
	local, clk := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc) // pc-test has LOKET 1
	siti := must(svc.SaveUser(admin, model.UserInput{Username: "siti", FullName: "Siti", Role: model.RoleKasir, Password: "1234"}))
	sitiToken := must(svc.Login("siti", "1234")).Token
	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup", Price: 5000000, Category: model.CategoryTopup}))
	useCounter(t, svc, admin, "pc-2", "LOKET 2", "2")
	sell := func(token, device string, items ...model.SaleItemInput) (model.Sale, error) {
		return svc.CreateSale(token, model.SaleInput{DeviceID: device, Items: items, Paid: 1e10})
	}

	// The update brings three shift types; only an admin sets them up.
	types := must(svc.ListShiftTypes(admin))
	if len(types) != 3 || types[0].Name != "Siang" || types[0].StartTime != "06:00" || types[2].Name != "Malam" || types[2].EndTime != "06:00" {
		t.Fatalf("starting types: %+v", types)
	}
	siang, sore, malam := types[0], types[1], types[2]
	if _, err := svc.ListShiftTypes(kasir); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir listed shift types: %v", err)
	}
	if _, err := svc.SaveShiftType(kasir, model.ShiftTypeInput{Name: "Pagi", StartTime: "05:00", EndTime: "09:00"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir saved a shift type: %v", err)
	}
	for _, in := range []model.ShiftTypeInput{
		{Name: " ", StartTime: "05:00", EndTime: "09:00"},
		{Name: "siang", StartTime: "05:00", EndTime: "09:00"}, // name taken, any case
		{Name: strings.Repeat("x", 31), StartTime: "05:00", EndTime: "09:00"},
		{Name: "Pagi", StartTime: "25:00", EndTime: "09:00"},
		{Name: "Pagi", StartTime: "05:00", EndTime: "9"},
		{Name: "Pagi", StartTime: "05:00", EndTime: "05:00"},
	} {
		if _, err := svc.SaveShiftType(admin, in); err == nil {
			t.Errorf("shift type %+v accepted", in)
		}
	}
	pagi := must(svc.SaveShiftType(admin, model.ShiftTypeInput{Name: " Pagi ", StartTime: "5:00", EndTime: "09:30"}))
	if pagi.Name != "Pagi" || pagi.StartTime != "05:00" || pagi.EndTime != "09:30" || !pagi.Active {
		t.Errorf("saved type: %+v", pagi)
	}
	must(0, svc.SetShiftTypeActive(admin, pagi.ID, false))
	if err := svc.SetShiftTypeActive(admin, 999, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown type: %v", err)
	}

	// Nothing sells before a shift opens; the shift running now is suggested.
	st := must(svc.ShiftStatus(kasir, testDevice))
	if st.Own != nil || st.Blocking != nil || st.CounterName != "LOKET 1" || len(st.Types) != 3 || st.SuggestedTypeID != siang.ID ||
		st.ServerTime != "2026-10-05 09:00:00" {
		t.Errorf("status before opening: %+v", st)
	}
	if _, err := sell(kasir, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 1}); err == nil || !strings.Contains(err.Error(), "Buka shift") {
		t.Errorf("sold without a shift: %v", err)
	}
	for _, in := range []model.OpenShiftInput{
		{DeviceID: testDevice, TypeID: siang.ID, OpeningCash: -1},
		{DeviceID: testDevice, TypeID: pagi.ID}, // turned off
		{DeviceID: testDevice, TypeID: 999},
		{DeviceID: "pc-none", TypeID: siang.ID},
	} {
		if _, err := svc.OpenShift(kasir, in); err == nil {
			t.Errorf("opened %+v", in)
		}
	}
	budi := must(svc.OpenShift(kasir, model.OpenShiftInput{DeviceID: testDevice, TypeID: siang.ID, OpeningCash: 50000000}))
	if budi.Name != "Siang" || budi.BusinessDate != "2026-10-05" || budi.ScheduledEndAt != "2026-10-05 14:00:00" || budi.CashierName != "Budi" ||
		budi.CounterName != "LOKET 1" || budi.OpeningCash != 50000000 || budi.OpenedAt != "2026-10-05 09:00:00" || budi.ClosedAt != nil {
		t.Errorf("opened: %+v", budi)
	}

	// One open shift per cashier and per counter; only its cashier or an
	// admin reaches it.
	if _, err := svc.OpenShift(kasir, model.OpenShiftInput{DeviceID: "pc-2", TypeID: siang.ID}); err == nil || !strings.Contains(err.Error(), "LOKET 1") {
		t.Errorf("second shift of a cashier: %v", err)
	}
	if _, err := svc.OpenShift(sitiToken, model.OpenShiftInput{DeviceID: testDevice, TypeID: siang.ID}); err == nil || !strings.Contains(err.Error(), "Budi") {
		t.Errorf("second shift at a counter: %v", err)
	}
	if st := must(svc.ShiftStatus(sitiToken, testDevice)); st.Own != nil || st.Blocking == nil || st.Blocking.ID != budi.ID {
		t.Errorf("status of the next cashier: %+v", st)
	}
	if _, err := svc.GetShift(sitiToken, budi.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("siti read budi's shift: %v", err)
	}
	if _, err := svc.CloseShift(sitiToken, model.CloseShiftInput{ShiftID: budi.ID}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("siti closed budi's shift: %v", err)
	}

	// Sales go into the shift, at the counter of the shift only.
	first := must(sell(kasir, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 2})) // 20.000
	clk.t = clk.t.Add(4 * time.Hour)
	must(sell(kasir, testDevice, model.SaleItemInput{TicketID: topup.ID, Qty: 1})) // 50.000
	if first.ShiftName != "Siang" {
		t.Errorf("sale shift %q", first.ShiftName)
	}
	if _, err := sell(kasir, "pc-2", model.SaleItemInput{TicketID: entry.ID, Qty: 1}); err == nil || !strings.Contains(err.Error(), "masih terbuka di LOKET 1") {
		t.Errorf("sold at another counter: %v", err)
	}
	own := must(svc.ShiftStatus(kasir, testDevice)).Own
	if own == nil || own.Shift.ID != budi.ID || own.Sales.Transactions != 2 || own.Sales.Total != 7000000 || own.Sales.TicketQty != 2 ||
		own.Sales.TopupTotal != 5000000 || own.ExpectedCash != 57000000 || own.Closing != nil ||
		own.Sales.FirstSaleAt != "2026-10-05 09:00:00" || own.Sales.LastSaleAt != "2026-10-05 13:00:00" {
		t.Errorf("own shift: %+v", own)
	}

	// Closing: a difference needs a note; a stale revision is refused.
	clk.t = clk.t.Add(90 * time.Minute)
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, ActualCash: 56900000}); err == nil || !strings.Contains(err.Error(), "Keterangan selisih") {
		t.Errorf("difference without a note: %v", err)
	}
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, Revision: 1, ActualCash: 57000000, Note: "x"}); err == nil || !strings.Contains(err.Error(), "Muat ulang") {
		t.Errorf("correction of an open shift: %v", err)
	}
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, ActualCash: -1, Note: "x"}); err == nil {
		t.Error("negative cash accepted")
	}
	closing := must(svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, ActualCash: 56900000, Note: " kurang seribu "}))
	if closing.Revision != 0 || closing.ExpectedCash != 57000000 || closing.Difference != -100000 || closing.Sales.Total != 7000000 ||
		closing.Note != "kurang seribu" || closing.ClosedByName != "Budi" || closing.PreviousActual != nil ||
		closing.Shift.ClosedAt == nil || *closing.Shift.ClosedAt != "2026-10-05 14:30:00" {
		t.Errorf("closing: %+v", closing)
	}
	if _, err := sell(kasir, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 1}); err == nil || !strings.Contains(err.Error(), "Buka shift") {
		t.Errorf("sold after closing: %v", err)
	}
	// The same closing sent again, e.g. from another PC, is not a correction.
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, ActualCash: 56900000, Note: "kurang seribu"}); err == nil || !strings.Contains(err.Error(), "Muat ulang") {
		t.Errorf("closed twice: %v", err)
	}

	// A correction needs a reason and becomes the closing that counts.
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, Revision: 1, ActualCash: 57000000}); err == nil || !strings.Contains(err.Error(), "Alasan koreksi") {
		t.Errorf("correction without a reason: %v", err)
	}
	fixed := must(svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, Revision: 1, ActualCash: 57000000, Note: "salah hitung"}))
	if fixed.Revision != 1 || fixed.Difference != 0 || fixed.PreviousActual == nil || *fixed.PreviousActual != 56900000 {
		t.Errorf("correction: %+v", fixed)
	}

	// The counter is free again for the next cashier.
	sore2 := must(svc.OpenShift(sitiToken, model.OpenShiftInput{DeviceID: testDevice, TypeID: sore.ID, OpeningCash: 20000000}))
	must(sell(sitiToken, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 1})) // 10.000

	// The closing keeps the sales as they were; a sale deleted later only
	// marks the shift as changed.
	must(0, svc.DeleteSale(admin, model.DeleteSaleInput{ID: first.ID, Reason: "salah input"}))
	r := must(svc.GetShift(kasir, budi.ID))
	if !r.Changed || r.Closing == nil || r.Closing.ID != fixed.ID || r.Closing.Sales.Total != 7000000 || r.Sales.Total != 5000000 ||
		r.Sales.DeletedCount != 1 || len(r.History) != 2 || r.History[1].ID != closing.ID {
		t.Errorf("changed shift: %+v", r)
	}
	if len(r.Deleted) != 1 || r.Deleted[0].ID != first.ID || r.Deleted[0].Reason != "salah input" || r.Deleted[0].DeletedByName != "Pak Admin" ||
		len(r.Closing.Deleted) != 0 {
		t.Errorf("deleted sales: shift %+v, closing %+v", r.Deleted, r.Closing.Deleted)
	}

	// Reprints are counted.
	must(0, svc.RecordShiftClosingPrint(kasir, fixed.ID))
	if r := must(svc.GetShift(kasir, budi.ID)); r.Closing.PrintCount != 1 || r.Closing.PreviousActual == nil {
		t.Errorf("printed closing: %+v", r.Closing)
	}
	if err := svc.RecordShiftClosingPrint(sitiToken, fixed.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("siti printed budi's closing: %v", err)
	}

	// A night shift crosses midnight and belongs to the day it started.
	clk.t = time.Date(2026, 10, 5, 23, 30, 0, 0, time.Local)
	if st := must(svc.ShiftStatus(kasir, "pc-2")); st.SuggestedTypeID != malam.ID || st.Own != nil || st.Blocking != nil {
		t.Errorf("status at night: %+v", st)
	}
	night := must(svc.OpenShift(kasir, model.OpenShiftInput{DeviceID: "pc-2", TypeID: malam.ID, OpeningCash: 10000000}))
	clk.t = time.Date(2026, 10, 6, 1, 30, 0, 0, time.Local)
	must(sell(kasir, "pc-2", model.SaleItemInput{TicketID: topup.ID, Qty: 2})) // 100.000
	if night.BusinessDate != "2026-10-05" || night.ScheduledEndAt != "2026-10-06 06:00:00" {
		t.Errorf("night shift: %+v", night)
	}

	// The admin gets the shifts of a date and the ones left open on others.
	day := must(svc.ShiftDay(admin, model.ShiftDayQuery{Date: "2026-10-05"}))
	if len(day.Shifts) != 3 || day.Shifts[0].Shift.ID != budi.ID || day.Shifts[1].Shift.ID != sore2.ID || day.Shifts[2].Shift.ID != night.ID ||
		day.Shifts[2].Sales.Total != 10000000 || day.Shifts[2].ExpectedCash != 20000000 || len(day.Unclosed) != 0 {
		t.Errorf("admin day: %+v", day)
	}
	if next := must(svc.ShiftDay(admin, model.ShiftDayQuery{Date: "2026-10-06"})); len(next.Shifts) != 0 || len(next.Unclosed) != 2 {
		t.Errorf("next day: %+v", next)
	}
	if _, err := svc.ShiftDay(admin, model.ShiftDayQuery{Date: "06-10-2026"}); err == nil {
		t.Error("bad date accepted")
	}

	// A cashier gets their shifts open or closed today, whatever the date
	// asked; yesterday's closing is out of reach, not for an admin.
	mine := must(svc.ShiftDay(kasir, model.ShiftDayQuery{Date: "2026-01-01"}))
	if mine.Date != "2026-10-06" || len(mine.Shifts) != 1 || mine.Shifts[0].Shift.ID != night.ID {
		t.Errorf("kasir day: %+v", mine)
	}
	if _, err := svc.GetShift(kasir, budi.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir read yesterday's shift: %v", err)
	}
	if _, err := svc.CloseShift(kasir, model.CloseShiftInput{ShiftID: budi.ID, Revision: 2, ActualCash: 1, Note: "x"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir corrected yesterday's shift: %v", err)
	}
	must(svc.GetShift(admin, budi.ID))

	q := model.ReportQuery{Period: "daily", From: "2026-10-05", To: "2026-10-06", ReportFilter: model.ReportFilter{ShiftTypeID: malam.ID}}
	if r := must(svc.Report(admin, q)); r.Transactions != 1 || r.Total != 10000000 {
		t.Errorf("report of the night shifts: %+v", r)
	}

	// A cashier with an open shift is only deleted once it is closed, here by
	// an admin.
	if err := svc.DeleteUser(admin, siti.ID); err == nil || !strings.Contains(err.Error(), "Sore") {
		t.Errorf("deleted a cashier with an open shift: %v", err)
	}
	byAdmin := closeShift(t, svc, admin, sore2.ID)
	if byAdmin.ClosedByName != "Pak Admin" || byAdmin.Shift.CashierName != "Siti" || byAdmin.ExpectedCash != 21000000 {
		t.Errorf("closed by admin: %+v", byAdmin)
	}
	must(0, svc.DeleteUser(admin, siti.ID))
}

func TestSaleCorrection(t *testing.T) {
	local, clk := newLocal(t)
	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	svc := NewRemote(srv.URL, testVersion)
	admin, kasir := setup(t, svc)
	entry := must(svc.SaveTicket(admin, model.TicketInput{Name: "Entry", Price: 1000000}))
	topup := must(svc.SaveTicket(admin, model.TicketInput{Name: "Topup", Price: 5000000, Category: model.CategoryTopup}))
	useCounter(t, svc, admin, "pc-2", "LOKET 2", "2")
	sell := func(token, device string, items ...model.SaleItemInput) model.Sale {
		return must(svc.CreateSale(token, model.SaleInput{DeviceID: device, Items: items, Paid: 1e10}))
	}
	correct := func(token, device string, id int64, reason string, items ...model.SaleItemInput) (model.Sale, error) {
		return svc.CreateSale(token, model.SaleInput{DeviceID: device, Items: items, Paid: 1e10, ReplacesSaleID: id, Reason: reason})
	}

	budi := onShift(t, svc, admin, kasir, testDevice)
	wrong := sell(kasir, testDevice, model.SaleItemInput{TicketID: entry.ID, Qty: 3}) // 30.000

	// A cashier changes no sale, not even their own of the shift still open:
	// lowering a sale after the customer left would take money unseen.
	if _, err := correct(kasir, testDevice, wrong.ID, "salah jumlah", model.SaleItemInput{TicketID: entry.ID, Qty: 2}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir corrected a sale: %v", err)
	}
	if err := svc.DeleteSale(kasir, model.DeleteSaleInput{ID: wrong.ID, Reason: "batal"}); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir deleted a sale: %v", err)
	}

	// An admin corrects it from another PC without a shift of their own: while
	// the cashier's shift is open the new sale goes into it, as the cashier's
	// sale at that counter, since the money changes hands at that drawer.
	if _, err := correct(admin, "pc-2", wrong.ID, " ", model.SaleItemInput{TicketID: entry.ID, Qty: 2}); err == nil || !strings.Contains(err.Error(), "Alasan koreksi") {
		t.Errorf("correction without a reason: %v", err)
	}
	fixed := must(correct(admin, "pc-2", wrong.ID, " salah jumlah ", model.SaleItemInput{TicketID: entry.ID, Qty: 2}))
	if fixed.ReplacesSaleID != wrong.ID || fixed.ReplacesInvoiceNo != wrong.InvoiceNo || fixed.Total != 2000000 || fixed.ShiftID != budi.ID ||
		fixed.CashierName != "Budi" || fixed.CounterName != "LOKET 1" || !strings.HasPrefix(fixed.InvoiceNo, "TKT1") {
		t.Errorf("correction: %+v", fixed)
	}
	old := must(svc.GetSale(kasir, wrong.ID))
	if old.DeletedAt == nil || old.DeleteReason != "salah jumlah" || old.DeletedByName != "Pak Admin" || old.ReplacedByInvoiceNo != fixed.InvoiceNo {
		t.Errorf("corrected sale: %+v", old)
	}
	if _, err := correct(admin, "pc-2", wrong.ID, "lagi", model.SaleItemInput{TicketID: entry.ID, Qty: 1}); err == nil || !strings.Contains(err.Error(), "sudah dihapus") {
		t.Errorf("corrected twice: %v", err)
	}
	// The shift counts the new sale and lists the deleted one for review.
	r := must(svc.GetShift(kasir, budi.ID))
	if r.Sales.Total != 2000000 || r.Sales.Transactions != 1 || r.Sales.DeletedCount != 1 || r.Sales.DeletedTotal != 3000000 ||
		len(r.Deleted) != 1 || r.Deleted[0].InvoiceNo != wrong.InvoiceNo || r.Deleted[0].Reason != "salah jumlah" ||
		r.Deleted[0].DeletedByName != "Pak Admin" || r.Deleted[0].ReplacedByInvoiceNo != fixed.InvoiceNo {
		t.Errorf("shift after the correction: %+v %+v", r.Sales, r.Deleted)
	}

	// An admin deletes a sale outright, also without a shift.
	cancelled := sell(kasir, testDevice, model.SaleItemInput{TicketID: topup.ID, Qty: 1})
	if err := svc.DeleteSale(admin, model.DeleteSaleInput{ID: cancelled.ID, Reason: " "}); err == nil {
		t.Error("deleted without a reason")
	}
	must(0, svc.DeleteSale(admin, model.DeleteSaleInput{ID: cancelled.ID, Reason: "batal beli"}))

	// Once the shift is closed, a correction goes into the admin's own shift
	// at the PC it is made on, and the closed shift shows the change.
	closing := closeShift(t, svc, kasir, budi.ID)
	if len(closing.Deleted) != 0 {
		t.Errorf("a closing saved by itself lists no deleted sales: %+v", closing.Deleted)
	}
	clk.t = clk.t.Add(time.Hour)
	if _, err := correct(admin, "pc-2", fixed.ID, "harusnya plus topup", model.SaleItemInput{TicketID: entry.ID, Qty: 1}); err == nil || !strings.Contains(err.Error(), "Buka shift") {
		t.Errorf("correction of a closed shift without a shift of one's own: %v", err)
	}
	mine := onShift(t, svc, admin, admin, "pc-2")
	byAdmin := must(correct(admin, "pc-2", fixed.ID, "harusnya plus topup", model.SaleItemInput{TicketID: entry.ID, Qty: 2}, model.SaleItemInput{TicketID: topup.ID, Qty: 1}))
	if byAdmin.ReplacesSaleID != fixed.ID || byAdmin.CashierName != "Pak Admin" || byAdmin.ShiftID != mine.ID || byAdmin.Total != 7000000 {
		t.Errorf("correction by admin: %+v", byAdmin)
	}
	r = must(svc.GetShift(admin, budi.ID))
	if !r.Changed || r.Sales.Total != 0 || r.Sales.DeletedCount != 3 || len(r.Deleted) != 3 {
		t.Errorf("closed shift after the admin's correction: %+v", r)
	}
	// The closing keeps listing what was deleted by the time it was saved.
	if len(r.Closing.Deleted) != 2 || r.Closing.Deleted[0].InvoiceNo != wrong.InvoiceNo || r.Closing.Deleted[1].InvoiceNo != cancelled.InvoiceNo {
		t.Errorf("closing's deleted sales: %+v", r.Closing.Deleted)
	}

	// Corrections chain on: each version names the one before and after it,
	// and only the last one counts.
	last := must(correct(admin, "pc-2", byAdmin.ID, "salah lagi", model.SaleItemInput{TicketID: topup.ID, Qty: 2}))
	if r := must(svc.GetShift(admin, mine.ID)); r.Sales.Transactions != 1 || r.Sales.Total != 10000000 || r.Sales.DeletedCount != 1 {
		t.Errorf("admin's shift after the third correction: %+v", r.Sales)
	}
	day := model.SaleQuery{From: "2026-10-05", To: "2026-10-05"}
	deleted := day
	deleted.Deleted = true
	byID := map[int64]model.Sale{}
	for _, q := range []model.SaleQuery{day, deleted} {
		for _, s := range must(svc.ListSales(admin, q)) {
			byID[s.ID] = s
		}
	}
	if byID[fixed.ID].ReplacedByInvoiceNo != byAdmin.InvoiceNo || byID[fixed.ID].ReplacesInvoiceNo != wrong.InvoiceNo ||
		byID[byAdmin.ID].ReplacesInvoiceNo != fixed.InvoiceNo || byID[byAdmin.ID].ReplacedByInvoiceNo != last.InvoiceNo ||
		byID[last.ID].ReplacesInvoiceNo != byAdmin.InvoiceNo || byID[last.ID].DeletedAt != nil || byID[cancelled.ID].ReplacedByInvoiceNo != "" {
		t.Errorf("sale links: %+v", byID)
	}
	if r := must(svc.Report(admin, model.ReportQuery{Period: "daily", From: "2026-10-05", To: "2026-10-05"})); r.Transactions != 1 || r.Total != 10000000 {
		t.Errorf("report after three corrections: %d transaksi, total %d", r.Transactions, r.Total)
	}
}
