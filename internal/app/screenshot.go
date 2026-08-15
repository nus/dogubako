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

	listLabel basicwidget.Text
	fileList  basicwidget.List[string]

	previewLabel basicwidget.Text
	preview      destPreview
	previewEmpty basicwidget.Text

	destLabel basicwidget.Text
	saveAsBtn basicwidget.Button
	copyBtn   basicwidget.Button
	sendBtn   basicwidget.Button
	folderBtn basicwidget.Button
	status    basicwidget.Text

	fileItems    []basicwidget.ListItem[string]
	toolbarItems []guigui.LinearLayoutItem
	listColItems []guigui.LinearLayoutItem
	prevColItems []guigui.LinearLayoutItem
	bodyItems    []guigui.LinearLayoutItem
	outputItems  []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
	toolbar      guigui.LinearLayout
	listCol      guigui.LinearLayout
	prevCol      guigui.LinearLayout
	bodyRow      guigui.LinearLayout
	outputRow    guigui.LinearLayout
}

func (t *ScreenshotTool) OnCapture(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventScreenshotCapture, f)
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
	adder.AddWidget(&t.listLabel)
	adder.AddWidget(&t.fileList)
	adder.AddWidget(&t.previewLabel)
	adder.AddWidget(&t.destLabel)
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

	model.RefreshFiles()

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

	setBoldText(&t.listLabel, true)
	t.listLabel.SetValue(i18n.T(lang, i18n.ScreenshotFiles))
	files := model.Files()
	t.fileItems = slices.Delete(t.fileItems, 0, len(t.fileItems))
	for _, f := range files {
		t.fileItems = append(t.fileItems, basicwidget.ListItem[string]{
			Text:  f.Name,
			Value: f.Path,
		})
	}
	t.fileList.SetStyle(basicwidget.ListStyleNormal)
	t.fileList.SetHighlightVisibleWhenUnfocused(true)
	t.fileList.SetItemHeight(u)
	t.fileList.SetItems(t.fileItems)
	if sel := model.SelectedPath(); sel != "" {
		t.fileList.SelectItemByValue(sel)
		if model.TakeJumpToSelected() {
			if idx := t.fileList.IndexByValue(sel); idx >= 0 {
				t.fileList.ForceEnsureItemVisibleByIndex(idx)
			}
		}
	}
	t.fileList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.fileList.ItemByIndex(index)
		if !ok || item.Value == "" {
			return
		}
		_ = model.LoadPath(item.Value)
	})
	context.SetEnabled(&t.fileList, !busy)

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

	t.listColItems = slices.Delete(t.listColItems, 0, len(t.listColItems))
	t.listColItems = append(t.listColItems,
		guigui.LinearLayoutItem{Widget: &t.listLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.fileList, Size: guigui.FlexibleSize(1)},
	)
	t.listCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.listColItems, Gap: u / 4}

	previewContent := guigui.Widget(&t.previewEmpty)
	if t.preview.HasImage() {
		previewContent = &t.preview
	}
	t.prevColItems = slices.Delete(t.prevColItems, 0, len(t.prevColItems))
	t.prevColItems = append(t.prevColItems,
		guigui.LinearLayoutItem{Widget: &t.previewLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: previewContent, Size: guigui.FlexibleSize(1)},
	)
	t.prevCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.prevColItems, Gap: u / 4}

	t.bodyItems = slices.Delete(t.bodyItems, 0, len(t.bodyItems))
	t.bodyItems = append(t.bodyItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(14 * u), Layout: &t.listCol},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.prevCol},
	)
	t.bodyRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.bodyItems, Gap: u / 2}

	t.outputItems = slices.Delete(t.outputItems, 0, len(t.outputItems))
	t.outputItems = append(t.outputItems,
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
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.bodyRow},
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
