package cjkembed

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestDefaultEmojiPaths(t *testing.T) {
	paths := defaultEmojiPaths()
	switch runtime.GOOS {
	case "darwin":
		if len(paths) == 0 || !strings.Contains(paths[0], "Apple Color Emoji") {
			t.Fatalf("darwin paths = %q", paths)
		}
	case "linux":
		if len(paths) == 0 || !strings.Contains(paths[0], "NotoColorEmoji") {
			t.Fatalf("linux paths = %q", paths)
		}
	default:
		if len(paths) != 0 {
			t.Fatalf("stub paths = %q", paths)
		}
	}
}

func TestOpenEmojiMissing(t *testing.T) {
	_, _, err := openEmoji([]string{filepath.Join(t.TempDir(), "missing.ttf")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenEmojiTTF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GoRegular.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	src, gotPath, err := openEmoji([]string{
		filepath.Join(dir, "missing.ttf"),
		path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q", gotPath)
	}
	if src == nil {
		t.Fatal("nil face")
	}
}

func TestPickEmojiFacePrefersEmojiFamily(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GoRegular.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := loadFaceSources(path)
	if err != nil {
		t.Fatal(err)
	}
	if pickEmojiFace(sources) != sources[0] {
		t.Fatal("fallback should be first face")
	}
	if pickEmojiFace(nil) != nil {
		t.Fatal("empty")
	}
}
