package app

import (
	"image"
	"testing"
)

func TestPreviewImageRectAtFitMatchesFittedRect(t *testing.T) {
	cases := []struct {
		bounds  image.Rectangle
		imgSize image.Point
	}{
		{image.Rect(0, 0, 200, 100), image.Pt(100, 100)},
		{image.Rect(10, 20, 210, 180), image.Pt(400, 300)},
		{image.Rect(0, 0, 101, 99), image.Pt(17, 53)},
	}
	for _, tc := range cases {
		got := previewImageRect(tc.bounds, tc.imgSize, 1, 0, 0)
		want := fittedRect(tc.bounds, tc.imgSize)
		if got != want {
			t.Errorf("bounds=%v img=%v: got %v want %v", tc.bounds, tc.imgSize, got, want)
		}
	}
}

func TestClampPreviewPan(t *testing.T) {
	if got := clampPreviewPan(12, 50, 80); got != 0 {
		t.Fatalf("smaller than bounds: %v", got)
	}
	if got := clampPreviewPan(100, 200, 100); got != 50 {
		t.Fatalf("high clamp: %v", got)
	}
	if got := clampPreviewPan(-100, 200, 100); got != -50 {
		t.Fatalf("low clamp: %v", got)
	}
}

func TestZoomPreviewAtKeepsPointUnderCursor(t *testing.T) {
	bounds := image.Rect(0, 0, 400, 400)
	imgSize := image.Pt(100, 100)
	cursor := image.Pt(100, 200)
	old := previewImageRect(bounds, imgSize, 1, 0, 0)
	oldPt := screenToImage(cursor, old, imgSize)
	scale, panX, panY := zoomPreviewAt(bounds, imgSize, 1, 0, 0, cursor, 2)
	if scale != 2 {
		t.Fatalf("scale = %v", scale)
	}
	got := previewImageRect(bounds, imgSize, scale, panX, panY)
	newPt := screenToImage(cursor, got, imgSize)
	if absInt(newPt.X-oldPt.X) > 1 || absInt(newPt.Y-oldPt.Y) > 1 {
		t.Fatalf("image point moved from %v to %v", oldPt, newPt)
	}
}

func TestPreviewZoomFactor(t *testing.T) {
	if previewZoomFactor(0) != 1 {
		t.Fatal("zero delta")
	}
	if previewZoomFactor(-1) != previewZoomStep {
		t.Fatal("scroll up should zoom in")
	}
	if previewZoomFactor(1) != 1/previewZoomStep {
		t.Fatal("scroll down should zoom out")
	}
}

func TestPreviewViewResetsWhenSizeChanges(t *testing.T) {
	var v previewView
	v.syncKey(image.Pt(10, 10))
	v.scale = 4
	v.panX = 10
	v.syncKey(image.Pt(10, 10))
	if v.Scale() != 4 || v.panX != 10 {
		t.Fatalf("same size should keep view, scale=%v pan=%v", v.Scale(), v.panX)
	}
	v.syncKey(image.Pt(20, 10))
	if v.Scale() != 1 || v.panX != 0 {
		t.Fatalf("size change should reset, scale=%v pan=%v", v.Scale(), v.panX)
	}
}

func TestVisibleSourceRectAndMapImageRect(t *testing.T) {
	full := image.Rect(0, 0, 80, 80)
	visible := image.Rect(0, 0, 40, 40)
	src := visibleSourceRect(full, visible, image.Pt(20, 20))
	if src.Dx() < 1 || src.Dy() < 1 {
		t.Fatalf("src = %v", src)
	}
	raster := mapImageRect(image.Rect(0, 0, 20, 20), image.Pt(20, 20), src)
	if raster.Empty() {
		t.Fatal("mapped raster empty")
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
