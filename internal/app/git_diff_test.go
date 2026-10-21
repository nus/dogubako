package app

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestGitDiffFontMonospace(t *testing.T) {
	if gitDiffFont() == nil || gitMonoSource == nil {
		t.Fatal("monospace font missing")
	}
	face := &text.GoTextFace{Source: gitMonoSource, Size: 16}
	half := text.Advance("i", face)
	if half == 0 || half != text.Advance("m", face) {
		t.Fatal("latin is not monospace")
	}
	full := text.Advance("あ", face)
	if full == 0 || full != text.Advance("漢", face) {
		t.Fatal("japanese is not monospace")
	}
	if full != half*2 {
		t.Fatalf("fullwidth %v, halfwidth %v", full, half)
	}
}

func TestDiffPathStats(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,2 +1,3 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new\n" +
		"+extra\n" +
		"diff --git a/pic.jpg b/pic.jpg\n" +
		"Binary files a/pic.jpg and b/pic.jpg differ\n"
	got := diffPathStats(diff)
	if len(got) != 2 {
		t.Fatalf("files = %+v", got)
	}
	if got[0].Path != "a.txt" || got[0].Add != 2 || got[0].Del != 1 {
		t.Fatalf("a.txt = %+v", got[0])
	}
	if got[1].Path != "pic.jpg" || got[1].Add != 0 || got[1].Del != 0 {
		t.Fatalf("pic.jpg = %+v", got[1])
	}
}

func TestFileTreeLines(t *testing.T) {
	stats := []gitPathStat{
		{Path: "internal/app/git.go", Add: 12, Del: 3},
		{Path: "README.md", Add: 1, Del: 0},
		{Path: "internal/app/git_diff.go", Add: 4, Del: 1},
	}
	lines := fileTreeLines(stats, nil)
	var names []string
	for _, ln := range lines {
		names = append(names, ln.Name)
	}
	want := []string{"README.md", "internal", "app", "git.go", "git_diff.go"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v", names)
	}
	if lines[0].Add != 1 || lines[0].Del != 0 || lines[0].Folder {
		t.Fatalf("readme = %+v", lines[0])
	}
	if !lines[1].Folder || lines[1].Collapsed {
		t.Fatalf("internal = %+v", lines[1])
	}
	text, addStart, addEnd, delStart, delEnd := fileTreeLabel("git.go", 12, 3)
	if text != "git.go  (+12 -3)" {
		t.Fatalf("label = %q", text)
	}
	if text[addStart:addEnd] != "+12" || text[delStart:delEnd] != "-3" {
		t.Fatalf("spans %q %q", text[addStart:addEnd], text[delStart:delEnd])
	}
	folded := fileTreeLines(stats, map[string]bool{"internal": true})
	if len(folded) != 2 || folded[1].Name != "internal" || !folded[1].Collapsed {
		t.Fatalf("folded = %+v", folded)
	}
}

func TestBridgeEdgesPrefersColor(t *testing.T) {
	got := bridgeEdges([]gitcli.Edge{
		{From: 1, To: 0, Gray: true},
		{From: 0, To: 0},
		{From: 2, To: 2},
	})
	if len(got) != 2 || got[0].Gray || got[0].To != 0 || got[1].To != 2 {
		t.Fatalf("edges = %+v", got)
	}
}

func TestFileDiffText(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n" +
		"diff --git a/b.txt b/b.txt\n" +
		"--- a/b.txt\n" +
		"+++ b/b.txt\n" +
		"@@ -1 +1 @@\n" +
		"-gone\n" +
		"+kept\n"
	got := fileDiffText(diff, "b.txt")
	if !strings.Contains(got, "+kept") || strings.Contains(got, "a.txt") {
		t.Fatalf("b.txt = %q", got)
	}
	if fileDiffText(diff, "missing") != "" || fileDiffText(diff, "") != "" {
		t.Fatal("unknown path should be empty")
	}
}

