package gitcli

import (
	"reflect"
	"strings"
	"testing"
)

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
	rows := LayoutGraph(commits)
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
	rows := LayoutGraph(commits)
	if rows[0].Lane != 0 {
		t.Fatalf("M lane = %d", rows[0].Lane)
	}
	if rows[1].Lane != 1 {
		t.Fatalf("F lane = %d, want 1", rows[1].Lane)
	}
	if rows[2].Lane != 0 {
		t.Fatalf("P lane = %d", rows[2].Lane)
	}
	if !hasEdge(rows[0].Outgoing, 0, 0) || !hasEdge(rows[0].Outgoing, 0, 1) {
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

	// A newer commit stays above HEAD. Uncommitted sits on the row directly
	// above HEAD and sprouts into it, without taking that commit's edge.
	ahead := []Commit{
		{Hash: "f", Parents: []string{"h"}},
		{Hash: "h"},
	}
	rows = WithUncommitted(ahead, "h")
	if rows[0].Commit.Hash != "f" || rows[1].Commit.Hash != Uncommitted || rows[2].Commit.Hash != "h" {
		t.Fatalf("order = %s %s %s", rows[0].Commit.Hash, rows[1].Commit.Hash, rows[2].Commit.Hash)
	}
	if rows[1].Lane == rows[2].Lane {
		t.Fatalf("uncommitted stole HEAD's lane: %d", rows[1].Lane)
	}
	if !edgeGray(rows[1].Outgoing, rows[1].Lane, rows[2].Lane) || !edgeGray(rows[2].Incoming, rows[1].Lane, rows[2].Lane) {
		t.Fatalf("sprout outgoing=%#v incoming=%#v", rows[1].Outgoing, rows[2].Incoming)
	}
	if !hasEdge(rows[2].Incoming, rows[2].Lane, rows[2].Lane) {
		t.Fatalf("newer commit no longer reaches HEAD: %#v", rows[2].Incoming)
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
	rows := LayoutGraph(commits)
	if rows[0].Lane != 0 || rows[1].Lane != 1 || rows[2].Lane != 0 {
		t.Fatalf("lanes = %d %d %d", rows[0].Lane, rows[1].Lane, rows[2].Lane)
	}
}

func hasEdge(edges []Edge, from, to int) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}
