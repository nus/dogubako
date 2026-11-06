package app

import (
	"path"
	"slices"
	"strings"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
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
	m.drainSSH()
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

// OpenHere shows path in the current tab.
// A tab that already has path is selected instead. A new tab is not added.
func (m *GitModel) OpenHere(path string) {
	path = canonicalPath(path)
	if path == "" || path == "." {
		return
	}
	m.ensure()
	m.rememberRepo(path)
	if i := m.tabIndex(path); i >= 0 {
		m.SelectTab(i)
		s := m.tabs[i]
		if !s.HasRepo() && !s.Busy() {
			s.open(path)
		}
		return
	}
	if m.Busy() {
		return
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

// TabDisplay is the repository path shown for the tab, matching the side pane.
// Local tabs show the directory. SSH tabs show host:dir, or host:port:dir when
// a port is set. It is empty when the tab has no repository.
func (m *GitModel) TabDisplay(index int) string {
	m.ensure()
	if index < 0 || index >= len(m.tabs) {
		return ""
	}
	repoPath := m.tabs[index].path
	if repoPath == "" {
		return ""
	}
	return gitcli.ParseLoc(repoPath).Display()
}

// TabLabel is the repository folder name, or empty when the tab has no repository.
// Tabs that share a folder name include the parent directory.
func (m *GitModel) TabLabel(index int) string {
	m.ensure()
	if index < 0 || index >= len(m.tabs) {
		return ""
	}
	repoPath := m.tabs[index].path
	if repoPath == "" {
		return ""
	}
	base := repoFolder(repoPath)
	for i, other := range m.tabs {
		if i == index || other.path == "" {
			continue
		}
		if repoFolder(other.path) == base {
			parent := repoParent(repoPath)
			if parent != "" && parent != "." && parent != "/" {
				return parent + "/" + base
			}
			break
		}
	}
	return base
}

func repoFolder(repoPath string) string {
	return path.Base(gitcli.ParseLoc(repoPath).Dir)
}

func repoParent(repoPath string) string {
	return path.Base(path.Dir(gitcli.ParseLoc(repoPath).Dir))
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

// SelectNeighbor shows the tab step places away in tab order.
// A positive step moves right and a negative step moves left. Either end wraps
// to the other end. The order of tabs does not change. One tab, or a zero step,
// stays on the displayed tab. The shown tab is the one a click would show,
// including the tab restored on the next launch.
func (m *GitModel) SelectNeighbor(step int) {
	m.ensure()
	n := len(m.tabs)
	if n <= 1 || step == 0 {
		return
	}
	step %= n
	if step < 0 {
		step += n
	}
	m.SelectTab((m.active + step) % n)
}

// gitTabStep is the neighbor a Ctrl+Tab chord selects.
// Control alone steps right. Control and Shift step left.
// The chord does not step when Tab is not firing, Control is up, or Alt or
// Command is down. Command stays out so macOS Command+Tab remains the app switcher.
func gitTabStep(tab, control, shift, alt, command bool) (step int, ok bool) {
	if !tab || !control || alt || command {
		return 0, false
	}
	if shift {
		return -1, true
	}
	return 1, true
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
		if s.Busy() || s.pendingMore != nil {
			return true
		}
	}
	return false
}

// HasMoreHistory reports that the active repository has older commits to load.
func (m *GitModel) HasMoreHistory() bool {
	m.ensure()
	return m.gitSession.HasMoreHistory()
}

// HistoryLoading reports that the active repository is reading history.
func (m *GitModel) HistoryLoading() bool {
	m.ensure()
	return m.gitSession.HistoryLoading()
}

// LoadMoreHistory fetches the next page of commits for the active repository.
func (m *GitModel) LoadMoreHistory() {
	m.ensure()
	m.gitSession.LoadMoreHistory()
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

// worktreeKey is the tab path for a working tree directory.
// SSH tabs keep the connection of repoPath. A different host is a different key.
func worktreeKey(repoPath, dir string) string {
	loc := gitcli.ParseLoc(repoPath)
	if loc.IsRemote() {
		if dir == "" || dir == "." {
			return ""
		}
		loc.Dir = path.Clean(dir)
		return loc.Key()
	}
	return canonicalPath(dir)
}

// mainWorktree is the main working tree. The bool is false when that tree
// is bare or missing, so it cannot be opened as a tab.
func mainWorktree(trees []gitcli.Worktree) (gitcli.Worktree, bool) {
	for _, wt := range trees {
		if !wt.Main {
			continue
		}
		if wt.Bare || strings.TrimSpace(wt.Path) == "" {
			return wt, false
		}
		return wt, true
	}
	return gitcli.Worktree{}, false
}

// worktreeTabAction is how tabs change after a linked worktree is removed.
// Indexes refer to the tab list before the change. -1 means none.
type worktreeTabAction struct {
	// Close is the tab showing the removed path, when that tab is removed.
	Close int
	// Blank keeps the only tab and clears it, instead of removing it.
	Blank bool
	// Reopen is the tab whose path becomes the parent.
	Reopen int
	// Select is the parent tab to show when the removed path was active
	// and the parent was already open. The index is before Close.
	Select int
}

// planWorktreeTabs decides where the removed path's tab goes.
// parentOpen is false when the parent is bare and must not be opened.
// Tabs on a different connection are left alone because their keys differ.
func planWorktreeTabs(paths []string, active int, removed, parent string, parentOpen bool) worktreeTabAction {
	action := worktreeTabAction{Close: -1, Reopen: -1, Select: -1}
	idx := -1
	for i, p := range paths {
		if p != "" && p == removed {
			idx = i
			break
		}
	}
	if idx < 0 {
		return action
	}
	if parentOpen && parent != "" && parent != removed {
		for i, p := range paths {
			if i != idx && p == parent {
				action.Close = idx
				if active == idx {
					action.Select = i
				}
				return action
			}
		}
		action.Reopen = idx
		return action
	}
	if len(paths) == 1 {
		action.Blank = true
		action.Close = idx
		return action
	}
	action.Close = idx
	return action
}

func (m *GitModel) tabPaths() []string {
	paths := make([]string, len(m.tabs))
	for i, s := range m.tabs {
		paths[i] = canonicalPath(s.path)
	}
	return paths
}

func (m *GitModel) hasSession(s *gitSession) bool {
	for _, tab := range m.tabs {
		if tab == s {
			return true
		}
	}
	return false
}

// becomeEmpty drops the repository and keeps the console, so the last tab
// can show the command that closed its worktree.
func (s *gitSession) becomeEmpty() {
	s.path = ""
	s.repo = nil
	s.snap = gitcli.Snapshot{}
	s.hidden = nil
	s.sel = ""
	s.detail = gitcli.CommitDetail{}
	s.draft = ""
	s.stageAll = false
	s.pendingLoad = nil
	s.pendingMore = nil
	s.pendingOp = nil
	s.pendingDetail = nil
	s.pendingStatus = nil
	s.histLimit = 0
	s.histMore = false
	s.histGen++
	s.reveal = ""
	s.workSlots = [2]gitWorkSlot{}
	s.wholeSlots = [2]gitWorkSlot{}
	s.commitWhole = gitWorkSlot{}
	s.graphRev++
	s.generation++
}

// finishCloseWorktree moves tabs off a worktree that was just removed.
// The status line is set on the tab the user is left looking at.
func (m *GitModel) finishCloseWorktree(from *gitSession, removedKey, parentKey string, parentOpen bool, okKey i18n.Key) {
	m.ensure()
	removedKey = canonicalPath(removedKey)
	if parentKey != "" {
		parentKey = canonicalPath(parentKey)
	}
	action := planWorktreeTabs(m.tabPaths(), m.active, removedKey, parentKey, parentOpen)
	show := from
	switch {
	case action.Blank:
		m.tabs[0].becomeEmpty()
		m.active = 0
		m.gitSession = m.tabs[0]
		m.extraGen++
		m.saveTabs()
		show = m.tabs[0]
	case action.Reopen >= 0 && action.Reopen < len(m.tabs):
		s := m.tabs[action.Reopen]
		s.openPath(parentKey, true)
		show = s
		if m.active != action.Reopen && from != nil && m.hasSession(from) {
			show = from
		}
	case action.Close >= 0:
		wasActive := m.active == action.Close
		selectIdx := action.Select
		if wasActive && from != nil {
			dest := -1
			if action.Select >= 0 {
				dest = action.Select
			} else if len(m.tabs) > 1 {
				dest = action.Close
				if dest >= len(m.tabs)-1 {
					dest = action.Close - 1
				}
			}
			if dest >= 0 && dest < len(m.tabs) && m.tabs[dest] != from {
				m.tabs[dest].console.takeLast(&from.console)
			}
		}
		m.CloseTab(action.Close)
		if selectIdx >= 0 {
			if selectIdx > action.Close {
				selectIdx--
			}
			m.SelectTab(selectIdx)
		}
		if wasActive || action.Select >= 0 {
			show = m.gitSession
		} else if from != nil && m.hasSession(from) {
			show = from
		}
	}
	if show == nil {
		guigui.RequestRebuild()
		return
	}
	show.SetStatus(okKey)
	if show.HasRepo() && show.pendingLoad == nil && show.pendingOp == nil && canonicalPath(show.path) != removedKey {
		show.refresh()
	}
	guigui.RequestRebuild()
}

// worktreeRemoveRepo is the repository that runs git worktree remove.
// The command is traced on this session. When this session is the tree being
// removed, the parent directory runs it and the trace stays here.
func (m *gitSession) worktreeRemoveRepo(targetDir, parentKey string, parentOpen bool) *gitcli.Repo {
	if m.repo == nil {
		return nil
	}
	if !repoAtDir(m.repo, targetDir) {
		return m.repo
	}
	if parentOpen {
		if runner := m.parentRunner(parentKey, targetDir); runner != nil {
			return runner
		}
	}
	return m.repo
}

func (m *gitSession) parentRunner(parentKey, targetDir string) *gitcli.Repo {
	if parentKey == "" {
		return nil
	}
	loc := gitcli.ParseLoc(parentKey)
	if !loc.IsRemote() {
		loc.Dir = parentKey
	}
	if strings.TrimSpace(loc.Dir) == "" || repoAtDir(&gitcli.Repo{Dir: loc.Dir, Dest: loc.Dest, Port: loc.Port}, targetDir) {
		return nil
	}
	runner := &gitcli.Repo{Dir: loc.Dir, Dest: loc.Dest, Port: loc.Port}
	runner.SetCommandTrace(m.console.onTrace)
	return runner
}

func repoAtDir(repo *gitcli.Repo, dir string) bool {
	if repo == nil || strings.TrimSpace(dir) == "" {
		return false
	}
	if repo.IsRemote() {
		return path.Clean(repo.Dir) == path.Clean(dir)
	}
	return canonicalPath(repo.Dir) == canonicalPath(dir)
}
