package imageproc

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// NormalizeDegrees wraps an angle into 0..359.
func NormalizeDegrees(degrees int) int {
	degrees %= 360
	if degrees < 0 {
		degrees += 360
	}
	return degrees
}

// RotatedSize is the bounding-box size of size after a clockwise rotation.
func RotatedSize(size image.Point, degrees int) image.Point {
	w := max(0, size.X)
	h := max(0, size.Y)
	switch NormalizeDegrees(degrees) {
	case 0, 180:
		return image.Pt(max(1, w), max(1, h))
	case 90, 270:
		return image.Pt(max(1, h), max(1, w))
	}
	rad := float64(NormalizeDegrees(degrees)) * math.Pi / 180
	sin, cos := math.Abs(math.Sin(rad)), math.Abs(math.Cos(rad))
	rw := int(math.Ceil(float64(w)*cos + float64(h)*sin - 1e-9))
	rh := int(math.Ceil(float64(w)*sin + float64(h)*cos - 1e-9))
	return image.Pt(max(1, rw), max(1, rh))
}

// Rotate turns src clockwise by degrees. 90° steps copy pixels exactly;
// other angles use bilinear sampling and expand the canvas to fit.
func Rotate(src image.Image, degrees int) (*image.NRGBA, error) {
	if src == nil {
		return nil, fmt.Errorf("no image")
	}
	degrees = NormalizeDegrees(degrees)
	if degrees == 0 {
		return asNRGBA(src), nil
	}
	out := RotatedSize(src.Bounds().Size(), degrees)
	if out.X > MaxDimension || out.Y > MaxDimension {
		return nil, fmt.Errorf("rotated size %d×%d exceeds the %dpx limit", out.X, out.Y, MaxDimension)
	}
	img := asNRGBA(src)
	switch degrees {
	case 90:
		return rotate90CW(img), nil
	case 180:
		return rotate180(img), nil
	case 270:
		return rotate270CW(img), nil
	default:
		return rotateArbitrary(img, degrees, out), nil
	}
}

func rotate90CW(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := y*src.Stride + x*4
			di := x*dst.Stride + (h-1-y)*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

func rotate180(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := y*src.Stride + x*4
			di := (h-1-y)*dst.Stride + (w-1-x)*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

func rotate270CW(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := y*src.Stride + x*4
			di := (w-1-x)*dst.Stride + y*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

func rotateArbitrary(src *image.NRGBA, degrees int, out image.Point) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, out.X, out.Y))
	rad := float64(degrees) * math.Pi / 180
	sin, cos := math.Sin(rad), math.Cos(rad)
	srcCx := float64(sw) / 2
	srcCy := float64(sh) / 2
	dstCx := float64(out.X) / 2
	dstCy := float64(out.Y) / 2
	for y := 0; y < out.Y; y++ {
		dy := (float64(y) + 0.5) - dstCy
		for x := 0; x < out.X; x++ {
			dx := (float64(x) + 0.5) - dstCx
			// Inverse of clockwise rotation in y-down coordinates.
			sx := dx*cos + dy*sin + srcCx - 0.5
			sy := -dx*sin + dy*cos + srcCy - 0.5
			c := sampleBilinear(src, sw, sh, sx, sy)
			i := y*dst.Stride + x*4
			dst.Pix[i] = c.R
			dst.Pix[i+1] = c.G
			dst.Pix[i+2] = c.B
			dst.Pix[i+3] = c.A
		}
	}
	return dst
}

func sampleBilinear(src *image.NRGBA, w, h int, x, y float64) color.NRGBA {
	if x <= -1 || y <= -1 || x >= float64(w) || y >= float64(h) {
		return color.NRGBA{}
	}
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	c00 := pixelAt(src, w, h, x0, y0)
	c10 := pixelAt(src, w, h, x0+1, y0)
	c01 := pixelAt(src, w, h, x0, y0+1)
	c11 := pixelAt(src, w, h, x0+1, y0+1)
	return mixNRGBA(mixNRGBA(c00, c10, fx), mixNRGBA(c01, c11, fx), fy)
}

func pixelAt(src *image.NRGBA, w, h, x, y int) color.NRGBA {
	if x < 0 || y < 0 || x >= w || y >= h {
		return color.NRGBA{}
	}
	i := y*src.Stride + x*4
	return color.NRGBA{R: src.Pix[i], G: src.Pix[i+1], B: src.Pix[i+2], A: src.Pix[i+3]}
}

func mixNRGBA(a, b color.NRGBA, t float64) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	u := 1 - t
	aa := float64(a.A)
	ba := float64(b.A)
	outA := aa*u + ba*t
	if outA < 0.5 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8((float64(a.R)*aa*u+float64(b.R)*ba*t)/outA + 0.5),
		G: uint8((float64(a.G)*aa*u+float64(b.G)*ba*t)/outA + 0.5),
		B: uint8((float64(a.B)*aa*u+float64(b.B)*ba*t)/outA + 0.5),
		A: uint8(outA + 0.5),
	}
}