func TestSplitUnifiedDiff(t *testing.T) {
	diff := `diff --git a/CHANGELOG b/CHANGELOG
deleted file mode 100644
index d3ff..0000
--- a/CHANGELOG
+++ /dev/null
@@ -1 +0,0 @@
-Initial changelog
diff --git a/binary.jpg b/binary.jpg
new file mode 100644
index 0000..d5c0
Binary files /dev/null and b/binary.jpg differ
diff --git a/test.txt b/test1.txt
rename from test.txt
rename to test1.txt
`
	files := splitUnifiedDiff(diff)
	if len(files) != 3 {
		t.Fatalf("files = %d", len(files))
	}
	if files[0].Name != "CHANGELOG" || files[0].Status != "D" || !strings.Contains(files[0].Text, "-Initial changelog") {
		t.Fatalf("deleted = %+v", files[0])
	}
	if files[1].Name != "binary.jpg" || files[1].Status != "A" || !strings.Contains(files[1].Text, "Binary files") {
		t.Fatalf("added = %+v", files[1])
	}
	if files[2].Name != "test1.txt" || files[2].Status != "R" {
		t.Fatalf("renamed = %+v", files[2])
	}
	if splitUnifiedDiff("") != nil {
		t.Fatal("empty diff should have no files")
	}
}

func TestStyleDiffColors(t *testing.T) {
	text := " ctx\n+add\n-del\n@@ hunk\n+++ b/file\n"
	styles := styleDiff(text, false)
	pal := diffPalette(false)
	assertColor(t, styles, len(" ctx\n"), len(" ctx\n+add\n"), color.NRGBA{})
	assertColor(t, styles, len(" ctx\n+add\n"), len(" ctx\n+add\n-del\n"), color.NRGBA{})
	assertColor(t, styles, len(" ctx\n+add\n-del\n"), len(" ctx\n+add\n-del\n@@ hunk\n"), pal.hunk)
	assertColor(t, styles, len(" ctx\n+add\n-del\n@@ hunk\n"), len(text), pal.meta)
	ctx, uniform := styles.ColorInRange(0, len(" ctx\n"), color.NRGBA{})
	if !uniform || ctx != (color.NRGBA{}) {
		t.Fatalf("context line color = %v uniform=%v", ctx, uniform)
	}
}

func TestCompactDiffMarkersStayOutOfBody(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new\n"
	marks, body, spans, kinds := compactDiff(diff)
	if strings.Join(splitLines(marks), ",") != " , , , , ,-,+" {
		t.Fatalf("marks %q", marks)
	}
	wantBody := []string{
		"diff --git a/a.txt b/a.txt",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,3 +1,3 @@",
		"ctx",
		"old",
		"new",
	}
	if strings.Join(splitLines(body), "\n") != strings.Join(wantBody, "\n") {
		t.Fatalf("body %q", body)
	}
	if string(kinds) != "mmm@ -+" {
		t.Fatalf("kinds %q", kinds)
	}
	old := lineRange(diff, "-old\n")
	sel := strings.Index(body, "old")
	got := overlappingSources(spans, sel, sel+len("old"))
	if len(got) != 1 || got[0] != old {
		t.Fatalf("old selection = %v, want %v", got, old)
	}
	if body[spans[5].viewStart:spans[5].viewEnd] != "old\n" {
		t.Fatalf("selectable old line %q", body[spans[5].viewStart:spans[5].viewEnd])
	}

	open := "@@ -1 +1 @@\n-a\n+b"
	marks, body, _, _ = compactDiff(open)
	if marks != " \n-\n+" || body != "@@ -1 +1 @@\na\nb" {
		t.Fatalf("open marks %q body %q", marks, body)
	}

	binary := "diff --git a/b.jpg b/b.jpg\nBinary files a/b.jpg and b/b.jpg differ\n"
	marks, body, spans, kinds = compactDiff(binary)
	if marks != "" || body != binary || spans != nil || kinds != nil {
		t.Fatalf("binary marks %q body %q spans %v kinds %q", marks, body, spans, kinds)
	}
	marks, body, spans, kinds = compactDiff("")
	if marks != "" || body != "" || spans != nil || kinds != nil {
		t.Fatalf("empty marks %q body %q", marks, body)
	}
}

