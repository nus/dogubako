package app

import (
	"image"
	"path"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

const gitSideDefaultUnits = 9

// gitSidePane is the left column. The workspace, including other worktrees,
// stays put. Branches, tags, remotes, and submodules scroll beneath it.
type gitSidePane struct {
	guigui.DefaultWidget

	fixed gitSideFixed
	panel basicwidget.Panel
	body  gitSideBody
}

func (p *gitSidePane) setOwner(t *GitTool) {
	p.fixed.owner = t
	p.body.owner = t
	p.body.scrollTo = func(contentY float64) {
		p.panel.SetScrollOffset(0, -contentY)
	}
}

func (p *gitSidePane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(p, true)
	adder.AddWidget(&p.fixed)
	adder.AddWidget(&p.panel)
	p.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.panel.SetAutoBorder(false)
	p.panel.SetBorders(basicwidget.PanelBorders{})
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	p.panel.SetContent(&p.body)
	return nil
}

func (p *gitSidePane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	fixedH := p.fixed.Measure(context, guigui.FixedWidthConstraints(bounds.Dx())).Y
	if fixedH > bounds.Dy() {
		fixedH = bounds.Dy()
	}
	if fixedH < 0 {
		fixedH = 0
	}
	fixedB := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+fixedH)
	scrollB := image.Rect(bounds.Min.X, fixedB.Max.Y, bounds.Max.X, bounds.Max.Y)
	if scrollB.Max.Y < scrollB.Min.Y {
		scrollB.Max.Y = scrollB.Min.Y
	}
	p.body.viewport = scrollB
	layouter.LayoutWidget(&p.fixed, fixedB)
	layouter.LayoutWidget(&p.panel, scrollB)
}

// gitSideWidth is the left column width. preferred is the dragged width;
// the default column is gitSideDefaultUnits wide. minSide and minRight keep
// both columns usable, and a tight window shares what remains.
func gitSideWidth(total, splitW, preferred, minSide, minRight int) int {
	avail := total - splitW
	if avail <= 1 {
		if avail < 0 {
			return 0
		}
		return avail
	}
	if preferred < 1 {
		preferred = 1
	}
	if minSide > avail-1 {
		minSide = avail / 3
		if minSide < 1 {
			minSide = 1
		}
	}
	side := preferred
	if side < minSide {
		side = minSide
	}
	maxSide := avail - minRight
	if maxSide < minSide {
		maxSide = avail - 1
	}
	if side > maxSide {
		side = maxSide
	}
	if side < 1 {
		side = 1
	}
	return side
}

// gitSideSplit is the draggable boundary of the left column. It draws nothing.
type gitSideSplit struct {
	guigui.DefaultWidget

	tool *GitTool
}

func (s *gitSideSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil {
		return guigui.HandleInputResult{}
	}
	if s.tool.sideDragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.tool.dragSideSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitSideSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool != nil && s.tool.sideDragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (t *GitTool) setSideUnitsFromCursor(x, u int) {
	splitW := gitWorkSplitWidth(u)
	sideW := x - t.bodyBounds.Min.X - splitW/2
	clamped := gitSideWidth(t.bodyBounds.Dx(), splitW, sideW, 6*u, 16*u)
	if u < 1 {
		u = 1
	}
	t.sideUnits = float64(clamped) / float64(u)
}

func (t *GitTool) dragSideSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !t.sideDragging {
		if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.sideDragging = true
	}
	t.setSideUnitsFromCursor(image.Pt(ebiten.CursorPosition()).X, u)
	t.setSideDragPassthrough(context, true)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.sideDragging = false
		t.setSideDragPassthrough(context, false)
	}
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(t)
}

func (t *GitTool) setSideDragPassthrough(context *guigui.Context, pass bool) {
	for _, w := range []guigui.Widget{
		&t.tabBar,
		&t.sidePane,
		&t.fetchBtn,
		&t.pullBtn,
		&t.pushBtn,
		&t.status,
		&t.hint,
		&t.workPanes,
		&t.workTree,
		&t.msgInput,
		&t.stageBtn,
		&t.unstageBtn,
		&t.commitBtn,
		&t.amendBtn,
		&t.expand,
		&t.commitList,
		&t.graphEmpty,
		&t.histStack,
		&t.branchMenu,
		&t.commitMenu,
		&t.tagMenu,
		&t.selMenu,
		&t.confirm,
		&t.rename,
	} {
		context.SetPassthrough(w, pass)
	}
}

func (t *GitTool) openWorktree(model *GitModel, dir string) {
	if model == nil || strings.TrimSpace(dir) == "" {
		return
	}
	loc := gitcli.ParseLoc(model.Path())
	if loc.IsRemote() {
		loc.Dir = path.Clean(dir)
	} else {
		loc.Dir = dir
	}
	model.Open(loc.Key())
}

