package app

import (
	"strings"
	"time"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"
)

const (
	spinBtnW          float32 = 16
	spinInitialDelay          = 400 * time.Millisecond
	spinRepeatEvery           = 50 * time.Millisecond
)

type spinHalf int

const (
	spinNone spinHalf = iota
	spinUp
	spinDown
)

// spinButtons is a compact up/down stepper for integer fields.
type spinButtons struct {
	widget.WidgetBase
	shell    *Shell
	disabled func() bool
	onUp     func()
	onDown   func()
	hover     spinHalf
	pressed   spinHalf
	pressedAt time.Time
	lastStep  time.Time
}

func newSpinButtons(s *Shell, disabled func() bool, onUp, onDown func()) *spinButtons {
	b := &spinButtons{shell: s, disabled: disabled, onUp: onUp, onDown: onDown}
	b.SetVisible(true)
	b.SetEnabled(true)
	return b
}

func (b *spinButtons) inactive() bool {
	return b.disabled != nil && b.disabled()
}

func (b *spinButtons) halfAt(pt geometry.Point) spinHalf {
	r := b.Bounds()
	if !r.Contains(pt) {
		return spinNone
	}
	if pt.Y < r.Min.Y+r.Height()/2 {
		return spinUp
	}
	return spinDown
}

func (b *spinButtons) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.Constrain(geometry.Sz(spinBtnW, fieldH))
	if size.Height < fieldH && cons.MaxHeight >= fieldH {
		size.Height = fieldH
	}
	b.SetBounds(geometry.FromPointSize(b.Position(), size))
	return size
}

func (b *spinButtons) Draw(ctx widget.Context, canvas widget.Canvas) {
	r := b.Bounds()
	if r.IsEmpty() {
		return
	}
	inactive := b.inactive()
	bg := widget.RGBA8(232, 236, 242, 255)
	fg := widget.RGBA8(70, 70, 70, 255)
	border := widget.RGBA8(200, 206, 216, 255)
	if inactive {
		bg = widget.RGBA8(240, 242, 245, 255)
		fg = widget.RGBA8(170, 170, 170, 255)
		border = widget.RGBA8(220, 224, 230, 255)
	}
	canvas.DrawRoundRect(r, bg, 4)
	canvas.StrokeRoundRect(r, border, 4, 1)

	midY := r.Min.Y + r.Height()/2
	up := geometry.NewRect(r.Min.X, r.Min.Y, r.Width(), r.Height()/2)
	down := geometry.NewRect(r.Min.X, midY, r.Width(), r.Max.Y-midY)
	if !inactive {
		paintSpinHalf(canvas, up, b.hover == spinUp, b.pressed == spinUp)
		paintSpinHalf(canvas, down, b.hover == spinDown, b.pressed == spinDown)
	}
	canvas.DrawLine(geometry.Pt(r.Min.X+3, midY), geometry.Pt(r.Max.X-3, midY), border, 1)
	drawSpinChevron(canvas, up, true, fg)
	drawSpinChevron(canvas, down, false, fg)
	b.tickRepeat(ctx)
}

func paintSpinHalf(canvas widget.Canvas, r geometry.Rect, hover, pressed bool) {
	if pressed {
		canvas.DrawRect(r, widget.RGBA8(200, 208, 220, 255))
		return
	}
	if hover {
		canvas.DrawRect(r, widget.RGBA8(220, 226, 235, 255))
	}
}

func drawSpinChevron(canvas widget.Canvas, r geometry.Rect, up bool, color widget.Color) {
	if r.IsEmpty() {
		return
	}
	cx := r.Min.X + r.Width()/2
	cy := r.Min.Y + r.Height()/2
	w := r.Width() * 0.22
	h := r.Height() * 0.22
	if w < 2 {
		w = 2
	}
	if h < 1.5 {
		h = 1.5
	}
	if up {
		canvas.DrawLine(geometry.Pt(cx-w, cy+h*0.35), geometry.Pt(cx, cy-h*0.55), color, 1.4)
		canvas.DrawLine(geometry.Pt(cx, cy-h*0.55), geometry.Pt(cx+w, cy+h*0.35), color, 1.4)
		return
	}
	canvas.DrawLine(geometry.Pt(cx-w, cy-h*0.35), geometry.Pt(cx, cy+h*0.55), color, 1.4)
	canvas.DrawLine(geometry.Pt(cx, cy+h*0.55), geometry.Pt(cx+w, cy-h*0.35), color, 1.4)
}

