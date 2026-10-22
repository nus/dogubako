package app

import (
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/gitcli"
)

const goDiff = "" +
	"diff --git a/cmd/app/main.go b/cmd/app/main.go\n" +
	"index 111..222 100644\n" +
	"--- a/cmd/app/main.go\n" +
	"+++ b/cmd/app/main.go\n" +
	"@@ -1,6 +1,6 @@\n" +
	" package main\n" +
	" \n" +
	" /* open\n" +
	"-inside\n" +
	"+inside new\n" +
	" close */\n" +
	"-func Old() int { return 1 }\n" +
	"+func New() string { return \"x\" } // 注釈\n"

func TestSyntaxGoTokens(t *testing.T) {
	v := openSyntax(goDiff, false, false, false, false, "", "")
	styles := v.compactBodyStyle()
	body := v.compactBody
	pal := syntaxPalette(false)
	meta := diffPalette(false)

	if got := lineFragColor(t, styles, body, "package main", "package"); got != pal.keyword {
		t.Fatalf("package = %v", got)
	}
	if got := lineFragColor(t, styles, body, "package main", "main"); got != (color.NRGBA{}) {
		t.Fatalf("plain name = %v", got)
	}
	if got := lineFragColor(t, styles, body, "package main", " "); got != (color.NRGBA{}) {
		t.Fatalf("space = %v", got)
	}
	for _, line := range []string{"inside", "inside new", "/* open", "close */"} {
		if got := lineFragColor(t, styles, body, line, line); got != pal.comment {
			t.Fatalf("%s = %v, want comment", line, got)
		}
	}
	if got := lineFragColor(t, styles, body, "func Old() int { return 1 }", "func"); got != pal.keyword {
		t.Fatalf("func = %v", got)
	}
	if got := lineFragColor(t, styles, body, "func Old() int { return 1 }", "Old"); got != pal.ident {
		t.Fatalf("Old = %v", got)
	}
	if got := lineFragColor(t, styles, body, "func Old() int { return 1 }", "int"); got != pal.typ {
		t.Fatalf("int = %v", got)
	}
	if got := lineFragColor(t, styles, body, "func Old() int { return 1 }", "return"); got != pal.keyword {
		t.Fatalf("return = %v", got)
	}
	if got := lineFragColor(t, styles, body, "func Old() int { return 1 }", "1"); got != pal.number {
		t.Fatalf("number = %v", got)
	}
	newLine := `func New() string { return "x" } // 注釈`
	if got := lineFragColor(t, styles, body, newLine, "New"); got != pal.ident {
		t.Fatalf("New = %v", got)
	}
	if got := lineFragColor(t, styles, body, newLine, "string"); got != pal.typ {
		t.Fatalf("string = %v", got)
	}
	if got := lineFragColor(t, styles, body, newLine, `"x"`); got != pal.str {
		t.Fatalf("string literal = %v", got)
	}
	if got := lineFragColor(t, styles, body, newLine, "// 注釈"); got != pal.comment {
		t.Fatalf("comment = %v", got)
	}
	if got := lineFragColor(t, styles, body, newLine, "func"); got != pal.keyword {
		t.Fatalf("added func = %v", got)
	}

	assertColor(t, styles, 0, strings.Index(body, "\n")+1, meta.meta)
	hunk := "@@ -1,6 +1,6 @@"
	if got := lineFragColor(t, styles, body, hunk, hunk); got != meta.hunk {
		t.Fatalf("hunk = %v", got)
	}

	dark := openSyntax(goDiff, true, false, false, false, "", "")
	darkPal := syntaxPalette(true)
	if got := lineFragColor(t, dark.compactBodyStyle(), dark.compactBody, "package main", "package"); got != darkPal.keyword {
		t.Fatalf("dark package = %v", got)
	}
	if darkPal.keyword == pal.keyword {
		t.Fatal("light and dark keyword colors match")
	}
	v.Set(goDiff, true, false, false, false, "")
	if got := lineFragColor(t, v.compactBodyStyle(), v.compactBody, "package main", "package"); got != darkPal.keyword {
		t.Fatalf("theme switch = %v", got)
	}

	_, bodyOnly, spans, kinds := compactDiff(goDiff)
	if v.compactBody != bodyOnly || !slices.Equal(v.compactSpans, spans) || !slices.Equal(v.compactKinds, kinds) {
		t.Fatal("syntax changed the selectable diff")
	}
	markStyle := styleSplit(v.compactMarks, v.compactKinds, false, false)
	if got := lineFragColor(t, markStyle, v.compactMarks, "+", "+"); got != (color.NRGBA{}) {
		t.Fatalf("marker = %v", got)
	}
	gutter := styleDiffGutter(v.gutter, v.text, false)
	assertColor(t, gutter, 0, len(v.gutter), meta.meta)
}

