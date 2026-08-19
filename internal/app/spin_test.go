package app

import (
	"testing"

	"github.com/gogpu/ui/geometry"
)

func TestStepIntValueClamps(t *testing.T) {
	if got := stepIntValue("10", 1, 20, 1); got != 11 {
		t.Fatalf("inc = %d", got)
	}
	if got := stepIntValue("20", 1, 20, 1); got != 20 {
		t.Fatalf("inc at max = %d", got)
	}
	if got := stepIntValue("1", 1, 20, -1); got != 1 {
		t.Fatalf("dec at min = %d", got)
	}
	if got := stepIntValue(" 5 ", 0, 10, -1); got != 4 {
		t.Fatalf("trim = %d", got)
	}
	if got := stepIntValue("x", 3, 9, 1); got != 4 {
		t.Fatalf("invalid starts at min = %d", got)
	}
}

func TestSpinButtonsHalfAt(t *testing.T) {
	b := newSpinButtons(nil, nil, nil, nil)
	b.SetBounds(geometry.NewRect(10, 20, 16, 28))
	if b.halfAt(geometry.Pt(18, 26)) != spinUp {
		t.Fatal("top half should be up")
	}
	if b.halfAt(geometry.Pt(18, 40)) != spinDown {
		t.Fatal("bottom half should be down")
	}
	if b.halfAt(geometry.Pt(0, 0)) != spinNone {
		t.Fatal("outside")
	}
}
