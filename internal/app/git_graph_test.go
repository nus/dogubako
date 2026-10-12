package app

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestGitGraphColWidth(t *testing.T) {
	const u = 24
	const lanes = 4
	natural := gitGraphNaturalWidth(u, lanes)
	if got := gitGraphColWidth(2000, u, lanes, 0); got != natural {
		t.Fatalf("auto = %d, want %d", got, natural)
	}

	wider := natural + 80
	if got := gitGraphColWidth(2000, u, lanes, float64(wider)/float64(u)); got != wider {
		t.Fatalf("wide = %d, want %d", got, wider)
	}

	minW := gitGraphMinWidth(u, lanes)
	narrow := minW + 8
	if narrow >= natural {
		t.Fatal("fixture: narrow width is not below the natural width")
	}
	if got := gitGraphColWidth(2000, u, lanes, float64(narrow)/float64(u)); got != narrow {
		t.Fatalf("narrow = %d, want %d", got, narrow)
	}
	if got := gitGraphColWidth(2000, u, lanes, 0.01); got != minW {
		t.Fatalf("min = %d, want %d", got, minW)
	}

	const rowW = 2000
	maxW := rowW - gitGraphReserve(u)
	if got := gitGraphColWidth(rowW, u, lanes, 100); got != maxW {
		t.Fatalf("max = %d, want %d", got, maxW)
	}
}

func TestGitGraphColWidthRoundTrip(t *testing.T) {
	const u = 24
	const lanes = 3
	const rowW = 1600
	for _, w := range []int{40, 80, 120, 400} {
		clamped := clampGitGraphCol(w, rowW, u, lanes)
		got := gitGraphColWidth(rowW, u, lanes, float64(clamped)/float64(u))
		if got != clamped {
			t.Fatalf("width %d clamped to %d came back as %d", w, clamped, got)
		}
	}
}