func TestSyntaxLayoutsMatch(t *testing.T) {
	light := syntaxPalette(false)
	compact := openSyntax(goDiff, false, false, false, false, "", "")
	wrapped := openSyntax(goDiff, false, false, true, false, "", "")
	split := openSyntax(goDiff, false, true, false, false, "", "")
	cStyles := compact.compactBodyStyle()
	wStyles := wrapped.compactBodyStyle()
	if !slices.Equal(compact.syntax.compact, wrapped.syntax.compact) {
		t.Fatal("wrapping changed tokens")
	}
	pkg := lineFragColor(t, cStyles, compact.compactBody, "package main", "package")
	if pkg != light.keyword || lineFragColor(t, wStyles, wrapped.compactBody, "package main", "package") != pkg {
		t.Fatal("compact and wrapped package colors differ")
	}
	left := split.splitBodyStyle(true)
	right := split.splitBodyStyle(false)
	if lineFragColor(t, left, split.leftBody, "package main", "package") != pkg {
		t.Fatal("split left package color differs")
	}
	if lineFragColor(t, right, split.rightBody, "package main", "package") != pkg {
		t.Fatal("split right package color differs")
	}
	oldLine := "func Old() int { return 1 }"
	if lineFragColor(t, left, split.leftBody, oldLine, "int") != light.typ ||
		lineFragColor(t, cStyles, compact.compactBody, oldLine, "int") != light.typ {
		t.Fatal("split and compact type colors differ")
	}
	newLine := `func New() string { return "x" } // 注釈`
	if lineFragColor(t, right, split.rightBody, newLine, `"x"`) != light.str ||
		lineFragColor(t, cStyles, compact.compactBody, newLine, `"x"`) != light.str {
		t.Fatal("split and compact string colors differ")
	}
	if _, _, ok := bodyLineRange(split.leftBody, "func New() string { return \"x\" } // 注釈"); ok {
		t.Fatal("old side shows the new line")
	}

	original := "" +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -3 +3 @@\n" +
		"-var old = 1\n" +
		"+var neu = 2\n"
	full := "" +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,4 +1,4 @@\n" +
		" package main\n" +
		" \n" +
		"-var old = 1\n" +
		"+var neu = 2\n" +
		" func Keep() {}\n"
	hunk := openSyntax(original, false, false, false, false, "", "")
	whole := openSyntax(original, false, false, false, true, full, "")
	wholeSplit := openSyntax(original, false, true, true, true, full, "")
	if _, _, ok := bodyLineRange(hunk.compactBody, "package main"); ok {
		t.Fatal("hunk view shows the unchanged prefix")
	}
	hStyles := hunk.compactBodyStyle()
	wStyles = whole.compactBodyStyle()
	if lineFragColor(t, hStyles, hunk.compactBody, "var old = 1", "var") != light.keyword ||
		lineFragColor(t, wStyles, whole.compactBody, "var old = 1", "var") != light.keyword ||
		lineFragColor(t, wStyles, whole.compactBody, "package main", "package") != light.keyword ||
		lineFragColor(t, wStyles, whole.compactBody, "func Keep() {}", "Keep") != light.ident {
		t.Fatal("whole-file tokens")
	}
	ws := wholeSplit.splitBodyStyle(true)
	wr := wholeSplit.splitBodyStyle(false)
	if lineFragColor(t, ws, wholeSplit.leftBody, "func Keep() {}", "func") != light.keyword ||
		lineFragColor(t, wr, wholeSplit.rightBody, "func Keep() {}", "func") != light.keyword {
		t.Fatal("wrapped split whole-file func")
	}
	assertWholePatch(t, whole, original, "var neu = 2", "+var neu = 2\n", false)
	assertWholePatch(t, hunk, original, "var old = 1", "-var old = 1\n", true)
}

