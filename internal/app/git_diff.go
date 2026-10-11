package app

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitFileDiff is one file section of a unified diff.
type gitFileDiff struct {
	Name   string
	Status string
	Text   string
}

// gitChanges is the changes tab: a file list beside the colored diff.
type gitChanges struct {
	guigui.DefaultWidget

	list basicwidget.List[int]
	mode gitDiffModeBar
	diff gitDiffView
	note basicwidget.Text

	files    []gitFileDiff
	items    []basicwidget.ListItem[int]
	selected int
	seen     string
	message  string
	split    bool
}

func (c *gitChanges) Set(lang i18n.Lang, model *GitModel, split bool, onSplit func(bool)) {
	c.mode.Set(lang, split, onSplit)
	if split != c.split {
		c.split = split
		c.diff.ResetScroll()
	}
	c.message = ""
	c.files = nil
	if !model.HasRepo() {
		c.message = i18n.T(lang, i18n.GitEmpty)
		c.note.SetValue(c.message)
		return
	}
	detail := model.Detail()
	if detail.Hash == "" {
		c.message = i18n.T(lang, i18n.GitNoCommits)
		c.note.SetValue(c.message)
		return
	}
	if detail.Hash != c.seen {
		c.seen = detail.Hash
		c.selected = 0
		c.diff.ResetScroll()
	}
	c.files = splitUnifiedDiff(detail.Diff)
	if len(c.files) == 0 {
		c.message = i18n.T(lang, i18n.GitNoChanges)
		c.note.SetValue(c.message)
		return
	}
	if c.selected < 0 || c.selected >= len(c.files) {
		c.selected = 0
	}
	c.items = c.items[:0]
	for i, f := range c.files {
		c.items = append(c.items, basicwidget.ListItem[int]{
			Text:  f.Status + "  " + f.Name,
			Value: i,
		})
	}
	c.list.SetItems(c.items)
	c.list.SelectItemByIndex(c.selected)
}

func (c *gitChanges) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if c.message != "" {
		c.note.SetMultiline(true)
		c.note.SetWrapMode(basicwidget.WrapModeNormal)
		c.note.SetSelectable(true)
		adder.AddWidget(&c.note)
		return nil
	}
	adder.AddWidget(&c.list)
	adder.AddWidget(&c.mode)
	adder.AddWidget(&c.diff)
	c.list.SetStyle(basicwidget.ListStyleNormal)
	c.list.SetHighlightVisibleWhenUnfocused(true)
	c.list.SetSelectedItemBold(false)
	c.list.OnItemSelected(func(context *guigui.Context, index int) {
		if index < 0 || index >= len(c.files) || index == c.selected {
			return
		}
		c.selected = index
		c.diff.ResetScroll()
		guigui.RequestRebuild()
	})
	text := ""
	if c.selected >= 0 && c.selected < len(c.files) {
		text = c.files[c.selected].Text
	}
	c.diff.Set(text, context.ColorMode() == ebiten.ColorModeDark, c.split)
	return nil
}

// gitDiffView shows a diff in the compact layout or the split layout.
// Compact keeps the unified patch with old and new line numbers beside it.
// Split puts the old file on the left and the new file on the right.
// Each split column scrolls horizontally on its own. Vertical scrolling stays in step.
type gitDiffView struct {
	guigui.DefaultWidget

	panel   basicwidget.Panel
	compact gitDiffSide

	leftPanel  basicwidget.Panel
	rightPanel basicwidget.Panel
	left       gitDiffSide
	right      gitDiffSide

	text  string
	split bool
	dark  bool

	gutter                string
	leftNum, leftBody     string
	rightNum, rightBody   string
	leftKinds, rightKinds []byte
	leftSpans, rightSpans []diffSpan
	onMenu                func(context *guigui.Context, start, end int) bool
	useMenu               bool
	menuRanges            [][2]int
	leftX, leftY          float64
	rightX, rightY        float64
}

func (v *gitDiffView) Set(text string, dark, split bool) {
	v.dark = dark
	if text == v.text && split == v.split {
		return
	}
	v.text = text
	v.split = split
	if split {
		doc := splitDiff(text)
		v.leftNum, v.leftBody = doc.leftNum, doc.leftBody
		v.rightNum, v.rightBody = doc.rightNum, doc.rightBody
		v.leftKinds, v.rightKinds = doc.leftKinds, doc.rightKinds
		v.leftSpans, v.rightSpans = doc.leftSpans, doc.rightSpans
		v.gutter = ""
	} else {
		v.gutter = diffGutter(text)
	}
	v.ResetScroll()
}

