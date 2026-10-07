package app

import "testing"

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

func TestGitTreeItemsSkipsEmpty(t *testing.T) {
	items := gitTreeItems([]gitTreeEntry{{Name: ""}, {Name: "/"}}, nil)
	if len(items) != 0 {
		t.Fatalf("len = %d", len(items))
	}
}
