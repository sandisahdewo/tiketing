package export

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/go-pdf/fpdf"

	"tiket/internal/money"
)

const (
	pdfMargin = 15.0
	rowHeight = 7.0
)

type pdfTable struct {
	pdf    *fpdf.Fpdf
	tr     func(string) string
	cols   []string
	widths []float64
	aligns []string
}

func (t *pdfTable) drawHeader() {
	p := t.pdf
	p.SetFont("Helvetica", "B", 9)
	p.SetFillColor(31, 111, 168)
	p.SetTextColor(255, 255, 255)
	for i, c := range t.cols {
		p.CellFormat(t.widths[i], rowHeight, t.tr(c), "1", 0, "C", true, 0, "")
	}
	p.Ln(-1)
	p.SetTextColor(0, 0, 0)
}

func (t *pdfTable) ensureSpace(h float64) {
	p := t.pdf
	_, pageH := p.GetPageSize()
	if p.GetY()+h > pageH-pdfMargin {
		p.AddPage()
		t.drawHeader()
	}
}

func (t *pdfTable) row(bold bool, values ...string) {
	p := t.pdf
	t.ensureSpace(rowHeight)
	style := ""
	if bold {
		style = "B"
		p.SetFillColor(238, 246, 252)
	}
	p.SetFont("Helvetica", style, 9)
	for i, v := range values {
		p.CellFormat(t.widths[i], rowHeight, t.fit(t.tr(v), t.widths[i]-2), "1", 0, t.aligns[i], bold, 0, "")
	}
	p.Ln(-1)
}

// fit shortens s so it does not overflow a cell of width w.
func (t *pdfTable) fit(s string, w float64) string {
	if t.pdf.GetStringWidth(s) <= w {
		return s
	}
	for len(s) > 0 && t.pdf.GetStringWidth(s+"...") > w {
		s = s[:len(s)-1]
	}
	return s + "..."
}

func newTable(p *fpdf.Fpdf, tr func(string) string, cols []string, widths []float64, aligns string) *pdfTable {
	t := &pdfTable{pdf: p, tr: tr, cols: cols, widths: widths}
	for _, a := range aligns {
		t.aligns = append(t.aligns, string(a))
	}
	t.ensureSpace(rowHeight * 3)
	t.drawHeader()
	return t
}

func section(p *fpdf.Fpdf, tr func(string) string, title string) {
	_, pageH := p.GetPageSize()
	if p.GetY()+rowHeight*4 > pageH-pdfMargin {
		p.AddPage()
	} else {
		p.Ln(5)
	}
	p.SetFont("Helvetica", "B", 11)
	p.CellFormat(0, 8, tr(title), "", 1, "L", false, 0, "")
}

func n(v int64) string { return money.Format(v * 100) }

// PDF renders the report as an A4 document ready to print.
func PDF(d Data) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	p.SetAutoPageBreak(false, pdfMargin)
	p.AliasNbPages("")
	tr := p.UnicodeTranslatorFromDescriptor("")
	r := d.Report

	p.SetFooterFunc(func() {
		p.SetY(-10)
		p.SetFont("Helvetica", "", 8)
		p.SetTextColor(120, 120, 120)
		p.CellFormat(120, 5, tr(printedText(d)), "", 0, "L", false, 0, "")
		p.CellFormat(0, 5, fmt.Sprintf("Halaman %d/{nb}", p.PageNo()), "", 0, "R", false, 0, "")
		p.SetTextColor(0, 0, 0)
	})
	p.AddPage()

	p.SetFont("Helvetica", "B", 15)
	p.CellFormat(0, 8, tr(title(d)), "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "", 10)
	p.CellFormat(0, 6, tr("Periode: "+rangeText(r)+"   ·   "+periodName[r.Period]), "", 1, "L", false, 0, "")
	if d.Filter != "" {
		p.CellFormat(0, 6, tr("Filter: "+d.Filter), "", 1, "L", false, 0, "")
	}

	section(p, tr, "Ringkasan")
	t := newTable(p, tr, []string{"", "Jumlah", "Pendapatan (Rp)"}, []float64{80, 40, 60}, "LRR")
	t.row(false, "Jumlah transaksi", n(r.Transactions), "")
	t.row(false, "Tiket", n(r.TicketQty), money.Format(r.TicketTotal))
	t.row(false, "Topup Card", n(r.TopupQty), money.Format(r.TopupTotal))
	t.row(true, "Total", n(r.Qty), money.Format(r.Total))

	section(p, tr, "Per periode ("+periodName[r.Period]+")")
	t = newTable(p, tr,
		[]string{"Periode", "Transaksi", "Tiket", "Rp Tiket", "Topup", "Rp Topup", "Total (Rp)"},
		[]float64{44, 20, 16, 28, 16, 26, 30}, "LRRRRRR")
	for _, row := range r.Rows {
		t.row(false, periodLabel(row.Period, r.Period), n(row.Transactions), n(row.TicketQty), money.Format(row.TicketTotal), n(row.TopupQty), money.Format(row.TopupTotal), money.Format(row.Total))
	}
	t.row(true, "Total", n(r.Transactions), n(r.TicketQty), money.Format(r.TicketTotal), n(r.TopupQty), money.Format(r.TopupTotal), money.Format(r.Total))

	section(p, tr, "Per item")
	t = newTable(p, tr, []string{"Item", "Terjual", "Total (Rp)"}, []float64{100, 30, 50}, "LRR")
	for _, g := range itemGroups(r) {
		t.row(true, g.label, "", "")
		for _, it := range g.items {
			t.row(false, "   "+it.TicketName, n(it.Qty), money.Format(it.Total))
		}
		t.row(true, "Subtotal "+g.label, n(g.qty), money.Format(g.total))
	}
	t.row(true, "Total", n(r.Qty), money.Format(r.Total))

	count := max(d.SalesCount, int64(len(d.Sales)))
	section(p, tr, "Daftar transaksi ("+strconv.FormatInt(count, 10)+")")
	if count > PDFMaxSales {
		p.SetFont("Helvetica", "", 10)
		p.MultiCell(0, 6, tr(fmt.Sprintf(
			"Terlalu banyak untuk dicetak (lebih dari %d transaksi). Daftar lengkap ada di Export Excel, atau pilih rentang tanggal yang lebih pendek.",
			PDFMaxSales)), "", "L", false)
		return output(p)
	}
	t = newTable(p, tr, []string{"Waktu", "No. Nota", "Kasir", "Loket", "Total (Rp)"}, []float64{32, 44, 38, 30, 36}, "LLLLR")
	var total int64
	for _, s := range d.Sales {
		t.row(false, formatDateTime(s.CreatedAt), s.InvoiceNo, s.CashierName, counterName(s), money.Format(s.Total))
		total += s.Total
	}
	t.row(true, "Total", "", "", "", money.Format(total))
	return output(p)
}

func output(p *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
