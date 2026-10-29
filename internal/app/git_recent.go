package app

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// buildEmptyHome is the new-tab view: recent repositories, a text filter, and
// the open buttons, centered in the area under the tab bar.
func (t *GitTool) buildEmptyHome(context *guigui.Context, adder *guigui.ChildAdder, lang i18n.Lang, model *GitModel) error {
	busy := model.Busy()
	t.openBtn.SetText(i18n.T(lang, i18n.GitOpen))
	t.openBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventGitOpen)
	})
	context.SetEnabled(&t.openBtn, !busy)
	adder.AddWidget(&t.openBtn)

	t.sshBtn.SetText(i18n.T(lang, i18n.GitSSH))
	t.sshBtn.OnDown(func(context *guigui.Context) {
		model.BeginSSH()
	})
	context.SetEnabled(&t.sshBtn, !busy)
	adder.AddWidget(&t.sshBtn)

	paths := model.RecentPaths()
	t.showRecent = len(paths) > 0
	if t.showRecent {
		setBoldText(&t.recentTitle, true)
		t.recentTitle.SetValue(i18n.T(lang, i18n.GitRecent))
		t.recentTitle.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.recentTitle.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		adder.AddWidget(&t.recentTitle)
		t.recentFilter.owner = t
		adder.AddWidget(&t.recentFilter)

		shown := gitRecentVisible(paths, t.recentQuery)
		t.recentRows.SetLen(len(paths))
		t.recentItems = t.recentItems[:0]
		j := 0
		for i, p := range paths {
			row := t.recentRows.At(i)
			row.Set(p)
			if j < len(shown) && shown[j] == p {
				t.recentItems = append(t.recentItems, basicwidget.ListItem[string]{
					Content: row,
					Value:   p,
				})
				j++
			}
		}
		t.recentList.SetStyle(basicwidget.ListStyleNormal)
		t.recentList.SetHighlightVisibleWhenUnfocused(true)
		t.recentList.SetHoverBackgroundVisible(true)
		t.recentList.SetItems(t.recentItems)
		if len(shown) > 0 {
			context.SetEnabled(&t.recentList, !busy)
			adder.AddWidget(&t.recentList)
		}
	}

	adder.AddWidget(&t.branchMenu)
	adder.AddWidget(&t.commitMenu)
	adder.AddWidget(&t.tagMenu)
	adder.AddWidget(&t.pushDialog)
	adder.AddWidget(&t.confirm)
	adder.AddWidget(&t.rename)
	t.noteRecentFilter(t.showRecent)
	return nil
}

func (t *GitTool) layoutEmptyHome(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter, u int) {
	pad := u / 2
	maxW := bounds.Dx() - 2*pad
	if maxW < u {
		maxW = bounds.Dx()
	}
	btn := t.openBtn.Measure(context, guigui.Constraints{})
	ssh := t.sshBtn.Measure(context, guigui.Constraints{})
	gap := u / 2

	var titleH, filterH, listH, listW int
	if t.showRecent {
		listW = btn.X
		minW := 16 * u
		if minW > maxW {
			minW = maxW
		}
		if listW < minW {
			listW = minW
		}
		for i := 0; i < t.recentRows.Len(); i++ {
			s := t.recentRows.At(i).Measure(context, guigui.Constraints{})
			if s.X > listW {
				listW = s.X
			}
		}
		if listW > maxW {
			listW = maxW
		}
		titleH = t.recentTitle.Measure(context, guigui.FixedWidthConstraints(listW)).Y
		if titleH < u*3/4 {
			titleH = u * 3 / 4
		}
		filterH = t.recentFilter.Measure(context, guigui.FixedWidthConstraints(listW)).Y
		if len(t.recentItems) > 0 {
			listH = t.recentList.Measure(context, guigui.FixedWidthConstraints(listW)).Y
		}
	}

	total := btn.Y + gap + ssh.Y
	room := bounds.Dy() - pad
	if room < u {
		room = bounds.Dy()
	}
	if t.showRecent {
		overhead := titleH + gap + filterH + gap + gap + btn.Y + gap + ssh.Y
		maxList := room - overhead
		if maxList < 2*u {
			maxList = 2 * u
		}
		if listH > maxList {
			listH = maxList
		}
		if listH == 0 {
			total = titleH + gap + filterH + gap + btn.Y + gap + ssh.Y
		} else {
			total = overhead + listH
		}
	}
	y := bounds.Min.Y + (bounds.Dy()-total)/2
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}

	if t.showRecent {
		x := bounds.Min.X + (bounds.Dx()-listW)/2
		layouter.LayoutWidget(&t.recentTitle, image.Rect(x, y, x+listW, y+titleH))
		y += titleH + gap
		layouter.LayoutWidget(&t.recentFilter, image.Rect(x, y, x+listW, y+filterH))
		y += filterH + gap
		if listH > 0 {
			layouter.LayoutWidget(&t.recentList, image.Rect(x, y, x+listW, y+listH))
			y += listH + gap
		}
	}
	bx := bounds.Min.X + (bounds.Dx()-btn.X)/2
	if bx < bounds.Min.X {
		bx = bounds.Min.X
	}
	layouter.LayoutWidget(&t.openBtn, image.Rect(bx, y, bx+btn.X, y+btn.Y))
	y += btn.Y + gap
	sx := bounds.Min.X + (bounds.Dx()-ssh.X)/2
	if sx < bounds.Min.X {
		sx = bounds.Min.X
	}
	layouter.LayoutWidget(&t.sshBtn, image.Rect(sx, y, sx+ssh.X, y+ssh.Y))
}

