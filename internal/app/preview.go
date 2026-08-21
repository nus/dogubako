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

// previewFrame is one preview draw: the raster to scale, the full-resolution
// image used once zoomed past that raster, the original pixel size, and the
// crop overlay.
type previewFrame struct {
	image       image.Image
	full        image.Image
	size        image.Point
	crop        image.Rectangle
	cropEnabled bool
}

// zoomable is the zoom and pan state shared by the preview widgets. The
// embedding widget supplies its own bounds because the two previews are
// laid out separately.
type zoomable struct {
	view      zoomView
	imageSize image.Point
	panning   bool
	panLast   image.Point
	onZoom    func()
}

// SetOnZoom registers the callback that refreshes the zoom readout.
func (z *zoomable) SetOnZoom(f func()) { z.onZoom = f }

// setImageSize resets the view when a different image arrives, so a new
// picture always starts fitted.
func (z *zoomable) setImageSize(size image.Point) {
	if size == z.imageSize {
		return
	}
	z.imageSize = size
	z.view.reset()
	z.panning = false
}

func (z *zoomable) zoomed(bounds image.Rectangle) bool {
	return z.view.canPan(bounds, z.imageSize)
}

func (z *zoomable) notifyZoom() {
	if z.onZoom != nil {
		z.onZoom()
	}
}

