package export

import (
	"fmt"
	"strings"
	"time"

	"tiket/internal/model"
)

// Data is everything a report export needs.
type Data struct {
	Settings  model.Settings
	Report    model.Report
	Sales     []model.Sale
	PrintedBy string
	PrintedAt time.Time
}

var (
	days       = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	months     = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	periodName = map[string]string{"daily": "Harian", "weekly": "Mingguan", "monthly": "Bulanan"}
)

var categories = []struct{ value, label string }{
	{model.CategoryTicket, "Tiket"},
	{model.CategoryTopup, "Topup Card"},
}

// FileName returns a default file name such as "Laporan 2026-10-01 sd 2026-10-31.xlsx".
func FileName(r model.Report, ext string) string {
	if r.From == r.To {
		return fmt.Sprintf("Laporan %s.%s", r.From, ext)
	}
	return fmt.Sprintf("Laporan %s sd %s.%s", r.From, r.To, ext)
}

func title(d Data) string {
	name := strings.TrimSpace(d.Settings.BusinessName)
	if name == "" {
		return "Laporan Penjualan"
	}
	return "Laporan Penjualan " + name
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

func periodLabel(period, kind string) string {
	if kind == "monthly" {
		t, err := time.Parse("2006-01", period)
		if err != nil {
			return period
		}
		return fmt.Sprintf("%s %d", months[t.Month()-1], t.Year())
	}
	t, err := time.Parse("2006-01-02", period)
	if err != nil {
		return period
	}
	if kind == "weekly" {
		return t.Format("02-01-2006") + " - " + t.AddDate(0, 0, 6).Format("02-01-2006")
	}
	return days[t.Weekday()] + ", " + t.Format("02-01-2006")
}

type itemGroup struct {
	label string
	items []model.ReportTicketRow
	qty   int64
	total int64
}

func itemGroups(r model.Report) []itemGroup {
	var groups []itemGroup
	for _, c := range categories {
		g := itemGroup{label: c.label}
		for _, it := range r.Tickets {
			if it.Category == c.value {
				g.items = append(g.items, it)
				g.qty += it.Qty
				g.total += it.Total
			}
		}
		if len(g.items) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}
