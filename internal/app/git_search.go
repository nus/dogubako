package app

import (
	"fmt"
	"image"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitHistSearch is the history field between Fetch / Pull / Push and the graph.
// It searches loaded commits and is as wide as the graph.
// The field shows which match is current, and moves to the next or previous one.
type gitHistSearch struct {
	guigui.DefaultWidget

	owner *GitTool
	input basicwidget.TextInput
	count basicwidget.Text
	prev  gitHistStep
	next  gitHistStep
	clear gitFilterClear
	tools bool
}

func (f *gitHistSearch) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(f, true)
	adder.AddWidget(&f.input)
	var lang i18n.Lang
	var selected string
	if v, ok := context.Env(f, EnvKeyModel); ok {
		model := v.(*Model)
		lang = model.Lang()
		selected = model.Git().Selected()
	}
	f.input.SetPlaceholder(i18n.T(lang, i18n.GitSearch))
	f.input.OnValueChanged(func(_ *guigui.Context, text string, _ bool) {
		if f.owner == nil {
			return
		}
		f.owner.setHistQuery(text)
	})
	f.input.OnHandleButtonInput(f.onKey)
	f.clear.clear = f.clearText
	if f.input.HasValue() {
		adder.AddWidget(&f.clear)
	}
	f.tools = strings.TrimSpace(f.input.Value()) != ""
	if !f.tools {
		return nil
	}
	var matches []string
	if f.owner != nil {
		matches = f.owner.histMatches
	}
	f.count.SetValue(historyMatchLabel(matches, selected))
	f.count.SetScale(0.85)
	f.count.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	f.count.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	f.count.SetWrapMode(basicwidget.WrapModeNone)
	f.count.SetSelectable(false)
	context.SetPassthrough(&f.count, true)
	adder.AddWidget(&f.count)
	f.prev.search = f
	f.prev.next = false
	f.prev.glyph = "↑"
	f.next.search = f
	f.next.next = true
	f.next.glyph = "↓"
	adder.AddWidget(&f.prev)
	adder.AddWidget(&f.next)
	return nil
}

func (f *gitHistSearch) clearText() {
	f.input.ForceSetValue("")
	if f.owner != nil {
		f.owner.setHistQuery("")
	}
	guigui.RequestRebuild()
}

func (f *gitHistSearch) onKey(context *guigui.Context, _ *guigui.WidgetBounds) guigui.HandleInputResult {
	if shortcutModifierPressed(context) && inpututil.IsKeyJustPressed(ebiten.KeyF) && !shiftDown() && !ebiten.IsKeyPressed(ebiten.KeyAlt) {
		if f.owner != nil && f.owner.openHistSearch() {
			return guigui.HandleInputByWidget(f)
		}
		return guigui.HandleInputResult{}
	}
	if shortcutModifierPressed(context) || ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyControl) {
		return guigui.HandleInputResult{}
	}
	if !inpututil.IsKeyJustPressed(ebiten.KeyEnter) && !inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		return guigui.HandleInputResult{}
	}
	// Enter moves to the next match. Shift+Enter moves to the previous one.
	f.step(context, !shiftDown())
	return guigui.HandleInputByWidget(f)
}

func (f *gitHistSearch) step(context *guigui.Context, next bool) {
	if f.owner == nil {
		return
	}
	v, ok := context.Env(f, EnvKeyModel)
	if !ok {
		return
	}
	f.owner.stepHistMatch(v.(*Model).Git(), next)
}

func (f *gitHistSearch) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	field := widgetBounds.Bounds()
	layouter.LayoutWidget(&f.input, field)
	u := basicwidget.UnitSize(context)
	layouter.LayoutWidget(&f.clear, gitHistToolBounds(field, u, 0))
	if !f.tools {
		return
	}
	prev := gitHistToolBounds(field, u, 2)
	layouter.LayoutWidget(&f.next, gitHistToolBounds(field, u, 1))
	layouter.LayoutWidget(&f.prev, prev)
	gap := histToolGap(u)
	w := f.count.Measure(context, guigui.Constraints{}).X
	right := prev.Min.X - gap
	left := right - w
	if left < field.Min.X {
		left = field.Min.X
	}
	if right < left {
		right = left
	}
	layouter.LayoutWidget(&f.count, image.Rect(left, prev.Min.Y, right, prev.Max.Y))
}

func (f *gitHistSearch) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return f.input.Measure(context, constraints)
}

// gitHistStep moves to the previous or next search match.
type gitHistStep struct {
	guigui.DefaultWidget

	search *gitHistSearch
	mark   basicwidget.Text
	glyph  string
	next   bool
	hover  bool
}

func (s *gitHistStep) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.mark)
	context.SetPassthrough(&s.mark, true)
	s.mark.SetValue(s.glyph)
	s.mark.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.mark.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	s.mark.SetScale(0.85)
	s.mark.SetSelectable(false)
	return nil
}

func (s *gitHistStep) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&s.mark, widgetBounds.Bounds())
}

func (s *gitHistStep) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if !s.hover {
		return
	}
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	cx := float32(b.Min.X + b.Dx()/2)
	cy := float32(b.Min.Y + b.Dy()/2)
	vector.FillCircle(dst, cx, cy, float32(b.Dx())*0.5, gitTabCloseHoverColor(context.ColorMode()), true)
}

