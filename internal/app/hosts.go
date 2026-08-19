package app

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"
)

type boundsSetter interface {
	SetBounds(geometry.Rect)
}

// keyHost fills the window and intercepts shortcut keys before the child tree.
type keyHost struct {
	widget.WidgetBase
	shell *Shell
	child widget.Widget
}

func newKeyHost(s *Shell, child widget.Widget) *keyHost {
	h := &keyHost{shell: s, child: child}
	h.SetVisible(true)
	h.SetEnabled(true)
	if child != nil {
		h.AddChild(child)
	}
	return h
}

func (h *keyHost) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(windowWidth, windowHeight)
	h.SetBounds(geometry.FromPointSize(h.Position(), size))
	if h.child != nil {
		sz := widget.LayoutChild(h.child, ctx, geometry.Tight(size))
		if b, ok := h.child.(boundsSetter); ok {
			b.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), sz))
		}
	}
	return size
}

func (h *keyHost) Draw(ctx widget.Context, canvas widget.Canvas) {
	if h.child != nil {
		widget.DrawChild(h.child, ctx, canvas)
	}
}

func (h *keyHost) Event(ctx widget.Context, e event.Event) bool {
	if ke, ok := e.(*event.KeyEvent); ok && h.shell.handleKey(ke) {
		return true
	}
	if h.child != nil {
		return h.child.Event(ctx, e)
	}
	return false
}

// modeHost shows the active tool panel.
type modeHost struct {
	widget.WidgetBase
	shell      *Shell
	image      widget.Widget
	screenshot widget.Widget
	android    widget.Widget
}

func newModeHost(s *Shell) *modeHost {
	h := &modeHost{
		shell:      s,
		image:      s.buildImageTool(),
		screenshot: s.buildScreenshotTool(),
		android:    s.buildAndroidTool(),
	}
	h.SetVisible(true)
	h.SetEnabled(true)
	h.AddChild(h.image)
	h.AddChild(h.screenshot)
	h.AddChild(h.android)
	return h
}

func (h *modeHost) active() widget.Widget {
	switch h.shell.model.Mode() {
	case ToolScreenshot:
		return h.screenshot
	case ToolAndroid:
		return h.android
	default:
		return h.image
	}
}

func (h *modeHost) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(windowWidth, windowHeight)
	h.SetBounds(geometry.FromPointSize(h.Position(), size))
	for _, child := range []widget.Widget{h.image, h.screenshot, h.android} {
		if child == nil {
			continue
		}
		sz := widget.LayoutChild(child, ctx, geometry.Tight(size))
		if b, ok := child.(boundsSetter); ok {
			b.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), sz))
		}
	}
	return size
}

func (h *modeHost) Draw(ctx widget.Context, canvas widget.Canvas) {
	if child := h.active(); child != nil {
		widget.DrawChild(child, ctx, canvas)
	}
}

func (h *modeHost) Event(ctx widget.Context, e event.Event) bool {
	if child := h.active(); child != nil {
		return child.Event(ctx, e)
	}
	return false
}

func (h *modeHost) Mount(ctx widget.Context) {
	sched := ctx.Scheduler()
	if sched == nil {
		return
	}
	h.AddBinding(state.BindToSchedulerLayout(h.shell.rev, h, sched))
}

func (h *modeHost) Unmount() {}
