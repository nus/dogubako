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

func (h *modeHost) tools() []widget.Widget {
	return []widget.Widget{h.image, h.screenshot, h.android}
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

// Children exposes only the active tool. gogpu/ui's compositor walks
// Children() to blit RepaintBoundary textures; inactive tools must not
// appear there or their last frame stays on screen after a switch.
func (h *modeHost) Children() []widget.Widget {
	if c := h.active(); c != nil {
		return []widget.Widget{c}
	}
	return nil
}

func (h *modeHost) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(windowWidth, windowHeight)
	h.SetBounds(geometry.FromPointSize(h.Position(), size))
	active := h.active()
	for _, child := range h.tools() {
		if child == nil {
			continue
		}
		if vis, ok := child.(interface{ SetVisible(bool) }); ok {
			vis.SetVisible(child == active)
		}
		if child == active {
			sz := widget.LayoutChild(child, ctx, geometry.Tight(size))
			if b, ok := child.(boundsSetter); ok {
				b.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), sz))
			}
			continue
		}
		if b, ok := child.(boundsSetter); ok {
			b.SetBounds(geometry.Rect{})
		}
	}
	return size
}

func (h *modeHost) Draw(ctx widget.Context, canvas widget.Canvas) {
	bounds := h.Bounds()
	if !bounds.IsEmpty() {
		canvas.DrawRect(bounds, widget.RGBA8(255, 255, 255, 255))
	}
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
	active := h.active()
	for _, child := range h.tools() {
		if child != nil && child != active {
			widget.MountTree(child, ctx)
		}
	}
	if sched := ctx.Scheduler(); sched != nil {
		h.AddBinding(state.BindToSchedulerLayout(h.shell.rev, h, sched))
	}
}

func (h *modeHost) Unmount() {
	active := h.active()
	for _, child := range h.tools() {
		if child != nil && child != active {
			widget.UnmountTree(child)
		}
	}
}

// imageFeatureHost shows clip or paint UI inside the image tool.
type imageFeatureHost struct {
	widget.WidgetBase
	shell *Shell
	clip  widget.Widget
	paint widget.Widget
}

func newImageFeatureHost(s *Shell) *imageFeatureHost {
	h := &imageFeatureHost{
		shell: s,
		clip:  s.buildClipFeature(),
		paint: s.buildPaintFeature(),
	}
	h.SetVisible(true)
	h.SetEnabled(true)
	h.AddChild(h.clip)
	h.AddChild(h.paint)
	return h
}

func (h *imageFeatureHost) features() []widget.Widget {
	return []widget.Widget{h.clip, h.paint}
}

func (h *imageFeatureHost) active() widget.Widget {
	if h.shell.model.Image().Feature() == ImagePaint {
		return h.paint
	}
	return h.clip
}

func (h *imageFeatureHost) Children() []widget.Widget {
	if c := h.active(); c != nil {
		return []widget.Widget{c}
	}
	return nil
}

func (h *imageFeatureHost) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(windowWidth, windowHeight)
	h.SetBounds(geometry.FromPointSize(h.Position(), size))
	active := h.active()
	for _, child := range h.features() {
		if child == nil {
			continue
		}
		if vis, ok := child.(interface{ SetVisible(bool) }); ok {
			vis.SetVisible(child == active)
		}
		if child == active {
			sz := widget.LayoutChild(child, ctx, geometry.Tight(size))
			if b, ok := child.(boundsSetter); ok {
				b.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), sz))
			}
			continue
		}
		if b, ok := child.(boundsSetter); ok {
			b.SetBounds(geometry.Rect{})
		}
	}
	return size
}

func (h *imageFeatureHost) Draw(ctx widget.Context, canvas widget.Canvas) {
	if child := h.active(); child != nil {
		widget.DrawChild(child, ctx, canvas)
	}
}

func (h *imageFeatureHost) Event(ctx widget.Context, e event.Event) bool {
	if child := h.active(); child != nil {
		return child.Event(ctx, e)
	}
	return false
}

func (h *imageFeatureHost) Mount(ctx widget.Context) {
	active := h.active()
	for _, child := range h.features() {
		if child != nil && child != active {
			widget.MountTree(child, ctx)
		}
	}
	if sched := ctx.Scheduler(); sched != nil {
		h.AddBinding(state.BindToSchedulerLayout(h.shell.rev, h, sched))
	}
}

func (h *imageFeatureHost) Unmount() {
	active := h.active()
	for _, child := range h.features() {
		if child != nil && child != active {
			widget.UnmountTree(child)
		}
	}
}
