package gitcli

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDetailMeta(t *testing.T) {
	line := strings.Join([]string{
		"abc123",
		"parent1 parent2",
		"Ada",
		"ada@example.com",
		"2026-10-11T09:17:39+09:00",
		"GitHub",
		"noreply@github.com",
		"fix graph",
	}, fieldSep)
	d, ok := parseDetailMeta(line + "\n")
	if !ok {
		t.Fatal("parse failed")
	}
	if d.Hash != "abc123" || d.Author != "Ada" || d.Email != "ada@example.com" {
		t.Fatalf("commit = %+v", d.Commit)
	}
	if d.Committer != "GitHub" || d.CommitterEmail != "noreply@github.com" {
		t.Fatalf("committer = %q <%s>", d.Committer, d.CommitterEmail)
	}
	if d.Subject != "fix graph" || len(d.Parents) != 2 || d.Parents[1] != "parent2" {
		t.Fatalf("subject %q parents %#v", d.Subject, d.Parents)
	}
	if d.When.IsZero() {
		t.Fatal("missing date")
	}
	if _, ok := parseDetailMeta(""); ok {
		t.Fatal("empty should fail")
	}
}

func TestParseLogLineAndDecorations(t *testing.T) {
	line := strings.Join([]string{
		"aaa111bbbbccccddddeeeeffff000011112222",
		"ppp111 qqq222",
		"Ada",
		"ada@example.com",
		"2026-01-02T03:04:05+09:00",
		"fix graph",
		"HEAD -> main, origin/main, tag: v1.0, origin/HEAD",
	}, fieldSep)
	c, ok := parseLogLine(line)
	if !ok {
		t.Fatal("parse failed")
	}
	if c.Short() != "aaa111b" {
		t.Fatalf("short = %q", c.Short())
	}
	if len(c.Parents) != 2 || c.Parents[0] != "ppp111" {
		t.Fatalf("parents = %#v", c.Parents)
	}
	if c.Subject != "fix graph" {
		t.Fatalf("subject = %q", c.Subject)
	}
	got := make([]string, 0, len(c.Decorations))
	for _, d := range c.Decorations {
		got = append(got, d.Kind+":"+d.Name)
	}
	want := []string{"head:main", "remote:origin/main", "tag:v1.0", "remote:origin/HEAD"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decorations = %#v", got)
	}
	hidden := map[string]bool{"origin": true}
	filtered := filterDecorations(c.Decorations, hidden)
	if len(filtered) != 2 || filtered[0].Name != "main" || filtered[1].Kind != "tag" {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestParseStatusBranchHeader(t *testing.T) {
	st := parseStatus("## main...origin/main [ahead 2, behind 1]\n M a.go\nA  b.go\n?? c.go\n")
	if st.Branch != "main" || st.Ahead != 2 || st.Behind != 1 {
		t.Fatalf("header = %+v", st)
	}
	if !st.Dirty() || !st.HasStaged() {
		t.Fatal("expected dirty with staged")
	}
	if len(st.Entries) != 3 {
		t.Fatalf("entries = %#v", st.Entries)
	}
	if !st.Entries[0].Unstaged() || st.Entries[0].Staged() {
		t.Fatalf("M unstaged: %+v", st.Entries[0])
	}
	if !st.Entries[1].Staged() {
		t.Fatalf("A staged: %+v", st.Entries[1])
	}
	if !st.Entries[2].Untracked() {
		t.Fatalf("untracked: %+v", st.Entries[2])
	}

	det := parseStatus("## HEAD (no branch)\n")
	if !det.Detached {
		t.Fatal("expected detached")
	}
}

func TestParseRefsSkipsRemoteHEAD(t *testing.T) {
	in := strings.Join([]string{
		"refs/heads/main\x00aaa\x00*",
		"refs/remotes/origin/main\x00aaa\x00 ",
		"refs/remotes/origin/HEAD\x00aaa\x00 ",
	}, "\n")
	refs := parseRefs(in)
	if len(refs) != 2 {
		t.Fatalf("refs = %#v", refs)
	}
	if !refs[0].Current || refs[0].Name != "main" {
		t.Fatalf("local = %+v", refs[0])
	}
	if refs[1].Remote != "origin" || refs[1].LocalName() != "main" {
		t.Fatalf("remote = %+v", refs[1])
	}
}

func TestLayoutGraphLinear(t *testing.T) {
	commits := []Commit{
		{Hash: "a", Parents: []string{"b"}},
		{Hash: "b", Parents: []string{"c"}},
		{Hash: "c"},
	}
	rows := LayoutGraph(commits, "")
	if len(rows) != 3 {
		t.Fatalf("len = %d", len(rows))
	}
	for i, r := range rows {
		if r.Lane != 0 {
			t.Fatalf("row %d lane = %d", i, r.Lane)
		}
	}
	if LaneCount(rows) != 1 {
		t.Fatalf("lanes = %d", LaneCount(rows))
	}
}

func TestLayoutGraphMerge(t *testing.T) {
	// Newest first: merge M of main P and feature F.
	commits := []Commit{
		{Hash: "M", Parents: []string{"P", "F"}},
		{Hash: "F", Parents: []string{"P"}},
		{Hash: "P"},
	}
	rows := LayoutGraph(commits, "")
	if rows[0].Lane != 0 {
		t.Fatalf("M lane = %d", rows[0].Lane)
	}
	if rows[1].Lane != 1 {
		t.Fatalf("F lane = %d, want 1", rows[1].Lane)
	}
	if rows[2].Lane != 0 {
		t.Fatalf("P lane = %d", rows[2].Lane)
	}
	if !hasEdge(rows[0].Outgoing, 0, 0) || !edgeLock(rows[0].Outgoing, 0, 1) {
		t.Fatalf("M outgoing = %#v", rows[0].Outgoing)
	}
	if !hasEdge(rows[1].Outgoing, 1, 0) || !hasEdge(rows[1].Outgoing, 0, 0) {
		t.Fatalf("F outgoing = %#v", rows[1].Outgoing)
	}
	if LaneCount(rows) != 2 {
		t.Fatalf("lanes = %d", LaneCount(rows))
	}
}

func TestWithUncommitted(t *testing.T) {
	commits := []Commit{
		{Hash: "a", Parents: []string{"b"}},
		{Hash: "b"},
	}
	rows := WithUncommitted(commits, "a")
	if rows[0].Commit.Hash != Uncommitted || rows[1].Commit.Hash != "a" {
		t.Fatalf("order = %s %s", rows[0].Commit.Hash, rows[1].Commit.Hash)
	}
	if !edgeGray(rows[0].Outgoing, 0, 0) || !edgeGray(rows[1].Incoming, 0, 0) {
		t.Fatalf("link outgoing=%#v incoming=%#v", rows[0].Outgoing, rows[1].Incoming)
	}
	for _, e := range rows[1].Outgoing {
		if e.Gray {
			t.Fatalf("HEAD outgoing is gray: %#v", rows[1].Outgoing)
		}
	}

	// Uncommitted is the first row. A newer commit on another branch sits
	// beside the gray line that runs down to HEAD.
	ahead := []Commit{
		{Hash: "f", Parents: []string{"h"}},
		{Hash: "h"},
	}
	rows = WithUncommitted(ahead, "h")
	if rows[0].Commit.Hash != Uncommitted || rows[1].Commit.Hash != "f" || rows[2].Commit.Hash != "h" {
		t.Fatalf("order = %s %s %s", rows[0].Commit.Hash, rows[1].Commit.Hash, rows[2].Commit.Hash)
	}
	if !grayReaches(rows, Uncommitted, "h") {
		t.Fatalf("gray line does not reach HEAD: %#v", rows)
	}
	if rowByHash(t, rows, "f").Lane == rowByHash(t, rows, "h").Lane {
		t.Fatalf("newer commit stayed on HEAD's lane %d", rowByHash(t, rows, "h").Lane)
	}
}

func edgeGray(edges []Edge, from, to int) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Gray {
			return true
		}
	}
	return false
}

