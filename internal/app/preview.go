package app

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/imageproc"
)

// sourcePreviewFrame is one source-preview draw: raster, original size, crop overlay.
type sourcePreviewFrame struct {
	image       image.Image
	size        image.Point
	crop        image.Rectangle
	cropEnabled bool
}

// sourcePreview shows the original image and lets the user drag a crop rectangle.
type sourcePreview struct {
	widget.WidgetBase

	rev           state.ReadonlySignal[uint64]
	provider      func() sourcePreviewFrame
	image         image.Image
	imageSize     image.Point
	crop          image.Rectangle
	cropEnabled   bool
	onCropChanged func(image.Rectangle)

	dragging  bool
	dragStart image.Point
	dragRect  image.Rectangle
}

func newSourcePreview(rev state.ReadonlySignal[uint64]) *sourcePreview {
	p := &sourcePreview{rev: rev}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

func (p *sourcePreview) SetProvider(f func() sourcePreviewFrame) {
	p.provider = f
	p.SetNeedsRedraw(true)
}

func (p *sourcePreview) SetImage(img image.Image, srcSize image.Point) {
	p.image = img
	p.imageSize = srcSize
	p.SetNeedsRedraw(true)
}

func (p *sourcePreview) SetCrop(crop image.Rectangle, enabled bool) {
	p.crop = crop
	p.cropEnabled = enabled
	p.SetNeedsRedraw(true)
}

func (p *sourcePreview) sync() {
	if p.rev != nil {
		_ = p.rev.Get()
	}
	if p.provider == nil {
		return
	}
	frame := p.provider()
	p.image = frame.image
	p.imageSize = frame.size
	p.crop = frame.crop
	p.cropEnabled = frame.cropEnabled
}

func (p *sourcePreview) OnCropChanged(f func(image.Rectangle)) {
	p.onCropChanged = f
}

func (p *sourcePreview) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(400, 300)
	p.SetBounds(geometry.FromPointSize(p.Position(), size))
	return size
}

func (p *sourcePreview) Draw(_ widget.Context, canvas widget.Canvas) {
	p.sync()
	b := p.Bounds()
	if b.IsEmpty() {
		return
	}
	drawCheckerboard(canvas, b)
	if p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return
	}
	ib := toImageRect(b)
	fitted := fittedRect(ib, p.imageSize)
	crop := image.Rectangle{}
	if p.cropEnabled || p.dragging {
		crop = p.crop
		if p.dragging {
			crop = p.dragRect
		}
	}
	drawFittedImage(canvas, p.image, fitted, p.imageSize, crop)
}

func (p *sourcePreview) Event(ctx widget.Context, e event.Event) bool {
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
			p.dragStart = screenToImage(pt, fitted, p.imageSize)
			p.dragRect = image.Rectangle{Min: p.dragStart, Max: p.dragStart.Add(image.Pt(1, 1))}.Canon()
			if cap, ok := ctx.(widget.PointerCapturer); ok {
				cap.CapturePointer(p)
			}
			ctx.SetCursor(widget.CursorCrosshair)
			p.SetNeedsRedraw(true)
			return true
		}
	case event.MouseDrag, event.MouseMove:
		if p.dragging {
			cur := screenToImage(pt, fitted, p.imageSize)
			p.dragRect = normalizeCrop(image.Rectangle{Min: p.dragStart, Max: cur.Add(image.Pt(1, 1))}, p.imageSize)
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
			rect := normalizeCrop(p.dragRect, p.imageSize)
			if rect.Dx() >= 1 && rect.Dy() >= 1 && p.onCropChanged != nil {
				p.onCropChanged(rect)
			}
			p.SetNeedsRedraw(true)
			return true
		}
	}
	return false
}

// localMouse maps an event into this widget's bounds space.
// Tree dispatch translates through parents (local). After CapturePointer,
// the window delivers the same events in window coordinates — subtracting
// ScreenOrigin puts the crop corner back on the cursor.
func (p *sourcePreview) localMouse(me *event.MouseEvent) image.Point {
	pos := me.Position
	if p.dragging && p.IsScreenOriginValid() {
		pos = pos.Sub(p.ScreenOrigin()).Add(p.Bounds().Min)
	}
	return image.Pt(int(math.Round(float64(pos.X))), int(math.Round(float64(pos.Y))))
}

func (p *sourcePreview) Children() []widget.Widget { return nil }

func (p *sourcePreview) Mount(ctx widget.Context) {
	if p.rev == nil {
		return
	}
	sched := ctx.Scheduler()
	if sched == nil {
		return
	}
	p.AddBinding(state.BindToScheduler(p.rev, p, sched))
}

func (p *sourcePreview) Unmount() {}

