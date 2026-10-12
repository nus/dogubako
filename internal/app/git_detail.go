package app

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitCommitExpand is the detail panel that opens directly under a commit row.
// The left side is the commit record. The right side is the changed-file tree.
type gitCommitExpand struct {
	guigui.DefaultWidget

	bridge    gitGraphBridge
	metaPanel basicwidget.Panel
	meta      basicwidget.Text
	filePanel basicwidget.Panel
	files     gitFileTreeBody
	note      basicwidget.Text

	hash        string
	seen        string
	file        string
	lanes       int
	graphUnits  float64
	loading     bool
	showFiles   bool
	metaValue   string
	fileMessage string
	ranges      []basicwidget.TextRange
	hashes      []string
	onParent    func(string)
}

func gitCommitExpandHeight(u int) int {
	return 10 * u
}

func (e *gitCommitExpand) Set(lang i18n.Lang, model *GitModel, row gitcli.GraphRow, lanes int, file string, onParent func(string), onFile func(string)) {
	e.hash = row.Commit.Hash
	e.lanes = lanes
	e.file = file
	e.onParent = onParent
	e.files.SetSelect(file, onFile)
	e.bridge.Set(lanes, row.Outgoing)

	var detail gitcli.CommitDetail
	if model != nil && model.HasRepo() {
		detail = model.Detail()
	}
	ready := detail.Hash != "" && detail.Hash == row.Commit.Hash
	if !ready {
		e.loading = true
		e.showFiles = false
		e.metaValue = i18n.T(lang, i18n.StatusGitLoading)
		e.fileMessage = ""
		e.ranges = nil
		e.hashes = nil
		e.files.SetStats(nil, e.seen != "")
		e.seen = ""
		return
	}

	e.loading = false
	reset := e.seen != detail.Hash
	if reset {
		e.metaPanel.ForceSetScrollOffset(0, 0)
		e.filePanel.ForceSetScrollOffset(0, 0)
	}
	e.seen = detail.Hash
	e.files.SetStats(diffPathStats(detail.Diff), reset)
	e.showFiles = e.files.Len() > 0
	if e.showFiles {
		e.fileMessage = ""
	} else {
		e.fileMessage = i18n.T(lang, i18n.GitNoChanges)
	}

	text, ranges, hashes := formatCommitMeta(lang, detail)
	if model.Snapshot().Status.Detached && detail.Hash == model.Snapshot().HEAD {
		prefix := i18n.T(lang, i18n.GitDetached) + "\n"
		text = prefix + text
		for i := range ranges {
			ranges[i].StartInBytes += len(prefix)
			ranges[i].EndInBytes += len(prefix)
		}
	}
	e.metaValue = text
	e.ranges = ranges
	e.hashes = hashes
}

func (e *gitCommitExpand) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(e.hash)
	w.WriteString(e.seen)
	w.WriteString(e.file)
	w.WriteBool(e.loading)
	w.WriteBool(e.showFiles)
	w.WriteInt(e.files.Len())
	w.WriteInt(e.files.FoldGen())
}

func (e *gitCommitExpand) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&e.bridge)
	adder.AddWidget(&e.metaPanel)
	adder.AddWidget(&e.filePanel)

	configureExpandPanel(&e.metaPanel)
	configureExpandPanel(&e.filePanel)
	e.metaPanel.SetContent(&e.meta)
	if e.showFiles {
		e.filePanel.SetContent(&e.files)
	} else {
		e.note.SetValue(e.fileMessage)
		e.note.SetMultiline(true)
		e.note.SetWrapMode(basicwidget.WrapModeNormal)
		e.note.SetSelectable(true)
		var noteStyle basicwidget.TextStyle
		useGitMono(&noteStyle)
		e.note.SetBaseStyle(&noteStyle)
		e.filePanel.SetContent(&e.note)
	}

	e.meta.SetValue(e.metaValue)
	e.meta.SetMultiline(true)
	e.meta.SetWrapMode(basicwidget.WrapModeNormal)
	e.meta.SetSelectable(true)
	var metaStyle basicwidget.TextStyle
	useGitMono(&metaStyle)
	e.meta.SetBaseStyle(&metaStyle)
	e.applyParentLinks(context)
	return nil
}

