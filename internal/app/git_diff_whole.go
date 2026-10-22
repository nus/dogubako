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
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/i18n"
)

// expLine is one line of a whole-file diff.
// start and end are its bytes in that diff.
// srcStart and srcEnd are the same line in the original patch.
// A line that the original patch does not contain has srcEnd <= srcStart.
type expLine struct {
	start, end       int
	srcStart, srcEnd int
}

// diffIsBinary reports a diff git could not expand into text lines.
func diffIsBinary(text string) bool {
	if strings.Contains(text, "GIT binary patch") {
		return true
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "Binary file ") {
			return true
		}
	}
	return false
}

// diffAlreadyWhole reports a new or deleted file, whose patch is already the whole file.
func diffAlreadyWhole(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "new file mode "), strings.HasPrefix(line, "deleted file mode "):
			return true
		case line == "--- /dev/null", line == "+++ /dev/null":
			return true
		}
	}
	return false
}

// displayDiff is the text the diff view renders.
// In whole-file mode, full is a unified diff with the lines the hunks skipped.
// The returned lines map that text back to the original patch.
// A file that is already complete, or that cannot be expanded, keeps the original text.
func displayDiff(text string, whole bool, full string) (string, []expLine) {
	if !whole || full == "" {
		return text, nil
	}
	lines, ok := wholeFileSources(text, full)
	if !ok {
		return text, nil
	}
	return full, lines
}

// wholeFileSources aligns full, a wider-context diff of the same change, with original.
// Added and removed lines keep the original patch's byte ranges so a selection can
// stage those lines. Context that only the wide diff contains maps to nothing.
// expanded is false when full adds no lines.
func wholeFileSources(original, full string) ([]expLine, bool) {
	if diffIsBinary(original) || diffIsBinary(full) || strings.TrimSpace(full) == "" {
		return nil, false
	}
	dels, adds := diffChangeOffsets(original)
	origBody := diffBodyCount(original)
	srcLines := scanDiffLines(full)
	if len(srcLines) == 0 {
		return nil, false
	}
	var out []expLine
	oldLine, newLine := 0, 0
	inHunk := false
	body := 0
	for _, ln := range srcLines {
		el := expLine{start: ln.start, end: ln.end}
		if strings.HasPrefix(ln.content, "diff --git ") {
			inHunk = false
			out = append(out, el)
			continue
		}
		if oldStart, newStart, ok := parseHunkStarts(ln.content); ok {
			oldLine, newLine = oldStart, newStart
			inHunk = true
			out = append(out, el)
			continue
		}
		if !inHunk || ln.content == "" {
			out = append(out, el)
			continue
		}
		switch ln.content[0] {
		case ' ':
			body++
			oldLine++
			newLine++
		case '-':
			body++
			if src, ok := dels[oldLine]; ok {
				el.srcStart, el.srcEnd = src[0], src[1]
			}
			oldLine++
		case '+':
			body++
			if src, ok := adds[newLine]; ok {
				el.srcStart, el.srcEnd = src[0], src[1]
			}
			newLine++
		}
		out = append(out, el)
	}
	if body <= origBody {
		return nil, false
	}
	return out, true
}

func diffBodyCount(text string) int {
	n := 0
	inHunk := false
	for _, ln := range scanDiffLines(text) {
		if strings.HasPrefix(ln.content, "diff --git ") {
			inHunk = false
			continue
		}
		if _, _, ok := parseHunkStarts(ln.content); ok {
			inHunk = true
			continue
		}
		if !inHunk || ln.content == "" {
			continue
		}
		switch ln.content[0] {
		case ' ', '-', '+':
			n++
		}
	}
	return n
}

// diffChangeOffsets records the original patch bytes of each added and removed line,
// keyed by that side's line number.
func diffChangeOffsets(text string) (dels, adds map[int][2]int) {
	dels = map[int][2]int{}
	adds = map[int][2]int{}
	oldLine, newLine := 0, 0
	inHunk := false
	for _, ln := range scanDiffLines(text) {
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
		switch ln.content[0] {
		case ' ':
			oldLine++
			newLine++
		case '-':
			dels[oldLine] = [2]int{ln.start, ln.end}
			oldLine++
		case '+':
			adds[newLine] = [2]int{ln.start, ln.end}
			newLine++
		}
	}
	return dels, adds
}

