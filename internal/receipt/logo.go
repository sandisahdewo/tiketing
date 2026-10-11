package receipt

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
)

const (
	MaxLogoBytes  = 1 << 20
	maxLogoPixels = 4000
)

type Bitmap struct {
	Width  int
	Height int
	Black  []bool
}

func PrinterDots(paperWidth int) int {
	if paperWidth == 80 {
		return 576
	}
	return 384
}

func ValidateLogo(data []byte) error {
	if len(data) == 0 {
		return errors.New("File logo kosong")
	}
	if len(data) > MaxLogoBytes {
		return errors.New("Ukuran logo maksimal 1 MB")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("File logo harus berupa gambar PNG, JPG, atau GIF")
	}
	if cfg.Width > maxLogoPixels || cfg.Height > maxLogoPixels {
		return errors.New("Dimensi logo terlalu besar (maksimal 4000 piksel)")
	}
	return nil
}

func PrepareLogo(data []byte, paperWidth int) (*Bitmap, error) {
	if err := ValidateLogo(data); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	width := PrinterDots(paperWidth) * 3 / 4 / 8 * 8
	if b.Dx() < width {
		width = (b.Dx() + 7) / 8 * 8
	}
	height := b.Dy() * width / b.Dx()
	if height < 1 {
		height = 1
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), src, b, draw.Over, nil)

	bm := &Bitmap{Width: width, Height: height, Black: make([]bool, width*height)}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			g := color.GrayModel.Convert(canvas.At(x, y)).(color.Gray)
			bm.Black[y*width+x] = g.Y < 128
		}
	}
	return bm, nil
}

func (bm *Bitmap) Raster() []byte {
	bytesPerRow := bm.Width / 8
	var out []byte
	const band = 128
	for top := 0; top < bm.Height; top += band {
		rows := min(band, bm.Height-top)
		out = append(out, 0x1D, 0x76, 0x30, 0x00,
			byte(bytesPerRow), byte(bytesPerRow>>8),
			byte(rows), byte(rows>>8))
		for y := top; y < top+rows; y++ {
			for xb := 0; xb < bytesPerRow; xb++ {
				var v byte
				for bit := 0; bit < 8; bit++ {
					if bm.Black[y*bm.Width+xb*8+bit] {
						v |= 0x80 >> bit
					}
				}
				out = append(out, v)
			}
		}
	}
	return out
}

func (bm *Bitmap) PNG() ([]byte, error) {
	img := image.NewGray(image.Rect(0, 0, bm.Width, bm.Height))
	for i, black := range bm.Black {
		if black {
			img.Pix[i] = 0
		} else {
			img.Pix[i] = 255
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}
