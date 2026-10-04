package app

import (
	"image"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

var eventGitOpen = guigui.GenerateEventKey()

// GitTool is a local Git client: graph, branches, and history operations.
type GitTool struct {
	guigui.DefaultWidget

	tabBar gitTabBar

	openBtn     basicwidget.Button
	reloadBtn   basicwidget.Button
	pathLabel   basicwidget.Text
	modeBtn     basicwidget.Button
	fetchBtn    basicwidget.Button
	pullBtn     basicwidget.Button
	pushBtn     basicwidget.Button
	branchTitle basicwidget.Text
	commitBtn   basicwidget.Button
	amendBtn    basicwidget.Button
	commitView  bool

	repoTitle    basicwidget.Text
	branchLabel  basicwidget.Text
	tagLabel     basicwidget.Text
	remotesLabel basicwidget.Text
	remoteRows   guigui.WidgetSlice[*gitRemoteRow]
	branchList   basicwidget.List[string]
	branchItems  []basicwidget.ListItem[string]
	tagList      basicwidget.List[string]
	tagItems     []basicwidget.ListItem[string]
	showTags     bool
	tagClicks    gitClickCount
	tagClick     string
	remoteList   basicwidget.List[string]
	remoteItems  []basicwidget.ListItem[string]

	commitList  basicwidget.List[string]
	commitRows  guigui.WidgetSlice[*gitCommitRow]
	commitItems []basicwidget.ListItem[string]
	graphEmpty  basicwidget.Text
	showEmpty   bool

	detail gitDetail

	workTree      basicwidget.Text
	showWorkTree  bool
	workTreeLines int

	stageBtn    basicwidget.Button
	unstageBtn  basicwidget.Button
	changeList  basicwidget.List[string]
	changeItems []basicwidget.ListItem[string]
	changeSel   []string
	showChanges bool
	msgInput    guigui.WidgetWithSize[*basicwidget.TextInput]
	status      basicwidget.Text
	hint        basicwidget.Text

	branchMenu gitBranchMenu
	commitMenu gitCommitMenu
	tagMenu    gitTagMenu
	confirm    gitConfirm
	rename     gitRename

	toolbarItems []guigui.LinearLayoutItem
	leftItems    []guigui.LinearLayoutItem
	rightItems   []guigui.LinearLayoutItem
	bodyItems    []guigui.LinearLayoutItem
	msgItems     []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
	toolbar      guigui.LinearLayout
	leftCol      guigui.LinearLayout
	rightCol     guigui.LinearLayout
	bodyRow      guigui.LinearLayout
	msgRow       guigui.LinearLayout
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

	t.tabBar.Sync(lang, model)
	adder.AddWidget(&t.tabBar)

	adder.AddWidget(&t.openBtn)
	adder.AddWidget(&t.reloadBtn)
	adder.AddWidget(&t.branchTitle)
	adder.AddWidget(&t.modeBtn)
	adder.AddWidget(&t.fetchBtn)
	adder.AddWidget(&t.pullBtn)
	adder.AddWidget(&t.pushBtn)
	adder.AddWidget(&t.status)
	adder.AddWidget(&t.hint)
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
			adder.AddWidget(&t.changeList)
		} else {
			adder.AddWidget(&t.workTree)
		}
		adder.AddWidget(&t.stageBtn)
		adder.AddWidget(&t.unstageBtn)
		adder.AddWidget(&t.msgInput)
		adder.AddWidget(&t.commitBtn)
		adder.AddWidget(&t.amendBtn)
	} else {
		adder.AddWidget(&t.repoTitle)
		adder.AddWidget(&t.pathLabel)
		adder.AddWidget(&t.branchLabel)
		adder.AddWidget(&t.tagLabel)
		adder.AddWidget(&t.remotesLabel)
		adder.AddWidget(&t.branchList)
		adder.AddWidget(&t.tagList)
		adder.AddWidget(&t.remoteList)
		adder.AddWidget(&t.detail)
		if t.showWorkTree {
			adder.AddWidget(&t.workTree)
		}
	}

	t.openBtn.SetText(i18n.T(lang, i18n.GitOpen))
	t.openBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventGitOpen)
	})
	context.SetEnabled(&t.openBtn, !busy)

	t.reloadBtn.SetText(i18n.T(lang, i18n.GitReload))
	t.reloadBtn.OnDown(func(context *guigui.Context) {
		model.Reload()
	})
	context.SetEnabled(&t.reloadBtn, !busy && has)

	path := model.Path()
	repoName := i18n.T(lang, i18n.GitNoRepo)
	if path != "" {
		repoName = filepath.Base(path)
	} else {
		path = repoName
	}
	setBoldText(&t.repoTitle, true)
	t.repoTitle.SetValue(repoName)
	t.repoTitle.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.pathLabel.SetValue(path)
	t.pathLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.pathLabel.SetWrapMode(basicwidget.WrapModeNone)

	snap := model.Snapshot()
	branchName := i18n.T(lang, i18n.GitNoRepo)
	if has {
		if snap.Detached || snap.Branch == "" {
			branchName = i18n.T(lang, i18n.GitDetached)
		} else {
			branchName = snap.Branch
			st := snap.Status
			if st.Ahead > 0 || st.Behind > 0 {
				branchName += "  " + i18n.T(lang, i18n.GitAheadBehind, st.Ahead, st.Behind)
			}
		}
	}
	setBoldText(&t.branchTitle, true)
	t.branchTitle.SetValue(branchName)
	t.branchTitle.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.branchTitle.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	enableNet := model.CanPushPull()
	if t.commitView {
		t.modeBtn.SetText(i18n.T(lang, i18n.GitGraph))
		t.modeBtn.OnDown(func(context *guigui.Context) {
			t.commitView = false
			guigui.RequestRebuild()
		})
	} else {
		t.modeBtn.SetText(i18n.T(lang, i18n.GitCommit))
		t.modeBtn.OnDown(func(context *guigui.Context) {
			t.commitView = true
			guigui.RequestRebuild()
		})
	}
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

	setBoldText(&t.branchLabel, true)
	t.branchLabel.SetValue(i18n.T(lang, i18n.GitBranches))
	setBoldText(&t.remotesLabel, true)
	t.remotesLabel.SetValue(i18n.T(lang, i18n.GitRemotes))

	t.remoteRows.SetLen(len(snap.Remotes))
	for i, rm := range snap.Remotes {
		row := t.remoteRows.At(i)
		name := rm.Name
		row.Set(name, model.RemoteVisible(name), func(visible bool) {
			model.SetRemoteVisible(name, visible)
		})
		if !t.commitView {
			adder.AddWidget(row)
		}
	}

	var mono basicwidget.TextStyle
	useGitMono(&mono)
	monoItem := basicwidget.ItemTextStyle{Style: mono}

	t.branchItems = slices.Delete(t.branchItems, 0, len(t.branchItems))
	for _, ref := range snap.Locals {
		text := ref.Name
		if ref.Current {
			text = "✓ " + text
		}
		t.branchItems = append(t.branchItems, basicwidget.ListItem[string]{
			Text:      text,
			TextStyle: monoItem,
			Value:     "local:" + ref.Name,
		})
	}
	t.branchList.SetStyle(basicwidget.ListStyleNormal)
	t.branchList.SetHighlightVisibleWhenUnfocused(true)
	t.branchList.SetItems(t.branchItems)
	t.branchList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.branchList.ItemByIndex(index)
		if !ok {
			return
		}
		t.selectRef(model, item.Value)
	})
	context.SetEnabled(&t.branchList, has && !busy)

	t.tagItems = slices.Delete(t.tagItems, 0, len(t.tagItems))
	for _, ref := range snap.Tags {
		t.tagItems = append(t.tagItems, basicwidget.ListItem[string]{
			Text:      ref.Name,
			TextStyle: monoItem,
			Value:     "tag:" + ref.Name,
		})
	}
	t.showTags = len(t.tagItems) > 0
	setBoldText(&t.tagLabel, true)
	t.tagLabel.SetValue(i18n.T(lang, i18n.GitTags))
	t.tagList.SetStyle(basicwidget.ListStyleNormal)
	t.tagList.SetHighlightVisibleWhenUnfocused(true)
	t.tagList.SetItems(t.tagItems)
	t.tagList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.tagList.ItemByIndex(index)
		if !ok {
			return
		}
		t.selectRef(model, item.Value)
	})
	context.SetEnabled(&t.tagList, has && !busy)

	t.remoteItems = slices.Delete(t.remoteItems, 0, len(t.remoteItems))
	for _, ref := range snap.RemoteBranches {
		if !model.RemoteVisible(ref.Remote) {
			continue
		}
		t.remoteItems = append(t.remoteItems, basicwidget.ListItem[string]{
			Text:  "  " + ref.LocalName(),
			Value: "remote:" + ref.Name,
		})
	}
	t.remoteList.SetStyle(basicwidget.ListStyleNormal)
	t.remoteList.SetHighlightVisibleWhenUnfocused(true)
	t.remoteList.SetItems(t.remoteItems)
	t.remoteList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.remoteList.ItemByIndex(index)
		if !ok {
			return
		}
		t.selectRef(model, item.Value)
	})
	context.SetEnabled(&t.remoteList, has && !busy)

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
			r.Set(row, lanes, row.Commit.Hash == snap.HEAD, lang, model.DoCheckout, onMenu)
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
	adder.AddWidget(&t.confirm)
	adder.AddWidget(&t.rename)
	return nil
}

