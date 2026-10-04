package app

import (
	"image"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/i18n"
)

// gitTabBarHeight is the tab strip height. Tabs sit on the bottom of the strip
// so the active tab meets the page underneath.
func gitTabBarHeight(u int) int {
	return u + u/3
}

// gitTabBar is the repository tab strip, drawn like a Chrome tab bar:
// a gray strip, an active tab that shares the page color, and a plain new-tab mark.
type gitTabBar struct {
	guigui.DefaultWidget

	tabs   guigui.WidgetSlice[*gitTab]
	add    gitTabAdd
	n      int
	items  []guigui.LinearLayoutItem
	widths []int
}

func (b *gitTabBar) Sync(lang i18n.Lang, model *GitModel) {
	n := model.TabCount()
	active := model.ActiveTab()
	b.n = n
	b.tabs.SetLen(n)
	for i := 0; i < n; i++ {
		label := model.TabLabel(i)
		if label == "" {
			label = i18n.T(lang, i18n.GitNewTab)
		}
		idx := i
		sep := i+1 < n && i != active && i+1 != active
		b.tabs.At(i).Set(label, i == active, sep, func() { model.SelectTab(idx) }, func() { model.CloseTab(idx) })
	}
	b.add.Set(func() { model.NewTab() })
}

func (b *gitTabBar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(b, true)
	for i := 0; i < b.n; i++ {
		adder.AddWidget(b.tabs.At(i))
	}
	adder.AddWidget(&b.add)
	return nil
}

func (b *gitTabBar) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	fillRect(dst, widgetBounds.Bounds(), gitTabStripColor(context.ColorMode()))
}

func (b *gitTabBar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	bounds := widgetBounds.Bounds()
	padTop := u / 5
	if padTop < 3 {
		padTop = 3
	}
	inner := bounds
	inner.Min.Y += padTop
	lead := u / 3
	gap := u / 6
	b.widths = b.tabWidths(context, inner.Dx()-lead-gap-u)
	b.items = slices.Delete(b.items, 0, len(b.items))
	if lead > 0 {
		b.items = append(b.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(lead)})
	}
	for i := 0; i < b.n; i++ {
		b.items = append(b.items, guigui.LinearLayoutItem{Widget: b.tabs.At(i), Size: guigui.FixedSize(b.widths[i])})
	}
	b.items = append(b.items,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(gap)},
		guigui.LinearLayoutItem{Widget: &b.add, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     b.items,
	}).LayoutWidgets(context, inner, layouter)
}

// tabWidths returns each tab's width, shrinking them together when the strip is narrow.
func (b *gitTabBar) tabWidths(context *guigui.Context, avail int) []int {
	widths := slices.Delete(b.widths, 0, len(b.widths))
	if b.n == 0 {
		return widths
	}
	total := 0
	for i := 0; i < b.n; i++ {
		w := b.tabs.At(i).Measure(context, guigui.Constraints{}).X
		widths = append(widths, w)
		total += w
	}
	if avail <= 0 || total <= avail {
		return widths
	}
	u := basicwidget.UnitSize(context)
	minW := 3 * u
	each := avail / b.n
	if each < minW {
		each = minW
	}
	for i := range widths {
		if widths[i] > each {
			widths[i] = each
		}
	}
	return widths
}

func (b *gitTabBar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Pt(0, gitTabBarHeight(basicwidget.UnitSize(context)))
}

// gitTab is one repository tab. The active tab is the page color with rounded
// top corners; inactive tabs are text on the strip.
type gitTab struct {
	guigui.DefaultWidget

	label basicwidget.Text
	close basicwidget.Text

	text     string
	active   bool
	sep      bool
	onSelect func()
	onClose  func()

	hover      bool
	closeHover bool
	closeBox   image.Rectangle
}

func (t *gitTab) Set(label string, active, sep bool, onSelect, onClose func()) {
	t.text = label
	t.active = active
	t.sep = sep
	t.onSelect = onSelect
	t.onClose = onClose
	t.label.SetValue(label)
	t.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.label.SetWrapMode(basicwidget.WrapModeNone)
	t.label.SetEllipsisString("…")
	t.label.SetSelectable(false)
	if active {
		t.label.SetOpacity(1)
	} else {
		t.label.SetOpacity(0.78)
	}
	t.close.SetValue("×")
	t.close.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.close.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.close.SetScale(0.85)
	t.close.SetSelectable(false)
	if active {
		t.close.SetOpacity(0.9)
	} else {
		t.close.SetOpacity(0.65)
	}
}

