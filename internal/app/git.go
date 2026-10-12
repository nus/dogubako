package app

import (
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/userdir"
)

var eventGitOpen = guigui.GenerateEventKey()

const (
	gitWorkspaceWorking = "working"
	gitWorkspaceHistory = "history"
)

// GitTool is a Git client: graph, branches, and history operations.
// Repositories may be local or on an SSH host.
type GitTool struct {
	guigui.DefaultWidget

	tabBar gitTabBar

	openBtn    basicwidget.Button
	pathLabel  basicwidget.Text
	fetchBtn   basicwidget.Button
	pullBtn    basicwidget.Button
	pushBtn    basicwidget.Button
	commitBtn  basicwidget.Button
	amendBtn   basicwidget.Button
	commitView bool

	workspaceLabel basicwidget.Text
	workspaceList  basicwidget.List[string]
	workspaceItems []basicwidget.ListItem[string]
	workRow        gitWorkspaceRow
	histRow        gitWorkspaceRow

	branchItems  []basicwidget.ListItem[string]
	branchFold   map[string]bool
	branchClosed bool
	tagItems     []basicwidget.ListItem[string]
	showTags     bool
	tagClosed    bool
	tagClicks    gitClickCount
	tagClick     string
	remoteItems  []basicwidget.ListItem[string]
	remoteFold   map[string]bool
	remoteClosed bool
	navSel       string

	commitList     basicwidget.List[string]
	commitRows     guigui.WidgetSlice[*gitCommitRow]
	commitItems    []basicwidget.ListItem[string]
	graphEmpty     basicwidget.Text
	showEmpty      bool
	openHash       string
	openShown      string
	openFromSelect bool
	building       bool
	expand         gitCommitExpand
	histFile       string
	histFor        string
	histFrac       float64
	histDragging   bool
	histBounds     image.Rectangle
	histGraph      image.Rectangle
	histStack      gitHistStack
	histSpin       gitSpin
	showHistSpin   bool
	// historyEndOn means End already asked for the next history page.
	// The pager waits until the user scrolls away or asks again.
	historyEndOn   bool
	historyEndSeen bool
	historyEndPath string
	// graphUnits is the commit-graph column width in unit sizes.
	// Zero uses the width that fits the lanes. A dragged width is restored next launch.
	graphUnits        float64
	graphWidthLoaded  bool
	graphDragging     bool
	graphLanes        int
	graphSplitBounds  image.Rectangle
	authorUnits       float64
	hashUnits         float64
	dateUnits         float64
	metaWidthLoaded   bool
	metaDrag          int
	authorSplitBounds image.Rectangle
	hashSplitBounds   image.Rectangle
	dateSplitBounds   image.Rectangle

	emptyHome   bool
	showRecent  bool
	recentTitle basicwidget.Text
	recentList  basicwidget.List[string]
	recentRows  guigui.WidgetSlice[*gitRecentRow]
	recentItems []basicwidget.ListItem[string]

	sshPick       bool
	sshBtn        basicwidget.Button
	sshHostsTitle basicwidget.Text
	sshHostList   basicwidget.List[string]
	sshHostItems  []basicwidget.ListItem[string]
	sshNoHosts    basicwidget.Text
	sshShowHosts  bool
	sshDestLabel  basicwidget.Text
	sshDestInput  basicwidget.TextInput
	sshPortLabel  basicwidget.Text
	sshPortInput  basicwidget.TextInput
	sshConnectBtn basicwidget.Button
	sshCancelBtn  basicwidget.Button
	sshTarget     basicwidget.Text
	sshPathInput  basicwidget.TextInput
	sshUpBtn      basicwidget.Button
	sshDirList    basicwidget.List[string]
	sshDirItems   []basicwidget.ListItem[string]
	sshOpenDirBtn basicwidget.Button
	sshBackBtn    basicwidget.Button
	sshStatus     basicwidget.Text
	sshStatusOn   bool
	sshWas        bool
	sshSeenGen    uint64
	sshDirShown   string
	sshConnected  bool

	workTree     basicwidget.Text
	showWorkTree bool

	stageBtn    basicwidget.Button
	unstageBtn  basicwidget.Button
	showChanges bool
	diffStaged  bool
	diffSplit   bool
	workPanes   gitWorkPanes
	msgInput    guigui.WidgetWithSize[*basicwidget.TextInput]
	status      basicwidget.Text
	hint        basicwidget.Text

	branchMenu gitBranchMenu
	commitMenu gitCommitMenu
	tagMenu    gitTagMenu
	selMenu    gitSelectionMenu
	confirm    gitConfirm
	rename     gitRename

	sidePane  gitSidePane
	sideSplit gitSideSplit
	// sideUnits is the left column width in unit sizes. Zero uses the default.
	sideUnits    float64
	sideDragging bool
	bodyBounds   image.Rectangle

	rightItems []guigui.LinearLayoutItem
	msgItems   []guigui.LinearLayoutItem
	netItems   []guigui.LinearLayoutItem
	rightCol   guigui.LinearLayout
	msgRow     guigui.LinearLayout
	netRow     guigui.LinearLayout
}

func (t *GitTool) OnOpen(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventGitOpen, f)
}

func (t *GitTool) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return
	}
	m := v.(*Model).Git()
	w.WriteUint64(m.Generation())
	w.WriteString(m.Path())
	w.WriteString(m.Selected())
	w.WriteBool(m.Busy())
	w.WriteBool(m.HistoryLoading())
	w.WriteBool(m.HasMoreHistory())
	w.WriteBool(t.commitView)
	w.WriteString(t.openHash)
	w.WriteString(t.histFile)
}