func TestDiffGutter(t *testing.T) {
	const edit = "" +
		"diff --git a/a.txt b/a.txt\n" +
		"index 111..222 100644\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -10,4 +10,4 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new\n" +
		" end\n"
	want := strings.Join([]string{
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("10", "10"),
		gutterCols("11", ""),
		gutterCols("", "11"),
		gutterCols("12", "12"),
	}, "\n") + "\n"
	assertGutter(t, edit, want)

	const fresh = "@@ -0,0 +1,2 @@\n+one\n+two\n"
	assertGutter(t, fresh, strings.Join([]string{
		gutterCols("", ""),
		gutterCols("", "1"),
		gutterCols("", "2"),
	}, "\n")+"\n")

	const gone = "@@ -8,2 +0,0 @@\n-one\n-two\n"
	assertGutter(t, gone, strings.Join([]string{
		gutterCols("", ""),
		gutterCols("8", ""),
		gutterCols("9", ""),
	}, "\n")+"\n")

	const two = "" +
		"@@ -1,1 +1,1 @@\n" +
		"-a\n" +
		"+b\n" +
		"@@ -30,1 +40,1 @@\n" +
		" c\n"
	assertGutter(t, two, strings.Join([]string{
		gutterCols("", ""),
		gutterCols("1", ""),
		gutterCols("", "1"),
		gutterCols("", ""),
		gutterCols("30", "40"),
	}, "\n")+"\n")

	const marker = "@@ -1 +1 @@\n-a\n\\ No newline at end of file\n"
	assertGutter(t, marker, strings.Join([]string{
		gutterCols("", ""),
		gutterCols("1", ""),
		gutterCols("", ""),
	}, "\n")+"\n")

	const open = "@@ -1 +1 @@\n-a\n+b"
	assertGutter(t, open, gutterCols("", "")+"\n"+gutterCols("1", "")+"\n"+gutterCols("", "1"))

	const files = "" +
		"@@ -1 +1 @@\n" +
		"-a\n" +
		"+b\n" +
		"diff --git a/b b/b\n" +
		"--- a/b\n" +
		"+++ b/b\n" +
		"@@ -3 +3 @@\n" +
		"-c\n" +
		"+d\n"
	assertGutter(t, files, strings.Join([]string{
		gutterCols("", ""),
		gutterCols("1", ""),
		gutterCols("", "1"),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("", ""),
		gutterCols("3", ""),
		gutterCols("", "3"),
	}, "\n")+"\n")

	wide := "@@ -9999,1 +10000,1 @@\n-a\n+b\n"
	got := diffGutter(wide)
	wantWide := strings.Join([]string{
		fmt.Sprintf(" %5s %5s ", "", ""),
		fmt.Sprintf(" %5s %5s ", "9999", ""),
		fmt.Sprintf(" %5s %5s ", "", "10000"),
	}, "\n") + "\n"
	if got != wantWide {
		t.Fatalf("wide gutter:\n%s", got)
	}

	binary := "diff --git a/b.jpg b/b.jpg\nBinary files a/b.jpg and b/b.jpg differ\n"
	if diffGutter(binary) != "" {
		t.Fatal("binary diff should have no line numbers")
	}
	if diffGutter("") != "" {
		t.Fatal("empty diff should have no line numbers")
	}
}