// sourcePreview shows the original image and lets the user drag a crop
// rectangle. Zoom is by wheel or by the buttons next to the panel title;
// panning uses the middle button or Shift with the left button, because a
// plain left drag draws the crop.
type sourcePreview struct {
	widget.WidgetBase
	zoomable

	rev           state.ReadonlySignal[uint64]
	provider      func() previewFrame
	image         image.Image
	full          image.Image
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

func (p *sourcePreview) SetProvider(f func() previewFrame) {
	p.provider = f
	p.SetNeedsRedraw(true)
}

func (p *sourcePreview) SetImage(img image.Image, srcSize image.Point) {
	p.image = img
	p.setImageSize(srcSize)
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
	p.full = frame.full
	p.setImageSize(frame.size)
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
	view := toImageRect(b)
	disp := p.view.displayRect(view, p.imageSize)
	crop := image.Rectangle{}
	if p.cropEnabled || p.dragging {
		crop = p.crop
		if p.dragging {
			crop = p.dragRect
		}
	}
	drawPreviewImage(canvas, p.raster(disp), disp, view, p.imageSize, crop)
}

// raster picks the sharpest source available for disp. The cached preview
// raster is capped at previewMaxEdge, so zooming past it falls back to the
// full-resolution image.
func (p *sourcePreview) raster(disp image.Rectangle) image.Image {
	return sharpestRaster(p.image, p.full, disp)
}

func (p *sourcePreview) Event(ctx widget.Context, e event.Event) bool {
	p.sync()
	if p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return false
	}
	view := toImageRect(p.Bounds())
	if we, ok := e.(*event.WheelEvent); ok {
		return p.wheel(ctx, we, view)
	}
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	disp := p.view.displayRect(view, p.imageSize)
	if disp.Empty() {
		return false
	}
	pt := p.localMouse(me)
	if p.panEvent(ctx, me, pt, view) {
		return true
	}

	switch me.MouseType {
	case event.MousePress:
		if me.Button == event.ButtonLeft && !p.panning && pt.In(disp) {
			p.dragging = true
			p.dragStart = screenToImage(pt, disp, p.imageSize)
			p.dragRect = image.Rectangle{Min: p.dragStart, Max: p.dragStart.Add(image.Pt(1, 1))}.Canon()
			capturePointer(ctx, p)
			ctx.SetCursor(widget.CursorCrosshair)
			p.SetNeedsRedraw(true)
			return true
		}
	case event.MouseDrag, event.MouseMove:
		if p.dragging {
			cur := screenToImage(pt, disp, p.imageSize)
			p.dragRect = normalizeCrop(image.Rectangle{Min: p.dragStart, Max: cur.Add(image.Pt(1, 1))}, p.imageSize)
			ctx.SetCursor(widget.CursorCrosshair)
			p.SetNeedsRedraw(true)
			return true
		}
		if pt.In(disp) {
			ctx.SetCursor(widget.CursorCrosshair)
		}
	case event.MouseRelease:
		if p.dragging && me.Button == event.ButtonLeft {
			p.dragging = false
			releasePointer(ctx, p)
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

// panEvent handles the middle-button and Shift+left drag that moves a
// zoomed-in image. It reports whether the event was consumed.
func (p *sourcePreview) panEvent(ctx widget.Context, me *event.MouseEvent, pt image.Point, view image.Rectangle) bool {
	switch me.MouseType {
	case event.MousePress:
		if p.dragging || !startsPan(me) || !pt.In(view) || !p.zoomed(view) {
			return false
		}
		p.beginPan(ctx, p, pt)
		return true
	case event.MouseDrag, event.MouseMove:
		return p.continuePan(ctx, pt, view)
	case event.MouseRelease:
		return p.endPan(ctx, p, me)
	}
	return false
}

func (p *sourcePreview) wheel(ctx widget.Context, we *event.WheelEvent, view image.Rectangle) bool {
	return applyWheelZoom(ctx, p, &p.zoomable, we, view)
}

// localMouse maps an event into this widget's bounds space.
// Tree dispatch translates through parents (local). After CapturePointer,
// the window delivers the same events in window coordinates — subtracting
// ScreenOrigin puts the crop corner back on the cursor.
func (p *sourcePreview) localMouse(me *event.MouseEvent) image.Point {
	return localMousePoint(p, me.Position, p.dragging || p.panning)
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

// destPreview shows a processed image fitted into its bounds, with the same
// zoom and pan controls as the source preview. Here a plain left drag pans,
// because there is nothing to select.
type destPreview struct {
	widget.WidgetBase
	zoomable

	rev       state.ReadonlySignal[uint64]
	provider  func() previewFrame
	source    image.Image
	image     image.Image
	full      image.Image
	emptyHint func() string
}

func newDestPreview(rev state.ReadonlySignal[uint64]) *destPreview {
	p := &destPreview{rev: rev}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

// SetProvider shows the image returned by f, scaled to fit.
func (p *destPreview) SetProvider(f func() image.Image) {
	if f == nil {
		p.SetFrameProvider(nil)
		return
	}
	p.SetFrameProvider(func() previewFrame {
		img := f()
		return previewFrame{image: img, size: imageSizeOf(img)}
	})
}

// SetFrameProvider also supplies the full-resolution image and the original
// pixel size, so the zoom readout and zoomed-in detail stay accurate when the
// preview raster is a downscaled copy.
func (p *destPreview) SetFrameProvider(f func() previewFrame) {
	p.provider = f
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetImage(img image.Image) {
	p.image = img
	p.source = nil
	p.setImageSize(imageSizeOf(img))
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetSource(img image.Image) {
	p.source = img
	p.image = nil
	p.setImageSize(imageSizeOf(img))
	p.SetNeedsRedraw(true)
}

func (p *destPreview) SetEmptyHint(f func() string) {
	p.emptyHint = f
}

func (p *destPreview) HasImage() bool {
	if p.provider != nil {
		return p.provider().image != nil
	}
	return p.source != nil || p.image != nil
}

func (p *destPreview) resolvedImage() image.Image {
	if p.rev != nil {
		_ = p.rev.Get()
	}
	if p.provider != nil {
		frame := p.provider()
		p.image = frame.image
		p.full = frame.full
		p.setImageSize(frame.size)
		return p.image
	}
	if p.image != nil {
		return p.image
	}
	if p.source == nil {
		return nil
	}
	p.image = previewImage(p.source)
	p.full = p.source
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
		view := toImageRect(b)
		disp := p.view.displayRect(view, p.imageSize)
		drawPreviewImage(canvas, sharpestRaster(img, p.full, disp), disp, view, p.imageSize, image.Rectangle{})
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

func (p *destPreview) Event(ctx widget.Context, e event.Event) bool {
	if p.resolvedImage() == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return false
	}
	view := toImageRect(p.Bounds())
	if we, ok := e.(*event.WheelEvent); ok {
		return applyWheelZoom(ctx, p, &p.zoomable, we, view)
	}
	me, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	pt := localMousePoint(p, me.Position, p.panning)
	switch me.MouseType {
	case event.MousePress:
		if me.Button != event.ButtonLeft && me.Button != event.ButtonMiddle {
			return false
		}
		if !pt.In(view) || !p.zoomed(view) {
			return false
		}
		p.beginPan(ctx, p, pt)
		return true
	case event.MouseDrag, event.MouseMove:
		if p.continuePan(ctx, pt, view) {
			return true
		}
		if pt.In(view) && p.zoomed(view) {
			ctx.SetCursor(widget.CursorMove)
			return true
		}
	case event.MouseRelease:
		return p.endPan(ctx, p, me)
	}
	return false
}

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

// previewZoomer is what the zoom buttons next to a panel title drive.
type previewZoomer interface {
	ZoomIn()
	ZoomOut()
	ZoomFit()
	ZoomPercent() int
}

func (p *sourcePreview) ZoomIn()          { zoomFromButton(p, &p.zoomable, zoomStepFactor) }
func (p *sourcePreview) ZoomOut()         { zoomFromButton(p, &p.zoomable, 1/zoomStepFactor) }
func (p *sourcePreview) ZoomFit()         { fitFromButton(p, &p.zoomable) }
func (p *sourcePreview) ZoomPercent() int { return zoomPercentOf(p, &p.zoomable) }

func (p *destPreview) ZoomIn()          { zoomFromButton(p, &p.zoomable, zoomStepFactor) }
func (p *destPreview) ZoomOut()         { zoomFromButton(p, &p.zoomable, 1/zoomStepFactor) }
func (p *destPreview) ZoomFit()         { fitFromButton(p, &p.zoomable) }
func (p *destPreview) ZoomPercent() int { return zoomPercentOf(p, &p.zoomable) }

// previewWidget is the part of a preview the shared zoom helpers need.
type previewWidget interface {
	widget.Widget
	Bounds() geometry.Rect
	ScreenOrigin() geometry.Point
	IsScreenOriginValid() bool
	SetNeedsRedraw(bool)
	sync()
}

func (p *destPreview) sync() { _ = p.resolvedImage() }

func zoomFromButton(w previewWidget, z *zoomable, factor float32) {
	w.sync()
	view := toImageRect(w.Bounds())
	center := image.Pt(view.Min.X+view.Dx()/2, view.Min.Y+view.Dy()/2)
	if !z.view.zoomBy(view, z.imageSize, factor, center) {
		return
	}
	w.SetNeedsRedraw(true)
	z.notifyZoom()
}

func fitFromButton(w previewWidget, z *zoomable) {
	w.sync()
	if z.view.fitted() {
		return
	}
	z.view.reset()
	w.SetNeedsRedraw(true)
	z.notifyZoom()
}

func zoomPercentOf(w previewWidget, z *zoomable) int {
	w.sync()
	return z.view.percent(toImageRect(w.Bounds()), z.imageSize)
}

func applyWheelZoom(ctx widget.Context, w previewWidget, z *zoomable, we *event.WheelEvent, view image.Rectangle) bool {
	anchor := image.Pt(int(math.Round(float64(we.Position.X))), int(math.Round(float64(we.Position.Y))))
	if !anchor.In(view) {
		return false
	}
	if !z.view.zoomBy(view, z.imageSize, wheelZoomFactor(we.DeltaY()), anchor) {
		return true
	}
	w.SetNeedsRedraw(true)
	if ctx != nil {
		ctx.InvalidateRect(w.Bounds())
	}
	z.notifyZoom()
	return true
}

// startsPan reports whether a press should start a pan instead of the
// widget's normal left-drag action.
func startsPan(me *event.MouseEvent) bool {
	if me.Button == event.ButtonMiddle {
		return true
	}
	return me.Button == event.ButtonLeft && me.Modifiers().IsShift()
}

func (z *zoomable) beginPan(ctx widget.Context, w previewWidget, pt image.Point) {
	z.panning = true
	z.panLast = pt
	capturePointer(ctx, w)
	if ctx != nil {
		ctx.SetCursor(widget.CursorMove)
	}
	w.SetNeedsRedraw(true)
}

func (z *zoomable) continuePan(ctx widget.Context, pt image.Point, view image.Rectangle) bool {
	if !z.panning {
		return false
	}
	if z.view.panBy(view, z.imageSize, pt.Sub(z.panLast)) {
		z.notifyZoom()
	}
	z.panLast = pt
	if ctx != nil {
		ctx.SetCursor(widget.CursorMove)
	}
	return true
}

func (z *zoomable) endPan(ctx widget.Context, w previewWidget, me *event.MouseEvent) bool {
	if !z.panning {
		return false
	}
	if me.Button != event.ButtonLeft && me.Button != event.ButtonMiddle {
		return false
	}
	z.panning = false
	releasePointer(ctx, w)
	w.SetNeedsRedraw(true)
	return true
}

func capturePointer(ctx widget.Context, w widget.Widget) {
	if cap, ok := ctx.(widget.PointerCapturer); ok {
		cap.CapturePointer(w)
	}
}

func releasePointer(ctx widget.Context, w widget.Widget) {
	if cap, ok := ctx.(widget.PointerCapturer); ok {
		cap.ReleasePointer(w)
	}
}

// localMousePoint maps a pointer position into the widget's bounds space.
// While the pointer is captured the window reports window coordinates.
func localMousePoint(w previewWidget, pos geometry.Point, captured bool) image.Point {
	if captured && w.IsScreenOriginValid() {
		pos = pos.Sub(w.ScreenOrigin()).Add(w.Bounds().Min)
	}
	return image.Pt(int(math.Round(float64(pos.X))), int(math.Round(float64(pos.Y))))
}

func imageSizeOf(img image.Image) image.Point {
	if img == nil {
		return image.Point{}
	}
	return img.Bounds().Size()
}

// sharpestRaster picks the image to sample from. preview is a cheap
// downscaled copy; full is used once the display is larger than it.
func sharpestRaster(preview, full image.Image, disp image.Rectangle) image.Image {
	if preview == nil {
		return full
	}
	if full == nil || disp.Empty() {
		return preview
	}
	if disp.Dx() <= preview.Bounds().Dx() {
		return preview
	}
	return full
}

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

// drawFittedImage scales src into fitted. It is the thumbnail path, where the
// image always fits its slot.
func drawFittedImage(canvas widget.Canvas, src image.Image, fitted image.Rectangle) {
	drawPreviewImage(canvas, src, fitted, fitted, imageSizeOf(src), image.Rectangle{})
}

// drawPreviewImage paints the part of src that falls inside view when the
// whole image occupies disp, baking the crop overlay into the same raster.
func drawPreviewImage(canvas widget.Canvas, src image.Image, disp, view image.Rectangle, imgSize image.Point, crop image.Rectangle) {
	tile, origin := previewTile(src, disp, view)
	if tile == nil {
		return
	}
	if imgSize.X > 0 && imgSize.Y > 0 && !crop.Empty() {
		tile = cloneRGBA(tile)
		applyCropHighlight(tile, imageRectToScreen(crop, imgSize, disp).Sub(origin))
	}
	canvas.DrawImage(tile, geometry.Pt(float32(origin.X), float32(origin.Y)))
}

// previewTile rasterizes only the visible part of src and returns it with the
// position to draw it at. Scaling the whole image would allocate gigabytes
// once the user zooms in.
func previewTile(src image.Image, disp, view image.Rectangle) (*image.RGBA, image.Point) {
	if src == nil || disp.Empty() {
		return nil, image.Point{}
	}
	visible := disp.Intersect(view)
	if visible.Empty() {
		return nil, image.Point{}
	}
	sb := src.Bounds()
	if sb.Dx() <= 0 || sb.Dy() <= 0 {
		return nil, image.Point{}
	}
	scaleX := float64(disp.Dx()) / float64(sb.Dx())
	scaleY := float64(disp.Dy()) / float64(sb.Dy())
	tile := image.Rect(
		sb.Min.X+int(math.Floor(float64(visible.Min.X-disp.Min.X)/scaleX)),
		sb.Min.Y+int(math.Floor(float64(visible.Min.Y-disp.Min.Y)/scaleY)),
		sb.Min.X+int(math.Ceil(float64(visible.Max.X-disp.Min.X)/scaleX)),
		sb.Min.Y+int(math.Ceil(float64(visible.Max.Y-disp.Min.Y)/scaleY)),
	).Intersect(sb)
	if tile.Empty() {
		return nil, image.Point{}
	}
	outW := max(1, int(math.Round(float64(tile.Dx())*scaleX)))
	outH := max(1, int(math.Round(float64(tile.Dy())*scaleY)))
	origin := image.Pt(
		disp.Min.X+int(math.Round(float64(tile.Min.X-sb.Min.X)*scaleX)),
		disp.Min.Y+int(math.Round(float64(tile.Min.Y-sb.Min.Y)*scaleY)),
	)
	rgba := toDisplayRGBA(scaleTile(src, tile, outW, outH, math.Min(scaleX, scaleY)))
	clip := visible.Sub(origin).Intersect(image.Rect(0, 0, outW, outH))
	if clip.Empty() {
		return nil, image.Point{}
	}
	if clip != image.Rect(0, 0, outW, outH) {
		rgba = cropRGBA(rgba, clip)
		origin = origin.Add(clip.Min)
	}
	return rgba, origin
}

// scaleTile resamples the tile of src to w×h. Deep zoom keeps hard pixel
// edges; Catmull-Rom would turn them into mush.
func scaleTile(src image.Image, tile image.Rectangle, w, h int, scale float64) image.Image {
	sub := subImage(src, tile)
	if w == tile.Dx() && h == tile.Dy() {
		return sub
	}
	if scale >= pixelZoomScale {
		return imageproc.ResizeNearest(sub, w, h)
	}
	return imageproc.Resize(sub, w, h)
}

func subImage(src image.Image, r image.Rectangle) image.Image {
	if r == src.Bounds() {
		return src
	}
	if si, ok := src.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return si.SubImage(r)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
	return dst
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

// cropRGBA copies r out of src into a fresh origin-aligned buffer.
func cropRGBA(src *image.RGBA, r image.Rectangle) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
	return dst
}

const cropDimMul = 112 // 112/256 ≈ remaining luminance under 0x90 black overlay

var cropBorder = color.RGBA{R: 0x2f, G: 0x81, B: 0xff, A: 0xff}

// applyCropHighlight darkens pixels outside screen, the crop rectangle in
// dst-local coordinates, and draws a 2px accent border around it.
// Overlay is baked into the bitmap because GPU DrawImage is composited over
// later DrawRect/StrokeRect, so a separate highlight pass is invisible.
func applyCropHighlight(dst *image.RGBA, screen image.Rectangle) {
	if dst == nil || screen.Empty() {
		return
	}
	b := dst.Rect
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
