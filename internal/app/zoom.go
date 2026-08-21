package app

import (
	"image"
	"math"
)

const (
	// zoomStepFactor is one press of the zoom in / zoom out button.
	zoomStepFactor float32 = 1.25
	// minZoomScale and maxZoomScale bound the view pixels per image pixel.
	minZoomScale float32 = 0.05
	maxZoomScale float32 = 16
	// pixelZoomScale is where upscaling switches to nearest neighbour so
	// single pixels stay crisp instead of being blurred by Catmull-Rom.
	pixelZoomScale float64 = 4
	// fitSnapRatio treats a scale within 1% of the fit scale as "fit", so a
	// window resize keeps showing the whole image.
	fitSnapRatio float32 = 0.01
)

// zoomView is the zoom and pan state of a preview area. scale counts view
// pixels per image pixel; the zero value means "fit the whole image".
type zoomView struct {
	scale float32
	pan   image.Point
}

func (v *zoomView) fitted() bool { return v.scale <= 0 }

func (v *zoomView) reset() {
	v.scale = 0
	v.pan = image.Point{}
}

// scaleIn returns the scale currently used to draw the image in bounds.
func (v *zoomView) scaleIn(bounds image.Rectangle, imgSize image.Point) float32 {
	if v.scale > 0 {
		return v.scale
	}
	return fitScale(bounds, imgSize)
}

// displayRect is where the whole image lands inside bounds. It may be larger
// than bounds when the user has zoomed in.
func (v *zoomView) displayRect(bounds image.Rectangle, imgSize image.Point) image.Rectangle {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return image.Rectangle{}
	}
	if v.scale <= 0 {
		return fittedRect(bounds, imgSize)
	}
	size := scaledSize(imgSize, v.scale)
	pan := clampPan(v.pan, size, bounds.Size())
	x := bounds.Min.X + (bounds.Dx()-size.X)/2 + pan.X
	y := bounds.Min.Y + (bounds.Dy()-size.Y)/2 + pan.Y
	return image.Rect(x, y, x+size.X, y+size.Y)
}

// zoomTo rescales around anchor, a point in bounds that keeps showing the same
// pixel of the image. It reports whether the view changed.
func (v *zoomView) zoomTo(bounds image.Rectangle, imgSize image.Point, scale float32, anchor image.Point) bool {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return false
	}
	fit := fitScale(bounds, imgSize)
	scale = clampScale(scale, fit)
	if nearFitScale(scale, fit) {
		if v.fitted() {
			return false
		}
		v.reset()
		return true
	}
	before := v.displayRect(bounds, imgSize)
	prev := v.scaleIn(bounds, imgSize)
	if before.Empty() || prev <= 0 {
		return false
	}
	imgX := float64(anchor.X-before.Min.X) / float64(prev)
	imgY := float64(anchor.Y-before.Min.Y) / float64(prev)
	size := scaledSize(imgSize, scale)
	originX := float64(bounds.Min.X) + float64(bounds.Dx()-size.X)/2
	originY := float64(bounds.Min.Y) + float64(bounds.Dy()-size.Y)/2
	pan := image.Pt(
		int(math.Round(float64(anchor.X)-imgX*float64(scale)-originX)),
		int(math.Round(float64(anchor.Y)-imgY*float64(scale)-originY)),
	)
	pan = clampPan(pan, size, bounds.Size())
	if v.scale == scale && v.pan == pan {
		return false
	}
	v.scale = scale
	v.pan = pan
	return true
}

// zoomBy multiplies the current scale by factor, stopping at 100% on the way
// so actual-size is reachable with the buttons and the wheel.
func (v *zoomView) zoomBy(bounds image.Rectangle, imgSize image.Point, factor float32, anchor image.Point) bool {
	cur := v.scaleIn(bounds, imgSize)
	if cur <= 0 || factor <= 0 {
		return false
	}
	return v.zoomTo(bounds, imgSize, stepScale(cur, factor), anchor)
}

// panBy moves the image inside the view. A fitted view has nothing to pan.
func (v *zoomView) panBy(bounds image.Rectangle, imgSize image.Point, delta image.Point) bool {
	if v.scale <= 0 || delta == (image.Point{}) {
		return false
	}
	size := scaledSize(imgSize, v.scale)
	pan := clampPan(v.pan.Add(delta), size, bounds.Size())
	if pan == v.pan {
		return false
	}
	v.pan = pan
	return true
}

// canPan reports whether the image is larger than the view in either axis.
func (v *zoomView) canPan(bounds image.Rectangle, imgSize image.Point) bool {
	disp := v.displayRect(bounds, imgSize)
	if disp.Empty() {
		return false
	}
	return disp.Dx() > bounds.Dx() || disp.Dy() > bounds.Dy()
}

// percent is the zoom level shown next to the preview, 100 meaning one view
// pixel per image pixel.
func (v *zoomView) percent(bounds image.Rectangle, imgSize image.Point) int {
	scale := v.scaleIn(bounds, imgSize)
	if scale <= 0 {
		return 0
	}
	return max(1, int(math.Round(float64(scale)*100)))
}

func fitScale(bounds image.Rectangle, imgSize image.Point) float32 {
	if imgSize.X <= 0 || imgSize.Y <= 0 || bounds.Empty() {
		return 0
	}
	return float32(math.Min(
		float64(bounds.Dx())/float64(imgSize.X),
		float64(bounds.Dy())/float64(imgSize.Y),
	))
}

func scaledSize(imgSize image.Point, scale float32) image.Point {
	return image.Pt(
		max(1, int(math.Round(float64(imgSize.X)*float64(scale)))),
		max(1, int(math.Round(float64(imgSize.Y)*float64(scale)))),
	)
}

// clampScale keeps the scale usable. Images too large to fit at minZoomScale
// may still zoom out to their fit scale.
func clampScale(scale, fit float32) float32 {
	low := minZoomScale
	if fit > 0 && fit < low {
		low = fit
	}
	if scale < low {
		return low
	}
	if scale > maxZoomScale {
		return maxZoomScale
	}
	return scale
}

func nearFitScale(scale, fit float32) bool {
	if fit <= 0 {
		return false
	}
	return math.Abs(float64(scale-fit)) <= float64(fit*fitSnapRatio)
}

func stepScale(cur, factor float32) float32 {
	next := cur * factor
	if (cur < 1 && next > 1) || (cur > 1 && next < 1) {
		return 1
	}
	return next
}

func clampPan(pan, disp, view image.Point) image.Point {
	return image.Pt(
		clampAbs(pan.X, panLimit(disp.X, view.X)),
		clampAbs(pan.Y, panLimit(disp.Y, view.Y)),
	)
}

// panLimit is how far the image may slide before an edge would move inside
// the view.
func panLimit(disp, view int) int {
	if disp <= view {
		return 0
	}
	return (disp - view) / 2
}

func clampAbs(v, limit int) int {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

// wheelZoomFactor converts a wheel or trackpad delta into a scale factor.
// Delta.Y is positive when scrolling down, so the sign is flipped to zoom in
// when the wheel turns away from the user. Trackpads send long streams of
// deltas, so one event is capped.
func wheelZoomFactor(deltaY float32) float32 {
	d := float64(deltaY)
	if d > 3 {
		d = 3
	}
	if d < -3 {
		d = -3
	}
	return float32(math.Pow(1.2, -d))
}
