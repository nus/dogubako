package app

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// splitDoc is a unified diff laid out as GitHub's split view:
// the old file on the left, the new file on the right.
// Deletions and additions from one change are paired on the same row.
// The side that has no line is blank. Header rows are repeated on both sides
// so the columns stay aligned.
type splitDoc struct {
	leftNum, leftBody   string
	rightNum, rightBody string
	leftKinds           []byte
	rightKinds          []byte
	leftSpans           []diffSpan
	rightSpans          []diffSpan
}

// diffSpan maps one displayed body line back to the unified diff.
// A blank filler row has srcEnd <= srcStart.
type diffSpan struct {
	viewStart, viewEnd int
	srcStart, srcEnd   int
}

type splitLine struct {
	num              int
	text             string
	kind             byte
	srcStart, srcEnd int
}

type splitRow struct {
	left, right splitLine
}

type diffSrcLine struct {
	content    string
	start, end int
}

func splitDiff(text string) splitDoc {
	lines := scanDiffLines(text)
	if len(lines) == 0 {
		return splitDoc{}
	}
	rows := make([]splitRow, 0, len(lines))
	oldLine, newLine := 0, 0
	inHunk := false
	maxN := 0
	note := func(n int) {
		if n > maxN {
			maxN = n
		}
	}
	for i := 0; i < len(lines); {
		ln := lines[i]
		if strings.HasPrefix(ln.content, "diff --git ") {
			inHunk = false
			rows = append(rows, splitBoth(ln, 'm'))
			i++
			continue
		}
		if oldStart, newStart, ok := parseHunkStarts(ln.content); ok {
			oldLine, newLine = oldStart, newStart
			inHunk = true
			rows = append(rows, splitBoth(ln, '@'))
			i++
			continue
		}
		if !inHunk || ln.content == "" {
			rows = append(rows, splitBoth(ln, 'm'))
			i++
			continue
		}
		switch ln.content[0] {
		case ' ':
			note(oldLine)
			note(newLine)
			payload := diffPayload(ln.content)
			rows = append(rows, splitRow{
				left:  splitSideLine(ln, oldLine, ' ', payload),
				right: splitSideLine(ln, newLine, ' ', payload),
			})
			oldLine++
			newLine++
			i++
		case '-', '+':
			var dels, adds []diffSrcLine
			var delNums, addNums []int
			for i < len(lines) {
				c := lines[i].content
				if c == "" || (c[0] != '-' && c[0] != '+') {
					break
				}
				if c[0] == '-' {
					note(oldLine)
					dels = append(dels, lines[i])
					delNums = append(delNums, oldLine)
					oldLine++
				} else {
					note(newLine)
					adds = append(adds, lines[i])
					addNums = append(addNums, newLine)
					newLine++
				}
				i++
			}
			n := max(len(dels), len(adds))
			for k := 0; k < n; k++ {
				var row splitRow
				if k < len(dels) {
					row.left = splitSideLine(dels[k], delNums[k], '-', diffPayload(dels[k].content))
				}
				if k < len(adds) {
					row.right = splitSideLine(adds[k], addNums[k], '+', diffPayload(adds[k].content))
				}
				rows = append(rows, row)
			}
		default:
			rows = append(rows, splitBoth(ln, 'm'))
			i++
		}
	}
	width := diffNumMinWidth
	if d := decimalDigits(maxN); maxN > 0 && d > width {
		width = d
	}
	return renderSplit(rows, width)
}

func splitBoth(ln diffSrcLine, kind byte) splitRow {
	line := splitLine{
		text:     ln.content,
		kind:     kind,
		srcStart: ln.start,
		srcEnd:   ln.end,
	}
	return splitRow{left: line, right: line}
}

func splitSideLine(ln diffSrcLine, num int, kind byte, text string) splitLine {
	return splitLine{
		num:      num,
		text:     text,
		kind:     kind,
		srcStart: ln.start,
		srcEnd:   ln.end,
	}
}

func diffPayload(content string) string {
	if content == "" {
		return ""
	}
	switch content[0] {
	case ' ', '+', '-':
		return content[1:]
	default:
		return content
	}
}

