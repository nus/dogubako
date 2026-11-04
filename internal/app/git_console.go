package app

import (
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitConsoleLimit is how many commands one tab keeps. Older ones are dropped.
const gitConsoleLimit = 200

type gitConsoleEntry struct {
	line   string
	stdout string
	stderr string
	done   bool
}

type consoleSpanKind int

const (
	consoleCommand consoleSpanKind = iota
	consoleStdout
	consoleStderr
)

// consoleSpan is a byte range in the rendered transcript.
type consoleSpan struct {
	start int
	end   int
	kind  consoleSpanKind
}

// consolePrompt marks a command line so it is not read as output.
const consolePrompt = "$ "

// gitConsoleLog is one tab's command transcript. Git writes it from a
// background command. The UI reads it on the frame loop.
type gitConsoleLog struct {
	mu      sync.Mutex
	entries []gitConsoleEntry
	cached  string
	spans   []consoleSpan
	valid   bool
	dirty   bool
}

func (c *gitConsoleLog) onTrace(begin bool, args []string, stdout, stderr string) {
	if begin {
		c.begin(args)
		return
	}
	c.finish(stdout, stderr)
}

func (c *gitConsoleLog) begin(args []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, gitConsoleEntry{line: gitcli.FormatCommand(args)})
	if len(c.entries) > gitConsoleLimit {
		drop := len(c.entries) - gitConsoleLimit
		c.entries = append([]gitConsoleEntry(nil), c.entries[drop:]...)
	}
	c.valid = false
	c.dirty = true
}

func (c *gitConsoleLog) finish(stdout, stderr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.entries) - 1; i >= 0; i-- {
		if c.entries[i].done {
			continue
		}
		c.entries[i].stdout = stdout
		c.entries[i].stderr = stderr
		c.entries[i].done = true
		break
	}
	c.valid = false
	c.dirty = true
}

func (c *gitConsoleLog) text() string {
	text, _ := c.render()
	return text
}

// render is the transcript. A blank line separates command blocks.
// The command line starts with a prompt. Output keeps the bytes git wrote.
func (c *gitConsoleLog) render() (string, []consoleSpan) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.valid {
		var b strings.Builder
		var spans []consoleSpan
		for i, e := range c.entries {
			if i > 0 {
				b.WriteByte('\n')
			}
			start := b.Len()
			b.WriteString(consolePrompt)
			b.WriteString(e.line)
			b.WriteByte('\n')
			spans = append(spans, consoleSpan{start: start, end: b.Len(), kind: consoleCommand})
			if !e.done {
				continue
			}
			spans = appendConsoleStream(&b, spans, e.stdout, consoleStdout)
			spans = appendConsoleStream(&b, spans, e.stderr, consoleStderr)
		}
		c.cached = b.String()
		c.spans = spans
		c.valid = true
	}
	return c.cached, append([]consoleSpan(nil), c.spans...)
}

func (c *gitConsoleLog) consumeDirty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	dirty := c.dirty
	c.dirty = false
	return dirty
}

func appendConsoleStream(b *strings.Builder, spans []consoleSpan, s string, kind consoleSpanKind) []consoleSpan {
	if s == "" {
		return spans
	}
	start := b.Len()
	b.WriteString(s)
	if !strings.HasSuffix(s, "\n") {
		b.WriteByte('\n')
	}
	return append(spans, consoleSpan{start: start, end: b.Len(), kind: kind})
}

func consolePalette(dark bool) (command, stderr color.NRGBA) {
	if dark {
		return color.NRGBA{R: 0x79, G: 0xc0, B: 0xff, A: 0xff},
			color.NRGBA{R: 0xff, G: 0x7b, B: 0x72, A: 0xff}
	}
	return color.NRGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
		color.NRGBA{R: 0xcf, G: 0x22, B: 0x2e, A: 0xff}
}

// consoleTextStyles paints the command and standard error.
// Standard output keeps the normal text color.
func consoleTextStyles(spans []consoleSpan, dark bool) basicwidget.TextStyles {
	var styles basicwidget.TextStyles
	command, stderr := consolePalette(dark)
	for _, sp := range spans {
		if sp.start >= sp.end {
			continue
		}
		switch sp.kind {
		case consoleCommand:
			styles.SetColorInRange(sp.start, sp.end, command)
		case consoleStderr:
			styles.SetColorInRange(sp.start, sp.end, stderr)
		}
	}
	return styles
}

