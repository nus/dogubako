package app

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
)

// sourcePreview shows the original image and lets the user drag a crop rectangle.
type sourcePreview struct {
	guigui.DefaultWidget

	image         *ebiten.Image
	imageSize     image.Point
	crop          image.Rectangle
	cropEnabled   bool
	generation    uint64
	onCropChanged func(image.Rectangle)

	dragging  bool
	dragStart image.Point
	dragRect  image.Rectangle
}

func (p *sourcePreview) SetImage(img *ebiten.Image, srcSize image.Point) {
	if p.image == img && p.imageSize == srcSize {
		return
	}
	p.image = img
	p.imageSize = srcSize
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

func (p *sourcePreview) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteUint64(p.generation)
	w.WriteBool(p.cropEnabled)
	w.WriteInt(p.crop.Min.X)
	w.WriteInt(p.crop.Min.Y)
	w.WriteInt(p.crop.Max.X)
	w.WriteInt(p.crop.Max.Y)
}

func (p *sourcePreview) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	drawCheckerboard(dst, b)
	if p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return
	}

	fitted := fittedRect(b, p.imageSize)
	drawFittedImage(dst, p.image, fitted)

	crop := p.crop
	if p.dragging {
		crop = p.dragRect
	}
	if p.cropEnabled || p.dragging {
		screenCrop := imageRectToScreen(crop, p.imageSize, fitted)
		if !screenCrop.Empty() {
			dim := color.NRGBA{A: 0x90}
			vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(screenCrop.Min.X-b.Min.X), float32(b.Dy()), dim, false)
			vector.FillRect(dst, float32(screenCrop.Max.X), float32(b.Min.Y), float32(b.Max.X-screenCrop.Max.X), float32(b.Dy()), dim, false)
			vector.FillRect(dst, float32(screenCrop.Min.X), float32(b.Min.Y), float32(screenCrop.Dx()), float32(screenCrop.Min.Y-b.Min.Y), dim, false)
			vector.FillRect(dst, float32(screenCrop.Min.X), float32(screenCrop.Max.Y), float32(screenCrop.Dx()), float32(b.Max.Y-screenCrop.Max.Y), dim, false)
			vector.StrokeRect(dst, float32(screenCrop.Min.X)+0.5, float32(screenCrop.Min.Y)+0.5, float32(screenCrop.Dx())-1, float32(screenCrop.Dy())-1, 2, color.NRGBA{R: 0x2f, G: 0x81, B: 0xff, A: 0xff}, false)
		}
	}
}

func (p *sourcePreview) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if p.image == nil || p.imageSize.X <= 0 || p.imageSize.Y <= 0 {
		return guigui.HandleInputResult{}
	}
	fitted := fittedRect(widgetBounds.Bounds(), p.imageSize)
	if fitted.Empty() {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	pt := image.Pt(cx, cy)

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

// destPreview shows the processed image fitted into its bounds.
type destPreview struct {
	guigui.DefaultWidget

	image      *ebiten.Image
	generation uint64
}

func (p *destPreview) SetImage(img *ebiten.Image) {
	if p.image == img {
		return
	}
	p.image = img
	p.generation++
}

func (p *destPreview) HasImage() bool { return p.image != nil }

func (p *destPreview) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteUint64(p.generation)
}

func (p *destPreview) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	drawCheckerboard(dst, b)
	if p.image == nil {
		return
	}
	drawFittedImage(dst, p.image, fittedRect(b, p.image.Bounds().Size()))
}

func (p *sourcePreview) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if p.image != nil && widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeCrosshair, true
	}
	return 0, false
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

func drawFittedImage(dst *ebiten.Image, src *ebiten.Image, fitted image.Rectangle) {
	if src == nil || fitted.Empty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	srcSize := src.Bounds().Size()
	op.GeoM.Scale(float64(fitted.Dx())/float64(srcSize.X), float64(fitted.Dy())/float64(srcSize.Y))
	op.GeoM.Translate(float64(fitted.Min.X), float64(fitted.Min.Y))
	dst.DrawImage(src, op)
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
