package app

import (
	"path"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/gitcli"
)

// worktreeParentKey identifies the main working tree of a loaded repository.
// Linked worktrees share it with that main tree. Local and SSH checkouts stay
// distinct, and so do two SSH destinations that differ by host or port.
// An empty key means the main tree is not known yet.
func (s *gitSession) worktreeParentKey() string {
	if s == nil || s.repo == nil {
		return ""
	}
	main := ""
	for _, wt := range s.snap.Worktrees {
		if wt.Main && wt.Path != "" {
			main = wt.Path
			break
		}
	}
	if main == "" {
		return ""
	}
	loc := gitcli.ParseLoc(s.path)
	if loc.IsRemote() {
		return "ssh\x00" + loc.Dest + "\x00" + loc.Port + "\x00" + path.Clean(main)
	}
	dir := canonicalPath(main)
	if dir == "" {
		return ""
	}
	return "local\x00" + dir
}

// listsWorktree reports that this repository's worktree list contains dir.
func (s *gitSession) listsWorktree(dir string) bool {
	if s == nil || s.repo == nil || dir == "" {
		return false
	}
	loc := gitcli.ParseLoc(s.path)
	want := gitcli.ParseLoc(dir)
	if loc.IsRemote() != want.IsRemote() {
		return false
	}
	if loc.IsRemote() && (loc.Dest != want.Dest || loc.Port != want.Port) {
		return false
	}
	target := canonicalPath(dir)
	if loc.IsRemote() {
		target = path.Clean(want.Dir)
	}
	if target == "" {
		return false
	}
	for _, wt := range s.snap.Worktrees {
		got := wt.Path
		if loc.IsRemote() {
			got = path.Clean(got)
		} else {
			got = canonicalPath(got)
		}
		if got == target {
			return true
		}
	}
	return false
}

// tabSharingWorktree is a tab whose parent worktree is also the parent of dir.
// That includes the parent checkout itself and every linked worktree.
func (m *GitModel) tabSharingWorktree(dir string) int {
	m.ensure()
	for i, s := range m.tabs {
		if s.listsWorktree(dir) {
			return i
		}
	}
	if len(m.parked) == 0 {
		return -1
	}
	parked := m.parked[canonicalPath(dir)]
	key := parked.worktreeParentKey()
	if key == "" {
		return -1
	}
	for i, s := range m.tabs {
		if s.worktreeParentKey() == key {
			return i
		}
	}
	return -1
}

// mergeWorktreeTabs keeps one tab for each parent worktree.
// The tab stays where the first member was. The shown checkout is the active
// member when that tab is active, and the earlier member otherwise.
// The checkouts that leave the strip are kept, so switching back restores them.
func (m *GitModel) mergeWorktreeTabs() {
	m.ensure()
	if len(m.tabs) < 2 {
		return
	}
	next := make([]*gitSession, 0, len(m.tabs))
	slot := map[string]int{}
	active := m.gitSession
	changed := false
	for _, s := range m.tabs {
		key := s.worktreeParentKey()
		if key == "" || s.path == "" {
			next = append(next, s)
			continue
		}
		prev, ok := slot[key]
		if !ok {
			slot[key] = len(next)
			next = append(next, s)
			continue
		}
		changed = true
		if s == active {
			m.park(next[prev])
			next[prev] = s
			continue
		}
		m.park(s)
	}
	if !changed {
		return
	}
	m.tabs = next
	if active == nil && len(next) > 0 {
		active = next[0]
	}
	m.gitSession = active
	m.active = 0
	for i, s := range m.tabs {
		if s == active {
			m.active = i
			break
		}
	}
	m.extraGen++
	m.saveTabs()
	guigui.RequestRebuild()
}
