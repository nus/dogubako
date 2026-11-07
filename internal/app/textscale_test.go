package app

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
)

func TestTextScaleSteps(t *testing.T) {
	want := []int{75, 100, 125, 150, 175, 200}
	if len(textScalePercents) != len(want) {
		t.Fatalf("steps = %v", textScalePercents)
	}
	for i, pct := range want {
		if textScalePercents[i] != pct {
			t.Fatalf("steps = %v", textScalePercents)
		}
		if i > 0 && pct-want[i-1] != 25 {
			t.Fatalf("gap before %d", pct)
		}
		got := appScaleFromPercent(pct)
		if got != float64(pct)/100 {
			t.Fatalf("scale(%d) = %v", pct, got)
		}
		if textScaleLabel(pct) != strconv.Itoa(pct)+"%" {
			t.Fatalf("label(%d) = %q", pct, textScaleLabel(pct))
		}
	}
	if appScaleFromPercent(80) != 1 {
		t.Fatal("off-step scale should be 1")
	}
}

func TestTextScaleDefaultDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	if SavedAppScale() != 1 {
		t.Fatalf("saved scale = %v", SavedAppScale())
	}
	var m Model
	if m.TextScalePercent() != 100 || m.TextAppScale() != 1 {
		t.Fatalf("percent=%d scale=%v", m.TextScalePercent(), m.TextAppScale())
	}
	if !m.CanShrinkText() || !m.CanGrowText() {
		t.Fatal("100% should step both ways")
	}
	if _, err := os.Stat(filepath.Join(dir, "dogubako", "text-scale")); !os.IsNotExist(err) {
		t.Fatal("reading the default should not write a file")
	}
}

