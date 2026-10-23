package app

import (
	"fmt"
	"image"
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
	want := []string{"README.md", "internal/app", "git.go", "git_diff.go"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v", names)
	}
	if lines[0].Add != 1 || lines[0].Del != 0 || lines[0].Folder || lines[0].Depth != 0 {
		t.Fatalf("readme = %+v", lines[0])
	}
	if !lines[1].Folder || lines[1].Collapsed || lines[1].Path != "internal/app" || lines[1].Depth != 0 {
		t.Fatalf("folder = %+v", lines[1])
	}
	if lines[2].Depth != 1 || lines[2].Path != "internal/app/git.go" || lines[2].Add != 12 || lines[2].Del != 3 {
		t.Fatalf("git.go = %+v", lines[2])
	}
	text, addStart, addEnd, delStart, delEnd := fileTreeLabel("git.go", 12, 3)
	if text != "git.go  (+12 -3)" {
		t.Fatalf("label = %q", text)
	}
	if text[addStart:addEnd] != "+12" || text[delStart:delEnd] != "-3" {
		t.Fatalf("spans %q %q", text[addStart:addEnd], text[delStart:delEnd])
	}
	folded := fileTreeLines(stats, map[string]bool{"internal/app": true})
	if len(folded) != 2 || folded[1].Name != "internal/app" || folded[1].Path != "internal/app" || !folded[1].Collapsed {
		t.Fatalf("folded = %+v", folded)
	}
}

func TestFileTreeLinesCompactsDirsWithoutFiles(t *testing.T) {
	branches := fileTreeLines([]gitPathStat{
		{Path: "internal/gitcli/parse.go"},
		{Path: "internal/app/git.go"},
	}, nil)
	if got := treeLineNames(branches); strings.Join(got, ",") != "internal/app,git.go,internal/gitcli,parse.go" {
		t.Fatalf("branches = %v", got)
	}
	if branches[0].Depth != 0 || branches[1].Depth != 1 || branches[2].Depth != 0 || branches[3].Depth != 1 {
		t.Fatalf("branch depths = %+v", branches)
	}
	if branches[1].Path != "internal/app/git.go" || branches[3].Path != "internal/gitcli/parse.go" {
		t.Fatalf("branch paths = %+v", branches)
	}

	mixed := fileTreeLines([]gitPathStat{
		{Path: "internal/app/git.go", Add: 2},
		{Path: "internal/README.md", Add: 1},
	}, nil)
	if got := treeLineNames(mixed); strings.Join(got, ",") != "internal,README.md,app,git.go" {
		t.Fatalf("mixed = %v", got)
	}
	if mixed[0].Depth != 0 || mixed[1].Depth != 1 || mixed[2].Depth != 1 || mixed[3].Depth != 2 {
		t.Fatalf("mixed depths = %+v", mixed)
	}
	if mixed[1].Add != 1 || mixed[1].Path != "internal/README.md" || mixed[3].Path != "internal/app/git.go" {
		t.Fatalf("mixed files = %+v", mixed)
	}

	deep := fileTreeLines([]gitPathStat{
		{Path: "a/b/c/d/other.go"},
		{Path: "a/b/file.go"},
	}, nil)
	if got := treeLineNames(deep); strings.Join(got, ",") != "a/b,c/d,other.go,file.go" {
		t.Fatalf("deep = %v", got)
	}
	if deep[0].Path != "a/b" || deep[1].Path != "a/b/c/d" || deep[1].Depth != 1 || deep[2].Depth != 2 || deep[3].Path != "a/b/file.go" || deep[3].Depth != 1 {
		t.Fatalf("deep lines = %+v", deep)
	}

	root := fileTreeLines([]gitPathStat{{Path: "README.md", Add: 1}}, nil)
	if len(root) != 1 || root[0].Folder || root[0].Name != "README.md" || root[0].Add != 1 {
		t.Fatalf("root = %+v", root)
	}
}

