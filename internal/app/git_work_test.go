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