func TestSplitDiffPairsChanges(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"@@ -10,3 +10,4 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new\n" +
		"+extra\n" +
		" end\n"
	doc := splitDiff(diff)
	left := splitLines(doc.leftBody)
	right := splitLines(doc.rightBody)
	if len(left) != len(right) {
		t.Fatalf("rows left %d right %d", len(left), len(right))
	}
	wantLeft := []string{"diff --git a/a.txt b/a.txt", "@@ -10,3 +10,4 @@", "ctx", "old", "", "end"}
	wantRight := []string{"diff --git a/a.txt b/a.txt", "@@ -10,3 +10,4 @@", "ctx", "new", "extra", "end"}
	if strings.Join(left, "\n") != strings.Join(wantLeft, "\n") || strings.Join(right, "\n") != strings.Join(wantRight, "\n") {
		t.Fatalf("left %q\nright %q", left, right)
	}
	if string(doc.leftKinds) != "m@ -\x00 " || string(doc.rightKinds) != "m@ ++ " {
		t.Fatalf("kinds left %q right %q", doc.leftKinds, doc.rightKinds)
	}
	leftNums := splitLines(doc.leftNum)
	rightNums := splitLines(doc.rightNum)
	if leftNums[3] != gutterNum("11") || rightNums[3] != gutterNum("11") || rightNums[4] != gutterNum("12") || leftNums[4] != gutterNum("") {
		t.Fatalf("nums left %q right %q", leftNums, rightNums)
	}

	src := lineRange(diff, "-old\n")
	if doc.leftSpans[3].srcStart != src[0] || doc.leftSpans[3].srcEnd != src[1] {
		t.Fatalf("old span = %+v want %v", doc.leftSpans[3], src)
	}
	if doc.rightSpans[4].srcStart != lineRange(diff, "+extra\n")[0] {
		t.Fatal("extra span does not point at the added line")
	}
	if doc.rightSpans[3].srcEnd <= doc.leftSpans[3].srcEnd {
		t.Fatal("paired addition should follow the deletion in the source")
	}
}

func TestSplitDiffNewAndDeleted(t *testing.T) {
	added := splitDiff("@@ -0,0 +1,1 @@\n+one\n")
	if splitLines(added.leftBody)[1] != "" || splitLines(added.rightBody)[1] != "one" {
		t.Fatalf("new file left %q right %q", added.leftBody, added.rightBody)
	}
	removed := splitDiff("@@ -2 +0,0 @@\n-gone\n")
	if splitLines(removed.leftBody)[1] != "gone" || splitLines(removed.rightBody)[1] != "" {
		t.Fatalf("deleted file left %q right %q", removed.leftBody, removed.rightBody)
	}
}

func splitLines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func gutterNum(n string) string {
	return fmt.Sprintf(" %*s ", diffNumMinWidth, n)
}

func lineRange(text, fragment string) [2]int {
	i := strings.Index(text, fragment)
	return [2]int{i, i + len(fragment)}
}

func gutterCols(old, new string) string {
	return fmt.Sprintf(" %*s %*s ", diffNumMinWidth, old, diffNumMinWidth, new)
}

func assertGutter(t *testing.T, diff, want string) {
	t.Helper()
	got := diffGutter(diff)
	if got != want {
		t.Fatalf("gutter:\n%q\nwant:\n%q", got, want)
	}
	if strings.Count(got, "\n") != strings.Count(diff, "\n") || strings.HasSuffix(got, "\n") != strings.HasSuffix(diff, "\n") {
		t.Fatalf("newline mismatch:\n%s", got)
	}
}

func TestStyleDiffGutter(t *testing.T) {
	diff := "+++ b/file\n@@ -1,2 +1,2 @@\n ctx\n-old\n+new\n"
	gutter := diffGutter(diff)
	styles := styleDiffGutter(gutter, diff, false)
	pal := diffPalette(false)
	got, uniform := styles.ColorInRange(0, len(gutter), color.NRGBA{})
	if !uniform || got != pal.meta {
		t.Fatalf("number color = %v uniform=%v", got, uniform)
	}
}

