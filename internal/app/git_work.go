package app

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitWorkPanes is the working copy.
// The top band places the staging lists on the left and the commit form on the right.
// The diff fills the area under that band.
type gitWorkPanes struct {
	guigui.DefaultWidget

	unstaged   gitFileSection
	staged     gitFileSection
	filesPanel basicwidget.Panel
	files      gitFileColumn
	diff       gitWorkDiff
	split      gitWorkSplit
	hSplit     gitWorkHSplit

	// sideStaged is true when the diff shows the staged list's selection.
	sideStaged bool

	commitInput *guigui.WidgetWithSize[*basicwidget.TextInput]
	stageBtn    *basicwidget.Button
	unstageBtn  *basicwidget.Button
	amendBtn    *basicwidget.Button
	commitBtn   *basicwidget.Button
	btnItems    []guigui.LinearLayoutItem

	// empty replaces the staging lists when the work tree is clean.
	empty    *basicwidget.Text
	showDiff bool

	// filesFrac is the staging column's share of the top-band width.
	// Zero uses half.
	filesFrac float64
	// topFrac is the top band's share of the height beside the splitter.
	// Zero uses two fifths, leaving the rest for the diff.
	topFrac   float64
	dragging  bool
	hDragging bool
	bounds    image.Rectangle
}

func (p *gitWorkPanes) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if p.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.dragging = false
		p.setDragPassthrough(context, false)
	}
	if p.hDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.hDragging = false
		p.setDragPassthrough(context, false)
	}
	p.split.panes = p
	p.hSplit.panes = p
	p.files.unstaged = &p.unstaged
	p.files.staged = &p.staged
	if p.empty != nil {
		adder.AddWidget(p.empty)
	} else {
		adder.AddWidget(&p.filesPanel)
		p.filesPanel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
		p.filesPanel.SetAutoBorder(false)
		p.filesPanel.SetBorders(basicwidget.PanelBorders{})
		p.filesPanel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
		p.filesPanel.SetContent(&p.files)
	}
	adder.AddWidget(&p.split)
	p.addCommitWidgets(adder)
	if p.showDiff {
		adder.AddWidget(&p.hSplit)
		adder.AddWidget(&p.diff)
	}
	return nil
}

func (p *gitWorkPanes) addCommitWidgets(adder *guigui.ChildAdder) {
	if p.commitInput != nil {
		adder.AddWidget(p.commitInput)
	}
	if p.stageBtn != nil {
		adder.AddWidget(p.stageBtn)
	}
	if p.unstageBtn != nil {
		adder.AddWidget(p.unstageBtn)
	}
	if p.amendBtn != nil {
		adder.AddWidget(p.amendBtn)
	}
	if p.commitBtn != nil {
		adder.AddWidget(p.commitBtn)
	}
}

func (p *gitWorkPanes) setDragPassthrough(context *guigui.Context, pass bool) {
	context.SetPassthrough(&p.filesPanel, pass)
	context.SetPassthrough(&p.diff, pass)
	context.SetPassthrough(&p.split, pass)
	context.SetPassthrough(&p.hSplit, pass)
	if p.empty != nil {
		context.SetPassthrough(p.empty, pass)
	}
	if p.commitInput != nil {
		context.SetPassthrough(p.commitInput, pass)
	}
	if p.stageBtn != nil {
		context.SetPassthrough(p.stageBtn, pass)
	}
	if p.unstageBtn != nil {
		context.SetPassthrough(p.unstageBtn, pass)
	}
	if p.amendBtn != nil {
		context.SetPassthrough(p.amendBtn, pass)
	}
	if p.commitBtn != nil {
		context.SetPassthrough(p.commitBtn, pass)
	}
}

