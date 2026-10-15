package app

import (
	"image"
	"testing"
)

func TestGitHistGraphHeight(t *testing.T) {
	const split = 12
	const minPane = 80
	if got := gitHistGraphHeight(900, split, 0, minPane, minPane); got != (900-split)/2 {
		t.Fatalf("default = %d", got)
	}
	avail := 900 - split
	if got := gitHistGraphHeight(900, split, 0.8, minPane, minPane); got != int(float64(avail)*0.8) {
		t.Fatalf("tall graph = %d", got)
	}
	if got := gitHistGraphHeight(900, split, 0.95, minPane, minPane); got != 900-split-minPane {
		t.Fatalf("capped graph = %d", got)
	}
	if got := gitHistGraphHeight(900, split, 0.01, minPane, minPane); got != minPane {
		t.Fatalf("short graph = %d", got)
	}
	if got := gitHistGraphHeight(100, split, 0.5, minPane, minPane); got < 1 || got >= 100-split {
		t.Fatalf("tight pane = %d", got)
	}
}

func TestCursorInGraphItem(t *testing.T) {
	graph := image.Rect(0, 0, 400, 200)
	row := image.Rect(0, 180, 400, 220)
	if !cursorInGraphItem(image.Pt(20, 190), graph, row) {
		t.Fatal("the visible part of a row is a commit click")
	}
	if cursorInGraphItem(image.Pt(20, 210), graph, row) {
		t.Fatal("the part under the graph is the splitter, not a commit click")
	}
}

func TestGitWorkFilesWidth(t *testing.T) {
	const split = 12
	const minFiles = 144
	const minDiff = 192

	if got := gitWorkFilesWidth(900, split, 0, minFiles, minDiff); got != (900-split)/3 {
		t.Fatalf("default split = %d", got)
	}
	if got := gitWorkFilesWidth(900, split, 0.9, minFiles, minDiff); got != 900-split-minDiff {
		t.Fatalf("wide files = %d", got)
	}
	if got := gitWorkFilesWidth(900, split, 0.05, minFiles, minDiff); got != minFiles {
		t.Fatalf("narrow files = %d", got)
	}
	if got := gitWorkFilesWidth(100, split, 0.5, minFiles, minDiff); got < 1 || got >= 100-split {
		t.Fatalf("tight pane = %d", got)
	}
}

func TestGitWorkBandHeight(t *testing.T) {
	const split = 12
	const minBand = 144
	const minDiff = 192
	avail := 900 - split
	if got := gitWorkBandHeight(900, split, 0, minBand, minDiff); got != int(float64(avail)*0.4) {
		t.Fatalf("default band = %d", got)
	}
	if got := gitWorkBandHeight(900, split, 0.7, minBand, minDiff); got != int(float64(avail)*0.7) {
		t.Fatalf("tall band = %d", got)
	}
	if got := gitWorkBandHeight(900, split, 0.95, minBand, minDiff); got != 900-split-minDiff {
		t.Fatalf("capped band = %d", got)
	}
	if got := gitWorkBandHeight(100, split, 0.5, minBand, minDiff); got < 1 || got >= 100-split {
		t.Fatalf("tight band = %d", got)
	}
}

func TestGitChangeFileLabel(t *testing.T) {
	tests := []struct {
		code, path, name, dir string
	}{
		{code: " M", path: "internal/app/git.go", name: "git.go", dir: "internal/app"},
		{code: "M ", path: "README.md", name: "README.md"},
		{code: "??", path: "catalog.go", name: "catalog.go"},
		{code: " D", path: "internal/i18n/catalog.go", name: "catalog.go", dir: "internal/i18n"},
		{code: "R ", path: "internal/old.go -> internal/app/git.go", name: "git.go", dir: "internal/app"},
		{code: "C ", path: "a -> b/c.txt", name: "c.txt", dir: "b"},
		{code: " M", path: "notes/a -> b.md", name: "a -> b.md", dir: "notes"},
		{code: "??", path: "", name: ""},
		{code: "??", path: "dir/", name: "dir"},
	}
	for _, tt := range tests {
		name, dir := gitChangeFileLabel(tt.code, tt.path)
		if name != tt.name || dir != tt.dir {
			t.Fatalf("gitChangeFileLabel(%q, %q) = %q, %q; want %q, %q", tt.code, tt.path, name, dir, tt.name, tt.dir)
		}
	}
}

func TestGitChangeMark(t *testing.T) {
	if got := gitChangeMark(" M", false); got != "M" {
		t.Fatalf("unstaged = %q", got)
	}
	if got := gitChangeMark("M ", true); got != "M" {
		t.Fatalf("staged = %q", got)
	}
	if got := gitChangeMark("MM", false); got != "M" {
		t.Fatalf("both unstaged = %q", got)
	}
	if got := gitChangeMark("MM", true); got != "M" {
		t.Fatalf("both staged = %q", got)
	}
	if got := gitChangeMark("??", false); got != "?" {
		t.Fatalf("untracked = %q", got)
	}
	if got := gitChangeMark("A ", true); got != "A" {
		t.Fatalf("added = %q", got)
	}
	if got := gitChangeMark("D ", true); got != "D" {
		t.Fatalf("deleted = %q", got)
	}
	if got := gitChangeMark("R ", true); got != "R" {
		t.Fatalf("renamed = %q", got)
	}
	if got := gitChangeMark("", false); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestGitChangeLabelWidths(t *testing.T) {
	if name, dir := gitChangeLabelWidths(100, 40, 30, 4); name != 40 || dir != 30 {
		t.Fatalf("fits = %d, %d", name, dir)
	}
	if name, dir := gitChangeLabelWidths(50, 40, 30, 4); name != 40 || dir != 6 {
		t.Fatalf("shrink dir = %d, %d", name, dir)
	}
	if name, dir := gitChangeLabelWidths(30, 40, 30, 4); name != 30 || dir != 0 {
		t.Fatalf("name fills = %d, %d", name, dir)
	}
	if name, dir := gitChangeLabelWidths(44, 40, 30, 4); name != 40 || dir != 0 {
		t.Fatalf("no room for dir = %d, %d", name, dir)
	}
	if name, dir := gitChangeLabelWidths(100, 10, 0, 4); name != 10 || dir != 0 {
		t.Fatalf("root file = %d, %d", name, dir)
	}
	if name, dir := gitChangeLabelWidths(0, 10, 8, 4); name != 0 || dir != 0 {
		t.Fatalf("empty row = %d, %d", name, dir)
	}
}
