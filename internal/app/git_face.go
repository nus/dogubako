package app

import "github.com/guigui-gui/guigui/basicwidget"

// gitFace is the screen last shown for one repository.
// Switching away keeps it. Switching back puts it on the widgets.
type gitFace struct {
	ready          bool
	commitView     bool
	openHash       string
	histFile       string
	histFrac       float64
	refFilter      string
	histQuery      string
	histSearchOpen bool
	graph          basicwidget.ListScrollPosition
	sideX          float64
	sideY          float64
	diffStaged     bool
	unstaged       []string
	staged         []string
	diffSplit      bool
	diffWrap       bool
	diffWhole      bool
	workDiff       gitDiffPlace
	histDiff       gitDiffPlace
}

// gitFileHold is a file selection waiting for the status list.
// An empty list while the repository is still loading must not drop it.
type gitFileHold struct {
	on         bool
	diffStaged bool
	unstaged   []string
	staged     []string
}

// gitDiffPlace is the diff scroll for one repository.
// Zero is the top. A set place is restored when that diff is shown again.
type gitDiffPlace struct {
	set                          bool
	compactX, compactY           float64
	leftX, leftY, rightX, rightY float64
	syncY                        float64
}

// stashFace stores the widgets onto the repository they belong to.
// A second switch in the same frame does not copy that screen onto the
// repository that was only passed through.
func (t *GitTool) stashFace(m *GitModel) {
	if m == nil {
		return
	}
	m.ensure()
	s := m.gitSession
	if s == nil {
		return
	}
	if t.faceSess != nil && t.faceSess != s {
		return
	}
	s.face = t.captureFace()
	t.faceSess = s
}

// adoptFace keeps the screen with the repository that was showing, then
// shows the screen of the repository that is showing now.
func (t *GitTool) adoptFace(m *GitModel) {
	if m == nil {
		return
	}
	m.ensure()
	s := m.gitSession
	if s == nil || t.faceSess == s {
		return
	}
	if t.faceSess != nil {
		t.faceSess.face = t.captureFace()
	}
	t.faceSess = s
	if s.face.ready {
		t.applyFace(m, s.face)
		return
	}
	t.resetFace(m)
}

func (t *GitTool) captureFace() gitFace {
	work := diffPlace(&t.workPanes.diff.view)
	if t.workHold.set {
		work = t.workHold
	}
	hist := diffPlace(&t.histStack.diff.view)
	if t.histHold.set {
		hist = t.histHold
	}
	return gitFace{
		ready:          true,
		commitView:     t.commitView,
		openHash:       t.openHash,
		histFile:       t.histFile,
		histFrac:       t.histFrac,
		refFilter:      t.refFilter,
		histQuery:      t.histQuery,
		histSearchOpen: t.histSearchOpen,
		graph:          t.commitList.ScrollPosition(),
		sideX:          t.sideX,
		sideY:          t.sideY,
		diffStaged:     t.diffStaged,
		unstaged:       append([]string(nil), t.workPanes.unstaged.sel...),
		staged:         append([]string(nil), t.workPanes.staged.sel...),
		diffSplit:      t.diffSplit,
		diffWrap:       t.diffWrap,
		diffWhole:      t.diffWhole,
		workDiff:       work,
		histDiff:       hist,
	}
}

func (t *GitTool) applyFace(m *GitModel, f gitFace) {
	t.commitView = f.commitView
	t.openHash = f.openHash
	t.openShown = ""
	t.histFile = f.histFile
	t.histFor = ""
	t.histFrac = f.histFrac
	t.refFilter = f.refFilter
	t.sidePane.filter.input.ForceSetValue(f.refFilter)
	t.histQuery = f.histQuery
	t.histSearchOpen = f.histSearchOpen
	t.histSearchFocus = false
	t.histSearch.input.ForceSetValue(f.histQuery)
	t.bindHistQuery(m)
	t.diffStaged = f.diffStaged
	t.workPanes.unstaged.sel = append([]string(nil), f.unstaged...)
	t.workPanes.staged.sel = append([]string(nil), f.staged...)
	t.fileHold = gitFileHold{
		on:         true,
		diffStaged: f.diffStaged,
		unstaged:   append([]string(nil), f.unstaged...),
		staged:     append([]string(nil), f.staged...),
	}
	t.diffSplit = f.diffSplit
	t.diffWrap = f.diffWrap
	t.diffWhole = f.diffWhole
	t.faceGraph = f.graph
	t.faceGraphOn = true
	t.sidePane.panel.ForceSetScrollOffset(f.sideX, f.sideY)
	t.sideX, t.sideY = f.sideX, f.sideY
	t.workHold = f.workDiff
	t.histHold = f.histDiff
	t.workPanes.diff.view.keep = gitDiffPlace{}
	t.histStack.diff.view.keep = gitDiffPlace{}
	t.commitSel = ""
}

// resetFace is the screen of a repository that has not been shown before.
// Diff split, wrap, and whole-file stay as they are.
func (t *GitTool) resetFace(m *GitModel) {
	t.commitView = false
	t.openHash = ""
	t.openShown = ""
	t.histFile = ""
	t.histFor = ""
	t.histFrac = 0
	t.refFilter = ""
	t.sidePane.filter.input.ForceSetValue("")
	t.histQuery = ""
	t.histSearchOpen = false
	t.histSearchFocus = false
	t.histJump = ""
	t.histSearch.input.ForceSetValue("")
	t.bindHistQuery(m)
	t.diffStaged = false
	t.workPanes.unstaged.sel = nil
	t.workPanes.staged.sel = nil
	t.fileHold = gitFileHold{}
	t.faceGraph = basicwidget.ListScrollPosition{}
	t.faceGraphOn = true
	t.sidePane.panel.ForceSetScrollOffset(0, 0)
	t.sideX, t.sideY = 0, 0
	t.workHold = gitDiffPlace{}
	t.histHold = gitDiffPlace{}
	t.workPanes.diff.view.keep = gitDiffPlace{}
	t.histStack.diff.view.keep = gitDiffPlace{}
	t.workPanes.diff.view.ResetScroll()
	t.histStack.diff.view.ResetScroll()
	t.commitSel = ""
}

// bindHistQuery marks the history search as belonging to the shown repository,
// so a later sync does not clear the text that was just restored.
func (t *GitTool) bindHistQuery(m *GitModel) {
	t.histQuerySet = true
	t.histQueryTab = m.ActiveTab()
	t.histQueryPath = m.Path()
}

func diffPlace(v *gitDiffView) gitDiffPlace {
	return gitDiffPlace{
		set:      true,
		compactX: v.compact.scrollX,
		compactY: v.compact.scrollY,
		leftX:    v.left.scrollX,
		leftY:    v.left.scrollY,
		rightX:   v.right.scrollX,
		rightY:   v.right.scrollY,
		syncY:    v.leftY,
	}
}

func (v *gitDiffView) restorePlace(p gitDiffPlace) {
	v.compact.scrollX = p.compactX
	v.compact.scrollY = p.compactY
	v.left.scrollX = p.leftX
	v.left.scrollY = p.leftY
	v.right.scrollX = p.rightX
	v.right.scrollY = p.rightY
	v.leftY = p.syncY
	v.rightY = p.syncY
}

func (v *gitDiffView) applyKeep(keep gitDiffPlace) {
	if !keep.set {
		return
	}
	v.restorePlace(keep)
	if v.text != "" {
		v.keep.set = false
	}
}