func (p *gitWorkPanes) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	p.bounds = b
	top := b
	if p.showDiff {
		splitH := gitWorkSplitWidth(u)
		topH := gitWorkBandHeight(b.Dy(), splitH, p.topFrac, 6*u, 8*u)
		top = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+topH)
		splitB := image.Rect(b.Min.X, top.Max.Y, b.Max.X, top.Max.Y+splitH)
		diffB := image.Rect(b.Min.X, splitB.Max.Y, b.Max.X, b.Max.Y)
		if diffB.Min.Y > diffB.Max.Y {
			diffB.Min.Y = diffB.Max.Y
		}
		layouter.LayoutWidget(&p.hSplit, splitB)
		layouter.LayoutWidget(&p.diff, diffB)
	}

	splitW := gitWorkSplitWidth(u)
	stageFrac := p.filesFrac
	if stageFrac <= 0 {
		stageFrac = 0.5
	}
	stageW := gitWorkFilesWidth(top.Dx(), splitW, stageFrac, 6*u, 8*u)
	stageB := image.Rect(top.Min.X, top.Min.Y, top.Min.X+stageW, top.Max.Y)
	splitB := image.Rect(stageB.Max.X, top.Min.Y, stageB.Max.X+splitW, top.Max.Y)
	commitB := image.Rect(splitB.Max.X, top.Min.Y, top.Max.X, top.Max.Y)
	if p.empty != nil {
		layouter.LayoutWidget(p.empty, stageB)
	} else {
		layouter.LayoutWidget(&p.filesPanel, stageB)
	}
	layouter.LayoutWidget(&p.split, splitB)
	p.layoutCommitForm(context, commitB, layouter, u)
}

// gitWorkBandHeight is the staging-and-commit band's height.
// frac is that band's share of the space beside the splitter. Zero uses two fifths.
func gitWorkBandHeight(total, splitH int, frac float64, minBand, minDiff int) int {
	if frac <= 0 {
		frac = 0.4
	}
	return gitWorkFilesWidth(total, splitH, frac, minBand, minDiff)
}

func (p *gitWorkPanes) layoutCommitForm(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter, u int) {
	if p.commitInput == nil || bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	btnH := u
	gap := u / 6
	msgMaxY := bounds.Max.Y - btnH - gap
	if msgMaxY < bounds.Min.Y {
		msgMaxY = bounds.Min.Y
	}
	layouter.LayoutWidget(p.commitInput, image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, msgMaxY))
	btnY := bounds.Max.Y - btnH
	if btnY < bounds.Min.Y {
		btnY = bounds.Min.Y
	}
	p.btnItems = p.btnItems[:0]
	if p.stageBtn != nil {
		p.btnItems = append(p.btnItems, guigui.LinearLayoutItem{Widget: p.stageBtn})
	}
	if p.unstageBtn != nil {
		p.btnItems = append(p.btnItems, guigui.LinearLayoutItem{Widget: p.unstageBtn})
	}
	p.btnItems = append(p.btnItems, guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)})
	if p.amendBtn != nil {
		p.btnItems = append(p.btnItems, guigui.LinearLayoutItem{Widget: p.amendBtn})
	}
	if p.commitBtn != nil {
		p.btnItems = append(p.btnItems, guigui.LinearLayoutItem{Widget: p.commitBtn})
	}
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     p.btnItems,
		Gap:       u / 4,
	}).LayoutWidgets(context, image.Rect(bounds.Min.X, btnY, bounds.Max.X, bounds.Max.Y), layouter)
}

// gitFileColumn is the scrolling file column: unstaged, then staged when present.
type gitFileColumn struct {
	guigui.DefaultWidget

	unstaged *gitFileSection
	staged   *gitFileSection
}

func (c *gitFileColumn) showStaged() bool {
	return c.staged != nil && len(c.staged.fileItems) > 0
}

func (c *gitFileColumn) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if c.unstaged != nil {
		adder.AddWidget(c.unstaged)
	}
	if c.showStaged() {
		adder.AddWidget(c.staged)
	}
	return nil
}

func (c *gitFileColumn) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	if c.unstaged == nil {
		return
	}
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	y := b.Min.Y
	h := c.unstaged.contentHeight(context, b.Dx())
	layouter.LayoutWidget(c.unstaged, image.Rect(b.Min.X, y, b.Max.X, y+h))
	if !c.showStaged() {
		return
	}
	y += h + u/3
	h = c.staged.contentHeight(context, b.Dx())
	layouter.LayoutWidget(c.staged, image.Rect(b.Min.X, y, b.Max.X, y+h))
}

