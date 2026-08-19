package app

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// equalRow lays out children in a horizontal row with equal width and height,
// matching guigui's form-panel row.
type equalRow struct {
	widget.WidgetBase
	children []widget.Widget
	gap      float32
}

func newEqualRow(gap float32, children ...widget.Widget) *equalRow {
	r := &equalRow{children: children, gap: gap}
	r.SetVisible(true)
	r.SetEnabled(true)
	for _, ch := range children {
		if ch != nil {
			r.AddChild(ch)
		}
	}
	return r
}

func (r *equalRow) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	n := 0
	for _, ch := range r.children {
		if ch != nil {
			n++
		}
	}
	if n == 0 {
		size := cons.Constrain(geometry.Sz(0, 0))
		r.SetBounds(geometry.FromPointSize(r.Position(), size))
		return size
	}
	innerW := cons.MaxWidth
	if !cons.HasBoundedWidth() {
		innerW = windowWidth
	}
	cellW := (innerW - r.gap*float32(n-1)) / float32(n)
	if cellW < 1 {
		cellW = 1
	}
	var maxH float32
	for _, ch := range r.children {
		if ch == nil {
			continue
		}
		sz := widget.LayoutChild(ch, ctx, geometry.Constraints{
			MinWidth: cellW, MaxWidth: cellW,
			MinHeight: 0, MaxHeight: cons.MaxHeight,
		})
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	x := float32(0)
	for _, ch := range r.children {
		if ch == nil {
			continue
		}
		sz := widget.LayoutChild(ch, ctx, geometry.Constraints{
			MinWidth: cellW, MaxWidth: cellW,
			MinHeight: maxH, MaxHeight: maxH,
		})
		if b, ok := ch.(boundsSetter); ok {
			b.SetBounds(geometry.FromPointSize(geometry.Pt(x, 0), geometry.Sz(cellW, maxH)))
		}
		_ = sz
		x += cellW + r.gap
	}
	size := cons.Constrain(geometry.Sz(innerW, maxH))
	r.SetBounds(geometry.FromPointSize(r.Position(), size))
	return size
}

func (r *equalRow) Draw(ctx widget.Context, canvas widget.Canvas) {
	b := r.Bounds()
	canvas.PushTransform(b.Min)
	for _, ch := range r.children {
		if ch == nil {
			continue
		}
		widget.StampScreenOrigin(ch, canvas)
		widget.DrawChild(ch, ctx, canvas)
	}
	canvas.PopTransform()
}

func (r *equalRow) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		for i := len(r.children) - 1; i >= 0; i-- {
			if r.children[i] != nil && r.children[i].Event(ctx, e) {
				return true
			}
		}
		return false
	}
	local := *me
	local.Position = me.Position.Sub(r.Bounds().Min)
	for i := len(r.children) - 1; i >= 0; i-- {
		ch := r.children[i]
		if ch == nil {
			continue
		}
		if bw, ok := ch.(interface{ Bounds() geometry.Rect }); ok && !bw.Bounds().Contains(local.Position) {
			continue
		}
		if ch.Event(ctx, &local) {
			return true
		}
	}
	return false
}

func (r *equalRow) Children() []widget.Widget { return r.children }
func (r *equalRow) Mount(ctx widget.Context) {
	for _, ch := range r.children {
		widget.MountTree(ch, ctx)
	}
}
func (r *equalRow) Unmount() {}
