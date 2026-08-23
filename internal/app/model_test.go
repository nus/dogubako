package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
)

func TestImageModelResizeAndCrop(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var m ImageModel
	m.setSource(src, "test.png", imageproc.FormatPNG)

	m.SetWidth(10)
	if m.Width() != 10 || m.Height() != 5 {
		t.Fatalf("keep-aspect resize: %dx%d", m.Width(), m.Height())
	}

	m.SetCropEnabled(true)
	m.SetCrop(image.Rect(0, 0, 10, 10))
	m.SetScalePercent(50)
	got, err := m.Processed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 5 || got.Bounds().Dy() != 5 {
		t.Fatalf("processed size = %v", got.Bounds())
	}
}

func TestImageModelRotate90SwapsSize(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var m ImageModel
	m.setSource(src, "test.png", imageproc.FormatPNG)

	m.Rotate90()
	if m.RotateDegrees() != 90 {
		t.Fatalf("angle = %d", m.RotateDegrees())
	}
	if m.Width() != 10 || m.Height() != 20 {
		t.Fatalf("size after 90° = %dx%d", m.Width(), m.Height())
	}
	got, err := m.Processed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 10 || got.Bounds().Dy() != 20 {
		t.Fatalf("processed size = %v", got.Bounds())
	}
	nrgba, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("type %T", got)
	}
	if c := nrgba.NRGBAAt(9, 0); c != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("rotated pixel = %+v", c)
	}

	m.SetRotateDegrees(1)
	if m.RotateDegrees() != 1 {
		t.Fatalf("1° = %d", m.RotateDegrees())
	}
	got, err = m.Processed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != m.Width() || got.Bounds().Dy() != m.Height() {
		t.Fatalf("1° processed %v vs params %dx%d", got.Bounds(), m.Width(), m.Height())
	}

	m.ResetRotate()
	if m.RotateDegrees() != 0 {
		t.Fatalf("reset = %d", m.RotateDegrees())
	}
	if m.Width() != 20 || m.Height() != 10 {
		t.Fatalf("size after reset = %dx%d", m.Width(), m.Height())
	}
}

func TestImageModelSaveRoundTrip(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{G: 200, A: 255})
		}
	}
	var m ImageModel
	m.setSource(src, "in.png", imageproc.FormatPNG)
	m.SetFormat(imageproc.FormatPNG)

	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	if err := m.SavePath(path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatal(err)
	}
}

func TestResultPreviewCapsLargeOutput(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	var m ImageModel
	m.setSource(src, "t.png", imageproc.FormatPNG)
	m.SetWidth(4000)
	prev := m.ResultPreview()
	if prev == nil {
		t.Fatal("expected preview")
	}
	if max(prev.Bounds().Dx(), prev.Bounds().Dy()) > previewMaxEdge {
		t.Fatalf("preview = %v", prev.Bounds())
	}
	got, err := m.Processed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 4000 {
		t.Fatalf("full processed = %v", got.Bounds())
	}
}

func TestSuggestedFilename(t *testing.T) {
	var m ImageModel
	m.sourceName = "photo.jpeg"
	m.params.Format = imageproc.FormatPNG
	if got := m.SuggestedFilename(); got != "photo-edited.png" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusTextTranslates(t *testing.T) {
	var m ImageModel
	m.SetStatus(i18n.StatusClipboardCopied)
	if got := m.StatusText(i18n.JA); got != "クリップボードにコピーしました" {
		t.Fatalf("ja = %q", got)
	}
	if got := m.StatusText(i18n.EN); got != "Copied to the clipboard" {
		t.Fatalf("en = %q", got)
	}

	m.SetStatus(i18n.StatusLoaded, "a.png", 8, 4)
	if got := m.StatusText(i18n.EN); got != "Loaded a.png (8×4)" {
		t.Fatalf("en loaded = %q", got)
	}
}

func TestToolTitle(t *testing.T) {
	tool := Tool{ID: ToolImage}
	if got := tool.Title(i18n.JA); got != "画像" {
		t.Fatalf("ja = %q", got)
	}
	if got := tool.Title(i18n.EN); got != "Image" {
		t.Fatalf("en = %q", got)
	}
	shot := Tool{ID: ToolScreenshot}
	if got := shot.Title(i18n.JA); got != "画面キャプチャ" {
		t.Fatalf("ja screenshot = %q", got)
	}
	if got := shot.Title(i18n.EN); got != "Screenshot" {
		t.Fatalf("en screenshot = %q", got)
	}
	android := Tool{ID: ToolAndroid}
	if got := android.Title(i18n.JA); got != "Android ファイル" {
		t.Fatalf("ja android = %q", got)
	}
	if got := android.Title(i18n.EN); got != "Android Files" {
		t.Fatalf("en android = %q", got)
	}
	androidShot := Tool{ID: ToolAndroidShot}
	if got := androidShot.Title(i18n.JA); got != "Android 画面" {
		t.Fatalf("ja android shot = %q", got)
	}
	if got := androidShot.Title(i18n.EN); got != "Android Screen" {
		t.Fatalf("en android shot = %q", got)
	}
}

func TestModelSetLang(t *testing.T) {
	dir := t.TempDir()
	restore := i18n.OverrideUserConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	m.SetLang(i18n.EN)
	if m.Lang() != i18n.EN {
		t.Fatalf("lang = %q", m.Lang())
	}
	m.SetLang(i18n.JA)
	if m.Lang() != i18n.JA {
		t.Fatalf("lang = %q", m.Lang())
	}
}

func TestApplyLangDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	restore := i18n.OverrideUserConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	if err := i18n.Save(i18n.JA); err != nil {
		t.Fatal(err)
	}
	var m Model
	m.applyLang(i18n.EN, false)
	if m.Lang() != i18n.EN {
		t.Fatalf("lang = %q", m.Lang())
	}
	if got := i18n.Load(); got != i18n.JA {
		t.Fatalf("saved = %q, want ja", got)
	}
}
