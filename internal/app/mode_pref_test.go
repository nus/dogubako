package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModePersists(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	if m.Mode() != ToolImage {
		t.Fatalf("default mode = %q", m.Mode())
	}
	m.SetMode(ToolGit)
	if m.Mode() != ToolGit {
		t.Fatalf("mode = %q", m.Mode())
	}
	data, err := os.ReadFile(filepath.Join(dir, "dogubako", "mode"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != string(ToolGit) {
		t.Fatalf("saved = %q", data)
	}

	var again Model
	if again.Mode() != ToolGit {
		t.Fatalf("reloaded mode = %q", again.Mode())
	}
	again.SetMode(ToolImage)
	if again.Mode() != ToolImage {
		t.Fatalf("mode = %q", again.Mode())
	}

	var third Model
	if third.Mode() != ToolImage {
		t.Fatalf("reloaded mode = %q", third.Mode())
	}
}

func TestModeIgnoresUnknown(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	if err := os.MkdirAll(filepath.Join(dir, "dogubako"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dogubako", "mode"), []byte("missing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var m Model
	if m.Mode() != ToolImage {
		t.Fatalf("unknown mode = %q", m.Mode())
	}
	m.SetMode(ToolID("missing"))
	if m.Mode() != ToolImage {
		t.Fatalf("rejected mode = %q", m.Mode())
	}
}