// remapDiffSpans points spans at the original patch.
// spans currently refer to the whole-file diff. A line with no original bytes is cleared
// so it cannot be staged.
func remapDiffSpans(spans []diffSpan, lines []expLine) []diffSpan {
	if len(lines) == 0 {
		return spans
	}
	byStart := make(map[int]expLine, len(lines))
	for _, ln := range lines {
		byStart[ln.start] = ln
	}
	out := make([]diffSpan, len(spans))
	for i, s := range spans {
		out[i] = s
		if s.srcEnd <= s.srcStart {
			continue
		}
		ln, ok := byStart[s.srcStart]
		if !ok || ln.srcEnd <= ln.srcStart {
			out[i].srcStart, out[i].srcEnd = 0, 0
			continue
		}
		out[i].srcStart = ln.srcStart
		out[i].srcEnd = ln.srcEnd
	}
	return out
}

// gitDiffWholeBar toggles between hunks and the whole file.
// The control is an icon. Hovering it shows its name.
// A binary file disables it.
type gitDiffWholeBar struct {
	guigui.DefaultWidget

	button  basicwidget.Button
	tip     basicwidget.TooltipArea
	lang    i18n.Lang
	on      bool
	enabled bool
	onWhole func(bool)
}

func (b *gitDiffWholeBar) Set(lang i18n.Lang, on, enabled bool, onWhole func(bool)) {
	b.lang = lang
	b.on = on
	b.enabled = enabled
	b.onWhole = onWhole
}

func (b *gitDiffWholeBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	size := basicwidget.LineHeight(context)
	b.button.SetIcon(diffWholeIcon(context.ColorMode(), size))
	b.button.SetText("")
	b.button.SetToggleable(true)
	b.button.SetPressed(b.on && b.enabled)
	b.button.OnDown(func(*guigui.Context) {
		if b.onWhole != nil && b.enabled {
			b.onWhole(!b.on)
		}
	})
	context.SetEnabled(&b.button, b.enabled)
	adder.AddWidget(&b.button)
	adder.AddWidget(&b.tip)
	b.tip.SetText(i18n.T(b.lang, i18n.GitDiffWhole))
	return nil
}

func (b *gitDiffWholeBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&b.button, bounds)
	layouter.LayoutWidget(&b.tip, bounds)
}

func (b *gitDiffWholeBar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return b.button.Measure(context, constraints)
}

type diffWholeIconKey struct {
	mode ebiten.ColorMode
	size int
}

var diffWholeIcons = map[diffWholeIconKey]*ebiten.Image{}

// diffWholeIcon draws a page of text, the toggle for showing every line.
func diffWholeIcon(mode ebiten.ColorMode, size int) *ebiten.Image {
	if size < 16 {
		size = 16
	}
	key := diffWholeIconKey{mode: mode, size: size}
	if img, ok := diffWholeIcons[key]; ok {
		return img
	}
	raster := size * 2
	img := ebiten.NewImage(raster, raster)
	sw := float32(raster) / 10
	if sw < 2 {
		sw = 2
	}
	for _, s := range diffWholeStrokes(float32(raster)) {
		vector.StrokeLine(img, s[0], s[1], s[2], s[3], sw, textColor(mode), true)
	}
	diffWholeIcons[key] = img
	return img
}

