package app

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

func TestHistoryMatchHashes(t *testing.T) {
	tip := gitcli.Commit{
		Hash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Parents: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Subject: "Tip work",
		Author:  "Ada",
		When:    time.Date(1999, 1, 2, 0, 0, 0, 0, time.UTC),
		Decorations: []gitcli.Decoration{
			{Kind: "head", Name: "feature"},
			{Kind: "remote", Name: "origin/feature"},
			{Kind: "tag", Name: "v1.2"},
		},
	}
	side := gitcli.Commit{
		Hash:    "cccccccccccccccccccccccccccccccccccccccc",
		Parents: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Subject: "side",
		Decorations: []gitcli.Decoration{
			{Kind: "remote", Name: "origin/side"},
		},
	}
	base := gitcli.Commit{
		Hash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Subject: "base\nbody mentions feature",
		Author:  "Feature Fan",
		When:    time.Date(1999, 3, 4, 0, 0, 0, 0, time.UTC),
	}
	rows := gitcli.WithUncommitted([]gitcli.Commit{tip, side, base}, tip.Hash)

	if got := historyMatchHashes(rows, "   "); got != nil {
		t.Fatalf("blank = %v", got)
	}
	if got := historyMatchHashes(rows, ""); got != nil {
		t.Fatalf("empty = %v", got)
	}

	if got := historyMatchHashes(rows, "  FEATURE "); len(got) != 1 || got[0] != tip.Hash {
		t.Fatalf("branch = %+v", got)
	}
	if got := historyMatchHashes(rows, "origin"); len(got) != 2 || got[0] != tip.Hash || got[1] != side.Hash {
		t.Fatalf("remote name = %+v", got)
	}
	if got := historyMatchHashes(rows, "origin/side"); len(got) != 1 || got[0] != side.Hash {
		t.Fatalf("remote-only = %+v", got)
	}
	if got := historyMatchHashes(rows, "origin/feature"); len(got) != 0 {
		t.Fatalf("merged badge is not origin/feature: %+v", got)
	}
	if got := historyMatchHashes(rows, "V1"); len(got) != 1 || got[0] != tip.Hash {
		t.Fatalf("tag = %+v", got)
	}
	if got := historyMatchHashes(rows, "tip"); len(got) != 1 || got[0] != tip.Hash {
		t.Fatalf("title = %+v", got)
	}
	if got := historyMatchHashes(rows, "bbbbbbb"); len(got) != 1 || got[0] != tip.Hash {
		t.Fatalf("short id = %+v", got)
	}
	if got := historyMatchHashes(rows, strings.ToUpper(tip.Hash)); len(got) != 1 || got[0] != tip.Hash {
		t.Fatalf("full id = %+v", got)
	}
	if got := historyMatchHashes(rows, "fan"); len(got) != 0 {
		t.Fatalf("author = %+v", got)
	}
	if got := historyMatchHashes(rows, "1999"); len(got) != 0 {
		t.Fatalf("date = %+v", got)
	}
	if got := historyMatchHashes(rows, "body"); len(got) != 0 {
		t.Fatalf("body = %+v", got)
	}
	if got := historyMatchHashes(rows, "uncommitted"); len(got) != 0 {
		t.Fatalf("uncommitted = %+v", got)
	}

	tip.Subject = "red tip"
	base.Subject = "red base"
	both := historyMatchHashes([]gitcli.GraphRow{{Commit: tip}, {Commit: base}}, "RED")
	if len(both) != 2 || both[0] != tip.Hash || both[1] != base.Hash {
		t.Fatalf("order = %+v", both)
	}
}

func TestHistoryMatchLabel(t *testing.T) {
	matches := []string{"a", "b"}
	if got := historyMatchLabel(matches, "b"); got != "2/2" {
		t.Fatalf("current = %s", got)
	}
	if got := historyMatchLabel(matches, "z"); got != "?/2" {
		t.Fatalf("other = %s", got)
	}
	if got := historyMatchLabel(matches, ""); got != "?/2" {
		t.Fatalf("none = %s", got)
	}
	if got := historyMatchLabel(nil, "a"); got != "?/0" {
		t.Fatalf("empty = %s", got)
	}
}

