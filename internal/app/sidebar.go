package app

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// Sidebar is the left tool-access menu.
type Sidebar struct {
	guigui.DefaultWidget

	panel        basicwidget.Panel
	panelContent sidebarContent
}

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

	title basicwidget.Text
	list  basicwidget.List[ToolID]
	lang  basicwidget.SegmentedControl[i18n.Lang]

	size        image.Point
	layoutItems []guigui.LinearLayoutItem
}

func (s *sidebarContent) setSize(size image.Point) {
	s.size = size
}

func (s *sidebarContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.title)
	adder.AddWidget(&s.list)
	adder.AddWidget(&s.lang)

	v, ok := context.Env(s, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model)
	lang := model.Lang()

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
	s.layoutItems = slices.Delete(s.layoutItems, 0, len(s.layoutItems))
	s.layoutItems = append(s.layoutItems,
		guigui.LinearLayoutItem{Widget: &s.title, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &s.list, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &s.lang, Size: guigui.FixedSize(u)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     s.layoutItems,
		Padding:   guigui.Padding{Top: u / 4, Bottom: u / 4, Start: u / 4, End: u / 4},
	}
}

func (s *sidebarContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (s *sidebarContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return s.size
}
