package app

import (
	"image"
	"image/color"
	"strings"

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
type gitDiffSide struct {
	guigui.DefaultWidget

	nums basicwidget.Text
	body gitDiffText

	numText   string
	bodyText  string
	numStyle  basicwidget.TextStyles
	bodyStyle basicwidget.TextStyles
	showNums  bool

	items []guigui.LinearLayoutItem
}

func (s *gitDiffSide) prepare(num, body string, numStyle, bodyStyle basicwidget.TextStyles) {
	s.numText = num
	s.bodyText = body
	s.numStyle = numStyle
	s.bodyStyle = bodyStyle
	s.showNums = num != ""
}

func (s *gitDiffSide) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if s.showNums {
		adder.AddWidget(&s.nums)
		s.nums.SetMultiline(true)
		s.nums.SetWrapMode(basicwidget.WrapModeNone)
		s.nums.SetSelectable(false)
		setGitDiffFont(&s.nums)
		s.nums.SetValue(s.numText)
		s.nums.SetOverrideStyles(&s.numStyle, false)
	}
	adder.AddWidget(&s.body)
	s.body.SetMultiline(true)
	s.body.SetWrapMode(basicwidget.WrapModeNone)
	s.body.SetSelectable(true)
	s.body.SetEditable(false)
	s.body.SetSelectionVisibleWhenUnfocused(true)
	setGitDiffFont(&s.body.Text)
	s.body.SetValue(s.bodyText)
	s.body.SetOverrideStyles(&s.bodyStyle, false)
	return nil
}

func (s *gitDiffSide) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.row().LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (s *gitDiffSide) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return s.row().Measure(context, constraints)
}

func (s *gitDiffSide) row() guigui.LinearLayout {
	s.items = s.items[:0]
	if s.showNums {
		s.items = append(s.items, guigui.LinearLayoutItem{Widget: &s.nums})
	}
	s.items = append(s.items, guigui.LinearLayoutItem{Widget: &s.body})
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     s.items,
	}
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