func (t *GitTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	t.building = true
	t.showHistSpin = false
	defer func() { t.building = false }()
	t.ensureGraphWidth()
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.Git()
	lang := appModel.Lang()
	model.EnsureLoaded()
	busy := model.Busy()
	has := model.HasRepo()
	onMenu := func(ref gitcli.Ref) { t.openBranchMenu(lang, model, ref) }

	if t.sideDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.sideDragging = false
		t.setSideDragPassthrough(context, false)
	}
	if t.histDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.histDragging = false
		t.setHistDragPassthrough(context, false)
	}
	if t.graphDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.stopGraphDrag(context)
	}
	if t.metaDrag != 0 && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.metaDrag = 0
		t.setGraphDragPassthrough(context, false)
	}
	t.tabBar.Sync(lang, model)
	adder.AddWidget(&t.tabBar)
	t.emptyHome = model.Path() == ""
	t.sshPick = t.emptyHome && model.SSHActive()
	if t.sshPick {
		return t.buildSSH(context, adder, lang, model)
	}
	t.sshWas = false
	t.sshDirShown = ""
	if t.emptyHome {
		return t.buildEmptyHome(context, adder, lang, model)
	}

	if !t.commitView {
		adder.AddWidget(&t.fetchBtn)
		adder.AddWidget(&t.pullBtn)
		adder.AddWidget(&t.pushBtn)
	}
	adder.AddWidget(&t.status)
	adder.AddWidget(&t.hint)
	t.sidePane.setOwner(t)
	adder.AddWidget(&t.sidePane)
	t.sideSplit.tool = t
	adder.AddWidget(&t.sideSplit)
	t.showWorkTree = model.HasRepo() && model.Snapshot().Status.Dirty()
	t.showChanges = model.Snapshot().Status.Dirty()
	if t.commitView {
		if !t.showWorkTree {
			t.workTree.SetValue(i18n.T(lang, i18n.GitNoChanges))
			t.workTree.SetMultiline(true)
			t.workTree.SetWrapMode(basicwidget.WrapModeNormal)
			t.workTree.SetSelectable(false)
		}
		if t.showChanges {
			adder.AddWidget(&t.workPanes)
		} else {
			adder.AddWidget(&t.workTree)
		}
		adder.AddWidget(&t.msgInput)
		adder.AddWidget(&t.stageBtn)
		adder.AddWidget(&t.unstageBtn)
		adder.AddWidget(&t.commitBtn)
		adder.AddWidget(&t.amendBtn)
	}

	repoPath := model.Path()
	loc := gitcli.ParseLoc(repoPath)
	shown := loc.Display()
	t.pathLabel.SetValue(shown)
	t.pathLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.pathLabel.SetWrapMode(basicwidget.WrapModeNone)
	t.pathLabel.SetSelectable(false)
	if repoPath == "" || loc.IsRemote() {
		t.pathLabel.SetHotspotRanges(nil)
	} else {
		t.pathLabel.SetHotspotRanges([]basicwidget.TextRange{{EndInBytes: len(shown)}})
		t.pathLabel.OnHotspotUp(func(*guigui.Context, basicwidget.TextRange) {
			if err := userdir.OpenInFileManager(repoPath); err != nil {
				model.SetStatus(i18n.StatusFolderOpenFailed, err)
				guigui.RequestRebuild()
			}
		})
	}

	setBoldText(&t.workspaceLabel, true)
	t.workspaceLabel.SetValue(i18n.T(lang, i18n.GitWorkspace))
	t.workRow.Set(i18n.T(lang, i18n.GitWorkingCopy), gitWorkspaceIconChanges, t.commitView)
	t.histRow.Set(i18n.T(lang, i18n.GitHistory), gitWorkspaceIconHistory, !t.commitView)
	t.workspaceItems = slices.Delete(t.workspaceItems, 0, len(t.workspaceItems))
	t.workspaceItems = append(t.workspaceItems,
		basicwidget.ListItem[string]{Content: &t.workRow, Value: gitWorkspaceWorking},
		basicwidget.ListItem[string]{Content: &t.histRow, Value: gitWorkspaceHistory},
	)
	t.workspaceList.SetStyle(basicwidget.ListStyleSidebar)
	t.workspaceList.SetItemHeight(basicwidget.UnitSize(context))
	t.workspaceList.SetItems(t.workspaceItems)
	if t.commitView {
		t.workspaceList.SelectItemByValue(gitWorkspaceWorking)
	} else {
		t.workspaceList.SelectItemByValue(gitWorkspaceHistory)
	}
	t.workspaceList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.workspaceList.ItemByIndex(index)
		if !ok {
			return
		}
		commit := item.Value == gitWorkspaceWorking
		if commit == t.commitView {
			return
		}
		t.commitView = commit
		guigui.RequestRebuild()
	})

	snap := model.Snapshot()
	enableNet := model.CanPushPull()
	t.fetchBtn.SetText(i18n.T(lang, i18n.GitFetch))
	t.fetchBtn.OnDown(func(context *guigui.Context) { model.DoFetch() })
	context.SetEnabled(&t.fetchBtn, enableNet)
	t.pullBtn.SetText(i18n.T(lang, i18n.GitPull))
	t.pullBtn.OnDown(func(context *guigui.Context) { model.DoPull() })
	context.SetEnabled(&t.pullBtn, enableNet)
	t.pushBtn.SetText(i18n.T(lang, i18n.GitPush))
	t.pushBtn.OnDown(func(context *guigui.Context) { model.DoPush() })
	context.SetEnabled(&t.pushBtn, enableNet)

	t.commitBtn.SetText(i18n.T(lang, i18n.GitCommit))
	t.commitBtn.SetType(basicwidget.ButtonTypePrimary)
	t.commitBtn.OnDown(func(context *guigui.Context) { model.DoCommit() })
	context.SetEnabled(&t.commitBtn, model.CanCommit())
	t.amendBtn.SetText(i18n.T(lang, i18n.GitAmend))
	t.amendBtn.OnDown(func(context *guigui.Context) { model.DoAmend() })
	context.SetEnabled(&t.amendBtn, model.CanAmend())

	var mono basicwidget.TextStyle
	useGitMono(&mono)
	monoItem := basicwidget.ItemTextStyle{Style: mono}

	branchEntries := make([]gitTreeEntry, 0, len(snap.Locals))
	for _, ref := range snap.Locals {
		branchEntries = append(branchEntries, gitTreeEntry{
			Name:    ref.Name,
			Value:   "local:" + ref.Name,
			Current: ref.Current,
		})
	}
	t.branchItems = gitTreeItems(branchEntries, t.branchFold)
	for i := range t.branchItems {
		t.branchItems[i].TextStyle = monoItem
	}

	t.tagItems = slices.Delete(t.tagItems, 0, len(t.tagItems))
	for _, ref := range snap.Tags {
		t.tagItems = append(t.tagItems, basicwidget.ListItem[string]{
			Text:      ref.Name,
			TextStyle: monoItem,
			Value:     "tag:" + ref.Name,
		})
	}
	t.showTags = len(t.tagItems) > 0

	remoteEntries := make([]gitTreeEntry, 0, len(snap.RemoteBranches))
	for _, ref := range snap.RemoteBranches {
		remoteEntries = append(remoteEntries, gitTreeEntry{
			Name:  ref.Remote + "/" + ref.LocalName(),
			Value: "remote:" + ref.Name,
		})
	}
	t.remoteItems = gitTreeItems(remoteEntries, t.remoteFold)
	for i := range t.remoteItems {
		t.remoteItems[i].TextStyle = monoItem
	}

	rows := snap.Graph
	t.showEmpty = !has || len(rows) == 0
	t.graphLanes = 0
	if !t.commitView && t.showEmpty {
		t.graphEmpty.SetMultiline(true)
		t.graphEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.graphEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.graphEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		if model.HistoryLoading() {
			t.graphEmpty.SetValue("")
		} else if !has {
			t.graphEmpty.SetValue(i18n.T(lang, i18n.GitEmpty))
		} else {
			t.graphEmpty.SetValue(i18n.T(lang, i18n.GitNoCommits))
		}
		var emptyStyle basicwidget.TextStyle
		useGitMono(&emptyStyle)
		t.graphEmpty.SetBaseStyle(&emptyStyle)
		t.histFile = ""
		t.histFor = ""
		t.commitList.SetHeaderHeight(0)
		t.histStack.tool = t
		if model.HistoryLoading() {
			t.histStack.graph = &t.histSpin
		} else {
			t.histStack.graph = &t.graphEmpty
		}
		t.histStack.showDiff = false
		adder.AddWidget(&t.histStack)
	} else if !t.commitView {
		if t.openHash != "" {
			found := false
			for _, row := range rows {
				if row.Commit.Hash == t.openHash {
					found = true
					break
				}
			}
			if !found {
				t.openHash = ""
			}
		}
		t.syncHistFile()
		t.commitRows.SetLen(len(rows))
		t.commitItems = slices.Delete(t.commitItems, 0, len(t.commitItems))
		lanes := snap.Lanes
		if lanes < 1 {
			lanes = 1
		}
		t.graphLanes = lanes
		for i, row := range rows {
			r := t.commitRows.At(i)
			r.tool = t
			r.Set(row, lanes, row.Commit.Hash == snap.HEAD, lang, func(ref gitcli.Ref) {
				t.switchBranch(lang, model, ref)
			}, onMenu)
			r.graphUnits = t.graphUnits
			r.authorUnits = t.authorUnits
			r.hashUnits = t.hashUnits
			r.dateUnits = t.dateUnits
			t.commitItems = append(t.commitItems, basicwidget.ListItem[string]{
				Content: r,
				Value:   row.Commit.Hash,
			})
			if row.Commit.Hash == t.openHash {
				opened := row
				t.expand.tool = t
				t.expand.Set(lang, model, opened, lanes, t.histFile, func(hash string) {
					t.openHash = hash
					t.revealCommit(model, hash)
				}, func(path string) {
					t.histFile = path
					t.histFor = t.openHash
					guigui.RequestRebuild()
				})
				t.expand.graphUnits = t.graphUnits
				t.commitItems = append(t.commitItems, basicwidget.ListItem[string]{
					Content:      &t.expand,
					Unselectable: true,
				})
			}
		}
		t.commitList.SetStyle(basicwidget.ListStyleNormal)
		t.commitList.SetHighlightVisibleWhenUnfocused(true)
		t.commitList.SetHeaderHeight(gitCommitHeaderHeight(basicwidget.UnitSize(context)))
		t.commitList.SetItems(t.commitItems)
		t.histStack.header.SetLang(lang)
		if sel := model.Selected(); sel != "" {
			t.commitList.SelectItemByValue(sel)
		}
		if reveal := model.TakeReveal(); reveal != "" {
			if idx := t.commitList.IndexByValue(reveal); idx >= 0 {
				t.commitList.EnsureItemVisibleByIndex(idx)
			}
		}
		if t.openHash != t.openShown {
			t.openShown = t.openHash
			if t.openHash != "" {
				if idx := t.commitList.IndexByValue(t.openHash); idx >= 0 && idx+1 < t.commitList.ItemCount() {
					t.commitList.EnsureItemVisibleByIndex(idx + 1)
				}
			}
		}
		context.SetClipChildren(&t.commitList, true)
		t.histStack.tool = t
		t.histStack.graph = &t.commitList
		t.histStack.showDiff = t.histFile != ""
		if t.histStack.showDiff {
			t.setHistDiff(lang, model)
		}
		adder.AddWidget(&t.histStack)
		t.commitList.OnItemSelected(func(context *guigui.Context, index int) {
			item, ok := t.commitList.ItemByIndex(index)
			if !ok || item.Value == "" {
				return
			}
			model.SelectCommit(item.Value)
			if t.building {
				return
			}
			arrow := graphKeyRepeating(ebiten.KeyUp) || graphKeyRepeating(ebiten.KeyDown)
			next := commitOpenAfterSelect(t.openHash, item.Value, arrow)
			if next == t.openHash {
				return
			}
			t.openHash = next
			if next != "" && !arrow {
				t.openFromSelect = true
			}
		})
		context.SetEnabled(&t.commitList, !busy)
		if model.HistoryLoading() {
			t.showHistSpin = true
			context.SetPassthrough(&t.histSpin, true)
			adder.AddWidget(&t.histSpin)
		}
	}

	t.setChangeList(context, lang, model, snap.Status.Entries, has && !busy)

	msg := t.msgInput.Widget()
	msg.SetMultiline(true)
	msg.SetPlaceholder(i18n.T(lang, i18n.GitMessage))
	msg.SetBaseStyle(&mono)
	msg.SetValue(model.Draft())
	msg.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		model.SetDraft(text)
	})
	context.SetEnabled(&t.msgInput, has && !busy)

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.hint.SetValue(i18n.T(lang, i18n.GitHint))
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	adder.AddWidget(&t.branchMenu)
	adder.AddWidget(&t.commitMenu)
	adder.AddWidget(&t.tagMenu)
	adder.AddWidget(&t.selMenu)
	adder.AddWidget(&t.confirm)
	adder.AddWidget(&t.rename)
	return nil
}

