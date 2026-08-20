package imageproc

import (
	"image"
	"image/color"
	"image/draw"
)

const (
	// MaxBrushSize is the largest paint brush radius in pixels.
	MaxBrushSize = 64
	// DefaultBrushSize is used when a brush size is out of range.
	DefaultBrushSize = 8
	// MaxPaintUndo is the number of paint snapshots kept for undo.
	MaxPaintUndo = 16
)

// ClampBrushSize limits radius to 1..MaxBrushSize.
func ClampBrushSize(radius int) int {
	if radius < 1 {
		return 1
	}
	if radius > MaxBrushSize {
		return MaxBrushSize
	}
	return radius
}

// CloneNRGBA copies src into a new origin-aligned NRGBA buffer.
func CloneNRGBA(src *image.NRGBA) *image.NRGBA {
	if src == nil {
		return nil
	}
	dst := image.NewNRGBA(src.Rect)
	copy(dst.Pix, src.Pix)
	return dst
}

// CompositeOver returns a copy of base with overlay blended on top.
func CompositeOver(base, overlay image.Image) *image.NRGBA {
	if base == nil {
		return nil
	}
	b := base.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), base, b.Min, draw.Src)
	if overlay != nil {
		ob := overlay.Bounds()
		draw.Draw(dst, dst.Bounds(), overlay, ob.Min, draw.Over)
	}
	return dst
}

// StampBrush paints a filled circle of radius at center.
// If erase is true, pixels are set to fully transparent.
func StampBrush(dst *image.NRGBA, center image.Point, radius int, c color.NRGBA, erase bool) {
	if dst == nil {
		return
	}
	radius = ClampBrushSize(radius)
	b := dst.Bounds()
	r2 := radius * radius
	minX := max(b.Min.X, center.X-radius)
	maxX := min(b.Max.X, center.X+radius+1)
	minY := max(b.Min.Y, center.Y-radius)
	maxY := min(b.Max.Y, center.Y+radius+1)
	for y := minY; y < maxY; y++ {
		dy := y - center.Y
		for x := minX; x < maxX; x++ {
			dx := x - center.X
			if dx*dx+dy*dy > r2 {
				continue
			}
			i := dst.PixOffset(x, y)
			if erase {
				dst.Pix[i] = 0
				dst.Pix[i+1] = 0
				dst.Pix[i+2] = 0
				dst.Pix[i+3] = 0
				continue
			}
			dst.Pix[i] = c.R
			dst.Pix[i+1] = c.G
			dst.Pix[i+2] = c.B
			dst.Pix[i+3] = c.A
			if c.A == 0 {
				dst.Pix[i+3] = 255
			}
		}
	}
}

// StrokeBrush stamps the brush along the line from start to end, inclusive.
func StrokeBrush(dst *image.NRGBA, start, end image.Point, radius int, c color.NRGBA, erase bool) {
	if dst == nil {
		return
	}
	dx := end.X - start.X
	if dx < 0 {
		dx = -dx
	}
	dy := end.Y - start.Y
	if dy < 0 {
		dy = -dy
	}
	steps := max(dx, dy)
	if steps == 0 {
		StampBrush(dst, start, radius, c, erase)
		return
	}
	for i := 0; i <= steps; i++ {
		x := start.X + (end.X-start.X)*i/steps
		y := start.Y + (end.Y-start.Y)*i/steps
		StampBrush(dst, image.Pt(x, y), radius, c, erase)
	}
}
