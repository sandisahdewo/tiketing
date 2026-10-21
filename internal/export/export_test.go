package export

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"tiket/internal/model"
)

func sample() Data {
	return Data{
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
			{InvoiceNo: "TKT202610010001", CashierName: "Budi", Terminal: "LOKET 1", Total: 22000000, Paid: 22000000, CreatedAt: "2026-10-01 10:00:00"},
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

func TestPeriodLabel(t *testing.T) {
	cases := map[[2]string]string{
		{"2026-10-05", "daily"}:  "Senin, 05-10-2026",
		{"2026-10-05", "weekly"}: "05-10-2026 - 11-10-2026",
		{"2026-10", "monthly"}:   "Oktober 2026",
	}
	for in, want := range cases {
		if got := periodLabel(in[0], in[1]); got != want {
			t.Errorf("periodLabel(%v) = %q, want %q", in, got, want)
		}
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
