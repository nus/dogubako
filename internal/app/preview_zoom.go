package app

import (
	"fmt"
	"image"
	"math"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/imageproc"
)

const (
	previewZoomMin  = 0.25
	previewZoomMax  = 32.0
	previewZoomStep = 1.25
)

// previewZoomer is a preview that can scale its view of the image.
type previewZoomer interface {
	ZoomIn()
	ZoomOut()
	ResetZoom()
	HasImage() bool
}

// previewView is a 1=fit zoom and a pan offset in widget pixels.
type previewView struct {
	scale      float64
	panX, panY float64
	key        image.Point
}

func (v *previewView) Scale() float64 {
	if v == nil || v.scale <= 0 {
		return 1
	}
	return v.scale
}

func (v *previewView) Reset() {
	if v == nil {
		return
	}
	v.scale = 1
	v.panX = 0
	v.panY = 0
}

func (v *previewView) syncKey(size image.Point) {
	if v == nil {
		return
	}
	if v.key == size {
		return
	}
	prev := v.key
	v.key = size
	if prev == (image.Point{}) {
		return
	}
	v.Reset()
}

func (v *previewView) imageRect(bounds image.Rectangle, imgSize image.Point) image.Rectangle {
	if v == nil {
		return fittedRect(bounds, imgSize)
	}
	v.clamp(bounds, imgSize)
	return previewImageRect(bounds, imgSize, v.Scale(), v.panX, v.panY)
}

func (v *previewView) clamp(bounds image.Rectangle, imgSize image.Point) {
	if v == nil {
		return
	}
	fit := fittedRect(bounds, imgSize)
	if fit.Empty() {
		v.panX = 0
		v.panY = 0
		return
	}
	z := v.Scale()
	w := max(1, int(math.Round(float64(fit.Dx())*z)))
	h := max(1, int(math.Round(float64(fit.Dy())*z)))
	v.panX = clampPreviewPan(v.panX, w, bounds.Dx())
	v.panY = clampPreviewPan(v.panY, h, bounds.Dy())
}

func (v *previewView) zoomAt(bounds image.Rectangle, imgSize, cursor image.Point, factor float64) {
	if v == nil || factor <= 0 {
		return
	}
	v.scale, v.panX, v.panY = zoomPreviewAt(bounds, imgSize, v.Scale(), v.panX, v.panY, cursor, factor)
}

func (v *previewView) panBy(dx, dy float64, bounds image.Rectangle, imgSize image.Point) {
	if v == nil {
		return
	}
	v.panX += dx
	v.panY += dy
	v.clamp(bounds, imgSize)
}

func (v *previewView) zoomIn(bounds image.Rectangle, imgSize image.Point) {
	v.zoomAt(bounds, imgSize, previewCenter(bounds), previewZoomStep)
}

func (v *previewView) zoomOut(bounds image.Rectangle, imgSize image.Point) {
	v.zoomAt(bounds, imgSize, previewCenter(bounds), 1/previewZoomStep)
}

func previewCenter(bounds image.Rectangle) image.Point {
	return image.Pt(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2)
}

func previewImageRect(bounds image.Rectangle, imgSize image.Point, scale, panX, panY float64) image.Rectangle {
	fit := fittedRect(bounds, imgSize)
	if fit.Empty() {
		return image.Rectangle{}
	}
	if scale <= 0 {
		scale = 1
	}
	w := max(1, int(math.Round(float64(fit.Dx())*scale)))
	h := max(1, int(math.Round(float64(fit.Dy())*scale)))
	panX = clampPreviewPan(panX, w, bounds.Dx())
	panY = clampPreviewPan(panY, h, bounds.Dy())
	x := fit.Min.X + (fit.Dx()-w)/2 + int(math.Round(panX))
	y := fit.Min.Y + (fit.Dy()-h)/2 + int(math.Round(panY))
	return image.Rect(x, y, x+w, y+h)
}

func clampPreviewPan(pan float64, disp, bound int) float64 {
	if disp <= bound {
		return 0
	}
	maxPan := float64(disp-bound) / 2
	if pan > maxPan {
		return maxPan
	}
	if pan < -maxPan {
		return -maxPan
	}
	return pan
}

func clampPreviewZoom(scale float64) float64 {
	if scale < previewZoomMin {
		return previewZoomMin
	}
	if scale > previewZoomMax {
		return previewZoomMax
	}
	return scale
}

func zoomPreviewAt(bounds image.Rectangle, imgSize image.Point, scale, panX, panY float64, cursor image.Point, factor float64) (float64, float64, float64) {
	newScale := clampPreviewZoom(scale * factor)
	if newScale == scale {
		return scale, panX, panY
	}
	old := previewImageRect(bounds, imgSize, scale, panX, panY)
	if old.Empty() {
		return newScale, 0, 0
	}
	relX := 0.5
	relY := 0.5
	if old.Dx() > 0 {
		relX = float64(cursor.X-old.Min.X) / float64(old.Dx())
	}
	if old.Dy() > 0 {
		relY = float64(cursor.Y-old.Min.Y) / float64(old.Dy())
	}
	fit := fittedRect(bounds, imgSize)
	newW := max(1, int(math.Round(float64(fit.Dx())*newScale)))
	newH := max(1, int(math.Round(float64(fit.Dy())*newScale)))
	newMinX := float64(cursor.X) - relX*float64(newW)
	newMinY := float64(cursor.Y) - relY*float64(newH)
	newCx := newMinX + float64(newW)/2
	newCy := newMinY + float64(newH)/2
	fitCx := float64(fit.Min.X) + float64(fit.Dx())/2
	fitCy := float64(fit.Min.Y) + float64(fit.Dy())/2
	newPanX := clampPreviewPan(newCx-fitCx, newW, bounds.Dx())
	newPanY := clampPreviewPan(newCy-fitCy, newH, bounds.Dy())
	return newScale, newPanX, newPanY
}

