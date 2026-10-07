package app

import "testing"

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