func (t *GitTool) setChangeList(context *guigui.Context, lang i18n.Lang, model *GitModel, entries []gitcli.StatusEntry, enabled bool) {
	var unstagedEntries, stagedEntries []gitcli.StatusEntry
	for _, e := range entries {
		if e.Unstaged() {
			unstagedEntries = append(unstagedEntries, e)
		}
		if e.Staged() {
			stagedEntries = append(stagedEntries, e)
		}
	}
	onCheck := func(paths []string, checked bool) {
		if checked {
			model.DoStage(paths)
			return
		}
		model.DoUnstage(paths)
	}
	t.workPanes.unstaged.setFiles(context, unstagedEntries, false, enabled, !t.diffStaged || len(stagedEntries) == 0, func() { t.diffStaged = false }, onCheck)
	t.workPanes.staged.setFiles(context, stagedEntries, true, enabled, t.diffStaged || len(unstagedEntries) == 0, func() { t.diffStaged = true }, onCheck)

	stagePaths := append([]string(nil), t.workPanes.unstaged.sel...)
	unstagePaths := append([]string(nil), t.workPanes.staged.sel...)
	t.stageBtn.SetText(i18n.T(lang, i18n.GitStage))
	t.stageBtn.OnDown(func(context *guigui.Context) {
		if _, staged := t.activeWorkTarget(); !staged {
			if patch, ok, selected := t.linePatch(false); selected {
				if ok {
					model.DoApplyPatch(patch, false)
				}
				return
			}
		}
		model.DoStage(stagePaths)
	})
	context.SetEnabled(&t.stageBtn, enabled && len(stagePaths) > 0)
	t.unstageBtn.SetText(i18n.T(lang, i18n.GitUnstage))
	t.unstageBtn.OnDown(func(context *guigui.Context) {
		if _, staged := t.activeWorkTarget(); staged {
			if patch, ok, selected := t.linePatch(true); selected {
				if ok {
					model.DoApplyPatch(patch, true)
				}
				return
			}
		}
		model.DoUnstage(unstagePaths)
	})
	context.SetEnabled(&t.unstageBtn, enabled && len(unstagePaths) > 0)
	t.setWorkDiff(lang, model)
}

func samePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	n := make(map[string]int, len(a))
	for _, p := range a {
		n[p]++
	}
	for _, p := range b {
		n[p]--
		if n[p] < 0 {
			return false
		}
	}
	return true
}

func (t *GitTool) setTreeFold(folded *map[string]bool, value string, expanded bool) {
	path, ok := strings.CutPrefix(value, gitTreeFolderPrefix)
	if !ok || path == "" {
		return
	}
	if *folded == nil {
		*folded = map[string]bool{}
	}
	if expanded {
		delete(*folded, path)
	} else {
		(*folded)[path] = true
	}
	guigui.RequestRebuild()
}

func (t *GitTool) selectRef(model *GitModel, value string) {
	kind, name, ok := strings.Cut(value, ":")
	if !ok || name == "" {
		return
	}
	snap := model.Snapshot()
	if kind == "local" {
		for _, ref := range snap.Locals {
			if ref.Name == name {
				t.revealCommit(model, ref.Hash)
				return
			}
		}
		return
	}
	if kind == "tag" {
		for _, ref := range snap.Tags {
			if ref.Name == name {
				t.revealCommit(model, ref.Hash)
				return
			}
		}
		return
	}
	for _, ref := range snap.RemoteBranches {
		if ref.Name == name {
			t.revealCommit(model, ref.Hash)
			return
		}
	}
}

// revealCommit shows History and scrolls the graph to hash.
func (t *GitTool) revealCommit(model *GitModel, hash string) {
	t.commitView = false
	model.RevealCommit(hash)
}

func (t *GitTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	t.ensureGraphWidth()
	t.graphSplitBounds = image.Rectangle{}
	t.authorSplitBounds = image.Rectangle{}
	t.hashSplitBounds = image.Rectangle{}
	t.dateSplitBounds = image.Rectangle{}
	u := basicwidget.UnitSize(context)
	bounds := widgetBounds.Bounds()
	barH := gitTabBarHeight(u)
	layouter.LayoutWidget(&t.tabBar, image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+barH))
	rest := bounds
	rest.Min.Y += barH
	if t.sshPick {
		t.layoutSSH(context, rest, layouter, u)
		t.layoutOverlays(context, layouter)
		return
	}
	if t.emptyHome {
		t.layoutEmptyHome(context, rest, layouter, u)
		t.layoutOverlays(context, layouter)
		return
	}

	if t.commitView {
		t.layoutCommitColumn(u)
	} else {
		t.layoutHistoryColumn(u)
	}

	inner := image.Rect(rest.Min.X+u/2, rest.Min.Y+u/4, rest.Max.X-u/2, rest.Max.Y-u/2)
	if inner.Max.X < inner.Min.X {
		inner.Max.X = inner.Min.X
	}
	if inner.Max.Y < inner.Min.Y {
		inner.Max.Y = inner.Min.Y
	}
	t.bodyBounds = inner
	splitW := gitWorkSplitWidth(u)
	pref := gitSideDefaultUnits * u
	if t.sideUnits > 0 {
		pref = int(t.sideUnits*float64(u) + 0.5)
	}
	sideW := gitSideWidth(inner.Dx(), splitW, pref, 6*u, 16*u)
	sideR := image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+sideW, inner.Max.Y)
	splitR := image.Rect(sideR.Max.X, inner.Min.Y, sideR.Max.X+splitW, inner.Max.Y)
	rightR := image.Rect(splitR.Max.X, inner.Min.Y, inner.Max.X, inner.Max.Y)
	layouter.LayoutWidget(&t.sidePane, sideR)
	layouter.LayoutWidget(&t.sideSplit, splitR)
	t.rightCol.LayoutWidgets(context, rightR, layouter)
	t.layoutOverlays(context, layouter)
}

// moveOpenCommit selects the next commit in the graph.
// The open detail panel is its own list row, so the list's arrow keys stop on it.
// down moves toward older commits. The key is consumed even at either end.
func (t *GitTool) moveOpenCommit(down bool) bool {
	if t.commitView || t.showEmpty || t.emptyHome || t.sshPick {
		return false
	}
	if t.rename.IsOpen() || t.confirm.IsOpen() || t.tagMenu.IsOpen() || t.branchMenu.IsOpen() || t.commitMenu.IsOpen() || t.selMenu.IsOpen() {
		return false
	}
	n := t.commitList.ItemCount()
	if n == 0 {
		return false
	}
	next := nextGraphCommit(n, func(i int) bool {
		item, ok := t.commitList.ItemByIndex(i)
		return ok && item.Value != "" && !item.Unselectable
	}, t.commitList.SelectedItemIndex(), down)
	if next < 0 {
		return true
	}
	if next != t.commitList.SelectedItemIndex() {
		t.commitList.SelectItemByIndex(next)
		t.commitList.EnsureItemVisibleByIndex(next)
	}
	return true
}

// nextGraphCommit is the next commit row. The detail panel is skipped.
// from < 0 starts from either end. The current index is returned when the
// search reaches the end of the list.
func nextGraphCommit(n int, commit func(int) bool, from int, down bool) int {
	if n <= 0 {
		return -1
	}
	if from < 0 || from >= n {
		if down {
			for i := 0; i < n; i++ {
				if commit(i) {
					return i
				}
			}
		} else {
			for i := n - 1; i >= 0; i-- {
				if commit(i) {
					return i
				}
			}
		}
		return -1
	}
	step := -1
	if down {
		step = 1
	}
	for i := from + step; i >= 0 && i < n; i += step {
		if commit(i) {
			return i
		}
	}
	return from
}

