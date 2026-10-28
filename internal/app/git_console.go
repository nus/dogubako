package app

import (
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
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
// keeps its margin. Open, both columns end on the console's top edge.
func gitViewBottoms(rest image.Rectangle, u int, consoleOpen bool) (listBottom, rightBottom int, console image.Rectangle) {
	listBottom = rest.Max.Y
	rightBottom = rest.Max.Y - u/2
	if rightBottom < rest.Min.Y {
		rightBottom = rest.Min.Y
	}
	if !consoleOpen || rest.Dy() <= 0 {
		return listBottom, rightBottom, image.Rectangle{}
	}
	h := gitConsoleHeight(u)
	if h > rest.Dy() {
		h = rest.Dy()
	}
	if maxH := rest.Dy() * 2 / 3; maxH > 0 && h > maxH {
		h = maxH
	}
	if h < 1 {
		h = 1
	}
	top := rest.Max.Y - h
	return top, top, image.Rect(rest.Min.X, top, rest.Max.X, rest.Max.Y)
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

	button basicwidget.Button
	tip    basicwidget.TooltipArea
}

func (t *gitConsoleToggle) set(context *guigui.Context, lang i18n.Lang, open bool, onDown func(*guigui.Context)) {
	t.button.SetIcon(gitConsoleIcon(context.ColorMode(), basicwidget.LineHeight(context)))
	t.button.SetText("")
	t.button.OnDown(onDown)
	key := i18n.GitConsoleShow
	if open {
		key = i18n.GitConsoleHide
	}
	t.tip.SetText(i18n.T(lang, key))
}

func (t *gitConsoleToggle) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetEnabled(&t.button, true)
	adder.AddWidget(&t.button)
	adder.AddWidget(&t.tip)
	return nil
}

func (t *gitConsoleToggle) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&t.button, bounds)
	layouter.LayoutWidget(&t.tip, bounds)
}

// gitConsole is the transcript under the status line and the hint.
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
	c.panel.SetBorders(basicwidget.PanelBorders{Top: true})
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