func (c *gitFileColumn) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 10 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	if c.unstaged == nil {
		return image.Pt(w, u)
	}
	h := c.unstaged.contentHeight(context, w)
	if c.showStaged() {
		h += u/3 + c.staged.contentHeight(context, w)
	}
	return image.Pt(w, h)
}

func gitWorkSplitWidth(u int) int {
	w := u / 2
	if w < 6 {
		return 6
	}
	return w
}

// gitWorkFilesWidth is the file-list column width for a pane of total pixels.
// frac is the file-list share of the space beside the splitter. Zero uses one third.
func gitWorkFilesWidth(total, splitW int, frac float64, minFiles, minDiff int) int {
	avail := total - splitW
	if avail <= 1 {
		if avail < 0 {
			return 0
		}
		return avail
	}
	if frac <= 0 {
		frac = 1.0 / 3.0
	}
	filesW := int(float64(avail) * frac)
	if minFiles > avail-1 {
		minFiles = avail / 3
		if minFiles < 1 {
			minFiles = 1
		}
	}
	if filesW < minFiles {
		filesW = minFiles
	}
	maxFiles := avail - minDiff
	if maxFiles < minFiles {
		maxFiles = avail - 1
	}
	if filesW > maxFiles {
		filesW = maxFiles
	}
	if filesW < 1 {
		filesW = 1
	}
	return filesW
}

func (p *gitWorkPanes) setFilesFracFromCursor(x, u int) {
	splitW := gitWorkSplitWidth(u)
	avail := p.bounds.Dx() - splitW
	if avail <= 0 {
		return
	}
	filesW := x - p.bounds.Min.X - splitW/2
	frac := float64(filesW) / float64(avail)
	minFrac := float64(6*u) / float64(avail)
	maxFrac := float64(avail-8*u) / float64(avail)
	if maxFrac < minFrac {
		maxFrac = minFrac
	}
	if frac < minFrac {
		frac = minFrac
	}
	if frac > maxFrac {
		frac = maxFrac
	}
	p.filesFrac = frac
}

func (p *gitWorkPanes) dragSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !p.dragging {
		if p.hDragging || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		p.dragging = true
	}
	p.setFilesFracFromCursor(image.Pt(ebiten.CursorPosition()).X, u)
	p.setDragPassthrough(context, true)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.dragging = false
		p.setDragPassthrough(context, false)
	}
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(p)
}

func (p *gitWorkPanes) setTopFracFromCursor(y, u int) {
	splitH := gitWorkSplitWidth(u)
	avail := p.bounds.Dy() - splitH
	if avail <= 0 {
		return
	}
	minBand := 6 * u
	minDiff := 8 * u
	bandH := y - p.bounds.Min.Y - splitH/2
	frac := float64(bandH) / float64(avail)
	minFrac := float64(minBand) / float64(avail)
	maxFrac := float64(avail-minDiff) / float64(avail)
	if maxFrac < minFrac {
		maxFrac = minFrac
	}
	if frac < minFrac {
		frac = minFrac
	}
	if frac > maxFrac {
		frac = maxFrac
	}
	if frac <= 0 {
		frac = 0.01
	}
	p.topFrac = frac
}

func (p *gitWorkPanes) dragHSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !p.hDragging {
		if p.dragging || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		p.hDragging = true
	}
	p.setTopFracFromCursor(image.Pt(ebiten.CursorPosition()).Y, u)
	p.setDragPassthrough(context, true)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.hDragging = false
		p.setDragPassthrough(context, false)
	}
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(p)
}

func (p *gitWorkPanes) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if p.dragging {
		return p.dragSplit(context)
	}
	if p.hDragging {
		return p.dragHSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (p *gitWorkPanes) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	released := false
	if p.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.dragging = false
		released = true
	}
	if p.hDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.hDragging = false
		released = true
	}
	if released {
		p.setDragPassthrough(context, false)
		guigui.RequestRebuild()
	}
	return nil
}

