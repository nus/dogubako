package app

import (
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

func assertColor(t *testing.T, styles basicwidget.TextStyles, start, end int, want color.NRGBA) {
	t.Helper()
	got, uniform := styles.ColorInRange(start, end, color.NRGBA{})
	if !uniform || got != want {
		t.Fatalf("color [%d,%d) = %v uniform=%v, want %v", start, end, got, uniform, want)
	}
}