func (s *gitHistStep) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	s.hover = widgetBounds.IsHitAtCursor()
	return nil
}

func (s *gitHistStep) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteBool(s.hover)
}

func (s *gitHistStep) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if s.search != nil {
		s.search.step(context, s.next)
		context.SetFocused(&s.search.input, true)
	}
	return guigui.HandleInputByWidget(s)
}

func (s *gitHistStep) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

// gitHistToolBounds is one trailing control inside the search field.
// fromEnd 0 is the clear mark. Larger values sit further toward the text.
func gitHistToolBounds(field image.Rectangle, u, fromEnd int) image.Rectangle {
	box := gitFilterClearBounds(field, u)
	if fromEnd <= 0 {
		return box
	}
	box = box.Add(image.Pt(-fromEnd*(box.Dx()+histToolGap(u)), 0))
	if box.Min.X < field.Min.X {
		box = image.Rect(field.Min.X, box.Min.Y, field.Min.X+box.Dx(), box.Max.Y)
	}
	return box
}

func histToolGap(u int) int {
	gap := u / 8
	if gap < 1 {
		gap = 1
	}
	return gap
}

// historyMatchHashes lists loaded commits whose title, id, branch name, or tag
// name contains query. The order is the graph order, newest first.
// Blank text matches nothing. Uncommitted changes are never a match.
func historyMatchHashes(rows []gitcli.GraphRow, query string) []string {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	q = strings.ToLower(q)
	var out []string
	for _, row := range rows {
		c := row.Commit
		if c.Hash == gitcli.Uncommitted || !historyCommitMatches(c, q) {
			continue
		}
		out = append(out, c.Hash)
	}
	return out
}

// historyMatchLabel is the position inside the field.
// A selected match is k/N. Anything else, including no matches, is ?/N.
func historyMatchLabel(matches []string, selected string) string {
	n := len(matches)
	for i, hash := range matches {
		if hash == selected {
			return fmt.Sprintf("%d/%d", i+1, n)
		}
	}
	return fmt.Sprintf("?/%d", n)
}

// historyStepMatch returns the match to highlight.
// matches follow the graph, newest first, so next walks toward older commits.
// A selected match wraps. A selection that is not a match moves to the nearest
// match in that direction, or wraps when none is left. No selection starts at
// the first match, or the last when moving backward. No matches returns empty.
func historyStepMatch(matches []string, rows []gitcli.GraphRow, selected string, next bool) string {
	n := len(matches)
	if n == 0 {
		return ""
	}
	for i, hash := range matches {
		if hash != selected {
			continue
		}
		if next {
			return matches[(i+1)%n]
		}
		return matches[(i-1+n)%n]
	}
	selRow := -1
	if selected != "" {
		for i, row := range rows {
			if row.Commit.Hash == selected {
				selRow = i
				break
			}
		}
	}
	if selRow < 0 {
		if next {
			return matches[0]
		}
		return matches[n-1]
	}
	rowOf := make(map[string]int, len(rows))
	for i, row := range rows {
		rowOf[row.Commit.Hash] = i
	}
	if next {
		for _, hash := range matches {
			if rowOf[hash] > selRow {
				return hash
			}
		}
		return matches[0]
	}
	for i := n - 1; i >= 0; i-- {
		if rowOf[matches[i]] < selRow {
			return matches[i]
		}
	}
	return matches[n-1]
}

// stepHistMatch highlights the next or previous loaded match and asks the graph
// to scroll it into view. It does not open the commit.
func (t *GitTool) stepHistMatch(model *GitModel, next bool) {
	if model == nil || strings.TrimSpace(t.histQuery) == "" {
		return
	}
	snap := model.Snapshot()
	t.syncHistMatches(snap.Graph, model.GraphRev())
	hash := historyStepMatch(t.histMatches, snap.Graph, model.Selected(), next)
	if hash == "" {
		return
	}
	t.histJump = hash
	model.HighlightCommit(hash)
	guigui.RequestRebuild()
}

func historyCommitMatches(c gitcli.Commit, q string) bool {
	if historyContains(historyTitle(c.Subject), q) || historyContains(c.Hash, q) {
		return true
	}
	for _, term := range historyBadgeTerms(c.Decorations) {
		if historyContains(term, q) {
			return true
		}
	}
	return false
}

func historyTitle(subject string) string {
	if i := strings.IndexByte(subject, '\n'); i >= 0 {
		return subject[:i]
	}
	return subject
}

func historyContains(s, q string) bool {
	return strings.Contains(strings.ToLower(s), q)
}

// historyBadgeTerms are the branch and tag names drawn on a commit.
// A merged badge contributes the local name and each remote name.
// A remote-only badge contributes the tracking name, such as origin/main.
func historyBadgeTerms(ds []gitcli.Decoration) []string {
	labels := refDecorations(ds)
	if len(labels) == 0 {
		return nil
	}
	terms := make([]string, 0, len(labels))
	for _, d := range labels {
		local, remote, ok := splitMergedBranch(d.Name)
		if d.Kind == "head" && ok {
			terms = append(terms, local)
			terms = append(terms, strings.Fields(remote)...)
			continue
		}
		if d.Name != "" {
			terms = append(terms, d.Name)
		}
	}
	return terms
}
