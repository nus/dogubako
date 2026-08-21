package app

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/uitest"

	"github.com/nus/dogubako/internal/imageproc"
)

func TestNormalizeCrop(t *testing.T) {
	got := normalizeCrop(image.Rect(8, 2, 2, 6), image.Pt(10, 8))
	if got != image.Rect(2, 2, 8, 6) {
		t.Fatalf("got %v", got)
	}
	got = normalizeCrop(image.Rect(-4, -4, 1, 1), image.Pt(10, 8))
	if got.Min.X < 0 || got.Min.Y < 0 || got.Empty() {
		t.Fatalf("clamped crop = %v", got)
	}
}

func TestFittedRectKeepsAspect(t *testing.T) {
	got := fittedRect(image.Rect(0, 0, 200, 100), image.Pt(100, 100))
	if got.Dx() != 100 || got.Dy() != 100 {
		t.Fatalf("got %v", got)
	}
	if got.Min.X != 50 {
		t.Fatalf("centered x = %d", got.Min.X)
	}
}

func TestDestPreviewHasImageFromSource(t *testing.T) {
	var p destPreview
	if p.HasImage() {
		t.Fatal("empty")
	}
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	p.SetSource(src)
	if !p.HasImage() {
		t.Fatal("source should count as an image before GPU upload")
	}
	p.SetSource(nil)
	if p.HasImage() {
		t.Fatal("cleared")
	}
}

func TestDestPreviewDrawsProviderWithoutSetImage(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 6, 4))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	p := newDestPreview(state.NewSignal[uint64](1))
	p.SetProvider(func() image.Image { return src })
	p.SetBounds(geometry.NewRect(0, 0, 120, 80))
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d, want 1 (provider must be read during Draw)", len(canvas.Images))
	}
	if canvas.Images[0].Image == nil {
		t.Fatal("drawn image is nil")
	}
}

func TestSourcePreviewDrawsProviderWithoutSetImage(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 8))
	src.SetNRGBA(1, 1, color.NRGBA{G: 255, A: 255})
	var m ImageModel
	m.setSource(src, "test.png", imageproc.FormatPNG)
	p := newSourcePreview(state.NewSignal[uint64](1))
	p.SetProvider(func() previewFrame {
		return previewFrame{
			image:       m.SourcePreview(),
			size:        m.SourceSize(),
			crop:        m.Crop(),
			cropEnabled: m.CropEnabled(),
		}
	})
	p.SetBounds(geometry.NewRect(0, 0, 200, 160))
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d, want 1", len(canvas.Images))
	}
}

func TestDestPreviewDrawsListThumbnailFromSetSource(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	src.SetNRGBA(0, 0, color.NRGBA{B: 255, A: 255})
	p := newDestPreview(nil)
	p.SetSource(src)
	p.SetBounds(geometry.NewRect(0, 0, 48, 48))
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)
	if len(canvas.Images) != 1 {
		t.Fatalf("thumbnail DrawImage calls = %d, want 1", len(canvas.Images))
	}
}

func TestToDisplayRGBAOriginAligned(t *testing.T) {
	src := image.NewNRGBA(image.Rect(2, 3, 6, 7))
	src.SetNRGBA(2, 3, color.NRGBA{R: 200, G: 10, B: 20, A: 255})
	got := toDisplayRGBA(src)
	if got.Rect.Min != (image.Point{}) {
		t.Fatalf("min = %v, want origin", got.Rect.Min)
	}
	if got.Stride != got.Rect.Dx()*4 {
		t.Fatalf("stride = %d", got.Stride)
	}
	c := got.RGBAAt(0, 0)
	if c.R != 200 || c.A != 255 {
		t.Fatalf("pixel = %+v", c)
	}
}

func TestComputedMustBeReadToRun(t *testing.T) {
	ran := false
	c := state.NewComputed(func() int {
		ran = true
		return 1
	})
	_ = c
	if ran {
		t.Fatal("discarded Computed ran; preview sync must not rely on unread Computeds")
	}
	if c.Get() != 1 {
		t.Fatal("Get should evaluate")
	}
	if !ran {
		t.Fatal("Get did not evaluate Computed")
	}
}

func TestScreenToImageRoundTrip(t *testing.T) {
	fitted := image.Rect(10, 20, 110, 70)
	imgSize := image.Pt(200, 100)
	pt := screenToImage(image.Pt(10, 20), fitted, imgSize)
	if pt != (image.Point{}) {
		t.Fatalf("top-left = %v", pt)
	}
	pt = screenToImage(image.Pt(109, 69), fitted, imgSize)
	if pt.X < 0 || pt.X >= imgSize.X || pt.Y < 0 || pt.Y >= imgSize.Y {
		t.Fatalf("bottom-right out of range: %v", pt)
	}
}

