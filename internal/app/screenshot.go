package app

import (
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/i18n"
)

var (
	eventScreenshotCapture    = guigui.GenerateEventKey()
	eventScreenshotSave       = guigui.GenerateEventKey()
	eventScreenshotSaveAs     = guigui.GenerateEventKey()
	eventScreenshotCopy       = guigui.GenerateEventKey()
	eventScreenshotSendImage  = guigui.GenerateEventKey()
	eventScreenshotShowFolder = guigui.GenerateEventKey()
)

// ScreenshotTool captures the screen, a window, or a region.
type ScreenshotTool struct {
	guigui.DefaultWidget

	modeText   basicwidget.Text
	mode       basicwidget.SegmentedControl[capture.Mode]
	delayText  basicwidget.Text
	delayInput guigui.WidgetWithSize[*basicwidget.NumberInput]
	hideText   basicwidget.Text
	hideToggle basicwidget.Toggle
	captureBtn basicwidget.Button

	previewLabel basicwidget.Text
	preview      destPreview
	previewEmpty basicwidget.Text

	destLabel basicwidget.Text
	saveBtn   basicwidget.Button
	saveAsBtn basicwidget.Button
	copyBtn   basicwidget.Button
	sendBtn   basicwidget.Button
	folderBtn basicwidget.Button
	status    basicwidget.Text

	toolbarItems []guigui.LinearLayoutItem
	outputItems  []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
	toolbar      guigui.LinearLayout
	outputRow    guigui.LinearLayout
}

func (t *ScreenshotTool) OnCapture(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotCapture, f)
}

func (t *ScreenshotTool) OnSave(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotSave, f)
}

func (t *ScreenshotTool) OnSaveAs(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotSaveAs, f)
}

func (t *ScreenshotTool) OnCopy(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotCopy, f)
}

func (t *ScreenshotTool) OnSendToImage(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotSendImage, f)
}

func (t *ScreenshotTool) OnShowFolder(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotShowFolder, f)
}