// diffWholeStrokes returns a document glyph in a size×size box.
// A folded corner and four lines distinguish it from a single changed hunk.
func diffWholeStrokes(size float32) [][4]float32 {
	left := size * 0.18
	right := size * 0.82
	top := size * 0.14
	bottom := size * 0.86
	fold := size * 0.16
	var strokes [][4]float32
	add := func(x0, y0, x1, y1 float32) {
		strokes = append(strokes, [4]float32{x0, y0, x1, y1})
	}
	add(left, top, right-fold, top)
	add(right-fold, top, right, top+fold)
	add(right-fold, top, right-fold, top+fold)
	add(right-fold, top+fold, right, top+fold)
	add(right, top+fold, right, bottom)
	add(right, bottom, left, bottom)
	add(left, bottom, left, top)
	pad := size * 0.12
	for _, y := range []float32{0.40, 0.52, 0.64, 0.76} {
		add(left+pad, size*y, right-pad, size*y)
	}
	return strokes
}

// gitDiffHunkBar jumps to the previous or next hunk.
// The controls are icons. Hovering one shows its name.
type gitDiffHunkBar struct {
	guigui.DefaultWidget

	prev    basicwidget.Button
	next    basicwidget.Button
	prevTip basicwidget.TooltipArea
	nextTip basicwidget.TooltipArea
	lang    i18n.Lang
	canPrev bool
	canNext bool
	onJump  func(lineH int, next bool)
}

func (b *gitDiffHunkBar) Set(lang i18n.Lang, canPrev, canNext bool, onJump func(lineH int, next bool)) {
	b.lang = lang
	b.canPrev = canPrev
	b.canNext = canNext
	b.onJump = onJump
}

func (b *gitDiffHunkBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	size := basicwidget.LineHeight(context)
	mode := context.ColorMode()
	b.prev.SetIcon(diffHunkIcon(false, mode, size))
	b.next.SetIcon(diffHunkIcon(true, mode, size))
	b.prev.SetText("")
	b.next.SetText("")
	b.prev.OnDown(func(ctx *guigui.Context) {
		if b.onJump != nil && b.canPrev {
			b.onJump(basicwidget.LineHeight(ctx), false)
		}
	})
	b.next.OnDown(func(ctx *guigui.Context) {
		if b.onJump != nil && b.canNext {
			b.onJump(basicwidget.LineHeight(ctx), true)
		}
	})
	context.SetEnabled(&b.prev, b.canPrev)
	context.SetEnabled(&b.next, b.canNext)
	adder.AddWidget(&b.prev)
	adder.AddWidget(&b.next)
	adder.AddWidget(&b.prevTip)
	adder.AddWidget(&b.nextTip)
	b.prevTip.SetText(i18n.T(b.lang, i18n.GitDiffPrevHunk))
	b.nextTip.SetText(i18n.T(b.lang, i18n.GitDiffNextHunk))
	return nil
}

func (b *gitDiffHunkBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	gap := basicwidget.UnitSize(context) / 8
	if gap < 1 {
		gap = 1
	}
	prev := b.prev.Measure(context, guigui.Constraints{})
	next := b.next.Measure(context, guigui.Constraints{})
	prevR := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+prev.X, bounds.Max.Y)
	nextX := prevR.Max.X + gap
	nextR := image.Rect(nextX, bounds.Min.Y, nextX+next.X, bounds.Max.Y)
	layouter.LayoutWidget(&b.prev, prevR)
	layouter.LayoutWidget(&b.next, nextR)
	layouter.LayoutWidget(&b.prevTip, prevR)
	layouter.LayoutWidget(&b.nextTip, nextR)
}

func (b *gitDiffHunkBar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	prev := b.prev.Measure(context, constraints)
	next := b.next.Measure(context, constraints)
	gap := basicwidget.UnitSize(context) / 8
	if gap < 1 {
		gap = 1
	}
	h := prev.Y
	if next.Y > h {
		h = next.Y
	}
	return image.Pt(prev.X+gap+next.X, h)
}

type diffHunkIconKey struct {
	next bool
	mode ebiten.ColorMode
	size int
}

var diffHunkIcons = map[diffHunkIconKey]*ebiten.Image{}