func (t *GitTool) syncHistFile() {
	if t.histFor == t.openHash {
		return
	}
	t.histFile = ""
	t.histFor = t.openHash
}

// commitOpenAfterSelect is the commit whose detail stays open after selecting hash.
// Arrow keys keep the panel open only when it was already open, and leave it
// closed otherwise. A click opens the commit. The uncommitted row has no panel.
func commitOpenAfterSelect(openHash, hash string, arrow bool) string {
	if hash == "" || hash == gitcli.Uncommitted {
		return ""
	}
	if arrow && openHash == "" {
		return ""
	}
	return hash
}

func (t *GitTool) layoutHistoryColumn(u int) {
	t.netItems = slices.Delete(t.netItems, 0, len(t.netItems))
	t.netItems = append(t.netItems,
		guigui.LinearLayoutItem{Widget: &t.fetchBtn},
		guigui.LinearLayoutItem{Widget: &t.pullBtn},
		guigui.LinearLayoutItem{Widget: &t.pushBtn},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
	)
	t.netRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.netItems, Gap: u / 4}

	t.rightItems = slices.Delete(t.rightItems, 0, len(t.rightItems))
	t.rightItems = append(t.rightItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.netRow},
		guigui.LinearLayoutItem{Widget: &t.histStack, Size: guigui.FlexibleSize(1)},
	)
	t.rightItems = append(t.rightItems,
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u * 3 / 4)},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FixedSize(u * 3 / 4)},
	)
	t.rightCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.rightItems, Gap: u / 6}
}

func (t *GitTool) layoutCommitColumn(u int) {
	t.msgInput.SetIntrinsicSize()
	t.msgItems = slices.Delete(t.msgItems, 0, len(t.msgItems))
	t.msgItems = append(t.msgItems,
		guigui.LinearLayoutItem{Widget: &t.stageBtn},
		guigui.LinearLayoutItem{Widget: &t.unstageBtn},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.amendBtn},
		guigui.LinearLayoutItem{Widget: &t.commitBtn},
	)
	t.msgRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.msgItems, Gap: u / 4}

	t.rightItems = slices.Delete(t.rightItems, 0, len(t.rightItems))
	if t.showChanges {
		t.rightItems = append(t.rightItems, guigui.LinearLayoutItem{Widget: &t.workPanes, Size: guigui.FlexibleSize(3)})
	} else {
		t.rightItems = append(t.rightItems, guigui.LinearLayoutItem{Widget: &t.workTree, Size: guigui.FlexibleSize(3)})
	}
	t.rightItems = append(t.rightItems,
		guigui.LinearLayoutItem{Widget: &t.msgInput, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.msgRow},
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u * 3 / 4)},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FixedSize(u * 3 / 4)},
	)
	t.rightCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.rightItems, Gap: u / 6}
}

func (t *GitTool) layoutOverlays(context *guigui.Context, layouter *guigui.ChildLayouter) {
	if t.branchMenu.IsOpen() {
		s := t.branchMenu.contentSize(context)
		p := t.branchMenu.pos
		layouter.LayoutWidget(&t.branchMenu, image.Rectangle{Min: p, Max: p.Add(s)})
	}
	if t.commitMenu.IsOpen() {
		s := t.commitMenu.contentSize(context)
		p := t.commitMenu.pos
		layouter.LayoutWidget(&t.commitMenu, image.Rectangle{Min: p, Max: p.Add(s)})
	}
	if t.selMenu.IsOpen() {
		s := t.selMenu.contentSize(context)
		p := t.selMenu.pos
		layouter.LayoutWidget(&t.selMenu, image.Rectangle{Min: p, Max: p.Add(s)})
	}
	if t.tagMenu.IsOpen() {
		s := t.tagMenu.contentSize(context)
		p := t.tagMenu.pos
		layouter.LayoutWidget(&t.tagMenu, image.Rectangle{Min: p, Max: p.Add(s)})
	}
	center := func(widget guigui.Widget, s image.Point) {
		app := context.AppBounds()
		p := image.Pt(app.Min.X+(app.Dx()-s.X)/2, app.Min.Y+(app.Dy()-s.Y)/2)
		layouter.LayoutWidget(widget, image.Rectangle{Min: p, Max: p.Add(s)})
	}
	if t.confirm.IsOpen() {
		center(&t.confirm, t.confirm.contentSize(context))
	}
	if t.rename.IsOpen() {
		center(&t.rename, t.rename.contentSize(context))
	}
	if t.showHistSpin && !t.histGraph.Empty() {
		layouter.LayoutWidget(&t.histSpin, t.histSpinBounds(basicwidget.UnitSize(context)))
	}
}

func (t *GitTool) histSpinBounds(u int) image.Rectangle {
	g := t.histGraph
	size := u * 2
	if size < 24 {
		size = 24
	}
	x := g.Min.X + (g.Dx()-size)/2
	y := g.Max.Y - size - u/5
	if y < g.Min.Y {
		y = g.Min.Y
	}
	if x < g.Min.X {
		x = g.Min.X
	}
	return image.Rect(x, y, x+size, y+size)
}

func (t *GitTool) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if v, ok := context.Env(t, EnvKeyModel); ok {
		t.maybeLoadMoreHistory(v.(*Model).Git())
	}
	openedBySelect := t.openFromSelect
	t.openFromSelect = false
	if t.sideDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.sideDragging = false
		t.setSideDragPassthrough(context, false)
		guigui.RequestRebuild()
	}
	if t.histDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.histDragging = false
		t.setHistDragPassthrough(context, false)
		guigui.RequestRebuild()
	}
	if t.graphDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.stopGraphDrag(context)
		guigui.RequestRebuild()
	}
	if t.metaDrag != 0 && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.metaDrag = 0
		t.setGraphDragPassthrough(context, false)
		guigui.RequestRebuild()
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	if t.rename.IsOpen() || t.confirm.IsOpen() || t.tagMenu.IsOpen() || t.branchMenu.IsOpen() || t.commitMenu.IsOpen() || t.selMenu.IsOpen() {
		return nil
	}
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model).Git()
	if t.sshPick {
		return nil
	}
	if t.emptyHome {
		t.openRecentAtCursor(model)
		return nil
	}
	if !t.commitView && !openedBySelect && !t.histDragging && !t.graphDragging && t.metaDrag == 0 && !t.cursorOnCommitSplit() {
		if hash, ok := t.commitAtCursor(); ok {
			if hash == t.openHash {
				t.openHash = ""
			} else {
				t.openHash = hash
			}
			guigui.RequestRebuild()
		}
	}
	ref, hit := t.refAtRows(&t.sidePane.body.tagRows, model)
	if !hit {
		t.tagClicks = gitClickCount{}
		t.tagClick = ""
		return nil
	}
	if t.tagClick != ref.Name {
		t.tagClicks = gitClickCount{}
		t.tagClick = ref.Name
	}
	if t.tagClicks.click(ebiten.Tick()) >= 2 {
		model.DoCheckout(ref)
	}
	return nil
}

