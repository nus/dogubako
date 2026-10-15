package app

import (
	"image"
	"testing"

	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestGitSideWidth(t *testing.T) {
	const split = 12
	const minSide = 144
	const minRight = 384

	if got := gitSideWidth(1200, split, 216, minSide, minRight); got != 216 {
		t.Fatalf("default = %d", got)
	}
	if got := gitSideWidth(1200, split, 2000, minSide, minRight); got != 1200-split-minRight {
		t.Fatalf("wide = %d", got)
	}
	if got := gitSideWidth(1200, split, 10, minSide, minRight); got != minSide {
		t.Fatalf("narrow = %d", got)
	}
	if got := gitSideWidth(100, split, 50, minSide, minRight); got < 1 || got >= 100-split {
		t.Fatalf("tight = %d", got)
	}
}

func TestGitStickyStack(t *testing.T) {
	const h = 24
	tops := []int{0, 200, 400}
	if got := gitStickyStack(nil, tops, 0, h); len(got) != 0 {
		t.Fatalf("at rest = %v", got)
	}
	if got := gitStickyStack(nil, tops, 40, h); len(got) != 1 || got[0] != 0 {
		t.Fatalf("first = %v", got)
	}
	got := gitStickyStack(nil, tops, 190, h)
	if len(got) != 2 || got[0] != 0 || got[1] != h {
		t.Fatalf("stacked = %v", got)
	}
	got = gitStickyStack(nil, tops, 400, h)
	if len(got) != 3 || got[0] != 0 || got[1] != h || got[2] != 2*h {
		t.Fatalf("all = %v", got)
	}
	if got := gitStickyStack(nil, nil, 10, h); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}

func TestGitStickyScrollTarget(t *testing.T) {
	const h = 24
	if got := gitStickyScrollTarget(0, 0, h); got != 0 {
		t.Fatalf("top = %d", got)
	}
	if got := gitStickyScrollTarget(200, 1, h); got != 200-h {
		t.Fatalf("second = %d", got)
	}
	if got := gitStickyScrollTarget(400, 2, h); got != 400-2*h {
		t.Fatalf("third = %d", got)
	}
	if got := gitStickyScrollTarget(10, 3, h); got != 0 {
		t.Fatalf("clamp = %d", got)
	}
}

func TestGitSideRefLists(t *testing.T) {
	tool := &GitTool{
		branchClosed: true,
		showTags:     true,
		branchItems: gitTreeItems([]gitTreeEntry{
			{Name: "main", Value: "local:main"},
			{Name: "feature/list", Value: "local:feature/list"},
		}, nil),
		tagItems: []basicwidget.ListItem[string]{
			{Text: "v1", Value: "tag:v1"},
		},
		remoteItems: gitTreeItems([]gitTreeEntry{
			{Name: "origin/main", Value: "remote:origin/main"},
		}, nil),
		submoduleItems: gitTreeItems([]gitTreeEntry{
			{Name: "vendor/lib", Value: "submodule:vendor/lib"},
			{Name: "extras/tool", Value: "submodule:extras/tool"},
		}, nil),
	}
	closed := gitSideRefLists(tool)
	if !closed.branchHead || closed.branchOpen || len(closed.branch) != 0 {
		t.Fatalf("closed branches = %+v", closed)
	}
	if !closed.tagHead || !closed.remoteHead || !closed.subHead {
		t.Fatalf("other sections = %+v", closed)
	}
	if closed.stickyCount() != 4 {
		t.Fatalf("sticky = %d", closed.stickyCount())
	}

	tool.refFilter = "list"
	filtered := gitSideRefLists(tool)
	if !filtered.branchHead || !filtered.branchOpen {
		t.Fatalf("filter opens matches = %+v", filtered)
	}
	if len(filtered.branch) != 2 || filtered.branch[1].Value != "local:feature/list" {
		t.Fatalf("branch rows = %+v", filtered.branch)
	}
	if !filtered.tagHead || !filtered.remoteHead || !filtered.subHead {
		t.Fatalf("empty sections hidden = %+v", filtered)
	}
	if len(filtered.tag) != 0 || len(filtered.remote) != 0 || len(filtered.sub) != 0 {
		t.Fatalf("unmatched rows = %+v", filtered)
	}
	if filtered.stickyCount() != 4 {
		t.Fatalf("filtered sticky = %d", filtered.stickyCount())
	}

	tool.refFilter = "lib"
	got := gitSideRefLists(tool)
	if len(got.sub) != 2 || got.sub[1].Value != "submodule:vendor/lib" {
		t.Fatalf("submodule = %+v", got.sub)
	}
	if !got.branchHead || len(got.branch) != 0 {
		t.Fatalf("empty branches = %+v", got)
	}

	tool.refFilter = "   "
	if got := gitSideRefLists(tool); !got.tagHead || !got.remoteHead {
		t.Fatalf("blank filter = %+v", got)
	}
}

func TestGitSideFilterInputSitsOnBottom(t *testing.T) {
	b := image.Rect(0, 0, 200, 48)
	in := gitSideFilterInputBounds(b, 24)
	if gap := b.Max.Y - in.Max.Y; gap != gitSideFilterBottom(24) {
		t.Fatalf("bottom gap = %d, want %d", gap, gitSideFilterBottom(24))
	}
	if in.Min.Y <= b.Min.Y || in.Min.X <= b.Min.X || in.Max.X >= b.Max.X {
		t.Fatalf("insets = %v", in)
	}
	box := gitFilterClearBounds(in, 24)
	if !box.In(in) || box.Empty() {
		t.Fatalf("clear = %v field = %v", box, in)
	}
	if box.Max.X > in.Max.X-2 {
		t.Fatalf("clear touches the edge: %v", box)
	}
}

func TestWorktreeTitle(t *testing.T) {
	if got := worktreeTitle(gitcli.Worktree{Path: "/repo/feature", Branch: "feature"}); got != "feature" {
		t.Fatalf("branch = %q", got)
	}
	if got := worktreeTitle(gitcli.Worktree{Path: "/repo/hotfix", Head: "abcdef123456", Detached: true}); got != "hotfix (abcdef1)" {
		t.Fatalf("detached = %q", got)
	}
}