func (t *gitTab) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(t, true)
	adder.AddWidget(&t.label)
	adder.AddWidget(&t.close)
	return nil
}

func (t *gitTab) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	mode := context.ColorMode()
	if t.active {
		top := gitTabRadius(u, b)
		fillChromeTab(dst, b, basicwidgetdraw.BackgroundColor(mode), top, top*0.55)
	} else if t.hover {
		inset := image.Rect(b.Min.X+2, b.Min.Y+2, b.Max.X-2, b.Max.Y-u/6)
		if inset.Dx() > 4 && inset.Dy() > 4 {
			r := float32(u) / 4
			fillChromeTab(dst, inset, gitTabHoverColor(mode), r, r)
		}
	}
	if t.sep && !t.hover && !t.active {
		drawTabSeparator(dst, b, gitTabSeparatorColor(mode))
	}
	if t.closeHover && !t.closeBox.Empty() {
		cx := float32(t.closeBox.Min.X + t.closeBox.Dx()/2)
		cy := float32(t.closeBox.Min.Y + t.closeBox.Dy()/2)
		vector.FillCircle(dst, cx, cy, float32(t.closeBox.Dx())*0.46, gitTabCloseHoverColor(mode), true)
	}
}

func (t *gitTab) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	close := u * 2 / 3
	if close < 16 {
		close = 16
	}
	pad := u / 4
	x := b.Max.X - pad - close
	y := b.Min.Y + (b.Dy()-close)/2
	t.closeBox = image.Rect(x, y, x+close, y+close)
	layouter.LayoutWidget(&t.close, t.closeBox)
	text := b
	text.Min.X += u / 3
	text.Max.X = x - u/8
	if text.Max.X < text.Min.X {
		text.Max.X = text.Min.X
	}
	layouter.LayoutWidget(&t.label, text)
}

func (t *gitTab) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	text := t.label.Measure(context, guigui.Constraints{})
	close := u * 2 / 3
	if close < 16 {
		close = 16
	}
	w := text.X + u/3 + u/4 + close + u/8
	minW := 5 * u
	maxW := 10 * u
	if w < minW {
		w = minW
	}
	if w > maxW {
		w = maxW
	}
	if fixed, ok := constraints.FixedWidth(); ok {
		w = fixed
	}
	h := gitTabBarHeight(u)
	if fixed, ok := constraints.FixedHeight(); ok {
		h = fixed
	}
	return image.Pt(w, h)
}

func (t *gitTab) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	hover := widgetBounds.IsHitAtCursor()
	pt := image.Pt(ebiten.CursorPosition())
	closeHover := hover && pt.In(t.closeBox)
	t.hover = hover
	t.closeHover = closeHover
	return nil
}

func (t *gitTab) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(t.text)
	w.WriteBool(t.active)
	w.WriteBool(t.sep)
	w.WriteBool(t.hover)
	w.WriteBool(t.closeHover)
}

func (t *gitTab) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if image.Pt(ebiten.CursorPosition()).In(t.closeBox) {
		if t.onClose != nil {
			t.onClose()
		}
		return guigui.HandleInputByWidget(t)
	}
	if t.onSelect != nil {
		t.onSelect()
	}
	return guigui.HandleInputByWidget(t)
}

func (t *gitTab) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

// gitTabAdd is the plain "+" after the last tab.
type gitTabAdd struct {
	guigui.DefaultWidget

	onAdd func()
	hover bool
}

func (a *gitTabAdd) Set(onAdd func()) {
	a.onAdd = onAdd
}

func (a *gitTabAdd) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	cx := float32(b.Min.X + b.Dx()/2)
	cy := float32(b.Min.Y + b.Dy()/2)
	mode := context.ColorMode()
	if a.hover {
		r := float32(u) * 0.38
		if maxR := float32(b.Dy()) * 0.38; r > maxR {
			r = maxR
		}
		vector.FillCircle(dst, cx, cy, r, gitTabHoverColor(mode), true)
	}
	arm := float32(u) * 0.18
	clr := basicwidgetdraw.TextColor(mode, true)
	width := float32(u) / 14
	if width < 1.4 {
		width = 1.4
	}
	strokeLine(dst, cx-arm, cy, cx+arm, cy, width, clr)
	strokeLine(dst, cx, cy-arm, cx, cy+arm, width, clr)
}