func (t *GitTool) maybeLoadMoreHistory(model *GitModel) {
	if t.commitView || t.showEmpty || t.emptyHome || t.sshPick {
		return
	}
	t.trackHistoryEnd(model.Path())
	if !model.HasRepo() || !model.HasMoreHistory() || model.HistoryLoading() {
		return
	}
	n := t.commitList.ItemCount()
	visible := n > 0 && t.commitList.IsItemInViewport(n-1)
	_, wy := ebiten.Wheel()
	wheelDown := wy < 0 && image.Pt(ebiten.CursorPosition()).In(t.histGraph)
	if t.holdHistoryEnd(visible, wheelDown) {
		return
	}
	if !visible {
		return
	}
	model.LoadMoreHistory()
}

func (t *GitTool) trackHistoryEnd(path string) {
	if t.historyEndPath == path {
		return
	}
	t.historyEndPath = path
	t.clearHistoryEnd()
}

func (t *GitTool) clearHistoryEnd() {
	t.historyEndOn = false
	t.historyEndSeen = false
}

func (t *GitTool) pinHistoryEnd() {
	t.historyEndOn = true
	t.historyEndSeen = false
}

// holdHistoryEnd reports that End's extra history page is already requested,
// so another page waits. A downward wheel, or scrolling back up, releases the hold.
func (t *GitTool) holdHistoryEnd(visible, wheelDown bool) bool {
	if !t.historyEndOn {
		return false
	}
	if t.historyEndSeen && (wheelDown || !visible) {
		t.clearHistoryEnd()
		return false
	}
	if visible {
		t.historyEndSeen = true
	}
	return true
}

func (t *GitTool) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if t.sideDragging || t.graphDragging || t.metaDrag != 0 {
		return ebiten.CursorShapeEWResize, true
	}
	if t.histDragging {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func (t *GitTool) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !t.commitGraphKeyTarget(context) {
		return guigui.HandleInputResult{}
	}
	var key ebiten.Key
	switch {
	case graphKeyRepeating(ebiten.KeyPageDown):
		key = ebiten.KeyPageDown
	case graphKeyRepeating(ebiten.KeyPageUp):
		key = ebiten.KeyPageUp
	case graphKeyRepeating(ebiten.KeyHome):
		key = ebiten.KeyHome
	case graphKeyRepeating(ebiten.KeyEnd):
		key = ebiten.KeyEnd
	default:
		return guigui.HandleInputResult{}
	}
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return guigui.HandleInputResult{}
	}
	t.scrollCommitGraph(context, v.(*Model).Git(), key)
	return guigui.HandleInputByWidget(t)
}

func (t *GitTool) commitGraphKeyTarget(context *guigui.Context) bool {
	if t.commitView || t.showEmpty || t.emptyHome || t.sshPick {
		return false
	}
	if t.rename.IsOpen() || t.confirm.IsOpen() || t.tagMenu.IsOpen() || t.branchMenu.IsOpen() || t.commitMenu.IsOpen() || t.selMenu.IsOpen() {
		return false
	}
	if shortcutModifierPressed(context) || ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyControl) {
		return false
	}
	if context.IsFocusedOrHasFocusedDescendant(&t.commitList) {
		return true
	}
	// Focus is outside this tool. The graph takes Page, Home, and End.
	return !context.IsFocusedOrHasFocusedDescendant(t)
}

func (t *GitTool) scrollCommitGraph(context *guigui.Context, model *GitModel, key ebiten.Key) {
	n := t.commitList.ItemCount()
	if n == 0 || model == nil {
		return
	}
	switch key {
	case ebiten.KeyHome:
		t.clearHistoryEnd()
		t.scrollGraphTo(model, 0)
	case ebiten.KeyEnd:
		end := nearestCommitIndex(n, t.commitListValue, n-1)
		if end < 0 {
			end = n - 1
		}
		t.scrollGraphTo(model, end)
		// One more page after landing on the last commit already loaded.
		// The hold keeps that from chaining while the new end stays on screen.
		t.pinHistoryEnd()
		model.LoadMoreHistory()
	case ebiten.KeyPageDown:
		t.clearHistoryEnd()
		t.scrollGraphPage(model, context, true)
	case ebiten.KeyPageUp:
		t.clearHistoryEnd()
		t.scrollGraphPage(model, context, false)
	}
}

func (t *GitTool) scrollGraphPage(model *GitModel, context *guigui.Context, down bool) {
	n := t.commitList.ItemCount()
	view := t.commitRowView(context)
	if n == 0 || view.Empty() {
		return
	}
	bounds := t.commitList.ItemBounds
	top := graphTopIndex(n, bounds, view)
	if top < 0 {
		return
	}
	index := graphPageIndex(n, bounds, view, down)
	if index == top {
		return
	}
	t.scrollGraphTo(model, index)
}

func (t *GitTool) scrollGraphTo(model *GitModel, index int) {
	n := t.commitList.ItemCount()
	if n == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= n {
		index = n - 1
	}
	t.commitList.JumpToItemByIndex(index)
	sel := nearestCommitIndex(n, t.commitListValue, index)
	if sel < 0 {
		return
	}
	hash := t.commitListValue(sel)
	model.HighlightCommit(hash)
}

func (t *GitTool) commitListValue(index int) string {
	item, ok := t.commitList.ItemByIndex(index)
	if !ok {
		return ""
	}
	return item.Value
}

func (t *GitTool) commitRowView(context *guigui.Context) image.Rectangle {
	view := t.histGraph
	if view.Empty() {
		return view
	}
	header := gitCommitHeaderHeight(basicwidget.UnitSize(context))
	view.Min.Y += header
	if view.Min.Y > view.Max.Y {
		view.Min.Y = view.Max.Y
	}
	return view
}

