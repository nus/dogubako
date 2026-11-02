package app

import (
	"image"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

const (
	// Geometric triangles in the same block as the file-tree marks.
	// Small triangles (◂/▸, U+25C2/U+25B8) are missing from Inter and Hiragino.
	sidebarCollapseMark = "◀"
	sidebarExpandMark   = "▶"
)

// Sidebar is the left tool-access menu.
type Sidebar struct {
	guigui.DefaultWidget

	panel        basicwidget.Panel
	panelContent sidebarContent
}

// sidebarWidth is the sidebar's horizontal size.
// A collapsed sidebar keeps a one-unit toggle plus the content padding.
func sidebarWidth(u int, collapsed bool) int {
	if collapsed {
		return u + 2*sidebarPadding(u)
	}
	return 8 * u
}

func sidebarPadding(u int) int { return u / 4 }

func (s *Sidebar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.panel)
	s.panel.SetStyle(basicwidget.PanelStyleSide)
	s.panel.SetBorders(basicwidget.PanelBorders{End: true})
	s.panel.SetContent(&s.panelContent)
	return nil
}

func (s *Sidebar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.panelContent.setSize(widgetBounds.Bounds().Size())
	layouter.LayoutWidget(&s.panel, widgetBounds.Bounds())
}

type sidebarContent struct {
	guigui.DefaultWidget

	title  basicwidget.Text
	toggle sidebarToggle
	list   basicwidget.List[ToolID]
	lang   basicwidget.SegmentedControl[i18n.Lang]

	rows      guigui.WidgetSlice[*sidebarToolRow]
	listItems []basicwidget.ListItem[ToolID]

	collapsed   bool
	size        image.Point
	layoutItems []guigui.LinearLayoutItem
	headerItems []guigui.LinearLayoutItem
}

// sidebarItemPadding cancels the list's corner inset while the rail is
// collapsed. That inset would shrink each icon below the one-unit rail.
// The outer sidebar width stays sidebarWidth.
func sidebarItemPadding(collapsed bool, inset int) guigui.Padding {
	if !collapsed || inset <= 0 {
		return guigui.Padding{}
	}
	return guigui.Padding{Start: -inset, End: -inset}
}

// sidebarToggle is the fold / open button, with a tooltip over the same bounds.
type sidebarToggle struct {
	guigui.DefaultWidget

	button basicwidget.Button
	tip    basicwidget.TooltipArea
}

func (t *sidebarToggle) configure(text, tip string, onDown func(*guigui.Context)) {
	t.button.SetText(text)
	t.button.OnDown(onDown)
	t.tip.SetText(tip)
}

func (t *sidebarToggle) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.button)
	adder.AddWidget(&t.tip)
	return nil
}

func (t *sidebarToggle) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&t.button, bounds)
	layouter.LayoutWidget(&t.tip, bounds)
}

func (s *sidebarContent) setSize(size image.Point) {
	s.size = size
}

func (s *sidebarContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.toggle)

	v, ok := context.Env(s, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model)
	lang := model.Lang()
	s.collapsed = model.SidebarCollapsed()

	mark := sidebarCollapseMark
	tipKey := i18n.SidebarCollapse
	if s.collapsed {
		mark = sidebarExpandMark
		tipKey = i18n.SidebarExpand
	}
	s.toggle.configure(mark, i18n.T(lang, tipKey, ShortcutLabel(context, "B")), func(*guigui.Context) {
		model.ToggleSidebar()
	})

	pad := sidebarItemPadding(s.collapsed, basicwidget.RoundedCornerRadius(context))
	s.rows.SetLen(len(Tools))
	s.listItems = slices.Delete(s.listItems, 0, len(s.listItems))
	for i, tool := range Tools {
		row := s.rows.At(i)
		row.Set(tool.ID, tool.Title(lang), s.collapsed)
		s.listItems = append(s.listItems, basicwidget.ListItem[ToolID]{
			Content: row,
			Value:   tool.ID,
			Padding: pad,
		})
	}
	s.list.SetStyle(basicwidget.ListStyleSidebar)
	s.list.SetItems(s.listItems)
	s.list.SelectItemByValue(model.Mode())
	s.list.SetItemHeight(basicwidget.UnitSize(context))
	s.list.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.list.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetMode(item.Value)
	})
	if s.collapsed {
		adder.AddWidget(&s.list)
		return nil
	}

	adder.AddWidget(&s.title)
	adder.AddWidget(&s.list)
	adder.AddWidget(&s.lang)

	s.title.SetValue(i18n.T(lang, i18n.AppTitle))
	setBoldText(&s.title, true)
	s.title.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	s.lang.SetItems([]basicwidget.SegmentedControlItem[i18n.Lang]{
		{Text: "日本語", Value: i18n.JA},
		{Text: "English", Value: i18n.EN},
	})
	s.lang.SelectItemByValue(lang)
	s.lang.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.lang.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetLang(item.Value)
	})
	return nil
}

