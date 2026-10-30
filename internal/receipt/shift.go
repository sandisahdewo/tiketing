package receipt

import (
	"fmt"
	"strings"
	"time"

	"tiket/internal/model"
	"tiket/internal/money"
)

func formatDate(s string) string {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("02-01-2006")
	}
	return s
}

// clock is the time of day of a stored time, "-" for none.
func clock(s string) string {
	if t, ok := parseTime(s); ok {
		return t.Format("15:04:05")
	}
	return "-"
}

// stamp is a stored time as date and time, "-" for none.
func stamp(s string) string {
	if t, ok := parseTime(s); ok {
		return t.Format("02-01-2006 15:04:05")
	}
	return "-"
}

// labelled is "Label     : value", the value wrapped under itself.
func labelled(label, value string, width int) []Line {
	const labelCols = 10
	prefix := fmt.Sprintf("%-*s: ", labelCols, label)
	var lines []Line
	for i, l := range wrap(value, width-len(prefix)) {
		if i > 0 {
			prefix = strings.Repeat(" ", len(prefix))
		}
		lines = append(lines, Line{Text: sanitize(prefix + l)})
	}
	if len(lines) == 0 {
		lines = append(lines, Line{Text: sanitize(prefix + "-")})
	}
	return lines
}

func amount(label string, sen int64, width int) Line {
	return Line{Text: leftRight(label, money.Format(sen), width)}
}

func signed(sen int64) string {
	if sen > 0 {
		return "+" + money.Format(sen)
	}
	return money.Format(sen)
}

func banner(word string, width int) Line {
	side := (width - len(word) - 2) / 2
	if side < 1 {
		return Line{Text: word}
	}
	s := strings.Repeat("*", side) + " " + word + " " + strings.Repeat("*", side)
	return Line{Text: s + strings.Repeat("*", width-len(s))}
}

// hours is a shift's name with its hours, e.g. "Malam 22:00-06:00".
func hours(h model.Shift) string {
	return h.Name + " " + h.StartTime + "-" + h.EndTime
}

// ShiftClosing is the printed report of one closed shift: its sales as they
// were at closing time and the cash counted against the opening cash.
func ShiftClosing(st model.Settings, c model.ShiftClosing, paperWidth int, printedAt time.Time) []Line {
	w := Columns(paperWidth)
	double := Line{Text: strings.Repeat("=", w)}
	dash := Line{Text: strings.Repeat("-", w)}
	h, d := c.Shift, c.Sales
	// A correction keeps the time the shift itself was closed.
	closedAt := c.ClosedAt
	if h.ClosedAt != nil {
		closedAt = *h.ClosedAt
	}

	lines := []Line{banner("MULAI", w)}
	lines = append(lines, header(st, w)...)
	lines = append(lines, Line{Text: center("LAPORAN SHIFT", w), Bold: true})
	if c.Revision > 0 {
		lines = append(lines, Line{Text: center(fmt.Sprintf("KOREKSI KE-%d", c.Revision), w), Bold: true})
	}
	if c.PrintCount > 0 {
		lines = append(lines, Line{Text: center(fmt.Sprintf("CETAK ULANG %d", c.PrintCount), w)})
	}
	lines = append(lines, labelled("Shift", hours(h), w)...)
	lines = append(lines, labelled("Tanggal", formatDate(h.BusinessDate), w)...)
	lines = append(lines, labelled("Kasir", h.CashierName, w)...)
	lines = append(lines, labelled("Loket", h.CounterName, w)...)
	lines = append(lines, labelled("Dibuka", stamp(h.OpenedAt), w)...)
	lines = append(lines, labelled("Ditutup", stamp(closedAt), w)...)
	lines = append(lines, labelled("Trx awal", clock(d.FirstSaleAt), w)...)
	lines = append(lines, labelled("Trx akhir", clock(d.LastSaleAt), w)...)
	lines = append(lines, labelled("Dicetak", printedAt.Format("02-01-2006 15:04:05"), w)...)

	lines = append(lines,
		double,
		Line{Text: "RINGKASAN", Bold: true},
		Line{Text: leftRight("Jumlah transaksi", fmt.Sprint(d.Transactions), w)},
		amount(fmt.Sprintf("Tiket (%d)", d.TicketQty), d.TicketTotal, w),
		amount(fmt.Sprintf("Topup (%d)", d.TopupQty), d.TopupTotal, w),
		dash,
		Line{Text: leftRight("Total penjualan", money.Format(d.Total), w), Bold: true},
	)
	if d.DeletedCount > 0 {
		lines = append(lines, amount(fmt.Sprintf("Nota dihapus (%d)", d.DeletedCount), d.DeletedTotal, w))
	}
	// Every deleted sale is listed, so it is looked at when the shift closes.
	if len(c.Deleted) > 0 {
		lines = append(lines, double, Line{Text: fmt.Sprintf("NOTA DIHAPUS (%d)", len(c.Deleted)), Bold: true})
		for i, x := range c.Deleted {
			if i > 0 {
				lines = append(lines, dash)
			}
			lines = append(lines, Line{Text: leftRight(x.InvoiceNo, money.Format(x.Total), w)})
			lines = append(lines, labelled("Oleh", x.DeletedByName+" "+clock(x.DeletedAt), w)...)
			lines = append(lines, labelled("Alasan", x.Reason, w)...)
			if x.ReplacedByInvoiceNo != "" {
				lines = append(lines, labelled("Diganti", x.ReplacedByInvoiceNo, w)...)
			}
		}
	}

	lines = append(lines,
		double,
		Line{Text: "KAS", Bold: true},
		amount("Modal awal", h.OpeningCash, w),
		amount("Penjualan", d.Total, w),
		dash,
		Line{Text: leftRight("Kas seharusnya", money.Format(c.ExpectedCash), w), Bold: true},
		amount("Kas aktual", c.ActualCash, w),
		dash,
		Line{Text: leftRight("Selisih", signed(c.Difference), w), Bold: true},
		dash,
	)
	if c.Revision > 0 && c.PreviousActual != nil {
		lines = append(lines, amount("Kas aktual sebelumnya", *c.PreviousActual, w))
	}
	if c.Note != "" {
		lines = append(lines, labelled("Keterangan", c.Note, w)...)
	}
	if c.Revision > 0 {
		lines = append(lines, labelled("Dikoreksi", stamp(c.ClosedAt), w)...)
	}
	lines = append(lines, labelled("Oleh", c.ClosedByName, w)...)
	return append(lines, banner("SELESAI", w))
}

