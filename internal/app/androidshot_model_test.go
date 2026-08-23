package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/userdir"
)

func waitAndroidShot(t *testing.T, m *AndroidShotModel) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.Drain()
		if !m.Busy() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for android shot model")
}

func TestAndroidShotCapturesAndSaves(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	png := solidPNG(t, 6, 4)
	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel 7"})
	fs.Shot = map[string][]byte{"pixel": png}

	var m AndroidShotModel
	m.SetClient(fs)
	m.EnsureLoaded()
	waitAndroidShot(t, &m)

	if m.Serial() != "pixel" {
		t.Fatalf("serial = %q", m.Serial())
	}
	if m.DelaySec() != 0 {
		t.Fatalf("delay = %d", m.DelaySec())
	}
	dest := m.DestDir()
	if !strings.HasSuffix(dest, filepath.Join("Screenshots", "Android")) && !strings.Contains(dest, string(filepath.Separator)+"Android") {
		t.Fatalf("dest = %q", dest)
	}

	m.StartCapture()
	waitAndroidShot(t, &m)
	if !m.HasImage() || m.Size() != (image.Point{X: 6, Y: 4}) {
		t.Fatalf("size = %v has=%v status=%q", m.Size(), m.HasImage(), m.StatusText(i18n.EN))
	}
	saved := m.LastSaved()
	if saved == "" {
		t.Fatal("empty last saved")
	}
	if filepath.Ext(saved) != ".png" {
		t.Fatalf("ext = %q", filepath.Ext(saved))
	}
	if !strings.HasPrefix(filepath.Base(saved), "Android-") {
		t.Fatalf("name = %q", filepath.Base(saved))
	}
	if !strings.Contains(saved, string(filepath.Separator)+"Android"+string(filepath.Separator)) {
		t.Fatalf("expected Android folder in %q", saved)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatal(err)
	}
	if len(m.Files()) != 1 {
		t.Fatalf("files = %d", len(m.Files()))
	}
	if got := m.StatusText(i18n.EN); !strings.HasPrefix(got, "Saved: ") {
		t.Fatalf("status = %q", got)
	}
}

func TestAndroidShotOfflineAndErrors(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	fs := adbfs.NewMem(adbfs.Device{Serial: "x", State: "unauthorized"})
	var m AndroidShotModel
	m.SetClient(fs)
	m.EnsureLoaded()
	waitAndroidShot(t, &m)
	if m.Online() {
		t.Fatal("unauthorized should be offline")
	}
	m.StartCapture()
	if got := m.StatusText(i18n.EN); got != i18n.T(i18n.EN, i18n.StatusAdbSelectOnline) {
		t.Fatalf("status = %q", got)
	}

	fs2 := adbfs.NewMem()
	fs2.DevErr = context.DeadlineExceeded
	var m2 AndroidShotModel
	m2.SetClient(fs2)
	m2.RefreshDevices()
	waitAndroidShot(t, &m2)
	if got := m2.StatusText(i18n.JA); got == "" {
		t.Fatal("expected connect error")
	}
}

func TestAndroidShotCaptureFailure(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel"})
	fs.ShotErr = context.DeadlineExceeded
	var m AndroidShotModel
	m.SetClient(fs)
	m.EnsureLoaded()
	waitAndroidShot(t, &m)
	m.StartCapture()
	waitAndroidShot(t, &m)
	if m.HasImage() {
		t.Fatal("should not have image")
	}
	if got := m.StatusText(i18n.EN); got != i18n.T(i18n.EN, i18n.StatusCaptureTimeout) {
		t.Fatalf("status = %q", got)
	}
}

func TestAndroidShotSelectsFirstOnlineDevice(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)

	fs := adbfs.NewMem(
		adbfs.Device{Serial: "off", State: "unauthorized"},
		adbfs.Device{Serial: "on", State: "device", Model: "Pixel"},
	)
	var m AndroidShotModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroidShot(t, &m)
	if m.Serial() != "on" {
		t.Fatalf("serial = %q", m.Serial())
	}
	if got := m.StatusText(i18n.EN); !strings.Contains(got, "Pixel") {
		t.Fatalf("status = %q", got)
	}
}

func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
