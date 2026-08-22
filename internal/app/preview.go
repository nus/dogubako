package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

const previewDoubleClick = 280 * time.Millisecond

// sourcePreview shows the original image and lets the user drag a crop rectangle.
type sourcePreview struct {
	guigui.DefaultWidget

	source        image.Image
	gpu           *ebiten.Image
	imageSize     image.Point
	crop          image.Rectangle
	cropEnabled   bool
	generation    uint64
	onCropChanged func(image.Rectangle)

	view      previewView
	bounds    image.Rectangle
	dragging  bool
	panning   bool
	panLast   image.Point
	dragStart image.Point
	dragRect  image.Rectangle
	lastClick time.Time
}

func (p *sourcePreview) SetImage(img image.Image, srcSize image.Point) {
	if p.source == img && p.imageSize == srcSize {
		return
	}
	p.source = img
	p.gpu = nil
	p.imageSize = srcSize
	p.view.syncKey(srcSize)
	p.generation++
}

func (p *sourcePreview) SetCrop(crop image.Rectangle, enabled bool) {
	if p.crop == crop && p.cropEnabled == enabled {
		return
	}
	p.crop = crop
	p.cropEnabled = enabled
	p.generation++
}

func (p *sourcePreview) OnCropChanged(f func(image.Rectangle)) {
	p.onCropChanged = f
}

func (p *sourcePreview) HasImage() bool {
	return p.source != nil && p.imageSize.X > 0 && p.imageSize.Y > 0
}

func (p *sourcePreview) ZoomIn() {
	p.view.zoomIn(p.bounds, p.imageSize)
	p.notifyView()
}

func (p *sourcePreview) ZoomOut() {
	p.view.zoomOut(p.bounds, p.imageSize)
	p.notifyView()
}

func (p *sourcePreview) ResetZoom() {
	p.view.Reset()
	p.notifyView()
}

func (p *sourcePreview) notifyView() {
	p.generation++
	guigui.RequestRedraw(p)
}

func (p *sourcePreview) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteUint64(p.generation)
	w.WriteBool(p.cropEnabled)
	w.WriteInt(p.crop.Min.X)
	w.WriteInt(p.crop.Min.Y)
	w.WriteInt(p.crop.Max.X)
	w.WriteInt(p.crop.Max.Y)
	w.WriteFloat64(p.view.Scale())
}

func (p *sourcePreview) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	p.bounds = widgetBounds.Bounds()
}

func (p *sourcePreview) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	p.bounds = b
	drawCheckerboard(dst, b)
	if p.source == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return
	}
	if p.gpu == nil {
		p.gpu = ebiten.NewImageFromImage(p.source)
	}
	p.view.syncKey(p.imageSize)
	drawViewedImage(dst, p.gpu, b, p.imageSize, &p.view)

	crop := p.crop
	if p.dragging {
		crop = p.dragRect
	}
	if p.cropEnabled || p.dragging {
		full := p.view.imageRect(b, p.imageSize)
		screenCrop := imageRectToScreen(crop, p.imageSize, full)
		if !screenCrop.Empty() {
			dim := color.NRGBA{A: 0x90}
			vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(screenCrop.Min.X-b.Min.X), float32(b.Dy()), dim, false)
			vector.FillRect(dst, float32(screenCrop.Max.X), float32(b.Min.Y), float32(b.Max.X-screenCrop.Max.X), float32(b.Dy()), dim, false)
			vector.FillRect(dst, float32(screenCrop.Min.X), float32(b.Min.Y), float32(screenCrop.Dx()), float32(screenCrop.Min.Y-b.Min.Y), dim, false)
			vector.FillRect(dst, float32(screenCrop.Min.X), float32(screenCrop.Max.Y), float32(screenCrop.Dx()), float32(b.Max.Y-screenCrop.Max.Y), dim, false)
			vector.StrokeRect(dst, float32(screenCrop.Min.X)+0.5, float32(screenCrop.Min.Y)+0.5, float32(screenCrop.Dx())-1, float32(screenCrop.Dy())-1, 2, color.NRGBA{R: 0x2f, G: 0x81, B: 0xff, A: 0xff}, false)
		}
	}
	drawZoomBadge(dst, b, p.view.Scale())
}

