package app

import (
	"image"

	"github.com/gogpu/ui/core/scrollview"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
)

const (
	screenshotRowH      float32 = 48
	screenshotThumbPad  float32 = 8
	screenshotThumbSize float32 = 40
	screenshotNameGap   float32 = 8
)

// screenshotFileList is a paint-based file list. gogpu ListView wraps each
// row in a RepaintBoundary, so items stay blank until hover and nested
// destPreview images are composited at the wrong origin.
type screenshotFileList struct {
	widget.WidgetBase
	shell   *Shell
	body    *screenshotListBody
	scroll  *scrollview.Widget
	scrollY state.Signal[float32]
}

type screenshotListBody struct {
	widget.WidgetBase
	list  *screenshotFileList
	hover int
}

func newScreenshotFileList(s *Shell) *screenshotFileList {
	scrollY := state.NewSignal[float32](0)
	l := &screenshotFileList{shell: s, scrollY: scrollY}
	l.SetVisible(true)
	l.SetEnabled(true)
	body := &screenshotListBody{list: l, hover: -1}
	body.SetVisible(true)
	body.SetEnabled(true)
	l.body = body
	l.scroll = scrollview.New(
		body,
		scrollview.DirectionOpt(scrollview.Vertical),
		scrollview.ScrollYSignal(scrollY),
		scrollview.ScrollStep(screenshotRowH),
	)
	l.AddChild(l.scroll)
	return l
}

func (l *screenshotFileList) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(screenshotListW, 400)
	l.SetBounds(geometry.FromPointSize(l.Position(), size))
	if l.scroll != nil {
		widget.LayoutChild(l.scroll, ctx, geometry.Tight(size))
		l.scroll.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	}
	l.syncSelection(size.Height)
	return size
}

func (l *screenshotFileList) syncSelection(viewportH float32) {
	if l.shell == nil {
		return
	}
	shot := l.shell.model.Screenshot()
	files := shot.Files()
	idx := screenshotFileIndex(files, shot.SelectedPath())
	if l.shell.shotSel.Get() != idx {
		l.shell.shotSel.Set(idx)
	}
	if !shot.TakeJumpToSelected() || idx < 0 || l.scrollY == nil {
		return
	}
	l.scrollY.Set(screenshotScrollToShow(idx, len(files), viewportH, l.scrollY.Get()))
}

func (l *screenshotFileList) Draw(ctx widget.Context, canvas widget.Canvas) {
	if l.scroll == nil {
		return
	}
	widget.StampScreenOrigin(l.scroll, canvas)
	widget.DrawChild(l.scroll, ctx, canvas)
}

func (l *screenshotFileList) Event(ctx widget.Context, e event.Event) bool {
	if l.scroll == nil {
		return false
	}
	switch ev := e.(type) {
	case *event.WheelEvent:
		if !l.Bounds().Contains(ev.Position) {
			return false
		}
	case *event.MouseEvent:
		if !l.Bounds().Contains(ev.Position) {
			return false
		}
	}
	return l.scroll.Event(ctx, e)
}

func (l *screenshotFileList) Children() []widget.Widget {
	if l.scroll == nil {
		return nil
	}
	return []widget.Widget{l.scroll}
}

func (l *screenshotFileList) Mount(ctx widget.Context) {
	if l.shell == nil {
		return
	}
	if sched := ctx.Scheduler(); sched != nil {
		l.AddBinding(state.BindToSchedulerLayout(l.shell.rev, l, sched))
	}
}

func (l *screenshotFileList) Unmount() {}

func (b *screenshotListBody) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	n := 0
	if b.list != nil && b.list.shell != nil {
		n = len(b.list.shell.model.Screenshot().Files())
	}
	width := cons.MaxWidth
	if !cons.HasBoundedWidth() {
		width = screenshotListW
	}
	size := cons.Constrain(screenshotListContentSize(n, width))
	b.SetBounds(geometry.FromPointSize(b.Position(), size))
	return size
}

func (b *screenshotListBody) Draw(_ widget.Context, canvas widget.Canvas) {
	if b.list == nil || b.list.shell == nil {
		return
	}
	shot := b.list.shell.model.Screenshot()
	files := shot.Files()
	n := len(files)
	if n == 0 {
		return
	}
	width := b.Bounds().Width()
	selected := b.list.shell.shotSel.Get()
	start, end := screenshotVisibleRange(n, b.scrollY(), b.viewportHeight())
	for i := start; i < end; i++ {
		row := screenshotRowRect(i, width)
		if i == selected {
			canvas.DrawRect(row, widget.RGBA8(47, 129, 255, 31))
		} else if i == b.hover {
			canvas.DrawRect(row, widget.RGBA8(0, 0, 0, 10))
		}
		thumbSlot := screenshotThumbRect(row)
		drawCheckerboard(canvas, thumbSlot)
		if img := shot.Thumbnail(files[i].Path); img != nil {
			fitted := fittedRect(toImageRect(thumbSlot), img.Bounds().Size())
			drawFittedImage(canvas, img, fitted, img.Bounds().Size(), image.Rectangle{})
		}
		name := screenshotNameRect(row)
		style := widget.TextStyle{
			FontFamily: cjkembed.FamilyName,
			FontSize:   13,
			Color:      widget.RGBA8(33, 33, 33, 255),
			Align:      widget.TextAlignLeft,
		}
		canvas.PushClip(name)
		if sd, ok := canvas.(widget.StyledTextDrawer); ok {
			sd.DrawStyledText(files[i].Name, name, style)
		} else {
			canvas.DrawText(files[i].Name, name, 13, style.Color, false, widget.TextAlignLeft)
		}
		canvas.PopClip()
	}
}

