package app

import (
	"testing"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

func TestBranchMenuItems(t *testing.T) {
	type item struct {
		text, value string
		disabled    bool
	}
	check := func(name string, ref gitcli.Ref, want []item) {
		t.Helper()
		got := branchMenuItems(ref, "Push", "Rename", "Delete")
		if len(got) != len(want) {
			t.Fatalf("%s len = %d", name, len(got))
		}
		for i, gotItem := range got {
			if gotItem.Text != want[i].text || gotItem.Value != want[i].value || gotItem.Disabled != want[i].disabled {
				t.Fatalf("%s [%d] = %q %q disabled=%v", name, i, gotItem.Text, gotItem.Value, gotItem.Disabled)
			}
		}
	}
	check("checked out", gitcli.Ref{Name: "main", Current: true}, []item{
		{"Push", "push", false},
		{"Rename", "rename", false},
		{"Delete", "delete", true},
	})
	check("other local", gitcli.Ref{Name: "topic"}, []item{
		{"Push", "push", false},
		{"Rename", "rename", false},
		{"Delete", "delete", false},
	})
	check("remote", gitcli.Ref{Name: "origin/main", Remote: "origin"}, []item{
		{"Push", "push", true},
		{"Rename", "rename", true},
		{"Delete", "delete", false},
	})
	check("current remote", gitcli.Ref{Name: "origin/main", Remote: "origin", Current: true}, []item{
		{"Push", "push", true},
		{"Rename", "rename", true},
		{"Delete", "delete", true},
	})
}

func TestCommitMenuItems(t *testing.T) {
	type item struct {
		text, value string
		disabled    bool
	}
	check := func(name string, rebaseDisabled bool, want []item) {
		t.Helper()
		got := commitMenuItems("Create branch", "Create tag", "Rebase onto this commit", rebaseDisabled)
		if len(got) != len(want) {
			t.Fatalf("%s len = %d", name, len(got))
		}
		for i, gotItem := range got {
			if gotItem.Text != want[i].text || gotItem.Value != want[i].value || gotItem.Disabled != want[i].disabled {
				t.Fatalf("%s [%d] = %q %q disabled=%v", name, i, gotItem.Text, gotItem.Value, gotItem.Disabled)
			}
		}
	}
	check("onto another commit", false, []item{
		{"Create branch", "branch", false},
		{"Create tag", "tag", false},
		{"Rebase onto this commit", "rebase", false},
	})
	check("disabled rebase stays listed", true, []item{
		{"Create branch", "branch", false},
		{"Create tag", "tag", false},
		{"Rebase onto this commit", "rebase", true},
	})
}

func TestCommitRebaseDisabled(t *testing.T) {
	if commitRebaseDisabled(false, "main", "aaa", "bbb") {
		t.Fatal("another commit on a branch should rebase")
	}
	if !commitRebaseDisabled(true, "main", "aaa", "bbb") {
		t.Fatal("detached HEAD")
	}
	if !commitRebaseDisabled(false, "", "aaa", "bbb") {
		t.Fatal("no branch")
	}
	if !commitRebaseDisabled(false, "main", "aaa", "aaa") {
		t.Fatal("HEAD itself")
	}
}

func TestWorktreeMenuItems(t *testing.T) {
	got := worktreeMenuItems("Close", false)
	if len(got) != 1 || got[0].Text != "Close" || got[0].Value != "close" || got[0].Disabled {
		t.Fatalf("linked = %+v", got)
	}
	got = worktreeMenuItems("閉じる", true)
	if len(got) != 1 || got[0].Text != "閉じる" || got[0].Value != "close" || !got[0].Disabled {
		t.Fatalf("main = %+v", got)
	}
	if !worktreeCloseDisabled(gitcli.Worktree{Main: true, Path: "/repo", Branch: "main"}) {
		t.Fatal("main row should not close")
	}
	if !worktreeCloseDisabled(gitcli.Worktree{Main: true, Bare: true, Path: "/repo/bare.git"}) {
		t.Fatal("bare parent should not close")
	}
	if worktreeCloseDisabled(gitcli.Worktree{Path: "/repo/feature", Branch: "feature", Current: true}) {
		t.Fatal("the open linked worktree stays enabled")
	}
}

func TestWorktreeCloseCopy(t *testing.T) {
	ja := i18n.T(i18n.JA, i18n.GitWorktreeCloseAsk, "feature")
	if ja != "ワークツリー「feature」を閉じますか？" {
		t.Fatalf("ja ask = %q", ja)
	}
	en := i18n.T(i18n.EN, i18n.GitWorktreeCloseAsk, "feature")
	if en != `Close worktree "feature"?` {
		t.Fatalf("en ask = %q", en)
	}
	if i18n.T(i18n.JA, i18n.GitWorktreeClose) != "閉じる" || i18n.T(i18n.EN, i18n.GitWorktreeClose) != "Close" {
		t.Fatal("close")
	}
	if i18n.T(i18n.JA, i18n.GitWorktreeForce) != "強制削除" || i18n.T(i18n.EN, i18n.GitWorktreeForce) != "Force Delete" {
		t.Fatal("force")
	}
	if got := worktreeRemoveChoice("閉じる", "remove"); got != "閉じる (remove)" {
		t.Fatalf("ja close choice = %q", got)
	}
	if got := worktreeRemoveChoice("Close", "remove"); got != "Close (remove)" {
		t.Fatalf("en close choice = %q", got)
	}
	if got := worktreeRemoveChoice("強制削除", "remove --force --force"); got != "強制削除 (remove --force --force)" {
		t.Fatalf("ja force choice = %q", got)
	}
	if got := worktreeRemoveChoice("Force Delete", "remove --force --force"); got != "Force Delete (remove --force --force)" {
		t.Fatalf("en force choice = %q", got)
	}
	if i18n.T(i18n.JA, i18n.StatusGitWorktreeClosed) != "ワークツリーを閉じました" || i18n.T(i18n.EN, i18n.StatusGitWorktreeClosed) != "Closed worktree" {
		t.Fatal("closed")
	}
	if i18n.T(i18n.JA, i18n.StatusGitWorktreeCloseErr, "locked") != "ワークツリーを閉じられませんでした: locked" {
		t.Fatal("ja err")
	}
	if i18n.T(i18n.EN, i18n.StatusGitWorktreeCloseErr, "locked") != "Could not close worktree: locked" {
		t.Fatal("en err")
	}
}

func TestRebaseAskCopy(t *testing.T) {
	short := gitcli.Commit{Hash: "abcdef1234567890"}.Short()
	if short != "abcdef1" {
		t.Fatalf("short = %s", short)
	}
	ja := i18n.T(i18n.JA, i18n.GitRebaseAsk, "main", short)
	if ja != "ブランチ main を abcdef1 の上へ rebase しますか？コミットは書き換わります。" {
		t.Fatalf("ja = %q", ja)
	}
	en := i18n.T(i18n.EN, i18n.GitRebaseAsk, "main", short)
	if en != "Rebase branch main onto abcdef1? This rewrites commits." {
		t.Fatalf("en = %q", en)
	}
	if i18n.T(i18n.JA, i18n.GitRebaseOnto) != "このコミットへ rebase" || i18n.T(i18n.EN, i18n.GitRebaseOnto) != "Rebase onto this commit" {
		t.Fatal("menu label")
	}
	if i18n.T(i18n.JA, i18n.GitRebase) != "Rebase" || i18n.T(i18n.EN, i18n.GitRebase) != "Rebase" {
		t.Fatal("button")
	}
}