func (c *gitConsoleLog) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (m *gitSession) watchCommands(repo *gitcli.Repo) {
	if repo == nil {
		return
	}
	repo.SetCommandTrace(m.console.onTrace)
}

func (m *gitSession) ConsoleText() string {
	if m == nil {
		return ""
	}
	return m.console.text()
}

func (m *gitSession) consoleTranscript() (string, []consoleSpan) {
	if m == nil {
		return "", nil
	}
	return m.console.render()
}

// ConsoleFollowing reports that new output should keep the view at the bottom.
// A tab that has not been scrolled follows.
func (m *gitSession) ConsoleFollowing() bool {
	if m == nil || !m.consoleSeen {
		return true
	}
	return m.consoleFollow
}

func (m *gitSession) ConsoleOffset() float64 {
	if m == nil {
		return 0
	}
	return m.consoleOffset
}

// NoteConsoleScroll records the viewport. It does not redraw by itself.
func (m *gitSession) NoteConsoleScroll(offsetY float64, viewH, contentH int) {
	if m == nil {
		return
	}
	follow := consoleAtBottom(offsetY, viewH, contentH)
	if m.consoleSeen && m.consoleFollow == follow && m.consoleOffset == offsetY {
		return
	}
	m.consoleSeen = true
	m.consoleFollow = follow
	m.consoleOffset = offsetY
}

func (m *gitSession) drainConsole() {
	if m.console.consumeDirty() {
		m.generation++
		guigui.RequestRebuild()
	}
}

// gitConsoleHeight is a fixed block of several monospace lines, plus padding.
func gitConsoleHeight(u int) int {
	if u < 1 {
		u = 1
	}
	line := u * 3 / 4
	if line < 1 {
		line = 1
	}
	pad := u / 4
	return line*5 + pad*2
}

// gitViewBottoms is where the columns stop and, when open, where the console sits.
// Closed, the list still reaches the bottom of the git view and the right column
// keeps its margin. Open, a horizontal boundary sits on the console's top edge
// and both columns end on that boundary, shorter by the boundary's height.
// frac is the dragged console share of the space beside the boundary.
// Zero keeps the fixed height, including the shrink when the window is short.
func gitViewBottoms(rest image.Rectangle, u int, consoleOpen bool, frac float64) (listBottom, rightBottom int, split, console image.Rectangle) {
	listBottom = rest.Max.Y
	rightBottom = rest.Max.Y - u/2
	if rightBottom < rest.Min.Y {
		rightBottom = rest.Min.Y
	}
	if !consoleOpen || rest.Dy() <= 0 {
		return listBottom, rightBottom, image.Rectangle{}, image.Rectangle{}
	}
	splitH := gitWorkSplitWidth(u)
	var h int
	if frac > 0 {
		h = gitConsoleDragHeight(rest.Dy(), splitH, frac, gitConsoleMinHeight(u), gitHistMinPane(u))
	} else {
		h = gitConsoleFixedHeight(rest.Dy(), u)
	}
	if splitH > rest.Dy() {
		splitH = rest.Dy()
	}
	if room := rest.Dy() - splitH; h > room {
		h = room
	}
	if h < 0 {
		h = 0
	}
	consoleTop := rest.Max.Y - h
	splitTop := consoleTop - splitH
	return splitTop, splitTop,
		image.Rect(rest.Min.X, splitTop, rest.Max.X, consoleTop),
		image.Rect(rest.Min.X, consoleTop, rest.Max.X, rest.Max.Y)
}

// gitConsoleFixedHeight is the console body before its top edge is dragged.
// A short window keeps the smaller of two thirds and the whole remainder.
func gitConsoleFixedHeight(total, u int) int {
	h := gitConsoleHeight(u)
	if total < 1 {
		return 0
	}
	if h > total {
		h = total
	}
	if maxH := total * 2 / 3; maxH > 0 && h > maxH {
		h = maxH
	}
	if h < 1 {
		h = 1
	}
	return h
}

// gitConsoleMinHeight is one monospace line plus the console's padding.
func gitConsoleMinHeight(u int) int {
	if u < 1 {
		u = 1
	}
	line := u * 3 / 4
	if line < 1 {
		line = 1
	}
	pad := u / 4
	return line + pad*2
}

// gitConsoleDragHeight is the console body after the top edge has been dragged.
// frac is the console's share of the space beside the boundary.
// The body stays between one line and the height that leaves the upper panes,
// and it may pass the fixed view's two-thirds cap.
func gitConsoleDragHeight(total, splitH int, frac float64, minConsole, minUpper int) int {
	if frac <= 0 {
		frac = 0.01
	}
	return gitWorkFilesWidth(total, splitH, frac, minConsole, minUpper)
}

