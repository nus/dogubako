package app

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/guigui-gui/guigui/basicwidget"
)

// diffPaint is a darker background on one displayed body, in bytes.
// add selects the green; a deletion uses the red.
type diffPaint struct {
	start, end int
	add        bool
}

// diffInline is that range inside one unified-diff line's payload,
// the text after the leading '+', '-', or context space.
type diffInline struct {
	line       int
	start, end int
	add        bool
}

// diffInlines marks the characters that differ inside paired lines.
// A change block is a run of deletions and additions in one hunk. A
// "\ No newline at end of file" line sits between those lines and does
// not split the block. Equal counts pair from the top. Unequal counts
// pair only lines that still share half of the shorter line, so a pure
// insertion keeps the line background. The comparison is in Unicode
// code points, and it is computed from the diff text already loaded.
func diffInlines(lines []diffSrcLine) []diffInline {
	var out []diffInline
	inHunk := false
	for i := 0; i < len(lines); {
		content := lines[i].content
		if strings.HasPrefix(content, "diff --git ") {
			inHunk = false
			i++
			continue
		}
		if _, _, ok := parseHunkStarts(content); ok {
			inHunk = true
			i++
			continue
		}
		if !inHunk || !isChangeLine(content) {
			i++
			continue
		}
		start := i
		for i < len(lines) && (isChangeLine(lines[i].content) || strings.HasPrefix(lines[i].content, "\\")) {
			i++
		}
		out = blockInlines(out, lines, start, i)
	}
	return out
}

func isChangeLine(content string) bool {
	return content != "" && (content[0] == '-' || content[0] == '+')
}

func blockInlines(dst []diffInline, lines []diffSrcLine, start, end int) []diffInline {
	var delIdx, addIdx []int
	var dels, adds [][]rune
	for i := start; i < end; i++ {
		content := lines[i].content
		if !isChangeLine(content) {
			continue
		}
		payload := content[1:]
		if content[0] == '-' {
			delIdx = append(delIdx, i)
			dels = append(dels, []rune(payload))
		} else {
			addIdx = append(addIdx, i)
			adds = append(adds, []rune(payload))
		}
	}
	for _, p := range pairChangeLines(dels, adds) {
		aSpans, bSpans := diffRuneSpans(dels[p[0]], adds[p[1]])
		dst = appendPayloadInlines(dst, lines[delIdx[p[0]]].content[1:], delIdx[p[0]], aSpans, false)
		dst = appendPayloadInlines(dst, lines[addIdx[p[1]]].content[1:], addIdx[p[1]], bSpans, true)
	}
	return dst
}

func appendPayloadInlines(dst []diffInline, payload string, line int, spans [][2]int, add bool) []diffInline {
	for _, sp := range runeRangesToBytes(payload, spans) {
		if sp[1] <= sp[0] {
			continue
		}
		dst = append(dst, diffInline{line: line, start: sp[0], end: sp[1], add: add})
	}
	return dst
}

// pairChangeLines pairs deletions with additions.
// Same lengths pair in order, including a line that was rewritten.
// Different lengths keep a pair only when the lines are similar, and
// among those alignments prefer more pairs, then a longer common
// subsequence, then the earliest lines.
func pairChangeLines(dels, adds [][]rune) [][2]int {
	n, m := len(dels), len(adds)
	if n == 0 || m == 0 {
		return nil
	}
	if n == m {
		pairs := make([][2]int, n)
		for i := range n {
			pairs[i] = [2]int{i, i}
		}
		return pairs
	}
	if n*m > 4096 {
		return greedyPairs(dels, adds)
	}
	return dpPairs(dels, adds)
}

// similarRunes reports the longest common subsequence and whether it
// covers at least half of the shorter line. Empty text shares nothing.
func similarRunes(a, b []rune) (int, bool) {
	if len(a) == 0 || len(b) == 0 {
		return 0, false
	}
	shorter := min(len(a), len(b))
	if runeOverlap(a, b)*2 < shorter {
		return 0, false
	}
	n := lcsLen(a, b)
	return n, n*2 >= shorter
}

