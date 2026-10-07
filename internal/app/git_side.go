package app

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

const gitSideDefaultUnits = 9

// gitSidePane is the scrolling left column: workspace, branches, tags, and remotes.
type gitSidePane struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	body  gitSideBody
}

func (p *gitSidePane) setOwner(t *GitTool) {
	p.body.owner = t
}

func (p *gitSidePane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.panel)
	p.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.panel.SetAutoBorder(false)
	p.panel.SetBorders(basicwidget.PanelBorders{})
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	p.panel.SetContent(&p.body)
	return nil
}

func (p *gitSidePane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&p.panel, widgetBounds.Bounds())
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
		&t.detail,
		&t.commitList,
		&t.graphEmpty,
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

// gitSideBody lays out the left column at the height of its contents.
// Branches, tags, and remotes each use their full list height. The pane scrolls.
type gitSideBody struct {
	guigui.DefaultWidget

	owner *GitTool
	items []guigui.LinearLayoutItem

	branchHead gitSectionHead
	tagHead    gitSectionHead
	remoteHead gitSectionHead

	branchRows guigui.WidgetSlice[*gitNavRow]
	tagRows    guigui.WidgetSlice[*gitNavRow]
	remoteRows guigui.WidgetSlice[*gitNavRow]
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
	adder.AddWidget(&t.repoTitle)
	adder.AddWidget(&t.pathLabel)
	adder.AddWidget(&t.workspaceLabel)
	adder.AddWidget(&t.workspaceList)
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
	add := func(widget guigui.Widget, h int) {
		if h < 1 {
			h = 1
		}
		b.items = append(b.items, guigui.LinearLayoutItem{Widget: widget, Size: guigui.FixedSize(h)})
	}
	space := func() {
		b.items = append(b.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(u / 4)})
	}
	add(&t.repoTitle, u)
	add(&t.pathLabel, u*3/4)
	add(&t.workspaceLabel, u*3/4)
	add(&t.workspaceList, gitListHeight(context, &t.workspaceList, width, 2*u))
	space()
	add(&b.branchHead, u)
	if !t.branchClosed {
		for i := 0; i < b.branchRows.Len(); i++ {
			add(b.branchRows.At(i), u)
		}
	}
	space()
	if t.showTags {
		add(&b.tagHead, u)
		if !t.tagClosed {
			for i := 0; i < b.tagRows.Len(); i++ {
				add(b.tagRows.At(i), u)
			}
		}
		space()
	}
	add(&b.remoteHead, u)
	if !t.remoteClosed {
		for i := 0; i < b.remoteRows.Len(); i++ {
			add(b.remoteRows.At(i), u)
		}
	}
}

func gitListHeight[T comparable](context *guigui.Context, list *basicwidget.List[T], width, min int) int {
	h := list.Measure(context, guigui.FixedWidthConstraints(width)).Y
	if h < min {
		return min
	}
	return h
}