func scanDiffLines(text string) []diffSrcLine {
	if text == "" {
		return nil
	}
	var out []diffSrcLine
	for i := 0; i < len(text); {
		j := strings.IndexByte(text[i:], '\n')
		if j < 0 {
			out = append(out, diffSrcLine{content: text[i:], start: i, end: len(text)})
			break
		}
		j += i
		out = append(out, diffSrcLine{content: text[i:j], start: i, end: j + 1})
		i = j + 1
	}
	return out
}

func renderSplit(rows []splitRow, width int) splitDoc {
	var doc splitDoc
	var leftNum, leftBody, rightNum, rightBody strings.Builder
	for _, row := range rows {
		writeSplitSide(&leftNum, &leftBody, &doc.leftSpans, &doc.leftKinds, row.left, width)
		writeSplitSide(&rightNum, &rightBody, &doc.rightSpans, &doc.rightKinds, row.right, width)
	}
	doc.leftNum = leftNum.String()
	doc.leftBody = leftBody.String()
	doc.rightNum = rightNum.String()
	doc.rightBody = rightBody.String()
	return doc
}

func writeSplitSide(nums, body *strings.Builder, spans *[]diffSpan, kinds *[]byte, line splitLine, width int) {
	nums.WriteByte(' ')
	nums.WriteString(formatDiffNum(line.num, width))
	nums.WriteByte(' ')
	nums.WriteByte('\n')

	viewStart := body.Len()
	body.WriteString(line.text)
	body.WriteByte('\n')
	*spans = append(*spans, diffSpan{
		viewStart: viewStart,
		viewEnd:   body.Len(),
		srcStart:  line.srcStart,
		srcEnd:    line.srcEnd,
	})
	*kinds = append(*kinds, line.kind)
}

func overlappingSources(spans []diffSpan, selStart, selEnd int) [][2]int {
	if selEnd <= selStart {
		return nil
	}
	var out [][2]int
	for _, s := range spans {
		if s.srcEnd <= s.srcStart {
			continue
		}
		if s.viewStart < selEnd && s.viewEnd > selStart {
			out = append(out, [2]int{s.srcStart, s.srcEnd})
		}
	}
	return out
}

func styleSplit(text string, kinds []byte, dark, gutter bool) basicwidget.TextStyles {
	var styles basicwidget.TextStyles
	if text == "" {
		return styles
	}
	pal := diffPalette(dark)
	line := 0
	for start := 0; start < len(text); {
		end := diffLineEnd(text, start)
		var kind byte
		if line < len(kinds) {
			kind = kinds[line]
		}
		fg, bg := splitLineLook(kind, pal)
		if gutter {
			fg = pal.meta
			if bg.A == 0 {
				bg = pal.numBg
			}
		}
		if fg.A != 0 {
			styles.SetColorInRange(start, end, fg)
		}
		if bg.A != 0 {
			styles.SetBackgroundColorInRange(start, end, bg)
		}
		line++
		if end == len(text) {
			break
		}
		start = end
	}
	return styles
}

func splitLineLook(kind byte, pal diffColors) (fg, bg color.NRGBA) {
	switch kind {
	case 'm':
		return pal.meta, color.NRGBA{}
	case '@':
		return pal.hunk, color.NRGBA{}
	case '-':
		return color.NRGBA{}, pal.delBg
	case '+':
		return color.NRGBA{}, pal.addBg
	case ' ':
		return color.NRGBA{}, color.NRGBA{}
	default:
		return color.NRGBA{}, pal.numBg
	}
}