func (t *ScreenshotTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.modeText)
	adder.AddWidget(&t.mode)
	adder.AddWidget(&t.delayText)
	adder.AddWidget(&t.delayInput)
	adder.AddWidget(&t.hideText)
	adder.AddWidget(&t.hideToggle)
	adder.AddWidget(&t.captureBtn)
	adder.AddWidget(&t.previewLabel)
	adder.AddWidget(&t.destLabel)
	adder.AddWidget(&t.saveBtn)
	adder.AddWidget(&t.saveAsBtn)
	adder.AddWidget(&t.copyBtn)
	adder.AddWidget(&t.sendBtn)
	adder.AddWidget(&t.folderBtn)
	adder.AddWidget(&t.status)

	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.Screenshot()
	lang := appModel.Lang()
	u := basicwidget.UnitSize(context)
	has := model.HasImage()
	busy := model.Capturing()

	t.modeText.SetValue(i18n.T(lang, i18n.ScreenshotMode))
	t.modeText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.mode.SetItems([]basicwidget.SegmentedControlItem[capture.Mode]{
		{Text: i18n.T(lang, i18n.ScreenshotFull), Value: capture.ModeFull},
		{Text: i18n.T(lang, i18n.ScreenshotWindow), Value: capture.ModeWindow},
		{Text: i18n.T(lang, i18n.ScreenshotRegion), Value: capture.ModeRegion},
	})
	t.mode.SelectItemByValue(model.Mode())
	t.mode.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.mode.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetMode(item.Value)
	})
	context.SetEnabled(&t.mode, !busy)

	t.delayText.SetValue(i18n.T(lang, i18n.ScreenshotDelay))
	t.delayText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	configureIntInput(t.delayInput.Widget(), 0, maxCaptureDelaySec, 1, model.DelaySec(), func(v int, committed bool) {
		if committed {
			model.SetDelaySec(v)
		}
	})
	t.delayInput.SetFixedWidth(3 * u)
	context.SetEnabled(&t.delayInput, !busy)

	t.hideText.SetValue(i18n.T(lang, i18n.ScreenshotHideWindow))
	t.hideText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.hideToggle.SetValue(model.HideWindow())
	t.hideToggle.OnValueChanged(func(context *guigui.Context, value bool) {
		model.SetHideWindow(value)
	})
	context.SetEnabled(&t.hideToggle, !busy)

	t.captureBtn.SetText(i18n.T(lang, i18n.ScreenshotCapture))
	t.captureBtn.SetType(basicwidget.ButtonTypePrimary)
	t.captureBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotCapture)
	})
	context.SetEnabled(&t.captureBtn, !busy)

	setBoldText(&t.previewLabel, true)
	if has {
		sz := model.Size()
		t.previewLabel.SetValue(i18n.T(lang, i18n.ScreenshotPreviewSz, sz.X, sz.Y))
		t.preview.SetImage(model.Preview())
		adder.AddWidget(&t.preview)
	} else {
		t.previewLabel.SetValue(i18n.T(lang, i18n.ScreenshotPreview))
		t.preview.SetImage(nil)
		t.previewEmpty.SetMultiline(true)
		t.previewEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.previewEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.previewEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		t.previewEmpty.SetValue(i18n.T(lang, i18n.ScreenshotEmpty))
		adder.AddWidget(&t.previewEmpty)
	}

	dest := model.DestDir()
	if dest == "" {
		dest = "—"
	}
	t.destLabel.SetValue(i18n.T(lang, i18n.ScreenshotDest, dest))
	t.destLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	t.saveBtn.SetText(i18n.T(lang, i18n.ScreenshotSave))
	t.saveBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotSave)
	})
	context.SetEnabled(&t.saveBtn, has && !busy)

	t.saveAsBtn.SetText(i18n.T(lang, i18n.ScreenshotSaveAs))
	t.saveAsBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotSaveAs)
	})
	context.SetEnabled(&t.saveAsBtn, has && !busy)

	t.copyBtn.SetText(i18n.T(lang, i18n.ScreenshotCopy))
	t.copyBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotCopy)
	})
	context.SetEnabled(&t.copyBtn, has && !busy)

	t.sendBtn.SetText(i18n.T(lang, i18n.ScreenshotSendImage))
	t.sendBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotSendImage)
	})
	context.SetEnabled(&t.sendBtn, has && !busy)

	t.folderBtn.SetText(i18n.T(lang, i18n.ScreenshotShowFolder))
	t.folderBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventScreenshotShowFolder)
	})
	context.SetEnabled(&t.folderBtn, dest != "—" && !busy)

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *ScreenshotTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.toolbarItems = slices.Delete(t.toolbarItems, 0, len(t.toolbarItems))
	t.toolbarItems = append(t.toolbarItems,
		guigui.LinearLayoutItem{Widget: &t.modeText},
		guigui.LinearLayoutItem{Widget: &t.mode},
		guigui.LinearLayoutItem{Widget: &t.delayText},
		guigui.LinearLayoutItem{Widget: &t.delayInput},
		guigui.LinearLayoutItem{Widget: &t.hideText},
		guigui.LinearLayoutItem{Widget: &t.hideToggle},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.captureBtn},
	)
	t.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.toolbarItems,
		Gap:       u / 4,
	}

	previewContent := guigui.Widget(&t.previewEmpty)
	if t.preview.HasImage() {
		previewContent = &t.preview
	}

	t.outputItems = slices.Delete(t.outputItems, 0, len(t.outputItems))
	t.outputItems = append(t.outputItems,
		guigui.LinearLayoutItem{Widget: &t.saveBtn},
		guigui.LinearLayoutItem{Widget: &t.saveAsBtn},
		guigui.LinearLayoutItem{Widget: &t.copyBtn},
		guigui.LinearLayoutItem{Widget: &t.sendBtn},
		guigui.LinearLayoutItem{Widget: &t.folderBtn},
		guigui.LinearLayoutItem{Widget: &t.destLabel, Size: guigui.FlexibleSize(1)},
	)
	t.outputRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.outputItems,
		Gap:       u / 4,
	}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Widget: &t.previewLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: previewContent, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.outputRow},
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u)},
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
