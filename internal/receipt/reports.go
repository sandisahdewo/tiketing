package receipt

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"tiket/internal/model"
	"tiket/internal/money"
)

// MaxSalesList is the most sales printed in one list: about three and a half
// metres of paper. Longer lists belong in a PDF or Excel export.
const MaxSalesList = 300

// reportHead is the top of a printed report; filter holds the active filters,
// one per line.
func reportHead(st model.Settings, title string, r model.Report, filter []string, w int, printedAt time.Time) []Line {
	period := formatDate(r.From)
	if r.To != r.From {
		period += " s/d " + formatDate(r.To)
	}
	lines := []Line{banner("MULAI", w)}
	lines = append(lines, header(st, w)...)
	lines = append(lines, Line{Text: center(title, w), Bold: true})
	lines = append(lines, labelled("Tanggal", period, w)...)
	lines = append(lines, labelled("Dicetak", printedAt.Format("02-01-2006 15:04:05"), w)...)
	for _, f := range filter {
		for _, l := range wrap(f, w) {
			lines = append(lines, Line{Text: l})
		}
	}
	return append(lines, Line{Text: strings.Repeat("=", w)})
}

// ItemSales is the printed report of what each item sold in a report's range,
// by category.
func ItemSales(st model.Settings, r model.Report, filter []string, paperWidth int, printedAt time.Time) []Line {
	w := Columns(paperWidth)
	double := Line{Text: strings.Repeat("=", w)}
	dash := Line{Text: strings.Repeat("-", w)}
	qtyValue := func(qty, value int64, bold bool) []Line {
		return []Line{
			{Text: leftRight("  Qty", fmt.Sprint(qty), w), Bold: bold},
			{Text: leftRight("  Nilai", money.Format(value), w), Bold: bold},
		}
	}

	lines := reportHead(st, "PENJUALAN PER ITEM", r, filter, w, printedAt)
	groups := r.ItemGroups()
	if len(groups) == 0 {
		lines = append(lines, Line{Text: "Tidak ada penjualan."}, double)
	}
	for _, g := range groups {
		lines = append(lines, Line{Text: strings.ToUpper(g.Label), Bold: true})
		// By name, so an item is in the same place every day.
		slices.SortStableFunc(g.Items, func(a, b model.ReportTicketRow) int {
			return cmp.Compare(strings.ToLower(a.TicketName), strings.ToLower(b.TicketName))
		})
		for _, it := range g.Items {
			prefix := "- "
			for _, l := range wrap(it.TicketName, w-len(prefix)) {
				lines = append(lines, Line{Text: prefix + l})
				prefix = "  "
			}
			lines = append(lines, qtyValue(it.Qty, it.Total, false)...)
		}
		lines = append(lines, dash, Line{Text: "Total " + g.Label, Bold: true})
		lines = append(lines, qtyValue(g.Qty, g.Total, true)...)
		lines = append(lines, double)
	}

	lines = append(lines, Line{Text: "TOTAL", Bold: true})
	lines = append(lines, qtyValue(r.Qty, r.Total, true)...)
	return append(lines, banner("SELESAI", w))
}

var periodTitle = map[string]string{
	"daily":   "PENJUALAN PER HARI",
	"weekly":  "PENJUALAN PER MINGGU",
	"monthly": "PENJUALAN PER BULAN",
}

// PeriodSales is the printed report of a report's days, weeks or months.
func PeriodSales(st model.Settings, r model.Report, filter []string, paperWidth int, printedAt time.Time) []Line {
	w := Columns(paperWidth)
	double := Line{Text: strings.Repeat("=", w)}
	dash := Line{Text: strings.Repeat("-", w)}
	figures := func(transactions, ticketQty, ticketTotal, topupQty, topupTotal, total int64) []Line {
		return []Line{
			{Text: leftRight("Transaksi", fmt.Sprint(transactions), w)},
			amount(fmt.Sprintf("Tiket (%d)", ticketQty), ticketTotal, w),
			amount(fmt.Sprintf("Topup (%d)", topupQty), topupTotal, w),
			{Text: leftRight("Total", money.Format(total), w), Bold: true},
		}
	}

	lines := reportHead(st, periodTitle[r.Period], r, filter, w, printedAt)
	if len(r.Rows) == 0 {
		lines = append(lines, Line{Text: "Tidak ada penjualan."}, double)
	}
	for i, row := range r.Rows {
		if i > 0 {
			lines = append(lines, dash)
		}
		lines = append(lines, Line{Text: model.PeriodLabel(row.Period, r.Period), Bold: true})
		lines = append(lines, figures(row.Transactions, row.TicketQty, row.TicketTotal, row.TopupQty, row.TopupTotal, row.Total)...)
	}
	if len(r.Rows) > 0 {
		lines = append(lines, double)
	}

	lines = append(lines, Line{Text: "TOTAL", Bold: true})
	lines = append(lines, figures(r.Transactions, r.TicketQty, r.TicketTotal, r.TopupQty, r.TopupTotal, r.Total)...)
	return append(lines, banner("SELESAI", w))
}

// SaleList is the printed list of the sales in a report's range, oldest first
// like a journal, whatever order they come in. Deleted sales say why.
func SaleList(st model.Settings, r model.Report, sales []model.Sale, deleted bool, filter []string, paperWidth int, printedAt time.Time) []Line {
	w := Columns(paperWidth)
	double := Line{Text: strings.Repeat("=", w)}
	title, empty := "DAFTAR TRANSAKSI", "Tidak ada transaksi."
	if deleted {
		title, empty = "NOTA DIHAPUS", "Tidak ada nota dihapus."
	}

	sorted := slices.Clone(sales)
	slices.SortStableFunc(sorted, func(a, b model.Sale) int {
		return cmp.Or(cmp.Compare(a.CreatedAt, b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})

	lines := reportHead(st, title, r, filter, w, printedAt)
	if len(sorted) == 0 {
		lines = append(lines, Line{Text: empty})
	}
	var total int64
	for _, s := range sorted {
		total += s.Total
		lines = append(lines, Line{Text: leftRight(s.InvoiceNo, money.Format(s.Total), w), Bold: true})
		for _, l := range wrap(formatTime(s.CreatedAt)+" "+s.CashierName, w) {
			lines = append(lines, Line{Text: l})
		}
		if deleted {
			if s.DeletedAt != nil {
				lines = append(lines, labelled("Oleh", s.DeletedByName+" "+stamp(*s.DeletedAt), w)...)
			}
			lines = append(lines, labelled("Alasan", s.DeleteReason, w)...)
			if s.ReplacedByInvoiceNo != "" {
				lines = append(lines, labelled("Diganti", s.ReplacedByInvoiceNo, w)...)
			}
		}
	}

	return append(lines,
		double,
		Line{Text: leftRight("Jumlah nota", fmt.Sprint(len(sorted)), w)},
		Line{Text: leftRight("Total", money.Format(total), w), Bold: true},
		banner("SELESAI", w),
	)
}