// gitDiffSide is one column of a diff: line numbers beside the code.
// Compact view also has a marker column. Those characters are drawn and are not selectable.
// The code scrolls horizontally, so a line that runs past the pane can be
// brought into view. Numbers and markers stay at the left and move only
// vertically with the code.
// Wrapping breaks the code inside the pane. The number and marker columns stay
// one row per source line by inserting blank rows under a line that wraps,
// so the next number does not slide up beside the continuation.
type gitDiffSide struct {
	guigui.DefaultWidget

	nums  basicwidget.Text
	marks basicwidget.Text
	body  gitDiffText
	clip  diffBodyClip
	pad   diffScrollPad

	// scrollX and scrollY move the code. Both are zero or negative.
	// Numbers and markers follow scrollY and stay at the left.
	scrollX   float64
	scrollY   float64
	onScrollY func(float64)

	// viewW/viewH is the code viewport. contentW/contentH is the code's size,
	// which is wider or taller than the viewport when there is something to scroll.
	viewW, viewH       int
	contentW, contentH int

	numText   string
	markText  string
	bodyText  string
	numStyle  basicwidget.TextStyles
	markStyle basicwidget.TextStyles
	bodyStyle basicwidget.TextStyles
	numKinds  []byte
	markKinds []byte
	dark      bool
	showNums  bool
	showMarks bool

	wrap bool

	// ownCounts is how many visual rows each logical body line occupies.
	// It is measured from the body widget after layout, which is the text
	// that actually wraps. A separate probe never receives font metrics, so
	// it reports one row per line and the following line numbers slide up
	// onto the continuation.
	ownCounts  []int
	countGen   int
	countWidth int
	boundBuf   []image.Rectangle
	bodyGen    int

	colReady     bool
	colGen       int
	colDark      bool
	colCounts    []int
	colNum       string
	colMark      string
	colNumStyle  basicwidget.TextStyles
	colMarkStyle basicwidget.TextStyles
}

func (s *gitDiffSide) setWrap(on bool) {
	if s.wrap == on {
		return
	}
	s.wrap = on
	s.ownCounts = nil
	s.countWidth = 0
	s.colReady = false
}

func (s *gitDiffSide) prepare(num, body string, numStyle, bodyStyle basicwidget.TextStyles, kinds []byte, dark bool) {
	s.prepareMarked(num, "", body, numStyle, basicwidget.TextStyles{}, bodyStyle, kinds, nil, dark)
}

func (s *gitDiffSide) prepareMarked(num, marks, body string, numStyle, markStyle, bodyStyle basicwidget.TextStyles, numKinds, markKinds []byte, dark bool) {
	if num != s.numText || marks != s.markText || body != s.bodyText {
		s.bodyGen++
		s.colReady = false
		s.ownCounts = nil
		s.countWidth = 0
	}
	if dark != s.dark {
		s.colReady = false
	}
	s.numText = num
	s.markText = marks
	s.bodyText = body
	s.numStyle = numStyle
	s.markStyle = markStyle
	s.bodyStyle = bodyStyle
	s.numKinds = numKinds
	s.markKinds = markKinds
	s.dark = dark
	s.showNums = num != ""
	s.showMarks = marks != ""
}

func (s *gitDiffSide) resetScroll() {
	s.scrollX, s.scrollY = 0, 0
}

func (s *gitDiffSide) setScrollY(y float64) {
	if y == s.scrollY {
		return
	}
	s.scrollY = y
	guigui.RequestRebuild()
}

// addScroll moves the code by dx, dy and reports whether the origin changed.
// A vertical change is reported to the other split column.
func (s *gitDiffSide) addScroll(dx, dy float64) bool {
	nx := clampScrollOffset(s.scrollX+dx, s.viewW, s.contentW)
	ny := clampScrollOffset(s.scrollY+dy, s.viewH, s.contentH)
	if nx == s.scrollX && ny == s.scrollY {
		return false
	}
	s.scrollX = nx
	yChanged := ny != s.scrollY
	s.scrollY = ny
	if yChanged && s.onScrollY != nil {
		s.onScrollY(ny)
	}
	guigui.RequestRebuild()
	return true
}

func (s *gitDiffSide) clampScroll() {
	s.scrollX = clampScrollOffset(s.scrollX, s.viewW, s.contentW)
	s.scrollY = clampScrollOffset(s.scrollY, s.viewH, s.contentH)
}