// diffHunkIcon draws a chevron. next is false for the previous hunk.
func diffHunkIcon(next bool, mode ebiten.ColorMode, size int) *ebiten.Image {
	if size < 16 {
		size = 16
	}
	key := diffHunkIconKey{next: next, mode: mode, size: size}
	if img, ok := diffHunkIcons[key]; ok {
		return img
	}
	raster := size * 2
	img := ebiten.NewImage(raster, raster)
	sw := float32(raster) / 10
	if sw < 2 {
		sw = 2
	}
	for _, s := range diffHunkStrokes(next, float32(raster)) {
		vector.StrokeLine(img, s[0], s[1], s[2], s[3], sw, textColor(mode), true)
	}
	diffHunkIcons[key] = img
	return img
}

// diffHunkStrokes returns a chevron in a size×size box.
func diffHunkStrokes(next bool, size float32) [][4]float32 {
	left := size * 0.22
	right := size * 0.78
	mid := size * 0.50
	var y0, y1 float32
	if next {
		y0, y1 = size*0.36, size*0.66
	} else {
		y0, y1 = size*0.64, size*0.34
	}
	return [][4]float32{
		{left, y0, mid, y1},
		{right, y0, mid, y1},
	}
}

// diffOverviewMark is a run of added or removed lines on the whole-file scrollbar.
// start and end are fractions of the file from the top.
// A split row can be both a deletion and an addition.
type diffOverviewMark struct {
	start, end float64
	add, del   bool
}

// overviewActive reports that the diff on screen is the whole file.
// A binary file has nothing to mark. A modified file waits until the wide diff arrives.
func (v *gitDiffView) overviewActive() bool {
	if !v.whole || v.text == "" || diffIsBinary(v.text) {
		return false
	}
	if diffAlreadyWhole(v.text) {
		return true
	}
	return v.full != "" && v.shown == v.full
}

func (v *gitDiffView) attachOverview(adder *guigui.ChildAdder) {
	if !v.overviewActive() {
		return
	}
	v.overview.view = v
	adder.AddWidget(&v.overview)
}

// layoutOverview reserves the scrollbar and returns the bounds left for the diff.
func (v *gitDiffView) layoutOverview(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter) image.Rectangle {
	if !v.overviewActive() {
		return bounds
	}
	w := diffOverviewWidth(context)
	gap := basicwidget.UnitSize(context) / 8
	if gap < 1 {
		gap = 1
	}
	if w+gap >= bounds.Dx() {
		w = bounds.Dx() / 5
		if w < 1 {
			w = 1
		}
		gap = 0
	}
	bar := image.Rect(bounds.Max.X-w, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	layouter.LayoutWidget(&v.overview, bar)
	bounds.Max.X = bar.Min.X - gap
	if bounds.Max.X < bounds.Min.X {
		bounds.Max.X = bounds.Min.X
	}
	return bounds
}

func diffOverviewWidth(context *guigui.Context) int {
	markW, gap, thumbW := diffOverviewParts(context.Scale())
	return markW + gap + thumbW
}

// diffOverviewParts is the mark lane, the gap, and the scrollbar lane.
// The scrollbar lane matches the width of the app's other scrollbars.
func diffOverviewParts(scale float64) (markW, gap, thumbW int) {
	if scale < 1 {
		scale = 1
	}
	markW = int(4 * scale)
	if markW < 3 {
		markW = 3
	}
	gap = int(scale)
	if gap < 1 {
		gap = 1
	}
	thumbW = int(12 * scale)
	if thumbW < 8 {
		thumbW = 8
	}
	return markW, gap, thumbW
}

// diffOverviewLanes splits the bar into the change marks and the scrollbar.
// A squeezed bar keeps a mark lane and gives the rest to the thumb.
func diffOverviewLanes(track image.Rectangle, scale float64) (marks, thumb image.Rectangle) {
	markW, gap, _ := diffOverviewParts(scale)
	if markW+gap >= track.Dx()-4 {
		markW = track.Dx() / 3
		if markW < 1 {
			markW = 1
		}
		gap = 0
		if markW >= track.Dx() {
			markW = track.Dx() / 2
		}
	}
	marks = image.Rect(track.Min.X, track.Min.Y, track.Min.X+markW, track.Max.Y)
	thumb = image.Rect(marks.Max.X+gap, track.Min.Y, track.Max.X, track.Max.Y)
	if thumb.Min.X > thumb.Max.X {
		thumb.Min.X = thumb.Max.X
	}
	return marks, thumb
}

func (v *gitDiffView) overviewMarks() []diffOverviewMark {
	if v.split {
		return diffOverviewMarks(v.leftKinds, v.rightKinds, v.overviewCounts())
	}
	return diffOverviewMarks(v.compactKinds, nil, v.overviewCounts())
}

// overviewCounts is how many visual rows each source line occupies while wrapping.
// Split columns wrap on their own, so a row uses the taller of the two.
func (v *gitDiffView) overviewCounts() []int {
	if !v.wrap {
		return nil
	}
	if !v.split {
		return v.compact.ownCounts
	}
	n := len(v.leftKinds)
	if len(v.rightKinds) < n {
		n = len(v.rightKinds)
	}
	if len(v.left.ownCounts) == 0 && len(v.right.ownCounts) == 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		w := 1
		if i < len(v.left.ownCounts) && v.left.ownCounts[i] > w {
			w = v.left.ownCounts[i]
		}
		if i < len(v.right.ownCounts) && v.right.ownCounts[i] > w {
			w = v.right.ownCounts[i]
		}
		out[i] = w
	}
	return out
}

