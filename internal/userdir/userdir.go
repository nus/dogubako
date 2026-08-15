package userdir

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Default screenshot location:
//
//   - Ubuntu / Linux: XDG_SCREENSHOTS_DIR if set, otherwise
//     XDG_PICTURES_DIR/Screenshots. Pictures comes from the environment,
//     then ~/.config/user-dirs.dirs (so a Japanese desktop uses ~/画像, not
//     a newly invented ~/Pictures).
//   - macOS: ~/Pictures/Screenshots. Finder's Pictures folder is always
//     named Pictures on disk; the Desktop default used by Cmd-Shift-3 is
//     skipped so captures do not pile onto the desktop.
//
// Hidden app dirs (~/.config, ~/Library/Application Support) are for settings,
// not for images the user expects to find in a file manager.

const screenshotsSubdir = "Screenshots"

var (
	currentOS   = runtime.GOOS
	userHomeDir = os.UserHomeDir
	getenv      = os.Getenv
	stat        = os.Stat
	mkdirAll    = os.MkdirAll
	readFile    = os.ReadFile
)

// Pictures returns the user's pictures directory.
func Pictures() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	if currentOS == "linux" {
		if dir := xdgValue(home, "XDG_PICTURES_DIR"); dir != "" {
			return dir, nil
		}
	}
	pictures := filepath.Join(home, "Pictures")
	if currentOS == "darwin" {
		return pictures, nil
	}
	if fi, err := stat(pictures); err == nil && fi.IsDir() {
		return pictures, nil
	}
	return home, nil
}

// Screenshots returns the directory where this app should write captures.
// The directory is not created here; see EnsureScreenshots.
func Screenshots() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	if currentOS == "linux" {
		if dir := xdgValue(home, "XDG_SCREENSHOTS_DIR"); dir != "" {
			return dir, nil
		}
	}
	pictures, err := Pictures()
	if err != nil {
		return "", err
	}
	if pictures == home {
		return filepath.Join(home, screenshotsSubdir), nil
	}
	return filepath.Join(pictures, screenshotsSubdir), nil
}

// EnsureScreenshots returns Screenshots after creating it if needed.
func EnsureScreenshots() (string, error) {
	dir, err := Screenshots()
	if err != nil {
		return "", err
	}
	if err := mkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create screenshot directory: %w", err)
	}
	return dir, nil
}

// Filename returns a timestamped PNG name, e.g. Screenshot-2026-08-15-134300.png.
func Filename(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("Screenshot-2006-01-02-150405.png")
}

// UniquePath returns dir/name, adding -2, -3, … if the file already exists.
func UniquePath(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := stat(path); err != nil {
		return path
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := stat(candidate); err != nil {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, time.Now().UnixNano(), ext))
}

// SuggestedPath is EnsureScreenshots plus a unique timestamped filename.
func SuggestedPath(t time.Time) (string, error) {
	dir, err := EnsureScreenshots()
	if err != nil {
		return "", err
	}
	return UniquePath(dir, Filename(t)), nil
}

func xdgValue(home, key string) string {
	if v := expandXDG(getenv(key), home); v != "" {
		return v
	}
	return xdgValueFromFile(home, key)
}

func xdgConfigHome(home string) string {
	if v := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

func xdgValueFromFile(home, key string) string {
	data, err := readFile(filepath.Join(xdgConfigHome(home), "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	return parseUserDirs(string(data), home)[key]
}

func parseUserDirs(contents, home string) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(contents))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !strings.HasPrefix(key, "XDG_") || !strings.HasSuffix(key, "_DIR") {
			continue
		}
		if dir := expandXDG(val, home); dir != "" {
			out[key] = dir
		}
	}
	return out
}

// OpenInFileManager reveals dir in Finder (macOS) or the file manager (Ubuntu).
func OpenInFileManager(dir string) error {
	if dir == "" {
		return fmt.Errorf("no directory")
	}
	var cmd *exec.Cmd
	switch currentOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

// Override replaces OS/home lookup. Tests should defer the returned restore function.
func Override(goos, home string, env map[string]string) (restore func()) {
	origOS, origHome, origEnv, origStat, origMkdir, origRead := currentOS, userHomeDir, getenv, stat, mkdirAll, readFile
	currentOS = goos
	userHomeDir = func() (string, error) { return home, nil }
	getenv = func(key string) string {
		if env != nil {
			return env[key]
		}
		return ""
	}
	stat = os.Stat
	mkdirAll = os.MkdirAll
	readFile = os.ReadFile
	return func() {
		currentOS, userHomeDir, getenv, stat, mkdirAll, readFile = origOS, origHome, origEnv, origStat, origMkdir, origRead
	}
}

func expandXDG(val, home string) string {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"'`)
	if val == "" {
		return ""
	}
	val = strings.ReplaceAll(val, "${HOME}", home)
	val = strings.ReplaceAll(val, "$HOME", home)
	if !filepath.IsAbs(val) {
		val = filepath.Join(home, val)
	}
	return filepath.Clean(val)
}