func TestHistoryStepMatch(t *testing.T) {
	const (
		tip   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		side  = "cccccccccccccccccccccccccccccccccccccccc"
		base  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		older = "dddddddddddddddddddddddddddddddddddddddd"
	)
	rows := []gitcli.GraphRow{
		{Commit: gitcli.Commit{Hash: gitcli.Uncommitted, Subject: "uncommitted"}},
		{Commit: gitcli.Commit{Hash: tip, Subject: "feature"}},
		{Commit: gitcli.Commit{Hash: side, Subject: "side"}},
		{Commit: gitcli.Commit{Hash: base, Subject: "feature base"}},
		{Commit: gitcli.Commit{Hash: older, Subject: "older"}},
	}
	matches := historyMatchHashes(rows, "feature")
	if len(matches) != 2 || matches[0] != tip || matches[1] != base {
		t.Fatalf("matches = %+v", matches)
	}
	cases := []struct {
		selected string
		next     bool
		want     string
	}{
		{"", true, tip},
		{"", false, base},
		{tip, true, base},
		{tip, false, base},
		{base, true, tip},
		{base, false, tip},
		{side, true, base},
		{side, false, tip},
		{gitcli.Uncommitted, true, tip},
		{gitcli.Uncommitted, false, base},
		{older, true, tip},
		{older, false, base},
	}
	for _, tc := range cases {
		if got := historyStepMatch(matches, rows, tc.selected, tc.next); got != tc.want {
			t.Fatalf("from %s next=%v = %s, want %s", tc.selected, tc.next, got, tc.want)
		}
	}
	only := []string{tip}
	if got := historyStepMatch(only, rows, tip, true); got != tip {
		t.Fatalf("single = %s", got)
	}
	if got := historyStepMatch(nil, rows, tip, true); got != "" {
		t.Fatalf("none = %s", got)
	}
}

func TestStepHistMatchHighlightsWithoutOpening(t *testing.T) {
	const (
		tip  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		side = "cccccccccccccccccccccccccccccccccccccccc"
		base = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	rows := []gitcli.GraphRow{
		{Commit: gitcli.Commit{Hash: gitcli.Uncommitted}},
		{Commit: gitcli.Commit{Hash: tip, Subject: "feature"}},
		{Commit: gitcli.Commit{Hash: side, Subject: "side"}},
		{Commit: gitcli.Commit{Hash: base, Subject: "feature base"}},
	}
	var model GitModel
	model.ensure()
	model.graphRev = 3
	model.sel = side
	model.snap = gitcli.Snapshot{Graph: rows, HEAD: tip}

	var tool GitTool
	tool.histQuery = "feature"
	tool.openHash = "keep"
	tool.stepHistMatch(&model, true)
	if model.Selected() != base || tool.histJump != base || tool.openHash != "keep" {
		t.Fatalf("sel=%s jump=%s open=%s", model.Selected(), tool.histJump, tool.openHash)
	}
	tool.stepHistMatch(&model, false)
	if model.Selected() != tip || tool.histJump != tip || tool.openHash != "keep" {
		t.Fatalf("back sel=%s jump=%s open=%s", model.Selected(), tool.histJump, tool.openHash)
	}
	tool.histQuery = "missing"
	tool.histJump = ""
	tool.stepHistMatch(&model, true)
	if model.Selected() != tip || tool.histJump != "" || tool.openHash != "keep" {
		t.Fatalf("miss sel=%s jump=%s open=%s", model.Selected(), tool.histJump, tool.openHash)
	}
}

func TestSetHistQueryDoesNotJump(t *testing.T) {
	var tool GitTool
	tool.setHistQuery("  feature")
	if tool.histQuery != "  feature" || tool.histJump != "" {
		t.Fatalf("query=%q jump=%s", tool.histQuery, tool.histJump)
	}
}

func TestSyncHistQueryClearsJump(t *testing.T) {
	var model GitModel
	model.ensure()
	model.path = "a"
	var tool GitTool
	tool.histQuerySet = true
	tool.histQueryTab = model.ActiveTab()
	tool.histQueryPath = "a"
	tool.histQuery = "feature"
	tool.histJump = "bbbb"
	tool.histSearchOpen = true
	tool.syncHistQuery(&model)
	if tool.histQuery != "feature" || tool.histJump != "bbbb" || !tool.histSearchOpen {
		t.Fatalf("same repo query=%q jump=%s open=%v", tool.histQuery, tool.histJump, tool.histSearchOpen)
	}
	model.path = "b"
	tool.syncHistQuery(&model)
	if tool.histQuery != "" || tool.histJump != "" || tool.histSearchOpen {
		t.Fatalf("other repo query=%q jump=%s open=%v", tool.histQuery, tool.histJump, tool.histSearchOpen)
	}
}

func TestHistSearchOpenClose(t *testing.T) {
	var tool GitTool
	tool.emptyHome = true
	if tool.openHistSearch() {
		t.Fatal("recent repositories")
	}
	tool.emptyHome = false
	tool.commitView = true
	if tool.openHistSearch() {
		t.Fatal("changes")
	}
	tool.commitView = false
	if !tool.openHistSearch() || !tool.histSearchOpen || !tool.histSearchFocus {
		t.Fatal("open")
	}
	if tool.closeHistSearch(nil) {
		t.Fatal("closed while hidden")
	}
	tool.showHistSearch = true
	if !tool.closeHistSearch(nil) || tool.histSearchOpen || tool.histSearchFocus {
		t.Fatal("close")
	}
}

func TestSyncHistMatchesUsesCache(t *testing.T) {
	rows := []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: "bbbbbbbb", Subject: "feature"}}}
	var tool GitTool
	tool.histQuery = "feature"
	tool.syncHistMatches(rows, 1)
	if len(tool.histMatches) != 1 || tool.histMatches[0] != "bbbbbbbb" {
		t.Fatalf("matches = %+v", tool.histMatches)
	}
	tool.syncHistMatches(nil, 1)
	if len(tool.histMatches) != 1 {
		t.Fatalf("cache dropped: %+v", tool.histMatches)
	}
	tool.syncHistMatches(rows, 2)
	if len(tool.histMatches) != 1 {
		t.Fatalf("rev = %+v", tool.histMatches)
	}
	tool.histQuery = ""
	tool.syncHistMatches(rows, 2)
	if tool.histMatches != nil {
		t.Fatalf("cleared = %+v", tool.histMatches)
	}
}