// scrollGeom is the vertical scroll shared by the diff.
// scrollY is zero at the top and negative below that.
func (v *gitDiffView) scrollGeom() (scrollY float64, viewH, contentH int) {
	if v.split {
		scrollY = v.left.scrollY
		viewH = v.left.viewH
		contentH = v.left.contentH
		if v.right.viewH > viewH {
			viewH = v.right.viewH
		}
		if v.right.contentH > contentH {
			contentH = v.right.contentH
		}
		return scrollY, viewH, contentH
	}
	return v.compact.scrollY, v.compact.viewH, v.compact.contentH
}

// hunkSides is the line kinds on screen and whether those lines are the whole file.
// A split row is one line across both columns.
func (v *gitDiffView) hunkSides() (left, right []byte, whole bool) {
	if v.split {
		return v.leftKinds, v.rightKinds, v.overviewActive()
	}
	return v.compactKinds, nil, v.overviewActive()
}

// canJumpHunk reports whether the previous or next hunk is somewhere off the current line.
func (v *gitDiffView) canJumpHunk(lineH int, next bool) bool {
	_, ok := v.hunkDestination(lineH, next)
	return ok
}

// jumpHunk scrolls so the previous or next hunk starts at the top of the view.
// lineH is the height of one visual row.
func (v *gitDiffView) jumpHunk(lineH int, next bool) bool {
	dest, ok := v.hunkDestination(lineH, next)
	if !ok {
		return false
	}
	if lineH < 1 {
		lineH = 1
	}
	v.setOverviewScroll(-float64(diffVisualRowsBefore(dest, v.hunkRowCounts()) * lineH))
	return true
}

// hunkRowCounts is the visual rows of the column whose scroll offset is moved.
// Split columns wrap separately and share one pixel offset, so the left column
// decides where a line sits.
func (v *gitDiffView) hunkRowCounts() []int {
	if !v.wrap {
		return nil
	}
	if v.split {
		return v.left.ownCounts
	}
	return v.compact.ownCounts
}

func (v *gitDiffView) hunkDestination(lineH int, next bool) (int, bool) {
	left, right, whole := v.hunkSides()
	n := len(left)
	if right != nil && len(right) > n {
		n = len(right)
	}
	scrollY, _, _ := v.scrollGeom()
	line := diffLineAt(scrollY, v.hunkRowCounts(), lineH, n)
	return diffHunkNeighbor(diffHunkTargets(left, right, whole), line, next)
}

