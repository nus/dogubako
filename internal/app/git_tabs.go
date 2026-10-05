package app

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/guigui-gui/guigui"
)

func (m *GitModel) ensure() {
	if len(m.tabs) == 0 {
		m.tabs = []*gitSession{{owner: m}}
	}
	if m.active < 0 || m.active >= len(m.tabs) {
		m.active = 0
	}
	for _, s := range m.tabs {
		if s != nil && s.owner == nil {
			s.owner = m
		}
	}
	m.gitSession = m.tabs[m.active]
}

func (m *GitModel) Generation() uint64 {
	m.ensure()
	g := m.extraGen
	for _, s := range m.tabs {
		g += s.generation
	}
	return g
}

func (m *GitModel) Drain() {
	m.ensure()
	for _, s := range m.tabs {
		s.Drain()
	}
	m.dedupeTabs()
}

func (m *GitModel) EnsureLoaded() {
	if m.loaded {
		return
	}
	m.loaded = true
	paths, active, recent := loadGitTabs()
	m.recent = recent
	if len(m.recent) == 0 {
		m.recent = seedGitRecent(paths, active)
	}
	if len(paths) == 0 {
		m.ensure()
		return
	}
	m.tabs = make([]*gitSession, 0, len(paths))
	for range paths {
		m.tabs = append(m.tabs, &gitSession{owner: m})
	}
	if active < 0 || active >= len(m.tabs) {
		active = 0
	}
	m.active = active
	m.gitSession = m.tabs[active]
	m.extraGen++
	for i, p := range paths {
		if strings.TrimSpace(p) != "" {
			m.tabs[i].open(p)
		}
	}
}

// Open shows path in the tab that already has it, fills the current tab when
// that tab is empty, or adds a tab and opens the repository there.
// Switching from an empty tab to one that already has the repository closes
// the empty tab.
func (m *GitModel) Open(path string) {
	path = canonicalPath(path)
	if path == "" || path == "." {
		return
	}
	m.ensure()
	m.rememberRepo(path)
	if i := m.tabIndex(path); i >= 0 {
		from := m.active
		closeEmpty := from != i && m.tabs[from].path == "" && !m.tabs[from].Busy()
		m.SelectTab(i)
		s := m.tabs[i]
		if !s.HasRepo() && !s.Busy() {
			s.open(path)
		}
		if closeEmpty {
			m.CloseTab(from)
		}
		return
	}
	if strings.TrimSpace(m.path) != "" {
		m.addTab()
	}
	m.open(path)
}

func (m *GitModel) TabCount() int {
	m.ensure()
	return len(m.tabs)
}

func (m *GitModel) ActiveTab() int {
	m.ensure()
	return m.active
}

// TabLabel is the repository folder name, or empty when the tab has no repository.
// Tabs that share a folder name include the parent directory.
func (m *GitModel) TabLabel(index int) string {
	m.ensure()
	if index < 0 || index >= len(m.tabs) {
		return ""
	}
	path := m.tabs[index].path
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	for i, other := range m.tabs {
		if i == index || other.path == "" {
			continue
		}
		if filepath.Base(other.path) == base {
			parent := filepath.Base(filepath.Dir(path))
			if parent != "" && parent != "." && parent != string(filepath.Separator) {
				return parent + "/" + base
			}
			break
		}
	}
	return base
}

func (m *GitModel) SelectTab(index int) {
	m.ensure()
	if index < 0 || index >= len(m.tabs) || index == m.active {
		return
	}
	m.active = index
	m.gitSession = m.tabs[index]
	m.tabs[index].lastPoll = time.Time{}
	m.extraGen++
	m.saveTabs()
	guigui.RequestRebuild()
}

func (m *GitModel) NewTab() {
	m.ensure()
	for i, s := range m.tabs {
		if s.path == "" && !s.Busy() {
			m.SelectTab(i)
			return
		}
	}
	m.addTab()
}

func (m *GitModel) CloseTab(index int) {
	m.ensure()
	if index < 0 || index >= len(m.tabs) {
		return
	}
	if len(m.tabs) == 1 {
		m.tabs[0] = &gitSession{owner: m}
		m.active = 0
		m.gitSession = m.tabs[0]
		m.extraGen++
		m.saveTabs()
		guigui.RequestRebuild()
		return
	}
	m.tabs = append(m.tabs[:index], m.tabs[index+1:]...)
	if index < m.active {
		m.active--
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	m.gitSession = m.tabs[m.active]
	m.extraGen++
	m.saveTabs()
	guigui.RequestRebuild()
}

func (m *GitModel) addTab() {
	s := &gitSession{owner: m}
	m.tabs = append(m.tabs, s)
	m.active = len(m.tabs) - 1
	m.gitSession = s
	m.extraGen++
	m.saveTabs()
	guigui.RequestRebuild()
}

func (m *GitModel) anyBusy() bool {
	m.ensure()
	for _, s := range m.tabs {
		if s.Busy() {
			return true
		}
	}
	return false
}

func (m *GitModel) tabIndex(path string) int {
	path = canonicalPath(path)
	if path == "" || path == "." {
		return -1
	}
	for i, s := range m.tabs {
		if s.path != "" && canonicalPath(s.path) == path {
			return i
		}
	}
	return -1
}

func (m *GitModel) saveTabs() {
	paths := make([]string, len(m.tabs))
	for i, s := range m.tabs {
		paths[i] = s.path
	}
	_ = saveGitTabs(paths, m.active, m.recent)
}

// RecentPaths is the most recently opened repositories, newest first.
func (m *GitModel) RecentPaths() []string {
	if len(m.recent) == 0 {
		return nil
	}
	return slices.Clone(m.recent)
}

func (m *GitModel) rememberRepo(path string) {
	path = canonicalPath(path)
	if path == "" || path == "." {
		return
	}
	next := make([]string, 0, len(m.recent)+1)
	next = append(next, path)
	for _, p := range m.recent {
		if p == path {
			continue
		}
		next = append(next, p)
		if len(next) == gitRecentMax {
			break
		}
	}
	if slices.Equal(m.recent, next) {
		return
	}
	m.recent = next
	m.extraGen++
	m.saveTabs()
}

func (m *GitModel) forgetRecent(path string) {
	path = canonicalPath(path)
	if path == "" || path == "." {
		return
	}
	next := make([]string, 0, len(m.recent))
	for _, p := range m.recent {
		if p == path {
			continue
		}
		next = append(next, p)
	}
	if len(next) == len(m.recent) {
		return
	}
	if len(next) == 0 {
		next = nil
	}
	m.recent = next
	m.extraGen++
	m.saveTabs()
}

func (m *GitModel) dedupeTabs() {
	seen := make(map[string]int, len(m.tabs))
	next := make([]*gitSession, 0, len(m.tabs))
	active := m.gitSession
	changed := false
	for _, s := range m.tabs {
		if s.path == "" {
			next = append(next, s)
			continue
		}
		key := canonicalPath(s.path)
		if prev, ok := seen[key]; ok {
			changed = true
			if s == active {
				active = next[prev]
			}
			continue
		}
		seen[key] = len(next)
		next = append(next, s)
	}
	if !changed {
		return
	}
	m.tabs = next
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
