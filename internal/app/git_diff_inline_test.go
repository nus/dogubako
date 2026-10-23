package app

import (
	"image/color"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestDiffRuneSpans(t *testing.T) {
	cases := []struct {
		a, b  string
		wantA string
		wantB string
	}{
		{"aXbYc", "aZbWc", "X,Y", "Z,W"},
		{"hello world", "hello there", "wo,ld", "the,e"},
		{"ab", "ab", "", ""},
		{"abc", "xyz", "abc", "xyz"},
		{"aaa", "aa", "a", ""},
		{"aa", "aaa", "", "a"},
		{"あいう", "あえう", "い", "え"},
		{"a b", "ab", " ", ""},
		{"foo", "foo bar", "", " bar"},
		{"XhelloY", "hello", "X,Y", ""},
		{"", "", "", ""},
		{"", "ab", "", "ab"},
		{"ab", "", "ab", ""},
	}
	for _, tc := range cases {
		a, b := diffRuneSpans([]rune(tc.a), []rune(tc.b))
		if got := joinedSpans(tc.a, a); got != tc.wantA {
			t.Fatalf("%q to %q old %q, want %q", tc.a, tc.b, got, tc.wantA)
		}
		if got := joinedSpans(tc.b, b); got != tc.wantB {
			t.Fatalf("%q to %q new %q, want %q", tc.a, tc.b, got, tc.wantB)
		}
		validRuneSpans(t, len([]rune(tc.a)), a)
		validRuneSpans(t, len([]rune(tc.b)), b)
	}
}

func TestInlinePairing(t *testing.T) {
	// Equal counts pair from the top, even when a later line would match better.
	equal := blockPairs("-abcd\n-qqqq\n+ab\n+abcd\n")
	if strings.Join(equal, ";") != "abcd|ab;qqqq|abcd" {
		t.Fatalf("equal counts = %v", equal)
	}
	// Half of the shorter line is enough. One character of a four-character line is not.
	half := blockPairs("-abcd\n-wxyz\n+abQQ\n")
	if strings.Join(half, ";") != "abcd|abQQ" {
		t.Fatalf("half = %v", half)
	}
	under := blockPairs("-abcd\n-wxyz\n+aQQQ\n")
	if len(under) != 0 {
		t.Fatalf("under half = %v", under)
	}
	// The earliest alignment wins a tie, so the inserted twin stays unpaired.
	early := blockPairs("-foo\n-bar\n+fooX\n+fooZ\n+barY\n")
	if strings.Join(early, ";") != "foo|fooX;bar|barY" {
		t.Fatalf("earliest = %v", early)
	}
	// A line shoved between two similar lines stays a whole-line addition.
	mid := blockPairs("-keep one\n-keep two\n+keep one\n+inserted zzzzz\n+keep two\n")
	if strings.Join(mid, ";") != "keep one|keep one;keep two|keep two" {
		t.Fatalf("middle insert = %v", mid)
	}
	// A "no newline" marker between the sides stays in the same block.
	nlText := "@@ -1 +1 @@\n-foo bar\n\\ No newline at end of file\n+foo baz\n"
	nlLines := scanDiffLines(nlText)
	var nl []string
	for _, in := range diffInlines(nlLines) {
		nl = append(nl, nlLines[in.line].content[1:][in.start:in.end])
	}
	if strings.Join(nl, ",") != "r,z" {
		t.Fatalf("no newline = %v", nl)
	}
}

func TestInlineWordColors(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"index 111..222 100644\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1 +1 @@\n" +
		" ctx\n" +
		"-hello world\n" +
		"+hello there\n" +
		" gap1\n" +
		"-aXbYc\n" +
		"+aZbWc\n" +
		" gap2\n" +
		"-only old\n" +
		"+kept\n" +
		"+brand new\n" +
		" gap3\n" +
		"-same\n" +
		"+same\n" +
		" gap4\n" +
		"-abc\n" +
		"+xyz\n" +
		" gap5\n" +
		"-あいう\n" +
		"+あえう\n" +
		" gap6\n" +
		"-a b\n" +
		"+ab\n" +
		" gap7\n" +
		"+only added\n"
	light := diffPalette(false)
	dark := diffPalette(true)
	v := openSyntax(diff, false, false, false, false, "", "")
	styles := v.compactBodyStyle()
	body := v.compactBody

	assertFragBG(t, styles, body, "hello world", "hello ", light.delBg)
	assertFragBG(t, styles, body, "hello world", "wo", light.delWord)
	assertFragBG(t, styles, body, "hello world", "r", light.delBg)
	assertFragBG(t, styles, body, "hello world", "ld", light.delWord)
	assertFragBG(t, styles, body, "hello there", "hello ", light.addBg)
	assertFragBG(t, styles, body, "hello there", "the", light.addWord)
	assertFragBG(t, styles, body, "hello there", "r", light.addBg)
	startThere, _, okThere := bodyLineRange(body, "hello there")
	if !okThere {
		t.Fatal("missing hello there")
	}
	if got, set := styleBackground(styles, startThere+len("hello there")-1); !set || got != light.addWord {
		t.Fatalf("last e background = %v set=%v", got, set)
	}
	assertFragBG(t, styles, body, "aXbYc", "a", light.delBg)
	assertFragBG(t, styles, body, "aXbYc", "X", light.delWord)
	assertFragBG(t, styles, body, "aXbYc", "b", light.delBg)
	assertFragBG(t, styles, body, "aXbYc", "Y", light.delWord)
	assertFragBG(t, styles, body, "aXbYc", "c", light.delBg)
	assertFragBG(t, styles, body, "aZbWc", "Z", light.addWord)
	assertFragBG(t, styles, body, "aZbWc", "W", light.addWord)

	// Different lengths, and the lines share nothing: the whole line stays pale.
	assertLineBG(t, styles, body, "only old", light.delBg)
	assertLineBG(t, styles, body, "kept", light.addBg)
	assertLineBG(t, styles, body, "brand new", light.addBg)
	assertLineBG(t, styles, body, "only added", light.addBg)
	assertNoBG(t, styles, body, "ctx")

	// A paired rewrite of the whole line is dark. An identical pair is not.
	assertLineBG(t, styles, body, "abc", light.delWord)
	assertLineBG(t, styles, body, "xyz", light.addWord)
	assertLineBG(t, styles, body, "same", light.delBg)
	assertLineBG(t, styles, body, "same", light.addBg, 1)

	assertFragBG(t, styles, body, "あいう", "あ", light.delBg)
	assertFragBG(t, styles, body, "あいう", "い", light.delWord)
	assertFragBG(t, styles, body, "あいう", "う", light.delBg)
	assertFragBG(t, styles, body, "あえう", "え", light.addWord)
	assertFragBG(t, styles, body, "a b", "a", light.delBg)
	assertFragBG(t, styles, body, "a b", " ", light.delWord)
	assertFragBG(t, styles, body, "a b", "b", light.delBg)
	assertLineBG(t, styles, body, "ab", light.addBg)

	start, end, ok := bodyLineRange(body, "hello world")
	if !ok || end <= start+len("hello world") {
		t.Fatal("missing newline after hello world")
	}
	if got, set := styleBackground(styles, end-1); !set || got != light.delBg {
		t.Fatalf("newline background = %v set=%v", got, set)
	}

	assertNoBG(t, styles, body, "diff --git a/a.txt b/a.txt")
	assertNoBG(t, styles, body, "@@ -1 +1 @@")
	assertNoWord(t, styleSplit(v.compactMarks, v.compactKinds, false, false), len(v.compactMarks), light)
	assertNoWord(t, styleDiffGutter(v.gutter, v.text, false), len(v.gutter), light)

	dv := openSyntax(diff, true, false, false, false, "", "")
	assertFragBG(t, dv.compactBodyStyle(), dv.compactBody, "hello world", "wo", dark.delWord)
	assertFragBG(t, dv.compactBodyStyle(), dv.compactBody, "hello there", "the", dark.addWord)
	if dark.delWord == light.delWord || dark.addWord == light.addWord {
		t.Fatal("light and dark word colors match")
	}
}

func TestInlineLayoutsMatch(t *testing.T) {
	original := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -2,2 +2,2 @@\n" +
		"-hello world\n" +
		"+hello there\n"
	full := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" before\n" +
		"-hello world\n" +
		"+hello there\n" +
		" after\n"
	light := diffPalette(false)
	compact := openSyntax(original, false, false, false, false, "", "")
	wrapped := openSyntax(original, false, false, true, false, "", "")
	whole := openSyntax(original, false, false, false, true, full, "")
	split := openSyntax(original, false, true, false, false, "", "")
	wholeSplit := openSyntax(original, false, true, true, true, full, "")

	cs := compact.compactBodyStyle()
	if !backgroundsEqual(cs, wrapped.compactBodyStyle(), len(compact.compactBody)) {
		t.Fatal("wrapping changed the painted ranges")
	}
	assertFragBG(t, cs, compact.compactBody, "hello world", "wo", light.delWord)
	assertFragBG(t, cs, compact.compactBody, "hello world", "ld", light.delWord)
	assertFragBG(t, cs, compact.compactBody, "hello world", "r", light.delBg)
	assertFragBG(t, whole.compactBodyStyle(), whole.compactBody, "hello world", "wo", light.delWord)
	assertFragBG(t, whole.compactBodyStyle(), whole.compactBody, "hello world", "ld", light.delWord)
	assertFragBG(t, whole.compactBodyStyle(), whole.compactBody, "hello there", "the", light.addWord)
	assertNoBG(t, whole.compactBodyStyle(), whole.compactBody, "before")

	left := split.splitBodyStyle(true)
	right := split.splitBodyStyle(false)
	assertFragBG(t, left, split.leftBody, "hello world", "wo", light.delWord)
	assertFragBG(t, left, split.leftBody, "hello world", "ld", light.delWord)
	assertFragBG(t, left, split.leftBody, "hello world", "hello ", light.delBg)
	assertFragBG(t, right, split.rightBody, "hello there", "the", light.addWord)
	assertNoWord(t, styleSplit(split.leftNum, split.leftKinds, false, true), len(split.leftNum), light)
	assertNoWord(t, styleSplit(split.rightNum, split.rightKinds, false, true), len(split.rightNum), light)

	ws := wholeSplit.splitBodyStyle(true)
	wr := wholeSplit.splitBodyStyle(false)
	assertFragBG(t, ws, wholeSplit.leftBody, "hello world", "ld", light.delWord)
	assertFragBG(t, wr, wholeSplit.rightBody, "hello there", "the", light.addWord)
	assertNoBG(t, wr, wholeSplit.rightBody, "after")
}

func TestInlineUnequalSplit(t *testing.T) {
	// The split rows zip from the top. The color pairs by similarity,
	// so the matching characters are not on the same row.
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,2 +1,3 @@\n" +
		"-zzzz unique\n" +
		"-alpha\n" +
		"+alpha beta\n"
	light := diffPalette(false)
	compact := openSyntax(diff, false, false, false, false, "", "")
	cs := compact.compactBodyStyle()
	assertLineBG(t, cs, compact.compactBody, "zzzz unique", light.delBg)
	assertLineBG(t, cs, compact.compactBody, "alpha", light.delBg)
	assertFragBG(t, cs, compact.compactBody, "alpha beta", "alpha", light.addBg)
	assertFragBG(t, cs, compact.compactBody, "alpha beta", " beta", light.addWord)

	split := openSyntax(diff, false, true, false, false, "", "")
	left := split.splitBodyStyle(true)
	right := split.splitBodyStyle(false)
	assertLineBG(t, left, split.leftBody, "zzzz unique", light.delBg)
	assertLineBG(t, left, split.leftBody, "alpha", light.delBg)
	assertFragBG(t, right, split.rightBody, "alpha beta", " beta", light.addWord)
	assertNoWord(t, left, len(split.leftBody), light)
}

func TestInlineStageUnchanged(t *testing.T) {
	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,2 +1,2 @@\n" +
		"-hello world\n" +
		"+hello there\n"
	v := openSyntax(diff, false, false, false, false, "", "")
	_ = v.compactBodyStyle()
	marks, body, spans, kinds, _ := compactDiff(diff)
	if v.compactMarks != marks || v.compactBody != body || !reflect.DeepEqual(v.compactSpans, spans) || !reflect.DeepEqual(v.compactKinds, kinds) {
		t.Fatal("word paint changed the selectable diff")
	}
	start, end, ok := bodyLineRange(v.compactBody, "hello there")
	if !ok {
		t.Fatal("missing new line")
	}
	got, gotOK := gitcli.PatchForRanges(diff, overlappingSources(v.compactSpans, start, end), false)
	wantRange := lineRange(diff, "+hello there\n")
	want, wantOK := gitcli.PatchForLines(diff, wantRange[0], wantRange[1], false)
	if !gotOK || !wantOK || got != want {
		t.Fatalf("stage got %q (%v)\nwant %q (%v)", got, gotOK, want, wantOK)
	}

	split := openSyntax(diff, false, true, false, false, "", "")
	_ = split.splitBodyStyle(true)
	_ = split.splitBodyStyle(false)
	doc := splitDiff(diff)
	if !reflect.DeepEqual(split.leftSpans, doc.leftSpans) || !reflect.DeepEqual(split.rightSpans, doc.rightSpans) {
		t.Fatal("word paint changed split ranges")
	}
}

func blockPairs(hunkBody string) []string {
	var dels, adds []string
	for _, ln := range scanDiffLines("@@ -1 +1 @@\n" + hunkBody) {
		if !isChangeLine(ln.content) {
			continue
		}
		if ln.content[0] == '-' {
			dels = append(dels, ln.content[1:])
		} else {
			adds = append(adds, ln.content[1:])
		}
	}
	return formatPairs(dels, adds)
}

func formatPairs(dels, adds []string) []string {
	dr := make([][]rune, len(dels))
	ar := make([][]rune, len(adds))
	for i, s := range dels {
		dr[i] = []rune(s)
	}
	for i, s := range adds {
		ar[i] = []rune(s)
	}
	var out []string
	for _, p := range pairChangeLines(dr, ar) {
		out = append(out, dels[p[0]]+"|"+adds[p[1]])
	}
	return out
}

func joinedSpans(s string, spans [][2]int) string {
	rs := []rune(s)
	parts := make([]string, len(spans))
	for i, sp := range spans {
		parts[i] = string(rs[sp[0]:sp[1]])
	}
	return strings.Join(parts, ",")
}

func validRuneSpans(t *testing.T, n int, spans [][2]int) {
	t.Helper()
	prev := 0
	for _, sp := range spans {
		if sp[0] < prev || sp[1] <= sp[0] || sp[1] > n {
			t.Fatalf("span %v in %d runes", sp, n)
		}
		prev = sp[1]
	}
}

func assertFragBG(t *testing.T, styles basicwidget.TextStyles, body, line, frag string, want color.NRGBA) {
	t.Helper()
	start, _, ok := bodyLineRange(body, line)
	if !ok {
		t.Fatalf("missing line %q", line)
	}
	rel := strings.Index(line, frag)
	if rel < 0 {
		t.Fatalf("missing %q in %q", frag, line)
	}
	for i := start + rel; i < start+rel+len(frag); i++ {
		got, set := styleBackground(styles, i)
		if !set || got != want {
			t.Fatalf("%q in %q byte %d background = %v set=%v, want %v", frag, line, i-start, got, set, want)
		}
	}
}

func assertLineBG(t *testing.T, styles basicwidget.TextStyles, body, line string, want color.NRGBA, nth ...int) {
	t.Helper()
	start, end, ok := nthLineRange(body, line, nth...)
	if !ok {
		t.Fatalf("missing line %q", line)
	}
	contentEnd := end
	if contentEnd > start && body[contentEnd-1] == '\n' {
		contentEnd--
	}
	for i := start; i < contentEnd; i++ {
		got, set := styleBackground(styles, i)
		if !set || got != want {
			t.Fatalf("line %q byte %d background = %v set=%v, want %v", line, i-start, got, set, want)
		}
	}
	if contentEnd == end {
		return
	}
	nl := want
	if wash, ok := lineWash(want); ok {
		nl = wash
	}
	if got, set := styleBackground(styles, contentEnd); !set || got != nl {
		t.Fatalf("line %q newline background = %v set=%v, want %v", line, got, set, nl)
	}
}

func lineWash(word color.NRGBA) (color.NRGBA, bool) {
	for _, dark := range []bool{false, true} {
		pal := diffPalette(dark)
		switch word {
		case pal.delWord:
			return pal.delBg, true
		case pal.addWord:
			return pal.addBg, true
		}
	}
	return color.NRGBA{}, false
}

func assertNoBG(t *testing.T, styles basicwidget.TextStyles, body, line string) {
	t.Helper()
	start, end, ok := bodyLineRange(body, line)
	if !ok {
		t.Fatalf("missing line %q", line)
	}
	for i := start; i < end; i++ {
		if got, set := styleBackground(styles, i); set {
			t.Fatalf("line %q byte %d has background %v", line, i-start, got)
		}
	}
}

func assertNoWord(t *testing.T, styles basicwidget.TextStyles, n int, pal diffColors) {
	t.Helper()
	for i := range n {
		got, set := styleBackground(styles, i)
		if set && (got == pal.addWord || got == pal.delWord) {
			t.Fatalf("word color %v at byte %d", got, i)
		}
	}
}

func nthLineRange(body, content string, nth ...int) (start, end int, ok bool) {
	want := 0
	if len(nth) > 0 {
		want = nth[0]
	}
	rest := body
	off := 0
	seen := 0
	for rest != "" {
		i := strings.IndexByte(rest, '\n')
		line := rest
		next := len(rest)
		if i >= 0 {
			line = rest[:i]
			next = i + 1
		}
		if line == content {
			if seen == want {
				return off, off + next, true
			}
			seen++
		}
		if i < 0 {
			break
		}
		off += next
		rest = rest[next:]
	}
	return 0, 0, false
}

func backgroundsEqual(a, b basicwidget.TextStyles, n int) bool {
	for i := range n {
		ca, oa := styleBackground(a, i)
		cb, ob := styleBackground(b, i)
		if oa != ob || ca != cb {
			return false
		}
	}
	return true
}

func styleBackground(styles basicwidget.TextStyles, index int) (color.NRGBA, bool) {
	root := reflect.ValueOf(&styles).Elem().FieldByName("runs")
	list := root.FieldByName("runs")
	for i := range list.Len() {
		run := list.Index(i)
		start := int(run.FieldByName("Start").Int())
		end := int(run.FieldByName("End").Int())
		if index < start || index >= end {
			continue
		}
		bg := run.FieldByName("Style").FieldByName("backgroundColor")
		if !bg.FieldByName("set").Bool() {
			return color.NRGBA{}, false
		}
		raw := bg.FieldByName("value")
		c := reflect.NewAt(raw.Type(), unsafe.Pointer(raw.UnsafeAddr())).Elem().Interface().(color.Color)
		return nrgbaOf(c), true
	}
	return color.NRGBA{}, false
}
