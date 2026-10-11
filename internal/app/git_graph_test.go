package app

import "testing"

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