func graphArrowBlocked(context *guigui.Context) bool {
	return shortcutModifierPressed(context) || ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyControl)
}

func graphArrow() (down, ok bool) {
	switch {
	case graphKeyRepeating(ebiten.KeyDown):
		return true, true
	case graphKeyRepeating(ebiten.KeyUp):
		return false, true
	default:
		return false, false
	}
}

// graphKeyRepeating matches the list widget's arrow-key repeat.
func graphKeyRepeating(key ebiten.Key) bool {
	if !ebiten.IsKeyPressed(key) {
		return false
	}
	d := inpututil.KeyPressDuration(key)
	if d <= 1 {
		return true
	}
	delay := ebiten.TPS() * 2 / 5
	if d < delay {
		return false
	}
	return (d-delay)%4 == 0
}

// graphPageIndex is the row that should sit at the top after scrolling one viewport.
// bounds returns each row's rectangle in the same space as view. An empty
// rectangle means the row is not laid out. down moves toward later commits.
func graphPageIndex(n int, bounds func(int) image.Rectangle, view image.Rectangle, down bool) int {
	if n <= 0 {
		return 0
	}
	last := n - 1
	if view.Empty() {
		if down {
			return last
		}
		return 0
	}
	first := graphTopIndex(n, bounds, view)
	if first < 0 {
		if down {
			return last
		}
		return 0
	}
	if !down {
		return graphPageUpIndex(first, bounds, view)
	}
	lb := bounds(last)
	if !lb.Empty() && lb.Min.Y >= view.Min.Y && lb.Max.Y <= view.Max.Y {
		return first
	}
	for i := first; i < n; i++ {
		b := bounds(i)
		if b.Empty() {
			if i > first {
				return i
			}
			break
		}
		if b.Min.Y >= view.Max.Y {
			return i
		}
		if b.Max.Y > view.Max.Y {
			// This row crosses the bottom. Land on it so the rest comes into view.
			// When it is already the top row, step forward so the key still moves.
			if i == first && b.Min.Y <= view.Min.Y && i < last {
				return i + 1
			}
			return i
		}
	}
	return last
}

// graphTopIndex is the first row that intersects view, or -1 when none do.
func graphTopIndex(n int, bounds func(int) image.Rectangle, view image.Rectangle) int {
	if n <= 0 || view.Empty() {
		return -1
	}
	for i := 0; i < n; i++ {
		b := bounds(i)
		if b.Empty() || b.Max.Y <= view.Min.Y {
			continue
		}
		if b.Min.Y >= view.Max.Y {
			break
		}
		return i
	}
	return -1
}

func graphPageUpIndex(first int, bounds func(int) image.Rectangle, view image.Rectangle) int {
	if first <= 0 {
		return 0
	}
	pageH := view.Dy()
	if pageH < 1 {
		return 0
	}
	b := bounds(first)
	hidden := view.Min.Y - b.Min.Y
	if hidden < 0 {
		hidden = 0
	}
	need := pageH - hidden
	if need <= 0 {
		return first
	}
	fallback := b.Dy()
	if fallback < 1 {
		fallback = 1
	}
	acc := 0
	for i := first - 1; i >= 0; i-- {
		h := bounds(i).Dy()
		if h < 1 {
			h = fallback
		}
		acc += h
		if acc >= need {
			return i
		}
	}
	return 0
}

// nearestCommitIndex is the commit row closest to index.
// An empty value is the detail panel under an open commit.
func nearestCommitIndex(n int, value func(int) string, index int) int {
	if n <= 0 {
		return -1
	}
	if index < 0 {
		index = 0
	}
	if index >= n {
		index = n - 1
	}
	if value(index) != "" {
		return index
	}
	for i := index - 1; i >= 0; i-- {
		if value(i) != "" {
			return i
		}
	}
	for i := index + 1; i < n; i++ {
		if value(i) != "" {
			return i
		}
	}
	return -1
}