func (s *gitDiffSide) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	numText, markText, numStyle, markStyle := s.columnText()
	if s.showNums {
		s.nums.SetMultiline(true)
		s.nums.SetWrapMode(basicwidget.WrapModeNone)
		s.nums.SetSelectable(false)
		setGitDiffFont(&s.nums)
		s.nums.SetValue(numText)
		s.nums.SetOverrideStyles(&numStyle, false)
	}
	if s.showMarks {
		s.marks.SetMultiline(true)
		s.marks.SetWrapMode(basicwidget.WrapModeNone)
		s.marks.SetSelectable(false)
		setGitDiffFont(&s.marks)
		s.marks.SetValue(markText)
		s.marks.SetOverrideStyles(&markStyle, false)
	}
	s.body.SetMultiline(true)
	// Anywhere, not only at spaces: a diff line is often one long token.
	if s.wrap {
		s.body.SetWrapMode(basicwidget.WrapModeAnywhere)
	} else {
		s.body.SetWrapMode(basicwidget.WrapModeNone)
	}
	s.body.SetSelectable(true)
	s.body.SetEditable(false)
	s.body.SetSelectionVisibleWhenUnfocused(true)
	setGitDiffFont(&s.body.Text)
	s.body.SetValue(s.bodyText)
	s.body.SetOverrideStyles(&s.bodyStyle, false)

	s.clip.side = s
	s.pad.side = s
	adder.AddWidget(&s.clip)
	if s.showNums {
		adder.AddWidget(&s.nums)
	}
	if s.showMarks {
		adder.AddWidget(&s.marks)
	}
	// The pad is last so it sees the wheel before the text. Clicks fall through.
	adder.AddWidget(&s.pad)
	context.SetClipChildren(s, true)
	return nil
}

// columnText is the number and marker text for this frame. While wrapping,
// a source line that occupies several visual rows is followed by blank rows.
// The number stays on the first row. The following source line's number does
// not move up onto the continuation.
func (s *gitDiffSide) columnText() (num, mark string, numStyle, markStyle basicwidget.TextStyles) {
	num, mark = s.numText, s.markText
	numStyle, markStyle = s.numStyle, s.markStyle
	if !s.wrap || len(s.ownCounts) == 0 || (!s.showNums && !s.showMarks) {
		return num, mark, numStyle, markStyle
	}
	if s.colReady && s.colGen == s.bodyGen && s.colDark == s.dark && slices.Equal(s.colCounts, s.ownCounts) {
		return s.colNum, s.colMark, s.colNumStyle, s.colMarkStyle
	}
	counts := s.ownCounts
	if s.showNums {
		if expanded := expandWrappedLines(s.numText, counts); expanded != s.numText {
			if kinds := expandKinds(s.numKinds, counts); kinds != nil {
				num = expanded
				numStyle = styleSplit(num, kinds, s.dark, true)
			}
		}
	}
	if s.showMarks {
		if expanded := expandWrappedLines(s.markText, counts); expanded != s.markText {
			if kinds := expandKinds(s.markKinds, counts); kinds != nil {
				mark = expanded
				markStyle = styleSplit(mark, kinds, s.dark, false)
			}
		}
	}
	s.colReady = true
	s.colGen = s.bodyGen
	s.colDark = s.dark
	s.colCounts = append(s.colCounts[:0], counts...)
	s.colNum, s.colMark = num, mark
	s.colNumStyle, s.colMarkStyle = numStyle, markStyle
	return num, mark, numStyle, markStyle
}

func (s *gitDiffSide) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	numW, markW, bodyW := s.columnWidths(context, bounds.Dx())
	x := bounds.Min.X
	bodyX := x + numW + markW
	if bodyX > bounds.Max.X {
		bodyX = bounds.Max.X
	}
	bodyBounds := image.Rect(bodyX, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	if bodyBounds.Dx() < 1 {
		bodyBounds.Max.X = bodyBounds.Min.X + 1
	}
	if bodyBounds.Dy() < 1 {
		bodyBounds.Max.Y = bodyBounds.Min.Y + 1
	}
	if s.wrap {
		s.syncWrapCounts(context, image.Rect(0, 0, bodyW, max(bounds.Dy(), 1)))
	}
	s.viewW = bodyBounds.Dx()
	s.viewH = bodyBounds.Dy()
	s.contentW, s.contentH = s.contentSize(context, bodyW)
	s.clampScroll()
	layouter.LayoutWidget(&s.clip, bodyBounds)

	h := s.contentH
	y := bounds.Min.Y + int(s.scrollY)
	if s.showNums {
		layouter.LayoutWidget(&s.nums, image.Rect(x, y, x+numW, y+h))
		x += numW
	}
	if s.showMarks {
		layouter.LayoutWidget(&s.marks, image.Rect(x, y, x+markW, y+h))
	}
	layouter.LayoutWidget(&s.pad, bounds)
}