// gitConsoleFracFromY is the console share for a pointer at y.
// Moving up grows the console. The share stays inside the min and max.
func gitConsoleFracFromY(rest image.Rectangle, y, u int) float64 {
	splitH := gitWorkSplitWidth(u)
	consoleH := rest.Max.Y - y - splitH/2
	return gitConsoleFrac(rest.Dy(), splitH, consoleH, gitConsoleMinHeight(u), gitHistMinPane(u))
}

// gitConsoleFrac clamps a console height to the share beside the boundary.
func gitConsoleFrac(total, splitH, consoleH, minConsole, minUpper int) float64 {
	avail := total - splitH
	if avail <= 0 {
		return 0.01
	}
	frac := float64(consoleH) / float64(avail)
	minFrac := float64(minConsole) / float64(avail)
	maxFrac := float64(avail-minUpper) / float64(avail)
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
	return frac
}

// consoleBottomOffset is the panel offset that shows the end of the transcript.
func consoleBottomOffset(viewH, contentH int) float64 {
	if contentH <= viewH {
		return 0
	}
	return float64(viewH - contentH)
}

// consoleAtBottom reports that offsetY is already showing the end of the transcript.
func consoleAtBottom(offsetY float64, viewH, contentH int) bool {
	return offsetY <= consoleBottomOffset(viewH, contentH)+4
}

// gitConsoleToggle is the icon at the end of the status line.
type gitConsoleToggle struct {
	guigui.DefaultWidget

	button  basicwidget.Button
	tip     basicwidget.TooltipArea
	tipBody gitConsoleTipBody
	bounds  image.Rectangle
}

func (t *gitConsoleToggle) set(context *guigui.Context, lang i18n.Lang, open bool, onDown func(*guigui.Context)) {
	t.button.SetIcon(gitConsoleIcon(context.ColorMode(), basicwidget.LineHeight(context)))
	t.button.SetText("")
	t.button.OnDown(onDown)
	key := i18n.GitConsoleShow
	if open {
		key = i18n.GitConsoleHide
	}
	t.tipBody.value = i18n.T(lang, key)
	t.tip.SetContent(&t.tipBody)
}

func (t *gitConsoleToggle) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetEnabled(&t.button, true)
	adder.AddWidget(&t.button)
	adder.AddWidget(&t.tip)
	return nil
}

func (t *gitConsoleToggle) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	t.bounds = bounds
	// The balloon is laid out later, and only while it is open. A closed
	// balloon must not keep last frame's rectangle over a commit row.
	t.tipBody.shown = false
	layouter.LayoutWidget(&t.button, bounds)
	layouter.LayoutWidget(&t.tip, bounds)
}

// blocksCommit reports that cursor is on the console icon or its balloon.
// The history hit test uses row rectangles, so a control drawn in front of a
// commit would otherwise open that commit too.
func (t *gitConsoleToggle) blocksCommit(cursor image.Point) bool {
	return consoleControlAt(cursor, t.bounds, t.tipBody.bounds, t.tipBody.shown)
}

// consoleControlAt reports a click on the console icon or its open balloon.
// tipShown is false once the balloon closes, even if tip still holds the
// rectangle from the last time it was open.
func consoleControlAt(cursor image.Point, button, tip image.Rectangle, tipShown bool) bool {
	if !button.Empty() && cursor.In(button) {
		return true
	}
	return tipShown && !tip.Empty() && cursor.In(tip)
}

// consoleClickKeepsCommit is a left click on the console icon or its balloon.
// The open commit stays as it is. handled means the icon already toggled the
// console, so the click is not applied a second time.
func consoleClickKeepsCommit(openHash string, consoleOpen, handled bool) (string, bool) {
	if !handled {
		consoleOpen = !consoleOpen
	}
	return openHash, consoleOpen
}

// gitConsoleTipBody is the balloon text. Its bounds are the balloon itself.
// The balloon does not take clicks, so the history click test has to consult
// this rectangle.
type gitConsoleTipBody struct {
	guigui.DefaultWidget

	text  basicwidget.Text
	value string

	bounds  image.Rectangle
	shown   bool
	clicked bool
}

func (b *gitConsoleTipBody) syncText() {
	b.text.SetMultiline(true)
	b.text.SetValue(b.value)
}

func (b *gitConsoleTipBody) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	b.clicked = true
	return guigui.AbortHandlingInputByWidget(b)
}

