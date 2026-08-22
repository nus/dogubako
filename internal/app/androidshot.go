package app

import (
	"image"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

var (
	eventAndroidShotCapture    = guigui.GenerateEventKey()
	eventAndroidShotRefresh    = guigui.GenerateEventKey()
	eventAndroidShotSaveAs     = guigui.GenerateEventKey()
	eventAndroidShotCopy       = guigui.GenerateEventKey()
	eventAndroidShotSendImage  = guigui.GenerateEventKey()
	eventAndroidShotShowFolder = guigui.GenerateEventKey()
)

// AndroidShotTool captures an Android device display over ADB.
type AndroidShotTool struct {
	guigui.DefaultWidget

	deviceLabel basicwidget.Text
	deviceList  basicwidget.List[string]
	refreshBtn  basicwidget.Button
	delayText   basicwidget.Text
	delayInput  guigui.WidgetWithSize[*basicwidget.NumberInput]
	hint        basicwidget.Text
	captureBtn  basicwidget.Button

	listLabel basicwidget.Text
	fileList  basicwidget.List[string]
	rows      guigui.WidgetSlice[*fileRow]
	thumbGPU  map[string]*ebiten.Image
	thumbMod  map[string]int64

	previewLabel basicwidget.Text
	preview      destPreview
	previewZoom  previewZoomBar
	previewEmpty basicwidget.Text
	showPreview  bool

	destLabel basicwidget.Text
	saveAsBtn basicwidget.Button
	copyBtn   basicwidget.Button
	sendBtn   basicwidget.Button
	folderBtn basicwidget.Button
	status    basicwidget.Text

	deviceItems   []basicwidget.ListItem[string]
	fileItems     []basicwidget.ListItem[string]
	toolbarItems  []guigui.LinearLayoutItem
	listColItems  []guigui.LinearLayoutItem
	prevHeadItems []guigui.LinearLayoutItem
	prevColItems  []guigui.LinearLayoutItem
	bodyItems     []guigui.LinearLayoutItem
	outputItems   []guigui.LinearLayoutItem
	layoutItems   []guigui.LinearLayoutItem
	toolbar       guigui.LinearLayout
	listCol       guigui.LinearLayout
	prevHead      guigui.LinearLayout
	prevCol       guigui.LinearLayout
	bodyRow       guigui.LinearLayout
	outputRow     guigui.LinearLayout
}

func (t *AndroidShotTool) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return
	}
	shot := v.(*Model).AndroidShot()
	w.WriteUint64(shot.Generation())
	w.WriteBool(shot.HasImage())
	w.WriteBool(shot.Busy())
	w.WriteString(shot.Serial())
}

func (t *AndroidShotTool) OnCapture(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotCapture, f)
}
func (t *AndroidShotTool) OnRefresh(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotRefresh, f)
}
func (t *AndroidShotTool) OnSaveAs(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotSaveAs, f)
}
func (t *AndroidShotTool) OnCopy(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotCopy, f)
}
func (t *AndroidShotTool) OnSendToImage(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotSendImage, f)
}
func (t *AndroidShotTool) OnShowFolder(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidShotShowFolder, f)
}

