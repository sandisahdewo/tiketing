package export

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"tiket/internal/model"
)

func sample() Data {
	return Data{
		Kind:     KindAll,
		Settings: model.Settings{BusinessName: "Pony Park"},
		Report: model.Report{
			Period: "daily", From: "2026-10-01", To: "2026-10-02",
			Rows: []model.ReportRow{
				{Period: "2026-10-01", Transactions: 2, Qty: 5, Total: 22000000, TicketQty: 4, TicketTotal: 14000000, TopupQty: 1, TopupTotal: 8000000},
				{Period: "2026-10-02", Transactions: 1, Qty: 1, Total: 5000000, TopupQty: 1, TopupTotal: 5000000},
			},
			Tickets: []model.ReportTicketRow{
				{TicketID: 1, TicketName: "Entry Weekday", Category: model.CategoryTicket, Qty: 4, Total: 14000000},
				{TicketID: 2, TicketName: "Topup 100.000", Category: model.CategoryTopup, Qty: 1, Total: 8000000},
				{TicketID: 3, TicketName: "Topup 50.000", Category: model.CategoryTopup, Qty: 1, Total: 5000000},
			},
			Transactions: 3, Qty: 6, Total: 27000000, TicketQty: 4, TicketTotal: 14000000, TopupQty: 2, TopupTotal: 13000000,
		},
		Sales: []model.Sale{
			{InvoiceNo: "TKT202610010001", CashierName: "Budi", Terminal: "LOKET 1", ShiftName: "Siang", Total: 22000000, Paid: 22000000, CreatedAt: "2026-10-01 10:00:00"},
			{InvoiceNo: "TKT202610020001", CashierName: "Budi", Terminal: "LOKET 1", Total: 5000000, Paid: 5000000, CreatedAt: "2026-10-02 11:00:00"},
		},
		PrintedBy: "Pak Admin",
		PrintedAt: time.Date(2026, 10, 17, 9, 0, 0, 0, time.Local),
	}
}

func TestExcel(t *testing.T) {
	body, err := Excel(sample())
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetSheetList(); len(got) != 3 || got[0] != "Ringkasan" {
		t.Fatalf("sheets = %v", got)
	}
	rows, _ := f.GetRows("Per Item")
	var found bool
	for _, r := range rows {
		if len(r) >= 4 && r[0] == "Subtotal Topup Card" && r[3] == "130,000" {
			found = true
		}
	}
	if !found {
		t.Errorf("topup subtotal missing: %v", rows)
	}
	// The transaction list names each sale's shift; sales from before shifts have none.
	rows, _ = f.GetRows("Transaksi")
	var header, first, second []string
	for i, r := range rows {
		if len(r) > 1 && r[0] == "Waktu" && i+2 < len(rows) {
			header, first, second = r, rows[i+1], rows[i+2]
		}
	}
	if len(header) < 5 || header[4] != "Shift" || len(first) < 5 || first[4] != "Siang" || len(second) < 5 || second[4] != "-" {
		t.Errorf("shift column: %v / %v / %v", header, first, second)
	}
	if dir := os.Getenv("EXPORT_SAMPLE_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "sample.xlsx"), body, 0o644)
	}
}

func TestPDF(t *testing.T) {
	body, err := PDF(sample())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	if dir := os.Getenv("EXPORT_SAMPLE_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "sample.pdf"), body, 0o644)
	}
}

func TestPDFOmitsLongSalesList(t *testing.T) {
	small, err := PDF(sample())
	if err != nil {
		t.Fatal(err)
	}
	full := sample()
	for len(full.Sales) < PDFMaxSales {
		full.Sales = append(full.Sales, full.Sales[0])
	}
	listed, err := PDF(full)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) < len(small)+20000 {
		t.Fatalf("%d transactions should still be listed: %d vs %d bytes", PDFMaxSales, len(listed), len(small))
	}

	big := sample()
	big.Sales = nil
	big.SalesCount = PDFMaxSales + 1
	omitted, err := PDF(big)
	if err != nil {
		t.Fatal(err)
	}
	if len(omitted) > len(small)+5000 {
		t.Errorf("list above %d should be omitted: %d vs %d bytes", PDFMaxSales, len(omitted), len(small))
	}
}