func (e *gitCommitExpand) applyParentLinks(context *guigui.Context) {
	if len(e.ranges) == 0 {
		e.meta.SetOverrideStyles(&basicwidget.TextStyles{}, false)
		e.meta.SetHotspotRanges(nil)
		return
	}
	var styles basicwidget.TextStyles
	link, _ := basicwidgetdraw.BorderAccentColors(context.ColorMode(), basicwidgetdraw.RoundedRectBorderTypeRegular)
	for _, r := range e.ranges {
		styles.SetColorInRange(r.StartInBytes, r.EndInBytes, link)
		styles.SetUnderlineInRange(r.StartInBytes, r.EndInBytes, true)
	}
	e.meta.SetOverrideStyles(&styles, false)
	e.meta.SetHotspotRanges(e.ranges)
	e.meta.OnHotspotUp(func(context *guigui.Context, r basicwidget.TextRange) {
		for i, hr := range e.ranges {
			if hr.StartInBytes != r.StartInBytes || hr.EndInBytes != r.EndInBytes {
				continue
			}
			if e.onParent != nil && i < len(e.hashes) {
				e.onParent(e.hashes[i])
			}
			return
		}
	})
}

func (e *gitCommitExpand) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	n := e.lanes
	if n < 1 {
		n = 1
	}
	graphW := gitGraphColWidth(b.Dx(), u, n, preferGraphUnits(e.graphUnits))
	gap := u / 6
	layouter.LayoutWidget(&e.bridge, image.Rect(b.Min.X, b.Min.Y, b.Min.X+graphW, b.Max.Y))

	rest := b
	rest.Min.X += graphW + gap
	rest.Min.Y += u / 8
	rest.Max.Y -= u / 8
	if rest.Max.Y < rest.Min.Y {
		rest.Max.Y = rest.Min.Y
	}
	rest.Max.X -= u / 4
	if rest.Max.X < rest.Min.X {
		rest.Max.X = rest.Min.X
	}
	mid := rest.Min.X + rest.Dx()/2
	metaBox := image.Rect(rest.Min.X, rest.Min.Y, mid-gap/2, rest.Max.Y)
	fileBox := image.Rect(mid+gap/2, rest.Min.Y, rest.Max.X, rest.Max.Y)
	if metaBox.Max.X < metaBox.Min.X {
		metaBox.Max.X = metaBox.Min.X
	}
	if fileBox.Min.X > fileBox.Max.X {
		fileBox.Min.X = fileBox.Max.X
	}
	layouter.LayoutWidget(&e.metaPanel, metaBox)
	layouter.LayoutWidget(&e.filePanel, fileBox)
}

func (e *gitCommitExpand) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 24 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	h := gitCommitExpandHeight(u)
	if fixed, ok := constraints.FixedHeight(); ok && fixed > 0 {
		h = fixed
	}
	return image.Pt(w, h)
}

func configureExpandPanel(p *basicwidget.Panel) {
	p.SetAutoBorder(false)
	p.SetBorders(basicwidget.PanelBorders{})
	p.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
}

