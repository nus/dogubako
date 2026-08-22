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

func TestNormalizeDegrees(t *testing.T) {
	cases := map[int]int{
		0:    0,
		90:   90,
		360:  0,
		-90:  270,
		450:  90,
		-1:   359,
		-360: 0,
	}
	for in, want := range cases {
		if got := NormalizeDegrees(in); got != want {
			t.Errorf("NormalizeDegrees(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestRotatedSize(t *testing.T) {
	base := image.Pt(20, 10)
	if got := RotatedSize(base, 0); got != base {
		t.Fatalf("0° = %v", got)
	}
	if got := RotatedSize(base, 90); got != (image.Point{X: 10, Y: 20}) {
		t.Fatalf("90° = %v", got)
	}
	if got := RotatedSize(base, 180); got != base {
		t.Fatalf("180° = %v", got)
	}
	if got := RotatedSize(base, 270); got != (image.Point{X: 10, Y: 20}) {
		t.Fatalf("270° = %v", got)
	}
	got := RotatedSize(image.Pt(10, 10), 45)
	if got.X < 14 || got.Y < 14 {
		t.Fatalf("45° square too small: %v", got)
	}
}

func TestRotate90PixelMapping(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})         // A
	src.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})         // B
	src.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})         // C
	src.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, A: 255}) // D

	got, err := Rotate(src, 90)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	// Clockwise: C A / D B
	if c := got.NRGBAAt(0, 0); c != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("top-left = %+v", c)
	}
	if c := got.NRGBAAt(1, 0); c != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("top-right = %+v", c)
	}
	if c := got.NRGBAAt(0, 1); c != (color.NRGBA{R: 255, G: 255, A: 255}) {
		t.Fatalf("bottom-left = %+v", c)
	}
	if c := got.NRGBAAt(1, 1); c != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("bottom-right = %+v", c)
	}
}

func TestRotate180And270(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	got, err := Rotate(src, 180)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != image.Rect(0, 0, 3, 2) {
		t.Fatalf("180 bounds = %v", got.Bounds())
	}
	if c := got.NRGBAAt(2, 1); c != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("180 pixel = %+v", c)
	}

	got, err = Rotate(src, 270)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatalf("270 bounds = %v", got.Bounds())
	}
	if c := got.NRGBAAt(0, 2); c != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("270 pixel = %+v", c)
	}
}

func TestRotateZeroIsIdentity(t *testing.T) {
	src := solidNRGBA(4, 3, color.NRGBA{G: 128, A: 255})
	got, err := Rotate(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != src.Bounds() {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	if c := got.NRGBAAt(1, 1); c != (color.NRGBA{G: 128, A: 255}) {
		t.Fatalf("pixel = %+v", c)
	}
}

func TestRotateArbitraryExpandsCanvas(t *testing.T) {
	src := solidNRGBA(10, 6, color.NRGBA{R: 255, A: 255})
	got, err := Rotate(src, 45)
	if err != nil {
		t.Fatal(err)
	}
	want := RotatedSize(image.Pt(10, 6), 45)
	if got.Bounds().Size() != want {
		t.Fatalf("size = %v, want %v", got.Bounds().Size(), want)
	}
	cx, cy := want.X/2, want.Y/2
	if c := got.NRGBAAt(cx, cy); c.R < 200 || c.A < 200 {
		t.Fatalf("center should stay opaque red, got %+v", c)
	}
}

func TestApplyCropThenRotateThenResize(t *testing.T) {
	src := solidNRGBA(20, 10, color.NRGBA{B: 200, A: 255})
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	got, err := Apply(src, Params{
		Width:         5,
		Height:        10,
		CropEnabled:   true,
		Crop:          image.Rect(0, 0, 10, 10),
		RotateDegrees: 90,
		Format:        FormatPNG,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 5 || got.Bounds().Dy() != 10 {
		t.Fatalf("size = %v", got.Bounds())
	}
}

func TestRotateRejectsOversized(t *testing.T) {
	got := RotatedSize(image.Pt(MaxDimension, MaxDimension), 45)
	if got.X <= MaxDimension && got.Y <= MaxDimension {
		t.Fatalf("expected 45° of max square to exceed limit, got %v", got)
	}
	img := &boundsImage{r: image.Rect(0, 0, MaxDimension, MaxDimension)}
	if _, err := Rotate(img, 45); err == nil {
		t.Fatal("expected error for oversized rotation")
	}
}

type boundsImage struct {
	r image.Rectangle
}

func (b *boundsImage) ColorModel() color.Model { return color.NRGBAModel }
func (b *boundsImage) Bounds() image.Rectangle { return b.r }
func (b *boundsImage) At(int, int) color.Color { return color.NRGBA{} }

func TestRotateNil(t *testing.T) {
	if _, err := Rotate(nil, 90); err == nil {
		t.Fatal("expected error")
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

func TestApplyPreviewCapsEdge(t *testing.T) {
	src := solidNRGBA(200, 100, color.NRGBA{R: 255, A: 255})
	got, err := ApplyPreview(src, Params{Width: 200, Height: 100, Format: FormatPNG}, 50)
	if err != nil {
		t.Fatal(err)
	}
	if max(got.Bounds().Dx(), got.Bounds().Dy()) > 50 {
		t.Fatalf("preview = %v", got.Bounds())
	}
}

func TestDecodeKeepsJPEGYCbCr(t *testing.T) {
	src := solidNRGBA(8, 8, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	data, err := Encode(src, FormatJPEG, 95)
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := DecodeBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatJPEG {
		t.Fatalf("format = %s", format)
	}
	if _, ok := img.(*image.YCbCr); !ok {
		t.Fatalf("type %T, want YCbCr", img)
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
