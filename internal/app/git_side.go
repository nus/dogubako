package app

import (
	"image"
	"path"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

const gitSideDefaultUnits = 9

// gitSidePane is the left column. The workspace, including other worktrees,
// stays put. Branches, tags, remotes, and submodules scroll beneath it.
// A filter box stays fixed along the bottom.
type gitSidePane struct {
	guigui.DefaultWidget

	fixed  gitSideFixed
	panel  basicwidget.Panel
	body   gitSideBody
	filter gitSideFilter
}

func (p *gitSidePane) setOwner(t *GitTool) {
	p.fixed.owner = t
	p.body.owner = t
	p.filter.owner = t
	p.body.scrollTo = func(contentY float64) {
		p.panel.SetScrollOffset(0, -contentY)
	}
}

func (p *gitSidePane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(p, true)
	adder.AddWidget(&p.fixed)
	adder.AddWidget(&p.panel)
	adder.AddWidget(&p.filter)
	p.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.panel.SetAutoBorder(false)
	p.panel.SetBorders(basicwidget.PanelBorders{})
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	p.panel.SetContent(&p.body)
	return nil
}

func (p *gitSidePane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	filterH := p.filter.Measure(context, guigui.FixedWidthConstraints(bounds.Dx())).Y
	if filterH > bounds.Dy() {
		filterH = bounds.Dy()
	}
	if filterH < 0 {
		filterH = 0
	}
	filterB := image.Rect(bounds.Min.X, bounds.Max.Y-filterH, bounds.Max.X, bounds.Max.Y)
	restMaxY := filterB.Min.Y
	if restMaxY < bounds.Min.Y {
		restMaxY = bounds.Min.Y
	}
	rest := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, restMaxY)
	fixedH := p.fixed.Measure(context, guigui.FixedWidthConstraints(rest.Dx())).Y
	if fixedH > rest.Dy() {
		fixedH = rest.Dy()
	}
	if fixedH < 0 {
		fixedH = 0
	}
	fixedB := image.Rect(rest.Min.X, rest.Min.Y, rest.Max.X, rest.Min.Y+fixedH)
	scrollB := image.Rect(rest.Min.X, fixedB.Max.Y, rest.Max.X, rest.Max.Y)
	if scrollB.Max.Y < scrollB.Min.Y {
		scrollB.Max.Y = scrollB.Min.Y
	}
	p.body.viewport = scrollB
	layouter.LayoutWidget(&p.fixed, fixedB)
	layouter.LayoutWidget(&p.panel, scrollB)
	layouter.LayoutWidget(&p.filter, filterB)
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

// gitSideFilter is the text box fixed to the bottom of the left column.
// Its text keeps branches, tags, and remote branches that contain it.
// A short gap sits under the field. A clear mark sits inside it.
type gitSideFilter struct {
	guigui.DefaultWidget

	owner *GitTool
	input basicwidget.TextInput
	clear gitFilterClear
}

func (f *gitSideFilter) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(f, true)
	adder.AddWidget(&f.input)
	var lang i18n.Lang
	if v, ok := context.Env(f, EnvKeyModel); ok {
		lang = v.(*Model).Lang()
	}
	f.input.SetPlaceholder(i18n.T(lang, i18n.GitFilter))
	f.input.OnValueChanged(func(_ *guigui.Context, text string, _ bool) {
		if f.owner == nil || f.owner.refFilter == text {
			return
		}
		f.owner.refFilter = text
		guigui.RequestRebuild()
	})
	f.clear.clear = f.clearText
	if f.input.HasValue() {
		adder.AddWidget(&f.clear)
	}
	return nil
}

func (f *gitSideFilter) clearText() {
	f.input.ForceSetValue("")
	if f.owner != nil {
		f.owner.refFilter = ""
	}
	guigui.RequestRebuild()
}

func (f *gitSideFilter) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	field := gitSideFilterInputBounds(widgetBounds.Bounds(), u)
	layouter.LayoutWidget(&f.input, field)
	layouter.LayoutWidget(&f.clear, gitFilterClearBounds(field, u))
}

func (f *gitSideFilter) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w, ok := constraints.FixedWidth()
	if !ok || w < 1 {
		w = gitSideDefaultUnits * u
	}
	px, top := gitSideFilterPad(u)
	inner := w - 2*px
	if inner < 1 {
		inner = 1
	}
	h := f.input.Measure(context, guigui.FixedWidthConstraints(inner)).Y
	return image.Pt(w, h+top+gitSideFilterBottom(u))
}

