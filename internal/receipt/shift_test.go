package receipt

import (
	"strings"
	"testing"
	"time"

	"tiket/internal/model"
)

func fits(t *testing.T, lines []Line, width int) string {
	t.Helper()
	for _, l := range lines {
		if len(l.Text) > width {
			t.Errorf("line wider than %d: %q", width, l.Text)
		}
	}
	return Text(lines)
}

func TestShiftClosing(t *testing.T) {
	prev := int64(151000000)
	closed := "2026-10-24 06:05:00"
	c := model.ShiftClosing{
		Shift: model.Shift{
			Name: "Malam", StartTime: "22:00", EndTime: "06:00", BusinessDate: "2026-10-23", CashierName: "Puput",
			CounterName: "TICKETING 4", OpeningCash: 50000000, OpenedAt: "2026-10-23 21:55:10", ClosedAt: &closed,
		},
		Sales: model.ShiftSales{
			Transactions: 32, TicketQty: 40, TicketTotal: 86000000, TopupQty: 10, TopupTotal: 20000000, Total: 106000000,
			FirstSaleAt: "2026-10-23 22:01:04", LastSaleAt: "2026-10-24 05:42:01", DeletedCount: 1, DeletedTotal: 2000000,
		},
		Revision: 1, ExpectedCash: 156000000, ActualCash: 155000000, Difference: -1000000,
		Note: "salah hitung uang lima puluh ribu di laci kedua", ClosedByName: "Pak Admin", ClosedAt: "2026-10-24 10:02:00",
		PrintCount: 2, PreviousActual: &prev,
		Deleted: []model.DeletedSale{{InvoiceNo: "TKT1202610230017", Total: 2000000, DeletedAt: "2026-10-23 23:15:40",
			DeletedByName: "Pak Admin", Reason: "salah jumlah tiket", ReplacedByInvoiceNo: "TKT1202610230018"}},
	}
	printed := time.Date(2026, 10, 24, 10, 41, 46, 0, time.Local)
	for _, paper := range []int{58, 80} {
		text := fits(t, ShiftClosing(model.Settings{BusinessName: "Pantai"}, c, paper, printed), Columns(paper))
		for _, want := range []string{"LAPORAN SHIFT", "KOREKSI KE-1", "CETAK ULANG 2", "Malam 22:00-06:00", "23-10-2026",
			"Puput", "TICKETING 4", "23-10-2026 21:55:10", "24-10-2026 06:05:00", "22:01:04", "05:42:01",
			"24-10-2026 10:41:46", "1.060.000", "Nota dihapus (1)", "Modal awal", "500.000", "Kas seharusnya",
			"1.560.000", "1.550.000", "-10.000", "1.510.000", "salah hitung", "Dikoreksi : 24-10-2026 10:02:00",
			"Oleh      : Pak Admin", "NOTA DIHAPUS (1)", "TKT1202610230017", "Pak Admin 23:15:40", "salah jumlah tiket",
			"Diganti   : TKT1202610230018", "SELESAI"} {
			if !strings.Contains(text, want) {
				t.Errorf("%dmm: missing %q in\n%s", paper, want, text)
			}
		}
	}

	// A first closing, previewed before it is saved: the shift is still open.
	c.Shift.ClosedAt = nil
	c.Revision, c.PrintCount, c.Note, c.ActualCash, c.Difference, c.Sales.DeletedCount, c.Deleted = 0, 0, "", 156500000, 500000, 0, nil
	text := Text(ShiftClosing(model.Settings{}, c, 58, printed))
	for _, unwanted := range []string{"KOREKSI", "CETAK ULANG", "Nota dihapus", "sebelumnya", "Keterangan", "Dikoreksi"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("first closing shows %q", unwanted)
		}
	}
	for _, want := range []string{"+5.000", "Ditutup   : 24-10-2026 10:02:00"} {
		if !strings.Contains(text, want) {
			t.Errorf("first closing: missing %q in\n%s", want, text)
		}
	}
}

func TestShiftDay(t *testing.T) {
	closing := model.ShiftClosing{Sales: model.ShiftSales{Transactions: 2, Total: 7000000, DeletedCount: 1, DeletedTotal: 3000000}, ExpectedCash: 12000000,
		ActualCash: 11900000, Difference: -100000, Revision: 1}
	day := model.ShiftDay{Date: "2026-10-05", Shifts: []model.ShiftReport{
		{Shift: model.Shift{Name: "Siang", StartTime: "06:00", EndTime: "14:00", CashierName: "Budi", CounterName: "LOKET 1", OpeningCash: 5000000},
			Sales: model.ShiftSales{Transactions: 1, Total: 5000000}, ExpectedCash: 10000000, Closing: &closing, Changed: true},
		{Shift: model.Shift{Name: "Sore", StartTime: "14:00", EndTime: "22:00", CashierName: "Siti", CounterName: "LOKET 1", OpeningCash: 2000000},
			Sales: model.ShiftSales{Transactions: 1, Total: 5000000}, ExpectedCash: 7000000},
	}}
	text := fits(t, ShiftDay(model.Settings{}, day, 58, time.Now()), 32)
	for _, want := range []string{"RINGKASAN SHIFT", "05-10-2026", "Siang - Budi", "LOKET 1, 06:00-14:00", "120.000", "119.000",
		"-1.000", "Nota dihapus (1)", "30.000", "Dikoreksi 1x", "Penjualan sekarang", "Sore - Siti", "BELUM DITUTUP", "70.000", "190.000", "1 shift"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	if text := Text(ShiftDay(model.Settings{}, model.ShiftDay{Date: "2026-10-05"}, 80, time.Now())); !strings.Contains(text, "Tidak ada shift.") {
		t.Errorf("empty day:\n%s", text)
	}
}