func (a *gitTabAdd) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	a.hover = widgetBounds.IsHitAtCursor()
	return nil
}

func (a *gitTabAdd) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteBool(a.hover)
}

func (a *gitTabAdd) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if a.onAdd != nil {
		a.onAdd()
	}
	return guigui.HandleInputByWidget(a)
}

func (a *gitTabAdd) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

func (a *gitTabAdd) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(u, u)
}

func gitTabRadius(u int, b image.Rectangle) float32 {
	r := float32(u) / 3
	if r < 8 {
		r = 8
	}
	if h := float32(b.Dy()); r > h/2 {
		r = h / 2
	}
	if w := float32(b.Dx()); r > w/2 {
		r = w / 2
	}
	return r
}

func fillChromeTab(dst *ebiten.Image, b image.Rectangle, clr color.Color, topR, botR float32) {
	if b.Dx() <= 1 || b.Dy() <= 1 {
		return
	}
	w := float32(b.Dx())
	h := float32(b.Dy())
	if topR < 0 {
		topR = 0
	}
	if botR < 0 {
		botR = 0
	}
	if topR*2 > w {
		topR = w / 2
	}
	if botR*2 > w {
		botR = w / 2
	}
	if topR+botR > h && topR+botR > 0 {
		scale := h / (topR + botR)
		topR *= scale
		botR *= scale
	}
	x0 := float32(b.Min.X)
	y0 := float32(b.Min.Y)
	x1 := float32(b.Max.X)
	y1 := float32(b.Max.Y)
	var p vector.Path
	p.MoveTo(x0+botR, y1)
	p.LineTo(x1-botR, y1)
	if botR > 0 {
		p.ArcTo(x1, y1, x1, y1-botR, botR)
	}
	p.LineTo(x1, y0+topR)
	if topR > 0 {
		p.ArcTo(x1, y0, x1-topR, y0, topR)
	}
	p.LineTo(x0+topR, y0)
	if topR > 0 {
		p.ArcTo(x0, y0, x0, y0+topR, topR)
	}
	p.LineTo(x0, y1-botR)
	if botR > 0 {
		p.ArcTo(x0, y1, x0+botR, y1, botR)
	}
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, nil, op)
}

func fillRect(dst *ebiten.Image, r image.Rectangle, clr color.Color) {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	var p vector.Path
	p.MoveTo(float32(r.Min.X), float32(r.Min.Y))
	p.LineTo(float32(r.Max.X), float32(r.Min.Y))
	p.LineTo(float32(r.Max.X), float32(r.Max.Y))
	p.LineTo(float32(r.Min.X), float32(r.Max.Y))
	p.Close()
	op := &vector.DrawPathOptions{}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, nil, op)
}

func drawTabSeparator(dst *ebiten.Image, b image.Rectangle, clr color.Color) {
	h := b.Dy()
	seg := h / 2
	if seg < 8 {
		seg = h
	}
	y0 := b.Min.Y + (h-seg)/2
	fillRect(dst, image.Rect(b.Max.X-1, y0, b.Max.X, y0+seg), clr)
}

func strokeLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color) {
	var p vector.Path
	p.MoveTo(x0, y0)
	p.LineTo(x1, y1)
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, &p, &vector.StrokeOptions{Width: width, LineCap: vector.LineCapRound}, op)
}

func gitTabStripColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return basicwidgetdraw.BackgroundSecondaryColor(mode)
	}
	// Chrome's light tab strip, a step darker than the page.
	return color.NRGBA{0xDE, 0xE1, 0xE6, 0xFF}
}

func gitTabHoverColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{0xFF, 0xFF, 0xFF, 28}
	}
	return color.NRGBA{0x00, 0x00, 0x00, 24}
}

func gitTabCloseHoverColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{0xFF, 0xFF, 0xFF, 48}
	}
	return color.NRGBA{0x00, 0x00, 0x00, 40}
}

func gitTabSeparatorColor(mode ebiten.ColorMode) color.Color {
	if mode == ebiten.ColorModeDark {
		return color.NRGBA{0xFF, 0xFF, 0xFF, 48}
	}
	return color.NRGBA{0x80, 0x86, 0x8B, 0x88}
}