func (b *screenshotListBody) Event(ctx widget.Context, e event.Event) bool {
	if b.list == nil || b.list.shell == nil {
		return false
	}
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	n := len(b.list.shell.model.Screenshot().Files())
	idx := screenshotHitIndex(me.Position.Y, n)
	switch me.MouseType {
	case event.MouseMove:
		if b.hover != idx {
			b.hover = idx
			b.SetNeedsRedraw(true)
		}
		if idx >= 0 {
			ctx.SetCursor(widget.CursorPointer)
		}
		return false
	case event.MouseLeave:
		if b.hover != -1 {
			b.hover = -1
			b.SetNeedsRedraw(true)
		}
		return false
	case event.MousePress:
		if me.Button != event.ButtonLeft || idx < 0 {
			return false
		}
		if b.list.shell.model.Screenshot().Capturing() {
			return true
		}
		files := b.list.shell.model.Screenshot().Files()
		b.list.shell.shotSel.Set(idx)
		_ = b.list.shell.model.Screenshot().LoadPath(files[idx].Path)
		b.list.shell.bump()
		return true
	}
	return false
}

func (b *screenshotListBody) Children() []widget.Widget { return nil }

func (b *screenshotListBody) Mount(ctx widget.Context) {
	if b.list == nil || b.list.shell == nil {
		return
	}
	if sched := ctx.Scheduler(); sched != nil {
		b.AddBinding(state.BindToScheduler(b.list.shell.rev, b, sched))
	}
}

func (b *screenshotListBody) Unmount() {}

func (b *screenshotListBody) scrollY() float32 {
	if b.list == nil || b.list.scrollY == nil {
		return 0
	}
	return b.list.scrollY.Get()
}

func (b *screenshotListBody) viewportHeight() float32 {
	if b.list == nil {
		return 400
	}
	h := b.list.Bounds().Height()
	if h <= 0 {
		return 400
	}
	return h
}

func screenshotListContentSize(n int, width float32) geometry.Size {
	if n < 0 {
		n = 0
	}
	if width < 0 {
		width = 0
	}
	return geometry.Sz(width, float32(n)*screenshotRowH)
}

func screenshotRowRect(index int, width float32) geometry.Rect {
	if index < 0 {
		index = 0
	}
	return geometry.NewRect(0, float32(index)*screenshotRowH, width, screenshotRowH)
}

func screenshotThumbRect(row geometry.Rect) geometry.Rect {
	y := row.Min.Y + (row.Height()-screenshotThumbSize)/2
	return geometry.NewRect(row.Min.X+screenshotThumbPad, y, screenshotThumbSize, screenshotThumbSize)
}

func screenshotNameRect(row geometry.Rect) geometry.Rect {
	thumb := screenshotThumbRect(row)
	x := thumb.Max.X + screenshotNameGap
	w := row.Max.X - screenshotThumbPad - x
	if w < 0 {
		w = 0
	}
	return geometry.NewRect(x, row.Min.Y, w, row.Height())
}

func screenshotHitIndex(y float32, n int) int {
	if y < 0 || screenshotRowH <= 0 || n <= 0 {
		return -1
	}
	i := int(y / screenshotRowH)
	if i < 0 || i >= n {
		return -1
	}
	return i
}

func screenshotFileIndex(files []ScreenshotFile, path string) int {
	if path == "" {
		return -1
	}
	for i, f := range files {
		if f.Path == path {
			return i
		}
	}
	return -1
}

func screenshotScrollToShow(index, n int, viewportH, current float32) float32 {
	if index < 0 || index >= n || screenshotRowH <= 0 {
		return current
	}
	top := float32(index) * screenshotRowH
	bottom := top + screenshotRowH
	if top < current {
		return top
	}
	if viewportH > 0 && bottom > current+viewportH {
		y := bottom - viewportH
		if y < 0 {
			y = 0
		}
		return y
	}
	return current
}

func screenshotVisibleRange(n int, scrollY, viewportH float32) (start, end int) {
	if n <= 0 {
		return 0, 0
	}
	if screenshotRowH <= 0 {
		return 0, n
	}
	start = int(scrollY / screenshotRowH)
	if start < 0 {
		start = 0
	}
	if viewportH <= 0 {
		return start, n
	}
	end = int((scrollY+viewportH)/screenshotRowH) + 2
	if end > n {
		end = n
	}
	if start > end {
		start = end
	}
	return start, end
}
