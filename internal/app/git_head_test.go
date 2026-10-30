package app

import (
	"testing"

	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

func TestHeadJumpCopy(t *testing.T) {
	if got := i18n.T(i18n.JA, i18n.GitJumpHead); got != "チェックアウト中のコミットへ" {
		t.Fatalf("ja = %q", got)
	}
	if got := i18n.T(i18n.EN, i18n.GitJumpHead); got != "Jump to the checked-out commit" {
		t.Fatalf("en = %q", got)
	}
}

func TestHeadJumpGlyph(t *testing.T) {
	const size = float32(100)
	cx, cy, r := headJumpCircle(size)
	sw := headJumpStroke(size)
	if cx-r-sw/2 < 0 || cy-r-sw/2 < 0 || cx+r+sw/2 > size || cy+r+sw/2 > size {
		t.Fatalf("ring is clipped: c=(%v,%v) r=%v sw=%v", cx, cy, r, sw)
	}
	check := headJumpCheck(size)
	if len(check) != 2 {
		t.Fatalf("check strokes = %d", len(check))
	}
	if check[0][2] != check[1][0] || check[0][3] != check[1][1] {
		t.Fatal("check strokes do not meet")
	}
	if !(check[0][2] > check[0][0] && check[0][3] > check[0][1]) {
		t.Fatal("check does not descend")
	}
	if !(check[1][2] > check[1][0] && check[1][3] < check[1][1]) {
		t.Fatal("check does not rise")
	}
	limit := r * 0.72
	var minX, minY, maxX, maxY float32
	minX, minY = size, size
	for _, s := range check {
		for i := 0; i < 4; i += 2 {
			x, y := s[i], s[i+1]
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy > limit*limit {
				t.Fatalf("check point (%v,%v) meets the ring", x, y)
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX-minX >= r*2 || maxY-minY >= r*2 {
		t.Fatalf("check span = %v,%v diameter %v", maxX-minX, maxY-minY, r*2)
	}
}

func TestHeadJumpReady(t *testing.T) {
	const (
		repo = true
		busy = true
		load = true
		none = true
		seen = true
		more = true
	)
	cases := []struct {
		name                                 string
		hasRepo, busy, loading, empty, graph bool
		hasMore                              bool
		head                                 string
		want                                 bool
	}{
		{name: "loaded during fetch", hasRepo: repo, busy: busy, graph: seen, head: "abc", want: true},
		{name: "needs page during fetch", hasRepo: repo, busy: busy, hasMore: more, head: "abc", want: false},
		{name: "needs page", hasRepo: repo, hasMore: more, head: "abc", want: true},
		{name: "no more pages", hasRepo: repo, head: "abc", want: false},
		{name: "no repo", graph: seen, head: "abc", want: false},
		{name: "empty head", hasRepo: repo, graph: seen, want: false},
		{name: "uncommitted head", hasRepo: repo, graph: seen, head: gitcli.Uncommitted, want: false},
		{name: "first page loading", hasRepo: repo, loading: load, empty: none, hasMore: more, head: "abc", want: false},
	}
	for _, tc := range cases {
		got := headJumpReady(tc.hasRepo, tc.busy, tc.loading, tc.empty, tc.graph, tc.hasMore, tc.head)
		if got != tc.want {
			t.Errorf("%s = %v", tc.name, got)
		}
	}
}

func TestGraphHasCommitSkipsUncommitted(t *testing.T) {
	rows := []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: gitcli.Uncommitted}}, {Commit: gitcli.Commit{Hash: "abc"}}}
	if graphHasCommit(rows, gitcli.Uncommitted) || graphHasCommit(rows, "") || graphHasCommit(rows, "missing") {
		t.Fatal("matched a row that is not a commit")
	}
	if !graphHasCommit(rows, "abc") {
		t.Fatal("missed abc")
	}
}

func TestStepHeadJump(t *testing.T) {
	loaded := headJumpView{path: "/repo", jumpPath: "/repo", head: "aaa", inGraph: true, hasMore: true, limit: 400}
	next, sel, load := stepHeadJump("old", loaded)
	if next != "" || sel != "aaa" || load {
		t.Fatalf("loaded next=%q sel=%q load=%v", next, sel, load)
	}
	page := loaded
	page.inGraph = false
	next, sel, load = stepHeadJump("old", page)
	if next != "aaa" || sel != "" || !load {
		t.Fatalf("page next=%q sel=%q load=%v", next, sel, load)
	}
	follow := page
	follow.head = "bbb"
	follow.busy = true
	next, sel, load = stepHeadJump("aaa", follow)
	if next != "bbb" || sel != "" || load {
		t.Fatalf("follow next=%q sel=%q load=%v", next, sel, load)
	}
	waiting := follow
	waiting.busy = false
	waiting.loading = true
	next, sel, load = stepHeadJump("aaa", waiting)
	if next != "bbb" || sel != "" || load {
		t.Fatalf("loading next=%q sel=%q load=%v", next, sel, load)
	}
	stalled := page
	stalled.asked = 400
	next, sel, load = stepHeadJump("aaa", stalled)
	if next != "" || sel != "" || load {
		t.Fatalf("stalled next=%q sel=%q load=%v", next, sel, load)
	}
	end := page
	end.hasMore = false
	next, sel, load = stepHeadJump("aaa", end)
	if next != "" || sel != "" || load {
		t.Fatalf("end next=%q sel=%q load=%v", next, sel, load)
	}
	other := page
	other.tab = 1
	next, sel, load = stepHeadJump("aaa", other)
	if next != "" || sel != "" || load {
		t.Fatalf("tab next=%q sel=%q load=%v", next, sel, load)
	}
	empty := page
	empty.head = ""
	next, sel, load = stepHeadJump("aaa", empty)
	if next != "" || sel != "" || load {
		t.Fatalf("empty head next=%q sel=%q load=%v", next, sel, load)
	}
	n, s, l := stepHeadJump("", loaded)
	if n != "" || s != "" || l {
		t.Fatal("idle jump")
	}
}

func TestSyncHeadJumpSelectsCheckedOutCommit(t *testing.T) {
	s := &gitSession{
		path:   "/repo",
		repo:   &gitcli.Repo{Dir: tTemp(t)},
		sel:    gitcli.Uncommitted,
		detail: gitcli.CommitDetail{Commit: gitcli.Commit{Hash: "keep"}},
		snap: gitcli.Snapshot{
			HEAD: "bbb",
			Graph: []gitcli.GraphRow{
				{Commit: gitcli.Commit{Hash: gitcli.Uncommitted}},
				{Commit: gitcli.Commit{Hash: "bbb"}},
			},
		},
	}
	s.SetStatus(i18n.StatusGitFetchOk)
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.openHash = "keep"
	tool.headJump = "bbb"
	tool.headJumpPath = "/repo"
	list := commitList(gitcli.Uncommitted, "bbb")
	if got := tool.syncHeadJump(m, list); got != "bbb" {
		t.Fatalf("scroll = %q", got)
	}
	if s.Selected() != "bbb" {
		t.Fatalf("selected = %q", s.Selected())
	}
	if s.Detail().Hash != "keep" || s.pendingDetail != nil {
		t.Fatalf("detail = %s pending=%v", s.Detail().Hash, s.pendingDetail != nil)
	}
	if tool.openHash != "keep" {
		t.Fatalf("open = %q", tool.openHash)
	}
	if tool.headJump != "" {
		t.Fatal("jump was not consumed")
	}
	if s.StatusText(i18n.EN) != "Fetched" {
		t.Fatalf("status = %q", s.StatusText(i18n.EN))
	}
	if got := tool.syncHeadJump(m, list); got != "" || s.Selected() != "bbb" {
		t.Fatalf("second scroll = %q sel = %q", got, s.Selected())
	}
	tool.headJump = "bbb"
	tool.headJumpPath = "/repo"
	if got := tool.syncHeadJump(m, list); got != "bbb" {
		t.Fatalf("again = %q", got)
	}
}

func TestSyncHeadJumpKeepsRequestUntilListed(t *testing.T) {
	s := &gitSession{
		path: "/repo",
		repo: &gitcli.Repo{Dir: tTemp(t)},
		sel:  "other",
		snap: gitcli.Snapshot{
			HEAD: "bbb",
			Graph: []gitcli.GraphRow{
				{Commit: gitcli.Commit{Hash: "other"}},
				{Commit: gitcli.Commit{Hash: "bbb"}},
			},
		},
		detail: gitcli.CommitDetail{Commit: gitcli.Commit{Hash: "keep"}},
	}
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.openHash = "keep"
	tool.headJump = "bbb"
	tool.headJumpPath = "/repo"
	if got := tool.syncHeadJump(m, commitList("other")); got != "" || tool.headJump != "bbb" || s.Selected() != "other" {
		t.Fatalf("scroll=%q jump=%q sel=%q", got, tool.headJump, s.Selected())
	}
	if s.pendingDetail != nil || tool.openHash != "keep" {
		t.Fatal("dropped request opened the commit")
	}
	if got := tool.syncHeadJump(m, commitList("other", "bbb")); got != "bbb" || s.Selected() != "bbb" || tool.headJump != "" {
		t.Fatalf("scroll=%q sel=%q jump=%q", got, s.Selected(), tool.headJump)
	}
	if s.pendingDetail != nil || s.Detail().Hash != "keep" || tool.openHash != "keep" {
		t.Fatal("listing the row opened the detail")
	}
}

func TestSyncHeadJumpWaitsForTheRow(t *testing.T) {
	s := &gitSession{
		path: "/repo",
		sel:  "visible",
		snap: gitcli.Snapshot{
			HEAD:  "later",
			Graph: []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: "visible"}}},
		},
		histMore:  true,
		histLimit: 300,
	}
	s.SetStatus(i18n.StatusGitOpened, "/repo", 1)
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.openHash = "visible"
	tool.headJump = "later"
	tool.headJumpPath = "/repo"
	list := commitList("visible")
	if got := tool.syncHeadJump(m, list); got != "" || tool.headJump != "later" || tool.headJumpAsked != 300 {
		t.Fatalf("scroll=%q jump=%q asked=%d", got, tool.headJump, tool.headJumpAsked)
	}
	if s.Selected() != "visible" || tool.openHash != "visible" {
		t.Fatalf("sel=%q open=%q", s.Selected(), tool.openHash)
	}
	before := s.StatusText(i18n.EN)
	s.histLimit = 400
	s.snap.HEAD = "moved"
	s.snap.Graph = append(s.snap.Graph, gitcli.GraphRow{Commit: gitcli.Commit{Hash: "moved"}})
	list = commitList("visible", "moved")
	if got := tool.syncHeadJump(m, list); got != "moved" {
		t.Fatalf("scroll = %q", got)
	}
	if s.Selected() != "moved" || tool.openHash != "visible" || tool.headJump != "" {
		t.Fatalf("sel=%q open=%q jump=%q", s.Selected(), tool.openHash, tool.headJump)
	}
	if s.StatusText(i18n.EN) != before {
		t.Fatalf("status = %q", s.StatusText(i18n.EN))
	}
}