func (b *gitConsoleTipBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	b.syncText()
	adder.AddWidget(&b.text)
	return nil
}

func (b *gitConsoleTipBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b.shown = true
	b.bounds = widgetBounds.Bounds()
	b.syncText()
	layouter.LayoutWidget(&b.text, insetPadding(b.bounds, basicwidget.TooltipTextPadding(context)))
}

func (b *gitConsoleTipBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	b.syncText()
	size := b.text.Measure(context, constraints)
	pad := basicwidget.TooltipTextPadding(context)
	return image.Pt(size.X+pad.Start+pad.End, size.Y+pad.Top+pad.Bottom)
}

func insetPadding(r image.Rectangle, pad guigui.Padding) image.Rectangle {
	r.Min.X += pad.Start
	r.Min.Y += pad.Top
	r.Max.X -= pad.End
	r.Max.Y -= pad.Bottom
	if r.Max.X < r.Min.X {
		r.Max.X = r.Min.X
	}
	if r.Max.Y < r.Min.Y {
		r.Max.Y = r.Min.Y
	}
	return r
}

// gitConsole is the transcript under the status line.
type gitConsole struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	body  gitConsoleBody

	value      string
	tab        int
	follow     bool
	pending    bool
	jumpFollow bool
	jumpY      float64
	offsetX    float64
	viewH      int
	contentH   int
}

func (c *gitConsole) prepare(text string, tab int, follow bool, offset float64) {
	if c.tab == tab && c.value == text {
		return
	}
	c.tab = tab
	c.value = text
	c.pending = true
	c.jumpFollow = follow
	c.jumpY = offset
}

func (c *gitConsole) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	v, ok := context.Env(c, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model).Git()
	text, spans := model.consoleTranscript()
	c.follow = model.ConsoleFollowing()
	c.prepare(text, model.ActiveTab(), c.follow, model.ConsoleOffset())
	c.body.value = c.value
	c.body.spans = spans
	c.body.pad = basicwidget.UnitSize(context) / 4
	c.panel.SetContent(&c.body)
	c.panel.SetAutoBorder(false)
	c.panel.SetBorders(basicwidget.PanelBorders{})
	c.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleSecondary)
	c.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	c.panel.OnScroll(func(ctx *guigui.Context, x, y float64) {
		c.offsetX = x
		env, ok := ctx.Env(c, EnvKeyModel)
		if !ok {
			return
		}
		env.(*Model).Git().NoteConsoleScroll(y, c.viewH, c.contentH)
	})
	adder.AddWidget(&c.panel)
	return nil
}

func (c *gitConsole) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	viewH := bounds.Dy()
	layouter.LayoutWidget(&c.panel, bounds)
	width := bounds.Dx()
	if width < 1 {
		width = 1
	}
	contentH := c.body.Measure(context, guigui.FixedWidthConstraints(width)).Y
	viewChanged := viewH != c.viewH
	contentChanged := contentH != c.contentH
	c.viewH = viewH
	c.contentH = contentH
	if !c.pending && !(c.follow && (viewChanged || contentChanged)) {
		return
	}
	y := c.jumpY
	if !c.pending || c.jumpFollow {
		y = consoleBottomOffset(c.viewH, c.contentH)
	}
	c.pending = false
	c.panel.ForceSetScrollOffset(c.offsetX, y)
}

type gitConsoleBody struct {
	guigui.DefaultWidget

	text  basicwidget.Text
	value string
	spans []consoleSpan
	pad   int
}

func (b *gitConsoleBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	b.text.SetValue(b.value)
	b.text.SetMultiline(true)
	b.text.SetSelectable(true)
	b.text.SetWrapMode(basicwidget.WrapModeNormal)
	b.text.SetVerticalAlign(basicwidget.VerticalAlignTop)
	var style basicwidget.TextStyle
	useGitMono(&style)
	b.text.SetBaseStyle(&style)
	styles := consoleTextStyles(b.spans, context.ColorMode() == ebiten.ColorModeDark)
	b.text.SetOverrideStyles(&styles, false)
	adder.AddWidget(&b.text)
	return nil
}

func (b *gitConsoleBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	pad := b.pad
	inner := image.Rect(bounds.Min.X+pad, bounds.Min.Y+pad, bounds.Max.X-pad, bounds.Max.Y-pad)
	if inner.Max.X < inner.Min.X {
		inner.Max.X = inner.Min.X
	}
	if inner.Max.Y < inner.Min.Y {
		inner.Max.Y = inner.Min.Y
	}
	layouter.LayoutWidget(&b.text, inner)
}