func (p *sourcePreview) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if p.source == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	p.bounds = b
	p.view.syncKey(p.imageSize)
	fitted := p.view.imageRect(b, p.imageSize)
	if fitted.Empty() {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	pt := image.Pt(cx, cy)

	if _, wy := ebiten.Wheel(); wy != 0 && widgetBounds.IsHitAtCursor() {
		p.view.zoomAt(b, p.imageSize, pt, previewZoomFactor(-wy))
		p.notifyView()
		return guigui.HandleInputByWidget(p)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && widgetBounds.IsHitAtCursor() {
		now := time.Now()
		if !p.lastClick.IsZero() && now.Sub(p.lastClick) < previewDoubleClick {
			p.view.Reset()
			p.lastClick = time.Time{}
			p.notifyView()
			return guigui.HandleInputByWidget(p)
		}
		p.lastClick = now
	}

	if (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)) && widgetBounds.IsHitAtCursor() {
		p.panning = true
		p.panLast = pt
		return guigui.HandleInputByWidget(p)
	}
	if p.panning && (ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)) {
		p.view.panBy(float64(pt.X-p.panLast.X), float64(pt.Y-p.panLast.Y), b, p.imageSize)
		p.panLast = pt
		p.notifyView()
		return guigui.HandleInputByWidget(p)
	}
	if p.panning {
		p.panning = false
		return guigui.HandleInputByWidget(p)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && widgetBounds.IsHitAtCursor() && pt.In(fitted) {
		p.dragging = true
		p.dragStart = screenToImage(pt, fitted, p.imageSize)
		p.dragRect = image.Rectangle{Min: p.dragStart, Max: p.dragStart.Add(image.Pt(1, 1))}.Canon()
		return guigui.HandleInputByWidget(p)
	}
	if p.dragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cur := screenToImage(pt, fitted, p.imageSize)
		p.dragRect = normalizeCrop(image.Rectangle{Min: p.dragStart, Max: cur.Add(image.Pt(1, 1))}, p.imageSize)
		guigui.RequestRedraw(p)
		return guigui.HandleInputByWidget(p)
	}
	if p.dragging && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		p.dragging = false
		rect := normalizeCrop(p.dragRect, p.imageSize)
		if rect.Dx() >= 1 && rect.Dy() >= 1 && p.onCropChanged != nil {
			p.onCropChanged(rect)
		}
		return guigui.HandleInputByWidget(p)
	}
	return guigui.HandleInputResult{}
}

func (p *sourcePreview) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if p.panning {
		return ebiten.CursorShapeMove, true
	}
	if p.source != nil && widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeCrosshair, true
	}
	return 0, false
}

// destPreview shows the processed image fitted into its bounds.
type destPreview struct {
	guigui.DefaultWidget

	source     image.Image
	gpu        *ebiten.Image
	logical    image.Point
	generation uint64

	view      previewView
	bounds    image.Rectangle
	panning   bool
	panLast   image.Point
	lastClick time.Time
}

func (p *destPreview) SetImage(img image.Image) {
	if p.source == img {
		return
	}
	p.source = img
	p.gpu = nil
	if img == nil {
		p.logical = image.Point{}
	}
	p.generation++
}

func (p *destPreview) SetLogicalSize(size image.Point) {
	if p.logical == size {
		return
	}
	p.logical = size
	p.view.syncKey(p.viewSize())
	p.generation++
}

func (p *destPreview) SetSource(img image.Image) {
	p.SetImage(img)
}

func (p *destPreview) HasImage() bool { return p.source != nil }

func (p *destPreview) viewSize() image.Point {
	if p.logical.X > 0 && p.logical.Y > 0 {
		return p.logical
	}
	if p.source == nil {
		return image.Point{}
	}
	return p.source.Bounds().Size()
}

func (p *destPreview) ZoomIn() {
	p.view.zoomIn(p.bounds, p.viewSize())
	p.notifyView()
}

func (p *destPreview) ZoomOut() {
	p.view.zoomOut(p.bounds, p.viewSize())
	p.notifyView()
}

func (p *destPreview) ResetZoom() {
	p.view.Reset()
	p.notifyView()
}

func (p *destPreview) notifyView() {
	p.generation++
	guigui.RequestRedraw(p)
}

func (p *destPreview) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteUint64(p.generation)
	w.WriteFloat64(p.view.Scale())
}

func (p *destPreview) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	p.bounds = widgetBounds.Bounds()
}

func (p *destPreview) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	p.bounds = b
	drawCheckerboard(dst, b)
	if p.source == nil {
		return
	}
	if p.gpu == nil {
		p.gpu = ebiten.NewImageFromImage(p.source)
	}
	sz := p.viewSize()
	p.view.syncKey(sz)
	drawViewedImage(dst, p.gpu, b, sz, &p.view)
	drawZoomBadge(dst, b, p.view.Scale())
}

func (p *destPreview) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if p.source == nil {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	p.bounds = b
	sz := p.viewSize()
	p.view.syncKey(sz)
	cx, cy := ebiten.CursorPosition()
	pt := image.Pt(cx, cy)

	if _, wy := ebiten.Wheel(); wy != 0 && widgetBounds.IsHitAtCursor() {
		p.view.zoomAt(b, sz, pt, previewZoomFactor(-wy))
		p.notifyView()
		return guigui.HandleInputByWidget(p)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && widgetBounds.IsHitAtCursor() {
		now := time.Now()
		if !p.lastClick.IsZero() && now.Sub(p.lastClick) < previewDoubleClick {
			p.view.Reset()
			p.lastClick = time.Time{}
			p.notifyView()
			return guigui.HandleInputByWidget(p)
		}
		p.lastClick = now
		p.panning = true
		p.panLast = pt
		return guigui.HandleInputByWidget(p)
	}
	if p.panning && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.view.panBy(float64(pt.X-p.panLast.X), float64(pt.Y-p.panLast.Y), b, sz)
		p.panLast = pt
		p.notifyView()
		return guigui.HandleInputByWidget(p)
	}
	if p.panning {
		p.panning = false
		return guigui.HandleInputByWidget(p)
	}
	return guigui.HandleInputResult{}
}

