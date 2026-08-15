package app

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
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

	size        image.Point
	layoutItems []guigui.LinearLayoutItem
}

func (s *sidebarContent) setSize(size image.Point) {
	s.size = size
}

func (s *sidebarContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.title)
	adder.AddWidget(&s.list)

	s.title.SetValue("道具箱")
	setBoldText(&s.title, true)
	s.title.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	v, ok := context.Env(s, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model)

	items := make([]basicwidget.ListItem[ToolID], 0, len(Tools))
	for _, tool := range Tools {
		items = append(items, basicwidget.ListItem[ToolID]{
			Text:  tool.Title,
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
	return nil
}

func (s *sidebarContent) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	s.layoutItems = slices.Delete(s.layoutItems, 0, len(s.layoutItems))
	s.layoutItems = append(s.layoutItems,
		guigui.LinearLayoutItem{Widget: &s.title, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &s.list, Size: guigui.FlexibleSize(1)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     s.layoutItems,
		Padding:   guigui.Padding{Top: u / 4},
	}
}

func (s *sidebarContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (s *sidebarContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return s.size
}
