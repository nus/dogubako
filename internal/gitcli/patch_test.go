package gitcli

import (
	"strings"
	"testing"
)

const sampleDiff = "" +
	"diff --git a/a.txt b/a.txt\n" +
	"index 111..222 100644\n" +
	"--- a/a.txt\n" +
	"+++ b/a.txt\n" +
	"@@ -1,4 +1,5 @@\n" +
	" alpha\n" +
	"-beta\n" +
	"+BETA\n" +
	" gamma\n" +
	"+delta\n" +
	" omega\n"

func TestPatchForLinesStagesOneAddition(t *testing.T) {
	start, end := lineSpan(t, sampleDiff, "+delta\n")
	got, ok := PatchForLines(sampleDiff, start, end, false)
	if !ok {
		t.Fatal("expected a patch")
	}
	want := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,4 +1,5 @@\n" +
		" alpha\n" +
		" beta\n" +
		" gamma\n" +
		"+delta\n" +
		" omega\n"
	if got != want {
		t.Fatalf("patch:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchForLinesStagesReplacement(t *testing.T) {
	start, _ := lineSpan(t, sampleDiff, "-beta\n")
	_, end := lineSpan(t, sampleDiff, "+BETA\n")
	got, ok := PatchForLines(sampleDiff, start, end, false)
	if !ok {
		t.Fatal("expected a patch")
	}
	if !strings.Contains(got, "-beta\n") || !strings.Contains(got, "+BETA\n") {
		t.Fatalf("missing replacement:\n%s", got)
	}
	if strings.Contains(got, "+delta\n") {
		t.Fatalf("unstaged addition included:\n%s", got)
	}
}

func TestPatchForLinesIgnoresContext(t *testing.T) {
	start, end := lineSpan(t, sampleDiff, " alpha\n")
	if _, ok := PatchForLines(sampleDiff, start, end, false); ok {
		t.Fatal("context-only selection produced a patch")
	}
}

func TestPatchForLinesUnstagesOneAddition(t *testing.T) {
	start, end := lineSpan(t, sampleDiff, "+delta\n")
	got, ok := PatchForLines(sampleDiff, start, end, true)
	if !ok {
		t.Fatal("expected a patch")
	}
	want := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,4 +1,5 @@\n" +
		" alpha\n" +
		" BETA\n" +
		" gamma\n" +
		"+delta\n" +
		" omega\n"
	if got != want {
		t.Fatalf("patch:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchForLinesPartialNewFile(t *testing.T) {
	diff := "" +
		"diff --git a/c.txt b/c.txt\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/c.txt\n" +
		"@@ -0,0 +1,2 @@\n" +
		"+one\n" +
		"+two\n"
	start, end := lineSpan(t, diff, "+one\n")
	got, ok := PatchForLines(diff, start, end, false)
	if !ok {
		t.Fatal("expected a patch")
	}
	if !strings.Contains(got, "--- /dev/null\n") || !strings.Contains(got, "+one\n") {
		t.Fatalf("patch:\n%s", got)
	}
	if strings.Contains(got, "+two\n") {
		t.Fatalf("second line included:\n%s", got)
	}
}

func TestPatchForRangesSkipsGap(t *testing.T) {
	b0, b1 := lineSpan(t, sampleDiff, "-beta\n")
	d0, d1 := lineSpan(t, sampleDiff, "+delta\n")
	got, ok := PatchForRanges(sampleDiff, [][2]int{{b0, b1}, {d0, d1}}, false)
	if !ok {
		t.Fatal("expected a patch")
	}
	if !strings.Contains(got, "-beta\n") || !strings.Contains(got, "+delta\n") {
		t.Fatalf("patch:\n%s", got)
	}
	if strings.Contains(got, "+BETA\n") || strings.Contains(got, "-BETA\n") {
		t.Fatalf("gap line included:\n%s", got)
	}
}

func lineSpan(t *testing.T, diff, fragment string) (int, int) {
	t.Helper()
	i := strings.Index(diff, fragment)
	if i < 0 {
		t.Fatalf("missing %q", fragment)
	}
	return i, i + len(fragment)
}
