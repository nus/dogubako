package app

import (
	"strings"

	"github.com/guigui-gui/guigui/basicwidget"
)

const gitTreeFolderPrefix = "folder:"

// gitTreeEntry is one branch placed in a slash-separated tree.
type gitTreeEntry struct {
	Name    string
	Value   string
	Current bool
}

type gitTreeNode struct {
	seg     string
	path    string
	value   string
	current bool
	kids    []*gitTreeNode
	index   map[string]int
}

func (n *gitTreeNode) child(seg, path string) *gitTreeNode {
	if n.index == nil {
		n.index = map[string]int{}
	}
	if i, ok := n.index[seg]; ok {
		return n.kids[i]
	}
	c := &gitTreeNode{seg: seg, path: path}
	n.index[seg] = len(n.kids)
	n.kids = append(n.kids, c)
	return c
}

// gitTreeItems groups entries by slash-separated path. A name like
// feature/admin-dashboard/list becomes a folder, a nested folder, and a leaf.
// Folders are not selectable. Indent starts at 1 so a root folder can show
// an expander. folded marks folder paths that are collapsed.
func gitTreeItems(entries []gitTreeEntry, folded map[string]bool) []basicwidget.ListItem[string] {
	root := &gitTreeNode{}
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		node := root
		var path string
		parts := strings.Split(e.Name, "/")
		last := -1
		for i, part := range parts {
			if part == "" {
				continue
			}
			last = i
			if path == "" {
				path = part
			} else {
				path += "/" + part
			}
			node = node.child(part, path)
		}
		if last < 0 {
			continue
		}
		node.value = e.Value
		node.current = e.Current
	}

	items := make([]basicwidget.ListItem[string], 0, len(entries))
	var walk func(*gitTreeNode, int)
	walk = func(n *gitTreeNode, depth int) {
		for _, c := range n.kids {
			dir := len(c.kids) > 0
			text := c.seg
			if c.current {
				text += "  HEAD"
			}
			value := c.value
			if dir {
				value = gitTreeFolderPrefix + c.path
			}
			items = append(items, basicwidget.ListItem[string]{
				Text:         text,
				Value:        value,
				IndentLevel:  depth,
				Unselectable: dir,
				Collapsed:    dir && folded[c.path],
			})
			if dir {
				walk(c, depth+1)
			}
		}
	}
	walk(root, 1)
	return items
}

// visibleTreeItems drops rows hidden by a collapsed ancestor.
func visibleTreeItems(items []basicwidget.ListItem[string]) []basicwidget.ListItem[string] {
	out := make([]basicwidget.ListItem[string], 0, len(items))
	hideAbove := 0
	for _, item := range items {
		if hideAbove > 0 && item.IndentLevel > hideAbove {
			continue
		}
		hideAbove = 0
		out = append(out, item)
		if item.Collapsed {
			hideAbove = item.IndentLevel
		}
	}
	return out
}
