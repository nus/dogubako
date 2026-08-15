package userdir

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseUserDirsJapanesePictures(t *testing.T) {
	home := "/home/yota"
	got := parseUserDirs(`
# This file is written by xdg-user-dirs-update
XDG_DESKTOP_DIR="$HOME/デスクトップ"
XDG_PICTURES_DIR="$HOME/画像"
XDG_SCREENSHOTS_DIR="$HOME/画像/Screenshots"
`, home)
	if got["XDG_PICTURES_DIR"] != "/home/yota/画像" {
		t.Fatalf("pictures = %q", got["XDG_PICTURES_DIR"])
	}
	if got["XDG_SCREENSHOTS_DIR"] != "/home/yota/画像/Screenshots" {
		t.Fatalf("screenshots = %q", got["XDG_SCREENSHOTS_DIR"])
	}
}

func TestExpandXDG(t *testing.T) {
	home := "/Users/me"
	cases := map[string]string{
		`"$HOME/Pictures"`: "/Users/me/Pictures",
		"${HOME}/Pictures": "/Users/me/Pictures",
		"/abs/pics":        "/abs/pics",
		`  "Pictures"  `:   "/Users/me/Pictures",
		"":                 "",
	}
	for in, want := range cases {
		if got := expandXDG(in, home); got != want {
			t.Errorf("expandXDG(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinuxUsesXDGThenFallback(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}
	restore := Override("linux", home, map[string]string{
		"XDG_CONFIG_HOME": config,
	})
	t.Cleanup(restore)

	if err := os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte(`XDG_PICTURES_DIR="$HOME/画像"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Pictures()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "画像") {
		t.Fatalf("pictures = %q", got)
	}
	shot, err := Screenshots()
	if err != nil {
		t.Fatal(err)
	}
	if shot != filepath.Join(home, "画像", "Screenshots") {
		t.Fatalf("screenshots = %q", shot)
	}
}

func TestLinuxPrefersScreenshotsEnv(t *testing.T) {
	home := t.TempDir()
	restore := Override("linux", home, map[string]string{
		"XDG_SCREENSHOTS_DIR": filepath.Join(home, "Captures"),
		"XDG_PICTURES_DIR":    filepath.Join(home, "Pics"),
	})
	t.Cleanup(restore)

	got, err := Screenshots()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "Captures") {
		t.Fatalf("got %q", got)
	}
}

func TestLinuxFallbackWithoutPicturesDir(t *testing.T) {
	home := t.TempDir()
	restore := Override("linux", home, nil)
	t.Cleanup(restore)

	got, err := Pictures()
	if err != nil {
		t.Fatal(err)
	}
	if got != home {
		t.Fatalf("pictures fallback = %q", got)
	}
	shot, err := Screenshots()
	if err != nil {
		t.Fatal(err)
	}
	if shot != filepath.Join(home, "Screenshots") {
		t.Fatalf("screenshots fallback = %q", shot)
	}
}

func TestDarwinPictures(t *testing.T) {
	home := t.TempDir()
	restore := Override("darwin", home, map[string]string{
		"XDG_PICTURES_DIR": home + "/ignored",
	})
	t.Cleanup(restore)

	got, err := Pictures()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "Pictures") {
		t.Fatalf("got %q", got)
	}
	shot, err := Screenshots()
	if err != nil {
		t.Fatal(err)
	}
	if shot != filepath.Join(home, "Pictures", "Screenshots") {
		t.Fatalf("screenshots = %q", shot)
	}
}

func TestEnsureScreenshotsCreates(t *testing.T) {
	home := t.TempDir()
	restore := Override("darwin", home, nil)
	t.Cleanup(restore)

	dir, err := EnsureScreenshots()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("dir = %v err=%v", dir, err)
	}
}

func TestFilenameAndUniquePath(t *testing.T) {
	ts := time.Date(2026, 8, 15, 13, 43, 0, 0, time.UTC)
	if got := Filename(ts); got != "Screenshot-2026-08-15-134300.png" {
		t.Fatalf("filename = %q", got)
	}
	dir := t.TempDir()
	first := UniquePath(dir, "Screenshot-2026-08-15-134300.png")
	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := UniquePath(dir, "Screenshot-2026-08-15-134300.png")
	if second == first {
		t.Fatal("expected a unique suffix")
	}
	if filepath.Base(second) != "Screenshot-2026-08-15-134300-2.png" {
		t.Fatalf("second = %q", second)
	}
}
