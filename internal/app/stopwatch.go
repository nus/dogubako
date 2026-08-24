package app

import (
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// StopwatchTool is a start / pause / reset timer. Space toggles running.
type StopwatchTool struct {
	guigui.DefaultWidget

	display basicwidget.Text
	status  basicwidget.Text
	start   basicwidget.Button
	reset   basicwidget.Button
	hint    basicwidget.Text

	buttonItems []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
	buttonRow   guigui.LinearLayout
}

func (t *StopwatchTool) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return
	}
	m := v.(*Model).Stopwatch()
	w.WriteUint64(m.Generation())
	w.WriteBool(m.Running())
	w.WriteInt64(m.DisplayTicks())
}

func (t *StopwatchTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.display)
	adder.AddWidget(&t.status)
	adder.AddWidget(&t.start)
	adder.AddWidget(&t.reset)
	adder.AddWidget(&t.hint)

	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.Stopwatch()
	lang := appModel.Lang()

	var style basicwidget.TextStyle
	style.SetBold(true)
	style.SetTabular(true)
	t.display.SetBaseStyle(&style)
	t.display.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.display.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.display.SetScale(4)
	t.display.SetValue(model.Display())

	statusKey := i18n.StopwatchStopped
	if model.Running() {
		statusKey = i18n.StopwatchRunning
	} else if model.CanReset() {
		statusKey = i18n.StopwatchPaused
	}
	t.status.SetValue(i18n.T(lang, statusKey))
	t.status.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if model.Running() {
		t.start.SetText(i18n.T(lang, i18n.StopwatchPause))
	} else {
		t.start.SetText(i18n.T(lang, i18n.StopwatchStart))
	}
	t.start.SetType(basicwidget.ButtonTypePrimary)
	t.start.OnDown(func(context *guigui.Context) {
		model.Toggle()
	})

	t.reset.SetText(i18n.T(lang, i18n.StopwatchReset))
	t.reset.OnDown(func(context *guigui.Context) {
		model.Reset()
	})
	context.SetEnabled(&t.reset, model.CanReset())

	t.hint.SetValue(i18n.T(lang, i18n.StopwatchHint))
	t.hint.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *StopwatchTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.buttonItems = slices.Delete(t.buttonItems, 0, len(t.buttonItems))
	t.buttonItems = append(t.buttonItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.start},
		guigui.LinearLayoutItem{Widget: &t.reset},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
	)
	t.buttonRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.buttonItems,
		Gap:       u / 2,
	}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Widget: &t.display, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.buttonRow},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     t.layoutItems,
		Gap:       u / 2,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