func previewZoomFactor(deltaY float32) float64 {
	if deltaY == 0 {
		return 1
	}
	if deltaY > 0 {
		return 1 / previewZoomStep
	}
	return previewZoomStep
}

func visibleSourceRect(full, visible image.Rectangle, imgSize image.Point) image.Rectangle {
	if full.Empty() || visible.Empty() || imgSize.X <= 0 || imgSize.Y <= 0 {
		return image.Rectangle{}
	}
	x0 := int(math.Floor(float64(visible.Min.X-full.Min.X) * float64(imgSize.X) / float64(full.Dx())))
	y0 := int(math.Floor(float64(visible.Min.Y-full.Min.Y) * float64(imgSize.Y) / float64(full.Dy())))
	x1 := int(math.Ceil(float64(visible.Max.X-full.Min.X) * float64(imgSize.X) / float64(full.Dx())))
	y1 := int(math.Ceil(float64(visible.Max.Y-full.Min.Y) * float64(imgSize.Y) / float64(full.Dy())))
	return normalizeCrop(image.Rect(x0, y0, x1, y1), imgSize)
}

func mapImageRect(raster image.Rectangle, imgSize image.Point, srcImg image.Rectangle) image.Rectangle {
	if imgSize.X <= 0 || imgSize.Y <= 0 || raster.Empty() || srcImg.Empty() {
		return image.Rectangle{}
	}
	x0 := raster.Min.X + srcImg.Min.X*raster.Dx()/imgSize.X
	y0 := raster.Min.Y + srcImg.Min.Y*raster.Dy()/imgSize.Y
	x1 := raster.Min.X + srcImg.Max.X*raster.Dx()/imgSize.X
	y1 := raster.Min.Y + srcImg.Max.Y*raster.Dy()/imgSize.Y
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return image.Rect(x0, y0, x1, y1).Intersect(raster)
}

func drawViewedImage(canvas widget.Canvas, src image.Image, bounds geometry.Rect, imgSize image.Point, crop image.Rectangle, view *previewView) {
	if src == nil {
		return
	}
	ib := toImageRect(bounds)
	full := view.imageRect(ib, imgSize)
	if full.Empty() {
		return
	}
	visible := full.Intersect(ib)
	if visible.Empty() {
		return
	}
	srcImg := visibleSourceRect(full, visible, imgSize)
	if srcImg.Empty() {
		return
	}
	raster := mapImageRect(src.Bounds(), imgSize, srcImg)
	sub, err := imageproc.Crop(src, raster)
	if err != nil {
		drawFittedImage(canvas, src, visible, srcImg.Size(), image.Rectangle{})
		return
	}
	if sub.Bounds().Dx() != visible.Dx() || sub.Bounds().Dy() != visible.Dy() {
		sub = imageproc.Resize(sub, visible.Dx(), visible.Dy())
	}
	rgba := toDisplayRGBA(sub)
	if imgSize.X > 0 && imgSize.Y > 0 && !crop.Empty() {
		rgba = cloneRGBA(rgba)
		local := crop.Intersect(srcImg)
		if local.Empty() {
			dimRGBA(rgba, rgba.Rect)
		} else {
			applyCropHighlight(rgba, srcImg.Size(), local.Sub(srcImg.Min))
		}
	}
	canvas.PushClip(bounds)
	canvas.DrawImage(rgba, geometry.Pt(float32(visible.Min.X), float32(visible.Min.Y)))
	canvas.PopClip()
}

func drawZoomBadge(canvas widget.Canvas, bounds geometry.Rect, scale float64) {
	if bounds.IsEmpty() || math.Abs(scale-1) < 0.001 {
		return
	}
	label := fmt.Sprintf("%d%%", int(math.Round(scale*100)))
	padX := float32(8)
	padY := float32(4)
	fontSize := float32(11)
	textW := float32(len([]rune(label))) * fontSize * 0.62
	w := textW + 2*padX
	h := fontSize + 2*padY
	r := geometry.NewRect(bounds.Max.X-w-8, bounds.Max.Y-h-8, w, h)
	canvas.DrawRoundRect(r, widget.RGBA8(0, 0, 0, 150), 6)
	style := widget.TextStyle{
		FontFamily: cjkembed.FamilyName,
		FontSize:   fontSize,
		Color:      widget.RGBA8(255, 255, 255, 255),
		Align:      widget.TextAlignCenter,
	}
	if sd, ok := canvas.(widget.StyledTextDrawer); ok {
		sd.DrawStyledText(label, r, style)
	} else {
		canvas.DrawText(label, r, fontSize, style.Color, false, widget.TextAlignCenter)
	}
}

func previewCanPan(bounds image.Rectangle, imgSize image.Point, view *previewView) bool {
	r := view.imageRect(bounds, imgSize)
	return r.Dx() > bounds.Dx() || r.Dy() > bounds.Dy()
}

func handlePreviewWheel(view *previewView, bounds geometry.Rect, imgSize image.Point, we *event.WheelEvent) bool {
	if view == nil || imgSize.X <= 0 || imgSize.Y <= 0 {
		return false
	}
	if !bounds.Contains(we.Position) {
		return false
	}
	factor := previewZoomFactor(we.Delta.Y)
	if factor == 1 {
		return false
	}
	cursor := image.Pt(int(math.Round(float64(we.Position.X))), int(math.Round(float64(we.Position.Y))))
	view.zoomAt(toImageRect(bounds), imgSize, cursor, factor)
	return true
}