func (p *gitWorkPanes) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if p.hDragging {
		return ebiten.CursorShapeNSResize, true
	}
	if p.dragging {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

// gitWorkSplit is the draggable vertical boundary between the staging lists and the commit form.
type gitWorkSplit struct {
	guigui.DefaultWidget

	panes *gitWorkPanes
}

func (s *gitWorkSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.panes == nil || s.panes.hDragging {
		return guigui.HandleInputResult{}
	}
	if s.panes.dragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.panes.dragSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitWorkSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.panes != nil && s.panes.hDragging {
		return 0, false
	}
	if s.panes != nil && s.panes.dragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (s *gitWorkSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	x := b.Min.X + b.Dx()/2
	fillRect(dst, image.Rect(x, b.Min.Y, x+1, b.Max.Y), gitHistSplitColor(context.ColorMode()))
}

// gitWorkHSplit is the draggable horizontal boundary between the commit band and the diff.
type gitWorkHSplit struct {
	guigui.DefaultWidget

	panes *gitWorkPanes
}

func (s *gitWorkHSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.panes == nil || s.panes.dragging {
		return guigui.HandleInputResult{}
	}
	if s.panes.hDragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.panes.dragHSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitWorkHSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.panes != nil && s.panes.dragging {
		return 0, false
	}
	if s.panes != nil && s.panes.hDragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func (s *gitWorkHSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	y := b.Min.Y + b.Dy()/2
	fillRect(dst, image.Rect(b.Min.X, y, b.Max.X, y+1), gitHistSplitColor(context.ColorMode()))
}

func (p *gitWorkPanes) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsKeyJustPressed(ebiten.KeySpace) || shortcutModifierPressed(context) {
		return guigui.HandleInputResult{}
	}
	// A focused file list handles Space itself. From the diff, Space follows the shown side.
	if !context.IsFocusedOrHasFocusedDescendant(&p.diff) {
		return guigui.HandleInputResult{}
	}
	section := &p.unstaged
	if p.sideStaged {
		section = &p.staged
	}
	if !context.IsEnabled(&section.list) || !section.toggleSelection() {
		return guigui.HandleInputResult{}
	}
	return guigui.HandleInputByWidget(p)
}

func (p *gitWorkPanes) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 12 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	return image.Pt(w, 8*u)
}

// gitFileSection is the unstaged or staged file list.
type gitFileSection struct {
	guigui.DefaultWidget

	title basicwidget.Text
	list  basicwidget.List[string]
	rows  guigui.WidgetSlice[*gitFileRow]

	items []guigui.LinearLayoutItem

	fileItems []basicwidget.ListItem[string]
	sel       []string

	checked bool
	onCheck func(paths []string, checked bool)
}

func (s *gitFileSection) SetTitle(title string) {
	setBoldText(&s.title, true)
	s.title.SetValue(title)
	s.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
}

func (s *gitFileSection) contentHeight(context *guigui.Context, width int) int {
	u := basicwidget.UnitSize(context)
	cons := guigui.Constraints{}
	if width > 0 {
		cons = guigui.FixedWidthConstraints(width)
	}
	listH := s.list.Measure(context, cons).Y
	if listH < u {
		listH = u
	}
	return u + u/6 + listH
}

func (s *gitFileSection) selectedPath() string {
	if len(s.sel) == 0 {
		return ""
	}
	return s.sel[0]
}

func (s *gitFileSection) setFiles(context *guigui.Context, entries []gitcli.StatusEntry, checked bool, enabled bool, pickEmpty bool, onPick func(), onCheck func(paths []string, checked bool)) {
	s.checked = checked
	s.onCheck = onCheck
	have := make(map[string]bool, len(entries))
	s.rows.SetLen(len(entries))
	s.fileItems = slices.Delete(s.fileItems, 0, len(s.fileItems))
	for i, e := range entries {
		have[e.Path] = true
		row := s.rows.At(i)
		row.Set(e, checked, enabled, onCheck)
		s.fileItems = append(s.fileItems, basicwidget.ListItem[string]{
			Content: row,
			Value:   e.Path,
		})
	}
	kept := make([]string, 0, len(s.sel))
	for _, p := range s.sel {
		if have[p] {
			kept = append(kept, p)
		}
	}
	s.sel = kept
	if pickEmpty && len(s.sel) == 0 && len(s.fileItems) > 0 {
		s.sel = []string{s.fileItems[0].Value}
	}
	s.list.SetStyle(basicwidget.ListStyleNormal)
	s.list.SetHighlightVisibleWhenUnfocused(true)
	s.list.SetMultiSelection(true)
	s.list.SetItems(s.fileItems)
	s.list.OnItemsSelected(func(context *guigui.Context, indices []int) {
		paths := make([]string, 0, len(indices))
		for _, i := range indices {
			item, ok := s.list.ItemByIndex(i)
			if ok && item.Value != "" {
				paths = append(paths, item.Value)
			}
		}
		if samePaths(s.sel, paths) {
			return
		}
		s.sel = paths
		if onPick != nil {
			onPick()
		}
		guigui.RequestRebuild()
	})
	if len(s.sel) > 0 {
		s.list.SelectItemsByValues(s.sel)
	}
	context.SetEnabled(&s.list, enabled)
}

// toggleSelection stages an unstaged selection and unstages a staged selection.
func (s *gitFileSection) toggleSelection() bool {
	if s.onCheck == nil || len(s.sel) == 0 {
		return false
	}
	s.onCheck(append([]string(nil), s.sel...), !s.checked)
	return true
}

func (s *gitFileSection) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsKeyJustPressed(ebiten.KeySpace) || shortcutModifierPressed(context) {
		return guigui.HandleInputResult{}
	}
	if !context.IsEnabled(&s.list) || !s.toggleSelection() {
		return guigui.HandleInputResult{}
	}
	return guigui.HandleInputByWidget(s)
}