func incomingGrayTo(row GraphRow, lane int) bool {
	for _, e := range row.Incoming {
		if e.To == lane && e.Gray {
			return true
		}
	}
	return false
}

func TestLayoutGraphTwoTips(t *testing.T) {
	commits := []Commit{
		{Hash: "E", Parents: []string{"C"}},
		{Hash: "D", Parents: []string{"C"}},
		{Hash: "C"},
	}
	rows := LayoutGraph(commits, "")
	if rows[0].Lane != 0 || rows[1].Lane != 1 || rows[2].Lane != 0 {
		t.Fatalf("lanes = %d %d %d", rows[0].Lane, rows[1].Lane, rows[2].Lane)
	}
}

func TestLayoutGraphBranches(t *testing.T) {
	// The first parent keeps the branch color. A second parent is a new
	// color that curves in.
	tips := []Commit{
		{Hash: "E", Parents: []string{"C"}},
		{Hash: "D", Parents: []string{"C"}},
		{Hash: "C"},
	}
	rows := LayoutGraph(tips, "D")
	expectLanes(t, rows, "E", 0, "D", 1, "C", 0)
	expectColors(t, rows, "E", 0, "D", 1, "C", 0)
	if !hasEdge(rows[0].Outgoing, 0, 0) || !hasEdge(rowByHash(t, rows, "D").Outgoing, 1, 0) {
		t.Fatalf("tips edges %#v / %#v", rows[0].Outgoing, rowByHash(t, rows, "D").Outgoing)
	}

	merged := []Commit{
		{Hash: "M", Parents: []string{"A", "B"}},
		{Hash: "B", Parents: []string{"P"}},
		{Hash: "A", Parents: []string{"P"}},
		{Hash: "P"},
	}
	rows = LayoutGraph(merged, "M")
	expectLanes(t, rows, "M", 0, "B", 1, "A", 0, "P", 0)
	expectColors(t, rows, "M", 0, "B", 1, "A", 0, "P", 0)
	if !hasEdge(rowByHash(t, rows, "A").Outgoing, 0, 0) {
		t.Fatalf("A outgoing = %#v", rowByHash(t, rows, "A").Outgoing)
	}
	if !hasEdge(rowByHash(t, rows, "B").Outgoing, 1, 1) {
		t.Fatalf("B outgoing = %#v", rowByHash(t, rows, "B").Outgoing)
	}

	interleaved := []Commit{
		{Hash: "F2", Parents: []string{"F1"}},
		{Hash: "M2", Parents: []string{"M1"}},
		{Hash: "F1", Parents: []string{"A"}},
		{Hash: "M1", Parents: []string{"A", "B"}},
		{Hash: "B", Parents: []string{"P"}},
		{Hash: "A", Parents: []string{"P"}},
		{Hash: "P"},
	}
	rows = LayoutGraph(interleaved, "M2")
	expectLanes(t, rows, "F2", 0, "M2", 1, "F1", 0, "M1", 1, "B", 2, "A", 0, "P", 0)
	expectColors(t, rows, "F2", 0, "M2", 1, "F1", 0, "M1", 1, "B", 2, "A", 0, "P", 0)
}

