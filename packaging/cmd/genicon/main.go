// Command genicon writes a toolbox-style icon as ICNS (macOS) or PNG (Linux).
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	out := flag.String("o", "dogubako.icns", "output .icns or .png path")
	size := flag.Int("size", 256, "png pixel size (ignored for icns)")
	flag.Parse()
	if err := writeIcon(*out, *size); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeIcon(path string, pngSize int) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return writePNG(path, pngSize)
	default:
		return writeICNS(path)
	}
}

func writePNG(path string, size int) error {
	if size < 16 || size > 1024 {
		return fmt.Errorf("png size must be 16..1024, got %d", size)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, scaleImage(renderIcon(1024), size))
}

func writeICNS(path string) error {
	master := renderIcon(1024)
	entries := []struct {
		osType string
		size   int
	}{
		{"icp4", 16},
		{"icp5", 32},
		{"icp6", 64},
		{"ic07", 128},
		{"ic08", 256},
		{"ic09", 512},
		{"ic10", 1024},
		{"ic11", 32},
		{"ic12", 64},
		{"ic13", 256},
		{"ic14", 512},
	}

	var body []byte
	for _, e := range entries {
		pngBytes, err := encodePNG(scaleImage(master, e.size))
		if err != nil {
			return err
		}
		header := make([]byte, 8)
		copy(header[:4], e.osType)
		binary.BigEndian.PutUint32(header[4:], uint32(8+len(pngBytes)))
		body = append(body, header...)
		body = append(body, pngBytes...)
	}

	file := make([]byte, 8+len(body))
	copy(file[:4], "icns")
	binary.BigEndian.PutUint32(file[4:], uint32(len(file)))
	copy(file[8:], body)
	return os.WriteFile(path, file, 0o644)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf []byte
	w := &sliceWriter{buf: &buf}
	if err := png.Encode(w, img); err != nil {
		return nil, err
	}
	return buf, nil
}

type sliceWriter struct{ buf *[]byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

func scaleImage(src *image.RGBA, size int) *image.RGBA {
	if src.Bounds().Dx() == size {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sw := src.Bounds().Dx()
	ratio := sw / size
	if ratio < 1 {
		ratio = 1
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			var n uint32
			x0, y0 := x*ratio, y*ratio
			x1, y1 := x0+ratio, y0+ratio
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					c := src.RGBAAt(sx, sy)
					r += uint32(c.R)
					g += uint32(c.G)
					b += uint32(c.B)
					a += uint32(c.A)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	return dst
}

func renderIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{R: 0x1B, G: 0x3A, B: 0x2F, A: 0xFF}
	fill(img, bg)

	s := float64(size)
	roundRect(img, 0.06*s, 0.06*s, 0.94*s, 0.94*s, 0.18*s, color.RGBA{R: 0x24, G: 0x4A, B: 0x3C, A: 0xFF})

	// Toolbox body and lid.
	roundRect(img, 0.18*s, 0.42*s, 0.82*s, 0.82*s, 0.06*s, color.RGBA{R: 0xE0, G: 0x9A, B: 0x3E, A: 0xFF})
	roundRect(img, 0.16*s, 0.34*s, 0.84*s, 0.48*s, 0.05*s, color.RGBA{R: 0xB8, G: 0x6F, B: 0x28, A: 0xFF})

	// Handle.
	handle := color.RGBA{R: 0x3D, G: 0x2A, B: 0x1A, A: 0xFF}
	roundRect(img, 0.36*s, 0.16*s, 0.42*s, 0.36*s, 0.02*s, handle)
	roundRect(img, 0.58*s, 0.16*s, 0.64*s, 0.36*s, 0.02*s, handle)
	roundRect(img, 0.36*s, 0.14*s, 0.64*s, 0.22*s, 0.03*s, handle)

	// Clasp.
	circle(img, 0.50*s, 0.47*s, 0.045*s, color.RGBA{R: 0xF0, G: 0xD7, B: 0x8C, A: 0xFF})
	circle(img, 0.50*s, 0.47*s, 0.022*s, handle)

	// Front panel line.
	roundRect(img, 0.24*s, 0.56*s, 0.76*s, 0.60*s, 0.01*s, color.RGBA{R: 0xC4, G: 0x82, B: 0x32, A: 0xFF})
	return img
}

func fill(img *image.RGBA, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func roundRect(img *image.RGBA, x0, y0, x1, y1, r float64, c color.RGBA) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	if r < 0 {
		r = 0
	}
	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	b := img.Bounds()
	if ix0 < b.Min.X {
		ix0 = b.Min.X
	}
	if iy0 < b.Min.Y {
		iy0 = b.Min.Y
	}
	if ix1 > b.Max.X {
		ix1 = b.Max.X
	}
	if iy1 > b.Max.Y {
		iy1 = b.Max.Y
	}
	r2 := r * r
	for y := iy0; y < iy1; y++ {
		for x := ix0; x < ix1; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5
			cx, cy := fx, fy
			if fx < x0+r {
				cx = x0 + r
			} else if fx > x1-r {
				cx = x1 - r
			}
			if fy < y0+r {
				cy = y0 + r
			} else if fy > y1-r {
				cy = y1 - r
			}
			dx, dy := fx-cx, fy-cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func circle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	roundRect(img, cx-r, cy-r, cx+r, cy+r, r, c)
}
