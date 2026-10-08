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

	commitList  basicwidget.List[string]
	commitRows  guigui.WidgetSlice[*gitCommitRow]
	commitItems []basicwidget.ListItem[string]
	graphEmpty  basicwidget.Text
	showEmpty   bool

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

	detail gitDetail

	workTree      basicwidget.Text
	showWorkTree  bool
	workTreeLines int

	stageBtn    basicwidget.Button
	unstageBtn  basicwidget.Button
	showChanges bool
	diffStaged  bool
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
	w.WriteBool(t.commitView)
}

func (t *GitTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
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
	t.setWorkTree(lang, model)
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
	} else {
		adder.AddWidget(&t.detail)
		if t.showWorkTree {
			adder.AddWidget(&t.workTree)
		}
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
	if !t.commitView && t.showEmpty {
		t.graphEmpty.SetMultiline(true)
		t.graphEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.graphEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.graphEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		if !has {
			t.graphEmpty.SetValue(i18n.T(lang, i18n.GitEmpty))
		} else {
			t.graphEmpty.SetValue(i18n.T(lang, i18n.GitNoCommits))
		}
		var emptyStyle basicwidget.TextStyle
		useGitMono(&emptyStyle)
		t.graphEmpty.SetBaseStyle(&emptyStyle)
		adder.AddWidget(&t.graphEmpty)
	} else if !t.commitView {
		adder.AddWidget(&t.commitList)
		t.commitRows.SetLen(len(rows))
		t.commitItems = slices.Delete(t.commitItems, 0, len(t.commitItems))
		lanes := snap.Lanes
		for i, row := range rows {
			r := t.commitRows.At(i)
			r.Set(row, lanes, row.Commit.Hash == snap.HEAD, lang, func(ref gitcli.Ref) {
				t.switchBranch(lang, model, ref)
			}, onMenu)
			t.commitItems = append(t.commitItems, basicwidget.ListItem[string]{
				Content: r,
				Value:   row.Commit.Hash,
			})
		}
		t.commitList.SetStyle(basicwidget.ListStyleNormal)
		t.commitList.SetHighlightVisibleWhenUnfocused(true)
		t.commitList.SetItems(t.commitItems)
		if sel := model.Selected(); sel != "" {
			t.commitList.SelectItemByValue(sel)
		}
		if reveal := model.TakeReveal(); reveal != "" {
			if idx := t.commitList.IndexByValue(reveal); idx >= 0 {
				t.commitList.EnsureItemVisibleByIndex(idx)
			}
		}
		t.commitList.OnItemSelected(func(context *guigui.Context, index int) {
			item, ok := t.commitList.ItemByIndex(index)
			if !ok || item.Value == "" {
				return
			}
			model.SelectCommit(item.Value)
		})
		context.SetEnabled(&t.commitList, !busy)
	}

	t.detail.Set(lang, model, onMenu)

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

func (t *GitTool) setWorkTree(lang i18n.Lang, model *GitModel) {
	t.showWorkTree = false
	t.workTreeLines = 0
	if !model.HasRepo() {
		return
	}
	st := model.Snapshot().Status
	if !st.Dirty() {
		return
	}
	var b strings.Builder
	b.WriteString(i18n.T(lang, i18n.GitWorkingTree))
	for _, e := range st.Entries {
		b.WriteByte('\n')
		b.WriteString(e.String())
	}
	t.workTree.SetValue(b.String())
	t.workTree.SetMultiline(true)
	t.workTree.SetWrapMode(basicwidget.WrapModeNone)
	t.workTree.SetSelectable(true)
	t.workTreeLines = 1 + len(st.Entries)
	t.showWorkTree = true
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

func (t *GitTool) layoutHistoryColumn(u int) {
	graphBody := guigui.Widget(&t.graphEmpty)
	if !t.showEmpty {
		graphBody = &t.commitList
	}

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
		guigui.LinearLayoutItem{Widget: graphBody, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Widget: &t.detail, Size: guigui.FlexibleSize(2)},
	)
	if t.showWorkTree {
		lines := t.workTreeLines
		if lines < 1 {
			lines = 1
		}
		if lines > 4 {
			lines = 4
		}
		t.rightItems = append(t.rightItems, guigui.LinearLayoutItem{Widget: &t.workTree, Size: guigui.FixedSize(lines * u)})
	}
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
}

func (t *GitTool) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if t.sideDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.sideDragging = false
		t.setSideDragPassthrough(context, false)
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

func (t *GitTool) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if t.sideDragging {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (t *GitTool) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if t.sideDragging {
		return t.dragSideSplit(context)
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
		if hash, ok := t.commitAtCursor(); ok {
			t.openCommitMenu(appModel.Lang(), model, hash)
			return guigui.HandleInputByWidget(t)
		}
	}
	return guigui.HandleInputResult{}
}

func (t *GitTool) commitAtCursor() (string, bool) {
	c := image.Pt(ebiten.CursorPosition())
	for i := 0; i < t.commitList.ItemCount(); i++ {
		if !c.In(t.commitList.ItemBounds(i)) {
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
	patch, ok := gitcli.PatchForLines(t.workPanes.diff.Text(), start, end, staged)
	if !ok {
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
				if !local.Current && model.Snapshot().Status.Dirty() {
					model.SetStatus(i18n.StatusGitCheckoutDirty)
					return
				}
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