func (v *gitDiffView) ResetScroll() {
	v.leftX, v.leftY = 0, 0
	v.rightX, v.rightY = 0, 0
	v.panel.ForceSetScrollOffset(0, 0)
	v.leftPanel.ForceSetScrollOffset(0, 0)
	v.rightPanel.ForceSetScrollOffset(0, 0)
}

func (v *gitDiffView) SetLineMenu(f func(context *guigui.Context, start, end int) bool) {
	v.onMenu = f
}

// Patch reports the patch for the current line selection.
// selected is true when the user has a non-empty selection.
// During a side's context menu, only that side's lines are included.
func (v *gitDiffView) Patch(unstage bool) (string, bool, bool) {
	if v.text == "" {
		return "", false, false
	}
	var ranges [][2]int
	selected := false
	if v.useMenu {
		ranges = v.menuRanges
		selected = len(ranges) > 0
	} else if v.split {
		if start, end := v.left.body.Selection(); end > start {
			selected = true
			ranges = append(ranges, overlappingSources(v.leftSpans, start, end)...)
		}
		if start, end := v.right.body.Selection(); end > start {
			selected = true
			ranges = append(ranges, overlappingSources(v.rightSpans, start, end)...)
		}
	} else if start, end := v.compact.body.Selection(); end > start {
		selected = true
		ranges = [][2]int{{start, end}}
	}
	if !selected {
		return "", false, false
	}
	if len(ranges) == 0 {
		return "", false, true
	}
	patch, ok := gitcli.PatchForRanges(v.text, ranges, unstage)
	return patch, ok, true
}

func (v *gitDiffView) bindMenu(text *gitDiffText, left bool) {
	text.onMenu = func(ctx *guigui.Context, start, end int) bool {
		if v.onMenu == nil {
			return false
		}
		if !v.split {
			return v.onMenu(ctx, start, end)
		}
		spans := v.rightSpans
		if left {
			spans = v.leftSpans
		}
		v.menuRanges = overlappingSources(spans, start, end)
		v.useMenu = true
		defer func() {
			v.useMenu = false
			v.menuRanges = nil
		}()
		return v.onMenu(ctx, start, end)
	}
}

func (v *gitDiffView) syncFromLeft(x, y float64) {
	v.leftX = x
	if y == v.leftY {
		return
	}
	v.leftY = y
	v.rightPanel.ForceSetScrollOffset(v.rightX, y)
}

func (v *gitDiffView) syncFromRight(x, y float64) {
	v.rightX = x
	if y == v.rightY {
		return
	}
	v.rightY = y
	v.leftPanel.ForceSetScrollOffset(v.leftX, y)
}

func (v *gitDiffView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if v.split {
		adder.AddWidget(&v.leftPanel)
		adder.AddWidget(&v.rightPanel)
		configureDiffPanel(&v.leftPanel)
		configureDiffPanel(&v.rightPanel)
		v.left.prepare(v.leftNum, v.leftBody, styleSplit(v.leftNum, v.leftKinds, v.dark, true), styleSplit(v.leftBody, v.leftKinds, v.dark, false))
		v.right.prepare(v.rightNum, v.rightBody, styleSplit(v.rightNum, v.rightKinds, v.dark, true), styleSplit(v.rightBody, v.rightKinds, v.dark, false))
		v.leftPanel.SetContent(&v.left)
		v.rightPanel.SetContent(&v.right)
		v.bindMenu(&v.left.body, true)
		v.bindMenu(&v.right.body, false)
		v.leftPanel.OnScroll(func(_ *guigui.Context, x, y float64) { v.syncFromLeft(x, y) })
		v.rightPanel.OnScroll(func(_ *guigui.Context, x, y float64) { v.syncFromRight(x, y) })
		return nil
	}
	adder.AddWidget(&v.panel)
	configureDiffPanel(&v.panel)
	v.compact.prepare(v.gutter, v.text, styleDiffGutter(v.gutter, v.text, v.dark), styleDiff(v.text, v.dark))
	v.panel.SetContent(&v.compact)
	v.bindMenu(&v.compact.body, false)
	return nil
}