// contentSize is the code's scrollable size. Without wrapping, the width is
// the longest line, so a line past the pane can be scrolled into view.
func (s *gitDiffSide) contentSize(context *guigui.Context, bodyW int) (w, h int) {
	var constraints guigui.Constraints
	if s.wrap && bodyW > 0 {
		constraints = guigui.FixedWidthConstraints(bodyW)
	}
	size := s.body.Measure(context, constraints)
	w, h = size.X, size.Y
	if s.showNums {
		h = max(h, s.nums.Measure(context, guigui.Constraints{}).Y)
	}
	if s.showMarks {
		h = max(h, s.marks.Measure(context, guigui.Constraints{}).Y)
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// syncWrapCounts records how many visual rows each logical line uses in the
// body that is on screen. The count comes from that widget, after it has been
// built with the diff font. A line that does not wrap stays one row, so its
// number is not repeated on a continuation that is not there.
func (s *gitDiffSide) syncWrapCounts(context *guigui.Context, bodyBounds image.Rectangle) {
	width := bodyBounds.Dx()
	if width < 1 {
		width = 1
	}
	if s.countGen == s.bodyGen && s.countWidth == width && len(s.ownCounts) > 0 {
		return
	}
	counts := wrapLineCounts(context, &s.body.Text, s.bodyText, bodyBounds, &s.boundBuf)
	s.countGen = s.bodyGen
	s.countWidth = width
	if slices.Equal(counts, s.ownCounts) {
		return
	}
	s.ownCounts = append(s.ownCounts[:0], counts...)
	s.colReady = false
	guigui.RequestRebuild()
}

func (s *gitDiffSide) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
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

func (s *gitDiffSide) columnWidths(context *guigui.Context, total int) (numW, markW, bodyW int) {
	if s.showNums {
		numW = s.nums.Measure(context, guigui.Constraints{}).X
	}
	if s.showMarks {
		markW = s.marks.Measure(context, guigui.Constraints{}).X
	}
	bodyW = total - numW - markW
	if bodyW < 1 {
		bodyW = 1
	}
	return numW, markW, bodyW
}

// wrapLineCounts reports the visual rows of each logical line of text laid
// out in bounds. A blank logical line has no glyph rectangles and still
// occupies one row. bounds.Dx is the width the body wraps at.
func wrapLineCounts(context *guigui.Context, text *basicwidget.Text, value string, bounds image.Rectangle, rects *[]image.Rectangle) []int {
	n := text.LineCount()
	if n == 0 {
		return nil
	}
	if bounds.Dx() < 1 {
		bounds.Max.X = bounds.Min.X + 1
	}
	if bounds.Dy() < 1 {
		bounds.Max.Y = bounds.Min.Y + 1
	}
	counts := make([]int, n)
	buf := *rects
	for i := range n {
		start := text.LineStartInBytes(i)
		end := len(value)
		if i+1 < n {
			end = text.LineStartInBytes(i + 1)
		}
		buf = text.AppendBoundsOfTextRange(buf[:0], context, bounds, start, end)
		c := len(buf)
		if c < 1 {
			c = 1
		}
		counts[i] = c
	}
	*rects = buf
	return counts
}

// diffWheelDelta matches the panel scroll wheel, so scrolling over the line
// numbers moves the code by the same amount as scrolling over the code.
func diffWheelDelta(context *guigui.Context) (float64, float64) {
	x, y := ebiten.Wheel()
	switch runtime.GOOS {
	case "darwin":
		x *= 2
		y *= 2
	case "js":
	default:
		x *= 10
		y *= 10
	}
	speed := 4 * context.Scale()
	return x * speed, y * speed
}

func shiftDown() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift) ||
		ebiten.IsKeyPressed(ebiten.KeyShiftLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyShiftRight)
}

