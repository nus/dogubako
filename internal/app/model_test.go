package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/nus/dogubako/internal/imageproc"
)

func TestImageModelResizeAndCrop(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var m ImageModel
	m.setSource(src, "test.png", imageproc.FormatPNG)

	m.SetWidth(10)
	if m.Width() != 10 || m.Height() != 5 {
		t.Fatalf("keep-aspect resize: %dx%d", m.Width(), m.Height())
	}

	m.SetCropEnabled(true)
	m.SetCrop(image.Rect(0, 0, 10, 10))
	m.SetScalePercent(50)
	got, err := m.Processed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 5 || got.Bounds().Dy() != 5 {
		t.Fatalf("processed size = %v", got.Bounds())
	}
}

func TestImageModelSaveRoundTrip(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{G: 200, A: 255})
		}
	}
	var m ImageModel
	m.setSource(src, "in.png", imageproc.FormatPNG)
	m.SetFormat(imageproc.FormatPNG)

	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	if err := m.SavePath(path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatal(err)
	}
}

func TestSuggestedFilename(t *testing.T) {
	var m ImageModel
	m.sourceName = "photo.jpeg"
	m.params.Format = imageproc.FormatPNG
	if got := m.SuggestedFilename(); got != "photo-edited.png" {
		t.Fatalf("got %q", got)
	}
}
