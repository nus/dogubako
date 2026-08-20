package app

import (
	"image"
	"image/color"
	"math"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/imageproc"
)

// paintCanvasFrame is one paint-canvas draw: raster, original size, overlay.
type paintCanvasFrame struct {
	image   image.Image
	size    image.Point
	overlay *image.NRGBA
}

// paintCanvas shows the source image and lets the user draw on it.
type paintCanvas struct {
	widget.WidgetBase

	rev      state.ReadonlySignal[uint64]
	provider func() paintCanvasFrame
	onBegin  func(image.Point)
	onDrag   func(image.Point)
	onEnd    func()

	image     image.Image
	imageSize image.Point
	overlay   *image.NRGBA
	emptyHint func() string

	dragging bool
}

func newPaintCanvas(rev state.ReadonlySignal[uint64]) *paintCanvas {
	p := &paintCanvas{rev: rev}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

func (p *paintCanvas) SetProvider(f func() paintCanvasFrame) {
	p.provider = f
	p.SetNeedsRedraw(true)
}

func (p *paintCanvas) SetEmptyHint(f func() string) {
	p.emptyHint = f
}

func (p *paintCanvas) OnStroke(begin, drag func(image.Point), end func()) {
	p.onBegin = begin
	p.onDrag = drag
	p.onEnd = end
}

func (p *paintCanvas) sync() {
	if p.rev != nil {
		_ = p.rev.Get()
	}
	if p.provider == nil {
		return
	}
	frame := p.provider()
	p.image = frame.image
	p.imageSize = frame.size
	p.overlay = frame.overlay
}

func (p *paintCanvas) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(400, 300)
	p.SetBounds(geometry.FromPointSize(p.Position(), size))
	return size
}

func (p *paintCanvas) Draw(_ widget.Context, canvas widget.Canvas) {
	p.sync()
	b := p.Bounds()
	if b.IsEmpty() {
		return
	}
	drawCheckerboard(canvas, b)
	if p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		if p.emptyHint == nil {
			return
		}
		hint := p.emptyHint()
		if hint == "" {
			return
		}
		style := widget.TextStyle{
			FontFamily: cjkembed.FamilyName,
			FontSize:   13,
			Color:      widget.RGBA8(120, 120, 120, 255),
			Align:      widget.TextAlignCenter,
		}
		if sd, ok := canvas.(widget.StyledTextDrawer); ok {
			sd.DrawStyledText(hint, b, style)
		} else {
			canvas.DrawText(hint, b, 13, style.Color, false, widget.TextAlignCenter)
		}
		return
	}
	ib := toImageRect(b)
	fitted := fittedRect(ib, p.imageSize)
	drawFittedPaintImage(canvas, p.image, p.overlay, fitted, p.imageSize)
}

func (p *paintCanvas) Event(ctx widget.Context, e event.Event) bool {
	p.sync()
	me, ok := e.(*event.MouseEvent)
	if !ok || p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return false
	}
	fitted := fittedRect(toImageRect(p.Bounds()), p.imageSize)
	if fitted.Empty() {
		return false
	}
	pt := p.localMouse(me)

	switch me.MouseType {
	case event.MousePress:
		if me.Button == event.ButtonLeft && pt.In(fitted) {
			p.dragging = true
			if cap, ok := ctx.(widget.PointerCapturer); ok {
				cap.CapturePointer(p)
			}
			ctx.SetCursor(widget.CursorCrosshair)
			if p.onBegin != nil {
				p.onBegin(screenToImage(pt, fitted, p.imageSize))
			}
			p.SetNeedsRedraw(true)
			return true
		}
	case event.MouseDrag, event.MouseMove:
		if p.dragging {
			cur := screenToImage(pt, fitted, p.imageSize)
			if p.onDrag != nil {
				p.onDrag(cur)
			}
			ctx.SetCursor(widget.CursorCrosshair)
			p.SetNeedsRedraw(true)
			return true
		}
		if pt.In(fitted) {
			ctx.SetCursor(widget.CursorCrosshair)
		}
	case event.MouseRelease:
		if p.dragging && me.Button == event.ButtonLeft {
			p.dragging = false
			if cap, ok := ctx.(widget.PointerCapturer); ok {
				cap.ReleasePointer(p)
			}
			if p.onEnd != nil {
				p.onEnd()
			}
			p.SetNeedsRedraw(true)
			return true
		}
	}
	return false
}

func (p *paintCanvas) localMouse(me *event.MouseEvent) image.Point {
	pos := me.Position
	if p.dragging && p.IsScreenOriginValid() {
		pos = pos.Sub(p.ScreenOrigin()).Add(p.Bounds().Min)
	}
	return image.Pt(int(math.Round(float64(pos.X))), int(math.Round(float64(pos.Y))))
}

func (p *paintCanvas) Children() []widget.Widget { return nil }

func (p *paintCanvas) Mount(ctx widget.Context) {
	if p.rev == nil {
		return
	}
	sched := ctx.Scheduler()
	if sched == nil {
		return
	}
	p.AddBinding(state.BindToScheduler(p.rev, p, sched))
}

func (p *paintCanvas) Unmount() {}