func TestSyntaxKnownNames(t *testing.T) {
	pal := syntaxPalette(false)

	makeDiff := "" +
		"diff --git a/Makefile b/Makefile\n" +
		"--- a/Makefile\n" +
		"+++ b/Makefile\n" +
		"@@ -1,2 +1,3 @@\n" +
		" build:\n" +
		" \tgo build\n" +
		"+# note\n"
	mv := openSyntax(makeDiff, false, false, false, false, "", "")
	ms := mv.compactBodyStyle()
	if got := lineFragColor(t, ms, mv.compactBody, "build:", "build"); got != pal.ident {
		t.Fatalf("make target = %v", got)
	}
	if got := lineFragColor(t, ms, mv.compactBody, "# note", "# note"); got != pal.comment {
		t.Fatalf("make comment = %v", got)
	}

	modDiff := "" +
		"diff --git a/go.mod b/go.mod\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/go.mod\n" +
		"@@ -0,0 +1,4 @@\n" +
		"+module example.com/mod\n" +
		"+\n" +
		"+go 1.22.0\n" +
		"+// indirect\n"
	gv := openSyntax(modDiff, false, false, false, false, "", "")
	gs := gv.compactBodyStyle()
	if got := lineFragColor(t, gs, gv.compactBody, "module example.com/mod", "module"); got != pal.keyword {
		t.Fatalf("module = %v", got)
	}
	if got := lineFragColor(t, gs, gv.compactBody, "module example.com/mod", "example.com/mod"); got != (color.NRGBA{}) {
		t.Fatalf("module path = %v", got)
	}
	if got := lineFragColor(t, gs, gv.compactBody, "go 1.22.0", "1.22.0"); got != pal.number {
		t.Fatalf("go version = %v", got)
	}
	if got := lineFragColor(t, gs, gv.compactBody, "// indirect", "// indirect"); got != pal.comment {
		t.Fatalf("go.mod comment = %v", got)
	}

	quoted := "" +
		"diff --git \"a/my file.go\" \"b/my file.go\"\n" +
		"--- \"a/my file.go\"\n" +
		"+++ \"b/my file.go\"\n" +
		"@@ -1 +1 @@\n" +
		"-func Old() {}\n" +
		"+func New() {}\n"
	qv := openSyntax(quoted, false, false, false, false, "", "")
	if got := lineFragColor(t, qv.compactBodyStyle(), qv.compactBody, "func New() {}", "func"); got != pal.keyword {
		t.Fatalf("quoted path = %v", got)
	}

	renamed := "" +
		"diff --git a/old.py b/new.go\n" +
		"--- a/old.py\n" +
		"+++ b/new.go\n" +
		"@@ -1 +1 @@\n" +
		"-def old():\n" +
		"+func New() {}\n"
	rv := openSyntax(renamed, false, false, false, false, "", "")
	rs := rv.compactBodyStyle()
	if got := lineFragColor(t, rs, rv.compactBody, "def old():", "def"); got != pal.keyword {
		t.Fatalf("python def = %v", got)
	}
	if got := lineFragColor(t, rs, rv.compactBody, "func New() {}", "func"); got != pal.keyword {
		t.Fatalf("renamed func = %v", got)
	}

	headerless := "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n"
	hv := openSyntax(headerless, false, false, false, false, "", "main.go")
	if got := lineFragColor(t, hv.compactBodyStyle(), hv.compactBody, "func Old() {}", "func"); got != pal.keyword {
		t.Fatalf("selected path = %v", got)
	}

	deleted := "" +
		"diff --git a/main.go b/main.go\n" +
		"deleted file mode 100644\n" +
		"--- a/main.go\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-func Old() int { return 1 }\n"
	dv := openSyntax(deleted, false, false, false, false, "", "")
	if got := lineFragColor(t, dv.compactBodyStyle(), dv.compactBody, "func Old() int { return 1 }", "int"); got != pal.typ {
		t.Fatalf("deleted file = %v", got)
	}
}