func TestExpandWrappedLines(t *testing.T) {
	cases := []struct {
		text   string
		counts []int
		want   string
	}{
		{"ab\ncd\n", []int{2, 1, 1}, "ab\n\ncd\n"},
		{"ab\ncd\n", []int{1, 2, 1}, "ab\ncd\n\n"},
		{"ab", []int{3}, "ab\n\n"},
		{"", []int{2}, "\n"},
		{"ab\n", []int{1, 1}, "ab\n"},
		{"ab\n", []int{2, 3}, "ab\n\n\n\n"},
		{"ab\n", []int{2}, "ab\n"},
	}
	for _, tc := range cases {
		got := expandWrappedLines(tc.text, tc.counts)
		if got != tc.want {
			t.Fatalf("expand(%q, %v) = %q, want %q", tc.text, tc.counts, got, tc.want)
		}
		if got == tc.text && len(diffLogicalLines(tc.text)) != len(tc.counts) {
			continue
		}
		if len(diffLogicalLines(got)) != sumWrapCounts(tc.counts) && len(diffLogicalLines(tc.text)) == len(tc.counts) {
			t.Fatalf("expand(%q, %v) lines %d, want %d", tc.text, tc.counts, len(diffLogicalLines(got)), sumWrapCounts(tc.counts))
		}
	}
}

func TestExpandKinds(t *testing.T) {
	if got := string(expandKinds([]byte("+-"), []int{2, 1, 1})); got != "++-" {
		t.Fatalf("with trailing = %q", got)
	}
	if got := string(expandKinds([]byte("+-"), []int{2, 1})); got != "++-" {
		t.Fatalf("without trailing = %q", got)
	}
	if expandKinds([]byte("+"), []int{1, 1, 1}) != nil {
		t.Fatal("mismatched counts should be dropped")
	}
	if expandKinds(nil, []int{2}) != nil {
		t.Fatal("no kinds")
	}
}

func TestExpandedGutterKeepsNumberColor(t *testing.T) {
	text := expandWrappedLines("  1 \n", []int{2, 1})
	kinds := expandKinds([]byte{' '}, []int{2, 1})
	styles := styleSplit(text, kinds, false, true)
	pal := diffPalette(false)
	got, uniform := styles.ColorInRange(0, len(text), color.NRGBA{})
	if !uniform || got != pal.meta {
		t.Fatalf("wrapped gutter color = %v uniform=%v", got, uniform)
	}
}

func TestCompactAndGutterShareLines(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,2 +1,2 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new\n"
	marks, body, _, kinds := compactDiff(diff)
	gutter := diffGutter(diff)
	if len(diffLogicalLines(body)) != len(diffLogicalLines(gutter)) || len(diffLogicalLines(body)) != len(diffLogicalLines(marks)) {
		t.Fatalf("lines body %d gutter %d marks %d", len(diffLogicalLines(body)), len(diffLogicalLines(gutter)), len(diffLogicalLines(marks)))
	}
	if len(kinds) != len(diffLogicalLines(body))-1 {
		t.Fatalf("kinds %d lines %d", len(kinds), len(diffLogicalLines(body)))
	}
}

func TestDiffLogicalLinesMatchWidget(t *testing.T) {
	samples := []string{"", "a", "a\n", "a\nb", "a\nb\n", "a\r\nb\r\n"}
	var widget basicwidget.Text
	widget.SetMultiline(true)
	widget.SetEditable(false)
	for _, s := range samples {
		widget.SetValue(s)
		lines := diffLogicalLines(s)
		if widget.LineCount() != len(lines) {
			t.Fatalf("%q widget %d logical %d", s, widget.LineCount(), len(lines))
		}
		for i, line := range lines {
			start := widget.LineStartInBytes(i)
			end := len(s)
			if i+1 < len(lines) {
				end = widget.LineStartInBytes(i + 1)
			}
			if s[start:end] != line {
				t.Fatalf("%q line %d %q != %q", s, i, s[start:end], line)
			}
		}
	}
}