func formatCommitMeta(lang i18n.Lang, d gitcli.CommitDetail) (string, []basicwidget.TextRange, []string) {
	var b strings.Builder
	var ranges []basicwidget.TextRange
	var hashes []string
	writeLine := func(label, value string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(value)
	}
	writeLine(i18n.T(lang, i18n.GitCommit), d.Hash)
	if parentCount(d.Parents) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(i18n.T(lang, i18n.GitParents))
		b.WriteString(": ")
		for _, p := range d.Parents {
			if p == "" {
				continue
			}
			if len(hashes) > 0 {
				b.WriteByte(' ')
			}
			start := b.Len()
			b.WriteString(p)
			ranges = append(ranges, basicwidget.TextRange{StartInBytes: start, EndInBytes: b.Len()})
			hashes = append(hashes, p)
		}
	}
	writeLine(i18n.T(lang, i18n.GitAuthor), formatPerson(d.Author, d.Email))
	if d.Committer != "" || d.CommitterEmail != "" {
		writeLine(i18n.T(lang, i18n.GitCommitter), formatPerson(d.Committer, d.CommitterEmail))
	}
	if !d.When.IsZero() {
		writeLine(i18n.T(lang, i18n.GitDate), formatCommitWhen(d.When))
	}
	if body := strings.TrimSpace(d.Body); body != "" {
		b.WriteString("\n\n")
		b.WriteString(body)
	}
	return b.String(), ranges, hashes
}

func parentCount(parents []string) int {
	n := 0
	for _, p := range parents {
		if p != "" {
			n++
		}
	}
	return n
}

func formatPerson(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case email != "":
		return "<" + email + ">"
	default:
		return name
	}
}

func formatCommitWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatCommitWhenIn(t.Local())
}

func formatCommitWhenIn(t time.Time) string {
	name, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("%s GMT%s%02d%02d (%s)",
		t.Format("Mon Jan 02 2006 15:04:05"),
		sign, offset/3600, (offset%3600)/60, name)
}

// gitTreeLine is one visible row of the changed-file tree.
type gitTreeLine struct {
	Name      string
	Path      string
	Depth     int
	Folder    bool
	Collapsed bool
	Add       int
	Del       int
}

type gitFileNode struct {
	name  string
	path  string
	add   int
	del   int
	kids  []*gitFileNode
	index map[string]int
}

func (n *gitFileNode) child(name, path string) *gitFileNode {
	if n.index == nil {
		n.index = map[string]int{}
	}
	if i, ok := n.index[name]; ok {
		return n.kids[i]
	}
	c := &gitFileNode{name: name, path: path}
	n.index[name] = len(n.kids)
	n.kids = append(n.kids, c)
	return c
}

// fileTreeLines groups file stats into a folder tree. folded paths are collapsed.
func fileTreeLines(stats []gitPathStat, folded map[string]bool) []gitTreeLine {
	order := append([]gitPathStat(nil), stats...)
	sort.Slice(order, func(i, j int) bool { return order[i].Path < order[j].Path })
	root := &gitFileNode{}
	for _, st := range order {
		if st.Path == "" {
			continue
		}
		node := root
		var path string
		parts := strings.Split(st.Path, "/")
		last := -1
		for i, part := range parts {
			if part == "" {
				continue
			}
			last = i
			if path == "" {
				path = part
			} else {
				path += "/" + part
			}
			node = node.child(part, path)
		}
		if last < 0 {
			continue
		}
		node.add = st.Add
		node.del = st.Del
	}
	var out []gitTreeLine
	var walk func(*gitFileNode, int)
	walk = func(n *gitFileNode, depth int) {
		for _, c := range n.kids {
			folder := len(c.kids) > 0
			line := gitTreeLine{Name: c.name, Path: c.path, Depth: depth, Folder: folder, Collapsed: folder && folded[c.path]}
			if !folder {
				line.Add = c.add
				line.Del = c.del
			}
			out = append(out, line)
			if folder && !folded[c.path] {
				walk(c, depth+1)
			}
		}
	}
	walk(root, 0)
	return out
}

func fileTreeLabel(name string, add, del int) (text string, addStart, addEnd, delStart, delEnd int) {
	if add == 0 && del == 0 {
		return name, 0, 0, 0, 0
	}
	prefix := name + "  ("
	addStr := fmt.Sprintf("+%d", add)
	delStr := fmt.Sprintf("-%d", del)
	text = prefix + addStr + " " + delStr + ")"
	addStart = len(prefix)
	addEnd = addStart + len(addStr)
	delStart = addEnd + 1
	delEnd = delStart + len(delStr)
	return text, addStart, addEnd, delStart, delEnd
}

