package app

import (
	"image"
	"math"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/imageproc"
)

const (
	previewZoomStep = 1.25
	previewZoomMin  = 0.1
	previewZoomMax  = 16.0
)

// previewZoomer is a displayed image that can be magnified and panned.
type previewZoomer interface {
	HasPreview() bool
	ZoomPercent() int
	ZoomFitting() bool
	ZoomIn()
	ZoomOut()
	ZoomFit()
}

// previewCam is the view transform for a preview: fit-to-pane, or a
// scale (screen pixels per image pixel) plus pan offset.
type previewCam struct {
	fit   bool
	scale float64
	panX  float64
	panY  float64
}

func (c *previewCam) resetFit() {
	c.fit = true
	c.scale = 0
	c.panX = 0
	c.panY = 0
}

func (c *previewCam) fitting() bool {
	return c == nil || c.fit || c.scale <= 0
}

func fitScale(bounds image.Rectangle, imgSize image.Point) float64 {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return 1
	}
	return math.Min(float64(bounds.Dx())/float64(imgSize.X), float64(bounds.Dy())/float64(imgSize.Y))
}

func clampPreviewScale(scale float64, bounds image.Rectangle, imgSize image.Point) float64 {
	if scale <= 0 {
		scale = 1
	}
	fit := fitScale(bounds, imgSize)
	minS := math.Min(previewZoomMin, fit)
	if minS < 0.05 {
		minS = 0.05
	}
	maxS := math.Max(previewZoomMax, fit)
	if scale < minS {
		return minS
	}
	if scale > maxS {
		return maxS
	}
	return scale
}

func (c *previewCam) resolve(bounds image.Rectangle, imgSize image.Point) (scale float64, origin image.Point) {
	if c == nil || c.fitting() {
		r := fittedRect(bounds, imgSize)
		if r.Empty() || imgSize.X <= 0 {
			return 0, image.Point{}
		}
		return float64(r.Dx()) / float64(imgSize.X), r.Min
	}
	scale = clampPreviewScale(c.scale, bounds, imgSize)
	w := math.Max(1, math.Round(float64(imgSize.X)*scale))
	h := math.Max(1, math.Round(float64(imgSize.Y)*scale))
	x, y := clampPan(c.panX, c.panY, bounds, w, h)
	return scale, image.Pt(int(math.Round(x)), int(math.Round(y)))
}

func (c *previewCam) viewRect(bounds image.Rectangle, imgSize image.Point) image.Rectangle {
	scale, origin := c.resolve(bounds, imgSize)
	if scale <= 0 || imgSize.X <= 0 || imgSize.Y <= 0 {
		return image.Rectangle{}
	}
	w := max(1, int(math.Round(float64(imgSize.X)*scale)))
	h := max(1, int(math.Round(float64(imgSize.Y)*scale)))
	return image.Rect(origin.X, origin.Y, origin.X+w, origin.Y+h)
}

func (c *previewCam) percent(bounds image.Rectangle, imgSize image.Point) int {
	scale, _ := c.resolve(bounds, imgSize)
	if scale <= 0 {
		return 100
	}
	p := int(math.Round(scale * 100))
	if p < 1 {
		return 1
	}
	return p
}

func (c *previewCam) zoomBy(bounds image.Rectangle, imgSize image.Point, factor float64, pivot image.Point) {
	if factor <= 0 || imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return
	}
	scale, origin := c.resolve(bounds, imgSize)
	if scale <= 0 {
		return
	}
	imgX := (float64(pivot.X) - float64(origin.X)) / scale
	imgY := (float64(pivot.Y) - float64(origin.Y)) / scale
	newScale := clampPreviewScale(scale*factor, bounds, imgSize)
	c.fit = false
	c.scale = newScale
	c.panX = float64(pivot.X) - imgX*newScale
	c.panY = float64(pivot.Y) - imgY*newScale
	c.clamp(bounds, imgSize)
}

func (c *previewCam) zoomIn(bounds image.Rectangle, imgSize image.Point) {
	c.zoomBy(bounds, imgSize, previewZoomStep, rectCenter(bounds))
}

func (c *previewCam) zoomOut(bounds image.Rectangle, imgSize image.Point) {
	c.zoomBy(bounds, imgSize, 1/previewZoomStep, rectCenter(bounds))
}

func (c *previewCam) panBy(bounds image.Rectangle, imgSize image.Point, dx, dy float64) {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() || !c.canPan(bounds, imgSize) {
		return
	}
	if c.fitting() {
		scale, origin := c.resolve(bounds, imgSize)
		c.fit = false
		c.scale = scale
		c.panX = float64(origin.X)
		c.panY = float64(origin.Y)
	}
	c.panX += dx
	c.panY += dy
	c.clamp(bounds, imgSize)
}

func (c *previewCam) canPan(bounds image.Rectangle, imgSize image.Point) bool {
	scale, _ := c.resolve(bounds, imgSize)
	if scale <= 0 {
		return false
	}
	w := float64(imgSize.X) * scale
	h := float64(imgSize.Y) * scale
	return w > float64(bounds.Dx())+0.5 || h > float64(bounds.Dy())+0.5
}

func (c *previewCam) clamp(bounds image.Rectangle, imgSize image.Point) {
	if c.fitting() {
		return
	}
	scale := clampPreviewScale(c.scale, bounds, imgSize)
	c.scale = scale
	w := math.Max(1, math.Round(float64(imgSize.X)*scale))
	h := math.Max(1, math.Round(float64(imgSize.Y)*scale))
	c.panX, c.panY = clampPan(c.panX, c.panY, bounds, w, h)
}

