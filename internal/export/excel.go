package export

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"

	"tiket/internal/model"
)

type sheetWriter struct {
	f      *excelize.File
	sheet  string
	row    int
	styles styles
}

type styles struct {
	title, bold, header, plain, money, moneyBold, number, numberBold, labelBold int
}

func newStyles(f *excelize.File, moneyFormat string) (styles, error) {
	var s styles
	border := []excelize.Border{
		{Type: "left", Color: "BFBFBF", Style: 1},
		{Type: "right", Color: "BFBFBF", Style: 1},
		{Type: "top", Color: "BFBFBF", Style: 1},
		{Type: "bottom", Color: "BFBFBF", Style: 1},
	}
	defs := []struct {
		dst   *int
		style excelize.Style
	}{
		{&s.title, excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}}},
		{&s.bold, excelize.Style{Font: &excelize.Font{Bold: true}}},
		{&s.header, excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F6FA8"}},
			Border:    border,
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		}},
		{&s.plain, excelize.Style{Border: border}},
		{&s.money, excelize.Style{CustomNumFmt: &moneyFormat, Border: border}},
		{&s.moneyBold, excelize.Style{CustomNumFmt: &moneyFormat, Border: border, Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EEF6FC"}}}},
		{&s.number, excelize.Style{NumFmt: 3, Border: border}},
		{&s.numberBold, excelize.Style{NumFmt: 3, Border: border, Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EEF6FC"}}}},
		{&s.labelBold, excelize.Style{Border: border, Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EEF6FC"}}}},
	}
	for _, d := range defs {
		id, err := f.NewStyle(&d.style)
		if err != nil {
			return s, err
		}
		*d.dst = id
	}
	return s, nil
}

// cell kinds used by writeRow
type (
	text   string
	num    int64
	rupiah int64
)

func (w *sheetWriter) cell(col int) string {
	name, _ := excelize.CoordinatesToCellName(col, w.row)
	return name
}

func (w *sheetWriter) line(style int, values ...any) {
	for i, v := range values {
		w.f.SetCellValue(w.sheet, w.cell(i+1), v)
		if style != 0 {
			w.f.SetCellStyle(w.sheet, w.cell(i+1), w.cell(i+1), style)
		}
	}
	w.row++
}

func (w *sheetWriter) header(cols ...string) {
	values := make([]any, len(cols))
	for i, c := range cols {
		values[i] = c
	}
	w.line(w.styles.header, values...)
}

func (w *sheetWriter) data(bold bool, values ...any) {
	for i, v := range values {
		ref := w.cell(i + 1)
		var style int
		switch v := v.(type) {
		case text:
			w.f.SetCellStr(w.sheet, ref, string(v))
			style = w.styles.plain
			if bold {
				style = w.styles.labelBold
			}
		case num:
			w.f.SetCellInt(w.sheet, ref, int64(v))
			style = w.styles.number
			if bold {
				style = w.styles.numberBold
			}
		case rupiah:
			w.f.SetCellFloat(w.sheet, ref, float64(v)/100, 2, 64)
			style = w.styles.money
			if bold {
				style = w.styles.moneyBold
			}
		}
		w.f.SetCellStyle(w.sheet, ref, ref, style)
	}
	w.row++
}

func (w *sheetWriter) heading(d Data, subtitle string) {
	w.line(w.styles.title, title(d))
	w.line(0, "Periode: "+rangeText(d.Report))
	if d.Filter != "" {
		w.line(0, "Filter: "+d.Filter)
	}
	w.line(0, printedText(d))
	if subtitle != "" {
		w.line(w.styles.bold, subtitle)
	}
	w.row++
}

func hasCents(d Data) bool {
	vals := []int64{d.Report.Total}
	for _, r := range d.Report.Rows {
		vals = append(vals, r.TicketTotal, r.TopupTotal, r.Total)
	}
	for _, t := range d.Report.Tickets {
		vals = append(vals, t.Total)
	}
	for _, s := range d.Sales {
		vals = append(vals, s.Total, s.Paid, s.ChangeAmount)
	}
	for _, v := range vals {
		if v%100 != 0 {
			return true
		}
	}
	return false
}

// Excel renders the report as an .xlsx workbook with three sheets (summary per
// period, per item, and the transaction list), or one tab of it as one sheet.
func Excel(d Data) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	moneyFormat := "#,##0"
	if hasCents(d) {
		moneyFormat = "#,##0.00"
	}
	st, err := newStyles(f, moneyFormat)
	if err != nil {
		return nil, err
	}
	r := d.Report

	first := true
	sheet := func(name string) *sheetWriter {
		if first {
			f.SetSheetName("Sheet1", name)
			first = false
		} else {
			f.NewSheet(name)
		}
		return &sheetWriter{f: f, sheet: name, row: 1, styles: st}
	}

	if d.includes(KindPeriod) {
		name := "Ringkasan"
		if d.Kind != KindAll {
			name = d.tabName()
		}
		w := sheet(name)
		w.heading(d, "")
		if d.Kind == KindAll {
			w.header("Ringkasan", "Jumlah", "Pendapatan")
			w.data(false, text("Jumlah transaksi"), num(r.Transactions), text(""))
			w.data(false, text("Tiket"), num(r.TicketQty), rupiah(r.TicketTotal))
			w.data(false, text("Topup Card"), num(r.TopupQty), rupiah(r.TopupTotal))
			w.data(true, text("Total"), num(r.Qty), rupiah(r.Total))
			w.row++
			w.line(w.styles.bold, "Per periode ("+periodName[r.Period]+")")
		}
		w.header("Periode", "Transaksi", "Tiket", "Rp Tiket", "Topup", "Rp Topup", "Total")
		for _, row := range r.Rows {
			w.data(false, text(model.PeriodLabel(row.Period, r.Period)), num(row.Transactions), num(row.TicketQty), rupiah(row.TicketTotal), num(row.TopupQty), rupiah(row.TopupTotal), rupiah(row.Total))
		}
		w.data(true, text("Total"), num(r.Transactions), num(r.TicketQty), rupiah(r.TicketTotal), num(r.TopupQty), rupiah(r.TopupTotal), rupiah(r.Total))
		f.SetColWidth(name, "A", "A", 28)
		f.SetColWidth(name, "B", "G", 15)
	}

	if d.includes(KindItems) {
		const items = "Per Item"
		w := sheet(items)
		w.heading(d, "Penjualan per item")
		w.header("Kategori", "Item", "Terjual", "Total")
		for _, g := range r.ItemGroups() {
			for _, it := range g.Items {
				w.data(false, text(g.Label), text(it.TicketName), num(it.Qty), rupiah(it.Total))
			}
			w.data(true, text("Subtotal "+g.Label), text(""), num(g.Qty), rupiah(g.Total))
		}
		w.data(true, text("Total"), text(""), num(r.Qty), rupiah(r.Total))
		f.SetColWidth(items, "A", "A", 22)
		f.SetColWidth(items, "B", "B", 32)
		f.SetColWidth(items, "C", "D", 15)
	}

	if d.includes(KindSales) {
		var total int64
		if d.Deleted {
			const deleted = "Nota Dihapus"
			w := sheet(deleted)
			w.heading(d, fmt.Sprintf("Nota dihapus (%d)", len(d.Sales)))
			w.header("Waktu", "No. Nota", "Kasir", "Loket", "Shift", "Total", "Dihapus", "Oleh", "Alasan", "Diganti")
			for _, s := range d.Sales {
				at := ""
				if s.DeletedAt != nil {
					at = formatDateTime(*s.DeletedAt)
				}
				w.data(false, text(formatDateTime(s.CreatedAt)), text(s.InvoiceNo), text(s.CashierName), text(counterName(s)), text(shiftName(s)), rupiah(s.Total), text(at), text(s.DeletedByName), text(s.DeleteReason), text(s.ReplacedByInvoiceNo))
				total += s.Total
			}
			w.data(true, text("Total"), text(""), text(""), text(""), text(""), rupiah(total), text(""), text(""), text(""), text(""))
			f.SetColWidth(deleted, "A", "A", 18)
			f.SetColWidth(deleted, "B", "B", 22)
			f.SetColWidth(deleted, "C", "E", 16)
			f.SetColWidth(deleted, "F", "F", 14)
			f.SetColWidth(deleted, "G", "G", 18)
			f.SetColWidth(deleted, "H", "H", 16)
			f.SetColWidth(deleted, "I", "I", 32)
			f.SetColWidth(deleted, "J", "J", 22)
		} else {
			const sales = "Transaksi"
			w := sheet(sales)
			w.heading(d, fmt.Sprintf("Daftar transaksi (%d)", len(d.Sales)))
			w.header("Waktu", "No. Nota", "Kasir", "Loket", "Shift", "Total", "Bayar", "Kembali", "Cetak")
			for _, s := range d.Sales {
				w.data(false, text(formatDateTime(s.CreatedAt)), text(s.InvoiceNo), text(s.CashierName), text(counterName(s)), text(shiftName(s)), rupiah(s.Total), rupiah(s.Paid), rupiah(s.ChangeAmount), num(s.PrintCount))
				total += s.Total
			}
			w.data(true, text("Total"), text(""), text(""), text(""), text(""), rupiah(total), text(""), text(""), text(""))
			f.SetColWidth(sales, "A", "A", 18)
			f.SetColWidth(sales, "B", "B", 22)
			f.SetColWidth(sales, "C", "E", 16)
			f.SetColWidth(sales, "F", "H", 14)
			f.SetColWidth(sales, "I", "I", 8)
		}
	}

	f.SetActiveSheet(0)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
