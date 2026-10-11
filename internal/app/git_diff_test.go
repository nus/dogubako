package app

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/guigui-gui/guigui/basicwidget"
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

func assertColor(t *testing.T, styles basicwidget.TextStyles, start, end int, want color.NRGBA) {
	t.Helper()
	got, uniform := styles.ColorInRange(start, end, color.NRGBA{})
	if !uniform || got != want {
		t.Fatalf("color [%d,%d) = %v uniform=%v, want %v", start, end, got, uniform, want)
	}
}
