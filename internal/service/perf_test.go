//go:build perf

package service_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"tiket/internal/export"
	"tiket/internal/model"
	"tiket/internal/server"
	. "tiket/internal/service"
)

// Run: go test -tags perf -run TestPerf -v -timeout 60m ./internal/service/
func TestPerf(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	local, c := newLocal(t)
	admin, kasir := setup(t, local)
	rng := rand.New(rand.NewSource(1))

	// --- seed -------------------------------------------------------------
	start := time.Now()
	db := local.Store.DB
	var kasirIDs []int64
	for i := 0; i < 5; i++ {
		u := must(local.SaveUser(admin, model.UserInput{Username: fmt.Sprintf("kasir%d", i), FullName: fmt.Sprintf("Kasir %d", i), Role: model.RoleKasir, Password: "1234"}))
		kasirIDs = append(kasirIDs, u.ID)
	}
	// Counters LOKET 1..5 with invoice codes 1..5; setup already made LOKET 1.
	counterIDs := []int64{must(local.Store.CounterByDevice(testDevice)).ID}
	for k := 2; k <= 5; k++ {
		counterIDs = append(counterIDs, must(local.SaveCounter(admin, model.CounterInput{Name: fmt.Sprintf("LOKET %d", k), InvoiceCode: fmt.Sprint(k)})).ID)
	}
	tx := must(db.Begin())
	type tk struct{ id, price int64 }
	var tickets []tk
	for i := 0; i < 1000; i++ {
		cat := model.CategoryTicket
		if i%10 >= 7 {
			cat = model.CategoryTopup
		}
		price := int64(10+rng.Intn(490)) * 1000 * 100
		res := must(tx.Exec(`INSERT INTO tickets (name, price, category, color, created_at, updated_at) VALUES (?, ?, ?, 'orange', '2026-09-01 08:00:00', '2026-09-01 08:00:00')`,
			fmt.Sprintf("Produk %04d", i), price, cat))
		id, _ := res.LastInsertId()
		tickets = append(tickets, tk{id, price})
	}
	insSale := must(tx.Prepare(`INSERT INTO sales (invoice_no, user_id, terminal, counter_id, total, paid, change_amount, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`))
	insItem := must(tx.Prepare(`INSERT INTO sale_items (sale_id, ticket_id, price, qty, subtotal) VALUES (?, ?, ?, ?, ?)`))
	insPrint := must(tx.Prepare(`INSERT INTO sale_prints (sale_id, user_id, terminal, printed_at) VALUES (?, ?, ?, ?)`))
	seedDay := func(day time.Time, n int) {
		secs := make([]int, n)
		for i := range secs {
			secs[i] = 8*3600 + rng.Intn(13*3600)
		}
		sort.Ints(secs)
		seq := map[int]int{}
		for i, s := range secs {
			k := i % len(kasirIDs)
			seq[k]++
			at := day.Add(time.Duration(s) * time.Second).Format("2006-01-02 15:04:05")
			invoice := fmt.Sprintf("TKT%d%s%04d", k+1, day.Format("20060102"), seq[k])
			var total int64
			type line struct {
				t   tk
				qty int64
			}
			var lines []line
			for j := 0; j < 1+rng.Intn(3); j++ {
				l := line{tickets[rng.Intn(len(tickets))], int64(1 + rng.Intn(4))}
				lines = append(lines, l)
				total += l.t.price * l.qty
			}
			res := must(insSale.Exec(invoice, kasirIDs[k], fmt.Sprintf("LOKET %d", k+1), counterIDs[k], total, total, 0, at))
			saleID, _ := res.LastInsertId()
			for _, l := range lines {
				must(insItem.Exec(saleID, l.t.id, l.t.price, l.qty, l.t.price*l.qty))
			}
			must(insPrint.Exec(saleID, kasirIDs[k], fmt.Sprintf("LOKET %d", k+1), at))
		}
	}
	// TIKET_PERF_YEAR=1 adds 100000 sales per month from 1 Oct 2025 to 30 Sep 2026.
	year := os.Getenv("TIKET_PERF_YEAR") != ""
	if year {
		for day := time.Date(2025, 10, 1, 0, 0, 0, 0, time.Local); day.Before(time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)); day = day.AddDate(0, 0, 1) {
			daysInMonth := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
			n := 100000 / daysInMonth
			if day.Day() <= 100000%daysInMonth {
				n++
			}
			seedDay(day, n)
		}
	}
	for d := 1; d <= 20; d++ {
		seedDay(time.Date(2026, 10, d, 0, 0, 0, 0, time.Local), 5000)
	}
	seedDay(time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local), 10000)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	must(db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"))
	var seeded int
	db.QueryRow("SELECT COUNT(*) FROM sales").Scan(&seeded)
	t.Logf("seed: 1000 tiket, %d transaksi dalam %v", seeded, time.Since(start).Round(time.Millisecond))

	// TIKET_PERF_OUT=<file> keeps a copy of the seeded database to open in the app.
	if out := os.Getenv("TIKET_PERF_OUT"); out != "" {
		os.Remove(out)
		must(db.Exec("VACUUM INTO ?", out))
		t.Logf("database disalin ke %s", out)
	}

	c.t = time.Date(2026, 10, 21, 15, 0, 0, 0, time.Local)
	kasir0 := must(local.Login("kasir0", "1234")).Token
	onShift(t, local, admin, kasir, testDevice)

	srv := httptest.NewServer(server.Handler(local, testVersion))
	defer srv.Close()
	remote := NewRemote(srv.URL, testVersion)
	remote.Client.Timeout = 10 * time.Minute

	const today, monthFrom = "2026-10-21", "2026-10-01"

	measure := func(name string, n int, fn func() (any, error)) {
		var ds []time.Duration
		var size int
		for i := 0; i < n; i++ {
			s := time.Now()
			out, err := fn()
			ds = append(ds, time.Since(s))
			if err != nil {
				t.Logf("%-46s GAGAL: %v", name, err)
				return
			}
			if i == 0 && out != nil {
				b, _ := json.Marshal(out)
				size = len(b)
			}
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		sz := ""
		if size > 0 {
			sz = fmt.Sprintf("  JSON %.1f MB", float64(size)/1e6)
		}
		t.Logf("%-46s median %8v  max %8v%s", name, ds[len(ds)/2].Round(100*time.Microsecond), ds[len(ds)-1].Round(100*time.Microsecond), sz)
	}

	if year {
		const yearFrom = "2025-10-01"
		yq := model.SaleQuery{From: yearFrom, To: today}
		t.Logf("--- setahun (%s s/d %s), langsung di server ---", yearFrom, today)
		for _, period := range []string{"daily", "weekly", "monthly"} {
			measure("Ringkasan laporan setahun, "+period, 3, func() (any, error) {
				return local.Report(admin, model.ReportQuery{Period: period, From: yearFrom, To: today})
			})
		}
		measure("Daftar transaksi setahun, halaman 1", 3, func() (any, error) {
			return local.SearchSales(admin, model.SalePageQuery{SaleQuery: yq, Page: 1, PageSize: 100})
		})
		measure("Daftar transaksi setahun, halaman terakhir", 3, func() (any, error) {
			return local.SearchSales(admin, model.SalePageQuery{SaleQuery: yq, Page: 13100, PageSize: 100})
		})
		yqs := yq
		yqs.Search = "Produk 0042"
		measure("Cari item 'Produk 0042' setahun", 3, func() (any, error) {
			return local.SearchSales(admin, model.SalePageQuery{SaleQuery: yqs, Page: 1, PageSize: 100})
		})
		yqs.Search = "TKT3202609150"
		measure("Cari no. nota setahun", 3, func() (any, error) {
			return local.SearchSales(admin, model.SalePageQuery{SaleQuery: yqs, Page: 1, PageSize: 100})
		})
		measure("Laporan kasir hari ini (dengan 1,3 jt data)", 5, func() (any, error) { return local.Report(kasir0, model.ReportQuery{}) })
		measure("Transaksi baru (dengan 1,3 jt data)", 50, func() (any, error) {
			return local.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Paid: 1e12, Items: []model.SaleItemInput{{TicketID: tickets[rng.Intn(1000)].id, Qty: 1}}})
		})
		quarter := model.SaleQuery{From: "2026-08-22", To: today}
		var qSales []model.Sale
		measure("Ambil semua transaksi 2 bulan (untuk Excel)", 1, func() (any, error) {
			var err error
			qSales, err = local.ListSales(admin, quarter)
			return nil, err
		})
		qReport := must(local.Report(admin, model.ReportQuery{Period: "monthly", From: quarter.From, To: quarter.To}))
		qData := export.Data{Settings: must(local.GetSettings(admin)), Report: qReport, Sales: qSales, SalesCount: int64(len(qSales)), PrintedAt: time.Now()}
		measure(fmt.Sprintf("Export Excel 2 bulan (%d transaksi)", len(qSales)), 1, func() (any, error) {
			b, err := export.Excel(qData)
			t.Logf("  Excel 2 bulan: %.1f MB", float64(len(b))/1e6)
			return nil, err
		})
		start := time.Now()
		backup := filepath.Join(t.TempDir(), "backup.db")
		must(db.Exec("VACUUM INTO ?", backup))
		t.Logf("%-46s %v", "Backup harian (VACUUM INTO)", time.Since(start).Round(time.Millisecond))
	}

	for _, side := range []struct {
		name string
		svc  Service
	}{{"server", local}, {"client", remote}} {
		s := side.svc
		t.Logf("--- %s ---", side.name)
		measure("Kasir: ListTickets aktif (1000)", 10, func() (any, error) { return s.ListTickets(kasir0, "", false, true) })
		measure("Riwayat kasir hari ini (~2000 nota)", 5, func() (any, error) {
			return s.ListSales(kasir0, model.SaleQuery{From: today, To: today})
		})
		measure("Riwayat admin hari ini (10000 nota)", 3, func() (any, error) {
			return s.ListSales(admin, model.SaleQuery{From: today, To: today})
		})
		measure("Laporan kasir hari ini", 5, func() (any, error) {
			return s.Report(kasir0, model.ReportQuery{})
		})
		measure("Laporan admin bulan ini, harian", 3, func() (any, error) {
			return s.Report(admin, model.ReportQuery{Period: "daily", From: monthFrom, To: today})
		})
		measure("Laporan admin bulan ini, bulanan", 3, func() (any, error) {
			return s.Report(admin, model.ReportQuery{Period: "monthly", From: monthFrom, To: today})
		})
		measure("Daftar transaksi bulan ini (110000 nota)", 1, func() (any, error) {
			return s.ListSales(admin, model.SaleQuery{From: monthFrom, To: today})
		})
		page := func(q model.SaleQuery, p int) func() (any, error) {
			return func() (any, error) {
				return s.SearchSales(admin, model.SalePageQuery{SaleQuery: q, Page: p, PageSize: 100})
			}
		}
		month := model.SaleQuery{From: monthFrom, To: today}
		measure("Halaman 1 bulan ini (100 dari 110000)", 5, page(month, 1))
		measure("Halaman terakhir bulan ini", 5, page(month, 1100))
		measure("Halaman 1 hari ini (100 dari 10000)", 5, page(model.SaleQuery{From: today, To: today}, 1))
		measure("Cari item 'Produk 0042' bulan ini", 5, page(model.SaleQuery{From: monthFrom, To: today, Search: "Produk 0042"}, 1))
		measure("Cari no. nota 'TKT3202610150' bulan ini", 5, page(model.SaleQuery{From: monthFrom, To: today, Search: "TKT3202610150"}, 1))
		measure("Cari kasir 'Kasir 2' hari ini", 5, page(model.SaleQuery{From: today, To: today, Search: "Kasir 2"}, 1))
		measure("Kasir: halaman 1 riwayat hari ini", 5, func() (any, error) {
			return s.SearchSales(kasir0, model.SalePageQuery{SaleQuery: model.SaleQuery{From: today, To: today}, PageSize: 100})
		})
		measure("Buka nota (GetSale)", 20, func() (any, error) { return s.GetSale(admin, int64(1+rng.Intn(110000))) })
		measure("Transaksi baru (CreateSale)", 50, func() (any, error) {
			return s.CreateSale(kasir, model.SaleInput{DeviceID: testDevice, Paid: 1e12, Items: []model.SaleItemInput{
				{TicketID: tickets[rng.Intn(1000)].id, Qty: 2}, {TicketID: tickets[rng.Intn(1000)].id, Qty: 1},
			}})
		})
		measure("Catat cetak (RecordPrint)", 20, func() (any, error) {
			return nil, s.RecordPrint(admin, model.PrintInput{SaleID: int64(1 + rng.Intn(110000)), DeviceID: testDevice})
		})
		measure("Hapus nota (DeleteSale)", 10, func() (any, error) {
			return nil, s.DeleteSale(admin, model.DeleteSaleInput{ID: int64(1 + rng.Intn(110000)), Reason: "perf"})
		})
	}

	t.Logf("--- export (dihitung di PC yang menekan tombol) ---")
	settings := must(local.GetSettings(admin))
	for _, rng := range []struct{ name, from string }{{"hari ini", today}, {"bulan ini", monthFrom}} {
		report := must(local.Report(admin, model.ReportQuery{Period: "daily", From: rng.from, To: today}))
		sales, err := local.ListSales(admin, model.SaleQuery{From: rng.from, To: today})
		if err != nil {
			t.Logf("Export %-10s GAGAL ambil transaksi: %v", rng.name, err)
			continue
		}
		data := export.Data{Settings: settings, Report: report, Sales: sales, SalesCount: int64(len(sales)), PrintedAt: time.Now()}
		pdfData := data
		if pdfData.SalesCount > export.PDFMaxSales {
			pdfData.Sales = nil // ExportReport only counts them for a long PDF
		}
		measure("Export PDF "+rng.name, 1, func() (any, error) {
			b, err := export.PDF(pdfData)
			t.Logf("  PDF %s: %.0f KB", rng.name, float64(len(b))/1e3)
			return nil, err
		})
		measure("Export Excel "+rng.name, 1, func() (any, error) { _, err := export.Excel(data); return nil, err })
	}

	var dbPath string
	db.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&dbPath)
	if fi, err := os.Stat(dbPath); err == nil {
		t.Logf("ukuran database: %.1f MB (%s)", float64(fi.Size())/1e6, filepath.Base(dbPath))
	}
}