// destPreview shows a processed image fitted into its bounds.
type destPreview struct {
	widget.WidgetBase

	rev       state.ReadonlySignal[uint64]
	provider  func() image.Image
	source    image.Image
	image     image.Image
	emptyHint func() string
}

func newDestPreview(rev state.ReadonlySignal[uint64]) *destPreview {
	p := &destPreview{rev: rev}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

func (p *destPreview) SetProvider(f func() image.Image) {
	p.provider = f
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetImage(img image.Image) {
	p.image = img
	p.source = nil
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetSource(img image.Image) {
	p.source = img
	p.image = nil
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetEmptyHint(f func() string) {
	p.emptyHint = f
}

func (p *destPreview) HasImage() bool {
	if p.provider != nil {
		return p.provider() != nil
	}
	return p.source != nil || p.image != nil
}

func (p *destPreview) resolvedImage() image.Image {
	if p.rev != nil {
		_ = p.rev.Get()
	}
	if p.provider != nil {
		return p.provider()
	}
	if p.image != nil {
		return p.image
	}
	if p.source == nil {
		return nil
	}
	p.image = previewImage(p.source)
	return p.image
}

func (p *destPreview) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.BiggestFinite(400, 300)
	p.SetBounds(geometry.FromPointSize(p.Position(), size))
	return size
}

func (p *destPreview) Draw(_ widget.Context, canvas widget.Canvas) {
	b := p.Bounds()
	if b.IsEmpty() {
		return
	}
	drawCheckerboard(canvas, b)
	img := p.resolvedImage()
	if img != nil {
		drawFittedImage(canvas, img, fittedRect(toImageRect(b), img.Bounds().Size()), img.Bounds().Size(), image.Rectangle{})
		return
	}
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
}

func (p *destPreview) Event(_ widget.Context, _ event.Event) bool { return false }

func (p *destPreview) Children() []widget.Widget { return nil }

func (p *destPreview) Mount(ctx widget.Context) {
	if p.rev == nil {
		return
	}
	sched := ctx.Scheduler()
	if sched == nil {
		return
	}
	p.AddBinding(state.BindToScheduler(p.rev, p, sched))
}

func (p *destPreview) Unmount() {}

func fittedRect(bounds image.Rectangle, imgSize image.Point) image.Rectangle {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return image.Rectangle{}
	}
	scale := math.Min(float64(bounds.Dx())/float64(imgSize.X), float64(bounds.Dy())/float64(imgSize.Y))
	w := max(1, int(math.Round(float64(imgSize.X)*scale)))
	h := max(1, int(math.Round(float64(imgSize.Y)*scale)))
	x := bounds.Min.X + (bounds.Dx()-w)/2
	y := bounds.Min.Y + (bounds.Dy()-h)/2
	return image.Rect(x, y, x+w, y+h)
}

func drawFittedImage(canvas widget.Canvas, src image.Image, fitted image.Rectangle, imgSize image.Point, crop image.Rectangle) {
	if src == nil || fitted.Empty() {
		return
	}
	size := src.Bounds().Size()
	if size.X != fitted.Dx() || size.Y != fitted.Dy() {
		src = imageproc.Resize(src, fitted.Dx(), fitted.Dy())
	}
	rgba := toDisplayRGBA(src)
	if imgSize.X > 0 && imgSize.Y > 0 && !crop.Empty() {
		rgba = cloneRGBA(rgba)
		applyCropHighlight(rgba, imgSize, crop)
	}
	canvas.DrawImage(rgba, geometry.Pt(float32(fitted.Min.X), float32(fitted.Min.Y)))
}

// toDisplayRGBA copies src into an origin-aligned RGBA buffer. GPU DrawImage
// treats Pix as tightly packed RGBA starting at (0,0); NRGBA and non-zero Min
// would otherwise upload garbage or nothing.
func toDisplayRGBA(src image.Image) *image.RGBA {
	if src == nil {
		return nil
	}
	if rgba, ok := src.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) && rgba.Stride == rgba.Rect.Dx()*4 {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	if src == nil {
		return nil
	}
	dst := image.NewRGBA(src.Rect)
	copy(dst.Pix, src.Pix)
	return dst
}

const cropDimMul = 112 // 112/256 ≈ remaining luminance under 0x90 black overlay

var cropBorder = color.RGBA{R: 0x2f, G: 0x81, B: 0xff, A: 0xff}

// applyCropHighlight darkens pixels outside crop and draws a 2px accent border.
// Overlay is baked into the bitmap because GPU DrawImage is composited over
// later DrawRect/StrokeRect, so a separate highlight pass is invisible.
func applyCropHighlight(dst *image.RGBA, imgSize image.Point, crop image.Rectangle) {
	if dst == nil || imgSize.X <= 0 || imgSize.Y <= 0 {
		return
	}
	b := dst.Rect
	screen := imageRectToScreen(crop, imgSize, b)
	if screen.Empty() {
		return
	}
	dimRGBA(dst, image.Rect(b.Min.X, b.Min.Y, screen.Min.X, b.Max.Y))
	dimRGBA(dst, image.Rect(screen.Max.X, b.Min.Y, b.Max.X, b.Max.Y))
	dimRGBA(dst, image.Rect(screen.Min.X, b.Min.Y, screen.Max.X, screen.Min.Y))
	dimRGBA(dst, image.Rect(screen.Min.X, screen.Max.Y, screen.Max.X, b.Max.Y))
	const t = 2
	fillRGBA(dst, image.Rect(screen.Min.X, screen.Min.Y, screen.Max.X, screen.Min.Y+t), cropBorder)
	fillRGBA(dst, image.Rect(screen.Min.X, screen.Max.Y-t, screen.Max.X, screen.Max.Y), cropBorder)
	fillRGBA(dst, image.Rect(screen.Min.X, screen.Min.Y, screen.Min.X+t, screen.Max.Y), cropBorder)
	fillRGBA(dst, image.Rect(screen.Max.X-t, screen.Min.Y, screen.Max.X, screen.Max.Y), cropBorder)
}

func dimRGBA(dst *image.RGBA, r image.Rectangle) {
	r = r.Intersect(dst.Rect)
	if r.Empty() {
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := dst.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.Pix[i+0] = uint8(uint32(dst.Pix[i+0]) * cropDimMul / 256)
			dst.Pix[i+1] = uint8(uint32(dst.Pix[i+1]) * cropDimMul / 256)
			dst.Pix[i+2] = uint8(uint32(dst.Pix[i+2]) * cropDimMul / 256)
			i += 4
		}
	}
}

func fillRGBA(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(dst.Rect)
	if r.Empty() {
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := dst.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.Pix[i+0] = c.R
			dst.Pix[i+1] = c.G
			dst.Pix[i+2] = c.B
			dst.Pix[i+3] = c.A
			i += 4
		}
	}
}