// worktreeTitle is the sidebar label for a linked working tree.
// A branch name identifies it. A detached tree shows its folder and a short hash.
func worktreeTitle(wt gitcli.Worktree) string {
	if wt.Branch != "" {
		return wt.Branch
	}
	name := path.Base(filepath.ToSlash(wt.Path))
	if name == "." || name == "/" || name == "" {
		name = wt.Path
	}
	if wt.Head == "" {
		return name
	}
	short := wt.Head
	if len(short) > 7 {
		short = short[:7]
	}
	if name == "" {
		return short
	}
	return name + " (" + short + ")"
}

// gitSideFixed is the part of the left column that does not scroll:
// the repository path, the workspace, and any other worktrees.
type gitSideFixed struct {
	guigui.DefaultWidget

	owner *GitTool
	items []guigui.LinearLayoutItem

	workLabel basicwidget.Text
	workRows  guigui.WidgetSlice[*gitNavRow]
}

func (f *gitSideFixed) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	t := f.owner
	if t == nil {
		f.workRows.SetLen(0)
		return nil
	}
	context.SetClipChildren(f, true)
	adder.AddWidget(&t.pathLabel)
	adder.AddWidget(&t.workspaceLabel)
	adder.AddWidget(&t.workspaceList)

	var trees []gitcli.Worktree
	enabled := false
	var model *GitModel
	var lang i18n.Lang
	if v, ok := context.Env(f, EnvKeyModel); ok {
		appModel := v.(*Model)
		model = appModel.Git()
		lang = appModel.Lang()
		enabled = model.HasRepo() && !model.Busy()
		trees = model.Snapshot().Worktrees
	}
	f.workRows.SetLen(len(trees))
	if len(trees) == 0 {
		return nil
	}
	setBoldText(&f.workLabel, true)
	f.workLabel.SetValue(i18n.T(lang, i18n.GitWorktrees))
	f.workLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	f.workLabel.SetWrapMode(basicwidget.WrapModeNone)
	f.workLabel.SetSelectable(false)
	adder.AddWidget(&f.workLabel)
	for i, wt := range trees {
		row := f.workRows.At(i)
		dir := wt.Path
		item := basicwidget.ListItem[string]{
			Text:  worktreeTitle(wt),
			Value: "worktree:" + dir,
		}
		var press func()
		if enabled {
			press = func() { t.openWorktree(model, dir) }
		}
		row.Set(item, false, press, nil)
		adder.AddWidget(row)
	}
	return nil
}

func (f *gitSideFixed) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	f.layout(context, bounds.Dx()).LayoutWidgets(context, bounds, layouter)
}

func (f *gitSideFixed) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w, ok := constraints.FixedWidth()
	if !ok || w < 1 {
		w = gitSideDefaultUnits * u
	}
	return f.layout(context, w).Measure(context, guigui.FixedWidthConstraints(w))
}

func (f *gitSideFixed) layout(context *guigui.Context, width int) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	pad := u / 2
	inner := width - pad
	if inner < 1 {
		inner = 1
	}
	f.stack(context, inner, u)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     f.items,
		Padding:   guigui.Padding{End: pad},
	}
}

func (f *gitSideFixed) stack(context *guigui.Context, width, u int) {
	if width < 1 {
		width = 1
	}
	t := f.owner
	f.items = f.items[:0]
	if t == nil {
		return
	}
	add := func(widget guigui.Widget, h int) {
		if h < 1 {
			h = 1
		}
		f.items = append(f.items, guigui.LinearLayoutItem{Widget: widget, Size: guigui.FixedSize(h)})
	}
	add(&t.pathLabel, u*3/4)
	add(&t.workspaceLabel, u*3/4)
	add(&t.workspaceList, gitListHeight(context, &t.workspaceList, width, 2*u))
	gap := u / 4
	if gap < 1 {
		gap = 1
	}
	if f.workRows.Len() > 0 {
		f.items = append(f.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(gap)})
		add(&f.workLabel, u*3/4)
		for i := 0; i < f.workRows.Len(); i++ {
			add(f.workRows.At(i), u)
		}
	}
	f.items = append(f.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(gap)})
}

// gitSideMark is one scrolling section header and its Y in the scroll content.
type gitSideMark struct {
	y    int
	head *gitSectionHead
}

