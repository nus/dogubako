package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSidebarWidth(t *testing.T) {
	const u = 24
	if got := sidebarWidth(u, false); got != 8*u {
		t.Fatalf("expanded width = %d", got)
	}
	wantCollapsed := u + 2*sidebarPadding(u)
	if got := sidebarWidth(u, true); got != wantCollapsed {
		t.Fatalf("collapsed width = %d, want %d", got, wantCollapsed)
	}
	if sidebarWidth(u, true) >= sidebarWidth(u, false) {
		t.Fatal("collapsed sidebar should be narrower")
	}
}

func TestSidebarToggleMarksUseAvailableGlyphs(t *testing.T) {
	if sidebarCollapseMark != "◀" || sidebarExpandMark != "▶" {
		t.Fatalf("marks = %q %q", sidebarCollapseMark, sidebarExpandMark)
	}
	for _, s := range []string{sidebarCollapseMark, sidebarExpandMark} {
		if strings.ContainsRune(s, '\u25C2') || strings.ContainsRune(s, '\u25B8') {
			t.Fatalf("small triangle missing on macOS: %q", s)
		}
	}
}

func TestSidebarCollapsedPersists(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	if m.SidebarCollapsed() {
		t.Fatal("default should be expanded")
	}
	m.ToggleSidebar()
	if !m.SidebarCollapsed() {
		t.Fatal("toggle should collapse")
	}
	data, err := os.ReadFile(filepath.Join(dir, "dogubako", "sidebar"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "collapsed" {
		t.Fatalf("saved = %q", data)
	}

	var again Model
	if !again.SidebarCollapsed() {
		t.Fatal("reloaded model should stay collapsed")
	}
	again.SetSidebarCollapsed(true)
	again.SetSidebarCollapsed(false)
	if again.SidebarCollapsed() {
		t.Fatal("set expanded")
	}

	var third Model
	if third.SidebarCollapsed() {
		t.Fatal("reloaded model should stay expanded")
	}
}

func TestSidebarCollapsedIgnoresUnknown(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	if err := os.MkdirAll(filepath.Join(dir, "dogubako"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dogubako", "sidebar"), []byte("maybe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var m Model
	if m.SidebarCollapsed() {
		t.Fatal("unknown value should stay expanded")
	}
}
