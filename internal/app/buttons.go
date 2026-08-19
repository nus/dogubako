package app

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/i18n"
)

const (
	btnHeight float32 = 28
	btnPadX   float32 = 10
	navHeight float32 = 28
)

func labelWidth(s string) float32 {
	var w float32
	for _, r := range s {
		if r < 0x80 {
			w += 7.4
			continue
		}
		w += 13.5
	}
	return w
}

// cjkButton is a compact action button that draws with the registered CJK
// family. Material painters use Inter-only DrawText, which cannot show Japanese.
type cjkButton struct {
	widget.WidgetBase
	shell    *Shell
	label    func() string
	onClick  func()
	disabled func() bool
	filled   func() bool
	hover    bool
	pressed  bool
}

func (s *Shell) btn(key i18n.Key, onClick func(), filled bool, disabled func() bool) *cjkButton {
	return s.btnFn(func() string { return i18n.T(s.model.Lang(), key) }, onClick, func() bool { return filled }, disabled)
}

func (s *Shell) btnFn(label func() string, onClick func(), filled func() bool, disabled func() bool) *cjkButton {
	if filled == nil {
		filled = func() bool { return false }
	}
	b := &cjkButton{shell: s, label: label, onClick: onClick, disabled: disabled, filled: filled}
	b.SetVisible(true)
	b.SetEnabled(true)
	return b
}

type segItem struct {
	label    func() string
	selected func() bool
	onClick  func()
	disabled func() bool
}

// segBar is a compact segmented control: equal-width toggle buttons in one row.
func (s *Shell) segBar(items ...segItem) widget.Widget {
	children := make([]widget.Widget, 0, len(items))
	for _, it := range items {
		it := it
		children = append(children, primitives.Expanded(s.btnFn(it.label, it.onClick, it.selected, it.disabled)))
	}
	return primitives.HBox(children...).Gap(2)
}

func (b *cjkButton) inactive() bool {
	return b.disabled != nil && b.disabled()
}

func (b *cjkButton) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	label := ""
	if b.label != nil {
		label = b.label()
	}
	w := labelWidth(label) + 2*btnPadX
	if w < 64 {
		w = 64
	}
	size := cons.Constrain(geometry.Sz(w, btnHeight))
	if size.Height < btnHeight && cons.MaxHeight >= btnHeight {
		size.Height = btnHeight
	}
	b.SetBounds(geometry.FromPointSize(b.Position(), size))
	return size
}

func (b *cjkButton) isFilled() bool {
	return b.filled != nil && b.filled()
}

func (b *cjkButton) Draw(_ widget.Context, canvas widget.Canvas) {
	bounds := b.Bounds()
	if bounds.IsEmpty() {
		return
	}
	inactive := b.inactive()
	filled := b.isFilled()
	bg := widget.RGBA8(232, 236, 242, 255)
	fg := widget.RGBA8(33, 33, 33, 255)
	if filled {
		bg = widget.RGBA8(47, 129, 255, 255)
		fg = widget.RGBA8(255, 255, 255, 255)
	}
	if b.hover && !inactive {
		if filled {
			bg = widget.RGBA8(30, 110, 230, 255)
		} else {
			bg = widget.RGBA8(220, 226, 235, 255)
		}
	}
	if b.pressed && !inactive {
		if filled {
			bg = widget.RGBA8(20, 90, 200, 255)
		} else {
			bg = widget.RGBA8(200, 208, 220, 255)
		}
	}
	if inactive {
		bg = widget.RGBA8(240, 242, 245, 255)
		fg = widget.RGBA8(160, 160, 160, 255)
	}
	canvas.DrawRoundRect(bounds, bg, 6)
	if !filled && !inactive {
		canvas.StrokeRoundRect(bounds, widget.RGBA8(200, 206, 216, 255), 6, 1)
	}
	label := ""
	if b.label != nil {
		label = b.label()
	}
	style := widget.TextStyle{
		FontFamily: cjkembed.FamilyName,
		FontSize:   13,
		Color:      fg,
		Align:      widget.TextAlignCenter,
	}
	canvas.PushClip(bounds)
	if sd, ok := canvas.(widget.StyledTextDrawer); ok {
		sd.DrawStyledText(label, bounds, style)
	} else {
		canvas.DrawText(label, bounds, 13, fg, false, widget.TextAlignCenter)
	}
	canvas.PopClip()
}