func (t *AndroidShotTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.deviceLabel)
	adder.AddWidget(&t.deviceList)
	adder.AddWidget(&t.refreshBtn)
	adder.AddWidget(&t.delayText)
	adder.AddWidget(&t.delayInput)
	adder.AddWidget(&t.hint)
	adder.AddWidget(&t.captureBtn)
	adder.AddWidget(&t.listLabel)
	adder.AddWidget(&t.fileList)
	adder.AddWidget(&t.previewLabel)
	adder.AddWidget(&t.previewZoom)
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
	model := appModel.AndroidShot()
	lang := appModel.Lang()
	u := basicwidget.UnitSize(context)
	has := model.HasImage()
	busy := model.Busy()
	capturing := model.Capturing()
	online := model.Online()

	model.EnsureLoaded()
	model.RefreshFiles()

	setBoldText(&t.deviceLabel, true)
	t.deviceLabel.SetValue(i18n.T(lang, i18n.AndroidDevices))

	devs := model.Devices()
	t.deviceItems = slices.Delete(t.deviceItems, 0, len(t.deviceItems))
	for _, d := range devs {
		t.deviceItems = append(t.deviceItems, basicwidget.ListItem[string]{
			Text:  d.Label(),
			Value: d.Serial,
		})
	}
	t.deviceList.SetStyle(basicwidget.ListStyleNormal)
	t.deviceList.SetHighlightVisibleWhenUnfocused(true)
	t.deviceList.SetItems(t.deviceItems)
	if model.Serial() != "" {
		t.deviceList.SelectItemByValue(model.Serial())
	}
	t.deviceList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.deviceList.ItemByIndex(index)
		if !ok || item.Value == "" {
			return
		}
		model.SelectDevice(item.Value)
	})
	context.SetEnabled(&t.deviceList, !busy)

	t.refreshBtn.SetText(i18n.T(lang, i18n.AndroidRefresh))
	t.refreshBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidShotRefresh)
	})
	context.SetEnabled(&t.refreshBtn, !busy)

	t.delayText.SetValue(i18n.T(lang, i18n.ScreenshotDelay))
	t.delayText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	configureIntInput(t.delayInput.Widget(), 0, maxCaptureDelaySec, 1, model.DelaySec(), func(v int, committed bool) {
		if committed {
			model.SetDelaySec(v)
		}
	})
	t.delayInput.SetFixedWidth(3 * u)
	context.SetEnabled(&t.delayInput, !busy)

	t.hint.SetValue(i18n.T(lang, i18n.AndroidShotHint))
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if capturing {
		t.captureBtn.SetText(i18n.T(lang, i18n.ScreenshotCaptureRetry))
	} else {
		t.captureBtn.SetText(i18n.T(lang, i18n.AndroidShotCapture))
	}
	t.captureBtn.SetType(basicwidget.ButtonTypePrimary)
	t.captureBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidShotCapture)
	})
	context.SetEnabled(&t.captureBtn, capturing || (online && !busy))

	setBoldText(&t.listLabel, true)
	t.listLabel.SetValue(i18n.T(lang, i18n.ScreenshotFiles))
	files := model.Files()
	t.rows.SetLen(len(files))
	t.fileItems = slices.Delete(t.fileItems, 0, len(t.fileItems))
	liveThumbs := make(map[string]struct{}, len(files))
	for i, f := range files {
		liveThumbs[f.Path] = struct{}{}
		row := t.rows.At(i)
		row.Set(f.Name, t.gpuThumb(f.Path, model.Thumbnail(f.Path), f.ModTime))
		t.fileItems = append(t.fileItems, basicwidget.ListItem[string]{
			Content: row,
			Value:   f.Path,
		})
	}
	for p := range t.thumbGPU {
		if _, ok := liveThumbs[p]; !ok {
			if img := t.thumbGPU[p]; img != nil {
				img.Deallocate()
			}
			delete(t.thumbGPU, p)
			delete(t.thumbMod, p)
		}
	}
	t.fileList.SetStyle(basicwidget.ListStyleNormal)
	t.fileList.SetHighlightVisibleWhenUnfocused(true)
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
	t.showPreview = has
	if has {
		sz := model.Size()
		t.previewLabel.SetValue(i18n.T(lang, i18n.ScreenshotPreviewSz, sz.X, sz.Y))
		t.preview.SetImage(model.Preview())
		t.preview.SetLogicalSize(sz)
		t.previewZoom.Configure(context, &t.preview, lang, true)
		adder.AddWidget(&t.preview)
	} else {
		t.previewLabel.SetValue(i18n.T(lang, i18n.ScreenshotPreview))
		t.preview.SetImage(nil)
		t.previewZoom.Configure(context, &t.preview, lang, false)
		t.previewEmpty.SetMultiline(true)
		t.previewEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.previewEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.previewEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		if len(devs) == 0 {
			t.previewEmpty.SetValue(i18n.T(lang, i18n.AndroidNoDevices))
		} else {
			t.previewEmpty.SetValue(i18n.T(lang, i18n.AndroidShotEmpty))
		}
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
		guigui.DispatchEvent(t, eventAndroidShotSaveAs)
	})
	context.SetEnabled(&t.saveAsBtn, has && !busy)

	t.copyBtn.SetText(i18n.T(lang, i18n.ScreenshotCopy))
	t.copyBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidShotCopy)
	})
	context.SetEnabled(&t.copyBtn, has && !busy)

	t.sendBtn.SetText(i18n.T(lang, i18n.ScreenshotSendImage))
	t.sendBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidShotSendImage)
	})
	context.SetEnabled(&t.sendBtn, has && !busy)

	t.folderBtn.SetText(i18n.T(lang, i18n.ScreenshotShowFolder))
	t.folderBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidShotShowFolder)
	})
	context.SetEnabled(&t.folderBtn, dest != "—" && !busy)

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *AndroidShotTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.toolbarItems = slices.Delete(t.toolbarItems, 0, len(t.toolbarItems))
	t.toolbarItems = append(t.toolbarItems,
		guigui.LinearLayoutItem{Widget: &t.refreshBtn},
		guigui.LinearLayoutItem{Widget: &t.delayText},
		guigui.LinearLayoutItem{Widget: &t.delayInput},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FlexibleSize(1)},
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
	if t.showPreview {
		previewContent = &t.preview
	}
	t.prevHeadItems = slices.Delete(t.prevHeadItems, 0, len(t.prevHeadItems))
	t.prevHeadItems = append(t.prevHeadItems,
		guigui.LinearLayoutItem{Widget: &t.previewLabel, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.previewZoom},
	)
	t.prevHead = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.prevHeadItems, Gap: u / 4}

	t.prevColItems = slices.Delete(t.prevColItems, 0, len(t.prevColItems))
	t.prevColItems = append(t.prevColItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.prevHead},
		guigui.LinearLayoutItem{Widget: previewContent, Size: guigui.FlexibleSize(1)},
	)
	t.prevCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.prevColItems, Gap: u / 4}

	t.bodyItems = slices.Delete(t.bodyItems, 0, len(t.bodyItems))
	t.bodyItems = append(t.bodyItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(16 * u), Layout: &t.listCol},
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

	deviceH := 3 * u
	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Widget: &t.deviceLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.deviceList, Size: guigui.FixedSize(deviceH)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.bodyRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.outputRow},
		guigui.LinearLayoutItem{Widget: &t.status, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     t.layoutItems,
		Gap:       u / 4,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (t *AndroidShotTool) gpuThumb(path string, src image.Image, mod time.Time) *ebiten.Image {
	if src == nil || path == "" {
		return nil
	}
	if t.thumbGPU == nil {
		t.thumbGPU = make(map[string]*ebiten.Image)
		t.thumbMod = make(map[string]int64)
	}
	nano := mod.UnixNano()
	if t.thumbMod[path] == nano {
		return t.thumbGPU[path]
	}
	if old := t.thumbGPU[path]; old != nil {
		old.Deallocate()
	}
	img := ebiten.NewImageFromImage(src)
	t.thumbGPU[path] = img
	t.thumbMod[path] = nano
	return img
}
