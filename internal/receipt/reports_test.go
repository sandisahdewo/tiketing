package receipt

import (
	"strings"
	"testing"
	"time"

	"tiket/internal/model"
)

var reportPrinted = time.Date(2026, 11, 1, 21, 15, 0, 0, time.Local)

func contains(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
}

func TestItemSales(t *testing.T) {
	long := "Entry Ticket Weekend / Holiday Dengan Nama Yang Sangat Panjang Sekali"
	r := model.Report{
		From: "2026-11-01", To: "2026-11-01", Qty: 121, Total: 690000000,
		Tickets: []model.ReportTicketRow{
			{TicketID: 1, TicketName: long, Category: model.CategoryTicket, Qty: 98, Total: 441000000},
			{TicketID: 2, TicketName: "CARD 10K + SALDO 40K", Category: model.CategoryTicket, Qty: 10, Total: 50000000},
			{TicketID: 3, TicketName: "SALDO (90K)", Category: model.CategoryTopup, Qty: 13, Total: 199000000},
		},
	}
	filter := []string{"Kategori: Tiket", "Kasir: Budi"}
	for _, paper := range []int{58, 80} {
		text := fits(t, ItemSales(model.Settings{BusinessName: "Pantai"}, r, filter, paper, reportPrinted), Columns(paper))
		contains(t, text, "PENJUALAN PER ITEM", "Tanggal   : 01-11-2026", "Dicetak   : 01-11-2026 21:15:00",
			"Kategori: Tiket", "Kasir: Budi", "TIKET", "- CARD 10K + SALDO 40K", "4.410.000", "Total Tiket", "4.910.000",
			"TOPUP CARD", "1.990.000", "Total Topup Card", "TOTAL", "6.900.000", "SELESAI")
		if strings.Contains(text, "?") {
			t.Errorf("%dmm: unprintable character in\n%s", paper, text)
		}
		// The full name is kept, and items are sorted whatever their case.
		if !strings.Contains(strings.Join(strings.Fields(text), " "), "- "+long) {
			t.Errorf("%dmm: name cut in\n%s", paper, text)
		}
		if strings.Index(text, "CARD 10K") > strings.Index(text, "Entry Ticket") {
			t.Errorf("%dmm: not by name:\n%s", paper, text)
		}
		if strings.Index(text, "Total Tiket") > strings.Index(text, "TOPUP CARD") {
			t.Errorf("%dmm: subtotal after the next group:\n%s", paper, text)
		}
	}

	r.To = "2026-11-07"
	contains(t, fits(t, ItemSales(model.Settings{}, r, nil, 58, reportPrinted), 32), "01-11-2026 s/d", "07-11-2026")

	empty := Text(ItemSales(model.Settings{}, model.Report{From: "2026-11-01", To: "2026-11-01"}, nil, 80, reportPrinted))
	contains(t, empty, "Tidak ada penjualan.")
}

func TestPeriodSales(t *testing.T) {
	r := model.Report{
		Period: "daily", From: "2026-10-05", To: "2026-10-06",
		Transactions: 3, TicketQty: 4, TicketTotal: 10000000, TopupQty: 2, TopupTotal: 20000000, Total: 30000000,
		Rows: []model.ReportRow{
			{Period: "2026-10-05", Transactions: 2, TicketQty: 3, TicketTotal: 7500000, TopupQty: 2, TopupTotal: 20000000, Total: 27500000},
			{Period: "2026-10-06", Transactions: 1, TicketQty: 1, TicketTotal: 2500000, Total: 2500000},
		},
	}
	for _, paper := range []int{58, 80} {
		text := fits(t, PeriodSales(model.Settings{}, r, []string{"Shift: Malam"}, paper, reportPrinted), Columns(paper))
		contains(t, text, "PENJUALAN PER HARI", "Shift: Malam", "Senin, 05-10-2026", "Selasa, 06-10-2026",
			"Tiket (3)", "75.000", "Topup (2)", "200.000", "275.000", "25.000", "TOTAL", "Tiket (4)", "300.000", "SELESAI")
		if strings.Index(text, "05-10-2026") > strings.Index(text, "06-10-2026") {
			t.Errorf("%dmm: not in order:\n%s", paper, text)
		}
	}

	r.Period, r.Rows = "weekly", []model.ReportRow{{Period: "2026-10-05", Transactions: 3, Total: 30000000}}
	contains(t, fits(t, PeriodSales(model.Settings{}, r, nil, 58, reportPrinted), 32), "PENJUALAN PER MINGGU", "05-10-2026 - 11-10-2026")
	r.Period, r.Rows = "monthly", []model.ReportRow{{Period: "2026-10", Transactions: 3, Total: 30000000}}
	contains(t, fits(t, PeriodSales(model.Settings{}, r, nil, 58, reportPrinted), 32), "PENJUALAN PER BULAN", "Oktober 2026")

	r.Rows = nil
	contains(t, Text(PeriodSales(model.Settings{}, r, nil, 58, reportPrinted)), "Tidak ada penjualan.")
}

func TestSaleList(t *testing.T) {
	r := model.Report{From: "2026-10-05", To: "2026-10-05"}
	deletedAt := "2026-10-05 10:30:00"
	sales := []model.Sale{ // newest first, as the store returns them
		{ID: 2, InvoiceNo: "TKT1202610050002", CashierName: "Budi Santoso Wijaya Kusuma Pratama", Total: 123456700, CreatedAt: "2026-10-05 10:00:00",
			DeletedAt: &deletedAt, DeletedByName: "Pak Admin", DeleteReason: "salah jumlah tiket", ReplacedByInvoiceNo: "TKT1202610050003"},
		{ID: 1, InvoiceNo: "TKT1202610050001", CashierName: "Budi", Total: 7500000, CreatedAt: "2026-10-05 09:00:00"},
	}
	for _, paper := range []int{58, 80} {
		text := fits(t, SaleList(model.Settings{}, r, sales, false, []string{"Cari: TKT"}, paper, reportPrinted), Columns(paper))
		contains(t, text, "DAFTAR TRANSAKSI", "Cari: TKT", "TKT1202610050001", "75.000", "05-10-2026 09:00 Budi", "1.234.567", "Jumlah nota", "1.309.567", "SELESAI")
		if strings.Contains(text, "Alasan") {
			t.Errorf("%dmm: reason on a normal list:\n%s", paper, text)
		}
		if strings.Index(text, "TKT1202610050001") > strings.Index(text, "TKT1202610050002") {
			t.Errorf("%dmm: not oldest first:\n%s", paper, text)
		}

		text = fits(t, SaleList(model.Settings{}, r, sales, true, nil, paper, reportPrinted), Columns(paper))
		contains(t, text, "NOTA DIHAPUS", "Pak Admin", "10:30:00", "salah jumlah tiket", "TKT1202610050003")
	}
	if sales[0].ID != 2 {
		t.Error("the caller's list was reordered")
	}

	contains(t, Text(SaleList(model.Settings{}, r, nil, false, nil, 58, reportPrinted)), "Tidak ada transaksi.", "Jumlah nota")
	contains(t, Text(SaleList(model.Settings{}, r, nil, true, nil, 58, reportPrinted)), "Tidak ada nota dihapus.")
}