func (s *gitFileSection) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.title)
	adder.AddWidget(&s.list)
	return nil
}

func (s *gitFileSection) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	s.items = s.items[:0]
	s.items = append(s.items,
		guigui.LinearLayoutItem{Widget: &s.title, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &s.list, Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     s.items,
		Gap:       u / 6,
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (s *gitFileSection) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 10 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	return image.Pt(w, 6*u)
}

// gitChangeRenamed reports a rename or copy in either porcelain column.
func gitChangeRenamed(code string) bool {
	for i := 0; i < len(code) && i < 2; i++ {
		if code[i] == 'R' || code[i] == 'C' {
			return true
		}
	}
	return false
}

// gitChangeFileLabel splits a porcelain path into the file name and the
// directory that contains it. A rename or copy uses the destination. A file
// at the repository root has an empty directory.
func gitChangeFileLabel(code, path string) (name, dir string) {
	if gitChangeRenamed(code) {
		// Porcelain joins a rename as "old -> new". A path can itself contain
		// the separator, so the destination is the last piece.
		if i := strings.LastIndex(path, " -> "); i >= 0 {
			path = path[i+len(" -> "):]
		}
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" || path == "." {
		return path, ""
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path, ""
	}
	return path[i+1:], path[:i]
}

// gitChangeMark is the one-letter status drawn at the right of a row.
// staged selects the index column; otherwise the work-tree column.
func gitChangeMark(code string, staged bool) string {
	if code == "" {
		return ""
	}
	var c byte
	if staged || len(code) == 1 {
		c = code[0]
	} else {
		c = code[1]
	}
	if c == ' ' || c == 0 {
		return ""
	}
	return string(c)
}

func gitChangeMarkColor(mark string, dark bool) color.NRGBA {
	switch mark {
	case "A", "?", "C":
		if dark {
			return color.NRGBA{R: 0x73, G: 0xc9, B: 0x91, A: 0xff}
		}
		return color.NRGBA{R: 0x1a, G: 0x7f, B: 0x37, A: 0xff}
	case "D":
		if dark {
			return color.NRGBA{R: 0xf8, G: 0x51, B: 0x49, A: 0xff}
		}
		return color.NRGBA{R: 0xcf, G: 0x22, B: 0x2e, A: 0xff}
	default:
		if dark {
			return color.NRGBA{R: 0xe2, G: 0xc0, B: 0x8d, A: 0xff}
		}
		return color.NRGBA{R: 0x9a, G: 0x67, B: 0x00, A: 0xff}
	}
}

// gitChangeLabelWidths splits the space after the icon between the file name
// and its directory. The name keeps its natural width. The directory takes
// what remains and can ellipsize. When the name alone fills the row, the
// directory width is zero.
func gitChangeLabelWidths(avail, nameW, dirW, gap int) (name, dir int) {
	if avail < 0 {
		avail = 0
	}
	if dirW <= 0 {
		if nameW > avail {
			return avail, 0
		}
		return nameW, 0
	}
	if nameW+gap+dirW <= avail {
		return nameW, dirW
	}
	if nameW >= avail {
		return avail, 0
	}
	room := avail - nameW - gap
	if room < 1 {
		return nameW, 0
	}
	return nameW, room
}

// gitFileRow is one working-copy file.
// A checkbox stages or unstages it. The label is the file name, then the
// directory that contains it. Every file uses the same glyph.
type gitFileRow struct {
	guigui.DefaultWidget

	box  basicwidget.Checkbox
	icon fileKindIcon
	name basicwidget.Text
	dir  basicwidget.Text
	mark basicwidget.Text

	nameStyle basicwidget.TextStyle
	dirStyle  basicwidget.TextStyle
	markStyle basicwidget.TextStyle

	path    string
	onCheck func(paths []string, checked bool)
}

func (r *gitFileRow) Set(entry gitcli.StatusEntry, checked, sync bool, onCheck func([]string, bool)) {
	r.path = entry.Path
	r.onCheck = onCheck
	name, dir := gitChangeFileLabel(entry.Code, entry.Path)
	r.name.SetValue(name)
	r.dir.SetValue(dir)
	r.mark.SetValue(gitChangeMark(entry.Code, checked))
	r.icon.SetFolder(false)
	if sync {
		r.box.SetValue(checked)
	}
}

func (r *gitFileRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	r.name.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.name.SetWrapMode(basicwidget.WrapModeNone)
	r.name.SetEllipsisString("…")
	r.name.SetSelectable(false)
	r.dir.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.dir.SetWrapMode(basicwidget.WrapModeNone)
	r.dir.SetEllipsisString("…")
	r.dir.SetSelectable(false)
	r.dir.SetOpacity(0.62)
	r.mark.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.mark.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	r.mark.SetWrapMode(basicwidget.WrapModeNone)
	r.mark.SetSelectable(false)
	r.box.OnValueChanged(func(context *guigui.Context, value bool) {
		if r.onCheck != nil && r.path != "" {
			r.onCheck([]string{r.path}, value)
		}
	})
	adder.AddWidget(&r.box)
	adder.AddWidget(&r.icon)
	adder.AddWidget(&r.name)
	if r.dir.Value() != "" {
		adder.AddWidget(&r.dir)
	}
	if r.mark.Value() != "" {
		adder.AddWidget(&r.mark)
	}
	return nil
}

func (r *gitFileRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.nameStyle = basicwidget.TextStyle{}
	useGitMono(&r.nameStyle)
	r.dirStyle = basicwidget.TextStyle{}
	useGitMono(&r.dirStyle)
	if v, ok := context.Env(r, basicwidget.EnvKeyListItemColorType); ok {
		if ct, ok := v.(basicwidget.ListItemColorType); ok {
			if c := ct.TextColor(context); c != nil {
				r.nameStyle.SetColor(c)
				r.dirStyle.SetColor(c)
			}
		}
	}
	r.name.SetBaseStyle(&r.nameStyle)
	r.dir.SetBaseStyle(&r.dirStyle)

	r.markStyle = basicwidget.TextStyle{}
	useGitMono(&r.markStyle)
	r.markStyle.SetColor(gitChangeMarkColor(r.mark.Value(), context.ColorMode() == ebiten.ColorModeDark))
	r.mark.SetBaseStyle(&r.markStyle)

	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	box := r.box.Measure(context, guigui.Constraints{}).X
	gap := u / 4
	if gap < 2 {
		gap = 2
	}
	// Keep the selection highlight's rounded left edge visible beside the checkbox.
	inset := u / 2
	y := b.Min.Y + (b.Dy()-box)/2
	x := b.Min.X + inset
	layouter.LayoutWidget(&r.box, image.Rect(x, y, x+box, y+box))
	x += box + gap

	icon := u * 2 / 3
	if icon < 8 {
		icon = 8
	}
	iy := b.Min.Y + (b.Dy()-icon)/2
	layouter.LayoutWidget(&r.icon, image.Rect(x, iy, x+icon, iy+icon))
	x += icon + gap

	right := b.Max.X - gap
	if r.mark.Value() != "" {
		markW := r.mark.Measure(context, guigui.Constraints{}).X
		if markW < u/2 {
			markW = u / 2
		}
		if right-markW < x {
			markW = right - x
		}
		if markW < 0 {
			markW = 0
		}
		layouter.LayoutWidget(&r.mark, image.Rect(right-markW, b.Min.Y, right, b.Max.Y))
		right -= markW + gap
	}
	if right < x {
		right = x
	}
	nameW := r.name.Measure(context, guigui.Constraints{}).X
	dirW := 0
	if r.dir.Value() != "" {
		dirW = r.dir.Measure(context, guigui.Constraints{}).X
	}
	nameUsed, dirUsed := gitChangeLabelWidths(right-x, nameW, dirW, gap)
	layouter.LayoutWidget(&r.name, image.Rect(x, b.Min.Y, x+nameUsed, b.Max.Y))
	if r.dir.Value() != "" {
		dx := x + nameUsed
		if dirUsed > 0 {
			dx += gap
		}
		layouter.LayoutWidget(&r.dir, image.Rect(dx, b.Min.Y, dx+dirUsed, b.Max.Y))
	}
}

func (r *gitFileRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	return image.Pt(w, u)
}

// gitWorkDiff is the single working-copy diff column.
type gitWorkDiff struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	mode  gitDiffModeBar
	view  gitDiffView
	note  basicwidget.Text

	body  string
	empty string
	shown string
	split bool
	show  bool
}