func TestApplyCropHighlightDarkensOutsideAndPaintsBorder(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	applyCropHighlight(dst, image.Rect(20, 20, 80, 80))

	outside := dst.RGBAAt(0, 0)
	if outside.R > 130 || outside.A != 255 {
		t.Fatalf("outside pixel should be dimmed, got %+v", outside)
	}
	inside := dst.RGBAAt(50, 50)
	if inside.R != 255 || inside.G != 255 || inside.B != 255 {
		t.Fatalf("inside pixel should stay white, got %+v", inside)
	}
	border := dst.RGBAAt(20, 50)
	if border != cropBorder {
		t.Fatalf("crop border = %+v, want %+v", border, cropBorder)
	}
}

func TestSourcePreviewDrawBakesCropOverlay(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(100, 100))
	p.SetCrop(image.Rect(20, 20, 80, 80), true)
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)
	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d, want 1", len(canvas.Images))
	}
	got, ok := canvas.Images[0].Image.(*image.RGBA)
	if !ok {
		t.Fatalf("drawn type %T", canvas.Images[0].Image)
	}
	outside := got.RGBAAt(0, 0)
	if outside.R > 130 {
		t.Fatalf("crop overlay missing: outside pixel %+v", outside)
	}
	inside := got.RGBAAt(50, 50)
	if inside.R != 255 {
		t.Fatalf("selected region should stay bright, got %+v", inside)
	}
}

func TestSourcePreviewDrawBakesOverlayWhileDragging(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(40, 40))
	p.SetCrop(src.Bounds(), false)
	p.dragging = true
	p.dragRect = image.Rect(5, 5, 30, 30)
	p.SetBounds(geometry.NewRect(0, 0, 40, 40))
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)
	got := canvas.Images[0].Image.(*image.RGBA)
	if got.RGBAAt(0, 0).R > 130 {
		t.Fatal("drag overlay should dim outside the drag rect")
	}
	if got.RGBAAt(18, 18).R != 255 {
		t.Fatal("inside drag rect should stay bright")
	}
}

func TestSourcePreviewWheelZoomsIn(t *testing.T) {
	p := newSourcePreview(nil)
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 100, 100)), image.Pt(100, 100))
	p.SetBounds(geometry.NewRect(0, 0, 200, 200))
	ctx := uitest.NewMockContext()

	before := p.ZoomPercent()
	if before != 200 {
		t.Fatalf("fitted percent = %d, want 200", before)
	}
	if !p.Event(ctx, uitest.WheelScroll(100, 100, 3)) {
		t.Fatal("wheel over the preview should be consumed")
	}
	if got := p.ZoomPercent(); got <= before {
		t.Fatalf("percent after zoom in = %d, want more than %d", got, before)
	}
	p.ZoomFit()
	if got := p.ZoomPercent(); got != before {
		t.Fatalf("percent after fit = %d, want %d", got, before)
	}
}

func TestPreviewZoomButtonsReportChanges(t *testing.T) {
	p := newDestPreview(nil)
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 40, 40)))
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	calls := 0
	p.SetOnZoom(func() { calls++ })

	p.ZoomIn()
	if calls != 1 {
		t.Fatalf("zoom in notifications = %d, want 1", calls)
	}
	if p.ZoomPercent() <= 200 {
		t.Fatalf("percent = %d, want more than the 200 fit", p.ZoomPercent())
	}
	p.ZoomOut()
	p.ZoomFit()
	if !p.view.fitted() {
		t.Fatal("fit button should restore the fitted view")
	}
}

func TestDestPreviewDrawStaysInsideBoundsWhenZoomed(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 60, 40))
	p := newDestPreview(nil)
	p.SetImage(src)
	bounds := geometry.NewRect(0, 0, 120, 80)
	p.SetBounds(bounds)
	for i := 0; i < 6; i++ {
		p.ZoomIn()
	}
	canvas := &uitest.MockCanvas{}
	p.Draw(nil, canvas)

	if len(canvas.Images) != 1 {
		t.Fatalf("DrawImage calls = %d, want 1", len(canvas.Images))
	}
	drawn := canvas.Images[0]
	size := drawn.Image.Bounds().Size()
	view := toImageRect(bounds)
	got := image.Rect(int(drawn.At.X), int(drawn.At.Y), int(drawn.At.X)+size.X, int(drawn.At.Y)+size.Y)
	if !got.In(view) {
		t.Fatalf("zoomed raster %v spills outside the preview %v", got, view)
	}
	if got.Dx() < view.Dx() || got.Dy() < view.Dy() {
		t.Fatalf("zoomed raster %v should cover the preview %v", got, view)
	}
}

