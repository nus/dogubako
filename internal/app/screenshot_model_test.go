package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/userdir"
)

func TestScreenshotModelDefaultsAndSave(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	src := image.NewNRGBA(image.Rect(0, 0, 6, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 6; x++ {
			src.SetNRGBA(x, y, color.NRGBA{B: 200, A: 255})
		}
	}

	var m ScreenshotModel
	if m.Mode() != capture.ModeFull {
		t.Fatalf("mode = %q", m.Mode())
	}
	if m.DelaySec() != 1 || !m.HideWindow() {
		t.Fatalf("delay=%d hide=%v", m.DelaySec(), m.HideWindow())
	}
	m.SetImage(src)
	if got := m.StatusText(i18n.EN); got != "Captured (6×4)" {
		t.Fatalf("status = %q", got)
	}

	if err := m.SaveDefault(); err != nil {
		t.Fatal(err)
	}
	saved := m.LastSaved()
	if saved == "" {
		t.Fatal("empty last saved")
	}
	if !strings.Contains(saved, "Screenshots") {
		t.Fatalf("expected Screenshots in path, got %q", saved)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(saved) != ".png" {
		t.Fatalf("ext = %q", saved)
	}
}

func TestApplyCaptureAutoSavesAndLists(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	first := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	second := image.NewNRGBA(image.Rect(0, 0, 5, 4))
	var m ScreenshotModel
	if err := m.ApplyCapture(first); err != nil {
		t.Fatal(err)
	}
	if len(m.Files()) != 1 {
		t.Fatalf("files after first = %d", len(m.Files()))
	}
	if got := m.StatusText(i18n.EN); !strings.HasPrefix(got, "Saved: ") {
		t.Fatalf("status = %q", got)
	}
	firstPath := m.LastSaved()

	if err := m.ApplyCapture(second); err != nil {
		t.Fatal(err)
	}
	if len(m.Files()) != 2 {
		t.Fatalf("files after second = %d", len(m.Files()))
	}
	if m.Files()[0].Path != m.LastSaved() {
		t.Fatalf("newest should be first: %+v", m.Files())
	}

	if err := m.LoadPath(firstPath); err != nil {
		t.Fatal(err)
	}
	if m.Size() != (image.Point{X: 3, Y: 2}) {
		t.Fatalf("loaded size = %v", m.Size())
	}
	if m.SelectedPath() != firstPath {
		t.Fatalf("selected = %q", m.SelectedPath())
	}
	if m.RevealPath() != firstPath {
		t.Fatalf("reveal = %q", m.RevealPath())
	}
	if m.image != nil {
		t.Fatal("full raster should be dropped after load")
	}
	if got := m.StatusText(i18n.EN); !strings.Contains(got, "Loaded") {
		t.Fatalf("loaded status = %q", got)
	}
}

func TestRevealPathFallsBackToDestDir(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	var m ScreenshotModel
	dest := m.DestDir()
	if dest == "" {
		t.Fatal("expected dest dir")
	}
	if m.RevealPath() != dest {
		t.Fatalf("reveal = %q, want dest %q", m.RevealPath(), dest)
	}
}

func TestThumbnailIsCreatedForSavedFiles(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	src := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var m ScreenshotModel
	if err := m.ApplyCapture(src); err != nil {
		t.Fatal(err)
	}
	thumb := m.Thumbnail(m.LastSaved())
	if thumb == nil {
		t.Fatal("expected thumbnail")
	}
	sz := thumb.Bounds().Size()
	if sz.X > thumbMaxEdge || sz.Y > thumbMaxEdge {
		t.Fatalf("thumb too large: %v", sz)
	}
	if sz.X != thumbMaxEdge {
		t.Fatalf("width = %d, want %d", sz.X, thumbMaxEdge)
	}
	if m.Thumbnail("/missing.png") != nil {
		t.Fatal("missing path should have no thumb")
	}
}

func TestScreenshotModelSaveWithoutImage(t *testing.T) {
	var m ScreenshotModel
	if err := m.SavePath(filepath.Join(t.TempDir(), "x.png")); err == nil {
		t.Fatal("expected error")
	}
	if got := m.StatusText(i18n.JA); got != "保存するキャプチャがありません" {
		t.Fatalf("status = %q", got)
	}
}

func TestImageModelLoadImage(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	var m ImageModel
	if err := m.LoadImage(src, "Screenshot-2026-08-15-134300.png"); err != nil {
		t.Fatal(err)
	}
	if !m.HasSource() || m.SourceSize() != (image.Point{X: 2, Y: 2}) {
		t.Fatalf("size = %v", m.SourceSize())
	}
}

func TestApplyCaptureFileKeepsPreviewOnly(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	src := filepath.Join(t.TempDir(), "cap.png")
	writeSolidPNG(t, src, 40, 20)
	var m ScreenshotModel
	if err := m.ApplyCaptureFile(src); err != nil {
		t.Fatal(err)
	}
	if m.image != nil {
		t.Fatal("full raster should be dropped")
	}
	if !m.HasImage() || m.Size() != (image.Point{X: 40, Y: 20}) {
		t.Fatalf("size = %v has=%v", m.Size(), m.HasImage())
	}
	if m.Preview() == nil {
		t.Fatal("expected preview")
	}
	data, err := m.ExportPNG()
	if err != nil || len(data) == 0 {
		t.Fatalf("export: %v len=%d", err, len(data))
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("temp capture file should have been moved")
	}
}

func TestRefreshFilesLoadsThumbsIncrementally(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	var m ScreenshotModel
	dir := m.DestDir()
	if dir == "" {
		t.Fatal("expected dest dir")
	}
	writeSolidPNG(t, filepath.Join(dir, "a.png"), 40, 20)
	writeSolidPNG(t, filepath.Join(dir, "b.png"), 40, 20)

	m.RefreshFiles()
	if got := countThumbs(&m); got != 1 {
		t.Fatalf("thumbs after first refresh = %d, want 1", got)
	}
	m.RefreshFiles()
	if got := countThumbs(&m); got != 2 {
		t.Fatalf("thumbs after second refresh = %d, want 2", got)
	}
}

func countThumbs(m *ScreenshotModel) int {
	n := 0
	for _, f := range m.Files() {
		if m.Thumbnail(f.Path) != nil {
			n++
		}
	}
	return n
}

func writeSolidPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestScreenshotSetModeAndDelay(t *testing.T) {
	var m ScreenshotModel
	m.SetMode(capture.ModeRegion)
	if m.Mode() != capture.ModeRegion {
		t.Fatalf("mode = %q", m.Mode())
	}
	m.SetDelaySec(99)
	if m.DelaySec() != 10 {
		t.Fatalf("clamped delay = %d", m.DelaySec())
	}
	m.SetHideWindow(false)
	if m.HideWindow() {
		t.Fatal("hide should be off")
	}
}