func (d *gitWorkDiff) Set(body, empty, shown string) {
	d.body = body
	d.empty = empty
	if shown != d.shown {
		d.shown = shown
		d.panel.ForceSetScrollOffset(0, 0)
		d.view.ResetScroll()
	}
}

func (d *gitWorkDiff) SetSplit(lang i18n.Lang, split bool, onSplit func(bool)) {
	d.mode.Set(lang, split, onSplit)
	if split == d.split {
		return
	}
	d.split = split
	d.view.ResetScroll()
}

func (d *gitWorkDiff) Text() string {
	return d.body
}

func (d *gitWorkDiff) Patch(unstage bool) (string, bool, bool) {
	if d.body == "" {
		return "", false, false
	}
	return d.view.Patch(unstage)
}

func (d *gitWorkDiff) SetLineMenu(f func(context *guigui.Context, start, end int) bool) {
	d.view.SetLineMenu(f)
}

func (d *gitWorkDiff) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	d.show = d.body != ""
	if !d.show {
		adder.AddWidget(&d.panel)
		configureDiffPanel(&d.panel)
		d.note.SetValue(d.empty)
		d.note.SetMultiline(true)
		d.note.SetWrapMode(basicwidget.WrapModeNormal)
		d.note.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		d.panel.SetContent(&d.note)
		return nil
	}
	adder.AddWidget(&d.mode)
	adder.AddWidget(&d.view)
	d.view.Set(d.body, context.ColorMode() == ebiten.ColorModeDark, d.split)
	return nil
}