func TestLayoutGraphUnloadedParent(t *testing.T) {
	// A parent that is not loaded yet still occupies a column through the
	// rows below it, so the next page can continue that line.
	rows := LayoutGraph([]Commit{
		{Hash: "A", Parents: []string{"P"}},
		{Hash: "B", Parents: []string{"Q"}},
	}, "")
	expectLanes(t, rows, "A", 0, "B", 1)
	if !hasEdge(rows[0].Outgoing, 0, 0) {
		t.Fatalf("A's unloaded parent left no line: %#v", rows[0].Outgoing)
	}
	if len(rows[1].Outgoing) != 0 {
		t.Fatalf("line continued past the last row: %#v", rows[1].Outgoing)
	}

	loaded := LayoutGraph([]Commit{
		{Hash: "A", Parents: []string{"P"}},
		{Hash: "B", Parents: []string{"Q"}},
		{Hash: "P"},
		{Hash: "Q"},
	}, "")
	if !hasEdge(rowByHash(t, loaded, "A").Outgoing, 0, 0) || !hasEdge(rowByHash(t, loaded, "B").Outgoing, 0, 0) {
		t.Fatalf("loaded parent did not continue A's line: %#v", loaded)
	}
	if rowByHash(t, loaded, "P").Lane != 0 || rowByHash(t, loaded, "Q").Lane != 0 {
		t.Fatalf("lanes P=%d Q=%d", rowByHash(t, loaded, "P").Lane, rowByHash(t, loaded, "Q").Lane)
	}
	if !hasEdge(rowByHash(t, loaded, "P").Outgoing, 1, 0) {
		t.Fatalf("B's line did not curve into Q: %#v", rowByHash(t, loaded, "P").Outgoing)
	}
}

