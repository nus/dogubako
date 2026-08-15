package app

import (
	"image"
	"testing"
)

func TestNormalizeCrop(t *testing.T) {
	got := normalizeCrop(image.Rect(8, 2, 2, 6), image.Pt(10, 8))
	if got != image.Rect(2, 2, 8, 6) {
		t.Fatalf("got %v", got)
	}
	got = normalizeCrop(image.Rect(-4, -4, 1, 1), image.Pt(10, 8))
	if got.Min.X < 0 || got.Min.Y < 0 || got.Empty() {
		t.Fatalf("clamped crop = %v", got)
	}
}

func TestFittedRectKeepsAspect(t *testing.T) {
	got := fittedRect(image.Rect(0, 0, 200, 100), image.Pt(100, 100))
	if got.Dx() != 100 || got.Dy() != 100 {
		t.Fatalf("got %v", got)
	}
	if got.Min.X != 50 {
		t.Fatalf("centered x = %d", got.Min.X)
	}
}

func TestDestPreviewHasImageFromSource(t *testing.T) {
	var p destPreview
	if p.HasImage() {
		t.Fatal("empty")
	}
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	p.SetSource(src)
	if !p.HasImage() {
		t.Fatal("source should count as an image before GPU upload")
	}
	p.SetSource(nil)
	if p.HasImage() {
		t.Fatal("cleared")
	}
}

func TestScreenToImageRoundTrip(t *testing.T) {
	fitted := image.Rect(10, 20, 110, 70)
	imgSize := image.Pt(200, 100)
	pt := screenToImage(image.Pt(10, 20), fitted, imgSize)
	if pt != (image.Point{}) {
		t.Fatalf("top-left = %v", pt)
	}
	pt = screenToImage(image.Pt(109, 69), fitted, imgSize)
	if pt.X < 0 || pt.X >= imgSize.X || pt.Y < 0 || pt.Y >= imgSize.Y {
		t.Fatalf("bottom-right out of range: %v", pt)
	}
}
