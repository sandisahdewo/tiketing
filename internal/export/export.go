package export

import (
	"fmt"
	"strings"
	"time"

	"tiket/internal/model"
)

// PDFMaxSales is the most transactions listed in a PDF; above it the PDF only
// carries the summaries and points to the Excel export for the full list.
const PDFMaxSales = 1000

// What an export holds: the whole report, or one tab of the report page.
const (
	KindAll    = "all"
	KindPeriod = "period"
	KindItems  = "items"
	KindSales  = "sales"
)

// Data is everything a report export needs. SalesCount is the number of
// transactions in the range; Sales may be left empty when it exceeds
// PDFMaxSales and only a PDF is made, or for a kind without a sale list.
type Data struct {
	Kind string
	// Deleted makes the sale list that of the deleted sales.
	Deleted    bool
	Settings   model.Settings
	Report     model.Report
	Sales      []model.Sale
	SalesCount int64
	// Filter describes the active report filters, e.g. "Kasir: Budi"; empty for none.
	Filter    string
	PrintedBy string
	PrintedAt time.Time
}

var (
	periodName = map[string]string{"daily": "Harian", "weekly": "Mingguan", "monthly": "Bulanan"}
	periodTab  = map[string]string{"daily": "Per Hari", "weekly": "Per Minggu", "monthly": "Per Bulan"}
)

// includes says whether the export holds the given part of the report.
func (d Data) includes(kind string) bool {
	return d.Kind == KindAll || d.Kind == kind
}

// tabName is the tab an export holds, "" for the whole report.
func (d Data) tabName() string {
	switch d.Kind {
	case KindPeriod:
		return periodTab[d.Report.Period]
	case KindItems:
		return "Per Item"
	case KindSales:
		if d.Deleted {
			return "Nota Dihapus"
		}
		return "Transaksi"
	}
	return ""
}

// FileName returns a default file name such as "Laporan 2026-10-01 sd 2026-10-31.xlsx"
// or, for one tab, "Laporan Per Item 2026-10-01.pdf".
func FileName(d Data, ext string) string {
	name := "Laporan"
	if tab := d.tabName(); tab != "" {
		name += " " + tab
	}
	r := d.Report
	if r.From == r.To {
		return fmt.Sprintf("%s %s.%s", name, r.From, ext)
	}
	return fmt.Sprintf("%s %s sd %s.%s", name, r.From, r.To, ext)
}

// counterName is the sale's counter as it is called now, like the report on
// screen (the receipt keeps the name it was printed with).
func counterName(s model.Sale) string {
	if s.CounterName != "" {
		return s.CounterName
	}
	return s.Terminal
}

// shiftName is the shift a sale was made in, "-" for sales from before shifts.
func shiftName(s model.Sale) string {
	if s.ShiftName != "" {
		return s.ShiftName
	}
	return "-"
}

func title(d Data) string {
	heading := "Laporan Penjualan"
	switch d.Kind {
	case KindPeriod, KindItems:
		heading = "Penjualan " + d.tabName()
	case KindSales:
		heading = "Daftar Transaksi"
		if d.Deleted {
			heading = "Nota Dihapus"
		}
	}
	if name := strings.TrimSpace(d.Settings.BusinessName); name != "" {
		return heading + " " + name
	}
	return heading
}

func formatDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02-01-2006")
}

func formatDateTime(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return s
	}
	return t.Format("02-01-2006 15:04")
}

func rangeText(r model.Report) string {
	if r.From == r.To {
		return formatDate(r.From)
	}
	return formatDate(r.From) + " s/d " + formatDate(r.To)
}

func printedText(d Data) string {
	s := "Dicetak: " + d.PrintedAt.Format("02-01-2006 15:04")
	if d.PrintedBy != "" {
		s += " oleh " + d.PrintedBy
	}
	return s
}