func treeLineNames(lines []gitTreeLine) []string {
	names := make([]string, len(lines))
	for i, ln := range lines {
		names[i] = ln.Name
	}
	return names
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
	marks, body, spans, kinds, _ := compactDiff(diff)
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
	marks, body, _, _, _ = compactDiff(open)
	if marks != " \n-\n+" || body != "@@ -1 +1 @@\na\nb" {
		t.Fatalf("open marks %q body %q", marks, body)
	}

	binary := "diff --git a/b.jpg b/b.jpg\nBinary files a/b.jpg and b/b.jpg differ\n"
	marks, body, spans, kinds, _ = compactDiff(binary)
	if marks != "" || body != binary || spans != nil || kinds != nil {
		t.Fatalf("binary marks %q body %q spans %v kinds %q", marks, body, spans, kinds)
	}
	marks, body, spans, kinds, _ = compactDiff("")
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
	marks, body, _, kinds, _ := compactDiff(diff)
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

func TestDiffWholeGlyph(t *testing.T) {
	const size = float32(100)
	strokes := diffWholeStrokes(size)
	if len(strokes) < 8 {
		t.Fatalf("document glyph has %d strokes", len(strokes))
	}
	if !diffWrapStrokesInside(strokes, size) {
		t.Fatal("glyph is clipped")
	}
	if diffWrapStrokesSame(strokes, diffWrapStrokes(false, size)) || diffWrapStrokesSame(strokes, diffWrapStrokes(true, size)) {
		t.Fatal("whole-file glyph matches a wrap glyph")
	}
}

func TestWholeFileKeepsOriginalPatchLines(t *testing.T) {
	original := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -3,3 +3,4 @@\n" +
		" c\n" +
		"-d\n" +
		"+D\n" +
		"+E\n" +
		" e\n"
	full := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,5 +1,6 @@\n" +
		" a\n" +
		" b\n" +
		" c\n" +
		"-d\n" +
		"+D\n" +
		"+E\n" +
		" e\n"

	var v gitDiffView
	v.Set(original, false, false, false, true, full)
	if !strings.Contains(v.gutter, gutterCols("1", "1")) {
		t.Fatalf("gutter missing the first line:\n%s", v.gutter)
	}
	if _, _, ok := bodyLineRange(v.compactBody, "a"); !ok {
		t.Fatalf("compact body missing the prefix:\n%s", v.compactBody)
	}
	assertWholePatch(t, &v, original, "E", "+E\n", false)
	assertWholePatch(t, &v, original, "d", "-d\n", false)
	if start, end, ok := bodyLineRange(v.compactBody, "a"); !ok {
		t.Fatal("missing a")
	} else if _, ok := gitcli.PatchForRanges(original, overlappingSources(v.compactSpans, start, end), false); ok {
		t.Fatal("unchanged prefix was staged")
	}

	v.Set(original, false, true, false, true, full)
	left := splitLines(v.leftBody)
	right := splitLines(v.rightBody)
	if len(left) != len(right) {
		t.Fatalf("split rows left %d right %d", len(left), len(right))
	}
	if !slicesContain(left, "a") || !slicesContain(left, "d") || !slicesContain(right, "a") || !slicesContain(right, "E") {
		t.Fatalf("left %q\nright %q", left, right)
	}
	if start, end, ok := bodyLineRange(v.leftBody, "d"); !ok {
		t.Fatal("split left missing d")
	} else {
		got, ok := gitcli.PatchForRanges(original, overlappingSources(v.leftSpans, start, end), true)
		want, wantOK := gitcli.PatchForLines(original, lineRange(original, "-d\n")[0], lineRange(original, "-d\n")[1], true)
		if !ok || !wantOK || got != want {
			t.Fatalf("unstage got %q (%v)\nwant %q (%v)", got, ok, want, wantOK)
		}
	}

	v.Set(original, false, false, false, false, "")
	if _, _, ok := bodyLineRange(v.compactBody, "a"); ok {
		t.Fatalf("hunk view still shows the prefix:\n%s", v.compactBody)
	}
}

func TestWholeFileLeavesCompleteFiles(t *testing.T) {
	added := "" +
		"diff --git a/c.txt b/c.txt\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/c.txt\n" +
		"@@ -0,0 +1,2 @@\n" +
		"+one\n" +
		"+two\n"
	deleted := "" +
		"diff --git a/c.txt b/c.txt\n" +
		"deleted file mode 100644\n" +
		"--- a/c.txt\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-one\n"
	binary := "diff --git a/b.jpg b/b.jpg\nBinary files a/b.jpg and b/b.jpg differ\n"
	if !diffAlreadyWhole(added) || !diffAlreadyWhole(deleted) || diffAlreadyWhole("diff --git a/a.txt b/a.txt\n") {
		t.Fatal("whole-file detection")
	}
	if !diffIsBinary(binary) || diffIsBinary(added) {
		t.Fatal("binary detection")
	}
	var v gitDiffView
	v.Set(added, false, false, false, false, "")
	body, gutter := v.compactBody, v.gutter
	v.Set(added, false, false, false, true, added)
	if v.compactBody != body || v.gutter != gutter {
		t.Fatal("new file display changed")
	}
	v.Set(deleted, false, false, false, true, deleted)
	if _, _, ok := bodyLineRange(v.compactBody, "one"); !ok {
		t.Fatalf("deleted file body:\n%s", v.compactBody)
	}
	if _, ok := wholeFileSources(binary, binary); ok {
		t.Fatal("binary diff expanded")
	}
}

func assertWholePatch(t *testing.T, v *gitDiffView, original, line, fragment string, unstage bool) {
	t.Helper()
	start, end, ok := bodyLineRange(v.compactBody, line)
	if !ok {
		t.Fatalf("line %q not in body:\n%s", line, v.compactBody)
	}
	got, ok := gitcli.PatchForRanges(original, overlappingSources(v.compactSpans, start, end), unstage)
	wantStart, wantEnd := lineRange(original, fragment)[0], lineRange(original, fragment)[1]
	want, wantOK := gitcli.PatchForLines(original, wantStart, wantEnd, unstage)
	if !ok || !wantOK || got != want {
		t.Fatalf("line %q got %q (%v)\nwant %q (%v)", line, got, ok, want, wantOK)
	}
}

func bodyLineRange(body, content string) (start, end int, ok bool) {
	rest := body
	off := 0
	for rest != "" {
		i := strings.IndexByte(rest, '\n')
		line := rest
		next := len(rest)
		if i >= 0 {
			line = rest[:i]
			next = i + 1
		}
		if line == content {
			return off, off + next, true
		}
		if i < 0 {
			break
		}
		off += next
		rest = rest[next:]
	}
	return 0, 0, false
}

func slicesContain(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func TestDiffOverviewMarks(t *testing.T) {
	kinds := []byte{'m', '@', ' ', '-', '-', '+', ' '}
	marks := diffOverviewMarks(kinds, nil, nil)
	if len(marks) != 2 {
		t.Fatalf("marks = %+v", marks)
	}
	if !marks[0].del || marks[0].add || marks[0].start != 3.0/7 || marks[0].end != 5.0/7 {
		t.Fatalf("deletion = %+v", marks[0])
	}
	if !marks[1].add || marks[1].del || marks[1].start != 5.0/7 || marks[1].end != 6.0/7 {
		t.Fatalf("addition = %+v", marks[1])
	}

	gapped := diffOverviewMarks([]byte{'-', ' ', '-'}, nil, nil)
	if len(gapped) != 2 || gapped[0].end == gapped[1].start {
		t.Fatalf("context should split marks: %+v", gapped)
	}

	wrapped := diffOverviewMarks(kinds, nil, []int{1, 1, 1, 3, 1, 1, 1})
	if len(wrapped) != 2 || wrapped[0].start != 3.0/9 || wrapped[0].end != 7.0/9 || wrapped[1].start != 7.0/9 {
		t.Fatalf("wrapped = %+v", wrapped)
	}

	split := diffOverviewMarks([]byte{'-', ' ', 0}, []byte{'+', ' ', '+'}, nil)
	if len(split) != 2 || !split[0].add || !split[0].del || split[1].del || !split[1].add {
		t.Fatalf("split = %+v", split)
	}
	if split[0].end == split[1].start {
		t.Fatal("unchanged row should separate the marks")
	}
}

func TestOverviewScrollAndMarkRect(t *testing.T) {
	track := image.Rect(0, 0, 10, 100)
	r := diffOverviewMarkRect(track, 0, 0.001, 4)
	if r.Dy() < 4 || r.Min.Y < track.Min.Y || r.Max.Y > track.Max.Y {
		t.Fatalf("tiny mark = %v", r)
	}
	bottom := diffOverviewMarkRect(track, 0.99, 1, 4)
	if bottom.Max.Y != track.Max.Y || bottom.Dy() < 4 {
		t.Fatalf("bottom mark = %v", bottom)
	}
	if got := overviewScrollOffset(0, 0, 100, 20, 100); got != 0 {
		t.Fatalf("top scroll = %v", got)
	}
	if got := overviewScrollOffset(100, 0, 100, 20, 100); got != -80 {
		t.Fatalf("bottom scroll = %v", got)
	}
	if got := overviewScrollOffset(50, 0, 100, 20, 100); got != -40 {
		t.Fatalf("middle scroll = %v", got)
	}
	thumb := overviewThumbRect(track, -40, 20, 100, 4)
	if thumb.Min.Y != 40 || thumb.Dy() != 20 {
		t.Fatalf("thumb = %v", thumb)
	}
	if overviewThumbRect(track, 0, 100, 50, 4).Empty() == false {
		t.Fatal("a file that fits should not draw a thumb")
	}
}

func TestDiffOverviewLanes(t *testing.T) {
	markW, gap, thumbW := diffOverviewParts(1)
	track := image.Rect(10, 20, 10+markW+gap+thumbW, 120)
	marks, thumb := diffOverviewLanes(track, 1)
	if marks.Dx() != markW || thumb.Dx() != thumbW || marks.Max.X+gap != thumb.Min.X {
		t.Fatalf("lanes marks=%v thumb=%v", marks, thumb)
	}
	if marks.Min.Y != track.Min.Y || thumb.Max.Y != track.Max.Y {
		t.Fatal("lanes should span the track")
	}
	if thumb.Overlaps(marks) {
		t.Fatal("the scrollbar should sit beside the change marks")
	}
}

func TestWholeFileOverviewFollowsMode(t *testing.T) {
	original := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"@@ -2 +2 @@\n" +
		"-a\n" +
		"+b\n"
	full := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" x\n" +
		"-a\n" +
		"+b\n" +
		" y\n"
	var v gitDiffView
	v.Set(original, false, false, false, true, full)
	if !v.overviewActive() {
		t.Fatal("whole file should show the scrollbar")
	}
	marks := v.overviewMarks()
	if len(marks) != 2 || !marks[0].del || marks[0].add || !marks[1].add || marks[1].del {
		t.Fatalf("marks = %+v", marks)
	}
	if marks[0].start == 0 {
		t.Fatal("the unchanged prefix should sit above the deletion")
	}
	v.Set(original, false, true, false, true, full)
	split := v.overviewMarks()
	if len(split) != 1 || !split[0].add || !split[0].del {
		t.Fatalf("split mark = %+v", split)
	}
	v.Set(original, false, false, false, false, "")
	if v.overviewActive() {
		t.Fatal("hunk view should not show the scrollbar")
	}
}

func TestDiffHunkTargets(t *testing.T) {
	kinds := []byte{'m', '@', ' ', '-', '+', ' ', '@', ' ', '-'}
	hunks := diffHunkTargets(kinds, nil, false)
	if len(hunks) != 2 || hunks[0] != 1 || hunks[1] != 6 {
		t.Fatalf("hunk headers = %v", hunks)
	}
	changes := diffHunkTargets(kinds, nil, true)
	if len(changes) != 2 || changes[0] != 3 || changes[1] != 8 {
		t.Fatalf("change runs = %v", changes)
	}
	gapped := diffHunkTargets([]byte{'-', ' ', '-'}, nil, true)
	if len(gapped) != 2 || gapped[0] != 0 || gapped[1] != 2 {
		t.Fatalf("context should split runs: %v", gapped)
	}
	split := diffHunkTargets([]byte{'-', ' ', 0}, []byte{'+', ' ', '+'}, true)
	if len(split) != 2 || split[0] != 0 || split[1] != 2 {
		t.Fatalf("split runs = %v", split)
	}
	headers := diffHunkTargets([]byte{'@', '-', ' '}, []byte{'@', '+', ' '}, false)
	if len(headers) != 1 || headers[0] != 0 {
		t.Fatalf("split header = %v", headers)
	}
}

func TestDiffHunkNeighborAndLine(t *testing.T) {
	targets := []int{1, 6}
	if dest, ok := diffHunkNeighbor(targets, 0, true); !ok || dest != 1 {
		t.Fatalf("next from header = %d %v", dest, ok)
	}
	if dest, ok := diffHunkNeighbor(targets, 1, true); !ok || dest != 6 {
		t.Fatalf("next from first = %d %v", dest, ok)
	}
	if _, ok := diffHunkNeighbor(targets, 6, true); ok {
		t.Fatal("no hunk after the last")
	}
	if _, ok := diffHunkNeighbor(targets, 1, false); ok {
		t.Fatal("no hunk before the first")
	}
	if dest, ok := diffHunkNeighbor(targets, 4, false); !ok || dest != 1 {
		t.Fatalf("previous = %d %v", dest, ok)
	}
	counts := []int{1, 1, 3}
	if diffVisualRowsBefore(2, counts) != 2 {
		t.Fatalf("rows before wrap = %d", diffVisualRowsBefore(2, counts))
	}
	if got := diffLineAt(-20, counts, 10, 3); got != 2 {
		t.Fatalf("line at wrapped scroll = %d", got)
	}
	if got := diffLineAt(-19, counts, 10, 3); got != 1 {
		t.Fatalf("line before wrapped row = %d", got)
	}
	if got := diffLineAt(0, nil, 10, 4); got != 0 {
		t.Fatalf("top line = %d", got)
	}
}

func TestJumpHunk(t *testing.T) {
	text := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" a\n" +
		"-b\n" +
		"+c\n" +
		"@@ -20,3 +20,3 @@\n" +
		" d\n" +
		"-e\n" +
		"+f\n"
	var v gitDiffView
	v.Set(text, false, false, false, false, "")
	v.compact.viewH = 40
	v.compact.contentH = 400
	if v.jumpHunk(10, false) {
		t.Fatal("the header is already above the first hunk")
	}
	if !v.jumpHunk(10, true) {
		t.Fatal("next hunk")
	}
	if y, _, _ := v.scrollGeom(); y != -10 {
		t.Fatalf("first hunk scroll = %v", y)
	}
	if !v.jumpHunk(10, true) {
		t.Fatal("second hunk")
	}
	if y, _, _ := v.scrollGeom(); y != -50 {
		t.Fatalf("second hunk scroll = %v", y)
	}
	if v.jumpHunk(10, true) {
		t.Fatal("no hunk after the last")
	}
	if !v.jumpHunk(10, false) {
		t.Fatal("previous hunk")
	}
	if y, _, _ := v.scrollGeom(); y != -10 {
		t.Fatalf("previous scroll = %v", y)
	}

	full := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"@@ -1,5 +1,5 @@\n" +
		" x\n" +
		" y\n" +
		"-a\n" +
		"+b\n" +
		" z\n" +
		"-c\n" +
		"+d\n"
	v.Set(text, false, false, false, true, full)
	v.compact.viewH = 40
	v.compact.contentH = 400
	if !v.overviewActive() {
		t.Fatal("whole file")
	}
	if !v.jumpHunk(10, true) {
		t.Fatal("first change")
	}
	if y, _, _ := v.scrollGeom(); y != -40 {
		t.Fatalf("first change scroll = %v", y)
	}
	if !v.jumpHunk(10, true) {
		t.Fatal("second change")
	}
	if y, _, _ := v.scrollGeom(); y != -70 {
		t.Fatalf("second change scroll = %v", y)
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
