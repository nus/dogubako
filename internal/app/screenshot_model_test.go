package app

import (
	"image"
	"image/color"
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