func TestSyncHeadJumpGivesUpAtTheEnd(t *testing.T) {
	s := &gitSession{
		path: "/repo",
		sel:  "visible",
		snap: gitcli.Snapshot{
			HEAD:  "gone",
			Graph: []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: "visible"}}},
		},
	}
	s.SetStatus(i18n.StatusGitFetchOk)
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.headJump = "gone"
	tool.headJumpPath = "/repo"
	if got := tool.syncHeadJump(m, commitList("visible")); got != "" || tool.headJump != "" {
		t.Fatalf("scroll=%q jump=%q", got, tool.headJump)
	}
	if s.Selected() != "visible" || s.StatusText(i18n.EN) != "Fetched" {
		t.Fatalf("sel=%q status=%q", s.Selected(), s.StatusText(i18n.EN))
	}
}

func TestSyncHeadJumpDoesNotRetryAStalledPage(t *testing.T) {
	s := &gitSession{
		path: "/repo",
		sel:  "visible",
		snap: gitcli.Snapshot{
			HEAD:  "later",
			Graph: []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: "visible"}}},
		},
		histMore:  true,
		histLimit: 300,
	}
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.headJump = "later"
	tool.headJumpPath = "/repo"
	list := commitList("visible")
	tool.syncHeadJump(m, list)
	if tool.headJumpAsked != 300 {
		t.Fatalf("asked = %d", tool.headJumpAsked)
	}
	if got := tool.syncHeadJump(m, list); got != "" || tool.headJump != "" || s.Selected() != "visible" {
		t.Fatalf("scroll=%q jump=%q sel=%q", got, tool.headJump, s.Selected())
	}
}

