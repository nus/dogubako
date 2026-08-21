package app

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/uitest"
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

func TestDestPreviewWheelZoomsAndDoubleClickResets(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	p := newDestPreview(nil)
	p.SetImage(src)
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	we := event.NewWheelEvent(geometry.Pt(0, -1), geometry.Pt(40, 40), geometry.Pt(40, 40), event.ModNone)
	if !p.Event(nil, we) {
		t.Fatal("wheel should be handled")
	}
	if p.view.Scale() <= 1 {
		t.Fatalf("scale = %v, want zoom in", p.view.Scale())
	}
	dbl := event.NewMouseEvent(
		event.MouseDoubleClick, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(40, 40), geometry.Pt(40, 40), event.ModNone,
	)
	if !p.Event(nil, dbl) {
		t.Fatal("double-click should reset")
	}
	if p.view.Scale() != 1 {
		t.Fatalf("after reset scale = %v", p.view.Scale())
	}
}

func TestDestPreviewDragPansWhenZoomed(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	p := newDestPreview(nil)
	p.SetImage(src)
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	p.view.scale = 4
	press := event.NewMouseEvent(
		event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(40, 40), geometry.Pt(40, 40), event.ModNone,
	)
	if !p.Event(nil, press) {
		t.Fatal("press should start pan")
	}
	drag := event.NewMouseEvent(
		event.MouseDrag, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(55, 40), geometry.Pt(55, 40), event.ModNone,
	)
	if !p.Event(nil, drag) {
		t.Fatal("drag should pan")
	}
	if p.view.panX == 0 {
		t.Fatal("expected horizontal pan")
	}
}

func TestSourcePreviewWheelZooms(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(40, 40))
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	we := event.NewWheelEvent(geometry.Pt(0, -1), geometry.Pt(40, 40), geometry.Pt(40, 40), event.ModNone)
	if !p.Event(nil, we) {
		t.Fatal("wheel should be handled")
	}
	if p.view.Scale() <= 1 {
		t.Fatalf("scale = %v", p.view.Scale())
	}
}

func TestSourcePreviewCropStillWorksWhenZoomed(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(100, 100))
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	p.view.scale = 2
	var got image.Rectangle
	p.OnCropChanged(func(r image.Rectangle) { got = r })

	press := event.NewMouseEvent(
		event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(50, 50), geometry.Pt(50, 50), event.ModNone,
	)
	if !p.Event(nil, press) {
		t.Fatal("press should start crop")
	}
	drag := event.NewMouseEvent(
		event.MouseDrag, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(70, 70), geometry.Pt(70, 70), event.ModNone,
	)
	if !p.Event(nil, drag) {
		t.Fatal("drag should update crop")
	}
	release := event.NewMouseEvent(
		event.MouseRelease, event.ButtonLeft, 0,
		geometry.Pt(70, 70), geometry.Pt(70, 70), event.ModNone,
	)
	p.Event(nil, release)
	if got.Dx() < 1 || got.Dy() < 1 {
		t.Fatalf("crop = %v", got)
	}
}

func TestDrawViewedImageClipsToBoundsWhenZoomed(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var view previewView
	view.scale = 8
	bounds := geometry.NewRect(0, 0, 40, 40)
	canvas := &uitest.MockCanvas{}
	drawViewedImage(canvas, src, bounds, image.Pt(20, 20), image.Rectangle{}, &view)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d", len(canvas.Images))
	}
	got := canvas.Images[0].Image.Bounds()
	if got.Dx() > 40 || got.Dy() > 40 {
		t.Fatalf("drawn bitmap %v should not exceed widget", got)
	}
	if len(canvas.Clips) != 1 {
		t.Fatalf("clips = %d, want 1", len(canvas.Clips))
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