func (t *GitTool) openRecentAtCursor(model *GitModel) {
	if !t.showRecent || len(t.recentItems) == 0 || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	c := image.Pt(ebiten.CursorPosition())
	for i := 0; i < t.recentList.ItemCount(); i++ {
		if !c.In(t.recentList.ItemBounds(i)) {
			continue
		}
		item, ok := t.recentList.ItemByIndex(i)
		if !ok || item.Value == "" {
			return
		}
		if row, ok := item.Content.(*gitRecentRow); ok && c.In(row.closeBox) {
			model.forgetRecent(item.Value)
			return
		}
		model.Open(item.Value)
		return
	}
}

// noteRecentSession drops the recent-list filter when the open tab changes.
// The text is not stored, so a later visit starts empty.
func (t *GitTool) noteRecentSession(session *gitSession) {
	if t.recentQuerySession != nil && session != t.recentQuerySession {
		t.clearRecentQuery()
	}
	t.recentQuerySession = session
}

func (t *GitTool) clearRecentQuery() {
	t.recentQuery = ""
	t.recentFilter.input.ForceSetValue("")
}

// noteRecentFilter requests focus when the new-tab filter appears.
// Staying on that screen, including a rebuild, does not move focus again.
func (t *GitTool) noteRecentFilter(shown bool) {
	if shown && !t.recentFilterOn {
		t.focusRecentFilter = true
	}
	t.recentFilterOn = shown
}

func (t *GitTool) focusRecentFilterIfNeeded(context *guigui.Context) {
	if !t.focusRecentFilter {
		return
	}
	t.focusRecentFilter = false
	if !t.emptyHome || t.sshPick || !t.showRecent {
		return
	}
	context.SetFocused(&t.recentFilter.input, true)
}

// gitRecentVisible keeps paths whose folder name or displayed path contains
// query. Matching ignores case and surrounding space. Blank text returns paths
// in the same order. paths is left unchanged.
func gitRecentVisible(paths []string, query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return paths
	}
	q := strings.ToLower(query)
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if gitRecentMatch(p, q) {
			out = append(out, p)
		}
	}
	return out
}

func gitRecentMatch(repoPath, queryLower string) bool {
	if strings.Contains(strings.ToLower(repoFolder(repoPath)), queryLower) {
		return true
	}
	return strings.Contains(strings.ToLower(gitcli.ParseLoc(repoPath).Display()), queryLower)
}

// gitRecentFilter is the text field under the recent-repositories heading.
// It matches the sidebar filter: the same placeholder, and a clear mark on
// the trailing edge while the field has text.
type gitRecentFilter struct {
	guigui.DefaultWidget

	owner *GitTool
	input basicwidget.TextInput
	clear gitFilterClear
}

func (f *gitRecentFilter) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(f, true)
	adder.AddWidget(&f.input)
	var lang i18n.Lang
	if v, ok := context.Env(f, EnvKeyModel); ok {
		lang = v.(*Model).Lang()
	}
	f.input.SetPlaceholder(i18n.T(lang, i18n.GitFilter))
	f.input.OnValueChanged(func(_ *guigui.Context, text string, _ bool) {
		if f.owner == nil || f.owner.recentQuery == text {
			return
		}
		f.owner.recentQuery = text
		guigui.RequestRebuild()
	})
	f.clear.clear = f.clearText
	if f.input.HasValue() {
		adder.AddWidget(&f.clear)
	}
	return nil
}

