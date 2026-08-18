package app

import (
	"image"
	"testing"
)

func TestClampAndroidColWidths(t *testing.T) {
	u := 24
	size, mod := clampAndroidColWidths(1, 1, u)
	if size != androidMinSizeCol(u) || mod != androidMinModCol(u) {
		t.Fatalf("min clamp = %d,%d", size, mod)
	}
	size, mod = clampAndroidColWidths(100000, 100000, u)
	if size != androidMaxCol(u) || mod != androidMaxCol(u) {
		t.Fatalf("max clamp = %d,%d", size, mod)
	}
	size, mod = clampAndroidColWidths(7*u, 11*u, u)
	if size != 7*u || mod != 11*u {
		t.Fatalf("passthrough = %d,%d", size, mod)
	}
}

func TestAndroidFitColWidths(t *testing.T) {
	u := 24
	size, mod := androidFitColWidths(6*u, 10*u, 20*u, u)
	if size != androidMinSizeCol(u) {
		t.Fatalf("narrow row should shrink size: %d", size)
	}
	if mod < androidMinModCol(u) {
		t.Fatalf("mod below min: %d", mod)
	}
	size, mod = androidFitColWidths(6*u, 10*u, 80*u, u)
	if size != 6*u || mod != 10*u {
		t.Fatalf("wide row = %d,%d", size, mod)
	}
}

func TestAndroidSplitterHit(t *testing.T) {
	b := image.Rect(10, 0, 410, 24)
	sizeW, modW, gap := 100, 150, 3
	sl, ml := androidSplitterXs(b, sizeW, modW, gap)
	if ml != 260 || sl != 157 {
		t.Fatalf("splitters = %d,%d", sl, ml)
	}
	if got := androidSplitterAt(157, sl, ml, 4); got != androidColDragSize {
		t.Fatalf("size hit = %d", got)
	}
	if got := androidSplitterAt(260, sl, ml, 4); got != androidColDragMod {
		t.Fatalf("mod hit = %d", got)
	}
	if got := androidSplitterAt(200, sl, ml, 4); got != 0 {
		t.Fatalf("miss = %d", got)
	}
}

func TestAndroidHeaderColAt(t *testing.T) {
	b := image.Rect(10, 0, 410, 24)
	sizeW, modW, gap, slop := 100, 150, 3, 4
	// splitters at 157 (size) and 260 (mod)
	if got := androidHeaderColAt(80, b, sizeW, modW, gap, slop); got != androidSortName {
		t.Fatalf("name = %d", got)
	}
	if got := androidHeaderColAt(200, b, sizeW, modW, gap, slop); got != androidSortSize {
		t.Fatalf("size = %d", got)
	}
	if got := androidHeaderColAt(300, b, sizeW, modW, gap, slop); got != androidSortMod {
		t.Fatalf("mod = %d", got)
	}
	if got := androidHeaderColAt(157, b, sizeW, modW, gap, slop); got != 0 {
		t.Fatalf("size splitter = %d", got)
	}
	if got := androidHeaderColAt(260, b, sizeW, modW, gap, slop); got != 0 {
		t.Fatalf("mod splitter = %d", got)
	}
	if got := androidHeaderColAt(5, b, sizeW, modW, gap, slop); got != 0 {
		t.Fatalf("outside = %d", got)
	}
}

func TestAndroidSortMark(t *testing.T) {
	if got := androidSortMark(androidSortName, androidSortName, false); got != " ▲" {
		t.Fatalf("asc = %q", got)
	}
	if got := androidSortMark(androidSortSize, androidSortName, true); got != "" {
		t.Fatalf("inactive = %q", got)
	}
	if got := androidSortMark(androidSortMod, androidSortMod, true); got != " ▼" {
		t.Fatalf("desc = %q", got)
	}
}