func clampPan(panX, panY float64, bounds image.Rectangle, w, h float64) (float64, float64) {
	if bounds.Empty() {
		return panX, panY
	}
	bw := float64(bounds.Dx())
	bh := float64(bounds.Dy())
	minX := float64(bounds.Min.X)
	minY := float64(bounds.Min.Y)
	if w <= bw {
		panX = minX + (bw-w)/2
	} else {
		panX = math.Min(minX, math.Max(minX+bw-w, panX))
	}
	if h <= bh {
		panY = minY + (bh-h)/2
	} else {
		panY = math.Min(minY, math.Max(minY+bh-h, panY))
	}
	return panX, panY
}

func rectCenter(r image.Rectangle) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
}

func zoomFactorFromWheel(deltaY float32) float64 {
	steps := float64(deltaY)
	if math.Abs(steps) > 4 {
		steps /= 40
	}
	if steps > 4 {
		steps = 4
	} else if steps < -4 {
		steps = -4
	}
	if steps == 0 {
		return 1
	}
	return math.Pow(previewZoomStep, -steps)
}

func applyPreviewWheel(cam *previewCam, bounds image.Rectangle, imgSize image.Point, e *event.WheelEvent) bool {
	if cam == nil || e == nil || imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return false
	}
	if e.Position.X < float32(bounds.Min.X) || e.Position.Y < float32(bounds.Min.Y) ||
		e.Position.X >= float32(bounds.Max.X) || e.Position.Y >= float32(bounds.Max.Y) {
		return false
	}
	factor := zoomFactorFromWheel(e.Delta.Y)
	if factor == 1 {
		return true
	}
	cam.zoomBy(bounds, imgSize, factor, eventPoint(e.Position))
	return true
}

func imageRectFromView(view image.Rectangle, imgSize image.Point, screen image.Rectangle) image.Rectangle {
	if view.Empty() || imgSize.X <= 0 || imgSize.Y <= 0 || screen.Empty() {
		return image.Rectangle{}
	}
	x0 := (screen.Min.X - view.Min.X) * imgSize.X / view.Dx()
	y0 := (screen.Min.Y - view.Min.Y) * imgSize.Y / view.Dy()
	x1 := (screen.Max.X - view.Min.X) * imgSize.X / view.Dx()
	y1 := (screen.Max.Y - view.Min.Y) * imgSize.Y / view.Dy()
	r := image.Rect(x0, y0, x1, y1).Canon().Intersect(image.Rect(0, 0, imgSize.X, imgSize.Y))
	if r.Dx() < 1 {
		r.Max.X = r.Min.X + 1
	}
	if r.Dy() < 1 {
		r.Max.Y = r.Min.Y + 1
	}
	return r.Intersect(image.Rect(0, 0, imgSize.X, imgSize.Y))
}

func applyViewCropHighlight(dst *image.RGBA, origVis image.Rectangle, crop image.Rectangle) {
	if dst == nil || origVis.Empty() {
		return
	}
	part := crop.Intersect(origVis)
	if part.Empty() {
		dimRGBA(dst, dst.Rect)
		return
	}
	applyCropHighlight(dst, origVis.Size(), part.Sub(origVis.Min))
}

func drawViewImage(canvas widget.Canvas, src image.Image, view image.Rectangle, imgSize image.Point, crop image.Rectangle, viewport image.Rectangle) {
	if src == nil || view.Empty() || viewport.Empty() {
		return
	}
	visible := view.Intersect(viewport)
	if visible.Empty() {
		return
	}
	srcB := src.Bounds()
	srcW, srcH := srcB.Dx(), srcB.Dy()
	viewW, viewH := view.Dx(), view.Dy()
	if srcW <= 0 || srcH <= 0 || viewW <= 0 || viewH <= 0 {
		return
	}
	sx0 := srcB.Min.X + (visible.Min.X-view.Min.X)*srcW/viewW
	sy0 := srcB.Min.Y + (visible.Min.Y-view.Min.Y)*srcH/viewH
	sx1 := srcB.Min.X + (visible.Max.X-view.Min.X)*srcW/viewW
	sy1 := srcB.Min.Y + (visible.Max.Y-view.Min.Y)*srcH/viewH
	srcRect := image.Rect(sx0, sy0, sx1, sy1).Canon().Intersect(srcB)
	if srcRect.Empty() {
		return
	}
	visW, visH := visible.Dx(), visible.Dy()
	var raster image.Image
	switch {
	case srcRect.Eq(srcB) && srcW == visW && srcH == visH:
		raster = src
	case srcRect.Dx() == visW && srcRect.Dy() == visH:
		cropped, err := imageproc.Crop(src, srcRect)
		if err != nil {
			return
		}
		raster = cropped
	default:
		cropped, err := imageproc.Crop(src, srcRect)
		if err != nil {
			return
		}
		if cropped.Bounds().Dx() == visW && cropped.Bounds().Dy() == visH {
			raster = cropped
		} else {
			raster = imageproc.Resize(cropped, visW, visH)
		}
	}
	rgba := toDisplayRGBA(raster)
	if imgSize.X > 0 && imgSize.Y > 0 && !crop.Empty() {
		rgba = cloneRGBA(rgba)
		applyViewCropHighlight(rgba, imageRectFromView(view, imgSize, visible), crop)
	}
	canvas.DrawImage(rgba, geometry.Pt(float32(visible.Min.X), float32(visible.Min.Y)))
}

func eventPoint(pos geometry.Point) image.Point {
	return image.Pt(int(math.Round(float64(pos.X))), int(math.Round(float64(pos.Y))))
}

var (
	_ previewZoomer = (*sourcePreview)(nil)
	_ previewZoomer = (*destPreview)(nil)
)
