package service_test

import (
	"bytes"
	"encoding/base64"
	"errors"
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
	return admin, kasir
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

	if got := must(svc.ListTickets(kasir, "week", false)); len(got) != 1 || got[0].ID != weekday.ID {
		t.Fatalf("search failed: %+v", got)
	}

	must(svc.SaveTicket(admin, model.TicketInput{ID: elder.ID, Name: "Lansia", Price: 1500000}))
	if err := svc.DeleteTicket(admin, elder.ID); err != nil {
		t.Fatal(err)
	}
	if got := must(svc.ListTickets(kasir, "", false)); len(got) != 2 {
		t.Fatalf("deleted ticket still listed: %+v", got)
	}
	if got := must(svc.ListTickets(admin, "", true)); len(got) != 1 || got[0].DeletedAt == nil {
		t.Fatalf("deleted list wrong: %+v", got)
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{Items: []model.SaleItemInput{{TicketID: elder.ID, Qty: 1}}, Paid: 9999999}); err == nil {
		t.Fatal("sold a deleted ticket")
	}
	if err := svc.RestoreTicket(admin, elder.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateSale(kasir, model.SaleInput{Items: []model.SaleItemInput{{TicketID: weekday.ID, Qty: 2}}, Paid: 100}); err == nil {
		t.Fatal("underpaid sale accepted")
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{}); err == nil {
		t.Fatal("empty sale accepted")
	}

	sale1 := must(svc.CreateSale(kasir, model.SaleInput{
		Terminal: "LOKET 1",
		Items: []model.SaleItemInput{
			{TicketID: weekday.ID, Qty: 2},
			{TicketID: couple.ID, Qty: 1},
			{TicketID: weekday.ID, Qty: 1},
		},
		Paid: 20000000,
	}))
	if sale1.InvoiceNo != "TKT202610050001" {
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

	sale2 := must(svc.CreateSale(admin, model.SaleInput{Items: []model.SaleItemInput{{TicketID: elder.ID, Qty: 4}}, Paid: 6000000}))
	if sale2.InvoiceNo != "TKT202610050002" {
		t.Errorf("invoice2 = %s", sale2.InvoiceNo)
	}
	if _, err := svc.GetSale(kasir, sale2.ID); !errors.Is(err, model.ErrForbidden) {
		t.Errorf("kasir read another user's sale: %v", err)
	}

	c.t = time.Date(2026, 10, 14, 10, 0, 0, 0, time.Local)
	sale3 := must(svc.CreateSale(kasir, model.SaleInput{Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	if sale3.InvoiceNo != "TKT202610140001" {
		t.Errorf("invoice3 = %s", sale3.InvoiceNo)
	}
	c.t = time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local)
	must(svc.CreateSale(kasir, model.SaleInput{Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 2}}, Paid: 10000100}))

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
	pony1 := must(svc.CreateSale(kasir, model.SaleInput{InvoiceCode: "25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	pony2 := must(svc.CreateSale(kasir, model.SaleInput{InvoiceCode: "25", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	other := must(svc.CreateSale(admin, model.SaleInput{InvoiceCode: "26", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}))
	if pony1.InvoiceNo != "PNY25202611030001" || pony2.InvoiceNo != "PNY25202611030002" || other.InvoiceNo != "PNY26202611030001" {
		t.Errorf("invoice format: %s %s %s", pony1.InvoiceNo, pony2.InvoiceNo, other.InvoiceNo)
	}
	if _, err := svc.CreateSale(kasir, model.SaleInput{InvoiceCode: "12345", Items: []model.SaleItemInput{{TicketID: couple.ID, Qty: 1}}, Paid: 5000050}); err == nil {
		t.Error("too long invoice code accepted")
	}

	if pony1.PrintCount != 0 {
		t.Errorf("new sale print count = %d", pony1.PrintCount)
	}
	for i := 0; i < 3; i++ {
		if err := svc.RecordPrint(kasir, model.PrintInput{SaleID: pony1.ID, Terminal: "LOKET 1"}); err != nil {
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

	sale := must(svc.CreateSale(kasir, model.SaleInput{
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
		return must(svc.CreateSale(token, model.SaleInput{Items: []model.SaleItemInput{{TicketID: tk.ID, Qty: qty}}, Paid: qty * 1000000}))
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
	if next.InvoiceNo != "TKT202610050004" {
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
