package app

import "testing"

func TestGitWorkFilesWidth(t *testing.T) {
	const split = 12
	const minFiles = 144
	const minDiff = 192

	if got := gitWorkFilesWidth(900, split, 0, minFiles, minDiff); got != (900-split)/3 {
		t.Fatalf("default split = %d", got)
	}
	if got := gitWorkFilesWidth(900, split, 0.9, minFiles, minDiff); got != 900-split-minDiff {
		t.Fatalf("wide files = %d", got)
	}
	if got := gitWorkFilesWidth(900, split, 0.05, minFiles, minDiff); got != minFiles {
		t.Fatalf("narrow files = %d", got)
	}
	if got := gitWorkFilesWidth(100, split, 0.5, minFiles, minDiff); got < 1 || got >= 100-split {
		t.Fatalf("tight pane = %d", got)
	}
}