func TestDestPreviewDragPansZoomedImage(t *testing.T) {
	p := newDestPreview(nil)
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 100, 100)))
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	ctx := uitest.NewMockContext()
	for i := 0; i < 4; i++ {
		p.ZoomIn()
	}
	view := toImageRect(p.Bounds())
	start := p.view.displayRect(view, p.imageSize)

	if !p.Event(ctx, uitest.Click(50, 50)) {
		t.Fatal("press on a zoomed preview should start a pan")
	}
	if !p.Event(ctx, uitest.MouseDrag(70, 50)) {
		t.Fatal("drag should be consumed while panning")
	}
	moved := p.view.displayRect(view, p.imageSize)
	if moved.Min.X <= start.Min.X {
		t.Fatalf("drag right should move the image right: %v -> %v", start, moved)
	}
	p.Event(ctx, uitest.Release(70, 50))
	if p.panning {
		t.Fatal("release should end the pan")
	}
}

func TestDestPreviewIgnoresDragWhenFitted(t *testing.T) {
	p := newDestPreview(nil)
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 10, 10)))
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	if p.Event(uitest.NewMockContext(), uitest.Click(50, 50)) {
		t.Fatal("a fitted preview has nothing to pan")
	}
}

func TestSourcePreviewCropFollowsZoomedView(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	p := newSourcePreview(nil)
	p.SetImage(src, image.Pt(100, 100))
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	var got image.Rectangle
	p.OnCropChanged(func(r image.Rectangle) { got = r })
	for i := 0; i < 4; i++ {
		p.ZoomIn()
	}
	ctx := uitest.NewMockContext()
	view := toImageRect(p.Bounds())
	disp := p.view.displayRect(view, p.imageSize)
	scale := float32(disp.Dx()) / 100

	p.Event(ctx, uitest.Click(10, 10))
	p.Event(ctx, uitest.MouseDrag(60, 60))
	p.Event(ctx, uitest.Release(60, 60))

	want := screenToImage(image.Pt(10, 10), disp, p.imageSize)
	if got.Min != want {
		t.Fatalf("crop origin = %v, want %v (scale %v)", got.Min, want, scale)
	}
	if got.Dx() >= 50 {
		t.Fatalf("crop width = %d, a zoomed drag must cover fewer image pixels", got.Dx())
	}
}

func TestPreviewResetsZoomWhenImageChanges(t *testing.T) {
	p := newDestPreview(nil)
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 40, 40)))
	p.SetBounds(geometry.NewRect(0, 0, 80, 80))
	p.ZoomIn()
	if p.view.fitted() {
		t.Fatal("should be zoomed")
	}
	p.SetImage(image.NewNRGBA(image.Rect(0, 0, 60, 60)))
	if !p.view.fitted() {
		t.Fatal("a new image should start fitted")
	}
}

func TestDestPreviewSamplesFullImageWhenZoomedPastPreview(t *testing.T) {
	full := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	small := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	p := newDestPreview(nil)
	p.SetFrameProvider(func() previewFrame {
		return previewFrame{image: small, full: full, size: image.Pt(400, 400)}
	})
	p.SetBounds(geometry.NewRect(0, 0, 100, 100))
	if got := sharpestRaster(small, full, image.Rect(0, 0, 100, 100)); got != image.Image(small) {
		t.Fatal("a fitted view should use the cheap preview raster")
	}
	if got := sharpestRaster(small, full, image.Rect(0, 0, 400, 400)); got != image.Image(full) {
		t.Fatal("zooming past the preview raster should sample the full image")
	}
}

func TestPreviewTileCoversOnlyTheVisiblePart(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	view := image.Rect(0, 0, 100, 100)
	disp := image.Rect(-100, -100, 300, 300) // 8x zoom, image larger than the view

	tile, origin := previewTile(src, disp, view)
	if tile == nil {
		t.Fatal("no tile")
	}
	got := image.Rect(origin.X, origin.Y, origin.X+tile.Bounds().Dx(), origin.Y+tile.Bounds().Dy())
	if got != view {
		t.Fatalf("tile = %v, want exactly the visible area %v", got, view)
	}
}

func TestSourcePreviewLocalMouseUsesScreenOriginWhileDragging(t *testing.T) {
	p := newSourcePreview(nil)
	p.SetBounds(geometry.NewRect(0, 0, 200, 150))
	p.SetScreenOrigin(geometry.Pt(192, 80))

	press := event.NewMouseEvent(
		event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(40, 30), geometry.Pt(40, 30), event.ModNone,
	)
	if got := p.localMouse(press); got != (image.Point{X: 40, Y: 30}) {
		t.Fatalf("press (tree-local) = %v, want (40,30)", got)
	}

	p.dragging = true
	move := event.NewMouseEvent(
		event.MouseMove, event.ButtonNone, event.ButtonStateLeft,
		geometry.Pt(192+40, 80+30), geometry.Pt(192+40, 80+30), event.ModNone,
	)
	if got := p.localMouse(move); got != (image.Point{X: 40, Y: 30}) {
		t.Fatalf("captured window coords = %v, want (40,30)", got)
	}
}