// diffHunkTargets lists where a jump lands.
// Hunk view uses each @@ line. Whole-file view uses the first line of each
// run of additions and deletions, since one wide hunk covers the file.
func diffHunkTargets(left, right []byte, whole bool) []int {
	n := len(left)
	if right != nil && len(right) > n {
		n = len(right)
	}
	var out []int
	prevChange := false
	for i := range n {
		hunk, change := diffRowChange(left, right, i)
		if whole {
			if change && !prevChange {
				out = append(out, i)
			}
		} else if hunk {
			out = append(out, i)
		}
		prevChange = change
	}
	return out
}

func diffRowChange(left, right []byte, i int) (hunk, change bool) {
	if i < len(left) {
		switch left[i] {
		case '@':
			hunk = true
		case '+', '-':
			change = true
		}
	}
	if i < len(right) {
		switch right[i] {
		case '@':
			hunk = true
		case '+', '-':
			change = true
		}
	}
	return hunk, change
}

// diffHunkNeighbor is the next target after line, or the previous target before it.
func diffHunkNeighbor(targets []int, line int, next bool) (int, bool) {
	if next {
		for _, t := range targets {
			if t > line {
				return t, true
			}
		}
		return 0, false
	}
	for i := len(targets) - 1; i >= 0; i-- {
		if targets[i] < line {
			return targets[i], true
		}
	}
	return 0, false
}

// diffVisualRowsBefore is how many visual rows sit above a logical line.
// counts is empty when lines are not wrapped.
func diffVisualRowsBefore(line int, counts []int) int {
	if line <= 0 {
		return 0
	}
	if len(counts) == 0 {
		return line
	}
	rows := 0
	for i := range line {
		c := 1
		if i < len(counts) && counts[i] > 0 {
			c = counts[i]
		}
		rows += c
	}
	return rows
}

// diffLineAt is the logical line at the top of a view scrolled by scrollY.
// scrollY is zero at the top and negative below that. Past the last line, it
// returns nLines.
func diffLineAt(scrollY float64, counts []int, lineH, nLines int) int {
	if lineH < 1 {
		lineH = 1
	}
	px := int(-scrollY)
	if px < 0 {
		px = 0
	}
	rows := 0
	for i := range nLines {
		c := 1
		if i < len(counts) && counts[i] > 0 {
			c = counts[i]
		}
		if px < (rows+c)*lineH {
			return i
		}
		rows += c
	}
	return nLines
}

func (v *gitDiffView) setOverviewScroll(y float64) {
	_, viewH, contentH := v.scrollGeom()
	y = clampScrollOffset(y, viewH, contentH)
	if v.split {
		v.leftY, v.rightY = y, y
		v.left.setScrollY(y)
		v.right.setScrollY(y)
		return
	}
	v.compact.setScrollY(y)
}

func (v *gitDiffView) scrollOverviewTo(track image.Rectangle, cursorY int) {
	_, viewH, contentH := v.scrollGeom()
	v.setOverviewScroll(overviewScrollOffset(cursorY, track.Min.Y, track.Dy(), viewH, contentH))
}

func (v *gitDiffView) nudgeOverviewScroll(dy float64) {
	y, _, _ := v.scrollGeom()
	v.setOverviewScroll(y + dy)
}

// overviewScrollOffset scrolls so the file fraction under cursorY sits in the middle of the view.
func overviewScrollOffset(cursorY, trackY, trackH, viewH, contentH int) float64 {
	if trackH <= 0 || viewH <= 0 || contentH <= viewH {
		return 0
	}
	frac := float64(cursorY-trackY) / float64(trackH)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	top := frac*float64(contentH) - float64(viewH)/2
	return clampScrollOffset(-top, viewH, contentH)
}

