package app

import (
	"math"

	"github.com/gogpu/ui/core/slider"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

const hSliderThumbR float32 = 10

// hSlider is a horizontal slider that keeps dragging on the cursor after
// CapturePointer. gogpu's slider maps captured window coordinates through
// widget-local Bounds, so the thumb sticks near the default (90%).
type hSlider struct {
	widget.WidgetBase
	shell    *Shell
	value    state.Signal[float32]
	min, max float32
	step     float32
	disabled func() bool
	onChange func(float32)
	painter  slider.Painter
	dragging bool
	hover    bool
}

func newHSlider(s *Shell, value state.Signal[float32], minV, maxV, step float32, disabled func() bool, onChange func(float32)) *hSlider {
	h := &hSlider{
		shell:    s,
		value:    value,
		min:      minV,
		max:      maxV,
		step:     step,
		disabled: disabled,
		onChange: onChange,
	}
	if s != nil && s.theme != nil {
		h.painter = material3.SliderPainter{Theme: s.theme}
	} else {
		h.painter = slider.DefaultPainter{}
	}
	h.SetVisible(true)
	h.SetEnabled(true)
	return h
}

func (h *hSlider) inactive() bool {
	return h.disabled != nil && h.disabled()
}

func (h *hSlider) current() float32 {
	if h.value != nil {
		return h.value.Get()
	}
	return h.min
}

func (h *hSlider) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	w := cons.MaxWidth
	if !cons.HasBoundedWidth() || w <= 0 || w >= geometry.Infinity {
		w = fieldWidth
	}
	size := cons.Constrain(geometry.Sz(w, fieldH))
	h.SetBounds(geometry.FromPointSize(h.Position(), size))
	return size
}

func (h *hSlider) Draw(_ widget.Context, canvas widget.Canvas) {
	b := h.Bounds()
	if b.IsEmpty() {
		return
	}
	v := h.current()
	span := h.max - h.min
	progress := float32(0)
	if span > 0 {
		progress = (v - h.min) / span
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
	}
	h.painter.PaintSlider(canvas, slider.PaintState{
		Value:       v,
		Min:         h.min,
		Max:         h.max,
		Progress:    progress,
		Hovered:     h.hover && !h.inactive(),
		Dragging:    h.dragging,
		Disabled:    h.inactive(),
		Bounds:      b,
		Orientation: slider.Horizontal,
	})
}

func (h *hSlider) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	pos := h.localMouse(me)
	switch me.MouseType {
	case event.MouseMove, event.MouseDrag:
		if h.dragging {
			if h.inactive() {
				return true
			}
			h.applyX(ctx, pos.X)
			ctx.SetCursor(widget.CursorPointer)
			return true
		}
		inside := h.Bounds().Contains(pos)
		if h.hover != inside {
			h.hover = inside
			h.SetNeedsRedraw(true)
		}
		if inside {
			if h.inactive() {
				ctx.SetCursor(widget.CursorDefault)
			} else {
				ctx.SetCursor(widget.CursorPointer)
			}
			return true
		}
	case event.MousePress:
		if me.Button != event.ButtonLeft || h.inactive() {
			return false
		}
		if !h.Bounds().Contains(pos) {
			return false
		}
		h.dragging = true
		if cap, ok := ctx.(widget.PointerCapturer); ok {
			cap.CapturePointer(h)
		}
		h.applyX(ctx, pos.X)
		ctx.SetCursor(widget.CursorPointer)
		return true
	case event.MouseRelease:
		if !h.dragging || me.Button != event.ButtonLeft {
			return false
		}
		h.dragging = false
		if cap, ok := ctx.(widget.PointerCapturer); ok {
			cap.ReleasePointer(h)
		}
		if !h.inactive() {
			h.applyX(ctx, pos.X)
		}
		h.hover = h.Bounds().Contains(pos)
		h.SetNeedsRedraw(true)
		return true
	}
	return false
}

func (h *hSlider) localMouse(me *event.MouseEvent) geometry.Point {
	pos := me.Position
	if h.dragging && h.IsScreenOriginValid() {
		pos = pos.Sub(h.ScreenOrigin()).Add(h.Bounds().Min)
	}
	return pos
}

func (h *hSlider) applyX(ctx widget.Context, x float32) {
	v := sliderValueAtX(x, h.Bounds(), h.min, h.max, h.step)
	cur := h.current()
	if v == cur {
		return
	}
	if h.value != nil {
		h.value.Set(v)
	}
	if h.onChange != nil {
		h.onChange(v)
	}
	h.SetNeedsRedraw(true)
	if ctx != nil {
		ctx.InvalidateRect(h.Bounds())
	}
	if h.shell != nil && h.shell.gpu != nil {
		h.shell.gpu.RequestRedraw()
	}
}

func sliderValueAtX(x float32, bounds geometry.Rect, minV, maxV, step float32) float32 {
	span := maxV - minV
	if span <= 0 {
		return minV
	}
	left := bounds.Min.X + hSliderThumbR
	right := bounds.Max.X - hSliderThumbR
	width := right - left
	if width <= 0 {
		return minV
	}
	progress := (x - left) / width
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return clampSliderStep(minV+progress*span, minV, maxV, step)
}

func clampSliderStep(val, minV, maxV, step float32) float32 {
	if val < minV {
		val = minV
	}
	if val > maxV {
		val = maxV
	}
	if step > 0 {
		offset := val - minV
		steps := float32(math.Round(float64(offset / step)))
		val = minV + steps*step
		if val < minV {
			val = minV
		}
		if val > maxV {
			val = maxV
		}
	}
	return val
}