// gitFileTreeBody is the scrolling list of changed files.
type gitFileTreeBody struct {
	guigui.DefaultWidget

	rows     guigui.WidgetSlice[*gitFileTreeRow]
	stats    []gitPathStat
	lines    []gitTreeLine
	folded   map[string]bool
	foldGen  int
	selected string
	onSelect func(string)
}

func (b *gitFileTreeBody) SetSelect(path string, onSelect func(string)) {
	b.selected = path
	b.onSelect = onSelect
}

func (b *gitFileTreeBody) SetStats(stats []gitPathStat, reset bool) {
	if reset {
		b.folded = nil
	}
	b.stats = stats
	b.lines = fileTreeLines(stats, b.folded)
}

func (b *gitFileTreeBody) Len() int { return len(b.lines) }

func (b *gitFileTreeBody) FoldGen() int { return b.foldGen }

func (b *gitFileTreeBody) toggle(path string) {
	if b.folded == nil {
		b.folded = map[string]bool{}
	}
	if b.folded[path] {
		delete(b.folded, path)
	} else {
		b.folded[path] = true
	}
	b.foldGen++
	b.lines = fileTreeLines(b.stats, b.folded)
	guigui.RequestRebuild()
}

func (b *gitFileTreeBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	b.rows.SetLen(len(b.lines))
	for i := range b.lines {
		row := b.rows.At(i)
		line := b.lines[i]
		var toggle func()
		var selectFile func()
		if line.Folder {
			path := line.Path
			toggle = func() { b.toggle(path) }
		} else {
			path := line.Path
			selectFile = func() {
				if b.onSelect != nil {
					b.onSelect(path)
				}
			}
		}
		row.Set(line, !line.Folder && line.Path == b.selected, toggle, selectFile)
		adder.AddWidget(row)
	}
	return nil
}

func (b *gitFileTreeBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	bounds := widgetBounds.Bounds()
	y := bounds.Min.Y
	for i := 0; i < b.rows.Len() && i < len(b.lines); i++ {
		layouter.LayoutWidget(b.rows.At(i), image.Rect(bounds.Min.X, y, bounds.Max.X, y+u))
		y += u
	}
}

func (b *gitFileTreeBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 12 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	h := len(b.lines) * u
	if h < 1 {
		h = 1
	}
	return image.Pt(w, h)
}

type gitFileTreeRow struct {
	guigui.DefaultWidget

	label basicwidget.Text

	line     gitTreeLine
	text     string
	addStart int
	addEnd   int
	delStart int
	delEnd   int
	selected bool
	onToggle func()
	onSelect func()
}

func (r *gitFileTreeRow) Set(line gitTreeLine, selected bool, onToggle, onSelect func()) {
	r.line = line
	r.selected = selected
	r.onToggle = onToggle
	r.onSelect = onSelect
	r.text, r.addStart, r.addEnd, r.delStart, r.delEnd = fileTreeLabel(line.Name, line.Add, line.Del)
}

func (r *gitFileTreeRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.label)
	r.label.SetValue(r.text)
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.label.SetWrapMode(basicwidget.WrapModeNone)
	r.label.SetEllipsisString("…")
	r.label.SetSelectable(false)
	var style basicwidget.TextStyle
	useGitMono(&style)
	style.SetBold(r.line.Folder)
	r.label.SetBaseStyle(&style)

	var styles basicwidget.TextStyles
	if r.addEnd > r.addStart || r.delEnd > r.delStart {
		add, del := diffStatColors(context.ColorMode() == ebiten.ColorModeDark)
		if r.addEnd > r.addStart {
			styles.SetColorInRange(r.addStart, r.addEnd, add)
		}
		if r.delEnd > r.delStart {
			styles.SetColorInRange(r.delStart, r.delEnd, del)
		}
	}
	r.label.SetOverrideStyles(&styles, false)
	return nil
}

