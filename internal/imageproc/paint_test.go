package imageproc

import (
	"image"
	"image/color"
	"testing"
)

func TestClampBrushSize(t *testing.T) {
	if got := ClampBrushSize(0); got != 1 {
		t.Fatalf("zero = %d", got)
	}
	if got := ClampBrushSize(MaxBrushSize + 8); got != MaxBrushSize {
		t.Fatalf("over = %d", got)
	}
	if got := ClampBrushSize(7); got != 7 {
		t.Fatalf("ok = %d", got)
	}
}

func TestStampBrushPaintsCircle(t *testing.T) {
	dst := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	c := color.NRGBA{R: 255, A: 255}
	StampBrush(dst, image.Pt(10, 10), 3, c, false)
	if got := dst.NRGBAAt(10, 10); got != c {
		t.Fatalf("center = %+v", got)
	}
	if got := dst.NRGBAAt(10, 13); got != c {
		t.Fatalf("edge = %+v", got)
	}
	if got := dst.NRGBAAt(0, 0); got.A != 0 {
		t.Fatalf("outside should stay empty, got %+v", got)
	}
}

func TestStampBrushErase(t *testing.T) {
	dst := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	red := color.NRGBA{R: 200, A: 255}
	StampBrush(dst, image.Pt(4, 4), 2, red, false)
	StampBrush(dst, image.Pt(4, 4), 2, color.NRGBA{}, true)
	if got := dst.NRGBAAt(4, 4); got.A != 0 {
		t.Fatalf("erased pixel = %+v", got)
	}
}

func TestStrokeBrushConnectsPoints(t *testing.T) {
	dst := image.NewNRGBA(image.Rect(0, 0, 30, 10))
	c := color.NRGBA{G: 255, A: 255}
	StrokeBrush(dst, image.Pt(2, 5), image.Pt(20, 5), 1, c, false)
	if got := dst.NRGBAAt(2, 5); got != c {
		t.Fatalf("start = %+v", got)
	}
	if got := dst.NRGBAAt(11, 5); got != c {
		t.Fatalf("mid = %+v", got)
	}
	if got := dst.NRGBAAt(20, 5); got != c {
		t.Fatalf("end = %+v", got)
	}
}

func TestCompositeOverBlendsOverlay(t *testing.T) {
	base := solidNRGBA(4, 4, color.NRGBA{B: 255, A: 255})
	over := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	over.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	got := CompositeOver(base, over)
	if c := got.NRGBAAt(0, 0); c.B < 250 || c.A < 250 {
		t.Fatalf("uncovered = %+v", c)
	}
	if c := got.NRGBAAt(1, 1); c != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("overlay = %+v", c)
	}
	if c := base.NRGBAAt(1, 1); c.R != 0 {
		t.Fatal("base should not be mutated")
	}
}

func TestCloneNRGBAIndependent(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{G: 10, A: 255})
	got := CloneNRGBA(src)
	got.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	if src.NRGBAAt(0, 0).R != 0 {
		t.Fatal("clone should not alias pix")
	}
}