func TestGitGraphWidthPersists(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	resetGitGraphUnitsPref()
	resetGitMetaUnitsPref()
	t.Cleanup(func() {
		restore()
		resetGitGraphUnitsPref()
		resetGitMetaUnitsPref()
	})

	if got := loadGitGraphUnits(); got != 0 {
		t.Fatalf("missing = %v", got)
	}
	if err := saveGitGraphUnits(0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dogubako", "git-graph-width")); !os.IsNotExist(err) {
		t.Fatalf("zero width wrote a file: %v", err)
	}
	if err := saveGitGraphUnits(12.5); err != nil {
		t.Fatal(err)
	}

	var tool GitTool
	tool.ensureGraphWidth()
	if tool.graphUnits != 12.5 {
		t.Fatalf("loaded = %v", tool.graphUnits)
	}
	if got := preferGraphUnits(0); got != 12.5 {
		t.Fatalf("fallback = %v", got)
	}
	natural := gitGraphNaturalWidth(24, 2)
	if got := gitGraphColWidth(2000, 24, 2, preferGraphUnits(0)); got == natural {
		t.Fatalf("restored width %d collapsed to the lane width", got)
	}
	tool.graphUnits = 4
	tool.ensureGraphWidth()
	if tool.graphUnits != 4 {
		t.Fatalf("second load = %v", tool.graphUnits)
	}

	var again GitTool
	again.graphDragging = true
	again.graphUnits = 9.25
	again.stopGraphDrag(nil)
	if again.graphDragging {
		t.Fatal("drag still active")
	}
	if got := loadGitGraphUnits(); got != 9.25 {
		t.Fatalf("saved on drag end = %v", got)
	}
	again.stopGraphDrag(nil)
	if got := loadGitGraphUnits(); got != 9.25 {
		t.Fatalf("second stop = %v", got)
	}

	for _, bad := range []string{"", "0", "-3", "nope", "nan", "inf"} {
		if err := os.WriteFile(filepath.Join(dir, "dogubako", "git-graph-width"), []byte(bad+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := loadGitGraphUnits(); got != 0 {
			t.Fatalf("invalid %q = %v", bad, got)
		}
	}
}

func TestGitMetaWidthsPersist(t *testing.T) {
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	resetGitMetaUnitsPref()
	t.Cleanup(func() {
		restore()
		resetGitMetaUnitsPref()
	})

	author, hash, date := preferredGitMetaUnits()
	if author != 0 || hash != 0 || date != 0 {
		t.Fatalf("missing = %v %v %v", author, hash, date)
	}
	rememberGitMetaUnits(4.5, 3.25, 6)
	saved := loadGitMetaUnits()
	if saved.Author != 4.5 || saved.Hash != 3.25 || saved.Date != 6 {
		t.Fatalf("saved = %+v", saved)
	}

	var tool GitTool
	tool.ensureMetaWidths()
	if tool.authorUnits != 4.5 || tool.hashUnits != 3.25 || tool.dateUnits != 6 {
		t.Fatalf("loaded = %v %v %v", tool.authorUnits, tool.hashUnits, tool.dateUnits)
	}
	tool.authorUnits = 2
	tool.ensureMetaWidths()
	if tool.authorUnits != 2 {
		t.Fatalf("second load = %v", tool.authorUnits)
	}

	const u = 24
	defAuthor, defHash, defDate := gitMetaCols(u)
	authorU, hashU, dateU := preferMetaUnits(0, 0, 0)
	gotAuthor, gotHash, gotDate := gitMetaColWidths(4000, u, 40, authorU, hashU, dateU)
	if gotAuthor == defAuthor && gotHash == defHash && gotDate == defDate {
		t.Fatal("restored columns stayed at the defaults")
	}
	wantAuthor, wantHash, wantDate := int(authorU*float64(u)+0.5), int(hashU*float64(u)+0.5), int(dateU*float64(u)+0.5)
	if gotAuthor != wantAuthor || gotHash != wantHash || gotDate != wantDate {
		t.Fatalf("restored = %d %d %d", gotAuthor, gotHash, gotDate)
	}

	resetGitMetaUnitsPref()
	if err := os.WriteFile(filepath.Join(dir, "dogubako", "git-meta-widths.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := loadGitMetaUnits()
	if empty.Author != 0 || empty.Hash != 0 || empty.Date != 0 {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestGitGraphColWidthTight(t *testing.T) {
	got := gitGraphColWidth(80, 24, 20, 0)
	if got < 1 || got >= 80 {
		t.Fatalf("tight = %d", got)
	}
}

func TestGitMetaColsResize(t *testing.T) {
	const u = 24
	author, hash, date := gitMetaColWidths(2000, u, 40, 0, 0, 0)
	defA, defH, defD := gitMetaCols(u)
	if author != defA || hash != defH || date != defD {
		t.Fatalf("defaults = %d %d %d, want %d %d %d", author, hash, date, defA, defH, defD)
	}

	const gap = 4
	const rowRight = 800
	subjectR, authorL, authorR, hashL, hashR, _, dateR := gitMetaEdges(rowRight, gap, author, hash, date)
	if got := gitResizeAuthor(authorR, gap, subjectR, 10, 1000); got != author {
		t.Fatalf("title edge at rest = %d, want %d", got, author)
	}
	if got := gitResizeAuthor(authorR, gap, subjectR+30, 10, 1000); got != author-30 {
		t.Fatalf("wider title = %d, want %d", got, author-30)
	}
	aw, hw := gitTradeCols(authorL, hashR, gap, authorR, 10, 10)
	if aw != author || hw != hash {
		t.Fatalf("author edge at rest = %d %d", aw, hw)
	}
	aw, hw = gitTradeCols(authorL, hashR, gap, authorR+20, 10, 10)
	if aw != author+20 || hw != hash-20 {
		t.Fatalf("wider author = %d %d, want %d %d", aw, hw, author+20, hash-20)
	}
	hw, dw := gitTradeCols(hashL, dateR, gap, hashR, 10, 10)
	if hw != hash || dw != date {
		t.Fatalf("date edge at rest = %d %d", hw, dw)
	}
	hw, dw = gitTradeCols(hashL, dateR, gap, hashR-15, 10, 10)
	if hw != hash-15 || dw != date+15 {
		t.Fatalf("wider date = %d %d, want %d %d", hw, dw, hash-15, date+15)
	}
}

func TestGitFitMetaCols(t *testing.T) {
	const u = 24
	author, hash, date := gitFitMetaCols(1000, 1000, 1000, 80, u, 10)
	if author != gitMinAuthorCol(u) || hash != gitMinHashCol(u) || date != gitMinDateCol(u) {
		t.Fatalf("narrow = %d %d %d", author, hash, date)
	}
	author, hash, date = gitFitMetaCols(8*u, 5*u, 9*u, 4000, u, 40)
	if author != 8*u || hash != 5*u || date != 9*u {
		t.Fatalf("wide = %d %d %d", author, hash, date)
	}
}

func TestGitLaneStep(t *testing.T) {
	for _, u := range []int{16, 20, 24, 32} {
		lane := gitLaneWidth(u)
		for _, lanes := range []int{1, 2, 5, 8} {
			col := gitGraphNaturalWidth(u, lanes)
			if got := gitLaneStep(col, lanes, u); got != lane {
				t.Fatalf("u=%d lanes=%d step=%d, want %d", u, lanes, got, lane)
			}
			if got := gitLaneStep(col+200, lanes, u); got != lane {
				t.Fatalf("u=%d lanes=%d wide step=%d, want %d", u, lanes, got, lane)
			}
		}
	}

	const u = 24
	const lanes = 4
	narrow := gitGraphMinWidth(u, lanes)
	if got := gitLaneStep(narrow, lanes, u); got >= gitLaneWidth(u) {
		t.Fatalf("packed step = %d, natural %d", got, gitLaneWidth(u))
	}
}

func uniformRows(n, h, origin int) func(int) image.Rectangle {
	return func(i int) image.Rectangle {
		if i < 0 || i >= n {
			return image.Rectangle{}
		}
		y := origin + i*h
		return image.Rect(0, y, 10, y+h)
	}
}

func TestGraphPageIndex(t *testing.T) {
	view := image.Rect(0, 0, 10, 100)
	bounds := uniformRows(20, 20, 0)
	if got := graphTopIndex(20, bounds, view); got != 0 {
		t.Fatalf("top = %d", got)
	}
	if got := graphPageIndex(20, bounds, view, true); got != 5 {
		t.Fatalf("page down = %d, want 5", got)
	}
	if got := graphPageIndex(20, bounds, view, false); got != 0 {
		t.Fatalf("page up at top = %d", got)
	}

	// Rows 5–9 fill the viewport.
	scrolled := uniformRows(20, 20, -100)
	if got := graphTopIndex(20, scrolled, view); got != 5 {
		t.Fatalf("scrolled top = %d", got)
	}
	if got := graphPageIndex(20, scrolled, view, true); got != 10 {
		t.Fatalf("scrolled page down = %d, want 10", got)
	}
	if got := graphPageIndex(20, scrolled, view, false); got != 0 {
		t.Fatalf("scrolled page up = %d, want 0", got)
	}

	// A partial row at the bottom becomes the next top row.
	short := image.Rect(0, 0, 10, 90)
	if got := graphPageIndex(20, bounds, short, true); got != 4 {
		t.Fatalf("partial page down = %d, want 4", got)
	}

	// The loaded rows already fit, so page down stays put.
	if got := graphPageIndex(3, uniformRows(3, 20, 0), view, true); got != 0 {
		t.Fatalf("short list page down = %d", got)
	}

	// A row taller than the viewport steps to the next row.
	tall := func(i int) image.Rectangle {
		switch i {
		case 0:
			return image.Rect(0, 0, 10, 200)
		case 1:
			return image.Rect(0, 200, 10, 220)
		default:
			return image.Rectangle{}
		}
	}
	if got := graphPageIndex(2, tall, view, true); got != 1 {
		t.Fatalf("tall page down = %d, want 1", got)
	}
}

func TestNextGraphCommit(t *testing.T) {
	// 0 commit, 1 detail panel, 2 commit, 3 commit
	commit := func(i int) bool { return i != 1 }
	if got := nextGraphCommit(4, commit, 0, true); got != 2 {
		t.Fatalf("down skips detail = %d, want 2", got)
	}
	if got := nextGraphCommit(4, commit, 2, false); got != 0 {
		t.Fatalf("up skips detail = %d, want 0", got)
	}
	if got := nextGraphCommit(4, commit, 3, true); got != 3 {
		t.Fatalf("down at end = %d, want 3", got)
	}
	if got := nextGraphCommit(4, commit, 0, false); got != 0 {
		t.Fatalf("up at start = %d, want 0", got)
	}
	if got := nextGraphCommit(4, commit, -1, true); got != 0 {
		t.Fatalf("from none down = %d, want 0", got)
	}
	if got := nextGraphCommit(4, commit, -1, false); got != 3 {
		t.Fatalf("from none up = %d, want 3", got)
	}
	if got := nextGraphCommit(0, commit, 0, true); got != -1 {
		t.Fatalf("empty = %d, want -1", got)
	}
}

func TestNearestCommitIndex(t *testing.T) {
	values := []string{"a", "", "b"}
	value := func(i int) string { return values[i] }
	if got := nearestCommitIndex(len(values), value, 1); got != 0 {
		t.Fatalf("expand row = %d, want 0", got)
	}
	if got := nearestCommitIndex(len(values), value, 2); got != 2 {
		t.Fatalf("last commit = %d", got)
	}
	if got := nearestCommitIndex(0, value, 0); got != -1 {
		t.Fatalf("empty = %d", got)
	}
}

func TestCommitOpenAfterSelect(t *testing.T) {
	if got := commitOpenAfterSelect("", "bbb", true); got != "" {
		t.Fatalf("arrow while closed = %q", got)
	}
	if got := commitOpenAfterSelect("aaa", "bbb", true); got != "bbb" {
		t.Fatalf("arrow while open = %q", got)
	}
	if got := commitOpenAfterSelect("aaa", gitcli.Uncommitted, true); got != "" {
		t.Fatalf("arrow to uncommitted = %q", got)
	}
	if got := commitOpenAfterSelect("", "bbb", false); got != "bbb" {
		t.Fatalf("click while closed = %q", got)
	}
	if got := commitOpenAfterSelect("aaa", "bbb", false); got != "bbb" {
		t.Fatalf("click while open = %q", got)
	}
}

func TestHistoryEndHold(t *testing.T) {
	var tool GitTool
	tool.trackHistoryEnd("repo")
	if tool.holdHistoryEnd(false, false) {
		t.Fatal("unpinned hold")
	}
	tool.pinHistoryEnd()
	if !tool.holdHistoryEnd(false, false) {
		t.Fatal("jump toward the end should hold")
	}
	if !tool.holdHistoryEnd(true, false) {
		t.Fatal("arrived end should hold")
	}
	if tool.holdHistoryEnd(true, true) {
		t.Fatal("wheel down should release")
	}

	tool.pinHistoryEnd()
	if !tool.holdHistoryEnd(true, false) {
		t.Fatal("pin again")
	}
	if tool.holdHistoryEnd(false, false) {
		t.Fatal("scrolling away should release")
	}

	tool.pinHistoryEnd()
	tool.trackHistoryEnd("other")
	if tool.holdHistoryEnd(true, false) {
		t.Fatal("another repository should release")
	}
}