func expectLanes(t *testing.T, rows []GraphRow, hashLane ...any) {
	t.Helper()
	for i := 0; i < len(hashLane); i += 2 {
		hash := hashLane[i].(string)
		want := hashLane[i+1].(int)
		if got := rowByHash(t, rows, hash).Lane; got != want {
			t.Fatalf("commit %s lane = %d, want %d", hash, got, want)
		}
	}
}

func expectColors(t *testing.T, rows []GraphRow, hashColor ...any) {
	t.Helper()
	for i := 0; i < len(hashColor); i += 2 {
		hash := hashColor[i].(string)
		want := hashColor[i+1].(int)
		if got := rowByHash(t, rows, hash).Color; got != want {
			t.Fatalf("commit %s color = %d, want %d", hash, got, want)
		}
	}
}

func grayReaches(rows []GraphRow, from, to string) bool {
	start, target := -1, -1
	for i, r := range rows {
		if r.Commit.Hash == from {
			start = i
		}
		if r.Commit.Hash == to {
			target = i
		}
	}
	if start < 0 || target <= start {
		return false
	}
	lane := rows[start].Lane
	for i := start; i < target; i++ {
		next, ok := -1, false
		for _, e := range rows[i].Outgoing {
			if e.Gray && e.From == lane {
				next = e.To
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
		lane = next
	}
	return lane == rows[target].Lane
}

func rowByHash(t *testing.T, rows []GraphRow, hash string) GraphRow {
	t.Helper()
	for _, r := range rows {
		if r.Commit.Hash == hash {
			return r
		}
	}
	t.Fatalf("missing commit %s", hash)
	return GraphRow{}
}

func TestParseGitlinks(t *testing.T) {
	const (
		lib  = "0123456789abcdef0123456789abcdef01234567"
		util = "abcdef0123456789abcdef0123456789abcdef01"
		sha2 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	out := "" +
		"100644 " + lib + " 0\tREADME\x00" +
		"160000 " + util + " 0\tvendor/util\x00" +
		"160000 " + lib + " 1\tvendor/util\x00" +
		"160000 " + lib + " 0\tlibs/foo bar\x00" +
		"160000 " + sha2 + " 0\tnested/deep\x00"
	got := parseGitlinks(out)
	want := []Submodule{
		{Path: "libs/foo bar", Hash: lib},
		{Path: "nested/deep", Hash: sha2},
		{Path: "vendor/util", Hash: util},
	}
	if len(got) != len(want) {
		t.Fatalf("gitlinks = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gitlink %d = %#v, want %#v", i, got[i], want[i])
		}
	}
	if len(parseGitlinks("")) != 0 {
		t.Fatal("empty output should have no submodules")
	}
}

func edgeLock(edges []Edge, from, to int) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.LockTop {
			return true
		}
	}
	return false
}

func hasEdge(edges []Edge, from, to int) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}
