package app

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// RTSPTool plays an RTSP stream and shows protocol, decode, and network stats.
type RTSPTool struct {
	guigui.DefaultWidget

	urlLabel   basicwidget.Text
	urlInput   basicwidget.TextInput
	connectBtn basicwidget.Button
	hint       basicwidget.Text

	previewLabel basicwidget.Text
	preview      destPreview
	previewZoom  previewZoomBar
	previewEmpty basicwidget.Text
	showPreview  bool

	decodeLabel basicwidget.Text
	decodeText  basicwidget.Text
	netLabel    basicwidget.Text
	netText     basicwidget.Text

	logLabel  basicwidget.Text
	logList   basicwidget.List[int]
	logDetail basicwidget.Text
	logItems  []basicwidget.ListItem[int]

	status basicwidget.Text

	urlRowItems   []guigui.LinearLayoutItem
	prevHeadItems []guigui.LinearLayoutItem
	prevColItems  []guigui.LinearLayoutItem
	statsItems    []guigui.LinearLayoutItem
	bodyItems     []guigui.LinearLayoutItem
	logBodyItems  []guigui.LinearLayoutItem
	layoutItems   []guigui.LinearLayoutItem

	urlRow   guigui.LinearLayout
	prevHead guigui.LinearLayout
	prevCol  guigui.LinearLayout
	statsCol guigui.LinearLayout
	bodyRow  guigui.LinearLayout
	logBody  guigui.LinearLayout
}

func (t *RTSPTool) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return
	}
	m := v.(*Model).RTSP()
	w.WriteUint64(m.Generation())
	w.WriteBool(m.Playing())
	w.WriteBool(m.HasImage())
	w.WriteString(m.URL())
	w.WriteInt(m.SelectedLog())
	w.WriteInt(m.DownloadPercent())
}