func (b *gitConsoleBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	pad := b.pad
	if pad < 0 {
		pad = 0
	}
	if w, ok := constraints.FixedWidth(); ok {
		inner := w - pad*2
		if inner < 1 {
			inner = 1
		}
		s := b.text.Measure(context, guigui.FixedWidthConstraints(inner))
		return image.Pt(w, s.Y+pad*2)
	}
	s := b.text.Measure(context, constraints)
	return image.Pt(s.X+pad*2, s.Y+pad*2)
}

type gitConsoleIconKey struct {
	mode ebiten.ColorMode
	size int
}

var gitConsoleIcons = map[gitConsoleIconKey]*ebiten.Image{}

// gitConsoleIcon draws a terminal prompt, the open and close control.
func gitConsoleIcon(mode ebiten.ColorMode, size int) *ebiten.Image {
	if size < 16 {
		size = 16
	}
	key := gitConsoleIconKey{mode: mode, size: size}
	if img, ok := gitConsoleIcons[key]; ok {
		return img
	}
	raster := size * 2
	img := ebiten.NewImage(raster, raster)
	sw := float32(raster) / 12
	if sw < 2 {
		sw = 2
	}
	s := float32(raster)
	left := s * 0.14
	top := s * 0.22
	w := s * 0.72
	h := s * 0.56
	col := textColor(mode)
	vector.StrokeRect(img, left, top, w, h, sw, col, true)
	vector.StrokeLine(img, s*0.30, s*0.40, s*0.42, s*0.50, sw, col, true)
	vector.StrokeLine(img, s*0.42, s*0.50, s*0.30, s*0.60, sw, col, true)
	vector.StrokeLine(img, s*0.48, s*0.62, s*0.70, s*0.62, sw, col, true)
	gitConsoleIcons[key] = img
	return img
}

// gitConsoleSplit is the draggable horizontal boundary on the console's top edge.
type gitConsoleSplit struct {
	guigui.DefaultWidget

	tool *GitTool
}

func (s *gitConsoleSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil {
		return guigui.HandleInputResult{}
	}
	hit := widgetBounds.IsHitAtCursor()
	if s.tool.consoleDragging || hit && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if result := s.tool.dragConsoleSplit(context); result.IsHandled() {
			return result
		}
	}
	if hit && (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)) {
		return guigui.AbortHandlingInputByWidget(s)
	}
	return guigui.HandleInputResult{}
}

func (s *gitConsoleSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool == nil {
		return 0, false
	}
	if s.tool.sideDragging || s.tool.graphDragging || s.tool.metaDrag != 0 || s.tool.histDragging {
		return 0, false
	}
	if s.tool.consoleDragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func (s *gitConsoleSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	y := b.Min.Y + b.Dy()/2
	fillRect(dst, image.Rect(b.Min.X, y, b.Max.X, y+1), gitHistSplitColor(context.ColorMode()))
}

func (t *GitTool) setConsoleFracFromCursor(y, u int) {
	if t.consoleArea.Dy() <= 0 {
		return
	}
	t.consoleFrac = gitConsoleFracFromY(t.consoleArea, y, u)
}

func (t *GitTool) dragConsoleSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !t.consoleDragging {
		if t.sideDragging || t.graphDragging || t.metaDrag != 0 || t.histDragging {
			return guigui.HandleInputResult{}
		}
		if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.consoleDragging = true
	}
	t.setConsoleFracFromCursor(image.Pt(ebiten.CursorPosition()).Y, u)
	t.setConsoleDragPassthrough(context, true)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.consoleDragging = false
		t.setConsoleDragPassthrough(context, false)
	}
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(t)
}

func (t *GitTool) setConsoleDragPassthrough(context *guigui.Context, pass bool) {
	for _, w := range []guigui.Widget{
		&t.tabBar,
		&t.sidePane,
		&t.sideSplit,
		&t.fetchBtn,
		&t.pullBtn,
		&t.pushBtn,
		&t.headBtn,
		&t.histSearch,
		&t.status,
		&t.consoleToggle,
		&t.console,
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
		&t.pushDialog,
		&t.selMenu,
		&t.changeMenu,
		&t.confirm,
		&t.rename,
	} {
		context.SetPassthrough(w, pass)
	}
}

func (t *GitTool) cursorOnConsoleSplit() bool {
	c := image.Pt(ebiten.CursorPosition())
	return !t.consoleSplitBounds.Empty() && c.In(t.consoleSplitBounds)
}