func (b *spinButtons) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	if b.pressed != spinNone {
		switch me.MouseType {
		case event.MouseMove, event.MouseDrag:
			return true
		case event.MouseRelease:
			if me.Button == event.ButtonLeft {
				b.stopRepeat(ctx)
				return true
			}
		}
	}
	half := b.halfAt(me.Position)
	switch me.MouseType {
	case event.MouseMove:
		if b.hover != half {
			b.hover = half
			b.SetNeedsRedraw(true)
		}
		if half != spinNone {
			if b.inactive() {
				ctx.SetCursor(widget.CursorDefault)
			} else {
				ctx.SetCursor(widget.CursorPointer)
			}
			return true
		}
	case event.MousePress:
		if half != spinNone && me.Button == event.ButtonLeft && !b.inactive() {
			b.pressed = half
			now := time.Now()
			b.pressedAt = now
			b.lastStep = now
			if cap, ok := ctx.(widget.PointerCapturer); ok {
				cap.CapturePointer(b)
			}
			b.fire()
			b.SetNeedsRedraw(true)
			b.scheduleRepeat(ctx)
			return true
		}
	}
	return false
}

func (b *spinButtons) fire() {
	if b.inactive() {
		return
	}
	if b.pressed == spinUp && b.onUp != nil {
		b.onUp()
	}
	if b.pressed == spinDown && b.onDown != nil {
		b.onDown()
	}
}

func (b *spinButtons) maybeRepeat(now time.Time) bool {
	if b.pressed == spinNone {
		return false
	}
	if now.Sub(b.pressedAt) < spinInitialDelay {
		return false
	}
	if now.Sub(b.lastStep) < spinRepeatEvery {
		return false
	}
	b.fire()
	b.lastStep = now
	return true
}

func (b *spinButtons) tickRepeat(ctx widget.Context) {
	if b.pressed == spinNone || ctx == nil {
		return
	}
	if !b.inactive() {
		b.maybeRepeat(time.Now())
	}
	b.SetNeedsRedraw(true)
	b.scheduleRepeat(ctx)
}

func (b *spinButtons) scheduleRepeat(ctx widget.Context) {
	if ctx != nil {
		if sched, ok := ctx.(widget.AnimationScheduler); ok {
			sched.ScheduleAnimationFrame()
		} else {
			ctx.InvalidateRect(b.Bounds())
		}
	}
	if b.shell != nil && b.shell.gpu != nil {
		b.shell.gpu.RequestRedraw()
	}
}

func (b *spinButtons) stopRepeat(ctx widget.Context) {
	b.pressed = spinNone
	b.SetNeedsRedraw(true)
	if ctx == nil {
		return
	}
	if cap, ok := ctx.(widget.PointerCapturer); ok {
		cap.ReleasePointer(b)
	}
}

func (b *spinButtons) Children() []widget.Widget { return nil }

func (b *spinButtons) Mount(ctx widget.Context) {
	if b.shell == nil {
		return
	}
	sched := ctx.Scheduler()
	if sched == nil {
		return
	}
	b.AddBinding(state.BindToScheduler(b.shell.rev, b, sched))
}

func (b *spinButtons) Unmount() {
	b.pressed = spinNone
}

func stepIntValue(cur string, minV, maxV, delta int) int {
	n, ok := parseInt(strings.TrimSpace(cur))
	if !ok {
		n = minV
	}
	return clampInt(n+delta, minV, maxV)
}

func clampInt(n, minV, maxV int) int {
	if n < minV {
		return minV
	}
	if n > maxV {
		return maxV
	}
	return n
}