func (d *gitWorkDiff) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	if !d.show {
		layouter.LayoutWidget(&d.panel, bounds)
		return
	}
	u := basicwidget.UnitSize(context)
	mode := d.mode.Measure(context, guigui.Constraints{})
	headerH := mode.Y
	if headerH <= 0 {
		headerH = u
	}
	modeW := mode.X
	if modeW > bounds.Dx() {
		modeW = bounds.Dx()
	}
	gap := u / 8
	layouter.LayoutWidget(&d.mode, image.Rect(bounds.Max.X-modeW, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+headerH))
	body := image.Rect(bounds.Min.X, bounds.Min.Y+headerH+gap, bounds.Max.X, bounds.Max.Y)
	if body.Min.Y > body.Max.Y {
		body.Min.Y = body.Max.Y
	}
	layouter.LayoutWidget(&d.view, body)
}

func (d *gitWorkDiff) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 16 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	return image.Pt(w, 8*u)
}

func (t *GitTool) setWorkDiff(lang i18n.Lang, model *GitModel) {
	if !t.commitView {
		return
	}
	t.workPanes.unstaged.SetTitle(i18n.T(lang, i18n.GitUnstaged))
	t.workPanes.staged.SetTitle(i18n.T(lang, i18n.GitStaged))
	path, staged := t.activeWorkTarget()
	t.workPanes.sideStaged = staged
	body, empty := t.workDiffText(lang, model, path, staged)
	shown := path + "\x00unstaged"
	if staged {
		shown = path + "\x00staged"
	}
	t.workPanes.diff.Set(body, empty, shown)
	t.workPanes.diff.SetSplit(lang, t.diffSplit, t.setDiffSplit)
	t.workPanes.diff.SetLineMenu(func(context *guigui.Context, start, end int) bool {
		return t.openLineMenu(context, start, end)
	})
}