func TestSyncCommitRowsKeepsGraphWhileSearching(t *testing.T) {
	commits := []gitcli.Commit{
		{Hash: "bbbbbbbb", Subject: "feature", Parents: []string{"aaaaaaaa"}},
		{Hash: "aaaaaaaa", Subject: "base"},
	}
	rows := gitcli.WithUncommitted(commits, "bbbbbbbb")
	var model GitModel
	model.ensure()
	model.path = "repo"
	model.repo = &gitcli.Repo{Dir: "repo"}
	model.graphRev = 1
	model.sel = "aaaaaaaa"
	model.snap = gitcli.Snapshot{
		HEAD:    "bbbbbbbb",
		Commits: commits,
		Graph:   rows,
		Lanes:   gitcli.LaneCount(rows),
	}

	var tool GitTool
	tool.histQuery = "feature"
	tool.openHash = "bbbbbbbb"
	if !tool.syncCommitRows(i18n.EN, &model, model.snap, nil) {
		t.Fatal("build")
	}
	if model.Selected() != "aaaaaaaa" || tool.openHash != "bbbbbbbb" {
		t.Fatalf("sel=%s open=%s", model.Selected(), tool.openHash)
	}
	if len(tool.commitItems) != 4 || tool.commitItems[0].Value != gitcli.Uncommitted || tool.commitItems[1].Value != "bbbbbbbb" || tool.commitItems[3].Value != "aaaaaaaa" {
		t.Fatalf("items = %+v", tool.commitItems)
	}
	if tool.syncCommitRows(i18n.EN, &model, model.snap, nil) {
		t.Fatal("same graph should keep the rows")
	}
}

func TestHistToolOrder(t *testing.T) {
	field := image.Rect(0, 0, 400, 32)
	clear := gitHistToolBounds(field, 24, 0)
	next := gitHistToolBounds(field, 24, 1)
	prev := gitHistToolBounds(field, 24, 2)
	if clear != gitFilterClearBounds(field, 24) {
		t.Fatalf("clear = %v", clear)
	}
	if !(prev.Max.X <= next.Min.X && next.Max.X <= clear.Min.X) {
		t.Fatalf("order prev %v next %v clear %v", prev, next, clear)
	}
}