func drawFittedPaintImage(canvas widget.Canvas, src image.Image, overlay *image.NRGBA, fitted image.Rectangle, imgSize image.Point) {
	if src == nil || fitted.Empty() {
		return
	}
	size := src.Bounds().Size()
	if size.X != fitted.Dx() || size.Y != fitted.Dy() {
		src = imageproc.Resize(src, fitted.Dx(), fitted.Dy())
	}
	rgba := toDisplayRGBA(src)
	if overlay != nil && imgSize.X > 0 && imgSize.Y > 0 {
		rgba = cloneRGBA(rgba)
		applyPaintOverlay(rgba, overlay, imgSize)
	}
	canvas.DrawImage(rgba, geometry.Pt(float32(fitted.Min.X), float32(fitted.Min.Y)))
}

// applyPaintOverlay blends overlay (source-image coordinates) onto a display bitmap.
func applyPaintOverlay(dst *image.RGBA, overlay *image.NRGBA, imgSize image.Point) {
	if dst == nil || overlay == nil || imgSize.X <= 0 || imgSize.Y <= 0 {
		return
	}
	b := dst.Rect
	dw, dh := b.Dx(), b.Dy()
	if dw <= 0 || dh <= 0 {
		return
	}
	ob := overlay.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		sy := ob.Min.Y + (y-b.Min.Y)*imgSize.Y/dh
		for x := b.Min.X; x < b.Max.X; x++ {
			sx := ob.Min.X + (x-b.Min.X)*imgSize.X/dw
			c := overlay.NRGBAAt(sx, sy)
			if c.A == 0 {
				continue
			}
			i := dst.PixOffset(x, y)
			if c.A == 255 {
				dst.Pix[i+0] = c.R
				dst.Pix[i+1] = c.G
				dst.Pix[i+2] = c.B
				dst.Pix[i+3] = 255
				continue
			}
			a := uint32(c.A)
			ia := 255 - a
			dst.Pix[i+0] = uint8((uint32(c.R)*a + uint32(dst.Pix[i+0])*ia + 127) / 255)
			dst.Pix[i+1] = uint8((uint32(c.G)*a + uint32(dst.Pix[i+1])*ia + 127) / 255)
			dst.Pix[i+2] = uint8((uint32(c.B)*a + uint32(dst.Pix[i+2])*ia + 127) / 255)
			dst.Pix[i+3] = 255
		}
	}
}

var paintPalette = []color.NRGBA{
	{R: 0, G: 0, B: 0, A: 255},
	{R: 255, G: 255, B: 255, A: 255},
	{R: 224, G: 49, B: 49, A: 255},
	{R: 247, G: 103, B: 7, A: 255},
	{R: 245, G: 159, B: 0, A: 255},
	{R: 47, G: 158, B: 68, A: 255},
	{R: 47, G: 129, B: 255, A: 255},
	{R: 112, G: 72, B: 232, A: 255},
	{R: 121, G: 85, B: 72, A: 255},
}

const swatchSize float32 = 22

type colorSwatch struct {
	widget.WidgetBase
	shell    *Shell
	color    color.NRGBA
	selected func() bool
	onClick  func()
	hover    bool
	pressed  bool
}

func (s *Shell) colorSwatch(c color.NRGBA) *colorSwatch {
	w := &colorSwatch{
		shell: s,
		color: c,
		selected: func() bool {
			cur := s.model.Image().PaintColor()
			return cur.R == c.R && cur.G == c.G && cur.B == c.B
		},
		onClick: func() {
			s.model.Image().SetPaintColor(c)
			s.bump()
		},
	}
	w.SetVisible(true)
	w.SetEnabled(true)
	return w
}

func (w *colorSwatch) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.Constrain(geometry.Sz(swatchSize, swatchSize))
	w.SetBounds(geometry.FromPointSize(w.Position(), size))
	return size
}

func (w *colorSwatch) Draw(_ widget.Context, canvas widget.Canvas) {
	b := w.Bounds()
	if b.IsEmpty() {
		return
	}
	fill := widget.RGBA8(w.color.R, w.color.G, w.color.B, 255)
	canvas.DrawRoundRect(b, fill, 4)
	border := widget.RGBA8(180, 186, 196, 255)
	if w.selected != nil && w.selected() {
		border = widget.RGBA8(47, 129, 255, 255)
		canvas.StrokeRoundRect(b, border, 4, 2)
	} else {
		canvas.StrokeRoundRect(b, border, 4, 1)
	}
}

func (w *colorSwatch) Event(ctx widget.Context, e event.Event) bool {
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	inside := w.Bounds().Contains(me.Position)
	switch me.MouseType {
	case event.MouseMove:
		if w.hover != inside {
			w.hover = inside
			w.SetNeedsRedraw(true)
		}
		if inside {
			ctx.SetCursor(widget.CursorPointer)
			return true
		}
	case event.MousePress:
		if inside && me.Button == event.ButtonLeft {
			w.pressed = true
			w.SetNeedsRedraw(true)
			return true
		}
	case event.MouseRelease:
		if w.pressed && me.Button == event.ButtonLeft {
			w.pressed = false
			w.SetNeedsRedraw(true)
			if inside && w.onClick != nil {
				w.onClick()
			}
			return true
		}
	}
	return false
}

func (w *colorSwatch) Children() []widget.Widget { return nil }

func (w *colorSwatch) Mount(ctx widget.Context) {
	sched := ctx.Scheduler()
	if sched == nil || w.shell == nil {
		return
	}
	w.AddBinding(state.BindToScheduler(w.shell.rev, w, sched))
}

func (w *colorSwatch) Unmount() {}