func (v *gitDiffView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	if !v.split {
		layouter.LayoutWidget(&v.panel, bounds)
		return
	}
	gap := basicwidget.UnitSize(context) / 8
	if gap < 1 {
		gap = 1
	}
	mid := bounds.Min.X + bounds.Dx()/2
	leftMax := mid - gap
	rightMin := mid + gap
	if leftMax < bounds.Min.X {
		leftMax = bounds.Min.X
	}
	if rightMin > bounds.Max.X {
		rightMin = bounds.Max.X
	}
	layouter.LayoutWidget(&v.leftPanel, image.Rect(bounds.Min.X, bounds.Min.Y, leftMax, bounds.Max.Y))
	layouter.LayoutWidget(&v.rightPanel, image.Rect(rightMin, bounds.Min.Y, bounds.Max.X, bounds.Max.Y))
}

func (v *gitDiffView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
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

func (c *gitChanges) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	if c.message != "" {
		layouter.LayoutWidget(&c.note, bounds)
		return
	}
	u := basicwidget.UnitSize(context)
	listW := bounds.Dx() / 3
	if maxW := 18 * u; listW > maxW {
		listW = maxW
	}
	if minW := 10 * u; listW < minW {
		listW = minW
	}
	if listW > bounds.Dx()/2 {
		listW = bounds.Dx() / 2
	}
	if listW < 0 {
		listW = 0
	}
	gap := u / 4
	left := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+listW, bounds.Max.Y)
	right := image.Rect(bounds.Min.X+listW+gap, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	if right.Min.X > right.Max.X {
		right.Min.X = right.Max.X
	}
	mode := c.mode.Measure(context, guigui.Constraints{})
	headerH := mode.Y
	if headerH <= 0 {
		headerH = u
	}
	header := image.Rect(right.Min.X, right.Min.Y, right.Max.X, right.Min.Y+headerH)
	body := image.Rect(right.Min.X, header.Max.Y+gap, right.Max.X, right.Max.Y)
	if body.Min.Y > body.Max.Y {
		body.Min.Y = body.Max.Y
	}
	modeW := mode.X
	if modeW > header.Dx() {
		modeW = header.Dx()
	}
	layouter.LayoutWidget(&c.list, left)
	layouter.LayoutWidget(&c.mode, image.Rect(header.Max.X-modeW, header.Min.Y, header.Max.X, header.Max.Y))
	layouter.LayoutWidget(&c.diff, body)
}

func configureDiffPanel(p *basicwidget.Panel) {
	p.SetAutoBorder(false)
	p.SetBorders(basicwidget.PanelBorders{})
	p.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	p.SetContentConstraints(basicwidget.PanelContentConstraintsNone)
}

func setGitDiffFont(t *basicwidget.Text) {
	family := gitDiffFont()
	if family == nil {
		return
	}
	var style basicwidget.TextStyle
	style.SetFontFamily(family)
	t.SetBaseStyle(&style)
}

func splitUnifiedDiff(diff string) []gitFileDiff {
	diff = strings.ReplaceAll(diff, "\r\n", "\n")
	if !strings.Contains(diff, "diff --git ") {
		return nil
	}
	var out []gitFileDiff
	var b strings.Builder
	var cur *gitFileDiff
	flush := func() {
		if cur == nil {
			return
		}
		cur.Text = b.String()
		out = append(out, *cur)
		b.Reset()
		cur = nil
	}
	for _, line := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			cur = &gitFileDiff{
				Name:   gitDiffName(strings.TrimRight(line, "\n")),
				Status: "M",
			}
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(trimmed, "new file mode "):
			cur.Status = "A"
		case strings.HasPrefix(trimmed, "deleted file mode "):
			cur.Status = "D"
		case strings.HasPrefix(trimmed, "rename from "), strings.HasPrefix(trimmed, "rename to "):
			cur.Status = "R"
			if name, ok := strings.CutPrefix(trimmed, "rename to "); ok {
				cur.Name = name
			}
		case strings.HasPrefix(trimmed, "copy from "), strings.HasPrefix(trimmed, "copy to "):
			cur.Status = "C"
			if name, ok := strings.CutPrefix(trimmed, "copy to "); ok {
				cur.Name = name
			}
		case strings.HasPrefix(trimmed, "+++ "):
			if p, ok := diffPath(trimmed[len("+++ "):]); ok && cur.Status != "R" && cur.Status != "C" {
				cur.Name = p
			}
		case strings.HasPrefix(trimmed, "--- "):
			if cur.Name == "" {
				if p, ok := diffPath(trimmed[len("--- "):]); ok {
					cur.Name = p
				}
			}
		}
		b.WriteString(line)
	}
	flush()
	return out
}

