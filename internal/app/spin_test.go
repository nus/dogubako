package app

import (
	"testing"
	"time"

	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/uitest"
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

func TestSpinPressStepsImmediately(t *testing.T) {
	n := 0
	b := newSpinButtons(nil, nil, func() { n++ }, nil)
	b.SetBounds(geometry.NewRect(0, 0, 16, 28))
	ctx := uitest.NewMockContext()
	if !b.Event(ctx, uitest.Click(8, 6)) {
		t.Fatal("press should be consumed")
	}
	if n != 1 {
		t.Fatalf("press steps = %d, want 1", n)
	}
	if b.pressed != spinUp {
		t.Fatal("should stay pressed until release")
	}
	if !b.Event(ctx, uitest.Release(8, 6)) {
		t.Fatal("release should be consumed")
	}
	if n != 1 {
		t.Fatalf("release must not step again, got %d", n)
	}
	if b.pressed != spinNone {
		t.Fatal("released")
	}
}

func TestSpinMaybeRepeatAfterDelay(t *testing.T) {
	n := 0
	b := newSpinButtons(nil, nil, func() { n++ }, nil)
	t0 := time.Unix(1, 0)
	b.pressed = spinUp
	b.pressedAt = t0
	b.lastStep = t0

	if b.maybeRepeat(t0.Add(100 * time.Millisecond)) {
		t.Fatal("should wait for initial delay")
	}
	if n != 0 {
		t.Fatalf("early steps = %d", n)
	}
	if !b.maybeRepeat(t0.Add(spinInitialDelay)) {
		t.Fatal("first repeat at initial delay")
	}
	if n != 1 {
		t.Fatalf("after delay = %d", n)
	}
	if b.maybeRepeat(t0.Add(spinInitialDelay + spinRepeatEvery/2)) {
		t.Fatal("should wait for repeat interval")
	}
	if !b.maybeRepeat(t0.Add(spinInitialDelay + spinRepeatEvery)) {
		t.Fatal("second repeat")
	}
	if n != 2 {
		t.Fatalf("got %d", n)
	}

	b.pressed = spinNone
	if b.maybeRepeat(t0.Add(10 * time.Second)) {
		t.Fatal("stopped")
	}
}
