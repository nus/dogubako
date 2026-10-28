package app

import (
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// AndroidPanel is the Android tool: one device list, then files or the screen.
type AndroidPanel struct {
	guigui.DefaultWidget

	deviceLabel basicwidget.Text
	deviceList  basicwidget.List[string]
	face        basicwidget.SegmentedControl[AndroidFace]

	files AndroidTool
	shot  AndroidShotTool

	deviceItems []basicwidget.ListItem[string]
	faceItems   []basicwidget.SegmentedControlItem[AndroidFace]
	layoutItems []guigui.LinearLayoutItem
}

func (p *AndroidPanel) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(p, EnvKeyModel)
	if !ok {
		return
	}
	m := v.(*Model)
	w.WriteString(string(m.AndroidFace()))
	w.WriteBool(p.shot.previewFill)
	w.WriteString(m.Android().Serial())
	w.WriteUint64(m.Android().Generation())
	w.WriteUint64(m.AndroidShot().Generation())
	w.WriteBool(m.Android().Busy())
	w.WriteBool(m.AndroidShot().Busy())
	w.WriteBool(m.AndroidShot().Live())
}

func (p *AndroidPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	p.files.embedded = true
	p.shot.embedded = true

	v, ok := context.Env(p, EnvKeyModel)
	if !ok {
		return nil
	}
	model := v.(*Model)
	lang := model.Lang()
	face := model.AndroidFace()
	filled := face == AndroidFaceScreen && p.shot.previewFill
	if filled {
		// Fill hides the device list, the pane switch, and the saved list.
		adder.AddWidget(&p.shot)
		return nil
	}

	adder.AddWidget(&p.deviceLabel)
	adder.AddWidget(&p.deviceList)
	adder.AddWidget(&p.face)
	if face == AndroidFaceScreen {
		adder.AddWidget(&p.shot)
	} else {
		adder.AddWidget(&p.files)
	}

	files := model.Android()
	shot := model.AndroidShot()
	busy := files.Busy() || shot.Busy()

	setBoldText(&p.deviceLabel, true)
	p.deviceLabel.SetValue(i18n.T(lang, i18n.AndroidDevices))

	p.deviceItems = slices.Delete(p.deviceItems, 0, len(p.deviceItems))
	for _, d := range files.Devices() {
		p.deviceItems = append(p.deviceItems, basicwidget.ListItem[string]{
			Text:  d.Label(),
			Value: d.Serial,
		})
	}
	p.deviceList.SetStyle(basicwidget.ListStyleNormal)
	p.deviceList.SetHighlightVisibleWhenUnfocused(true)
	p.deviceList.SetItems(p.deviceItems)
	if serial := files.Serial(); serial != "" {
		p.deviceList.SelectItemByValue(serial)
	}
	p.deviceList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := p.deviceList.ItemByIndex(index)
		if !ok || item.Value == "" {
			return
		}
		model.SelectAndroidDevice(item.Value)
	})
	context.SetEnabled(&p.deviceList, !busy)

	p.faceItems = slices.Delete(p.faceItems, 0, len(p.faceItems))
	p.faceItems = append(p.faceItems,
		basicwidget.SegmentedControlItem[AndroidFace]{Text: i18n.T(lang, i18n.AndroidScreen), Value: AndroidFaceScreen},
		basicwidget.SegmentedControlItem[AndroidFace]{Text: i18n.T(lang, i18n.AndroidFiles), Value: AndroidFaceFiles},
	)
	p.face.SetItems(p.faceItems)
	p.face.SelectItemByValue(face)
	p.face.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := p.face.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetAndroidFace(item.Value)
	})
	return nil
}

func (p *AndroidPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	face := AndroidFaceFiles
	if v, ok := context.Env(p, EnvKeyModel); ok {
		face = v.(*Model).AndroidFace()
	}
	bounds := widgetBounds.Bounds()
	if face == AndroidFaceScreen && p.shot.previewFill {
		layouter.LayoutWidget(&p.shot, bounds)
		return
	}

	u := basicwidget.UnitSize(context)
	body := guigui.Widget(&p.files)
	if face == AndroidFaceScreen {
		body = &p.shot
	}
	p.layoutItems = slices.Delete(p.layoutItems, 0, len(p.layoutItems))
	p.layoutItems = append(p.layoutItems,
		guigui.LinearLayoutItem{Widget: &p.deviceLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.deviceList, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Widget: &p.face, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: body, Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     p.layoutItems,
		Gap:       u / 4,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}).LayoutWidgets(context, bounds, layouter)
}