// gitSideBody lays out the scrolling part of the left column.
// Branches, tags, remotes, and submodules each use their full list height.
type gitSideBody struct {
	guigui.DefaultWidget

	owner    *GitTool
	items    []guigui.LinearLayoutItem
	viewport image.Rectangle
	marks    []gitSideMark
	scrollTo func(contentY float64)

	branchHead    gitSectionHead
	tagHead       gitSectionHead
	remoteHead    gitSectionHead
	submoduleHead gitSectionHead
	sticky        guigui.WidgetSlice[*gitSectionHead]
	pins          []int

	branchRows    guigui.WidgetSlice[*gitNavRow]
	tagRows       guigui.WidgetSlice[*gitNavRow]
	remoteRows    guigui.WidgetSlice[*gitNavRow]
	submoduleRows guigui.WidgetSlice[*gitNavRow]
}

func (b *gitSideBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	t := b.owner
	if t == nil {
		return nil
	}
	enabled := false
	var model *GitModel
	var lang i18n.Lang
	if v, ok := context.Env(b, EnvKeyModel); ok {
		appModel := v.(*Model)
		model = appModel.Git()
		lang = appModel.Lang()
		enabled = model.HasRepo() && !model.Busy()
	}
	b.branchHead.Set(i18n.T(lang, i18n.GitBranches), t.branchClosed, func() {
		t.branchClosed = !t.branchClosed
		guigui.RequestRebuild()
	})
	adder.AddWidget(&b.branchHead)
	if t.branchClosed {
		b.branchRows.SetLen(0)
	} else {
		b.syncRows(context, adder, &b.branchRows, visibleTreeItems(t.branchItems), enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.branchFold, value, expanded)
		})
	}
	if t.showTags {
		b.tagHead.Set(i18n.T(lang, i18n.GitTags), t.tagClosed, func() {
			t.tagClosed = !t.tagClosed
			guigui.RequestRebuild()
		})
		adder.AddWidget(&b.tagHead)
		if t.tagClosed {
			b.tagRows.SetLen(0)
		} else {
			b.syncRows(context, adder, &b.tagRows, t.tagItems, enabled, func(value string) {
				t.navSel = value
				if model != nil {
					t.selectRef(model, value)
				}
			}, nil)
		}
	} else {
		b.tagRows.SetLen(0)
	}
	b.remoteHead.Set(i18n.T(lang, i18n.GitRemotes), t.remoteClosed, func() {
		t.remoteClosed = !t.remoteClosed
		guigui.RequestRebuild()
	})
	adder.AddWidget(&b.remoteHead)
	if t.remoteClosed {
		b.remoteRows.SetLen(0)
	} else {
		b.syncRows(context, adder, &b.remoteRows, visibleTreeItems(t.remoteItems), enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.remoteFold, value, expanded)
		})
	}
	b.submoduleHead.Set(i18n.T(lang, i18n.GitSubmodules), t.submoduleClosed, func() {
		t.submoduleClosed = !t.submoduleClosed
		guigui.RequestRebuild()
	})
	adder.AddWidget(&b.submoduleHead)
	if t.submoduleClosed {
		b.submoduleRows.SetLen(0)
	} else {
		b.syncRows(context, adder, &b.submoduleRows, visibleTreeItems(t.submoduleItems), enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.submoduleFold, value, expanded)
		})
	}
	nSticky := 1
	if t.showTags {
		nSticky++
	}
	nSticky += 2
	b.sticky.SetLen(nSticky)
	for i := 0; i < b.sticky.Len(); i++ {
		adder.AddWidget(b.sticky.At(i))
	}
	return nil
}

func (b *gitSideBody) syncRows(context *guigui.Context, adder *guigui.ChildAdder, rows *guigui.WidgetSlice[*gitNavRow], items []basicwidget.ListItem[string], enabled bool, onPress func(string), onExpand func(string, bool)) {
	t := b.owner
	rows.SetLen(len(items))
	for i, item := range items {
		row := rows.At(i)
		value := item.Value
		collapsed := item.Collapsed
		folder := item.Unselectable
		var press func()
		var expand func()
		if folder {
			expand = func() {
				if onExpand != nil {
					onExpand(value, collapsed)
				}
			}
		} else if enabled {
			press = func() {
				if onPress != nil {
					onPress(value)
				}
			}
		}
		row.Set(item, value == t.navSel, press, expand)
		adder.AddWidget(row)
	}
}

func (b *gitSideBody) layout(context *guigui.Context, width int) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	pad := u / 2
	inner := width - pad
	if inner < 1 {
		inner = 1
	}
	b.stack(context, inner, u)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     b.items,
		Padding:   guigui.Padding{End: pad},
	}
}

func (b *gitSideBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	b.layout(context, bounds.Dx()).LayoutWidgets(context, bounds, layouter)
	b.placeSticky(context, bounds, layouter)
}

