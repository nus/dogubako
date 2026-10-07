package app

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

type gitNavIcon int

const (
	gitNavIconNone gitNavIcon = iota
	gitNavIconFolder
	gitNavIconBranch
	gitNavIconTag
)

// gitNavIconKind is the glyph for a sidebar row. Folders use a folder,
// local and remote branch names use a branch, and tags use a tag.
func gitNavIconKind(folder bool, value string) gitNavIcon {
	if folder {
		return gitNavIconFolder
	}
	switch {
	case strings.HasPrefix(value, "tag:"):
		return gitNavIconTag
	case strings.HasPrefix(value, "local:"), strings.HasPrefix(value, "remote:"):
		return gitNavIconBranch
	default:
		return gitNavIconNone
	}
}

// gitNavLead is the arrow and icon placed before a row's label.
// Positions are relative to the row's left edge. arrowX is negative when
// the row has no expander. A leaf lines its icon up with the folder icon
// at the same indent.
type gitNavLead struct {
	arrowX   int
	arrowW   int
	iconX    int
	iconSize int
	textX    int
}

func gitNavLeadLayout(indent int, folder bool, u int) gitNavLead {
	step := u / 2
	if step < 1 {
		step = 1
	}
	icon := u * 2 / 3
	if icon < 8 {
		icon = 8
	}
	gap := u / 6
	if gap < 2 {
		gap = 2
	}
	x := indent * step
	if indent == 0 && !folder {
		x = u / 4
		if x < 2 {
			x = 2
		}
	}
	lead := gitNavLead{arrowX: -1, iconSize: icon}
	if folder {
		lead.arrowX = x
		lead.arrowW = step
		x += step
	} else if indent > 0 {
		x += step
	}
	lead.iconX = x
	lead.textX = x + icon + gap
	return lead
}

// gitNavRow is one branch, tag, or remote line in the scrolling sidebar.
type gitNavRow struct {
	guigui.DefaultWidget

	label basicwidget.Text
	style basicwidget.TextStyle

	value     string
	indent    int
	folder    bool
	collapsed bool
	selected  bool
	bounds    image.Rectangle

	onPress  func()
	onExpand func()
}

func (r *gitNavRow) Set(item basicwidget.ListItem[string], selected bool, onPress, onExpand func()) {
	r.value = item.Value
	r.indent = item.IndentLevel
	r.folder = item.Unselectable
	r.collapsed = item.Collapsed
	r.selected = selected
	r.onPress = onPress
	r.onExpand = onExpand
	r.style = item.TextStyle.Style
	r.style.SetBold(r.folder)
	r.label.SetValue(item.Text)
	r.label.SetBaseStyle(&r.style)
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
}

func (r *gitNavRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.label)
	return nil
}

func (r *gitNavRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.bounds = widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	b := r.bounds
	b.Min.X += r.textStart(u)
	layouter.LayoutWidget(&r.label, b)
}

func (r *gitNavRow) textStart(u int) int {
	return gitNavLeadLayout(r.indent, r.folder, u).textX
}

func (r *gitNavRow) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	mode := context.ColorMode()
	cursor := image.Pt(ebiten.CursorPosition())
	if r.selected {
		fillRect(dst, b, gitNavSelectedColor(mode))
	} else if cursor.In(b) {
		fillRect(dst, b, gitTabHoverColor(mode))
	}
	u := basicwidget.UnitSize(context)
	lead := gitNavLeadLayout(r.indent, r.folder, u)
	clr := textColor(mode)
	if lead.arrowX >= 0 {
		size := lead.arrowW
		y := b.Min.Y + (b.Dy()-size)/2
		drawNavArrow(dst, b.Min.X+lead.arrowX, y, size, !r.collapsed, clr)
	}
	icon := gitNavIconKind(r.folder, r.value)
	if icon == gitNavIconNone {
		return
	}
	box := image.Rect(b.Min.X+lead.iconX, 0, b.Min.X+lead.iconX+lead.iconSize, 0)
	box.Min.Y = b.Min.Y + (b.Dy()-lead.iconSize)/2
	box.Max.Y = box.Min.Y + lead.iconSize
	switch icon {
	case gitNavIconFolder:
		drawFolderGlyph(dst, box, clr)
	case gitNavIconBranch:
		drawBranchGlyph(dst, box)
	case gitNavIconTag:
		drawTagGlyph(dst, box, clr)
	}
}

func gitNavSelectedColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{R: 0x3a, G: 0x7b, B: 0xd5, A: 0x66}
	}
	return color.NRGBA{R: 0x2f, G: 0x81, B: 0xf7, A: 0x33}
}

func textColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	}
	return color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}
}

type gitWorkspaceIcon int

const (
	gitWorkspaceIconChanges gitWorkspaceIcon = iota
	gitWorkspaceIconHistory
)

// gitWorkspaceRow is one workspace entry: an icon and a label.
// The list draws the selection; this row only draws its contents.
type gitWorkspaceRow struct {
	guigui.DefaultWidget

	label basicwidget.Text
	style basicwidget.TextStyle
	icon  gitWorkspaceIcon
}

func (r *gitWorkspaceRow) Set(text string, icon gitWorkspaceIcon, selected bool) {
	r.icon = icon
	r.style.SetBold(selected)
	r.label.SetValue(text)
	r.label.SetBaseStyle(&r.style)
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
}

func (r *gitWorkspaceRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.label)
	return nil
}

func (r *gitWorkspaceRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	_, _, textX := gitWorkspaceLead(u)
	b := widgetBounds.Bounds()
	b.Min.X += textX
	layouter.LayoutWidget(&r.label, b)
}

func (r *gitWorkspaceRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	s := image.Pt(6*u, u)
	if w, ok := constraints.FixedWidth(); ok && w > 0 {
		s.X = w
	}
	if h, ok := constraints.FixedHeight(); ok && h > 0 {
		s.Y = h
	}
	return s
}

func (r *gitWorkspaceRow) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	u := basicwidget.UnitSize(context)
	iconX, iconSize, _ := gitWorkspaceLead(u)
	b := widgetBounds.Bounds()
	box := image.Rect(b.Min.X+iconX, 0, b.Min.X+iconX+iconSize, 0)
	box.Min.Y = b.Min.Y + (b.Dy()-iconSize)/2
	box.Max.Y = box.Min.Y + iconSize
	switch r.icon {
	case gitWorkspaceIconChanges:
		drawChangesGlyph(dst, box)
	case gitWorkspaceIconHistory:
		drawHistoryGlyph(dst, box)
	}
}

// gitWorkspaceLead places the workspace icon at the row's left. The list
// already insets the row, so this does not add the flat-tag padding.
func gitWorkspaceLead(u int) (iconX, iconSize, textX int) {
	iconSize = u * 2 / 3
	if iconSize < 8 {
		iconSize = 8
	}
	gap := u / 6
	if gap < 2 {
		gap = 2
	}
	return 0, iconSize, iconSize + gap
}

func drawChangesGlyph(dst *ebiten.Image, bounds image.Rectangle) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	col := color.NRGBA{R: 0x3d, G: 0x7a, B: 0xd6, A: 0xff}
	w := float32(r.Dx())
	h := float32(r.Dy())
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	sw := float32(1.6)
	vector.StrokeLine(dst, x+w*0.22, y+h*0.28, x+w, y+h*0.28, sw, col, true)
	vector.StrokeLine(dst, x+w*0.22, y+h*0.52, x+w*0.68, y+h*0.52, sw, col, true)
	vector.StrokeLine(dst, x+w*0.22, y+h*0.76, x+w*0.88, y+h*0.76, sw, col, true)
	rad := w * 0.09
	if rad < 1.2 {
		rad = 1.2
	}
	vector.FillCircle(dst, x+w*0.08, y+h*0.52, rad, col, true)
}

func drawHistoryGlyph(dst *ebiten.Image, bounds image.Rectangle) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	col := color.NRGBA{R: 0x5c, G: 0x6b, B: 0xc0, A: 0xff}
	w := float32(r.Dx())
	h := float32(r.Dy())
	cx := float32(r.Min.X) + w*0.5
	cy := float32(r.Min.Y) + h*0.5
	rad := w * 0.42
	if rad < 2 {
		rad = 2
	}
	vector.StrokeCircle(dst, cx, cy, rad, 1.25, col, true)
	vector.StrokeLine(dst, cx, cy, cx, cy-rad*0.55, 1.25, col, true)
	vector.StrokeLine(dst, cx, cy, cx+rad*0.42, cy+rad*0.08, 1.25, col, true)
}

func drawBranchGlyph(dst *ebiten.Image, bounds image.Rectangle) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	col := color.NRGBA{R: 0x2f, G: 0x9e, B: 0x6b, A: 0xff}
	w := float32(r.Dx())
	h := float32(r.Dy())
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	rad := w * 0.16
	if rad < 1.25 {
		rad = 1.25
	}
	lx := x + w*0.30
	rx := x + w*0.74
	ty := y + h*0.26
	by := y + h*0.76
	my := y + h*0.50
	var p vector.Path
	p.MoveTo(lx, by)
	p.LineTo(lx, ty)
	p.MoveTo(lx, my)
	p.QuadTo(lx, ty, rx, ty)
	strokeGitPath(dst, &p, 1.25, col)
	vector.FillCircle(dst, lx, by, rad, col, true)
	vector.FillCircle(dst, lx, ty, rad, col, true)
	vector.FillCircle(dst, rx, ty, rad, col, true)
}

func drawTagGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	w := float32(r.Dx())
	h := float32(r.Dy())
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	tip := x
	body := x + w*0.36
	right := x + w
	top := y + h*0.16
	bot := y + h*0.84
	mid := y + h*0.5
	var p vector.Path
	p.MoveTo(tip, mid)
	p.LineTo(body, top)
	p.LineTo(right, top)
	p.LineTo(right, bot)
	p.LineTo(body, bot)
	p.Close()
	fill := color.NRGBA{R: 0xe0, G: 0x8a, B: 0x2e, A: 0xff}
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(fill)
	vector.FillPath(dst, &p, nil, op)
	strokeGitPath(dst, &p, 1, fg)
	hole := w * 0.09
	if hole < 1.1 {
		hole = 1.1
	}
	vector.FillCircle(dst, body+w*0.16, mid, hole, fg, true)
}

func drawNavArrow(dst *ebiten.Image, x, y, size int, expanded bool, clr color.Color) {
	if size < 2 {
		return
	}
	var p vector.Path
	cx := float32(x) + float32(size)/2
	cy := float32(y) + float32(size)/2
	s := float32(size) * 0.32
	if expanded {
		p.MoveTo(cx-s, cy-s*0.55)
		p.LineTo(cx+s, cy-s*0.55)
		p.LineTo(cx, cy+s*0.75)
	} else {
		p.MoveTo(cx-s*0.55, cy-s)
		p.LineTo(cx+s*0.75, cy)
		p.LineTo(cx-s*0.55, cy+s)
	}
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, nil, op)
}

// gitSectionHead is the accordion title for branches, tags, or remotes.
type gitSectionHead struct {
	guigui.DefaultWidget

	label basicwidget.Text
	mark  basicwidget.Text
	style basicwidget.TextStyle

	collapsed bool
	onToggle  func()
}

func (h *gitSectionHead) Set(text string, collapsed bool, onToggle func()) {
	h.collapsed = collapsed
	h.onToggle = onToggle
	h.style.SetBold(true)
	h.label.SetValue(text)
	h.label.SetBaseStyle(&h.style)
	h.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	if collapsed {
		h.mark.SetValue(">")
	} else {
		h.mark.SetValue("⌄")
	}
	h.mark.SetBaseStyle(&h.style)
	h.mark.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	h.mark.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
}

func (h *gitSectionHead) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&h.label)
	adder.AddWidget(&h.mark)
	return nil
}

func (h *gitSectionHead) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	gap := u / 6
	if gap < 2 {
		gap = 2
	}
	mw := h.mark.Measure(context, guigui.Constraints{}).X
	if mw < 1 {
		mw = u / 2
	}
	mark := b
	mark.Min.X = b.Max.X - mw - gap
	if mark.Min.X < b.Min.X {
		mark.Min.X = b.Min.X
	}
	layouter.LayoutWidget(&h.mark, mark)
	label := b
	label.Max.X = mark.Min.X - gap
	if label.Dx() < 1 {
		label.Max.X = label.Min.X + 1
	}
	layouter.LayoutWidget(&h.label, label)
}

func (h *gitSectionHead) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if image.Pt(ebiten.CursorPosition()).In(b) {
		fillRect(dst, b, gitTabHoverColor(context.ColorMode()))
	}
}

func (h *gitSectionHead) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if !image.Pt(ebiten.CursorPosition()).In(widgetBounds.Bounds()) {
		return guigui.HandleInputResult{}
	}
	if h.onToggle != nil {
		h.onToggle()
	}
	return guigui.HandleInputByWidget(h)
}

func (r *gitNavRow) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if !image.Pt(ebiten.CursorPosition()).In(widgetBounds.Bounds()) {
		return guigui.HandleInputResult{}
	}
	if !context.IsEnabled(r) {
		return guigui.AbortHandlingInputByWidget(r)
	}
	if r.folder {
		if r.onExpand != nil {
			r.onExpand()
		}
		return guigui.HandleInputByWidget(r)
	}
	if r.onPress != nil {
		r.onPress()
	}
	return guigui.HandleInputByWidget(r)
}