func (p *destPreview) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if p.source == nil || !widgetBounds.IsHitAtCursor() {
		return 0, false
	}
	if p.panning || previewCanPan(widgetBounds.Bounds(), p.viewSize(), &p.view) {
		return ebiten.CursorShapeMove, true
	}
	return 0, false
}

type previewZoomBar struct {
	guigui.DefaultWidget

	minus basicwidget.Button
	plus  basicwidget.Button
	fit   basicwidget.Button

	layoutItems []guigui.LinearLayoutItem
}

func (z *previewZoomBar) Configure(context *guigui.Context, zoomer previewZoomer, lang i18n.Lang, enabled bool) {
	z.minus.SetText("−")
	z.plus.SetText("+")
	z.fit.SetText(i18n.T(lang, i18n.PreviewFit))
	z.minus.OnDown(func(context *guigui.Context) {
		if zoomer != nil {
			zoomer.ZoomOut()
		}
	})
	z.plus.OnDown(func(context *guigui.Context) {
		if zoomer != nil {
			zoomer.ZoomIn()
		}
	})
	z.fit.OnDown(func(context *guigui.Context) {
		if zoomer != nil {
			zoomer.ResetZoom()
		}
	})
	context.SetEnabled(&z.minus, enabled)
	context.SetEnabled(&z.plus, enabled)
	context.SetEnabled(&z.fit, enabled)
}

func (z *previewZoomBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&z.minus)
	adder.AddWidget(&z.plus)
	adder.AddWidget(&z.fit)
	return nil
}

func (z *previewZoomBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	z.layoutItems = slices.Delete(z.layoutItems, 0, len(z.layoutItems))
	z.layoutItems = append(z.layoutItems,
		guigui.LinearLayoutItem{Widget: &z.minus},
		guigui.LinearLayoutItem{Widget: &z.plus},
		guigui.LinearLayoutItem{Widget: &z.fit},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     z.layoutItems,
		Gap:       u / 8,
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
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

func drawViewedImage(dst, src *ebiten.Image, bounds image.Rectangle, imgSize image.Point, view *previewView) {
	if src == nil || imgSize.X <= 0 || imgSize.Y <= 0 {
		return
	}
	full := view.imageRect(bounds, imgSize)
	if full.Empty() {
		return
	}
	visible := full.Intersect(bounds)
	if visible.Empty() {
		return
	}
	srcImg := visibleSourceRect(full, visible, imgSize)
	if srcImg.Empty() {
		return
	}
	raster := mapImageRect(src.Bounds(), imgSize, srcImg)
	if raster.Empty() {
		return
	}
	sub, ok := src.SubImage(raster).(*ebiten.Image)
	if !ok {
		return
	}
	subSize := sub.Bounds().Size()
	if subSize.X <= 0 || subSize.Y <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Translate(-float64(sub.Bounds().Min.X), -float64(sub.Bounds().Min.Y))
	op.GeoM.Scale(float64(visible.Dx())/float64(subSize.X), float64(visible.Dy())/float64(subSize.Y))
	op.GeoM.Translate(float64(visible.Min.X), float64(visible.Min.Y))
	dst.DrawImage(sub, op)
}

func drawZoomBadge(dst *ebiten.Image, bounds image.Rectangle, scale float64) {
	if bounds.Empty() || math.Abs(scale-1) < 0.001 {
		return
	}
	label := fmt.Sprintf("%d%%", int(math.Round(scale*100)))
	x := bounds.Max.X - 8 - 8*len(label)
	y := bounds.Max.Y - 20
	if x < bounds.Min.X+4 {
		x = bounds.Min.X + 4
	}
	if y < bounds.Min.Y+4 {
		y = bounds.Min.Y + 4
	}
	w := 8*len(label) + 10
	h := 16
	vector.FillRect(dst, float32(x-4), float32(y-2), float32(w), float32(h), color.NRGBA{A: 150}, false)
	ebitenutil.DebugPrintAt(dst, label, x, y)
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

func drawCheckerboard(dst *ebiten.Image, b image.Rectangle) {
	const cell = 8
	c0 := color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}
	c1 := color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
	vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), c0, false)
	for y := b.Min.Y; y < b.Max.Y; y += cell {
		for x := b.Min.X; x < b.Max.X; x += cell {
			if ((x-b.Min.X)/cell+(y-b.Min.Y)/cell)%2 == 0 {
				continue
			}
			w := min(cell, b.Max.X-x)
			h := min(cell, b.Max.Y-y)
			vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), c1, false)
		}
	}
}