func (f *gitSideFilter) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	fillRect(dst, b, basicwidgetdraw.BackgroundColor(context.ColorMode()))
	fillRect(dst, image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+1), gitHistSplitColor(context.ColorMode()))
}

func gitSideFilterPad(u int) (x, top int) {
	x = u / 2
	top = u / 4
	if x < 1 {
		x = 1
	}
	if top < 1 {
		top = 1
	}
	return x, top
}

// gitSideFilterBottom is the gap under the field, about a fifth of a line.
func gitSideFilterBottom(u int) int {
	if u < 1 {
		return 0
	}
	return (u*2 + 5) / 10
}

// gitSideFilterInputBounds keeps a gap above the field and a short one below it.
func gitSideFilterInputBounds(b image.Rectangle, u int) image.Rectangle {
	px, top := gitSideFilterPad(u)
	in := image.Rect(b.Min.X+px, b.Min.Y+top, b.Max.X-px, b.Max.Y-gitSideFilterBottom(u))
	if in.Max.X < in.Min.X {
		in.Max.X = in.Min.X
	}
	if in.Max.Y < in.Min.Y {
		in.Max.Y = in.Min.Y
	}
	return in
}

// gitFilterClearBounds is the clear mark inside the field, on the trailing edge.
func gitFilterClearBounds(field image.Rectangle, u int) image.Rectangle {
	if u < 1 {
		u = 1
	}
	inset := u / 6
	if inset < 2 {
		inset = 2
	}
	size := u * 2 / 3
	if size < 12 {
		size = 12
	}
	limit := field.Dy() - 2*inset
	if limit < 1 {
		limit = 1
	}
	if size > limit {
		size = limit
	}
	x := field.Max.X - inset - size
	if x < field.Min.X {
		x = field.Min.X
	}
	y := field.Min.Y + (field.Dy()-size)/2
	return image.Rect(x, y, x+size, y+size)
}

// gitFilterClear is the × inside the filter field. It empties the field.
type gitFilterClear struct {
	guigui.DefaultWidget

	mark  basicwidget.Text
	hover bool
	clear func()
}

func (c *gitFilterClear) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&c.mark)
	context.SetPassthrough(&c.mark, true)
	c.mark.SetValue("×")
	c.mark.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.mark.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	c.mark.SetScale(0.85)
	c.mark.SetSelectable(false)
	return nil
}

func (c *gitFilterClear) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&c.mark, widgetBounds.Bounds())
}

func (c *gitFilterClear) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if !c.hover {
		return
	}
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	cx := float32(b.Min.X + b.Dx()/2)
	cy := float32(b.Min.Y + b.Dy()/2)
	vector.FillCircle(dst, cx, cy, float32(b.Dx())*0.5, gitTabCloseHoverColor(context.ColorMode()), true)
}

func (c *gitFilterClear) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	c.hover = widgetBounds.IsHitAtCursor()
	return nil
}

func (c *gitFilterClear) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteBool(c.hover)
}

func (c *gitFilterClear) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if c.clear != nil {
		c.clear()
	}
	return guigui.HandleInputByWidget(c)
}

