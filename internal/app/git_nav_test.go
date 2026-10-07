package app

import "testing"

func TestGitNavIconKind(t *testing.T) {
	if got := gitNavIconKind(true, "folder:feature"); got != gitNavIconFolder {
		t.Fatalf("folder = %d", got)
	}
	if got := gitNavIconKind(false, "local:main"); got != gitNavIconBranch {
		t.Fatalf("local = %d", got)
	}
	if got := gitNavIconKind(false, "remote:origin/main"); got != gitNavIconBranch {
		t.Fatalf("remote = %d", got)
	}
	if got := gitNavIconKind(false, "tag:v1.0"); got != gitNavIconTag {
		t.Fatalf("tag = %d", got)
	}
}

func TestGitNavLeadAlignsLeafWithFolder(t *testing.T) {
	const u = 24
	folder := gitNavLeadLayout(1, true, u)
	leaf := gitNavLeadLayout(1, false, u)
	if folder.arrowX < 0 || leaf.arrowX >= 0 {
		t.Fatalf("arrow folder=%d leaf=%d", folder.arrowX, leaf.arrowX)
	}
	if folder.iconX != leaf.iconX {
		t.Fatalf("icon x folder=%d leaf=%d", folder.iconX, leaf.iconX)
	}
	if folder.textX <= folder.iconX+folder.iconSize || leaf.textX != folder.textX {
		t.Fatalf("text folder=%+v leaf=%+v", folder, leaf)
	}
	tag := gitNavLeadLayout(0, false, u)
	if tag.textX <= tag.iconX+tag.iconSize {
		t.Fatalf("tag lead = %+v", tag)
	}
}

func TestGitWorkspaceLeadLeavesRoom(t *testing.T) {
	iconX, iconSize, textX := gitWorkspaceLead(24)
	if iconX != 0 || textX <= iconX+iconSize {
		t.Fatalf("lead = %d %d %d", iconX, iconSize, textX)
	}
}
