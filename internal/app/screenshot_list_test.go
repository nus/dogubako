package app

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/uitest"

	"github.com/nus/dogubako/internal/userdir"
)

func TestScreenshotListGeometry(t *testing.T) {
	sz := screenshotListContentSize(3, 200)
	if sz != (geometry.Size{Width: 200, Height: 3 * screenshotRowH}) {
		t.Fatalf("content size = %+v", sz)
	}
	row := screenshotRowRect(2, 200)
	if row.Min.Y != 2*screenshotRowH || row.Height() != screenshotRowH {
		t.Fatalf("row = %+v", row)
	}
	thumb := screenshotThumbRect(row)
	if thumb.Min.X != screenshotThumbPad {
		t.Fatalf("thumb x = %v", thumb.Min.X)
	}
	if thumb.Min.Y < row.Min.Y || thumb.Max.Y > row.Max.Y {
		t.Fatalf("thumb %v outside row %v", thumb, row)
	}
	if thumb.Width() != screenshotThumbSize || thumb.Height() != screenshotThumbSize {
		t.Fatalf("thumb size = %v x %v", thumb.Width(), thumb.Height())
	}
	name := screenshotNameRect(row)
	if name.Min.X < thumb.Max.X {
		t.Fatalf("name overlaps thumb: name=%v thumb=%v", name, thumb)
	}
	if name.Max.X > row.Max.X {
		t.Fatalf("name overflows row: name=%v row=%v", name, row)
	}
}

func TestScreenshotHitIndex(t *testing.T) {
	if got := screenshotHitIndex(-1, 3); got != -1 {
		t.Fatalf("negative = %d", got)
	}
	if got := screenshotHitIndex(0, 3); got != 0 {
		t.Fatalf("first = %d", got)
	}
	if got := screenshotHitIndex(screenshotRowH, 3); got != 1 {
		t.Fatalf("second = %d", got)
	}
	if got := screenshotHitIndex(screenshotRowH*3, 3); got != -1 {
		t.Fatalf("past last = %d", got)
	}
	if got := screenshotHitIndex(10, 0); got != -1 {
		t.Fatalf("empty = %d", got)
	}
}

func TestScreenshotScrollToShow(t *testing.T) {
	if got := screenshotScrollToShow(0, 10, 100, 40); got != 0 {
		t.Fatalf("above viewport = %v", got)
	}
	if got := screenshotScrollToShow(1, 10, 100, 0); got != 0 {
		t.Fatalf("already visible = %v", got)
	}
	got := screenshotScrollToShow(5, 10, 96, 0)
	want := 6*screenshotRowH - 96
	if got != want {
		t.Fatalf("below viewport = %v, want %v", got, want)
	}
}

func TestScreenshotVisibleRange(t *testing.T) {
	start, end := screenshotVisibleRange(20, 0, screenshotRowH*3)
	if start != 0 || end < 3 || end > 6 {
		t.Fatalf("range = %d,%d", start, end)
	}
	start, end = screenshotVisibleRange(20, screenshotRowH*10, screenshotRowH*2)
	if start < 9 || start > 10 {
		t.Fatalf("scrolled start = %d", start)
	}
	if end <= start {
		t.Fatalf("scrolled end = %d start = %d", end, start)
	}
}

func TestScreenshotFileIndex(t *testing.T) {
	files := []ScreenshotFile{{Path: "/a.png"}, {Path: "/b.png"}}
	if got := screenshotFileIndex(files, "/b.png"); got != 1 {
		t.Fatalf("index = %d", got)
	}
	if got := screenshotFileIndex(files, "/missing.png"); got != -1 {
		t.Fatalf("missing = %d", got)
	}
}

func TestScreenshotListJumpsToSelected(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	s := &Shell{shotSel: state.NewSignal(-1), rev: state.NewSignal[uint64](0)}
	shot := s.model.Screenshot()
	if err := shot.ApplyCapture(src); err != nil {
		t.Fatal(err)
	}
	l := newScreenshotFileList(s)
	l.SetBounds(geometry.NewRect(0, 0, 200, screenshotRowH))
	l.syncSelection(screenshotRowH)
	if s.shotSel.Get() != 0 {
		t.Fatalf("selected index = %d", s.shotSel.Get())
	}
	if shot.TakeJumpToSelected() {
		t.Fatal("jump flag should be consumed")
	}
}

func TestScreenshotListDrawsThumbInsideRowSlot(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	src := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	s := &Shell{shotSel: state.NewSignal(-1), rev: state.NewSignal[uint64](0)}
	if err := s.model.Screenshot().ApplyCapture(src); err != nil {
		t.Fatal(err)
	}

	body := &screenshotListBody{list: &screenshotFileList{shell: s, scrollY: state.NewSignal[float32](0)}, hover: -1}
	body.SetBounds(geometry.NewRect(0, 0, 240, screenshotRowH*2))
	canvas := &uitest.MockCanvas{}
	body.Draw(nil, canvas)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d, want 1", len(canvas.Images))
	}
	at := canvas.Images[0].At
	thumb := screenshotThumbRect(screenshotRowRect(0, 240))
	if at.X < thumb.Min.X || at.Y < thumb.Min.Y || at.X >= thumb.Max.X || at.Y >= thumb.Max.Y {
		t.Fatalf("thumb drawn at %+v, want inside %+v", at, thumb)
	}
}
