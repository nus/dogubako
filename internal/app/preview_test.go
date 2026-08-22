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

func TestSourcePreviewZoomInOutReset(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	var p sourcePreview
	p.SetImage(src, image.Pt(40, 40))
	p.bounds = image.Rect(0, 0, 80, 80)
	p.ZoomIn()
	if p.view.Scale() <= 1 {
		t.Fatalf("scale = %v", p.view.Scale())
	}
	p.ZoomOut()
	if p.view.Scale() != 1 {
		t.Fatalf("after zoom out scale = %v", p.view.Scale())
	}
	p.ZoomIn()
	p.ResetZoom()
	if p.view.Scale() != 1 {
		t.Fatalf("after reset scale = %v", p.view.Scale())
	}
}

func TestDestPreviewZoomUsesLogicalSize(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	var p destPreview
	p.SetImage(src)
	p.SetLogicalSize(image.Pt(200, 200))
	p.bounds = image.Rect(0, 0, 80, 80)
	p.ZoomIn()
	if p.view.key != image.Pt(200, 200) {
		t.Fatalf("zoom key = %v", p.view.key)
	}
	if p.view.Scale() <= 1 {
		t.Fatalf("scale = %v", p.view.Scale())
	}
}