func (b *gitSideBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w, ok := constraints.FixedWidth()
	if !ok || w < 1 {
		w = gitSideDefaultUnits * u
	}
	return b.layout(context, w).Measure(context, guigui.FixedWidthConstraints(w))
}

func (b *gitSideBody) stack(context *guigui.Context, width, u int) {
	if width < 1 {
		width = 1
	}
	t := b.owner
	b.items = b.items[:0]
	b.marks = b.marks[:0]
	if t == nil {
		return
	}
	y := 0
	add := func(widget guigui.Widget, h int) {
		if h < 1 {
			h = 1
		}
		b.items = append(b.items, guigui.LinearLayoutItem{Widget: widget, Size: guigui.FixedSize(h)})
		y += h
	}
	space := func() {
		h := u / 4
		if h < 1 {
			h = 1
		}
		b.items = append(b.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(h)})
		y += h
	}
	mark := func(head *gitSectionHead) {
		b.marks = append(b.marks, gitSideMark{y: y, head: head})
		add(head, u)
	}
	mark(&b.branchHead)
	if !t.branchClosed {
		for i := 0; i < b.branchRows.Len(); i++ {
			add(b.branchRows.At(i), u)
		}
	}
	space()
	if t.showTags {
		mark(&b.tagHead)
		if !t.tagClosed {
			for i := 0; i < b.tagRows.Len(); i++ {
				add(b.tagRows.At(i), u)
			}
		}
		space()
	}
	mark(&b.remoteHead)
	if !t.remoteClosed {
		for i := 0; i < b.remoteRows.Len(); i++ {
			add(b.remoteRows.At(i), u)
		}
	}
	space()
	mark(&b.submoduleHead)
	if !t.submoduleClosed {
		for i := 0; i < b.submoduleRows.Len(); i++ {
			add(b.submoduleRows.At(i), u)
		}
	}
}

// placeSticky keeps every section header that has scrolled past, piled at the
// top of the viewport. A later header stacks under the earlier ones instead of
// pushing them off.
func (b *gitSideBody) placeSticky(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter) {
	if b.owner == nil {
		return
	}
	u := basicwidget.UnitSize(context)
	viewTop := 0
	if b.viewport.Dy() > 0 || b.viewport.Dx() > 0 {
		viewTop = b.viewport.Min.Y - bounds.Min.Y
	}
	if viewTop < 0 {
		viewTop = 0
	}
	tops := make([]int, len(b.marks))
	for i, mark := range b.marks {
		tops[i] = mark.y
	}
	b.pins = gitStickyStack(b.pins[:0], tops, viewTop, u)
	pad := u / 2
	right := bounds.Max.X - pad
	if right <= bounds.Min.X {
		right = bounds.Min.X + 1
	}
	for i := 0; i < b.sticky.Len(); i++ {
		head := b.sticky.At(i)
		if i >= len(b.pins) || i >= len(b.marks) {
			context.SetVisible(head, false)
			layouter.LayoutWidget(head, image.Rectangle{})
			continue
		}
		context.SetVisible(head, true)
		head.mirror(b.marks[i].head)
		headerY := b.marks[i].y
		index := i
		headerH := u
		head.onToggle = func() {
			if b.scrollTo == nil {
				return
			}
			b.scrollTo(float64(gitStickyScrollTarget(headerY, index, headerH)))
		}
		top := b.viewport.Min.Y + b.pins[i]
		layouter.LayoutWidget(head, image.Rect(bounds.Min.X, top, right, top+u))
	}
}

// gitStickyScrollTarget is the content Y that brings a piled header back into
// the list, just under the headers stacked above it.
func gitStickyScrollTarget(headerY, index, headerH int) int {
	if headerH < 1 {
		headerH = 1
	}
	if index < 0 {
		index = 0
	}
	y := headerY - index*headerH
	if y < 0 {
		return 0
	}
	return y
}

// gitStickyStack lists viewport-relative Y positions for headers that have
// scrolled above their place in the pile. Earlier headers stay put; the next
// one stacks beneath them.
func gitStickyStack(dst, tops []int, viewTop, headerH int) []int {
	if headerH < 1 || len(tops) == 0 {
		return dst[:0]
	}
	dst = dst[:0]
	for _, top := range tops {
		natural := top - viewTop
		stickAt := len(dst) * headerH
		if natural >= stickAt {
			break
		}
		dst = append(dst, stickAt)
	}
	return dst
}

func gitListHeight[T comparable](context *guigui.Context, list *basicwidget.List[T], width, min int) int {
	h := list.Measure(context, guigui.FixedWidthConstraints(width)).Y
	if h < min {
		return min
	}
	return h
}