func imageRectToScreen(rect image.Rectangle, imgSize image.Point, fitted image.Rectangle) image.Rectangle {
	if imgSize.X <= 0 || imgSize.Y <= 0 || fitted.Empty() || rect.Empty() {
		return image.Rectangle{}
	}
	x0 := fitted.Min.X + rect.Min.X*fitted.Dx()/imgSize.X
	y0 := fitted.Min.Y + rect.Min.Y*fitted.Dy()/imgSize.Y
	x1 := fitted.Min.X + rect.Max.X*fitted.Dx()/imgSize.X
	y1 := fitted.Min.Y + rect.Max.Y*fitted.Dy()/imgSize.Y
	return image.Rect(x0, y0, x1, y1)
}

func screenToImage(pt image.Point, fitted image.Rectangle, imgSize image.Point) image.Point {
	if fitted.Empty() {
		return image.Point{}
	}
	x := (pt.X - fitted.Min.X) * imgSize.X / fitted.Dx()
	y := (pt.Y - fitted.Min.Y) * imgSize.Y / fitted.Dy()
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= imgSize.X {
		x = imgSize.X - 1
	}
	if y >= imgSize.Y {
		y = imgSize.Y - 1
	}
	return image.Pt(x, y)
}

func normalizeCrop(rect image.Rectangle, imgSize image.Point) image.Rectangle {
	r := rect.Canon().Intersect(image.Rect(0, 0, imgSize.X, imgSize.Y))
	if r.Dx() < 1 {
		r.Max.X = r.Min.X + 1
	}
	if r.Dy() < 1 {
		r.Max.Y = r.Min.Y + 1
	}
	return r.Intersect(image.Rect(0, 0, imgSize.X, imgSize.Y))
}

func drawCheckerboard(canvas widget.Canvas, b geometry.Rect) {
	const cell = 8
	c0 := widget.RGBA8(0x3a, 0x3a, 0x3a, 0xff)
	c1 := widget.RGBA8(0x2a, 0x2a, 0x2a, 0xff)
	canvas.DrawRect(b, c0)
	ib := toImageRect(b)
	for y := ib.Min.Y; y < ib.Max.Y; y += cell {
		for x := ib.Min.X; x < ib.Max.X; x += cell {
			if ((x-ib.Min.X)/cell+(y-ib.Min.Y)/cell)%2 == 0 {
				continue
			}
			w := min(cell, ib.Max.X-x)
			h := min(cell, ib.Max.Y-y)
			canvas.DrawRect(geometry.NewRect(float32(x), float32(y), float32(w), float32(h)), c1)
		}
	}
}

func toImageRect(r geometry.Rect) image.Rectangle {
	return image.Rect(int(r.Min.X), int(r.Min.Y), int(r.Max.X), int(r.Max.Y))
}
