package imageproc

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Format is an output image format.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
)

const (
	// MaxDimension is the largest width or height Apply will produce.
	MaxDimension = 10000
	// DefaultJPEGQuality is used when JPEGQuality is out of range.
	DefaultJPEGQuality = 90
)

// Params describes how to transform a source image.
type Params struct {
	Width  int
	Height int

	CropEnabled bool
	Crop        image.Rectangle

	// RotateDegrees is a clockwise angle in 1° steps. Values outside 0..359
	// are wrapped by Apply.
	RotateDegrees int

	Format      Format
	JPEGQuality int
}

// Decode reads an image and reports its source format when known.
func Decode(r io.Reader) (image.Image, Format, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	return asNRGBA(img), NormalizeFormat(format), nil
}

// DecodeBytes decodes an image from an in-memory buffer.
func DecodeBytes(data []byte) (image.Image, Format, error) {
	return Decode(bytes.NewReader(data))
}

// NormalizeFormat maps image.Decode format names onto Format.
func NormalizeFormat(name string) Format {
	switch strings.ToLower(name) {
	case "jpeg", "jpg":
		return FormatJPEG
	case "png":
		return FormatPNG
	default:
		return FormatPNG
	}
}

// Extension returns the usual file suffix for format, including the dot.
func Extension(format Format) string {
	switch format {
	case FormatJPEG:
		return ".jpg"
	default:
		return ".png"
	}
}

// Apply crops, rotates, and resizes src according to p. Format is ignored here; use Encode for that.
func Apply(src image.Image, p Params) (image.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("no image")
	}
	img := asNRGBA(src)
	if p.CropEnabled {
		cropped, err := Crop(img, p.Crop)
		if err != nil {
			return nil, err
		}
		img = cropped
	}
	if NormalizeDegrees(p.RotateDegrees) != 0 {
		rotated, err := Rotate(img, p.RotateDegrees)
		if err != nil {
			return nil, err
		}
		img = rotated
	}
	if p.Width <= 0 || p.Height <= 0 {
		return nil, fmt.Errorf("invalid size %d×%d", p.Width, p.Height)
	}
	if p.Width > MaxDimension || p.Height > MaxDimension {
		return nil, fmt.Errorf("size %d×%d exceeds the %dpx limit", p.Width, p.Height, MaxDimension)
	}
	if img.Bounds().Dx() == p.Width && img.Bounds().Dy() == p.Height {
		return img, nil
	}
	return Resize(img, p.Width, p.Height), nil
}

// Crop returns the intersection of src and rect as a new image origin at (0,0).
func Crop(src image.Image, rect image.Rectangle) (*image.NRGBA, error) {
	if src == nil {
		return nil, fmt.Errorf("no image")
	}
	srcBounds := src.Bounds()
	c := rect.Intersect(srcBounds)
	if c.Empty() {
		return nil, fmt.Errorf("crop rectangle is empty")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, c.Dx(), c.Dy()))
	draw.Draw(dst, dst.Bounds(), src, c.Min, draw.Src)
	return dst, nil
}

// Resize scales src to width×height with Catmull-Rom interpolation.
func Resize(src image.Image, width, height int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}

// Encode writes img in the requested format.
func Encode(img image.Image, format Format, jpegQuality int) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("no image")
	}
	var buf bytes.Buffer
	switch format {
	case FormatJPEG:
		q := jpegQuality
		if q < 1 || q > 100 {
			q = DefaultJPEGQuality
		}
		if err := jpeg.Encode(&buf, flattenForJPEG(img), &jpeg.Options{Quality: q}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
	case FormatPNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
	return buf.Bytes(), nil
}

// EncodePNG is a convenience wrapper used for clipboard output.
func EncodePNG(img image.Image) ([]byte, error) {
	return Encode(img, FormatPNG, 0)
}

// ScaleSize returns width and height after applying percent to base.
func ScaleSize(base image.Point, percent int) image.Point {
	if percent < 1 {
		percent = 1
	}
	w := max(1, int(math.Round(float64(base.X)*float64(percent)/100)))
	h := max(1, int(math.Round(float64(base.Y)*float64(percent)/100)))
	return image.Pt(min(w, MaxDimension), min(h, MaxDimension))
}

// HeightForWidth keeps aspect ratio from base.
func HeightForWidth(base image.Point, width int) int {
	if base.X <= 0 {
		return max(1, width)
	}
	h := int(math.Round(float64(width) * float64(base.Y) / float64(base.X)))
	return max(1, min(h, MaxDimension))
}

// WidthForHeight keeps aspect ratio from base.
func WidthForHeight(base image.Point, height int) int {
	if base.Y <= 0 {
		return max(1, height)
	}
	w := int(math.Round(float64(height) * float64(base.X) / float64(base.Y)))
	return max(1, min(w, MaxDimension))
}

func asNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Rect.Min == image.Pt(0, 0) {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func flattenForJPEG(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
