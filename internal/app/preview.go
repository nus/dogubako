package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

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
	clearEbitenImage(&p.gpu)
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
		drawCropOverlay(dst, b, crop, p.imageSize, full)
	}
	drawZoomBadge(dst, b, p.view.Scale(), context.Scale())
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
	clearEbitenImage(&p.gpu)
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
	drawZoomBadge(dst, b, p.view.Scale(), context.Scale())
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

func clearEbitenImage(dst **ebiten.Image) {
	if dst == nil || *dst == nil {
		return
	}
	(*dst).Deallocate()
	*dst = nil
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
	if full.Empty() || full.Intersect(bounds).Empty() {
		return
	}
	srcSize := src.Bounds().Size()
	if srcSize.X <= 0 || srcSize.Y <= 0 {
		return
	}
	// Scale the whole raster into `full` so a crop overlay mapped through
	// the same rectangle stays on the pixels. Stretching only the visible
	// source subset to the widget made the blue frame drift under zoom.
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Translate(-float64(src.Bounds().Min.X), -float64(src.Bounds().Min.Y))
	op.GeoM.Scale(float64(full.Dx())/float64(srcSize.X), float64(full.Dy())/float64(srcSize.Y))
	op.GeoM.Translate(float64(full.Min.X), float64(full.Min.Y))
	dst.DrawImage(src, op)
}

// zoomBadgeSizeAtScale1 is the badge font size in logical pixels. UI body text
// is 12px; ebitenutil.DebugPrint is about 8px.
const zoomBadgeSizeAtScale1 = 16

var (
	zoomBadgeFontOnce sync.Once
	zoomBadgeFont     *text.GoTextFaceSource
)

func zoomBadgeLabel(scale float64) string {
	return fmt.Sprintf("%d%%", int(math.Round(scale*100)))
}

func zoomBadgeFontSize(uiScale float64) float64 {
	if uiScale <= 0 {
		uiScale = 1
	}
	return zoomBadgeSizeAtScale1 * uiScale
}

func zoomBadgeTextFace(uiScale float64) *text.GoTextFace {
	zoomBadgeFontOnce.Do(func() {
		src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			return
		}
		zoomBadgeFont = src
	})
	if zoomBadgeFont == nil {
		return nil
	}
	return &text.GoTextFace{Source: zoomBadgeFont, Size: zoomBadgeFontSize(uiScale)}
}

func drawZoomBadge(dst *ebiten.Image, bounds image.Rectangle, scale, uiScale float64) {
	if bounds.Empty() || math.Abs(scale-1) < 0.001 {
		return
	}
	label := zoomBadgeLabel(scale)
	face := zoomBadgeTextFace(uiScale)
	if face == nil {
		return
	}
	if uiScale <= 0 {
		uiScale = 1
	}
	m := face.Metrics()
	tw, th := text.Measure(label, face, m.HAscent+m.HDescent)
	padX := 6 * uiScale
	padY := 4 * uiScale
	margin := 8 * uiScale
	bw := tw + padX*2
	bh := th + padY*2
	x := float64(bounds.Max.X) - margin - bw
	y := float64(bounds.Max.Y) - margin - bh
	if x < float64(bounds.Min.X)+margin {
		x = float64(bounds.Min.X) + margin
	}
	if y < float64(bounds.Min.Y)+margin {
		y = float64(bounds.Min.Y) + margin
	}
	vector.FillRect(dst, float32(x), float32(y), float32(bw), float32(bh), color.NRGBA{A: 150}, false)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x+padX, y+padY)
	text.Draw(dst, label, face, op)
}

func imageRectToScreenF(rect image.Rectangle, imgSize image.Point, fitted image.Rectangle) (minX, minY, maxX, maxY float32) {
	if imgSize.X <= 0 || imgSize.Y <= 0 || fitted.Empty() || rect.Empty() {
		return 0, 0, 0, 0
	}
	sx := float32(fitted.Dx()) / float32(imgSize.X)
	sy := float32(fitted.Dy()) / float32(imgSize.Y)
	minX = float32(fitted.Min.X) + float32(rect.Min.X)*sx
	minY = float32(fitted.Min.Y) + float32(rect.Min.Y)*sy
	maxX = float32(fitted.Min.X) + float32(rect.Max.X)*sx
	maxY = float32(fitted.Min.Y) + float32(rect.Max.Y)*sy
	return minX, minY, maxX, maxY
}

func imageRectToScreen(rect image.Rectangle, imgSize image.Point, fitted image.Rectangle) image.Rectangle {
	x0, y0, x1, y1 := imageRectToScreenF(rect, imgSize, fitted)
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(int(math.Round(float64(x0))), int(math.Round(float64(y0))), int(math.Round(float64(x1))), int(math.Round(float64(y1))))
}

func drawCropOverlay(dst *ebiten.Image, bounds, crop image.Rectangle, imgSize image.Point, full image.Rectangle) {
	x0, y0, x1, y1 := imageRectToScreenF(crop, imgSize, full)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	dim := color.NRGBA{A: 0x90}
	bx0, by0 := float32(bounds.Min.X), float32(bounds.Min.Y)
	bx1, by1 := float32(bounds.Max.X), float32(bounds.Max.Y)
	if w := clampf32(x0, bx0, bx1) - bx0; w > 0 {
		vector.FillRect(dst, bx0, by0, w, by1-by0, dim, false)
	}
	if w := bx1 - clampf32(x1, bx0, bx1); w > 0 {
		vector.FillRect(dst, clampf32(x1, bx0, bx1), by0, w, by1-by0, dim, false)
	}
	cx0 := clampf32(x0, bx0, bx1)
	cx1 := clampf32(x1, bx0, bx1)
	if cx1 > cx0 {
		if h := clampf32(y0, by0, by1) - by0; h > 0 {
			vector.FillRect(dst, cx0, by0, cx1-cx0, h, dim, false)
		}
		if h := by1 - clampf32(y1, by0, by1); h > 0 {
			vector.FillRect(dst, cx0, clampf32(y1, by0, by1), cx1-cx0, h, dim, false)
		}
	}
	vector.StrokeRect(dst, x0+0.5, y0+0.5, (x1-x0)-1, (y1-y0)-1, 2, color.NRGBA{R: 0x2f, G: 0x81, B: 0xff, A: 0xff}, false)
}

func clampf32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func screenToImage(pt image.Point, fitted image.Rectangle, imgSize image.Point) image.Point {
	if fitted.Empty() || imgSize.X <= 0 || imgSize.Y <= 0 {
		return image.Point{}
	}
	x := int(math.Floor(float64(pt.X-fitted.Min.X) * float64(imgSize.X) / float64(fitted.Dx())))
	y := int(math.Floor(float64(pt.Y-fitted.Min.Y) * float64(imgSize.Y) / float64(fitted.Dy())))
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