// gitDiffText is the diff body. A right-click on the selected lines opens the stage menu.
type gitDiffText struct {
	basicwidget.Text

	onMenu func(context *guigui.Context, start, end int) bool
}

func (t *gitDiffText) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	start, end := t.Selection()
	bounds := widgetBounds.Bounds()
	hit := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && t.pointerInSelection(context, bounds, start, end)
	result := t.Text.HandlePointingInput(context, widgetBounds)
	if result.IsHandled() {
		return result
	}
	if !hit || t.onMenu == nil {
		return result
	}
	t.SetSelection(start, end)
	if t.onMenu(context, start, end) {
		return guigui.HandleInputByWidget(t)
	}
	return result
}

func (t *gitDiffText) pointerInSelection(context *guigui.Context, bounds image.Rectangle, start, end int) bool {
	if end <= start {
		return false
	}
	rects := t.AppendBoundsOfTextRange(nil, context, bounds, start, end)
	c := image.Pt(ebiten.CursorPosition())
	for _, r := range rects {
		r.Min.X = bounds.Min.X
		r.Max.X = bounds.Max.X
		if c.In(r) {
			return true
		}
	}
	return false
}

func (t *GitTool) activeWorkTarget() (string, bool) {
	if t.diffStaged {
		if path := t.workPanes.staged.selectedPath(); path != "" {
			return path, true
		}
	}
	if path := t.workPanes.unstaged.selectedPath(); path != "" {
		return path, false
	}
	return t.workPanes.staged.selectedPath(), true
}

func (t *GitTool) workDiffText(lang i18n.Lang, model *GitModel, path string, staged bool) (string, string) {
	if path == "" {
		return "", i18n.T(lang, i18n.GitNoChanges)
	}
	diff, ready, err := model.WorkDiff(path, staged)
	if !ready {
		return "", ""
	}
	if err != nil {
		return "", err.Error()
	}
	if strings.TrimSpace(diff) == "" {
		return "", i18n.T(lang, i18n.GitNoChanges)
	}
	return diff, ""
}

// linePatch reports a patch for the selection in the diff column.
// selected is true when the user has a non-empty selection.
func (t *GitTool) linePatch(unstage bool) (string, bool, bool) {
	return t.workPanes.diff.Patch(unstage)
}