func (t *GitTool) setChangeList(context *guigui.Context, lang i18n.Lang, model *GitModel, entries []gitcli.StatusEntry, enabled bool) {
	var mono basicwidget.TextStyle
	useGitMono(&mono)
	monoItem := basicwidget.ItemTextStyle{Style: mono}

	have := make(map[string]gitcli.StatusEntry, len(entries))
	t.changeItems = slices.Delete(t.changeItems, 0, len(t.changeItems))
	for _, e := range entries {
		have[e.Path] = e
		t.changeItems = append(t.changeItems, basicwidget.ListItem[string]{
			Text:      e.String(),
			TextStyle: monoItem,
			Value:     e.Path,
		})
	}
	kept := make([]string, 0, len(t.changeSel))
	for _, p := range t.changeSel {
		if _, ok := have[p]; ok {
			kept = append(kept, p)
		}
	}
	t.changeSel = kept

	t.changeList.SetStyle(basicwidget.ListStyleNormal)
	t.changeList.SetHighlightVisibleWhenUnfocused(true)
	t.changeList.SetMultiSelection(true)
	t.changeList.SetItems(t.changeItems)
	t.changeList.OnItemsSelected(func(context *guigui.Context, indices []int) {
		paths := make([]string, 0, len(indices))
		for _, i := range indices {
			item, ok := t.changeList.ItemByIndex(i)
			if ok && item.Value != "" {
				paths = append(paths, item.Value)
			}
		}
		if samePaths(t.changeSel, paths) {
			return
		}
		t.changeSel = paths
		guigui.RequestRebuild()
	})
	if len(t.changeSel) > 0 {
		t.changeList.SelectItemsByValues(t.changeSel)
	}
	context.SetEnabled(&t.changeList, enabled)

	var stagePaths, unstagePaths []string
	selected := make(map[string]bool, len(t.changeSel))
	for _, p := range t.changeSel {
		selected[p] = true
	}
	for _, e := range entries {
		if !selected[e.Path] {
			continue
		}
		if e.Unstaged() {
			stagePaths = append(stagePaths, e.Path)
		}
		if e.Staged() {
			unstagePaths = append(unstagePaths, e.Path)
		}
	}
	t.stageBtn.SetText(i18n.T(lang, i18n.GitStage))
	t.stageBtn.OnDown(func(context *guigui.Context) { model.DoStage(stagePaths) })
	context.SetEnabled(&t.stageBtn, enabled && len(stagePaths) > 0)
	t.unstageBtn.SetText(i18n.T(lang, i18n.GitUnstage))
	t.unstageBtn.OnDown(func(context *guigui.Context) { model.DoUnstage(unstagePaths) })
	context.SetEnabled(&t.unstageBtn, enabled && len(unstagePaths) > 0)
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

func (t *GitTool) selectRef(model *GitModel, value string) {
	kind, name, ok := strings.Cut(value, ":")
	if !ok || name == "" {
		return
	}
	snap := model.Snapshot()
	if kind == "local" {
		for _, ref := range snap.Locals {
			if ref.Name == name {
				model.RevealCommit(ref.Hash)
				return
			}
		}
		return
	}
	if kind == "tag" {
		for _, ref := range snap.Tags {
			if ref.Name == name {
				model.RevealCommit(ref.Hash)
				return
			}
		}
		return
	}
	for _, ref := range snap.RemoteBranches {
		if ref.Name == name {
			model.RevealCommit(ref.Hash)
			return
		}
	}
}

func (t *GitTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	bounds := widgetBounds.Bounds()
	barH := gitTabBarHeight(u)
	layouter.LayoutWidget(&t.tabBar, image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+barH))
	rest := bounds
	rest.Min.Y += barH

	t.toolbarItems = slices.Delete(t.toolbarItems, 0, len(t.toolbarItems))
	t.toolbarItems = append(t.toolbarItems,
		guigui.LinearLayoutItem{Widget: &t.modeBtn},
		guigui.LinearLayoutItem{Widget: &t.fetchBtn},
		guigui.LinearLayoutItem{Widget: &t.pullBtn},
		guigui.LinearLayoutItem{Widget: &t.pushBtn},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.branchTitle},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.openBtn},
		guigui.LinearLayoutItem{Widget: &t.reloadBtn},
	)
	t.toolbar = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.toolbarItems, Gap: u / 4}

	if t.commitView {
		t.layoutCommit(context, rest, layouter, u)
		t.layoutOverlays(context, layouter)
		return
	}

	t.leftItems = slices.Delete(t.leftItems, 0, len(t.leftItems))
	t.leftItems = append(t.leftItems,
		guigui.LinearLayoutItem{Widget: &t.repoTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.pathLabel, Size: guigui.FixedSize(u * 3 / 4)},
		guigui.LinearLayoutItem{Widget: &t.branchLabel, Size: guigui.FixedSize(u * 3 / 4)},
		guigui.LinearLayoutItem{Widget: &t.branchList, Size: guigui.FlexibleSize(3)},
	)
	if t.showTags {
		t.leftItems = append(t.leftItems,
			guigui.LinearLayoutItem{Widget: &t.tagLabel, Size: guigui.FixedSize(u * 3 / 4)},
			guigui.LinearLayoutItem{Widget: &t.tagList, Size: guigui.FlexibleSize(1)},
		)
	}
	t.leftItems = append(t.leftItems,
		guigui.LinearLayoutItem{Widget: &t.remotesLabel, Size: guigui.FixedSize(u * 3 / 4)},
	)
	for i := 0; i < t.remoteRows.Len(); i++ {
		t.leftItems = append(t.leftItems, guigui.LinearLayoutItem{Widget: t.remoteRows.At(i), Size: guigui.FixedSize(u)})
	}
	t.leftItems = append(t.leftItems, guigui.LinearLayoutItem{Widget: &t.remoteList, Size: guigui.FlexibleSize(1)})
	t.leftCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.leftItems, Gap: u / 8}

	graphBody := guigui.Widget(&t.graphEmpty)
	if !t.showEmpty {
		graphBody = &t.commitList
	}

	t.rightItems = slices.Delete(t.rightItems, 0, len(t.rightItems))
	t.rightItems = append(t.rightItems,
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

	t.bodyItems = slices.Delete(t.bodyItems, 0, len(t.bodyItems))
	t.bodyItems = append(t.bodyItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(9 * u), Layout: &t.leftCol},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.rightCol},
	)
	t.bodyRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.bodyItems, Gap: u / 2}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.bodyRow},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     t.layoutItems,
		Gap:       u / 6,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 4, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, rest, layouter)
	t.layoutOverlays(context, layouter)
}

