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
	p.SetProvider(func() sourcePreviewFrame {
		return sourcePreviewFrame{
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
	applyCropHighlight(dst, image.Pt(100, 100), image.Rect(20, 20, 80, 80))

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