// diffWheelAxes turns a wheel delta into a code scroll.
// Shift scrolls sideways when a line runs past the pane. A diff that already
// fits vertically does too, so a mouse wheel can reach the end of a long line.
func diffWheelAxes(dx, dy float64, viewW, viewH, contentW, contentH int, shift bool) (float64, float64) {
	wide := viewW > 0 && contentW > viewW
	if wide && (shift || contentH <= viewH) {
		return dx + dy, 0
	}
	return dx, dy
}

func clampScrollOffset(v float64, view, content int) float64 {
	minV := float64(min(view-content, 0))
	if v < minV {
		return minV
	}
	if v > 0 {
		return 0
	}
	return v
}

// diffBodyClip keeps the code inside the column. The text itself is laid out
// at its full size and shifted by the scroll origin, so the clip reveals the
// part that would otherwise run past the pane.
type diffBodyClip struct {
	guigui.DefaultWidget

	side *gitDiffSide
}

func (c *diffBodyClip) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if c.side == nil {
		return nil
	}
	adder.AddWidget(&c.side.body)
	context.SetClipChildren(c, true)
	return nil
}

func (c *diffBodyClip) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	if c.side == nil {
		return
	}
	b := widgetBounds.Bounds()
	w := c.side.contentW
	h := c.side.contentH
	// Wrapping is tied to the pane. A narrower measurement would wrap again
	// inside the widget and leave a gap at the right edge.
	if c.side.wrap && w < b.Dx() {
		w = b.Dx()
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	x := b.Min.X + int(c.side.scrollX)
	y := b.Min.Y + int(c.side.scrollY)
	layouter.LayoutWidget(&c.side.body, image.Rect(x, y, x+w, y+h))
}

// diffScrollPad scrolls the code from the wheel. Clicks pass through to the text.
type diffScrollPad struct {
	guigui.DefaultWidget

	side *gitDiffSide
}

func (p *diffScrollPad) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	s := p.side
	if s == nil || !widgetBounds.IsHitAtCursor() {
		return guigui.HandleInputResult{}
	}
	dx, dy := diffWheelDelta(context)
	if dx == 0 && dy == 0 {
		return guigui.HandleInputResult{}
	}
	dx, dy = diffWheelAxes(dx, dy, s.viewW, s.viewH, s.contentW, s.contentH, shiftDown())
	if s.addScroll(dx, dy) {
		return guigui.HandleInputByWidget(p)
	}
	return guigui.HandleInputResult{}
}

// gitDiffModeBar switches the diff between the compact and split layouts.
type gitDiffModeBar struct {
	guigui.DefaultWidget

	bar     basicwidget.SegmentedControl[bool]
	items   []basicwidget.SegmentedControlItem[bool]
	split   bool
	onSplit func(bool)
}

func (b *gitDiffModeBar) Set(lang i18n.Lang, split bool, onSplit func(bool)) {
	b.split = split
	b.onSplit = onSplit
	b.items = b.items[:0]
	b.items = append(b.items,
		basicwidget.SegmentedControlItem[bool]{Text: i18n.T(lang, i18n.GitDiffCompact), Value: false},
		basicwidget.SegmentedControlItem[bool]{Text: i18n.T(lang, i18n.GitDiffSplit), Value: true},
	)
}

func (b *gitDiffModeBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&b.bar)
	b.bar.SetItems(b.items)
	b.bar.SelectItemByValue(b.split)
	b.bar.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := b.bar.ItemByIndex(index)
		if !ok || item.Value == b.split || b.onSplit == nil {
			return
		}
		b.onSplit(item.Value)
	})
	return nil
}

func (b *gitDiffModeBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&b.bar, widgetBounds.Bounds())
}

func (b *gitDiffModeBar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return b.bar.Measure(context, constraints)
}

// gitDiffWrapBar switches the diff between scrolling sideways and wrapping.
// The segments are icons: a line that runs off to the right, and a line that turns back.
// Hovering a segment shows its name.
type gitDiffWrapBar struct {
	guigui.DefaultWidget

	bar    basicwidget.SegmentedControl[bool]
	items  []basicwidget.SegmentedControlItem[bool]
	tips   [2]basicwidget.TooltipArea
	lang   i18n.Lang
	wrap   bool
	onWrap func(bool)
}