func TestDiffWrapGlyphs(t *testing.T) {
	const size = float32(100)
	off := diffWrapStrokes(false, size)
	on := diffWrapStrokes(true, size)
	if len(off) == 0 || len(on) == 0 {
		t.Fatal("empty glyph")
	}
	if diffWrapStrokesSame(off, on) {
		t.Fatal("wrap and no-wrap glyphs match")
	}
	if !diffWrapStrokesInside(off, size) || !diffWrapStrokesInside(on, size) {
		t.Fatal("glyph is clipped")
	}
	if diffWrapVerticalCount(off) != 0 {
		t.Fatal("no-wrap glyph turns")
	}
	if diffWrapVerticalCount(on) == 0 {
		t.Fatal("wrap glyph does not turn down")
	}
	if !diffWrapArrowPoints(off, 1) {
		t.Fatal("no-wrap glyph does not point right")
	}
	if !diffWrapArrowPoints(on, -1) {
		t.Fatal("wrap glyph does not point back")
	}
}

func diffWrapStrokesSame(a, b [][4]float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func diffWrapStrokesInside(strokes [][4]float32, size float32) bool {
	for _, s := range strokes {
		for _, v := range s {
			if v < 0 || v > size {
				return false
			}
		}
	}
	return true
}

func diffWrapVerticalCount(strokes [][4]float32) int {
	n := 0
	for _, s := range strokes {
		if s[0] == s[2] && s[1] != s[3] {
			n++
		}
	}
	return n
}

func diffWrapArrowPoints(strokes [][4]float32, dir float32) bool {
	minX := strokes[0][0]
	maxX := strokes[0][0]
	for _, s := range strokes {
		for _, x := range []float32{s[0], s[2]} {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
	}
	for _, s := range strokes {
		if s[1] == s[3] || s[0] == s[2] {
			continue
		}
		left, right := s[0], s[2]
		if right < left {
			left, right = right, left
		}
		if dir > 0 && right == maxX {
			return true
		}
		if dir < 0 && left == minX {
			return true
		}
	}
	return false
}

func sumWrapCounts(counts []int) int {
	n := 0
	for _, c := range counts {
		if c < 1 {
			c = 1
		}
		n += c
	}
	return n
}

func TestDiffWheelAxes(t *testing.T) {
	// A mouse wheel is vertical. A short, wide diff uses it to reach the line end.
	dx, dy := diffWheelAxes(0, -12, 200, 400, 800, 100, false)
	if dx != -12 || dy != 0 {
		t.Fatalf("short wide diff: got %v,%v", dx, dy)
	}
	// A tall diff keeps the vertical wheel, and Shift sends it sideways.
	dx, dy = diffWheelAxes(0, -12, 200, 400, 800, 900, false)
	if dx != 0 || dy != -12 {
		t.Fatalf("tall diff: got %v,%v", dx, dy)
	}
	dx, dy = diffWheelAxes(0, -12, 200, 400, 800, 900, true)
	if dx != -12 || dy != 0 {
		t.Fatalf("shift: got %v,%v", dx, dy)
	}
	dx, dy = diffWheelAxes(0, -12, 200, 400, 100, 900, true)
	if dx != 0 || dy != -12 {
		t.Fatalf("shift with nothing past the pane: got %v,%v", dx, dy)
	}
}

func TestClampScrollOffset(t *testing.T) {
	if got := clampScrollOffset(-20, 100, 250); got != -20 {
		t.Fatalf("inside range: %v", got)
	}
	if got := clampScrollOffset(-999, 100, 250); got != -150 {
		t.Fatalf("past the end: %v", got)
	}
	if got := clampScrollOffset(10, 100, 250); got != 0 {
		t.Fatalf("before the start: %v", got)
	}
	if got := clampScrollOffset(-10, 100, 40); got != 0 {
		t.Fatalf("content that fits: %v", got)
	}
}

func assertColor(t *testing.T, styles basicwidget.TextStyles, start, end int, want color.NRGBA) {
	t.Helper()
	got, uniform := styles.ColorInRange(start, end, color.NRGBA{})
	if !uniform || got != want {
		t.Fatalf("color [%d,%d) = %v uniform=%v, want %v", start, end, got, uniform, want)
	}
}
