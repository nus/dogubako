package app

import (
	"image"
	"slices"

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

	collapsed   bool
	size        image.Point
	layoutItems []guigui.LinearLayoutItem
	headerItems []guigui.LinearLayoutItem
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
	if s.collapsed {
		return nil
	}

	adder.AddWidget(&s.title)
	adder.AddWidget(&s.list)
	adder.AddWidget(&s.lang)

	s.title.SetValue(i18n.T(lang, i18n.AppTitle))
	setBoldText(&s.title, true)
	s.title.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	items := make([]basicwidget.ListItem[ToolID], 0, len(Tools))
	for _, tool := range Tools {
		items = append(items, basicwidget.ListItem[ToolID]{
			Text:  tool.Title(lang),
			Value: tool.ID,
		})
	}
	s.list.SetStyle(basicwidget.ListStyleSidebar)
	s.list.SetItems(items)
	s.list.SelectItemByValue(model.Mode())
	s.list.SetItemHeight(basicwidget.UnitSize(context))
	s.list.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.list.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetMode(item.Value)
	})

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
