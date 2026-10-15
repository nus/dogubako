package app

import (
	"strings"

	"github.com/guigui-gui/guigui/basicwidget"
)

const gitTreeFolderPrefix = "folder:"

// gitTreeEntry is one branch placed in a slash-separated tree.
// Note is extra text shown on a leaf, such as a submodule commit.
type gitTreeEntry struct {
	Name    string
	Value   string
	Current bool
	Note    string
}

type gitTreeNode struct {
	seg     string
	path    string
	value   string
	current bool
	note    string
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
		node.note = e.Note
	}

	items := make([]basicwidget.ListItem[string], 0, len(entries))
	var walk func(*gitTreeNode, int)
	walk = func(n *gitTreeNode, depth int) {
		for _, c := range n.kids {
			dir := len(c.kids) > 0
			text := c.seg
			if c.current {
				text += "  HEAD"
			} else if !dir && c.note != "" {
				text += "  " + c.note
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

// gitRefItemName is the branch, tag, remote-branch, or submodule name stored on a row.
func gitRefItemName(value string) string {
	switch {
	case strings.HasPrefix(value, "local:"):
		return strings.TrimPrefix(value, "local:")
	case strings.HasPrefix(value, "tag:"):
		return strings.TrimPrefix(value, "tag:")
	case strings.HasPrefix(value, "remote:"):
		return strings.TrimPrefix(value, "remote:")
	case strings.HasPrefix(value, "submodule:"):
		return strings.TrimPrefix(value, "submodule:")
	default:
		return value
	}
}

// gitFilterRefItems keeps leaves whose name contains query, plus the folders
// on the path to those leaves. Matching ignores case. Folders are expanded so
// a collapsed parent does not hide a match. An empty query returns items.
func gitFilterRefItems(items []basicwidget.ListItem[string], query string) []basicwidget.ListItem[string] {
	query = strings.TrimSpace(query)
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	keep := make([]bool, len(items))
	stack := make([]int, 0, 8)
	indents := make([]int, 0, 8)
	for i, item := range items {
		for len(stack) > 0 && indents[len(indents)-1] >= item.IndentLevel {
			stack = stack[:len(stack)-1]
			indents = indents[:len(indents)-1]
		}
		if item.Unselectable {
			stack = append(stack, i)
			indents = append(indents, item.IndentLevel)
			continue
		}
		if strings.Contains(strings.ToLower(gitRefItemName(item.Value)), q) {
			keep[i] = true
			for _, idx := range stack {
				keep[idx] = true
			}
		}
	}
	out := make([]basicwidget.ListItem[string], 0, len(items))
	for i, item := range items {
		if !keep[i] {
			continue
		}
		if item.Unselectable {
			item.Collapsed = false
		}
		out = append(out, item)
	}
	return out
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