func runeOverlap(a, b []rune) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	counts := make(map[rune]int, len(a))
	for _, r := range a {
		counts[r]++
	}
	n := 0
	for _, r := range b {
		if counts[r] == 0 {
			continue
		}
		counts[r]--
		n++
	}
	return n
}

func lcsLen(a, b []rune) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for _, ra := range a {
		for j, rb := range b {
			if ra == rb {
				cur[j+1] = prev[j] + 1
				continue
			}
			if prev[j+1] >= cur[j] {
				cur[j+1] = prev[j+1]
			} else {
				cur[j+1] = cur[j]
			}
		}
		prev, cur = cur, prev
		clear(cur)
	}
	return prev[len(b)]
}

const (
	pairFromAdd byte = iota + 1
	pairFromDel
	pairFromPair
)

func dpPairs(dels, adds [][]rune) [][2]int {
	n, m := len(dels), len(adds)
	sim := make([]lineSim, n*m)
	for i := range n {
		for j := range m {
			lcs, ok := similarRunes(dels[i], adds[j])
			sim[i*m+j] = lineSim{lcs: lcs, ok: ok}
		}
	}
	stride := m + 1
	at := func(i, j int) int { return i*stride + j }
	pairs := make([]int, (n+1)*stride)
	common := make([]int, (n+1)*stride)
	from := make([]byte, (n+1)*stride)
	for i := 0; i <= n; i++ {
		for j := 0; j <= m; j++ {
			if i == 0 && j == 0 {
				continue
			}
			var bp, bc int
			var bf byte
			have := false
			take := func(p, c int, f byte) {
				if !have || p > bp || (p == bp && c > bc) || (p == bp && c == bc && f < bf) {
					bp, bc, bf = p, c, f
					have = true
				}
			}
			if i > 0 {
				take(pairs[at(i-1, j)], common[at(i-1, j)], pairFromDel)
			}
			if j > 0 {
				take(pairs[at(i, j-1)], common[at(i, j-1)], pairFromAdd)
			}
			if i > 0 && j > 0 {
				if s := sim[(i-1)*m+(j-1)]; s.ok {
					take(pairs[at(i-1, j-1)]+1, common[at(i-1, j-1)]+s.lcs, pairFromPair)
				}
			}
			pairs[at(i, j)] = bp
			common[at(i, j)] = bc
			from[at(i, j)] = bf
		}
	}
	i, j := n, m
	var out [][2]int
	for i > 0 || j > 0 {
		switch from[at(i, j)] {
		case pairFromPair:
			out = append(out, [2]int{i - 1, j - 1})
			i--
			j--
		case pairFromDel:
			i--
		case pairFromAdd:
			j--
		default:
			if j > 0 {
				j--
			} else {
				i--
			}
		}
	}
	slices.Reverse(out)
	return out
}

type lineSim struct {
	lcs int
	ok  bool
}

// greedyPairs is the same rule for a block too large to align exactly.
// It keeps the earliest similar addition inside a short window.
func greedyPairs(dels, adds [][]rune) [][2]int {
	var out [][2]int
	j := 0
	const window = 32
	for i := range dels {
		end := min(len(adds), j+window)
		found := -1
		for k := j; k < end; k++ {
			if _, ok := similarRunes(dels[i], adds[k]); ok {
				found = k
				break
			}
		}
		if found < 0 {
			continue
		}
		out = append(out, [2]int{i, found})
		j = found + 1
	}
	return out
}

// diffRuneSpans returns the code-point ranges of a and b that are not
// part of a common subsequence. A long middle that would be slow to
// align is one range on each side.
func diffRuneSpans(a, b []rune) (aSpans, bSpans [][2]int) {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	a = a[prefix:]
	b = b[prefix:]
	suffix := 0
	for suffix < len(a) && suffix < len(b) && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	a = a[:len(a)-suffix]
	b = b[:len(b)-suffix]
	var as, bs [][2]int
	switch {
	case len(a) == 0 && len(b) == 0:
		return nil, nil
	case len(a) == 0:
		bs = [][2]int{{0, len(b)}}
	case len(b) == 0:
		as = [][2]int{{0, len(a)}}
	case len(a)*len(b) > 250000:
		as = [][2]int{{0, len(a)}}
		bs = [][2]int{{0, len(b)}}
	default:
		as, bs = myersSpans(a, b)
	}
	return shiftSpans(as, prefix), shiftSpans(bs, prefix)
}

