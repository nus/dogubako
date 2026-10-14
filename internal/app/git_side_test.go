package app

import (
	"testing"

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

func TestWorktreeTitle(t *testing.T) {
	if got := worktreeTitle(gitcli.Worktree{Path: "/repo/feature", Branch: "feature"}); got != "feature" {
		t.Fatalf("branch = %q", got)
	}
	if got := worktreeTitle(gitcli.Worktree{Path: "/repo/hotfix", Head: "abcdef123456", Detached: true}); got != "hotfix (abcdef1)" {
		t.Fatalf("detached = %q", got)
	}
}