func (c *gitFilterClear) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
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

	lists gitSideLists
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
	b.lists = gitSideRefLists(t)
	if b.lists.branchHead {
		b.branchHead.Set(i18n.T(lang, i18n.GitBranches), !b.lists.branchOpen, func() {
			t.branchClosed = !t.branchClosed
			guigui.RequestRebuild()
		})
		adder.AddWidget(&b.branchHead)
	}
	if b.lists.branchOpen {
		b.syncRows(context, adder, &b.branchRows, b.lists.branch, enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.branchFold, value, expanded)
		})
	} else {
		b.branchRows.SetLen(0)
	}
	if b.lists.tagHead {
		b.tagHead.Set(i18n.T(lang, i18n.GitTags), !b.lists.tagOpen, func() {
			t.tagClosed = !t.tagClosed
			guigui.RequestRebuild()
		})
		adder.AddWidget(&b.tagHead)
	}
	if b.lists.tagOpen {
		b.syncRows(context, adder, &b.tagRows, b.lists.tag, enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, nil)
	} else {
		b.tagRows.SetLen(0)
	}
	if b.lists.remoteHead {
		b.remoteHead.Set(i18n.T(lang, i18n.GitRemotes), !b.lists.remoteOpen, func() {
			t.remoteClosed = !t.remoteClosed
			guigui.RequestRebuild()
		})
		adder.AddWidget(&b.remoteHead)
	}
	if b.lists.remoteOpen {
		b.syncRows(context, adder, &b.remoteRows, b.lists.remote, enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.remoteFold, value, expanded)
		})
	} else {
		b.remoteRows.SetLen(0)
	}
	if b.lists.subHead {
		b.submoduleHead.Set(i18n.T(lang, i18n.GitSubmodules), !b.lists.subOpen, func() {
			t.submoduleClosed = !t.submoduleClosed
			guigui.RequestRebuild()
		})
		adder.AddWidget(&b.submoduleHead)
	}
	if b.lists.subOpen {
		b.syncRows(context, adder, &b.submoduleRows, b.lists.sub, enabled, func(value string) {
			t.navSel = value
			if model != nil {
				t.selectRef(model, value)
			}
		}, func(value string, expanded bool) {
			t.setTreeFold(&t.submoduleFold, value, expanded)
		})
	} else {
		b.submoduleRows.SetLen(0)
	}
	b.sticky.SetLen(b.lists.stickyCount())
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
	addSection := func(on bool, head *gitSectionHead, open bool, rows *guigui.WidgetSlice[*gitNavRow]) {
		if !on {
			return
		}
		if len(b.items) > 0 {
			space()
		}
		mark(head)
		if !open {
			return
		}
		for i := 0; i < rows.Len(); i++ {
			add(rows.At(i), u)
		}
	}
	addSection(b.lists.branchHead, &b.branchHead, b.lists.branchOpen, &b.branchRows)
	addSection(b.lists.tagHead, &b.tagHead, b.lists.tagOpen, &b.tagRows)
	addSection(b.lists.remoteHead, &b.remoteHead, b.lists.remoteOpen, &b.remoteRows)
	addSection(b.lists.subHead, &b.submoduleHead, b.lists.subOpen, &b.submoduleRows)
}

// gitSideLists is which branch, tag, remote, and submodule rows the sidebar shows.
// A non-empty filter keeps only names that contain it. Section headers stay up
// even when every row is filtered out.
type gitSideLists struct {
	branch     []basicwidget.ListItem[string]
	tag        []basicwidget.ListItem[string]
	remote     []basicwidget.ListItem[string]
	sub        []basicwidget.ListItem[string]
	branchHead bool
	tagHead    bool
	remoteHead bool
	subHead    bool
	branchOpen bool
	tagOpen    bool
	remoteOpen bool
	subOpen    bool
}

func gitSideRefLists(t *GitTool) gitSideLists {
	var out gitSideLists
	if t == nil {
		return out
	}
	query := strings.TrimSpace(t.refFilter)
	out.branch, out.branchHead, out.branchOpen = gitSideSection(t.branchItems, query, t.branchClosed, true)
	if t.showTags {
		out.tag, out.tagHead, out.tagOpen = gitSideSection(t.tagItems, query, t.tagClosed, false)
	}
	out.remote, out.remoteHead, out.remoteOpen = gitSideSection(t.remoteItems, query, t.remoteClosed, true)
	out.sub, out.subHead, out.subOpen = gitSideSection(t.submoduleItems, query, t.submoduleClosed, true)
	return out
}

// gitSideSection keeps the header. A filter narrows the rows. A closed section
// stays shut when nothing matches, and opens when a row does.
func gitSideSection(items []basicwidget.ListItem[string], query string, closed, tree bool) (rows []basicwidget.ListItem[string], head, open bool) {
	head = true
	if strings.TrimSpace(query) != "" {
		rows = gitFilterRefItems(items, query)
		open = !closed || len(rows) > 0
		if !open {
			rows = nil
		}
		return rows, head, open
	}
	open = !closed
	if !open {
		return nil, head, open
	}
	if tree {
		return visibleTreeItems(items), head, open
	}
	return items, head, open
}

// stickyCount is the section headers that can pin to the top of the list.
func (l gitSideLists) stickyCount() int {
	n := 0
	if l.branchHead {
		n++
	}
	if l.tagHead {
		n++
	}
	if l.remoteHead {
		n++
	}
	if l.subHead {
		n++
	}
	return n
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
