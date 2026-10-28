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
