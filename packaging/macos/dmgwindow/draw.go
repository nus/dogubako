package dmgwindow

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Render draws the window background at the given scale.
// Scale 1 is one pixel per point. Scale 2 is the retina image.
func Render(layout Layout, scale int) (*image.RGBA, error) {
	arrow, err := layout.Arrow()
	if err != nil {
		return nil, err
	}
	if scale < 1 {
		return nil, fmt.Errorf("scale must be at least 1")
	}
	s := float64(scale)
	img := image.NewRGBA(image.Rect(0, 0, layout.Width*scale, layout.Height*scale))
	fill(img, Background)

	// Geometry in pixels. Samples use pixel centers.
	a := Arrow{
		X0:       arrow.X0 * s,
		X1:       arrow.X1 * s,
		Tip:      arrow.Tip * s,
		Y:        arrow.Y * s,
		Shaft:    arrow.Shaft * s,
		HeadHalf: arrow.HeadHalf * s,
	}
	radius := a.Shaft / 2
	pad := radius + 2
	xMin := int(math.Floor(a.X0 - pad))
	xMax := int(math.Ceil(a.Tip + 2))
	yMin := int(math.Floor(a.Y - a.HeadHalf - 2))
	yMax := int(math.Ceil(a.Y + a.HeadHalf + 2))
	bounds := img.Bounds()
	if xMin < bounds.Min.X {
		xMin = bounds.Min.X
	}
	if yMin < bounds.Min.Y {
		yMin = bounds.Min.Y
	}
	if xMax > bounds.Max.X {
		xMax = bounds.Max.X
	}
	if yMax > bounds.Max.Y {
		yMax = bounds.Max.Y
	}

	const samples = 4
	for y := yMin; y < yMax; y++ {
		for x := xMin; x < xMax; x++ {
			hit := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := float64(x) + (float64(sx)+0.5)/samples
					py := float64(y) + (float64(sy)+0.5)/samples
					if coversArrow(px, py, a, radius) {
						hit++
					}
				}
			}
			if hit == 0 {
				continue
			}
			img.SetRGBA(x, y, blend(Background, ArrowColor, float64(hit)/float64(samples*samples)))
		}
	}
	return img, nil
}

func coversArrow(px, py float64, a Arrow, radius float64) bool {
	if distToSegment(px, py, a.X0, a.X1, a.Y) <= radius {
		return true
	}
	return pointInTriangle(px, py,
		a.Tip, a.Y,
		a.X1, a.Y-a.HeadHalf,
		a.X1, a.Y+a.HeadHalf,
	)
}

func distToSegment(px, py, x0, x1, y float64) float64 {
	if px < x0 {
		return math.Hypot(px-x0, py-y)
	}
	if px > x1 {
		return math.Hypot(px-x1, py-y)
	}
	return math.Abs(py - y)
}

func pointInTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	d := (by-cy)*(ax-cx) + (cx-bx)*(ay-cy)
	if d == 0 {
		return false
	}
	wa := ((by-cy)*(px-cx) + (cx-bx)*(py-cy)) / d
	wb := ((cy-ay)*(px-cx) + (ax-cx)*(py-cy)) / d
	wc := 1 - wa - wb
	const eps = -1e-9
	return wa >= eps && wb >= eps && wc >= eps
}

func blend(dst, src color.RGBA, cov float64) color.RGBA {
	if cov >= 1 {
		return src
	}
	inv := 1 - cov
	return color.RGBA{
		R: uint8(float64(dst.R)*inv + float64(src.R)*cov + 0.5),
		G: uint8(float64(dst.G)*inv + float64(src.G)*cov + 0.5),
		B: uint8(float64(dst.B)*inv + float64(src.B)*cov + 0.5),
		A: 255,
	}
}

func fill(img *image.RGBA, c color.RGBA) {
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}
}

// WriteDir writes background.png, background@2x.png, and layout.json.
// dmgbuild combines the two images into one HiDPI background and reads
// the icon positions from the JSON file.
func WriteDir(dir string, layout Layout) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(dir, "background.png"), layout, 1); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(dir, "background@2x.png"), layout, 2); err != nil {
		return err
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "layout.json"), data, 0o644)
}

func writePNG(path string, layout Layout, scale int) error {
	img, err := Render(layout, scale)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
