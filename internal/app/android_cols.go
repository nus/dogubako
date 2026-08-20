package app

import "image"

const (
	androidColDragSize = 1
	androidColDragMod  = 2
)

func androidColGap(u int) int { return u / 4 }

func androidExpandColWidth(u int) int { return u }

func androidIconColWidth(u int) int { return 2 * u }

func androidDefaultSizeCol(u int) int { return 6 * u }

func androidDefaultModCol(u int) int { return 10 * u }

func androidMinSizeCol(u int) int { return 4 * u }

func androidMinModCol(u int) int { return 6 * u }

func androidMaxCol(u int) int { return 48 * u }

func androidMinNameCol(u int) int { return 8 * u }

func androidSplitterSlop(u int) int {
	s := u / 4
	if s < 4 {
		return 4
	}
	return s
}

func clampAndroidColWidths(sizeW, modW, u int) (int, int) {
	minS, minM := androidMinSizeCol(u), androidMinModCol(u)
	maxW := androidMaxCol(u)
	if sizeW < minS {
		sizeW = minS
	}
	if modW < minM {
		modW = minM
	}
	if sizeW > maxW {
		sizeW = maxW
	}
	if modW > maxW {
		modW = maxW
	}
	return sizeW, modW
}

// androidFitColWidths keeps a minimum name column when the row is narrow.
func androidFitColWidths(sizeW, modW, rowW, u int) (int, int) {
	sizeW, modW = clampAndroidColWidths(sizeW, modW, u)
	if rowW <= 0 {
		return sizeW, modW
	}
	gap := androidColGap(u)
	minS, minM := androidMinSizeCol(u), androidMinModCol(u)
	minName := androidMinNameCol(u)
	reserved := func(sw, mw int) int {
		return androidExpandColWidth(u) + androidIconColWidth(u) + sw + mw + 4*gap + minName
	}
	overflow := reserved(sizeW, modW) - rowW
	if overflow <= 0 {
		return sizeW, modW
	}
	shrink := overflow
	if extra := sizeW - minS; extra > 0 {
		d := extra
		if d > shrink {
			d = shrink
		}
		sizeW -= d
		shrink -= d
	}
	if shrink > 0 {
		if extra := modW - minM; extra > 0 {
			d := extra
			if d > shrink {
				d = shrink
			}
			modW -= d
		}
	}
	return sizeW, modW
}

func androidColWidthAfterDrag(originW, originX, x int) int {
	return originW - (x - originX)
}

func androidSplitterXs(bounds image.Rectangle, sizeW, modW, gap int) (sizeLeft, modLeft int) {
	modLeft = bounds.Max.X - modW
	sizeLeft = modLeft - gap - sizeW
	return sizeLeft, modLeft
}

func androidSplitterAt(x, sizeLeft, modLeft, slop int) int {
	if d := x - sizeLeft; d <= slop && d >= -slop {
		return androidColDragSize
	}
	if d := x - modLeft; d <= slop && d >= -slop {
		return androidColDragMod
	}
	return 0
}

const (
	androidSortName = 1
	androidSortSize = 2
	androidSortMod  = 3
)

func androidHeaderColAt(x int, bounds image.Rectangle, sizeW, modW, gap, slop int) int {
	if x < bounds.Min.X || x >= bounds.Max.X {
		return 0
	}
	sl, ml := androidSplitterXs(bounds, sizeW, modW, gap)
	if androidSplitterAt(x, sl, ml, slop) != 0 {
		return 0
	}
	if x >= ml {
		return androidSortMod
	}
	if x >= sl {
		return androidSortSize
	}
	return androidSortName
}

func androidAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func androidSortMark(col, current int, desc bool) string {
	if col != current {
		return ""
	}
	if desc {
		return " ▼"
	}
	return " ▲"
}