func (t *RTSPTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.urlLabel)
	adder.AddWidget(&t.urlInput)
	adder.AddWidget(&t.connectBtn)
	adder.AddWidget(&t.hint)
	adder.AddWidget(&t.previewLabel)
	adder.AddWidget(&t.previewZoom)
	adder.AddWidget(&t.decodeLabel)
	adder.AddWidget(&t.decodeText)
	adder.AddWidget(&t.netLabel)
	adder.AddWidget(&t.netText)
	adder.AddWidget(&t.logLabel)
	adder.AddWidget(&t.logList)
	adder.AddWidget(&t.logDetail)
	adder.AddWidget(&t.status)

	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.RTSP()
	lang := appModel.Lang()
	playing := model.Playing()
	has := model.HasImage()

	t.urlLabel.SetValue(i18n.T(lang, i18n.RTSPURL))
	t.urlLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	t.urlInput.SetPlaceholder(i18n.T(lang, i18n.RTSPPlaceholder))
	t.urlInput.SetValue(model.URL())
	t.urlInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		model.SetURL(text)
	})
	t.urlInput.OnHandleButtonInput(func(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
			model.Toggle()
			return guigui.HandleInputByWidget(&t.urlInput)
		}
		return guigui.HandleInputResult{}
	})
	context.SetEnabled(&t.urlInput, !playing)

	if playing {
		t.connectBtn.SetText(i18n.T(lang, i18n.RTSPDisconnect))
	} else {
		t.connectBtn.SetText(i18n.T(lang, i18n.RTSPConnect))
	}
	t.connectBtn.SetType(basicwidget.ButtonTypePrimary)
	t.connectBtn.OnDown(func(context *guigui.Context) {
		model.Toggle()
	})

	t.hint.SetValue(model.Hint(lang))
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

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
		t.previewLabel.SetValue(i18n.T(lang, i18n.RTSPPreview))
		t.preview.SetImage(nil)
		t.previewZoom.Configure(context, &t.preview, lang, false)
		t.previewEmpty.SetMultiline(true)
		t.previewEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.previewEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.previewEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		if pct := model.DownloadPercent(); playing && pct >= 0 {
			t.previewEmpty.SetValue(i18n.T(lang, i18n.AndroidShotLiveDownload, pct))
		} else if playing {
			t.previewEmpty.SetValue(i18n.T(lang, i18n.StatusRTSPWaiting))
		} else {
			t.previewEmpty.SetValue(i18n.T(lang, i18n.RTSPEmpty))
		}
		adder.AddWidget(&t.previewEmpty)
	}

	setBoldText(&t.decodeLabel, true)
	t.decodeLabel.SetValue(i18n.T(lang, i18n.RTSPDecode))
	t.decodeText.SetMultiline(true)
	t.decodeText.SetValue(model.DecodeStatsText(lang))

	setBoldText(&t.netLabel, true)
	t.netLabel.SetValue(i18n.T(lang, i18n.RTSPNetwork))
	t.netText.SetMultiline(true)
	t.netText.SetValue(model.NetworkStatsText(lang))

	setBoldText(&t.logLabel, true)
	t.logLabel.SetValue(i18n.T(lang, i18n.RTSPLog))
	logs := model.Logs()
	t.logItems = slices.Delete(t.logItems, 0, len(t.logItems))
	for i, e := range logs {
		t.logItems = append(t.logItems, basicwidget.ListItem[int]{
			Text:  e.Summary(),
			Value: i,
		})
	}
	t.logList.SetStyle(basicwidget.ListStyleNormal)
	t.logList.SetHighlightVisibleWhenUnfocused(true)
	t.logList.SetItems(t.logItems)
	if len(logs) > 0 {
		t.logList.SelectItemByValue(model.SelectedLog())
		t.logList.ForceEnsureItemVisibleByIndex(model.SelectedLog())
	}
	t.logList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.logList.ItemByIndex(index)
		if !ok {
			return
		}
		model.SelectLog(item.Value)
	})

	t.logDetail.SetMultiline(true)
	t.logDetail.SetSelectable(true)
	t.logDetail.SetEditable(false)
	t.logDetail.SetWrapMode(basicwidget.WrapModeNormal)
	detail := model.SelectedLogText()
	if detail == "" {
		t.logDetail.SetValue(i18n.T(lang, i18n.RTSPLogEmpty))
	} else {
		t.logDetail.SetValue(detail)
	}

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *RTSPTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.urlRowItems = slices.Delete(t.urlRowItems, 0, len(t.urlRowItems))
	t.urlRowItems = append(t.urlRowItems,
		guigui.LinearLayoutItem{Widget: &t.urlLabel},
		guigui.LinearLayoutItem{Widget: &t.urlInput, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.connectBtn},
	)
	t.urlRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.urlRowItems,
		Gap:       u / 4,
	}

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

	t.statsItems = slices.Delete(t.statsItems, 0, len(t.statsItems))
	t.statsItems = append(t.statsItems,
		guigui.LinearLayoutItem{Widget: &t.decodeLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.decodeText, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.netLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.netText, Size: guigui.FlexibleSize(1)},
	)
	t.statsCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.statsItems, Gap: u / 4}

	t.bodyItems = slices.Delete(t.bodyItems, 0, len(t.bodyItems))
	t.bodyItems = append(t.bodyItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(3), Layout: &t.prevCol},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(14 * u), Layout: &t.statsCol},
	)
	t.bodyRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.bodyItems, Gap: u / 2}

	t.logBodyItems = slices.Delete(t.logBodyItems, 0, len(t.logBodyItems))
	t.logBodyItems = append(t.logBodyItems,
		guigui.LinearLayoutItem{Widget: &t.logList, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.logDetail, Size: guigui.FlexibleSize(1)},
	)
	t.logBody = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.logBodyItems, Gap: u / 4}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.urlRow},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(2), Layout: &t.bodyRow},
		guigui.LinearLayoutItem{Widget: &t.logLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.logBody},
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
