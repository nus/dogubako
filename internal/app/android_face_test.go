package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/userdir"
)

func TestAndroidShotModeOpensScreen(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	if err := os.MkdirAll(filepath.Join(dir, "dogubako"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dogubako", "mode"), []byte("android-shot\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var m Model
	if m.Mode() != ToolAndroid {
		t.Fatalf("mode = %q", m.Mode())
	}
	if m.AndroidFace() != AndroidFaceScreen {
		t.Fatalf("face = %q", m.AndroidFace())
	}
	data, err := os.ReadFile(filepath.Join(dir, "dogubako", "mode"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != string(ToolAndroid) {
		t.Fatalf("saved mode = %q", data)
	}
	face, err := os.ReadFile(filepath.Join(dir, "dogubako", "android-face"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(face)) != string(AndroidFaceScreen) {
		t.Fatalf("saved face = %q", face)
	}
}

func TestAndroidFacePersists(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	if m.AndroidFace() != AndroidFaceFiles {
		t.Fatalf("default face = %q", m.AndroidFace())
	}
	m.SetMode(ToolAndroid)
	m.SetAndroidFace(AndroidFaceScreen)
	if m.AndroidFace() != AndroidFaceScreen {
		t.Fatalf("face = %q", m.AndroidFace())
	}
	data, err := os.ReadFile(filepath.Join(dir, "dogubako", "mode"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != string(ToolAndroid) {
		t.Fatalf("mode = %q", data)
	}

	var again Model
	if again.Mode() != ToolAndroid {
		t.Fatalf("reloaded mode = %q", again.Mode())
	}
	if again.AndroidFace() != AndroidFaceScreen {
		t.Fatalf("reloaded face = %q", again.AndroidFace())
	}
}

func TestSharedAndroidDeviceSelection(t *testing.T) {
	home := t.TempDir()
	restore := userdir.Override("linux", home, nil)
	t.Cleanup(restore)
	restoreMode := overrideSidebarConfigDir(func() (string, error) { return t.TempDir(), nil })
	t.Cleanup(restoreMode)

	fs := adbfs.NewMem(
		adbfs.Device{Serial: "a", State: "device", Model: "A"},
		adbfs.Device{Serial: "b", State: "device", Model: "B"},
	)
	var m Model
	m.Android().SetClient(fs)
	m.AndroidShot().SetClient(fs)
	m.Android().EnsureLoaded()
	waitAndroid(t, m.Android())
	m.AndroidShot().Drain()
	if m.Android().Serial() != "a" || m.AndroidShot().Serial() != "a" {
		t.Fatalf("serial files=%q shot=%q", m.Android().Serial(), m.AndroidShot().Serial())
	}
	m.SelectAndroidDevice("b")
	waitAndroid(t, m.Android())
	if m.Android().Serial() != "b" || m.AndroidShot().Serial() != "b" {
		t.Fatalf("selected files=%q shot=%q", m.Android().Serial(), m.AndroidShot().Serial())
	}
	if got := len(m.Android().Devices()); got != 2 || len(m.AndroidShot().Devices()) != 2 {
		t.Fatalf("devices files=%d shot=%d", len(m.Android().Devices()), len(m.AndroidShot().Devices()))
	}
}