// diffOverviewMarks collapses changed lines into scrollbar spans.
// right is the new side of a split diff. A nil right reads additions and deletions from left,
// which is the compact diff. counts is the visual height of each line; missing counts are one row.
// Unchanged lines and headers leave a gap, so two hunks do not merge across the file.
func diffOverviewMarks(left, right []byte, counts []int) []diffOverviewMark {
	n := len(left)
	if right != nil {
		n = min(len(left), len(right))
	}
	if n == 0 {
		return nil
	}
	total := 0
	weights := make([]int, n)
	for i := range weights {
		w := 1
		if i < len(counts) && counts[i] > 1 {
			w = counts[i]
		}
		weights[i] = w
		total += w
	}
	if total == 0 {
		return nil
	}
	var out []diffOverviewMark
	pos := 0
	for i := range n {
		add, del := overviewLineChange(left, right, i)
		start := float64(pos) / float64(total)
		pos += weights[i]
		end := float64(pos) / float64(total)
		if !add && !del {
			continue
		}
		if len(out) > 0 {
			prev := &out[len(out)-1]
			if prev.add == add && prev.del == del && prev.end == start {
				prev.end = end
				continue
			}
		}
		out = append(out, diffOverviewMark{start: start, end: end, add: add, del: del})
	}
	return out
}

func overviewLineChange(left, right []byte, i int) (add, del bool) {
	if i < len(left) {
		switch left[i] {
		case '+':
			add = true
		case '-':
			del = true
		}
	}
	if right != nil && i < len(right) {
		switch right[i] {
		case '+':
			add = true
		case '-':
			del = true
		}
	}
	return add, del
}

// diffOverviewMarkRect is the pixel span of one mark. A single line stays at least minH tall
// so it remains visible in a long file.
func diffOverviewMarkRect(track image.Rectangle, start, end float64, minH int) image.Rectangle {
	if track.Dy() <= 0 || track.Dx() <= 0 {
		return image.Rectangle{}
	}
	if start < 0 {
		start = 0
	}
	if end > 1 {
		end = 1
	}
	if end < start {
		end = start
	}
	if minH < 1 {
		minH = 1
	}
	if minH > track.Dy() {
		minH = track.Dy()
	}
	y0 := track.Min.Y + int(start*float64(track.Dy()))
	y1 := track.Min.Y + int(end*float64(track.Dy())+0.999)
	if y1 <= y0 {
		y1 = y0 + 1
	}
	if y1-y0 < minH {
		extra := minH - (y1 - y0)
		y0 -= extra / 2
		y1 = y0 + minH
	}
	if y0 < track.Min.Y {
		y1 += track.Min.Y - y0
		y0 = track.Min.Y
	}
	if y1 > track.Max.Y {
		y0 -= y1 - track.Max.Y
		y1 = track.Max.Y
	}
	if y0 < track.Min.Y {
		y0 = track.Min.Y
	}
	if y1 <= y0 {
		y1 = y0 + 1
		if y1 > track.Max.Y {
			y1 = track.Max.Y
		}
	}
	return image.Rect(track.Min.X, y0, track.Max.X, y1)
}

// overviewThumbRect is the part of the file currently on screen.
// It uses the same fraction of the file as the change marks.
func overviewThumbRect(track image.Rectangle, scrollY float64, viewH, contentH, minH int) image.Rectangle {
	if track.Dy() <= 0 || track.Dx() <= 0 || contentH <= 0 || viewH <= 0 || contentH <= viewH {
		return image.Rectangle{}
	}
	if minH < 1 {
		minH = 1
	}
	if minH > track.Dy() {
		minH = track.Dy()
	}
	topFrac := -scrollY / float64(contentH)
	if topFrac < 0 {
		topFrac = 0
	}
	heightFrac := float64(viewH) / float64(contentH)
	y0 := track.Min.Y + int(topFrac*float64(track.Dy()))
	h := int(heightFrac*float64(track.Dy()) + 0.5)
	if h < minH {
		h = minH
	}
	if h > track.Dy() {
		h = track.Dy()
	}
	if y0+h > track.Max.Y {
		y0 = track.Max.Y - h
	}
	if y0 < track.Min.Y {
		y0 = track.Min.Y
	}
	return image.Rect(track.Min.X, y0, track.Max.X, y0+h)
}

func diffOverviewThumbColor(dark bool) color.NRGBA {
	// Same strength as the app's scrollbar when it is fully shown:
	// foreground at 0.8, then the shown opacity of 0.75.
	if dark {
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x99}
	}
	return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
}