func TestExcelOneTab(t *testing.T) {
	deletedAt := "2026-10-02 12:00:00"
	cases := []struct {
		kind    string
		period  string
		deleted bool
		sheet   string
		wants   []string
		absent  []string
	}{
		{KindPeriod, "daily", false, "Per Hari", []string{"Penjualan Per Hari Pony Park", "Kamis, 01-10-2026"}, []string{"Ringkasan"}},
		{KindPeriod, "weekly", false, "Per Minggu", []string{"Penjualan Per Minggu Pony Park"}, nil},
		{KindItems, "daily", false, "Per Item", []string{"Penjualan Per Item Pony Park", "Entry Weekday", "Subtotal Topup Card"}, nil},
		{KindSales, "daily", false, "Transaksi", []string{"Daftar Transaksi Pony Park", "TKT202610010001", "Bayar"}, []string{"Alasan"}},
		{KindSales, "daily", true, "Nota Dihapus", []string{"Nota Dihapus Pony Park", "TKT202610010001", "Pak Admin", "salah input", "02-10-2026 12:00", "TKT2"}, []string{"Bayar"}},
	}
	for _, c := range cases {
		d := sample()
		d.Kind, d.Deleted, d.Report.Period = c.kind, c.deleted, c.period
		d.Sales[0].DeletedAt, d.Sales[0].DeletedByName, d.Sales[0].DeleteReason, d.Sales[0].ReplacedByInvoiceNo = &deletedAt, "Pak Admin", "salah input", "TKT2"
		body, err := Excel(d)
		if err != nil {
			t.Fatal(err)
		}
		f, err := excelize.OpenReader(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if got := f.GetSheetList(); len(got) != 1 || got[0] != c.sheet {
			t.Errorf("%s: sheets = %v", c.sheet, got)
			continue
		}
		rows, _ := f.GetRows(c.sheet)
		var cells []string
		for _, r := range rows {
			cells = append(cells, r...)
		}
		for _, want := range c.wants {
			if !slices.Contains(cells, want) {
				t.Errorf("%s: missing %q in %v", c.sheet, want, cells)
			}
		}
		for _, no := range c.absent {
			if slices.Contains(cells, no) {
				t.Errorf("%s: %q should not be there: %v", c.sheet, no, cells)
			}
		}
	}
}

func TestPDFOneTab(t *testing.T) {
	whole, err := PDF(sample())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{KindPeriod, KindItems, KindSales} {
		d := sample()
		d.Kind = kind
		body, err := PDF(d)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(body, []byte("%PDF")) || len(body) >= len(whole) {
			t.Errorf("%s: %d bytes, the whole report %d", kind, len(body), len(whole))
		}
	}
	d := sample()
	d.Kind, d.Deleted = KindSales, true
	if body, err := PDF(d); err != nil || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Errorf("deleted sales: %v", err)
	}
}

func TestFileName(t *testing.T) {
	d := sample()
	cases := []struct {
		kind    string
		deleted bool
		want    string
	}{
		{KindAll, false, "Laporan 2026-10-01 sd 2026-10-02.pdf"},
		{KindPeriod, false, "Laporan Per Hari 2026-10-01 sd 2026-10-02.pdf"},
		{KindItems, false, "Laporan Per Item 2026-10-01 sd 2026-10-02.pdf"},
		{KindSales, false, "Laporan Transaksi 2026-10-01 sd 2026-10-02.pdf"},
		{KindSales, true, "Laporan Nota Dihapus 2026-10-01 sd 2026-10-02.pdf"},
	}
	for _, c := range cases {
		d.Kind, d.Deleted = c.kind, c.deleted
		if got := FileName(d, "pdf"); got != c.want {
			t.Errorf("FileName = %q, want %q", got, c.want)
		}
	}
	d.Kind, d.Report.To = KindAll, d.Report.From
	if got := FileName(d, "xlsx"); got != "Laporan 2026-10-01.xlsx" {
		t.Errorf("one day: %q", got)
	}
}
