package app

import (
	"image"
	"testing"
)

func TestIconInnerIsCenteredSquare(t *testing.T) {
	b := image.Rect(10, 20, 10+48, 20+24)
	got := iconInner(b)
	if got.Dx() != got.Dy() {
		t.Fatalf("icon inner = %v, want square", got)
	}
	cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	gx, gy := (got.Min.X+got.Max.X)/2, (got.Min.Y+got.Max.Y)/2
	if gx != cx || gy != cy {
		t.Fatalf("center = %d,%d want %d,%d (inner %v)", gx, gy, cx, cy, got)
	}
}

func TestIconInnerTooSmall(t *testing.T) {
	if got := iconInner(image.Rect(0, 0, 3, 24)); !got.Empty() {
		t.Fatalf("tiny bounds = %v, want empty", got)
	}
}