func (r *gitFileTreeRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	b.Min.X += gitNavLeadLayout(r.line.Depth, r.line.Folder, u).textX
	layouter.LayoutWidget(&r.label, b)
}

func (r *gitFileTreeRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	h := u
	if fixed, ok := constraints.FixedHeight(); ok && fixed > 0 {
		h = fixed
	}
	return image.Pt(w, h)
}

func (r *gitFileTreeRow) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if r.line.Folder {
		if r.onToggle == nil {
			return guigui.HandleInputResult{}
		}
		r.onToggle()
		return guigui.HandleInputByWidget(r)
	}
	if r.onSelect == nil {
		return guigui.HandleInputResult{}
	}
	r.onSelect()
	return guigui.HandleInputByWidget(r)
}

func (r *gitFileTreeRow) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if (r.line.Folder || r.onSelect != nil) && widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

func (r *gitFileTreeRow) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	mode := context.ColorMode()
	switch {
	case r.selected:
		fillRect(dst, b, gitNavSelectedColor(mode))
	case image.Pt(ebiten.CursorPosition()).In(b):
		fillRect(dst, b, gitTabHoverColor(mode))
	}
	u := basicwidget.UnitSize(context)
	lead := gitNavLeadLayout(r.line.Depth, r.line.Folder, u)
	clr := textColor(mode)
	if lead.arrowX >= 0 {
		y := b.Min.Y + (b.Dy()-lead.arrowW)/2
		drawNavArrow(dst, b.Min.X+lead.arrowX, y, lead.arrowW, !r.line.Collapsed, clr)
	}
	box := image.Rect(b.Min.X+lead.iconX, 0, b.Min.X+lead.iconX+lead.iconSize, 0)
	box.Min.Y = b.Min.Y + (b.Dy()-lead.iconSize)/2
	box.Max.Y = box.Min.Y + lead.iconSize
	if r.line.Folder {
		drawFolderGlyph(dst, box, clr)
		return
	}
	drawFileGlyph(dst, box, clr)
}

func diffStatColors(dark bool) (add, del color.NRGBA) {
	if dark {
		return color.NRGBA{R: 0x3f, G: 0xb9, B: 0x50, A: 0xff},
			color.NRGBA{R: 0xf8, G: 0x51, B: 0x49, A: 0xff}
	}
	return color.NRGBA{R: 0x1a, G: 0x7f, B: 0x37, A: 0xff},
		color.NRGBA{R: 0xcf, G: 0x22, B: 0x2e, A: 0xff}
}

// gitHistStack is the history column body: the commit graph, and the diff
// of the file chosen in the open commit. A horizontal splitter sets their heights.
// Vertical splitters set the graph, title, author, commit-id, and date widths.
type gitHistStack struct {
	guigui.DefaultWidget

	tool        *GitTool
	graph       guigui.Widget
	header      gitCommitHeader
	diff        gitWorkDiff
	split       gitHistSplit
	graphSplit  gitGraphSplit
	authorSplit gitMetaSplit
	hashSplit   gitMetaSplit
	dateSplit   gitMetaSplit
	showDiff    bool
}

func (s *gitHistStack) showGraphSplit() bool {
	return s.tool != nil && s.tool.graphLanes > 0 && s.graph == &s.tool.commitList
}