// ShiftDay is the printed summary of every shift of one business date, for
// the admin.
func ShiftDay(st model.Settings, day model.ShiftDay, paperWidth int, printedAt time.Time) []Line {
	w := Columns(paperWidth)
	double := Line{Text: strings.Repeat("=", w)}
	dash := Line{Text: strings.Repeat("-", w)}

	lines := []Line{banner("MULAI", w)}
	lines = append(lines, header(st, w)...)
	lines = append(lines, Line{Text: center("RINGKASAN SHIFT", w), Bold: true})
	lines = append(lines, labelled("Tanggal", formatDate(day.Date), w)...)
	lines = append(lines, labelled("Dicetak", printedAt.Format("02-01-2006 15:04:05"), w)...)
	lines = append(lines, double)

	var transactions, opening, sales, expected, actual, difference int64
	open := 0
	for _, r := range day.Shifts {
		h := r.Shift
		lines = append(lines,
			Line{Text: sanitize(h.Name + " - " + h.CashierName), Bold: true},
			Line{Text: sanitize(h.CounterName + ", " + h.StartTime + "-" + h.EndTime)},
			amount("Modal awal", h.OpeningCash, w),
		)
		opening += h.OpeningCash
		// A closed shift counts as it was closed, like its own report.
		if r.Closing == nil {
			lines = append(lines, amount(fmt.Sprintf("Penjualan (%d trx)", r.Sales.Transactions), r.Sales.Total, w))
			if r.Sales.DeletedCount > 0 {
				lines = append(lines, amount(fmt.Sprintf("Nota dihapus (%d)", r.Sales.DeletedCount), r.Sales.DeletedTotal, w))
			}
			lines = append(lines,
				amount("Kas seharusnya", r.ExpectedCash, w),
				Line{Text: leftRight("Kas aktual", "BELUM DITUTUP", w)},
			)
			transactions += r.Sales.Transactions
			sales += r.Sales.Total
			expected += r.ExpectedCash
			open++
		} else {
			k := r.Closing
			lines = append(lines, amount(fmt.Sprintf("Penjualan (%d trx)", k.Sales.Transactions), k.Sales.Total, w))
			if k.Sales.DeletedCount > 0 {
				lines = append(lines, amount(fmt.Sprintf("Nota dihapus (%d)", k.Sales.DeletedCount), k.Sales.DeletedTotal, w))
			}
			lines = append(lines,
				amount("Kas seharusnya", k.ExpectedCash, w),
				amount("Kas aktual", k.ActualCash, w),
				Line{Text: leftRight("Selisih", signed(k.Difference), w)},
			)
			if k.Revision > 0 {
				lines = append(lines, Line{Text: fmt.Sprintf("Dikoreksi %dx", k.Revision)})
			}
			if r.Changed {
				lines = append(lines, Line{Text: leftRight("Penjualan sekarang", money.Format(r.Sales.Total), w)})
			}
			transactions += k.Sales.Transactions
			sales += k.Sales.Total
			expected += k.ExpectedCash
			actual += k.ActualCash
			difference += k.Difference
		}
		lines = append(lines, dash)
	}
	if len(day.Shifts) == 0 {
		lines = append(lines, Line{Text: "Tidak ada shift."}, dash)
	}

	lines = append(lines,
		Line{Text: "TOTAL", Bold: true},
		Line{Text: leftRight("Jumlah transaksi", fmt.Sprint(transactions), w)},
		amount("Modal awal", opening, w),
		amount("Total penjualan", sales, w),
		amount("Kas seharusnya", expected, w),
		amount("Kas aktual", actual, w),
		Line{Text: leftRight("Selisih", signed(difference), w), Bold: true},
	)
	if open > 0 {
		lines = append(lines, Line{Text: leftRight("Belum ditutup", fmt.Sprintf("%d shift", open), w)})
	}
	return append(lines, double, banner("SELESAI", w))
}