func (t *GitTool) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if t.sideDragging {
		return t.dragSideSplit(context)
	}
	if t.graphDragging {
		return t.dragGraphSplit(context)
	}
	if t.metaDrag != 0 {
		return t.dragMetaSplit(context, t.metaDrag)
	}
	if t.histDragging {
		return t.dragHistSplit(context)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return guigui.HandleInputResult{}
	}
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return guigui.HandleInputResult{}
	}
	appModel := v.(*Model)
	model := appModel.Git()
	if ref, ok := t.refAtRows(&t.sidePane.body.branchRows, model); ok {
		t.openBranchMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if ref, ok := t.refAtRows(&t.sidePane.body.remoteRows, model); ok {
		t.openBranchMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if ref, ok := t.refAtRows(&t.sidePane.body.tagRows, model); ok {
		t.openTagMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if !t.commitView {
		if t.cursorOnCommitSplit() {
			return guigui.HandleInputByWidget(t)
		}
		if hash, ok := t.commitAtCursor(); ok {
			t.openCommitMenu(appModel.Lang(), model, hash)
			return guigui.HandleInputByWidget(t)
		}
	}
	return guigui.HandleInputResult{}
}

// cursorInGraphItem reports a commit-row hit that lies inside the graph.
// A row rectangle can continue into the splitter under the graph.
func cursorInGraphItem(cursor image.Point, graph, item image.Rectangle) bool {
	return cursor.In(graph) && cursor.In(item)
}

func (t *GitTool) commitAtCursor() (string, bool) {
	c := image.Pt(ebiten.CursorPosition())
	for i := 0; i < t.commitList.ItemCount(); i++ {
		if !cursorInGraphItem(c, t.histGraph, t.commitList.ItemBounds(i)) {
			continue
		}
		item, ok := t.commitList.ItemByIndex(i)
		if !ok || item.Value == "" || item.Value == gitcli.Uncommitted {
			return "", false
		}
		return item.Value, true
	}
	return "", false
}

func (t *GitTool) setHistDiff(lang i18n.Lang, model *GitModel) {
	detail := model.Detail()
	body := ""
	empty := i18n.T(lang, i18n.StatusGitLoading)
	if detail.Hash == t.openHash {
		body = fileDiffText(detail.Diff, t.histFile)
		empty = i18n.T(lang, i18n.GitNoChanges)
	}
	shown := t.openHash + "\x00" + t.histFile
	t.histStack.diff.Set(body, empty, shown)
	t.histStack.diff.SetSplit(lang, t.diffSplit, t.setDiffSplit)
}

func (t *GitTool) setDiffSplit(split bool) {
	if t.diffSplit == split {
		return
	}
	t.diffSplit = split
	guigui.RequestRebuild()
}

func (t *GitTool) openLineMenu(context *guigui.Context, start, end int) bool {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok || end <= start {
		return false
	}
	appModel := v.(*Model)
	model := appModel.Git()
	if !model.HasRepo() || model.Busy() || t.workPanes.diff.Text() == "" {
		return false
	}
	_, staged := t.activeWorkTarget()
	patch, ok, selected := t.workPanes.diff.Patch(staged)
	if !selected || !ok {
		return false
	}
	lang := appModel.Lang()
	label := i18n.T(lang, i18n.GitStageLines)
	if staged {
		label = i18n.T(lang, i18n.GitUnstageLines)
	}
	t.selMenu.Open(patch, staged, image.Pt(ebiten.CursorPosition()), label, func(patch string, unstage bool) {
		model.DoApplyPatch(patch, unstage)
	})
	return true
}

func (t *GitTool) openCommitMenu(lang i18n.Lang, model *GitModel, hash string) {
	if hash == "" || hash == gitcli.Uncommitted || !model.HasRepo() || model.Busy() {
		return
	}
	model.SelectCommit(hash)
	t.commitMenu.Open(hash, image.Pt(ebiten.CursorPosition()),
		i18n.T(lang, i18n.GitCreateBranch),
		i18n.T(lang, i18n.GitCreateTag),
		func(hash string) {
			t.rename.Ask(
				i18n.T(lang, i18n.GitBranchPrompt),
				"",
				i18n.T(lang, i18n.GitCreateBranch),
				i18n.T(lang, i18n.GitCancel),
				func(name string) { model.DoCreateBranch(hash, name) },
			)
		},
		func(hash string) {
			t.rename.AskTag(
				i18n.T(lang, i18n.GitTagPrompt),
				i18n.T(lang, i18n.GitPushTag),
				i18n.T(lang, i18n.GitCreateTag),
				i18n.T(lang, i18n.GitCancel),
				func(name string, push bool) { model.DoCreateTag(hash, name, push) },
			)
		},
	)
}

func (t *GitTool) refAtRows(rows *guigui.WidgetSlice[*gitNavRow], model *GitModel) (gitcli.Ref, bool) {
	c := image.Pt(ebiten.CursorPosition())
	for i := 0; i < rows.Len(); i++ {
		row := rows.At(i)
		if !c.In(row.bounds) {
			continue
		}
		return t.refByValue(model, row.value)
	}
	return gitcli.Ref{}, false
}

// switchBranch checks out ref. A remote branch whose name matches a local
// branch asks whether to check out that local branch and pull, or cancel.
func (t *GitTool) switchBranch(lang i18n.Lang, model *GitModel, ref gitcli.Ref) {
	if ref.Remote != "" {
		name := ref.LocalName()
		if name != "" && name != "HEAD" {
			if local, ok := localBranchByName(model.Snapshot().Locals, name); ok {
				remote := ref
				t.confirm.Ask(
					i18n.T(lang, i18n.GitCheckoutLocalAsk, name),
					i18n.T(lang, i18n.GitCheckoutPull),
					i18n.T(lang, i18n.GitCancel),
					func() { model.DoCheckoutAndPull(local, remote) },
				)
				return
			}
		}
	}
	model.DoCheckout(ref)
}

func localBranchByName(locals []gitcli.Ref, name string) (gitcli.Ref, bool) {
	if name == "" {
		return gitcli.Ref{}, false
	}
	for _, ref := range locals {
		if ref.Remote == "" && ref.Name == name {
			return ref, true
		}
	}
	return gitcli.Ref{}, false
}

func (t *GitTool) refByValue(model *GitModel, value string) (gitcli.Ref, bool) {
	kind, name, ok := strings.Cut(value, ":")
	if !ok || name == "" {
		return gitcli.Ref{}, false
	}
	snap := model.Snapshot()
	switch kind {
	case "local":
		for _, ref := range snap.Locals {
			if ref.Name == name {
				return ref, true
			}
		}
		return gitcli.Ref{}, false
	case "tag":
		for _, ref := range snap.Tags {
			if ref.Name == name {
				return ref, true
			}
		}
		return gitcli.Ref{}, false
	}
	for _, ref := range snap.RemoteBranches {
		if ref.Name == name {
			return ref, true
		}
	}
	return gitcli.Ref{}, false
}

func (t *GitTool) openTagMenu(lang i18n.Lang, model *GitModel, ref gitcli.Ref) {
	if ref.Name == "" || !model.HasRepo() || model.Busy() {
		return
	}
	if ref.Hash != "" {
		model.RevealCommit(ref.Hash)
	}
	name := ref.Name
	t.tagMenu.Open(name, image.Pt(ebiten.CursorPosition()),
		i18n.T(lang, i18n.GitDelete),
		i18n.T(lang, i18n.GitPushTag),
		i18n.T(lang, i18n.GitRemoteDelete),
		func(name string) {
			t.confirm.Ask(
				i18n.T(lang, i18n.GitTagDeleteAsk, name),
				i18n.T(lang, i18n.GitDelete),
				i18n.T(lang, i18n.GitCancel),
				func() { model.DoDeleteTag(name) },
			)
		},
		func(name string) { model.DoPushTag(name) },
		func(name string) {
			t.confirm.Ask(
				i18n.T(lang, i18n.GitTagRemoteAsk, name),
				i18n.T(lang, i18n.GitDelete),
				i18n.T(lang, i18n.GitCancel),
				func() { model.DoDeleteRemoteTag(name) },
			)
		},
	)
}

func (t *GitTool) openBranchMenu(lang i18n.Lang, model *GitModel, ref gitcli.Ref) {
	if ref.Name == "" || !model.HasRepo() || model.Busy() {
		return
	}
	if ref.Hash != "" {
		model.SelectCommit(ref.Hash)
	}
	target := ref
	t.branchMenu.Open(target, image.Pt(ebiten.CursorPosition()), i18n.T(lang, i18n.GitRename), i18n.T(lang, i18n.GitDelete), func(ref gitcli.Ref) {
		t.rename.Ask(
			i18n.T(lang, i18n.GitRenamePrompt),
			ref.Name,
			i18n.T(lang, i18n.GitRename),
			i18n.T(lang, i18n.GitCancel),
			func(name string) { model.DoRenameBranch(ref, name) },
		)
	}, func(ref gitcli.Ref) {
		t.confirm.Ask(
			i18n.T(lang, i18n.GitDeleteConfirm, ref.Name),
			i18n.T(lang, i18n.GitDelete),
			i18n.T(lang, i18n.GitCancel),
			func() { model.DoDeleteBranch(ref) },
		)
	})
}
