package gitcli

import (
	"fmt"
	"strconv"
	"strings"
)

// PatchForLines builds a patch that contains only the added and removed lines
// of diff whose byte ranges overlap [selStart, selEnd).
// unstage selects lines from a staged diff (git diff --cached) so the patch
// can be reverse-applied onto the index. Otherwise the patch stages lines
// from an unstaged diff onto the index.
func PatchForLines(diff string, selStart, selEnd int, unstage bool) (string, bool) {
	if selEnd <= selStart || strings.TrimSpace(diff) == "" {
		return "", false
	}
	sections := parseDiffSections(diff)
	var b strings.Builder
	any := false
	for _, sec := range sections {
		if sec.path == "" {
			continue
		}
		var hunks []string
		oldN, newN := 0, 0
		for _, h := range sec.hunks {
			body, o, n, ok := filterHunk(h, selStart, selEnd, unstage)
			if !ok {
				continue
			}
			oldStart, newStart := h.oldStart, h.newStart
			if oldStart == 0 && o > 0 {
				oldStart = newStart
			}
			if newStart == 0 && n > 0 {
				newStart = oldStart
			}
			hunks = append(hunks, formatHunk(oldStart, o, newStart, n, body))
			oldN += o
			newN += n
		}
		if len(hunks) == 0 {
			continue
		}
		b.WriteString(fileHeader(sec.path, oldN, newN))
		for _, h := range hunks {
			b.WriteString(h)
		}
		any = true
	}
	if !any {
		return "", false
	}
	return b.String(), true
}

type diffSection struct {
	path  string
	hunks []diffHunk
}

type diffHunk struct {
	oldStart int
	newStart int
	lines    []diffBody
}

type diffBody struct {
	kind    byte
	payload string
	raw     string
	start   int
	end     int
}

type rawDiffLine struct {
	text       string
	start, end int
}

func parseDiffSections(diff string) []diffSection {
	lines := splitDiffLines(diff)
	var out []diffSection
	var cur *diffSection
	var hunk *diffHunk
	flush := func() {
		if cur == nil || hunk == nil {
			return
		}
		cur.hunks = append(cur.hunks, *hunk)
		hunk = nil
	}
	for _, ln := range lines {
		if strings.HasPrefix(ln.text, "diff --git ") {
			flush()
			sec := diffSection{path: pathFromDiffGit(ln.text)}
			out = append(out, sec)
			cur = &out[len(out)-1]
			continue
		}
		if cur == nil {
			continue
		}
		if oldStart, newStart, ok := parseHunkAt(ln.text); ok {
			flush()
			hunk = &diffHunk{oldStart: oldStart, newStart: newStart}
			continue
		}
		if hunk == nil {
			continue
		}
		kind, payload := classifyDiffLine(ln.text)
		hunk.lines = append(hunk.lines, diffBody{
			kind:    kind,
			payload: payload,
			raw:     ln.text,
			start:   ln.start,
			end:     ln.end,
		})
	}
	flush()
	return out
}

func splitDiffLines(diff string) []rawDiffLine {
	var out []rawDiffLine
	for i := 0; i < len(diff); {
		j := strings.IndexByte(diff[i:], '\n')
		var end int
		var text string
		if j < 0 {
			end = len(diff)
			text = diff[i:]
		} else {
			end = i + j + 1
			text = diff[i : end-1]
		}
		out = append(out, rawDiffLine{text: text, start: i, end: end})
		if j < 0 {
			break
		}
		i = end
	}
	return out
}

func pathFromDiffGit(line string) string {
	rest := strings.TrimPrefix(line, "diff --git ")
	i := strings.LastIndex(rest, " b/")
	if i >= 0 && strings.HasPrefix(rest, "a/") {
		to := rest[i+len(" b/"):]
		if to != "" && to != "/dev/null" {
			return to
		}
		from := rest[len("a/"):i]
		if from != "" && from != "/dev/null" {
			return from
		}
	}
	return strings.TrimSpace(rest)
}

func parseHunkAt(line string) (oldStart, newStart int, ok bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return 0, 0, false
	}
	rest := line[len("@@ -"):]
	oldStart, rest, ok = parseDiffNum(rest)
	if !ok {
		return 0, 0, false
	}
	if strings.HasPrefix(rest, ",") {
		_, rest, ok = parseDiffNum(rest[1:])
		if !ok {
			return 0, 0, false
		}
	}
	rest = strings.TrimPrefix(rest, " ")
	if !strings.HasPrefix(rest, "+") {
		return 0, 0, false
	}
	newStart, _, ok = parseDiffNum(rest[1:])
	if !ok {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

func parseDiffNum(s string) (int, string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, s, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, s, false
	}
	return n, s[i:], true
}

func classifyDiffLine(text string) (byte, string) {
	if text == "" {
		return ' ', ""
	}
	switch text[0] {
	case '+', '-', ' ':
		return text[0], text[1:]
	case '\\':
		return '\\', text
	default:
		return ' ', text
	}
}

func filterHunk(h diffHunk, selStart, selEnd int, unstage bool) (string, int, int, bool) {
	var b strings.Builder
	oldN, newN := 0, 0
	changed := false
	prev := false
	for _, ln := range h.lines {
		if ln.kind == '\\' {
			if prev {
				b.WriteString(ln.raw)
				b.WriteByte('\n')
			}
			continue
		}
		selected := ln.start < selEnd && ln.end > selStart
		emit, include := filterDiffKind(ln.kind, selected, unstage)
		if !include {
			prev = false
			continue
		}
		if emit == '+' || emit == '-' {
			changed = true
		}
		b.WriteByte(emit)
		b.WriteString(ln.payload)
		b.WriteByte('\n')
		switch emit {
		case ' ':
			oldN++
			newN++
		case '-':
			oldN++
		case '+':
			newN++
		}
		prev = true
	}
	if !changed {
		return "", 0, 0, false
	}
	return b.String(), oldN, newN, true
}

func filterDiffKind(kind byte, selected, unstage bool) (byte, bool) {
	switch kind {
	case ' ':
		return ' ', true
	case '-':
		if unstage {
			if selected {
				return '-', true
			}
			return 0, false
		}
		if selected {
			return '-', true
		}
		return ' ', true
	case '+':
		if unstage {
			if selected {
				return '+', true
			}
			return ' ', true
		}
		if selected {
			return '+', true
		}
		return 0, false
	default:
		return 0, false
	}
}

func formatHunk(oldStart, oldCount, newStart, newCount int, body string) string {
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@\n%s", oldStart, oldCount, newStart, newCount, body)
}

func fileHeader(path string, oldN, newN int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	switch {
	case oldN == 0 && newN > 0:
		b.WriteString("new file mode 100644\n")
		b.WriteString("--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	case newN == 0 && oldN > 0:
		b.WriteString("deleted file mode 100644\n")
		fmt.Fprintf(&b, "--- a/%s\n", path)
		b.WriteString("+++ /dev/null\n")
	default:
		fmt.Fprintf(&b, "--- a/%s\n", path)
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	}
	return b.String()
}
