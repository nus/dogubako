package app

import (
	"testing"

	"github.com/nus/dogubako/internal/gitcli"
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
