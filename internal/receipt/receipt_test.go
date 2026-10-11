package receipt

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"tiket/internal/model"
)

func settings() model.Settings {
	return model.Settings{
		BusinessName:   "Pony Park",
		PurposeWeekday: "WEEKDAY REGULER",
		PurposeWeekend: "WEEKEND REGULER",
	}
}

func sampleSale() model.Sale {
	return model.Sale{
		InvoiceNo:    "PNY25202610090003",
		CashierName:  "Puput",
		Terminal:     "LOKET 1",
		CreatedAt:    "2026-10-09 10:18:00",
		Total:        7000000,
		Paid:         10000000,
		ChangeAmount: 3000000,
		Items: []model.SaleItem{
			{TicketName: "Entry Ticket Weekday", Price: 3500000, Qty: 2, Subtotal: 7000000},
		},
	}
}

func lineWidth(l Line) int {
	if l.Size == SizeDouble {
		return len(l.Text) * 2
	}
	return len(l.Text)
}

func TestBuildMatchesSample(t *testing.T) {
	for _, paper := range []int{58, 80} {
		cols := Columns(paper)
		lines := Build(settings(), sampleSale(), paper)
		for _, l := range lines {
			if lineWidth(l) > cols {
				t.Errorf("paper %d: line %q too wide (%d > %d)", paper, l.Text, lineWidth(l), cols)
			}
		}
		text := Text(lines)
		for _, want := range []string{
			"No       : PNY25202610090003",
			"Date     : 09-10-2026 10:18",
			"Time In  : 09-10-2026 10:18",
			"Purpose  : WEEKDAY REGULER",
			"Cashier  : Puput",
			"Entry Ticket Weekday",
			"2x    @35.000",
			"2 items",
			"Grand Total :",
			"CASH :",
			"Change :",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("paper %d: receipt missing %q\n%s", paper, want, text)
			}
		}
		if strings.Contains(text, "Reprint") {
			t.Errorf("first print must not be labelled reprint")
		}
	}
}

func TestPurposeWeekend(t *testing.T) {
	sale := sampleSale()
	sale.CreatedAt = "2026-10-10 09:00:00"
	if got := Purpose(settings(), sale.CreatedAt); got != "WEEKEND REGULER" {
		t.Errorf("saturday purpose = %q", got)
	}
	if got := Purpose(settings(), "2026-10-11 09:00:00"); got != "WEEKEND REGULER" {
		t.Errorf("sunday purpose = %q", got)
	}
	if got := Purpose(settings(), "2026-10-12 09:00:00"); got != "WEEKDAY REGULER" {
		t.Errorf("monday purpose = %q", got)
	}
}

func TestReprintLabel(t *testing.T) {
	sale := sampleSale()
	sale.PrintCount = 2
	lines := Build(settings(), sale, 80)
	var found bool
	for _, l := range lines {
		if strings.TrimSpace(l.Text) == "Reprint 2" {
			found = l.Size == SizeDouble
		}
	}
	if !found {
		t.Errorf("missing double-size Reprint 2 label:\n%s", Text(lines))
	}
}

func TestTotalsAligned(t *testing.T) {
	lines := Build(settings(), sampleSale(), 58)
	var colons []int
	for _, l := range lines {
		for _, label := range []string{"Grand Total :", "CASH :", "Change :"} {
			if strings.Contains(l.Text, label) {
				colons = append(colons, strings.Index(l.Text, ":"))
			}
		}
	}
	if len(colons) != 3 || colons[0] != colons[1] || colons[1] != colons[2] {
		t.Errorf("colons not aligned: %v", colons)
	}
}

func TestLogoHeader(t *testing.T) {
	st := settings()
	st.HasLogo = true
	lines := Build(st, sampleSale(), 58)
	if !lines[0].Logo {
		t.Fatalf("first line should be logo, got %+v", lines[0])
	}
	if strings.Contains(Text(lines), "Pony Park") {
		t.Error("business name should be replaced by logo")
	}
}

func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.Transparent)
			}
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestPrepareLogo(t *testing.T) {
	bm, err := PrepareLogo(testPNG(1000, 500), 58)
	if err != nil {
		t.Fatal(err)
	}
	if bm.Width != 288 || bm.Height != 144 {
		t.Fatalf("logo size = %dx%d, want 288x144", bm.Width, bm.Height)
	}
	if !bm.Black[10*bm.Width+10] || bm.Black[10*bm.Width+bm.Width-10] {
		t.Error("left should be black, transparent right should be white")
	}
	raster := bm.Raster()
	header := []byte{0x1D, 0x76, 0x30, 0x00, 36, 0, 128, 0}
	if !bytes.HasPrefix(raster, header) {
		t.Errorf("raster header = % x", raster[:8])
	}
	want := 2*8 + 36*144
	if len(raster) != want {
		t.Errorf("raster length = %d, want %d", len(raster), want)
	}

	small, err := PrepareLogo(testPNG(100, 50), 80)
	if err != nil {
		t.Fatal(err)
	}
	if small.Width != 104 {
		t.Errorf("small logo should not be upscaled, width = %d", small.Width)
	}

	if _, err := PrepareLogo([]byte("bukan gambar"), 58); err == nil {
		t.Error("invalid image accepted")
	}
}

func TestESCPOS(t *testing.T) {
	bm, _ := PrepareLogo(testPNG(80, 40), 58)
	data := ESCPOS([]Line{{Logo: true}, {Text: "Reprint 1", Size: SizeDouble}, {Text: "A", Bold: true}, {Text: "B"}}, bm)
	if !bytes.HasPrefix(data, escInit) || !bytes.HasSuffix(data, escCut) {
		t.Error("missing init/cut")
	}
	if !bytes.Contains(data, append(append([]byte{}, escCenter...), 0x1D, 0x76, 0x30)) {
		t.Error("logo not centered raster")
	}
	if !bytes.Contains(data, append(append([]byte{}, escSizeDbl...), "Reprint 1"...)) {
		t.Error("double size not applied")
	}
	want := append(append(append([]byte{}, escBoldOn...), 'A'), escBoldOff...)
	if !bytes.Contains(data, want) {
		t.Error("missing bold line")
	}
	noLogo := ESCPOS([]Line{{Logo: true}}, nil)
	if bytes.Contains(noLogo, []byte{0x1D, 0x76, 0x30}) {
		t.Error("raster emitted without logo")
	}
}
