package userdir

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func TestEnsureAndroidScreenshotsCreates(t *testing.T) {
	home := t.TempDir()
	restore := Override("darwin", home, nil)
	t.Cleanup(restore)

	dir, err := EnsureAndroidScreenshots()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Pictures", "Screenshots", "Android")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("stat = %v err=%v", dir, err)
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

func TestOpenInFileManagerSelectsFileDarwin(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := Override("darwin", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)

	if err := OpenInFileManager(file); err != nil {
		t.Fatal(err)
	}
	if len(log.ran) != 1 || log.ran[0][0] != "open" {
		t.Fatalf("ran = %q", log.ran)
	}
	if got := log.ran[0][1:]; len(got) != 2 || got[0] != "-R" || got[1] != file {
		t.Fatalf("args = %q", got)
	}
	if len(log.started) != 0 {
		t.Fatalf("started = %q", log.started)
	}
}

func TestOpenInFileManagerOpensDirDarwin(t *testing.T) {
	dir := t.TempDir()
	restore := Override("darwin", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)

	if err := OpenInFileManager(dir); err != nil {
		t.Fatal(err)
	}
	if len(log.started) != 1 || log.started[0][0] != "open" || log.started[0][1] != dir {
		t.Fatalf("started = %q", log.started)
	}
}

func TestOpenInFileManagerSelectsFileLinux(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := Override("linux", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)

	if err := OpenInFileManager(file); err != nil {
		t.Fatal(err)
	}
	if len(log.ran) != 1 || log.ran[0][0] != "dbus-send" {
		t.Fatalf("ran = %q", log.ran)
	}
	args := log.ran[0][1:]
	want := linuxShowItemsArgs(fileURI(file))
	if !slices.Equal(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
	if len(log.started) != 0 {
		t.Fatalf("opened dir despite successful reveal: %q", log.started)
	}
}

func TestOpenInFileManagerLinuxFallsBackWhenRevealFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := Override("linux", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)
	log.runErr = errRevealFailed

	if err := OpenInFileManager(file); err != nil {
		t.Fatal(err)
	}
	if len(log.started) != 1 || log.started[0][0] != "xdg-open" || log.started[0][1] != dir {
		t.Fatalf("started = %q", log.started)
	}
}

func TestOpenInFileManagerOpensDirLinux(t *testing.T) {
	dir := t.TempDir()
	restore := Override("linux", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)

	if err := OpenInFileManager(dir); err != nil {
		t.Fatal(err)
	}
	if len(log.started) != 1 || log.started[0][0] != "xdg-open" || log.started[0][1] != dir {
		t.Fatalf("started = %q", log.started)
	}
}

func TestOpenInFileManagerMissingFileOpensParent(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone.png")
	restore := Override("linux", dir, nil)
	t.Cleanup(restore)
	log := stubCommands(t)

	if err := OpenInFileManager(missing); err != nil {
		t.Fatal(err)
	}
	if len(log.started) != 1 || log.started[0][0] != "xdg-open" || log.started[0][1] != dir {
		t.Fatalf("started = %q", log.started)
	}
}

func TestOpenInFileManagerEmptyPath(t *testing.T) {
	if err := OpenInFileManager(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestFileURIEncodesNonASCII(t *testing.T) {
	path := "/home/yota/画像/Screenshots/撮影.png"
	got := fileURI(path)
	want := "file:///home/yota/%E7%94%BB%E5%83%8F/Screenshots/%E6%92%AE%E5%BD%B1.png"
	if got != want {
		t.Fatalf("uri = %q, want %q", got, want)
	}
	spaced := fileURI("/tmp/my shot.png")
	if spaced != "file:///tmp/my%20shot.png" {
		t.Fatalf("spaced uri = %q", spaced)
	}
}

type cmdLog struct {
	started [][]string
	ran     [][]string
	runErr  error
}

var errRevealFailed = errors.New("reveal failed")

func stubCommands(t *testing.T) *cmdLog {
	t.Helper()
	log := &cmdLog{}
	origStart, origRun := startCommand, runCommand
	startCommand = log.start
	runCommand = log.run
	t.Cleanup(func() {
		startCommand = origStart
		runCommand = origRun
	})
	return log
}

func (c *cmdLog) start(name string, args ...string) error {
	c.started = append(c.started, append([]string{name}, args...))
	return nil
}

func (c *cmdLog) run(name string, args ...string) error {
	c.ran = append(c.ran, append([]string{name}, args...))
	return c.runErr
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
