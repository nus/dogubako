package app

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/i18n"
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

func TestSidebarItemPaddingFitsCollapsedRail(t *testing.T) {
	if got := sidebarItemPadding(false, 6); got != (guigui.Padding{}) {
		t.Fatalf("expanded padding = %+v", got)
	}
	if got := sidebarItemPadding(true, 0); got != (guigui.Padding{}) {
		t.Fatalf("zero inset = %+v", got)
	}
	got := sidebarItemPadding(true, 6)
	if got.Start != -6 || got.End != -6 || got.Top != 0 || got.Bottom != 0 {
		t.Fatalf("collapsed padding = %+v", got)
	}
}

func TestSidebarTooltipUsesCurrentTitle(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if !sidebarHasTool(ToolMTP) {
			t.Fatal("macOS sidebar should include MTP")
		}
	} else if sidebarHasTool(ToolMTP) {
		t.Fatal("Ubuntu sidebar should omit MTP")
	}
	for _, lang := range []i18n.Lang{i18n.JA, i18n.EN} {
		for _, tool := range Tools {
			title := tool.Title(lang)
			var row sidebarToolRow
			row.Set(tool.ID, title, false)
			if row.tooltipText() != "" {
				t.Fatalf("expanded %s tip = %q", tool.ID, row.tooltipText())
			}
			row.Set(tool.ID, title, true)
			if row.tooltipText() != title {
				t.Fatalf("collapsed %s tip = %q, want %q", tool.ID, row.tooltipText(), title)
			}
		}
	}
}

func TestSidebarTipBoundsSitToTheRight(t *testing.T) {
	icon := image.Rect(8, 40, 32, 64)
	size := image.Pt(80, 20)
	app := image.Rect(0, 0, 400, 300)
	got := sidebarTipBounds(icon, size, app, 4)
	if got.Min.X != icon.Max.X+4 || got.Max.X != got.Min.X+size.X {
		t.Fatalf("horizontal = %v", got)
	}
	if got.Min.Y != 42 || got.Dy() != size.Y {
		t.Fatalf("vertical = %v", got)
	}

	tall := sidebarTipBounds(image.Rect(8, 0, 32, 24), image.Pt(40, 40), app, 4)
	if tall.Min.Y != 0 {
		t.Fatalf("top clamp = %d", tall.Min.Y)
	}
	if tall.Min.X != 36 {
		t.Fatalf("still to the right, x = %d", tall.Min.X)
	}
}

func TestSidebarTipOpensOnHover(t *testing.T) {
	if open, changed := sidebarTipTransition(false, false); open || changed {
		t.Fatalf("idle open=%v changed=%v", open, changed)
	}
	open, changed := sidebarTipTransition(true, false)
	if !open || !changed {
		t.Fatalf("enter open=%v changed=%v", open, changed)
	}
	open, changed = sidebarTipTransition(true, true)
	if !open || changed {
		t.Fatalf("stay open=%v changed=%v", open, changed)
	}
	open, changed = sidebarTipTransition(false, true)
	if open || !changed {
		t.Fatalf("leave open=%v changed=%v", open, changed)
	}
}

func sidebarHasTool(id ToolID) bool {
	for _, tool := range Tools {
		if tool.ID == id {
			return true
		}
	}
	return false
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