func TestSyntaxSkipsUnknownAndBinary(t *testing.T) {
	plain := "" +
		"diff --git a/notes.txt b/notes.txt\n" +
		"--- a/notes.txt\n" +
		"+++ b/notes.txt\n" +
		"@@ -1 +1 @@\n" +
		"-func Old()\n" +
		"+func New()\n"
	pv := openSyntax(plain, false, false, false, false, "", "")
	ps := pv.compactBodyStyle()
	if len(pv.syntax.compact) != 0 {
		t.Fatalf("plaintext spans = %d", len(pv.syntax.compact))
	}
	if got := lineFragColor(t, ps, pv.compactBody, "func New()", "func"); got != (color.NRGBA{}) {
		t.Fatalf("plaintext func = %v", got)
	}

	unknown := "" +
		"diff --git a/CHANGELOG b/CHANGELOG\n" +
		"--- a/CHANGELOG\n" +
		"+++ b/CHANGELOG\n" +
		"@@ -1 +1 @@\n" +
		"-func Old()\n" +
		"+func New()\n"
	uv := openSyntax(unknown, false, false, false, false, "", "")
	us := uv.compactBodyStyle()
	if len(uv.syntax.compact) != 0 {
		t.Fatal("unknown name was colored")
	}
	if got := lineFragColor(t, us, uv.compactBody, "func New()", "func"); got != (color.NRGBA{}) {
		t.Fatalf("unknown func = %v", got)
	}

	binary := "diff --git a/main.go b/main.go\nBinary files a/main.go and b/main.go differ\n"
	bv := openSyntax(binary, false, false, false, false, "", "main.go")
	bs := bv.compactBodyStyle()
	if len(bv.syntax.compact) != 0 {
		t.Fatal("binary diff was colored")
	}
	assertColor(t, bs, 0, len(bv.compactBody), diffPalette(false).meta)
}

func TestSyntaxContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal := syntaxPalette(dark)
		mode := ebiten.ColorModeLight
		if dark {
			mode = ebiten.ColorModeDark
		}
		page := nrgbaOf(basicwidgetdraw.BackgroundColor(mode))
		diff := diffPalette(dark)
		colors := []color.NRGBA{pal.keyword, pal.str, pal.comment, pal.number, pal.typ, pal.ident}
		backs := []color.NRGBA{page, diff.addBg, diff.delBg}
		for _, fg := range colors {
			for _, bg := range backs {
				if c := contrastRatio(fg, bg); c < 4.5 {
					t.Fatalf("dark=%v fg=%v bg=%v contrast %.2f", dark, fg, bg, c)
				}
			}
		}
		for i := range colors {
			for j := i + 1; j < len(colors); j++ {
				if colors[i] == colors[j] {
					t.Fatalf("dark=%v colors %d and %d match", dark, i, j)
				}
			}
		}
	}
}

func TestSyntaxStageUnchanged(t *testing.T) {
	v := openSyntax(goDiff, false, true, true, false, "", "")
	_ = v.splitBodyStyle(true)
	_ = v.splitBodyStyle(false)
	doc := splitDiff(goDiff)
	if !slices.Equal(v.leftSpans, doc.leftSpans) || !slices.Equal(v.rightSpans, doc.rightSpans) {
		t.Fatal("syntax changed split ranges")
	}
	start, end, ok := bodyLineRange(v.rightBody, `func New() string { return "x" } // 注釈`)
	if !ok {
		t.Fatal("missing new line")
	}
	got, ok := gitcli.PatchForRanges(goDiff, overlappingSources(v.rightSpans, start, end), false)
	wantStart, wantEnd := lineRange(goDiff, "+func New() string { return \"x\" } // 注釈\n")[0], lineRange(goDiff, "+func New() string { return \"x\" } // 注釈\n")[1]
	want, wantOK := gitcli.PatchForLines(goDiff, wantStart, wantEnd, false)
	if !ok || !wantOK || got != want {
		t.Fatalf("stage got %q (%v)\nwant %q (%v)", got, ok, want, wantOK)
	}
}

func openSyntax(text string, dark, split, wrap, whole bool, full, path string) *gitDiffView {
	v := &gitDiffView{filePath: path}
	v.Set(text, dark, split, wrap, whole, full)
	return v
}

func lineFragColor(t *testing.T, styles basicwidget.TextStyles, body, line, frag string) color.NRGBA {
	t.Helper()
	start, _, ok := bodyLineRange(body, line)
	if !ok {
		t.Fatalf("missing line %q in\n%s", line, body)
	}
	rel := strings.Index(line, frag)
	if rel < 0 {
		t.Fatalf("missing %q in %q", frag, line)
	}
	return uniformNRGBA(t, styles, start+rel, start+rel+len(frag))
}

func uniformNRGBA(t *testing.T, styles basicwidget.TextStyles, start, end int) color.NRGBA {
	t.Helper()
	got, uniform := styles.ColorInRange(start, end, color.NRGBA{})
	if !uniform {
		t.Fatalf("color [%d,%d) is not uniform: %v", start, end, got)
	}
	c, ok := got.(color.NRGBA)
	if !ok {
		t.Fatalf("color %T %v", got, got)
	}
	return c
}

func nrgbaOf(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func contrastRatio(fg, bg color.NRGBA) float64 {
	l1, l2 := relLum(fg), relLum(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relLum(c color.NRGBA) float64 {
	ch := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}
