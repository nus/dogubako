package app

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// buildEmptyHome is the new-tab view: recent repositories and the open button,
// centered in the area under the tab bar.
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

		t.recentRows.SetLen(len(paths))
		t.recentItems = t.recentItems[:0]
		for i, p := range paths {
			row := t.recentRows.At(i)
			row.Set(p)
			t.recentItems = append(t.recentItems, basicwidget.ListItem[string]{
				Content: row,
				Value:   p,
			})
		}
		t.recentList.SetStyle(basicwidget.ListStyleNormal)
		t.recentList.SetHighlightVisibleWhenUnfocused(true)
		t.recentList.SetHoverBackgroundVisible(true)
		t.recentList.SetItems(t.recentItems)
		context.SetEnabled(&t.recentList, !busy)
		adder.AddWidget(&t.recentList)
	}

	adder.AddWidget(&t.branchMenu)
	adder.AddWidget(&t.commitMenu)
	adder.AddWidget(&t.tagMenu)
	adder.AddWidget(&t.confirm)
	adder.AddWidget(&t.rename)
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

	var titleH, listH, listW int
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
		listH = t.recentList.Measure(context, guigui.FixedWidthConstraints(listW)).Y
	}

	total := btn.Y + gap + ssh.Y
	room := bounds.Dy() - pad
	if room < u {
		room = bounds.Dy()
	}
	if t.showRecent {
		overhead := titleH + gap + gap + btn.Y + gap + ssh.Y
		maxList := room - overhead
		if maxList < 2*u {
			maxList = 2 * u
		}
		if listH > maxList {
			listH = maxList
		}
		total = overhead + listH
	}
	y := bounds.Min.Y + (bounds.Dy()-total)/2
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}

	if t.showRecent {
		x := bounds.Min.X + (bounds.Dx()-listW)/2
		layouter.LayoutWidget(&t.recentTitle, image.Rect(x, y, x+listW, y+titleH))
		y += titleH + gap
		layouter.LayoutWidget(&t.recentList, image.Rect(x, y, x+listW, y+listH))
		y += listH + gap
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
	if !t.showRecent || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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
		if i < t.recentRows.Len() && c.In(t.recentRows.At(i).closeBox) {
			model.forgetRecent(item.Value)
			return
		}
		model.Open(item.Value)
		return
	}
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