func (s *sidebarContent) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	pad := sidebarPadding(u)
	if v, ok := context.Env(s, EnvKeyModel); ok {
		s.collapsed = v.(*Model).SidebarCollapsed()
	}
	s.layoutItems = slices.Delete(s.layoutItems, 0, len(s.layoutItems))
	if s.collapsed {
		s.layoutItems = append(s.layoutItems,
			guigui.LinearLayoutItem{Widget: &s.toggle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &s.list, Size: guigui.FlexibleSize(1)},
		)
		return guigui.LinearLayout{
			Direction: guigui.LayoutDirectionVertical,
			Items:     s.layoutItems,
			Padding:   guigui.Padding{Top: pad, Bottom: pad, Start: pad, End: pad},
		}
	}
	s.headerItems = slices.Delete(s.headerItems, 0, len(s.headerItems))
	// The toggle stays at the start so it does not jump when the menu folds.
	// The matching spacer keeps the title centered.
	s.headerItems = append(s.headerItems,
		guigui.LinearLayoutItem{Widget: &s.toggle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &s.title, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u)},
	)
	s.layoutItems = append(s.layoutItems,
		guigui.LinearLayoutItem{
			Layout: guigui.LinearLayout{
				Direction: guigui.LayoutDirectionHorizontal,
				Items:     s.headerItems,
			},
			Size: guigui.FixedSize(u),
		},
		guigui.LinearLayoutItem{Widget: &s.list, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &s.lang, Size: guigui.FixedSize(u)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     s.layoutItems,
		Padding:   guigui.Padding{Top: pad, Bottom: pad, Start: pad, End: pad},
	}
}

func (s *sidebarContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (s *sidebarContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return s.size
}

// sidebarToolRow is one tool in the sidebar list. Expanded rows show an icon
// and the function name. Collapsed rows show the icon only, and hovering it
// shows that same name.
type sidebarToolRow struct {
	guigui.DefaultWidget

	label basicwidget.Text
	tip   sidebarTip

	id       ToolID
	title    string
	iconOnly bool
}

func (r *sidebarToolRow) Set(id ToolID, title string, iconOnly bool) {
	r.id = id
	r.title = title
	r.iconOnly = iconOnly
}

// tooltipText is the hover label. Expanded rows already show the name.
func (r *sidebarToolRow) tooltipText() string {
	if !r.iconOnly {
		return ""
	}
	return r.title
}

func (r *sidebarToolRow) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(string(r.id))
	w.WriteString(r.title)
	w.WriteBool(r.iconOnly)
}

func (r *sidebarToolRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if r.iconOnly {
		adder.AddWidget(&r.tip)
		r.tip.SetText(r.title)
		return nil
	}
	r.tip.Close()
	adder.AddWidget(&r.label)
	r.label.SetValue(r.title)
	r.label.SetHorizontalAlign(basicwidget.HorizontalAlignStart)
	r.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (r *sidebarToolRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	if r.iconOnly {
		layouter.LayoutWidget(&r.tip, bounds)
		return
	}
	fg, bold := sidebarItemInk(context, r)
	var style basicwidget.TextStyle
	style.SetBold(bold)
	if fg != nil {
		style.SetColor(fg)
	}
	r.label.SetBaseStyle(&style)
	side := bounds.Dy()
	if side > bounds.Dx() {
		side = bounds.Dx()
	}
	gap := basicwidget.UnitSize(context) / 6
	if gap < 2 {
		gap = 2
	}
	text := bounds
	text.Min.X += side + gap
	if text.Min.X > text.Max.X {
		text.Min.X = text.Max.X
	}
	layouter.LayoutWidget(&r.label, text)
}

func (r *sidebarToolRow) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	bounds := widgetBounds.Bounds()
	if bounds.Empty() {
		return
	}
	fg, _ := sidebarItemInk(context, r)
	box := bounds
	if !r.iconOnly {
		side := bounds.Dy()
		if side > bounds.Dx() {
			side = bounds.Dx()
		}
		box = image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+side, bounds.Min.Y+side)
	}
	drawToolGlyph(dst, box, r.id, fg)
}

func (r *sidebarToolRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	s := image.Pt(u, u)
	if w, ok := constraints.FixedWidth(); ok {
		s.X = w
	}
	if h, ok := constraints.FixedHeight(); ok {
		s.Y = h
	}
	return s
}

func sidebarItemInk(context *guigui.Context, w guigui.Widget) (color.Color, bool) {
	fg := color.Color(color.NRGBA{R: 0xc8, G: 0xc8, B: 0xc8, A: 0xff})
	bold := false
	v, ok := context.Env(w, basicwidget.EnvKeyListItemColorType)
	if !ok {
		return fg, bold
	}
	ct, ok := v.(basicwidget.ListItemColorType)
	if !ok {
		return fg, bold
	}
	if c := ct.TextColor(context); c != nil {
		fg = c
	}
	bold = ct == basicwidget.ListItemColorTypeHighlighted ||
		ct == basicwidget.ListItemColorTypeSelectedInUnfocusedList
	return fg, bold
}