// diffOverviewBar is the whole-file scrollbar.
// Change marks use the diff's addition and deletion colors. Dragging scrolls the file
// so the point under the cursor stays in view.
type diffOverviewBar struct {
	guigui.DefaultWidget

	view *gitDiffView
	drag bool
}

func (b *diffOverviewBar) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if b.view == nil || !b.view.overviewActive() {
		b.drag = false
		return guigui.HandleInputResult{}
	}
	bounds := widgetBounds.Bounds()
	if bounds.Empty() {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && widgetBounds.IsHitAtCursor() {
		b.drag = true
		_, y := ebiten.CursorPosition()
		b.view.scrollOverviewTo(bounds, y)
		return guigui.HandleInputByWidget(b)
	}
	if b.drag && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		_, y := ebiten.CursorPosition()
		b.view.scrollOverviewTo(bounds, y)
		return guigui.HandleInputByWidget(b)
	}
	if b.drag && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		b.drag = false
	}
	if widgetBounds.IsHitAtCursor() {
		_, dy := diffWheelDelta(context)
		if dy != 0 {
			b.view.nudgeOverviewScroll(dy)
			return guigui.HandleInputByWidget(b)
		}
	}
	return guigui.HandleInputResult{}
}

func (b *diffOverviewBar) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if b.view == nil || !widgetBounds.IsHitAtCursor() {
		return 0, false
	}
	return ebiten.CursorShapePointer, true
}

// WriteStateKey includes the scroll geometry. The bar stays in place while the
// file scrolls, and a rebuild does not repaint a widget whose bounds did not
// change. The thumb would otherwise stay put until the window size changes.
func (b *diffOverviewBar) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if b.view == nil || !b.view.overviewActive() {
		w.WriteBool(false)
		return
	}
	w.WriteBool(true)
	scrollY, viewH, contentH := b.view.scrollGeom()
	w.WriteFloat64(scrollY)
	w.WriteInt(viewH)
	w.WriteInt(contentH)
	w.WriteBool(b.view.dark)
}

func (b *diffOverviewBar) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if b.view == nil || !b.view.overviewActive() {
		return
	}
	track := widgetBounds.Bounds()
	if track.Empty() {
		return
	}
	pal := diffPalette(b.view.dark)
	fillRect(dst, track, pal.numBg)
	marksLane, thumbLane := diffOverviewLanes(track, context.Scale())
	scrollY, viewH, contentH := b.view.scrollGeom()
	minThumb := basicwidget.UnitSize(context)
	if half := thumbLane.Dy() / 2; minThumb > half {
		minThumb = half
	}
	if minThumb < 4 {
		minThumb = 4
	}
	pad := int(2 * context.Scale())
	if pad < 1 {
		pad = 1
	}
	inner := thumbLane
	if inner.Dx() > pad*2+2 {
		inner.Min.X += pad
		inner.Max.X -= pad
	}
	if thumb := overviewThumbRect(inner, scrollY, viewH, contentH, minThumb); !thumb.Empty() {
		radius := thumb.Dx() / 2
		if radius < 1 {
			radius = 1
		}
		basicwidgetdraw.DrawRoundedRect(context, dst, thumb, diffOverviewThumbColor(b.view.dark), radius)
	}
	minMark := int(2 * context.Scale())
	if minMark < 2 {
		minMark = 2
	}
	for _, mark := range b.view.overviewMarks() {
		r := diffOverviewMarkRect(marksLane, mark.start, mark.end, minMark)
		switch {
		case mark.del && mark.add:
			mid := r.Min.X + r.Dx()/2
			if mid <= r.Min.X {
				mid = r.Min.X + 1
			}
			if mid >= r.Max.X {
				mid = r.Max.X
			}
			fillRect(dst, image.Rect(r.Min.X, r.Min.Y, mid, r.Max.Y), pal.delBg)
			fillRect(dst, image.Rect(mid, r.Min.Y, r.Max.X, r.Max.Y), pal.addBg)
		case mark.del:
			fillRect(dst, r, pal.delBg)
		case mark.add:
			fillRect(dst, r, pal.addBg)
		}
	}
}
