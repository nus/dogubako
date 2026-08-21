package app

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/uitest"
)

func TestFitScaleAndViewRectMatchFittedRect(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 100)
	imgSize := image.Pt(100, 100)
	var cam previewCam
	cam.resetFit()
	got := cam.viewRect(bounds, imgSize)
	want := fittedRect(bounds, imgSize)
	if got != want {
		t.Fatalf("fit view = %v, want %v", got, want)
	}
	if math.Abs(fitScale(bounds, imgSize)-1) > 1e-9 {
		t.Fatalf("fit scale = %v", fitScale(bounds, imgSize))
	}
}

func TestZoomByKeepsPivot(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 100)
	imgSize := image.Pt(100, 100)
	var cam previewCam
	cam.resetFit()
	pivot := image.Pt(100, 50)
	before := screenToImage(pivot, cam.viewRect(bounds, imgSize), imgSize)
	cam.zoomBy(bounds, imgSize, 2, pivot)
	after := screenToImage(pivot, cam.viewRect(bounds, imgSize), imgSize)
	if after != before {
		t.Fatalf("pivot image point %v → %v", before, after)
	}
	if cam.fitting() {
		t.Fatal("expected explicit zoom")
	}
	if p := cam.percent(bounds, imgSize); p != 200 {
		t.Fatalf("percent = %d", p)
	}
}

func TestClampPanCentersSmallAndClampsLarge(t *testing.T) {
	bounds := image.Rect(10, 20, 110, 80)
	x, y := clampPan(0, 0, bounds, 40, 20)
	if x != 10+(100-40)/2 || y != 20+(60-20)/2 {
		t.Fatalf("centered = %v,%v", x, y)
	}
	x, y = clampPan(-1000, 500, bounds, 400, 200)
	if x != 10+100-400 || y != 20 {
		t.Fatalf("clamped = %v,%v", x, y)
	}
}

func TestZoomFactorFromWheel(t *testing.T) {
	if got := zoomFactorFromWheel(0); got != 1 {
		t.Fatalf("zero = %v", got)
	}
	in := zoomFactorFromWheel(-1)
	if math.Abs(in-previewZoomStep) > 1e-9 {
		t.Fatalf("scroll up factor = %v", in)
	}
	out := zoomFactorFromWheel(1)
	if math.Abs(out-1/previewZoomStep) > 1e-9 {
		t.Fatalf("scroll down factor = %v", out)
	}
}

func TestSourcePreviewWheelZooms(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(40, 40))
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	if !p.ZoomFitting() {
		t.Fatal("start at fit")
	}
	if !p.Event(nil, uitest.WheelScroll(40, 40, -1)) {
		t.Fatal("wheel should be handled")
	}
	if p.ZoomFitting() {
		t.Fatal("wheel should leave fit")
	}
	if p.ZoomPercent() <= 100 {
		t.Fatalf("zoom in percent = %d", p.ZoomPercent())
	}
	p.ZoomFit()
	if !p.ZoomFitting() {
		t.Fatal("fit")
	}
}

func TestDestPreviewWheelAndPan(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 80, 80))
	p := newDestPreview(nil)
	p.SetImage(src)
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	p.ZoomIn()
	p.ZoomIn()
	if p.ZoomFitting() {
		t.Fatal("zoomed")
	}
	start := p.cam.panX
	press := event.NewMouseEvent(
		event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(40, 40), geometry.Pt(40, 40), event.ModNone,
	)
	if !p.Event(nil, press) {
		t.Fatal("press")
	}
	drag := event.NewMouseEvent(
		event.MouseDrag, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(30, 40), geometry.Pt(30, 40), event.ModNone,
	)
	if !p.Event(nil, drag) {
		t.Fatal("drag")
	}
	if p.cam.panX == start && p.cam.canPan(toImageRect(p.Bounds()), image.Pt(80, 80)) {
		t.Fatal("pan should move")
	}
}

func TestDrawViewImageClipsToViewport(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var cam previewCam
	cam.fit = false
	cam.scale = 4
	cam.panX, cam.panY = 0, 0
	bounds := image.Rect(0, 0, 100, 80)
	view := cam.viewRect(bounds, image.Pt(400, 400))
	if view.Dx() < 400 {
		t.Fatalf("zoomed view width = %d", view.Dx())
	}
	canvas := &uitest.MockCanvas{}
	drawViewImage(canvas, src, view, image.Pt(400, 400), image.Rectangle{}, bounds)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d", len(canvas.Images))
	}
	got := canvas.Images[0].Image.Bounds().Size()
	if got.X > 100 || got.Y > 80 {
		t.Fatalf("rasterized %v, should stay within viewport", got)
	}
}

func TestImageRectFromViewRoundTrip(t *testing.T) {
	view := image.Rect(10, 20, 110, 70)
	imgSize := image.Pt(200, 100)
	screen := image.Rect(10, 20, 60, 45)
	got := imageRectFromView(view, imgSize, screen)
	if got.Min.X != 0 || got.Min.Y != 0 {
		t.Fatalf("top-left mapped to %v", got)
	}
	if got.Dx() < 1 || got.Dy() < 1 {
		t.Fatalf("empty mapping %v", got)
	}
}

func TestDestPreviewProviderZoomPercent(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 50, 25))
	p := newDestPreview(state.NewSignal[uint64](1))
	p.SetProvider(func() image.Image { return src })
	p.SetBounds(geometry.NewRect(0, 0, 200, 100))
	if !p.HasPreview() {
		t.Fatal("provider image")
	}
	// 50×25 fitted into 200×100 is 400%.
	if p.ZoomPercent() != 400 {
		t.Fatalf("fit percent = %d", p.ZoomPercent())
	}
	p.ZoomOut()
	if p.ZoomFitting() {
		t.Fatal("zoom out should leave fit")
	}
}

func TestSourcePreviewPanWithRightDrag(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(40, 40))
	p.SetBounds(geometry.NewRect(0, 0, 40, 40))
	p.ZoomIn()
	p.ZoomIn()
	press := event.NewMouseEvent(
		event.MousePress, event.ButtonRight, event.ButtonStateRight,
		geometry.Pt(20, 20), geometry.Pt(20, 20), event.ModNone,
	)
	if !p.Event(nil, press) {
		t.Fatal("right press should pan")
	}
	if p.dragging {
		t.Fatal("right drag must not start a crop")
	}
}
