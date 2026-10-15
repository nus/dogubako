package app

import (
	"testing"

	"github.com/guigui-gui/guigui/basicwidget"
)

func TestGitTreeItemsShowsLeafNote(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{
		{Name: "vendor/lib", Value: "submodule:vendor/lib", Note: "0123456"},
	}, nil)
	if len(items) != 2 {
		t.Fatalf("len = %d", len(items))
	}
	if items[1].Text != "lib  0123456" || items[1].Value != "submodule:vendor/lib" || items[1].Unselectable {
		t.Fatalf("leaf = %+v", items[1])
	}
}

func TestGitTreeItemsNestsSlashes(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{
		{Name: "main", Value: "local:main", Current: true},
		{Name: "feature/admin-dashboard/list", Value: "local:feature/admin-dashboard/list"},
		{Name: "feature/order-processing", Value: "local:feature/order-processing"},
	}, nil)

	want := []struct {
		text   string
		value  string
		indent int
		dir    bool
	}{
		{"main  HEAD", "local:main", 1, false},
		{"feature", "folder:feature", 1, true},
		{"admin-dashboard", "folder:feature/admin-dashboard", 2, true},
		{"list", "local:feature/admin-dashboard/list", 3, false},
		{"order-processing", "local:feature/order-processing", 2, false},
	}
	if len(items) != len(want) {
		t.Fatalf("len = %d, want %d", len(items), len(want))
	}
	for i, w := range want {
		got := items[i]
		if got.Text != w.text || got.Value != w.value || got.IndentLevel != w.indent || got.Unselectable != w.dir || got.Collapsed {
			t.Fatalf("item %d = %+v", i, got)
		}
	}
}

func TestGitTreeItemsCollapsedFolder(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{
		{Name: "origin/main", Value: "remote:origin/main"},
		{Name: "origin/feature/foo", Value: "remote:origin/feature/foo"},
	}, map[string]bool{"origin": true})

	if len(items) != 4 {
		t.Fatalf("len = %d", len(items))
	}
	if items[0].Text != "origin" || !items[0].Collapsed || !items[0].Unselectable {
		t.Fatalf("origin = %+v", items[0])
	}
	if items[1].Text != "main" || items[1].IndentLevel != 2 {
		t.Fatalf("main = %+v", items[1])
	}
	if items[2].Text != "feature" || items[2].Collapsed {
		t.Fatalf("feature = %+v", items[2])
	}
}

func TestVisibleTreeItemsHidesCollapsedChildren(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{
		{Name: "origin/main", Value: "remote:origin/main"},
		{Name: "origin/feature/foo", Value: "remote:origin/feature/foo"},
	}, map[string]bool{"origin": true})
	vis := visibleTreeItems(items)
	if len(vis) != 1 || vis[0].Text != "origin" {
		t.Fatalf("visible = %+v", vis)
	}
}

func TestGitFilterRefItems(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{
		{Name: "main", Value: "local:main", Current: true},
		{Name: "feature/admin-dashboard/list", Value: "local:feature/admin-dashboard/list"},
		{Name: "feature/order-processing", Value: "local:feature/order-processing"},
	}, map[string]bool{"feature": true})

	if got := gitFilterRefItems(items, "  "); len(got) != len(items) || !got[1].Collapsed {
		t.Fatalf("blank query = %+v", got)
	}

	got := gitFilterRefItems(items, "  List ")
	if len(got) != 3 {
		t.Fatalf("list len = %d", len(got))
	}
	if got[0].Value != "folder:feature" || got[0].Collapsed {
		t.Fatalf("feature = %+v", got[0])
	}
	if got[1].Value != "folder:feature/admin-dashboard" || got[1].Collapsed {
		t.Fatalf("admin = %+v", got[1])
	}
	if got[2].Value != "local:feature/admin-dashboard/list" {
		t.Fatalf("leaf = %+v", got[2])
	}

	got = gitFilterRefItems(items, "MAIN")
	if len(got) != 1 || got[0].Value != "local:main" {
		t.Fatalf("main = %+v", got)
	}

	if got = gitFilterRefItems(items, "zzz"); len(got) != 0 {
		t.Fatalf("none = %+v", got)
	}

	tags := []basicwidget.ListItem[string]{
		{Text: "v1.0", Value: "tag:v1.0"},
		{Text: "v2.0", Value: "tag:v2.0"},
	}
	got = gitFilterRefItems(tags, "v2")
	if len(got) != 1 || got[0].Value != "tag:v2.0" {
		t.Fatalf("tag = %+v", got)
	}

	remotes := gitTreeItems([]gitTreeEntry{
		{Name: "origin/main", Value: "remote:origin/main"},
		{Name: "origin/feature/foo", Value: "remote:origin/feature/foo"},
	}, nil)
	got = gitFilterRefItems(remotes, "foo")
	if len(got) != 3 || got[2].Value != "remote:origin/feature/foo" {
		t.Fatalf("remote = %+v", got)
	}
	got = gitFilterRefItems(remotes, "main")
	if len(got) != 2 || got[0].Value != "folder:origin" || got[1].Value != "remote:origin/main" {
		t.Fatalf("origin/main = %+v", got)
	}
}

func TestGitTreeItemsSkipsEmpty(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{{Name: ""}, {Name: "/"}}, nil)
	if len(items) != 0 {
		t.Fatalf("len = %d", len(items))
	}
}