func (f *gitRecentFilter) clearText() {
	f.input.ForceSetValue("")
	if f.owner != nil {
		f.owner.recentQuery = ""
	}
	guigui.RequestRebuild()
}

func (f *gitRecentFilter) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	field := widgetBounds.Bounds()
	layouter.LayoutWidget(&f.input, field)
	layouter.LayoutWidget(&f.clear, gitFilterClearBounds(field, basicwidget.UnitSize(context)))
}

func (f *gitRecentFilter) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w, ok := constraints.FixedWidth()
	if !ok || w < 1 {
		w = 16 * u
	}
	h := f.input.Measure(context, guigui.FixedWidthConstraints(w)).Y
	if h < 1 {
		h = u
	}
	return image.Pt(w, h)
}

type gitRecentRow struct {
	guigui.DefaultWidget

	name  basicwidget.Text
	path  basicwidget.Text
	close basicwidget.Text

	closeBox   image.Rectangle
	closeHover bool
}

func (r *gitRecentRow) Set(repoPath string) {
	loc := gitcli.ParseLoc(repoPath)
	setBoldText(&r.name, true)
	r.name.SetValue(repoFolder(repoPath))
	r.name.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.name.SetWrapMode(basicwidget.WrapModeNone)
	r.name.SetEllipsisString("…")
	r.name.SetSelectable(false)
	r.path.SetValue(loc.Display())
	r.path.SetOpacity(0.62)
	r.path.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.path.SetWrapMode(basicwidget.WrapModeNone)
	r.path.SetEllipsisString("…")
	r.path.SetSelectable(false)
	r.close.SetValue("×")
	r.close.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	r.close.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.close.SetScale(0.85)
	r.close.SetOpacity(0.7)
	r.close.SetSelectable(false)
}

func (r *gitRecentRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.name)
	adder.AddWidget(&r.path)
	adder.AddWidget(&r.close)
	return nil
}

func (r *gitRecentRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	inset := u / 3
	close := recentCloseSize(u)
	pad := u / 4
	x := b.Max.X - pad - close
	cy := b.Min.Y + (b.Dy()-close)/2
	r.closeBox = image.Rect(x, cy, x+close, cy+close)
	layouter.LayoutWidget(&r.close, r.closeBox)

	text := b
	text.Min.X += inset
	text.Max.X = x - u/8
	if text.Max.X < text.Min.X {
		text.Max.X = text.Min.X
	}
	nameH := r.name.Measure(context, guigui.Constraints{}).Y
	pathH := r.path.Measure(context, guigui.Constraints{}).Y
	gap := u / 8
	block := nameH + gap + pathH
	y := text.Min.Y + (text.Dy()-block)/2
	if y < text.Min.Y {
		y = text.Min.Y
	}
	layouter.LayoutWidget(&r.name, image.Rect(text.Min.X, y, text.Max.X, y+nameH))
	y += nameH + gap
	layouter.LayoutWidget(&r.path, image.Rect(text.Min.X, y, text.Max.X, y+pathH))
}

func (r *gitRecentRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	name := r.name.Measure(context, guigui.Constraints{})
	path := r.path.Measure(context, guigui.Constraints{})
	w := name.X
	if path.X > w {
		w = path.X
	}
	w += u/3 + u/8 + recentCloseSize(u) + u/4
	h := name.Y + u/8 + path.Y + u/2
	if fixed, ok := constraints.FixedWidth(); ok {
		w = fixed
	}
	if fixed, ok := constraints.FixedHeight(); ok {
		h = fixed
	}
	return image.Pt(w, h)
}

func (r *gitRecentRow) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if !r.closeHover || r.closeBox.Empty() {
		return
	}
	cx := float32(r.closeBox.Min.X + r.closeBox.Dx()/2)
	cy := float32(r.closeBox.Min.Y + r.closeBox.Dy()/2)
	vector.FillCircle(dst, cx, cy, float32(r.closeBox.Dx())*0.46, gitTabCloseHoverColor(context.ColorMode()), true)
}

func (r *gitRecentRow) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	hover := widgetBounds.IsHitAtCursor()
	r.closeHover = hover && image.Pt(ebiten.CursorPosition()).In(r.closeBox)
	return nil
}

func (r *gitRecentRow) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(r.path.Value())
	w.WriteBool(r.closeHover)
}

func recentCloseSize(u int) int {
	close := u * 2 / 3
	if close < 16 {
		close = 16
	}
	return close
}
