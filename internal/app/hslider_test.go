package app

import (
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"

	"github.com/nus/dogubako/internal/i18n"
)

func TestSliderValueAtX(t *testing.T) {
	b := geometry.NewRect(0, 0, 120, 28)
	if got := sliderValueAtX(hSliderThumbR, b, 1, 100, 1); got != 1 {
		t.Fatalf("min = %v", got)
	}
	if got := sliderValueAtX(120-hSliderThumbR, b, 1, 100, 1); got != 100 {
		t.Fatalf("max = %v", got)
	}
	mid := hSliderThumbR + (120-2*hSliderThumbR)*0.5
	if got := sliderValueAtX(mid, b, 1, 100, 1); got < 50 || got > 51 {
		t.Fatalf("mid = %v", got)
	}
}

func TestHSliderLocalMouseUsesScreenOriginWhileDragging(t *testing.T) {
	sig := state.NewSignal[float32](90)
	h := newHSlider(nil, sig, 1, 100, 1, nil, nil)
	h.SetBounds(geometry.NewRect(0, 0, 120, 28))
	h.SetScreenOrigin(geometry.Pt(800, 500))

	press := event.NewMouseEvent(
		event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(20, 14), geometry.Pt(20, 14), event.ModNone,
	)
	if got := h.localMouse(press); got != (geometry.Pt(20, 14)) {
		t.Fatalf("press (tree-local) = %v", got)
	}

	h.dragging = true
	move := event.NewMouseEvent(
		event.MouseMove, event.ButtonNone, event.ButtonStateLeft,
		geometry.Pt(800+20, 500+14), geometry.Pt(800+20, 500+14), event.ModNone,
	)
	if got := h.localMouse(move); got != (geometry.Pt(20, 14)) {
		t.Fatalf("captured window coords = %v, want (20,14)", got)
	}
}

func TestHSliderApplyXUpdatesValue(t *testing.T) {
	sig := state.NewSignal[float32](90)
	var got float32
	h := newHSlider(nil, sig, 1, 100, 1, nil, func(v float32) { got = v })
	h.SetBounds(geometry.NewRect(0, 0, 120, 28))
	h.applyX(nil, 120-hSliderThumbR)
	if sig.Get() != 100 || got != 100 {
		t.Fatalf("value=%v onChange=%v", sig.Get(), got)
	}
}

func TestComputedTracksRev(t *testing.T) {
	s := &Shell{rev: state.NewSignal[uint64](0)}
	s.quality = state.NewSignal[float32](90)
	label := s.computed(func() string {
		return i18n.T(i18n.JA, i18n.JPEGQuality, int(s.quality.Get()+0.5))
	})
	if got := label.Get(); got != "JPEG 品質  90" {
		t.Fatalf("initial = %q", got)
	}
	s.quality.Set(40)
	s.rev.Set(1)
	if got := label.Get(); got != "JPEG 品質  40" {
		t.Fatalf("after rev = %q", got)
	}
}