func gitDiffName(header string) string {
	rest := strings.TrimPrefix(header, "diff --git ")
	i := strings.LastIndex(rest, " b/")
	if i >= 0 && strings.HasPrefix(rest, "a/") {
		to := rest[i+len(" b/"):]
		if to != "" {
			return to
		}
		return rest[len("a/"):i]
	}
	return strings.TrimSpace(rest)
}

func diffPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "/dev/null" {
		return "", false
	}
	if strings.HasPrefix(s, "a/") || strings.HasPrefix(s, "b/") {
		return s[len("a/"):], true
	}
	return s, true
}

func styleDiff(text string, dark bool) basicwidget.TextStyles {
	var styles basicwidget.TextStyles
	pal := diffPalette(dark)
	for start := 0; start < len(text); {
		end := diffLineEnd(text, start)
		fg, bg := diffLineColors(text[start:end], pal)
		if fg.A != 0 {
			styles.SetColorInRange(start, end, fg)
		}
		if bg.A != 0 {
			styles.SetBackgroundColorInRange(start, end, bg)
		}
		if end == len(text) {
			break
		}
		start = end
	}
	return styles
}

func styleDiffGutter(gutter, diff string, dark bool) basicwidget.TextStyles {
	var styles basicwidget.TextStyles
	if gutter == "" {
		return styles
	}
	pal := diffPalette(dark)
	styles.SetColorInRange(0, len(gutter), pal.meta)
	styles.SetBackgroundColorInRange(0, len(gutter), pal.numBg)
	gs, ds := 0, 0
	for gs < len(gutter) && ds < len(diff) {
		gEnd := diffLineEnd(gutter, gs)
		dEnd := diffLineEnd(diff, ds)
		if _, bg := diffLineColors(diff[ds:dEnd], pal); bg.A != 0 {
			styles.SetBackgroundColorInRange(gs, gEnd, bg)
		}
		if gEnd == len(gutter) || dEnd == len(diff) {
			break
		}
		gs, ds = gEnd, dEnd
	}
	return styles
}

func diffLineEnd(text string, start int) int {
	end := strings.IndexByte(text[start:], '\n')
	if end < 0 {
		return len(text)
	}
	return start + end + 1
}

func diffLineColors(line string, pal diffColors) (fg, bg color.NRGBA) {
	switch {
	case diffMetaLine(line):
		fg = pal.meta
	case strings.HasPrefix(line, "@@"):
		fg = pal.hunk
	case strings.HasPrefix(line, "+"):
		bg = pal.addBg
	case strings.HasPrefix(line, "-"):
		bg = pal.delBg
	}
	return fg, bg
}

func diffMetaLine(line string) bool {
	return strings.HasPrefix(line, "diff ") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "new file ") ||
		strings.HasPrefix(line, "deleted file ") ||
		strings.HasPrefix(line, "old mode ") ||
		strings.HasPrefix(line, "new mode ") ||
		strings.HasPrefix(line, "similarity ") ||
		strings.HasPrefix(line, "dissimilarity ") ||
		strings.HasPrefix(line, "rename ") ||
		strings.HasPrefix(line, "copy ") ||
		strings.HasPrefix(line, "Binary files ") ||
		strings.HasPrefix(line, "\\") ||
		strings.HasPrefix(line, "---") ||
		strings.HasPrefix(line, "+++")
}

// diffNumMinWidth is the digit width of each line-number column.
// Shorter files still get this much padding so the two columns stay put.
const diffNumMinWidth = 4