func shiftSpans(spans [][2]int, delta int) [][2]int {
	if delta == 0 {
		return spans
	}
	for i := range spans {
		spans[i][0] += delta
		spans[i][1] += delta
	}
	return spans
}

func myersSpans(a, b []rune) (aSpans, bSpans [][2]int) {
	n, m := len(a), len(b)
	maxD := n + m
	v := make([]int, 2*maxD+1)
	offset := maxD
	trace := make([][]int, 0, maxD+1)
	for d := 0; d <= maxD; d++ {
		snap := make([]int, len(v))
		copy(snap, v)
		trace = append(trace, snap)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return backtrackMyers(trace, offset, n, m)
			}
		}
	}
	return [][2]int{{0, n}}, [][2]int{{0, m}}
}

func backtrackMyers(trace [][]int, offset, n, m int) (aSpans, bSpans [][2]int) {
	x, y := n, m
	var dels, adds []int
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
		}
		if x == prevX {
			adds = append(adds, prevY)
		} else {
			dels = append(dels, prevX)
		}
		x, y = prevX, prevY
	}
	return coalesceIdx(dels), coalesceIdx(adds)
}

func coalesceIdx(idxs []int) [][2]int {
	if len(idxs) == 0 {
		return nil
	}
	slices.Sort(idxs)
	out := make([][2]int, 0, len(idxs))
	start, end := idxs[0], idxs[0]+1
	for _, i := range idxs[1:] {
		if i < end {
			continue
		}
		if i == end {
			end++
			continue
		}
		out = append(out, [2]int{start, end})
		start, end = i, i+1
	}
	out = append(out, [2]int{start, end})
	return out
}

func runeRangesToBytes(s string, spans [][2]int) [][2]int {
	if len(spans) == 0 {
		return nil
	}
	n := 0
	for _, sp := range spans {
		if sp[1] > n {
			n = sp[1]
		}
	}
	offs := make([]int, n+1)
	pos := 0
	for ri := 0; ri < n && pos < len(s); ri++ {
		_, size := utf8.DecodeRuneInString(s[pos:])
		pos += size
		offs[ri+1] = pos
	}
	out := make([][2]int, len(spans))
	for i, sp := range spans {
		out[i] = [2]int{offs[sp[0]], offs[sp[1]]}
	}
	return out
}

func indexInlines(inlines []diffInline) [][]diffInline {
	if len(inlines) == 0 {
		return nil
	}
	n := 0
	for _, in := range inlines {
		if in.line >= n {
			n = in.line + 1
		}
	}
	out := make([][]diffInline, n)
	for _, in := range inlines {
		out[in.line] = append(out[in.line], in)
	}
	return out
}

func inlinesAt(all [][]diffInline, line int) []diffInline {
	if line < 0 || line >= len(all) {
		return nil
	}
	return all[line]
}

func applyLinePaints(dst []diffPaint, viewStart int, kind byte, src int, byLine [][]diffInline) []diffPaint {
	if kind != '+' && kind != '-' {
		return dst
	}
	for _, in := range inlinesAt(byLine, src) {
		if in.end <= in.start || in.add != (kind == '+') {
			continue
		}
		dst = append(dst, diffPaint{start: viewStart + in.start, end: viewStart + in.end, add: in.add})
	}
	return dst
}

func paintDiffChars(styles *basicwidget.TextStyles, paints []diffPaint, dark bool) {
	if len(paints) == 0 {
		return
	}
	pal := diffPalette(dark)
	for _, p := range paints {
		if p.end <= p.start {
			continue
		}
		bg := pal.delWord
		if p.add {
			bg = pal.addWord
		}
		styles.SetBackgroundColorInRange(p.start, p.end, bg)
	}
}
