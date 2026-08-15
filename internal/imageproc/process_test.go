package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func solidNRGBA(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r, g, b, a := c.RGBA()
	px := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, px)
		}
	}
	return img
}

func TestCrop(t *testing.T) {
	src := solidNRGBA(10, 8, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(3, 2, color.NRGBA{G: 255, A: 255})

	got, err := Crop(src, image.Rect(2, 1, 6, 5))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != image.Rect(0, 0, 4, 4) {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	if c := got.NRGBAAt(1, 1); c != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("pixel = %+v", c)
	}
}

func TestCropRejectsEmpty(t *testing.T) {
	src := solidNRGBA(4, 4, color.White)
	if _, err := Crop(src, image.Rect(10, 10, 12, 12)); err == nil {
		t.Fatal("expected error")
	}
}

func TestResize(t *testing.T) {
	src := solidNRGBA(4, 2, color.NRGBA{B: 255, A: 255})
	got := Resize(src, 8, 4)
	if got.Bounds().Dx() != 8 || got.Bounds().Dy() != 4 {
		t.Fatalf("size = %v", got.Bounds())
	}
	if c := got.NRGBAAt(0, 0); c.B < 250 || c.A < 250 {
		t.Fatalf("unexpected color %+v", c)
	}
}

func TestApplyCropThenResize(t *testing.T) {
	src := solidNRGBA(20, 10, color.NRGBA{R: 200, A: 255})
	got, err := Apply(src, Params{
		Width:       5,
		Height:      5,
		CropEnabled: true,
		Crop:        image.Rect(0, 0, 10, 10),
		Format:      FormatPNG,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 5 || got.Bounds().Dy() != 5 {
		t.Fatalf("size = %v", got.Bounds())
	}
}

func TestApplyRejectsMissingImageAndBadSize(t *testing.T) {
	if _, err := Apply(nil, Params{Width: 1, Height: 1}); err == nil {
		t.Fatal("expected error for nil image")
	}
	src := solidNRGBA(2, 2, color.White)
	if _, err := Apply(src, Params{Width: 0, Height: 2}); err == nil {
		t.Fatal("expected error for invalid size")
	}
	if _, err := Apply(src, Params{Width: MaxDimension + 1, Height: 1}); err == nil {
		t.Fatal("expected error for oversized output")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	src := solidNRGBA(6, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	pngBytes, err := Encode(src, FormatPNG, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(pngBytes)); err != nil {
		t.Fatal(err)
	}
	img, format, err := DecodeBytes(pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatPNG {
		t.Fatalf("format = %s", format)
	}
	if img.Bounds().Dx() != 6 || img.Bounds().Dy() != 4 {
		t.Fatalf("png size = %v", img.Bounds())
	}

	jpegBytes, err := Encode(src, FormatJPEG, 95)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(jpegBytes)); err != nil {
		t.Fatal(err)
	}
	img, format, err = DecodeBytes(jpegBytes)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatJPEG {
		t.Fatalf("format = %s", format)
	}
}

func TestEncodePNGClipboardHelper(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{A: 255})
	data, err := EncodePNG(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestScaleAndAspectHelpers(t *testing.T) {
	base := image.Pt(200, 100)
	if got := ScaleSize(base, 50); got != (image.Point{X: 100, Y: 50}) {
		t.Fatalf("scale = %v", got)
	}
	if got := HeightForWidth(base, 300); got != 150 {
		t.Fatalf("height = %d", got)
	}
	if got := WidthForHeight(base, 50); got != 100 {
		t.Fatalf("width = %d", got)
	}
	if got := ScaleSize(base, 0); got.X < 1 || got.Y < 1 {
		t.Fatalf("zero percent = %v", got)
	}
}

func TestNormalizeFormatAndExtension(t *testing.T) {
	if NormalizeFormat("jpg") != FormatJPEG {
		t.Fatal("jpg")
	}
	if NormalizeFormat("JPEG") != FormatJPEG {
		t.Fatal("JPEG")
	}
	if Extension(FormatJPEG) != ".jpg" {
		t.Fatal(Extension(FormatJPEG))
	}
	if Extension(FormatPNG) != ".png" {
		t.Fatal(Extension(FormatPNG))
	}
}