// diffGutter returns old and new line numbers for a unified diff, one pair
// per line. The old number is the left column and the new number is the
// right column, both right-aligned. A side the line does not belong to is
// blank, as are headers. Diffs with no hunk body return an empty string.
func diffGutter(text string) string {
	lines := splitDiffTextLines(text)
	if len(lines) == 0 {
		return ""
	}
	nums := make([][2]int, len(lines))
	oldLine, newLine := 0, 0
	inHunk := false
	maxN := 0
	numbered := false
	for i, ln := range lines {
		if strings.HasPrefix(ln.content, "diff --git ") {
			inHunk = false
			continue
		}
		if oldStart, newStart, ok := parseHunkStarts(ln.content); ok {
			oldLine, newLine = oldStart, newStart
			inHunk = true
			continue
		}
		if !inHunk || ln.content == "" {
			continue
		}
		var oldN, newN int
		switch ln.content[0] {
		case ' ':
			oldN, newN = oldLine, newLine
			oldLine++
			newLine++
		case '-':
			oldN = oldLine
			oldLine++
		case '+':
			newN = newLine
			newLine++
		default:
			continue
		}
		if oldN > maxN {
			maxN = oldN
		}
		if newN > maxN {
			maxN = newN
		}
		if oldN > 0 || newN > 0 {
			numbered = true
		}
		nums[i] = [2]int{oldN, newN}
	}
	if !numbered {
		return ""
	}
	width := diffNumMinWidth
	if d := decimalDigits(maxN); d > width {
		width = d
	}
	var b strings.Builder
	b.Grow(len(lines) * (width*2 + 4))
	for i, ln := range lines {
		b.WriteByte(' ')
		b.WriteString(formatDiffNum(nums[i][0], width))
		b.WriteByte(' ')
		b.WriteString(formatDiffNum(nums[i][1], width))
		b.WriteByte(' ')
		if ln.nl {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type diffTextLine struct {
	content string
	nl      bool
}

func splitDiffTextLines(text string) []diffTextLine {
	if text == "" {
		return nil
	}
	var out []diffTextLine
	for text != "" {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			out = append(out, diffTextLine{content: text})
			return out
		}
		out = append(out, diffTextLine{content: text[:i], nl: true})
		text = text[i+1:]
	}
	return out
}

func parseHunkStarts(line string) (oldStart, newStart int, ok bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return 0, 0, false
	}
	rest := line[len("@@ -"):]
	oldStart, rest, ok = parseHunkNum(rest)
	if !ok {
		return 0, 0, false
	}
	if strings.HasPrefix(rest, ",") {
		_, rest, ok = parseHunkNum(rest[1:])
		if !ok {
			return 0, 0, false
		}
	}
	rest = strings.TrimPrefix(rest, " ")
	if !strings.HasPrefix(rest, "+") {
		return 0, 0, false
	}
	newStart, _, ok = parseHunkNum(rest[1:])
	if !ok {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

func parseHunkNum(s string) (int, string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, s, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, s, false
	}
	return n, s[i:], true
}

func formatDiffNum(n, width int) string {
	if n <= 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*d", width, n)
}

func decimalDigits(n int) int {
	if n < 0 {
		n = -n
	}
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

type diffColors struct {
	addBg, delBg, numBg, hunk, meta color.NRGBA
}

func diffPalette(dark bool) diffColors {
	if dark {
		return diffColors{
			addBg: color.NRGBA{R: 0x1b, G: 0x3d, B: 0x26, A: 0xff},
			delBg: color.NRGBA{R: 0x3d, G: 0x1b, B: 0x1e, A: 0xff},
			numBg: color.NRGBA{R: 0x16, G: 0x1b, B: 0x22, A: 0xff},
			hunk:  color.NRGBA{R: 0x79, G: 0xc0, B: 0xff, A: 0xff},
			meta:  color.NRGBA{R: 0x8b, G: 0x94, B: 0x9e, A: 0xff},
		}
	}
	return diffColors{
		addBg: color.NRGBA{R: 0xda, G: 0xfb, B: 0xe1, A: 0xff},
		delBg: color.NRGBA{R: 0xff, G: 0xeb, B: 0xe9, A: 0xff},
		numBg: color.NRGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff},
		hunk:  color.NRGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
		meta:  color.NRGBA{R: 0x57, G: 0x60, B: 0x6a, A: 0xff},
	}
}