func (b *cjkButton) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	inside := b.Bounds().Contains(me.Position)
	switch me.MouseType {
	case event.MouseMove:
		if b.hover != inside {
			b.hover = inside
			b.SetNeedsRedraw(true)
		}
		if inside {
			if b.inactive() {
				ctx.SetCursor(widget.CursorDefault)
			} else {
				ctx.SetCursor(widget.CursorPointer)
			}
			return true
		}
	case event.MousePress:
		if inside && me.Button == event.ButtonLeft && !b.inactive() {
			b.pressed = true
			b.SetNeedsRedraw(true)
			return true
		}
	case event.MouseRelease:
		if b.pressed && me.Button == event.ButtonLeft {
			b.pressed = false
			b.SetNeedsRedraw(true)
			if inside && !b.inactive() && b.onClick != nil {
				b.onClick()
			}
			return true
		}
	}
	return false
}

func (b *cjkButton) Children() []widget.Widget { return nil }

func (b *cjkButton) Mount(ctx widget.Context) {
	sched := ctx.Scheduler()
	if sched == nil || b.shell == nil {
		return
	}
	b.AddBinding(state.BindToScheduler(b.shell.rev, b, sched))
}

func (b *cjkButton) Unmount() {}

// navItem is a full-width sidebar row matching the old guigui list style.
type navItem struct {
	widget.WidgetBase
	shell    *Shell
	label    func() string
	selected func() bool
	onClick  func()
	hover    bool
	pressed  bool
}

func (s *Shell) navItem(label func() string, selected func() bool, onClick func()) *navItem {
	n := &navItem{shell: s, label: label, selected: selected, onClick: onClick}
	n.SetVisible(true)
	n.SetEnabled(true)
	return n
}

func (n *navItem) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	w := cons.MaxWidth
	if !cons.HasBoundedWidth() || w > sidebarWidth {
		w = sidebarWidth - 16
	}
	if w < 80 {
		w = 80
	}
	size := cons.Constrain(geometry.Sz(w, navHeight))
	n.SetBounds(geometry.FromPointSize(n.Position(), size))
	return size
}

func (n *navItem) Draw(_ widget.Context, canvas widget.Canvas) {
	bounds := n.Bounds()
	if bounds.IsEmpty() {
		return
	}
	sel := n.selected != nil && n.selected()
	fg := widget.RGBA8(33, 33, 33, 255)
	if sel {
		fg = widget.RGBA8(47, 129, 255, 255)
		canvas.DrawRoundRect(bounds, widget.RGBA8(47, 129, 255, 28), 6)
	} else if n.hover {
		canvas.DrawRoundRect(bounds, widget.RGBA8(0, 0, 0, 16), 6)
	}
	label := ""
	if n.label != nil {
		label = n.label()
	}
	textBounds := geometry.NewRect(bounds.Min.X+10, bounds.Min.Y, bounds.Width()-14, bounds.Height())
	style := widget.TextStyle{
		FontFamily: cjkembed.FamilyName,
		FontSize:   13,
		Color:      fg,
		Align:      widget.TextAlignLeft,
		Bold:       sel,
	}
	canvas.PushClip(bounds)
	if sd, ok := canvas.(widget.StyledTextDrawer); ok {
		sd.DrawStyledText(label, textBounds, style)
	} else {
		canvas.DrawText(label, textBounds, 13, fg, sel, widget.TextAlignLeft)
	}
	canvas.PopClip()
}

func (n *navItem) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	inside := n.Bounds().Contains(me.Position)
	switch me.MouseType {
	case event.MouseMove:
		if n.hover != inside {
			n.hover = inside
			n.SetNeedsRedraw(true)
		}
		if inside {
			ctx.SetCursor(widget.CursorPointer)
			return true
		}
	case event.MousePress:
		if inside && me.Button == event.ButtonLeft {
			n.pressed = true
			n.SetNeedsRedraw(true)
			return true
		}
	case event.MouseRelease:
		if n.pressed && me.Button == event.ButtonLeft {
			n.pressed = false
			n.SetNeedsRedraw(true)
			if inside && n.onClick != nil {
				n.onClick()
			}
			return true
		}
	}
	return false
}

func (n *navItem) Children() []widget.Widget { return nil }

func (n *navItem) Mount(ctx widget.Context) {
	sched := ctx.Scheduler()
	if sched == nil || n.shell == nil {
		return
	}
	n.AddBinding(state.BindToScheduler(n.shell.rev, n, sched))
}

func (n *navItem) Unmount() {}