func (s *gitHistStack) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if s.tool != nil && s.tool.histDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.tool.histDragging = false
		s.tool.setHistDragPassthrough(context, false)
	}
	if s.tool != nil && s.tool.graphDragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.tool.stopGraphDrag(context)
	}
	if s.tool != nil && s.tool.metaDrag != 0 && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.tool.metaDrag = 0
		s.tool.setGraphDragPassthrough(context, false)
	}
	if s.graph != nil {
		adder.AddWidget(s.graph)
	}
	if s.showDiff {
		s.split.tool = s.tool
		adder.AddWidget(&s.diff)
		adder.AddWidget(&s.split)
	}
	if s.showGraphSplit() {
		s.header.tool = s.tool
		context.SetClipChildren(&s.header, true)
		adder.AddWidget(&s.header)
		s.graphSplit.tool = s.tool
		adder.AddWidget(&s.graphSplit)
		s.authorSplit.tool = s.tool
		s.authorSplit.kind = gitMetaDragAuthor
		adder.AddWidget(&s.authorSplit)
		s.hashSplit.tool = s.tool
		s.hashSplit.kind = gitMetaDragHash
		adder.AddWidget(&s.hashSplit)
		s.dateSplit.tool = s.tool
		s.dateSplit.kind = gitMetaDragDate
		adder.AddWidget(&s.dateSplit)
	}
	return nil
}

func (s *gitHistStack) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	if s.tool != nil {
		s.tool.histBounds = b
	}
	if s.graph == nil {
		return
	}
	graphB := b
	if !s.showDiff {
		if s.tool != nil {
			s.tool.histGraph = b
		}
		layouter.LayoutWidget(s.graph, b)
	} else {
		u := basicwidget.UnitSize(context)
		splitH := gitWorkSplitWidth(u)
		minPane := gitHistMinPane(u)
		frac := 0.0
		if s.tool != nil {
			frac = s.tool.histFrac
		}
		graphH := gitHistGraphHeight(b.Dy(), splitH, frac, minPane, minPane)
		graphB = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+graphH)
		if s.tool != nil {
			s.tool.histGraph = graphB
		}
		splitB := image.Rect(b.Min.X, graphB.Max.Y, b.Max.X, graphB.Max.Y+splitH)
		diffB := image.Rect(b.Min.X, splitB.Max.Y, b.Max.X, b.Max.Y)
		if diffB.Min.Y > diffB.Max.Y {
			diffB.Min.Y = diffB.Max.Y
		}
		layouter.LayoutWidget(s.graph, graphB)
		layouter.LayoutWidget(&s.split, splitB)
		layouter.LayoutWidget(&s.diff, diffB)
	}
	s.layoutGraphSplit(context, graphB, layouter)
}

// layoutGraphSplit places the draggable edges of the graph and the text columns.
// List items are inset by the corner radius, and the graph column starts there.
func (s *gitHistStack) layoutGraphSplit(context *guigui.Context, graphB image.Rectangle, layouter *guigui.ChildLayouter) {
	if s.tool == nil || !s.showGraphSplit() {
		return
	}
	u := basicwidget.UnitSize(context)
	inset := basicwidget.RoundedCornerRadius(context)
	rowW := graphB.Dx() - 2*inset
	if rowW < 1 {
		return
	}
	rowLeft := graphB.Min.X + inset
	rowRight := graphB.Max.X - inset
	s.tool.ensureGraphWidth()
	graphW := gitGraphColWidth(rowW, u, s.tool.graphLanes, preferGraphUnits(s.tool.graphUnits))
	splitW := gitWorkSplitWidth(u)
	headerB := image.Rect(graphB.Min.X, graphB.Min.Y, graphB.Max.X, graphB.Min.Y+gitCommitHeaderHeight(u))
	if headerB.Max.Y > graphB.Max.Y {
		headerB.Max.Y = graphB.Max.Y
	}
	layouter.LayoutWidget(&s.header, headerB)
	splitB := gitColSplitRect(rowLeft+graphW, splitW, headerB.Min.Y, headerB.Max.Y, graphB.Min.X, graphB.Max.X)
	s.tool.graphSplitBounds = splitB
	layouter.LayoutWidget(&s.graphSplit, splitB)

	authorU, hashU, dateU := preferMetaUnits(s.tool.authorUnits, s.tool.hashUnits, s.tool.dateUnits)
	authorW, hashW, dateW := gitMetaColWidths(rowW, u, graphW, authorU, hashU, dateU)
	subjectR, authorR, hashR := gitMetaSplitEdges(rowRight, gitCommitColGap(u), authorW, hashW, dateW)
	place := func(edge int, dst *image.Rectangle, widget guigui.Widget) {
		r := gitColSplitRect(edge, splitW, headerB.Min.Y, headerB.Max.Y, graphB.Min.X, graphB.Max.X)
		*dst = r
		layouter.LayoutWidget(widget, r)
	}
	place(subjectR, &s.tool.authorSplitBounds, &s.authorSplit)
	place(authorR, &s.tool.hashSplitBounds, &s.hashSplit)
	place(hashR, &s.tool.dateSplitBounds, &s.dateSplit)
}

