package receipt

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tiket/internal/model"
	"tiket/internal/money"
)

const (
	SizeNormal = ""
	SizeTall   = "tall"
	SizeDouble = "double"
)

type Line struct {
	Text string `json:"text"`
	Bold bool   `json:"bold"`
	Size string `json:"size"`
	Logo bool   `json:"logo"`
}

func Columns(paperWidth int) int {
	if paperWidth == 80 {
		return 48
	}
	return 32
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteByte(' ')
		case r >= 32 && r <= 126:
			b.WriteRune(r)
		case r < 32:
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func wrap(s string, width int) []string {
	var lines []string
	for _, para := range strings.Split(sanitize(strings.ReplaceAll(s, "\r", "")), "\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		line := ""
		for _, word := range strings.Fields(para) {
			for len(word) > width {
				if line != "" {
					lines = append(lines, line)
					line = ""
				}
				lines = append(lines, word[:width])
				word = word[width:]
			}
			switch {
			case line == "":
				line = word
			case len(line)+1+len(word) <= width:
				line += " " + word
			default:
				lines = append(lines, line)
				line = word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func center(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", (width-len(s))/2) + s
}

func leftRight(left, right string, width int) string {
	left, right = sanitize(left), sanitize(right)
	gap := width - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func header(st model.Settings, width int) []Line {
	var lines []Line
	if st.HasLogo {
		lines = append(lines, Line{Logo: true})
	} else {
		name := st.BusinessName
		if strings.TrimSpace(name) == "" {
			name = "TIKET"
		}
		for _, l := range wrap(name, width/2) {
			lines = append(lines, Line{Text: center(l, width/2), Bold: true, Size: SizeDouble})
		}
	}
	for _, l := range wrap(st.Address, width) {
		lines = append(lines, Line{Text: center(l, width)})
	}
	return lines
}

func footer(st model.Settings, width int) []Line {
	var lines []Line
	for _, l := range wrap(st.Footer, width) {
		lines = append(lines, Line{Text: center(l, width)})
	}
	return lines
}

func parseTime(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	return t, err == nil
}

func formatTime(s string) string {
	if t, ok := parseTime(s); ok {
		return t.Format("02-01-2006 15:04")
	}
	return s
}

func Purpose(st model.Settings, createdAt string) string {
	t, ok := parseTime(createdAt)
	if ok && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
		return st.PurposeWeekend
	}
	return st.PurposeWeekday
}

func field(label, value string) Line {
	return Line{Text: sanitize(fmt.Sprintf("%-9s: %s", label, value))}
}

func total(label, amount string, width int) string {
	const amountCols = 12
	left := label + " :"
	pad := width - amountCols - len(left)
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + left + fmt.Sprintf("%*s", amountCols, amount)
}

func Build(st model.Settings, sale model.Sale, paperWidth int) []Line {
	w := Columns(paperWidth)
	dash := Line{Text: strings.Repeat("-", w)}
	lines := header(st, w)
	lines = append(lines, dash)
	if sale.PrintCount > 0 {
		lines = append(lines, Line{Text: center(fmt.Sprintf("Reprint %d", sale.PrintCount), w/2), Size: SizeDouble})
	}
	date := formatTime(sale.CreatedAt)
	lines = append(lines,
		field("No", sale.InvoiceNo),
		field("Date", date),
		field("Time In", date),
		field("Purpose", Purpose(st, sale.CreatedAt)),
		field("Cashier", sale.CashierName),
		dash,
	)
	var qty int64
	for _, it := range sale.Items {
		qty += it.Qty
		for _, l := range wrap(it.TicketName, w) {
			lines = append(lines, Line{Text: l})
		}
		left := fmt.Sprintf("%-5s @%s", strconv.FormatInt(it.Qty, 10)+"x", money.Format(it.Price))
		lines = append(lines, Line{Text: leftRight(left, money.Format(it.Subtotal), w)})
	}
	lines = append(lines,
		dash,
		Line{Text: fmt.Sprintf("%d items", qty)},
		Line{Text: total("Grand Total", money.Format(sale.Total), w), Size: SizeTall},
		Line{Text: total("CASH", money.Format(sale.Paid), w)},
		Line{Text: strings.Repeat(" ", w/3) + strings.Repeat("-", w-w/3)},
		Line{Text: total("Change", money.Format(sale.ChangeAmount), w)},
		Line{},
	)
	lines = append(lines, footer(st, w)...)
	return append(lines, dash)
}

func BuildTest(st model.Settings, paperWidth int, printerName string) []Line {
	w := Columns(paperWidth)
	lines := header(st, w)
	lines = append(lines,
		Line{Text: strings.Repeat("-", w)},
		Line{Text: center("TES PRINTER", w/2), Size: SizeDouble},
		Line{Text: sanitize("Printer : " + printerName)},
		Line{Text: sanitize("Kertas  : " + strconv.Itoa(paperWidth) + " mm (" + strconv.Itoa(w) + " kolom)")},
		Line{Text: strings.Repeat("1234567890", w/10+1)[:w]},
		Line{Text: total("Grand Total", "70.000", w), Size: SizeTall},
		Line{Text: strings.Repeat("-", w)},
	)
	return lines
}

func Text(lines []Line) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		if l.Logo {
			parts[i] = "[LOGO]"
		} else {
			parts[i] = l.Text
		}
	}
	return strings.Join(parts, "\n")
}

var (
	escInit      = []byte{0x1B, 0x40}
	escBoldOn    = []byte{0x1B, 0x45, 0x01}
	escBoldOff   = []byte{0x1B, 0x45, 0x00}
	escCenter    = []byte{0x1B, 0x61, 0x01}
	escLeft      = []byte{0x1B, 0x61, 0x00}
	escSizeTall  = []byte{0x1D, 0x21, 0x01}
	escSizeDbl   = []byte{0x1D, 0x21, 0x11}
	escSizeReset = []byte{0x1D, 0x21, 0x00}
	escFeed      = []byte{0x1B, 0x64, 0x04}
	escCut       = []byte{0x1D, 0x56, 0x42, 0x00}
)

func ESCPOS(lines []Line, logo *Bitmap) []byte {
	out := append([]byte{}, escInit...)
	for _, l := range lines {
		if l.Logo {
			if logo != nil {
				out = append(out, escCenter...)
				out = append(out, logo.Raster()...)
				out = append(out, escLeft...)
			}
			continue
		}
		switch l.Size {
		case SizeTall:
			out = append(out, escSizeTall...)
		case SizeDouble:
			out = append(out, escSizeDbl...)
		}
		if l.Bold {
			out = append(out, escBoldOn...)
		}
		out = append(out, sanitize(l.Text)...)
		if l.Bold {
			out = append(out, escBoldOff...)
		}
		if l.Size != SizeNormal {
			out = append(out, escSizeReset...)
		}
		out = append(out, '\n')
	}
	out = append(out, escFeed...)
	return append(out, escCut...)
}