func TestNoteHeadJumpDropsOtherTab(t *testing.T) {
	s := &gitSession{path: "/repo"}
	other := &gitSession{path: "/other"}
	m := &GitModel{gitSession: other, tabs: []*gitSession{s, other}, active: 1}
	var tool GitTool
	tool.headJump = "abc"
	tool.headJumpTab = 0
	tool.headJumpPath = "/repo"
	tool.headJumpAsked = 300
	tool.noteHeadJump(m)
	if tool.headJump != "" || tool.headJumpAsked != 0 {
		t.Fatalf("jump=%q asked=%d", tool.headJump, tool.headJumpAsked)
	}
	tool.headJump = "abc"
	tool.headJumpPath = "/repo"
	m.active = 0
	m.gitSession = s
	s.path = "/moved"
	tool.noteHeadJump(m)
	if tool.headJump != "" {
		t.Fatal("path change kept the jump")
	}
}

func TestStartHeadJump(t *testing.T) {
	s := &gitSession{
		path: "/repo",
		repo: &gitcli.Repo{Dir: tTemp(t)},
		snap: gitcli.Snapshot{
			HEAD:  "bbb",
			Graph: []gitcli.GraphRow{{Commit: gitcli.Commit{Hash: "bbb"}}},
		},
		detail:    gitcli.CommitDetail{Commit: gitcli.Commit{Hash: "keep"}},
		pendingOp: make(chan gitOpResult),
	}
	m := &GitModel{gitSession: s, tabs: []*gitSession{s}}
	var tool GitTool
	tool.openHash = "keep"
	if !tool.canJumpHead(m) {
		t.Fatal("loaded row should stay enabled while busy")
	}
	tool.startHeadJump(m)
	if tool.headJump != "bbb" || tool.headJumpPath != "/repo" || tool.headJumpAsked != 0 {
		t.Fatalf("jump=%q path=%q asked=%d", tool.headJump, tool.headJumpPath, tool.headJumpAsked)
	}
	if s.pendingDetail != nil || s.Detail().Hash != "keep" || tool.openHash != "keep" {
		t.Fatal("start read the commit")
	}
	s.pendingOp = nil
	s.snap.Graph = nil
	s.snap.HEAD = ""
	tool.clearHeadJump()
	tool.startHeadJump(m)
	if tool.headJump != "" {
		t.Fatal("empty HEAD started a jump")
	}
	s.snap.HEAD = "later"
	s.histMore = true
	s.pendingOp = make(chan gitOpResult)
	tool.startHeadJump(m)
	if tool.headJump != "" {
		t.Fatal("busy page request started a jump")
	}
}

func tTemp(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func commitList(hashes ...string) *basicwidget.List[string] {
	var list basicwidget.List[string]
	items := make([]basicwidget.ListItem[string], len(hashes))
	for i, hash := range hashes {
		items[i].Value = hash
	}
	list.SetItems(items)
	return &list
}