func (b *gitDiffWrapBar) Set(lang i18n.Lang, wrap bool, onWrap func(bool)) {
	b.lang = lang
	b.wrap = wrap
	b.onWrap = onWrap
}

func (b *gitDiffWrapBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	size := basicwidget.LineHeight(context)
	b.items = b.items[:0]
	b.items = append(b.items,
		basicwidget.SegmentedControlItem[bool]{Icon: diffWrapIcon(false, context.ColorMode(), size), Value: false},
		basicwidget.SegmentedControlItem[bool]{Icon: diffWrapIcon(true, context.ColorMode(), size), Value: true},
	)
	adder.AddWidget(&b.bar)
	adder.AddWidget(&b.tips[0])
	adder.AddWidget(&b.tips[1])
	b.tips[0].SetText(i18n.T(b.lang, i18n.GitDiffNoWrap))
	b.tips[1].SetText(i18n.T(b.lang, i18n.GitDiffWrap))
	b.bar.SetItems(b.items)
	b.bar.SelectItemByValue(b.wrap)
	b.bar.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := b.bar.ItemByIndex(index)
		if !ok || item.Value == b.wrap || b.onWrap == nil {
			return
		}
		b.onWrap(item.Value)
	})
	return nil
}

func (b *gitDiffWrapBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&b.bar, bounds)
	mid := bounds.Min.X + bounds.Dx()/2
	layouter.LayoutWidget(&b.tips[0], image.Rect(bounds.Min.X, bounds.Min.Y, mid, bounds.Max.Y))
	layouter.LayoutWidget(&b.tips[1], image.Rect(mid, bounds.Min.Y, bounds.Max.X, bounds.Max.Y))
}

func (b *gitDiffWrapBar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return b.bar.Measure(context, constraints)
}

type diffWrapIconKey struct {
	wrap bool
	mode ebiten.ColorMode
	size int
}

var diffWrapIcons = map[diffWrapIconKey]*ebiten.Image{}

// diffWrapIcon draws the wrap toggle. wrap is false for the line that keeps
// going right, and true for the line that turns down onto the next row.
func diffWrapIcon(wrap bool, mode ebiten.ColorMode, size int) *ebiten.Image {
	if size < 16 {
		size = 16
	}
	key := diffWrapIconKey{wrap: wrap, mode: mode, size: size}
	if img, ok := diffWrapIcons[key]; ok {
		return img
	}
	raster := size * 2
	img := ebiten.NewImage(raster, raster)
	sw := float32(raster) / 10
	if sw < 2 {
		sw = 2
	}
	for _, s := range diffWrapStrokes(wrap, float32(raster)) {
		vector.StrokeLine(img, s[0], s[1], s[2], s[3], sw, textColor(mode), true)
	}
	diffWrapIcons[key] = img
	return img
}

// diffWrapStrokes returns the glyph in a size×size box.
// A false wrap is three text lines, the first ending in a right arrow.
// A true wrap bends that line down and points back to the left.
func diffWrapStrokes(wrap bool, size float32) [][4]float32 {
	left := size * 0.16
	right := size * 0.84
	y1 := size * 0.28
	y2 := size * 0.50
	y3 := size * 0.74
	ah := size * 0.18
	spread := ah * 0.55
	var strokes [][4]float32
	add := func(x0, y0, x1, y1 float32) {
		strokes = append(strokes, [4]float32{x0, y0, x1, y1})
	}
	if wrap {
		add(left, y1, right, y1)
		add(right, y1, right, y2)
		add(right, y2, left+ah, y2)
		add(left+ah, y2-spread, left, y2)
		add(left+ah, y2+spread, left, y2)
		add(left, y3, size*0.50, y3)
		return strokes
	}
	add(left, y1, right-ah, y1)
	add(right-ah, y1-spread, right, y1)
	add(right-ah, y1+spread, right, y1)
	add(left, y2, size*0.58, y2)
	add(left, y3, size*0.42, y3)
	return strokes
}
