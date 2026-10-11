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
)

// gitWorkPanes is the working copy: unstaged and staged file lists beside one diff.
type gitWorkPanes struct {
	guigui.DefaultWidget

	unstaged   gitFileSection
	staged     gitFileSection
	filesPanel basicwidget.Panel
	files      gitFileColumn
	diff       gitWorkDiff
	split      gitWorkSplit

	// sideStaged is true when the diff column shows the staged list's selection.
	sideStaged bool

	// filesFrac is the file-list share of the width beside the splitter.
	// Zero uses the default one-third split.
	filesFrac float64
	dragging  bool
	bounds    image.Rectangle
}

func (p *gitWorkPanes) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if p.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.dragging = false
		p.setDragPassthrough(context, false)
	}
	p.split.panes = p
	p.files.unstaged = &p.unstaged
	p.files.staged = &p.staged
	adder.AddWidget(&p.filesPanel)
	p.filesPanel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.filesPanel.SetAutoBorder(false)
	p.filesPanel.SetBorders(basicwidget.PanelBorders{})
	p.filesPanel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	p.filesPanel.SetContent(&p.files)
	adder.AddWidget(&p.diff)
	adder.AddWidget(&p.split)
	return nil
}

func (p *gitWorkPanes) setDragPassthrough(context *guigui.Context, pass bool) {
	context.SetPassthrough(&p.filesPanel, pass)
	context.SetPassthrough(&p.diff, pass)
}

func (p *gitWorkPanes) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	p.bounds = b
	splitW := gitWorkSplitWidth(u)
	filesW := gitWorkFilesWidth(b.Dx(), splitW, p.filesFrac, 6*u, 8*u)

	filesBounds := image.Rect(b.Min.X, b.Min.Y, b.Min.X+filesW, b.Max.Y)
	splitBounds := image.Rect(filesBounds.Max.X, b.Min.Y, filesBounds.Max.X+splitW, b.Max.Y)
	diffBounds := image.Rect(splitBounds.Max.X, b.Min.Y, b.Max.X, b.Max.Y)
	layouter.LayoutWidget(&p.filesPanel, filesBounds)
	layouter.LayoutWidget(&p.split, splitBounds)
	layouter.LayoutWidget(&p.diff, diffBounds)
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
		if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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

func (p *gitWorkPanes) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !p.dragging {
		return guigui.HandleInputResult{}
	}
	return p.dragSplit(context)
}

func (p *gitWorkPanes) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if p.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		p.dragging = false
		p.setDragPassthrough(context, false)
		guigui.RequestRebuild()
	}
	return nil
}

func (p *gitWorkPanes) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if p.dragging {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

// gitWorkSplit is the draggable vertical boundary between the file lists and the diff.
type gitWorkSplit struct {
	guigui.DefaultWidget

	panes *gitWorkPanes
}

func (s *gitWorkSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.panes == nil {
		return guigui.HandleInputResult{}
	}
	if s.panes.dragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.panes.dragSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitWorkSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.panes != nil && s.panes.dragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
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
		row.Set(e.String(), e.Path, checked, enabled, onCheck)
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

// gitFileRow is one working-copy file, with a checkbox on the left.
// Checking stages the file. Unchecking unstages it.
type gitFileRow struct {
	guigui.DefaultWidget

	box   basicwidget.Checkbox
	label basicwidget.Text
	style basicwidget.TextStyle

	path    string
	onCheck func(paths []string, checked bool)
}

func (r *gitFileRow) Set(text, path string, checked, sync bool, onCheck func([]string, bool)) {
	r.path = path
	r.onCheck = onCheck
	r.label.SetValue(text)
	if sync {
		r.box.SetValue(checked)
	}
}

func (r *gitFileRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.label.SetWrapMode(basicwidget.WrapModeNone)
	r.box.OnValueChanged(func(context *guigui.Context, value bool) {
		if r.onCheck != nil && r.path != "" {
			r.onCheck([]string{r.path}, value)
		}
	})
	adder.AddWidget(&r.box)
	adder.AddWidget(&r.label)
	return nil
}

func (r *gitFileRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.style = basicwidget.TextStyle{}
	useGitMono(&r.style)
	if v, ok := context.Env(r, basicwidget.EnvKeyListItemColorType); ok {
		if ct, ok := v.(basicwidget.ListItemColorType); ok {
			if c := ct.TextColor(context); c != nil {
				r.style.SetColor(c)
			}
		}
	}
	r.label.SetBaseStyle(&r.style)

	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	box := r.box.Measure(context, guigui.Constraints{}).X
	gap := u / 4
	// Keep the selection highlight's rounded left edge visible beside the checkbox.
	inset := u / 2
	y := b.Min.Y + (b.Dy()-box)/2
	x := b.Min.X + inset
	layouter.LayoutWidget(&r.box, image.Rect(x, y, x+box, y+box))
	layouter.LayoutWidget(&r.label, image.Rect(x+box+gap, b.Min.Y, b.Max.X, b.Max.Y))
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
