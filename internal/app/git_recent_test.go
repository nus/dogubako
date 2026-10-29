package app

import (
	"slices"
	"testing"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestGitRecentVisible(t *testing.T) {
	proj := "/Users/me/Project"
	appPath := "/Users/me/app"
	dev := gitcli.Loc{Dest: "dev", Port: "2222", Dir: "/home/me/repo"}.Key()
	other := gitcli.Loc{Dest: "other", Dir: "/srv/app"}.Key()
	spaced := gitcli.Loc{Dest: "my host", Dir: "/home/foo/repo"}.Key()
	paths := []string{proj, dev, appPath, other, spaced}
	before := slices.Clone(paths)

	if got := gitRecentVisible(paths, "  "); len(got) != len(paths) {
		t.Fatalf("blank = %v", got)
	}
	if got := gitRecentVisible(paths, ""); len(got) != len(paths) {
		t.Fatalf("empty = %v", got)
	}

	got := gitRecentVisible(paths, "PROJ")
	if len(got) != 1 || got[0] != proj {
		t.Fatalf("folder = %v", got)
	}
	got = gitRecentVisible(paths, "2222")
	if len(got) != 1 || got[0] != dev {
		t.Fatalf("port = %v", got)
	}
	got = gitRecentVisible(paths, "DEV")
	if len(got) != 1 || got[0] != dev {
		t.Fatalf("host = %v", got)
	}
	got = gitRecentVisible(paths, "app")
	if len(got) != 2 || got[0] != appPath || got[1] != other {
		t.Fatalf("path = %v", got)
	}
	got = gitRecentVisible(paths, "my host")
	if len(got) != 1 || got[0] != spaced {
		t.Fatalf("display = %v", got)
	}
	if got = gitRecentVisible(paths, "my%20host"); len(got) != 0 {
		t.Fatalf("storage key = %v", got)
	}
	if got = gitRecentVisible(paths, "ssh:"); len(got) != 0 {
		t.Fatalf("prefix = %v", got)
	}
	got = gitRecentVisible(paths, "repo")
	if len(got) != 2 || got[0] != dev || got[1] != spaced {
		t.Fatalf("order = %v", got)
	}
	if !slices.Equal(paths, before) {
		t.Fatalf("mutated = %v", paths)
	}
}

func TestRecentFilterFocusOnArrival(t *testing.T) {
	var tool GitTool
	tool.noteRecentFilter(false)
	if tool.focusRecentFilter {
		t.Fatal("hidden")
	}
	tool.noteRecentFilter(true)
	if !tool.focusRecentFilter {
		t.Fatal("first show")
	}
	tool.focusRecentFilter = false
	tool.noteRecentFilter(true)
	if tool.focusRecentFilter {
		t.Fatal("rebuild")
	}
	tool.noteRecentFilter(false)
	tool.noteRecentFilter(true)
	if !tool.focusRecentFilter {
		t.Fatal("return")
	}
}

func TestRecentQueryClearsOnAnotherTab(t *testing.T) {
	var tool GitTool
	a := &gitSession{}
	b := &gitSession{}
	tool.noteRecentSession(a)
	tool.recentQuery = "keep"
	tool.recentFilter.input.ForceSetValue("keep")
	tool.noteRecentSession(a)
	if tool.recentQuery != "keep" || tool.recentFilter.input.Value() != "keep" {
		t.Fatalf("same tab query = %q input = %q", tool.recentQuery, tool.recentFilter.input.Value())
	}
	tool.noteRecentSession(b)
	if tool.recentQuery != "" || tool.recentFilter.input.HasValue() {
		t.Fatalf("other tab query = %q has = %v", tool.recentQuery, tool.recentFilter.input.HasValue())
	}
	tool.recentQuery = "next"
	tool.recentFilter.input.ForceSetValue("next")
	tool.noteRecentSession(b)
	if tool.recentQuery != "next" || tool.recentFilter.input.Value() != "next" {
		t.Fatalf("stay query = %q input = %q", tool.recentQuery, tool.recentFilter.input.Value())
	}
}