func (t *GitTool) layoutCommit(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter, u int) {
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

	changes := guigui.Widget(&t.workTree)
	if t.showChanges {
		changes = &t.changeList
	}
	t.rightItems = slices.Delete(t.rightItems, 0, len(t.rightItems))
	t.rightItems = append(t.rightItems,
		guigui.LinearLayoutItem{Widget: changes, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Widget: &t.msgInput, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.msgRow},
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u * 3 / 4)},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FixedSize(u * 3 / 4)},
	)
	t.rightCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.rightItems, Gap: u / 6}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.rightCol},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     t.layoutItems,
		Gap:       u / 6,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 4, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, bounds, layouter)
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
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	if t.rename.IsOpen() || t.confirm.IsOpen() || t.tagMenu.IsOpen() || t.branchMenu.IsOpen() || t.commitMenu.IsOpen() {
		return nil
	}
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model).Git()
	ref, hit := t.branchAtCursor(&t.tagList, model)
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

func (t *GitTool) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return guigui.HandleInputResult{}
	}
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return guigui.HandleInputResult{}
	}
	appModel := v.(*Model)
	model := appModel.Git()
	if ref, ok := t.branchAtCursor(&t.branchList, model); ok {
		t.openBranchMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if ref, ok := t.branchAtCursor(&t.remoteList, model); ok {
		t.openBranchMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if ref, ok := t.branchAtCursor(&t.tagList, model); ok {
		t.openTagMenu(appModel.Lang(), model, ref)
		return guigui.HandleInputByWidget(t)
	}
	if hash, ok := t.commitAtCursor(); ok {
		t.openCommitMenu(appModel.Lang(), model, hash)
		return guigui.HandleInputByWidget(t)
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

func (t *GitTool) openCommitMenu(lang i18n.Lang, model *GitModel, hash string) {
	if hash == "" || hash == gitcli.Uncommitted || !model.HasRepo() || model.Busy() {
		return
	}
	model.SelectCommit(hash)
	t.commitMenu.Open(hash, image.Pt(ebiten.CursorPosition()), i18n.T(lang, i18n.GitCreateTag), func(hash string) {
		t.rename.AskTag(
			i18n.T(lang, i18n.GitTagPrompt),
			i18n.T(lang, i18n.GitPushTag),
			i18n.T(lang, i18n.GitCreateTag),
			i18n.T(lang, i18n.GitCancel),
			func(name string, push bool) { model.DoCreateTag(hash, name, push) },
		)
	})
}

func (t *GitTool) branchAtCursor(list *basicwidget.List[string], model *GitModel) (gitcli.Ref, bool) {
	c := image.Pt(ebiten.CursorPosition())
	for i := 0; i < list.ItemCount(); i++ {
		if !c.In(list.ItemBounds(i)) {
			continue
		}
		item, ok := list.ItemByIndex(i)
		if !ok {
			return gitcli.Ref{}, false
		}
		return t.refByValue(model, item.Value)
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

type gitRemoteRow struct {
	guigui.DefaultWidget

	check basicwidget.Checkbox
	label basicwidget.Text

	layoutItems []guigui.LinearLayoutItem
}

func (r *gitRemoteRow) Set(name string, visible bool, on func(bool)) {
	r.label.SetValue(name)
	r.check.SetValue(visible)
	r.check.OnValueChanged(func(context *guigui.Context, value bool) {
		if on != nil {
			on(value)
		}
	})
}

func (r *gitRemoteRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.check)
	adder.AddWidget(&r.label)
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (r *gitRemoteRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	r.layoutItems = append(r.layoutItems,
		guigui.LinearLayoutItem{Widget: &r.check, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &r.label, Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.layoutItems,
		Gap:       u / 8,
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *gitRemoteRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	s := image.Pt(6*u, u)
	if w, ok := constraints.FixedWidth(); ok {
		s.X = w
	}
	return s
}
