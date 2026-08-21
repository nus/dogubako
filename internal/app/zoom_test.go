package app

import (
	"image"
	"testing"
)

func TestZoomViewFittedByDefault(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 200, 100)
	imgSize := image.Pt(400, 400)
	if !v.fitted() {
		t.Fatal("zero value should be fitted")
	}
	got := v.displayRect(bounds, imgSize)
	if got != fittedRect(bounds, imgSize) {
		t.Fatalf("fitted display = %v", got)
	}
	if pct := v.percent(bounds, imgSize); pct != 25 {
		t.Fatalf("percent = %d, want 25", pct)
	}
}

func TestZoomViewZoomInGrowsAroundAnchor(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 200, 200)
	imgSize := image.Pt(100, 100)
	anchor := image.Pt(60, 60)
	before := v.displayRect(bounds, imgSize)
	imgX := float64(anchor.X-before.Min.X) / 2 // fit scale is 2

	if !v.zoomBy(bounds, imgSize, 2, anchor) {
		t.Fatal("zoom in should change the view")
	}
	after := v.displayRect(bounds, imgSize)
	if after.Dx() != 400 {
		t.Fatalf("zoomed width = %d, want 400", after.Dx())
	}
	gotX := float64(anchor.X-after.Min.X) / 4
	if diff := gotX - imgX; diff > 1 || diff < -1 {
		t.Fatalf("anchor moved: image x %v -> %v", imgX, gotX)
	}
}

func TestZoomViewStopsAtActualSize(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 200, 200)
	imgSize := image.Pt(400, 400)
	// Fit is 50%; one step of 1.25 would land at 62.5%, further steps must
	// stop at 100% so actual size is reachable.
	for i := 0; i < 4; i++ {
		v.zoomBy(bounds, imgSize, zoomStepFactor, image.Pt(100, 100))
	}
	if pct := v.percent(bounds, imgSize); pct != 100 {
		t.Fatalf("percent = %d, want a stop at 100", pct)
	}
}

func TestZoomViewZoomOutSnapsBackToFit(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 200, 200)
	imgSize := image.Pt(200, 200)
	v.zoomBy(bounds, imgSize, 2, image.Pt(100, 100))
	if v.fitted() {
		t.Fatal("should be zoomed in")
	}
	v.zoomBy(bounds, imgSize, 0.5, image.Pt(100, 100))
	if !v.fitted() {
		t.Fatalf("scale %v should have snapped back to fit", v.scale)
	}
	if v.pan != (image.Point{}) {
		t.Fatalf("fit should recenter, pan = %v", v.pan)
	}
}

func TestZoomViewPanClampedToImageEdges(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 100, 100)
	imgSize := image.Pt(100, 100)
	v.zoomBy(bounds, imgSize, 2, image.Pt(50, 50))

	if !v.panBy(bounds, imgSize, image.Pt(20, 0)) {
		t.Fatal("pan should move a zoomed image")
	}
	v.panBy(bounds, imgSize, image.Pt(10000, 10000))
	disp := v.displayRect(bounds, imgSize)
	if disp.Min.X > bounds.Min.X || disp.Min.Y > bounds.Min.Y {
		t.Fatalf("panning left a gap at the top left: %v", disp)
	}
	if disp.Max.X < bounds.Max.X || disp.Max.Y < bounds.Max.Y {
		t.Fatalf("panning left a gap at the bottom right: %v", disp)
	}
}

func TestZoomViewFittedImageDoesNotPan(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 200, 200)
	imgSize := image.Pt(100, 50)
	if v.canPan(bounds, imgSize) {
		t.Fatal("a fitted image has nothing to pan")
	}
	if v.panBy(bounds, imgSize, image.Pt(10, 10)) {
		t.Fatal("pan should be ignored while fitted")
	}
}

func TestZoomViewClampsScale(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 100, 100)
	imgSize := image.Pt(100, 100)
	for i := 0; i < 40; i++ {
		v.zoomBy(bounds, imgSize, 2, image.Pt(50, 50))
	}
	if v.scale != maxZoomScale {
		t.Fatalf("scale = %v, want the %v cap", v.scale, maxZoomScale)
	}
	for i := 0; i < 40; i++ {
		v.zoomBy(bounds, imgSize, 0.5, image.Pt(50, 50))
	}
	if !v.fitted() && v.scale < minZoomScale {
		t.Fatalf("scale = %v, want at least %v", v.scale, minZoomScale)
	}
}

func TestZoomViewHugeImageZoomsOutToFit(t *testing.T) {
	var v zoomView
	bounds := image.Rect(0, 0, 100, 100)
	imgSize := image.Pt(40000, 40000) // fit scale is below minZoomScale
	v.zoomBy(bounds, imgSize, 4, image.Pt(50, 50))
	for i := 0; i < 10; i++ {
		v.zoomBy(bounds, imgSize, 0.5, image.Pt(50, 50))
	}
	if !v.fitted() {
		t.Fatalf("scale %v should reach fit for an oversized image", v.scale)
	}
}

func TestWheelZoomFactorDirectionAndCap(t *testing.T) {
	// Delta.Y is negative when the wheel turns away from the user.
	if wheelZoomFactor(-1) <= 1 {
		t.Fatal("scrolling up should zoom in")
	}
	if wheelZoomFactor(1) >= 1 {
		t.Fatal("scrolling down should zoom out")
	}
	if got := wheelZoomFactor(-50); got != wheelZoomFactor(-3) {
		t.Fatalf("large trackpad deltas should be capped, got %v", got)
	}
}