func (s *gitHistStack) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w, h := 16*u, 8*u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	if fixed, ok := constraints.FixedHeight(); ok && fixed > 0 {
		h = fixed
	}
	return image.Pt(w, h)
}

func (s *gitHistStack) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil {
		return guigui.HandleInputResult{}
	}
	if s.tool.graphDragging {
		return s.tool.dragGraphSplit(context)
	}
	if s.tool.metaDrag != 0 {
		return s.tool.dragMetaSplit(context, s.tool.metaDrag)
	}
	if s.tool.histDragging {
		return s.tool.dragHistSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitHistStack) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool != nil && (s.tool.graphDragging || s.tool.metaDrag != 0) {
		return ebiten.CursorShapeEWResize, true
	}
	if s.tool != nil && s.tool.histDragging {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func gitHistMinPane(u int) int {
	if u < 1 {
		u = 1
	}
	return 4 * u
}

// gitHistGraphHeight is the commit-graph share of a vertical stack.
// frac is the graph's share of the space beside the splitter. Zero uses half.
func gitHistGraphHeight(total, splitH int, frac float64, minGraph, minDiff int) int {
	if frac <= 0 {
		frac = 0.5
	}
	return gitWorkFilesWidth(total, splitH, frac, minGraph, minDiff)
}

func (t *GitTool) setHistFracFromCursor(y, u int) {
	splitH := gitWorkSplitWidth(u)
	avail := t.histBounds.Dy() - splitH
	if avail <= 0 {
		return
	}
	minPane := gitHistMinPane(u)
	graphH := y - t.histBounds.Min.Y - splitH/2
	frac := float64(graphH) / float64(avail)
	minFrac := float64(minPane) / float64(avail)
	maxFrac := float64(avail-minPane) / float64(avail)
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
	t.histFrac = frac
}

func (t *GitTool) dragHistSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !t.histDragging {
		if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.histDragging = true
	}
	t.setHistFracFromCursor(image.Pt(ebiten.CursorPosition()).Y, u)
	t.setHistDragPassthrough(context, true)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.histDragging = false
		t.setHistDragPassthrough(context, false)
	}
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(t)
}

func (t *GitTool) setHistDragPassthrough(context *guigui.Context, pass bool) {
	if t.histStack.graph != nil {
		context.SetPassthrough(t.histStack.graph, pass)
	}
	context.SetPassthrough(&t.histStack.diff, pass)
	for _, w := range []guigui.Widget{
		&t.tabBar,
		&t.sidePane,
		&t.sideSplit,
		&t.fetchBtn,
		&t.pullBtn,
		&t.pushBtn,
		&t.status,
		&t.hint,
		&t.expand,
	} {
		context.SetPassthrough(w, pass)
	}
}

// gitHistSplit is the draggable horizontal boundary between the graph and the diff.
type gitHistSplit struct {
	guigui.DefaultWidget

	tool *GitTool
}

func (s *gitHistSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil {
		return guigui.HandleInputResult{}
	}
	if s.tool.histDragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.tool.dragHistSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitHistSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool != nil && s.tool.histDragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func (s *gitHistSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	y := b.Min.Y + b.Dy()/2
	fillRect(dst, image.Rect(b.Min.X, y, b.Max.X, y+1), gitHistSplitColor(context.ColorMode()))
}

func gitHistSplitColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x40}
	}
	return color.NRGBA{A: 0x40}
}
