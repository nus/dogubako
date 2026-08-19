package app

import (
	"image"
	"image/color"
	"testing"

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