func TestTextScalePersistsOneStep(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	m.StepTextScale(0)
	m.StepTextScale(2)
	if m.TextScalePercent() != 100 {
		t.Fatalf("non-step dir changed percent to %d", m.TextScalePercent())
	}
	m.StepTextScale(1)
	if m.TextScalePercent() != 125 || m.TextAppScale() != 1.25 {
		t.Fatalf("up = %d (%v)", m.TextScalePercent(), m.TextAppScale())
	}
	data, err := os.ReadFile(filepath.Join(dir, "dogubako", "text-scale"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "125\n" {
		t.Fatalf("saved = %q", data)
	}

	var again Model
	if again.TextScalePercent() != 125 || again.TextAppScale() != 1.25 {
		t.Fatalf("reloaded = %d (%v)", again.TextScalePercent(), again.TextAppScale())
	}
	if SavedAppScale() != 1.25 {
		t.Fatalf("launch scale = %v", SavedAppScale())
	}

	again.StepTextScale(-1)
	if again.TextScalePercent() != 100 {
		t.Fatalf("down = %d", again.TextScalePercent())
	}
}

func TestTextScaleStopsAtEnds(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	var m Model
	for m.CanShrinkText() {
		m.StepTextScale(-1)
	}
	if m.TextScalePercent() != 75 || m.CanShrinkText() || !m.CanGrowText() {
		t.Fatalf("min = %d shrink=%v grow=%v", m.TextScalePercent(), m.CanShrinkText(), m.CanGrowText())
	}
	before, err := os.ReadFile(filepath.Join(dir, "dogubako", "text-scale"))
	if err != nil {
		t.Fatal(err)
	}
	m.StepTextScale(-1)
	after, err := os.ReadFile(filepath.Join(dir, "dogubako", "text-scale"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != "75\n" || string(after) != string(before) {
		t.Fatalf("end step wrote %q -> %q", before, after)
	}
	if m.TextAppScale() != 0.75 {
		t.Fatalf("min scale = %v", m.TextAppScale())
	}

	for m.CanGrowText() {
		m.StepTextScale(1)
	}
	if m.TextScalePercent() != 200 || !m.CanShrinkText() || m.CanGrowText() {
		t.Fatalf("max = %d shrink=%v grow=%v", m.TextScalePercent(), m.CanShrinkText(), m.CanGrowText())
	}
	if m.TextAppScale() != 2 {
		t.Fatalf("max scale = %v", m.TextAppScale())
	}
	m.StepTextScale(1)
	if m.TextScalePercent() != 200 {
		t.Fatalf("past max = %d", m.TextScalePercent())
	}
	got, err := os.ReadFile(filepath.Join(dir, "dogubako", "text-scale"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "200\n" {
		t.Fatalf("saved max = %q", got)
	}

	var again Model
	if again.TextScalePercent() != 200 {
		t.Fatalf("reloaded max = %d", again.TextScalePercent())
	}
}

func TestTextScaleEachStepRoundTrips(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	path := filepath.Join(dir, "dogubako", "text-scale")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, pct := range textScalePercents {
		if err := os.WriteFile(path, []byte(strconv.Itoa(pct)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var m Model
		if m.TextScalePercent() != pct || m.TextAppScale() != float64(pct)/100 {
			t.Fatalf("load %d -> %d (%v)", pct, m.TextScalePercent(), m.TextAppScale())
		}
		if SavedAppScale() != float64(pct)/100 {
			t.Fatalf("launch %d -> %v", pct, SavedAppScale())
		}
	}
}

func TestTextScaleInvalidDoesNotRewrite(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	path := filepath.Join(dir, "dogubako", "text-scale")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "\n", "nope", "50", "110", "75%", "75.0", "075", "100.0", " 100 %"} {
		original := []byte(raw)
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatal(err)
		}
		var m Model
		if m.TextScalePercent() != 100 || m.TextAppScale() != 1 {
			t.Fatalf("%q -> %d", raw, m.TextScalePercent())
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != raw {
			t.Fatalf("%q was rewritten to %q", raw, got)
		}
	}
}

func TestTextScaleSaveFailureKeepsSession(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	// A file where the config directory should be makes the write fail.
	if err := os.WriteFile(filepath.Join(dir, "dogubako"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var m Model
	if m.TextScalePercent() != 100 {
		t.Fatalf("fallback = %d", m.TextScalePercent())
	}
	m.StepTextScale(1)
	if m.TextScalePercent() != 125 || m.TextAppScale() != 1.25 {
		t.Fatalf("session = %d (%v)", m.TextScalePercent(), m.TextAppScale())
	}
	m.StepTextScale(1)
	if m.TextScalePercent() != 150 {
		t.Fatalf("second step = %d", m.TextScalePercent())
	}
	if info, err := os.Stat(filepath.Join(dir, "dogubako")); err != nil || info.IsDir() {
		t.Fatal("failed save should leave the blocking file in place")
	}
}

func TestTextScaleConfigDirError(t *testing.T) {
	restore := overrideSidebarConfigDir(func() (string, error) {
		return "", os.ErrNotExist
	})
	t.Cleanup(restore)
	if SavedAppScale() != 1 {
		t.Fatalf("scale = %v", SavedAppScale())
	}
	var m Model
	m.StepTextScale(-1)
	if m.TextScalePercent() != 75 {
		t.Fatalf("session = %d", m.TextScalePercent())
	}
}

func TestTextScaleTooltips(t *testing.T) {
	if got := i18n.T(i18n.JA, i18n.SidebarSmaller); got != "文字を小さく" {
		t.Fatalf("ja smaller = %q", got)
	}
	if got := i18n.T(i18n.EN, i18n.SidebarSmaller); got != "Smaller text" {
		t.Fatalf("en smaller = %q", got)
	}
	if got := i18n.T(i18n.JA, i18n.SidebarLarger); got != "文字を大きく" {
		t.Fatalf("ja larger = %q", got)
	}
	if got := i18n.T(i18n.EN, i18n.SidebarLarger); got != "Larger text" {
		t.Fatalf("en larger = %q", got)
	}
}
