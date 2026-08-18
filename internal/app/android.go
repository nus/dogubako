package app

import (
	"image"
	"image/color"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

var (
	eventAndroidRefresh    = guigui.GenerateEventKey()
	eventAndroidUp         = guigui.GenerateEventKey()
	eventAndroidPull       = guigui.GenerateEventKey()
	eventAndroidPushFile   = guigui.GenerateEventKey()
	eventAndroidPushFolder = guigui.GenerateEventKey()
)

// AndroidTool browses files on a connected Android device over ADB.
type AndroidTool struct {
	guigui.DefaultWidget

	deviceLabel basicwidget.Text
	deviceList  basicwidget.List[string]
	refreshBtn  basicwidget.Button
	upBtn       basicwidget.Button
	pathLabel   basicwidget.Text
	hint        basicwidget.Text

	header   androidFileHeader
	fileList basicwidget.List[string]
	rows     guigui.WidgetSlice[*androidFileRow]
	empty    basicwidget.Text

	pullBtn  basicwidget.Button
	pushFile basicwidget.Button
	pushDir  basicwidget.Button
	status   basicwidget.Text

	deviceItems  []basicwidget.ListItem[string]
	fileItems    []basicwidget.ListItem[string]
	toolbarItems []guigui.LinearLayoutItem
	headerItems  []guigui.LinearLayoutItem
	fileColItems []guigui.LinearLayoutItem
	actionItems  []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
	toolbar      guigui.LinearLayout
	fileCol      guigui.LinearLayout
	actionRow    guigui.LinearLayout

	showEmpty bool
}

func (t *AndroidTool) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return
	}
	m := v.(*Model).Android()
	w.WriteUint64(m.Generation())
	w.WriteString(m.Serial())
	w.WriteString(m.Selected())
	w.WriteBool(m.Busy())
}

func (t *AndroidTool) OnRefresh(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidRefresh, f)
}
func (t *AndroidTool) OnUp(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidUp, f)
}
func (t *AndroidTool) OnPull(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidPull, f)
}
func (t *AndroidTool) OnPushFile(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidPushFile, f)
}
func (t *AndroidTool) OnPushFolder(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventAndroidPushFolder, f)
}

func (t *AndroidTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.deviceLabel)
	adder.AddWidget(&t.deviceList)
	adder.AddWidget(&t.refreshBtn)
	adder.AddWidget(&t.upBtn)
	adder.AddWidget(&t.pathLabel)
	adder.AddWidget(&t.hint)
	adder.AddWidget(&t.header)
	adder.AddWidget(&t.pullBtn)
	adder.AddWidget(&t.pushFile)
	adder.AddWidget(&t.pushDir)
	adder.AddWidget(&t.status)

	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.Android()
	lang := appModel.Lang()
	busy := model.Busy()
	online := false
	for _, d := range model.Devices() {
		if d.Serial == model.Serial() && d.Online() {
			online = true
			break
		}
	}

	model.EnsureLoaded()

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
		guigui.DispatchEvent(t, eventAndroidRefresh)
	})
	context.SetEnabled(&t.refreshBtn, !busy)

	t.upBtn.SetText(i18n.T(lang, i18n.AndroidUp))
	t.upBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidUp)
	})
	context.SetEnabled(&t.upBtn, !busy && model.CanGoUp())

	t.pathLabel.SetValue(model.Root())
	t.pathLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	t.hint.SetValue(i18n.T(lang, i18n.AndroidHint))
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	t.header.SetLang(lang)

	rows := model.Rows()
	t.showEmpty = len(rows) == 0
	if t.showEmpty {
		t.empty.SetMultiline(true)
		t.empty.SetWrapMode(basicwidget.WrapModeNormal)
		t.empty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.empty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		if len(devs) == 0 {
			t.empty.SetValue(i18n.T(lang, i18n.AndroidNoDevices))
		} else {
			t.empty.SetValue(i18n.T(lang, i18n.AndroidEmpty))
		}
		adder.AddWidget(&t.empty)
	} else {
		adder.AddWidget(&t.fileList)
		t.rows.SetLen(len(rows))
		t.fileItems = slices.Delete(t.fileItems, 0, len(t.fileItems))
		for i, row := range rows {
			r := t.rows.At(i)
			path := row.Entry.Path
			r.Set(row, func() {
				model.ToggleExpand(path)
			})
			t.fileItems = append(t.fileItems, basicwidget.ListItem[string]{
				Content: r,
				Value:   path,
			})
		}
		t.fileList.SetStyle(basicwidget.ListStyleNormal)
		t.fileList.SetHighlightVisibleWhenUnfocused(true)
		t.fileList.SetItems(t.fileItems)
		if sel := model.Selected(); sel != "" {
			t.fileList.SelectItemByValue(sel)
		}
		t.fileList.OnItemSelected(func(context *guigui.Context, index int) {
			item, ok := t.fileList.ItemByIndex(index)
			if !ok || item.Value == "" {
				return
			}
			model.SelectPath(item.Value)
		})
		context.SetEnabled(&t.fileList, !busy)
	}

	t.pullBtn.SetText(i18n.T(lang, i18n.AndroidPull))
	t.pullBtn.SetType(basicwidget.ButtonTypePrimary)
	t.pullBtn.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidPull)
	})
	context.SetEnabled(&t.pullBtn, !busy && online && model.HasSelection())

	t.pushFile.SetText(i18n.T(lang, i18n.AndroidPushFile))
	t.pushFile.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidPushFile)
	})
	context.SetEnabled(&t.pushFile, !busy && online)

	t.pushDir.SetText(i18n.T(lang, i18n.AndroidPushFolder))
	t.pushDir.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventAndroidPushFolder)
	})
	context.SetEnabled(&t.pushDir, !busy && online)

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *AndroidTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.toolbarItems = slices.Delete(t.toolbarItems, 0, len(t.toolbarItems))
	t.toolbarItems = append(t.toolbarItems,
		guigui.LinearLayoutItem{Widget: &t.refreshBtn},
		guigui.LinearLayoutItem{Widget: &t.upBtn},
		guigui.LinearLayoutItem{Widget: &t.pathLabel, Size: guigui.FlexibleSize(1)},
	)
	t.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.toolbarItems,
		Gap:       u / 4,
	}

	fileBody := guigui.Widget(&t.empty)
	if !t.showEmpty {
		fileBody = &t.fileList
	}
	t.fileColItems = slices.Delete(t.fileColItems, 0, len(t.fileColItems))
	t.fileColItems = append(t.fileColItems,
		guigui.LinearLayoutItem{Widget: &t.header, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: fileBody, Size: guigui.FlexibleSize(1)},
	)
	t.fileCol = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.fileColItems, Gap: u / 8}

	t.actionItems = slices.Delete(t.actionItems, 0, len(t.actionItems))
	t.actionItems = append(t.actionItems,
		guigui.LinearLayoutItem{Widget: &t.pullBtn},
		guigui.LinearLayoutItem{Widget: &t.pushFile},
		guigui.LinearLayoutItem{Widget: &t.pushDir},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FlexibleSize(1)},
	)
	t.actionRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.actionItems,
		Gap:       u / 4,
	}

	deviceH := 4 * u
	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Widget: &t.deviceLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.deviceList, Size: guigui.FixedSize(deviceH)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.fileCol},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.actionRow},
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

type androidFileHeader struct {
	guigui.DefaultWidget

	name basicwidget.Text
	size basicwidget.Text
	mod  basicwidget.Text
	crt  basicwidget.Text

	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (h *androidFileHeader) SetLang(lang i18n.Lang) {
	h.name.SetValue(i18n.T(lang, i18n.AndroidColName))
	h.size.SetValue(i18n.T(lang, i18n.AndroidColSize))
	h.mod.SetValue(i18n.T(lang, i18n.AndroidColModified))
	h.crt.SetValue(i18n.T(lang, i18n.AndroidColCreated))
}

func (h *androidFileHeader) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&h.name)
	adder.AddWidget(&h.size)
	adder.AddWidget(&h.mod)
	adder.AddWidget(&h.crt)
	setBoldText(&h.name, true)
	setBoldText(&h.size, true)
	setBoldText(&h.mod, true)
	setBoldText(&h.crt, true)
	h.size.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	h.name.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	h.size.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	h.mod.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	h.crt.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (h *androidFileHeader) layoutRow(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	h.layoutItems = slices.Delete(h.layoutItems, 0, len(h.layoutItems))
	h.layoutItems = append(h.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &h.name, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &h.size, Size: guigui.FixedSize(5 * u)},
		guigui.LinearLayoutItem{Widget: &h.mod, Size: guigui.FixedSize(8 * u)},
		guigui.LinearLayoutItem{Widget: &h.crt, Size: guigui.FixedSize(8 * u)},
	)
	h.layout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     h.layoutItems,
		Gap:       u / 8,
	}
	return h.layout
}

func (h *androidFileHeader) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	h.layoutRow(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (h *androidFileHeader) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return h.layoutRow(context).Measure(context, constraints)
}

type androidFileRow struct {
	guigui.DefaultWidget

	expand basicwidget.Button
	name   basicwidget.Text
	size   basicwidget.Text
	mod    basicwidget.Text
	crt    basicwidget.Text

	depth int
	isDir bool

	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (r *androidFileRow) Set(row AndroidTreeRow, onExpand func()) {
	r.depth = row.Depth
	r.isDir = row.Entry.IsDir
	if row.Entry.IsDir {
		if row.Expanded {
			r.expand.SetText("▾")
		} else {
			r.expand.SetText("▸")
		}
		r.expand.OnDown(func(context *guigui.Context) {
			if onExpand != nil {
				onExpand()
			}
		})
	} else {
		r.expand.SetText("")
		r.expand.OnDown(func(context *guigui.Context) {})
	}
	r.name.SetValue(row.Entry.Name)
	r.size.SetValue(formatFileSize(row.Entry.Size, row.Entry.IsDir))
	r.mod.SetValue(formatFileTime(row.Entry.ModTime))
	r.crt.SetValue(formatFileTime(row.Entry.CrtTime))
}

func (r *androidFileRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if r.isDir {
		adder.AddWidget(&r.expand)
	}
	adder.AddWidget(&r.name)
	adder.AddWidget(&r.size)
	adder.AddWidget(&r.mod)
	adder.AddWidget(&r.crt)
	r.name.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.size.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.mod.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.crt.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.size.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	r.name.SetWrapMode(basicwidget.WrapModeAnywhere)
	return nil
}

func (r *androidFileRow) layoutRow(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	expandOrPad := guigui.LinearLayoutItem{Size: guigui.FixedSize(u)}
	if r.isDir {
		expandOrPad = guigui.LinearLayoutItem{Widget: &r.expand, Size: guigui.FixedSize(u)}
	}
	indent := r.depth * (u / 2)
	if indent > 0 {
		r.layoutItems = append(r.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(indent)})
	}
	r.layoutItems = append(r.layoutItems,
		expandOrPad,
		guigui.LinearLayoutItem{Widget: &r.name, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.size, Size: guigui.FixedSize(5 * u)},
		guigui.LinearLayoutItem{Widget: &r.mod, Size: guigui.FixedSize(8 * u)},
		guigui.LinearLayoutItem{Widget: &r.crt, Size: guigui.FixedSize(8 * u)},
	)
	r.layout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.layoutItems,
		Gap:       u / 8,
	}
	return r.layout
}

func (r *androidFileRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	var clr color.Color
	if v, ok := context.Env(r, basicwidget.EnvKeyListItemColorType); ok {
		if ct, ok := v.(basicwidget.ListItemColorType); ok {
			clr = ct.TextColor(context)
		}
	}
	var style basicwidget.TextStyle
	if clr != nil {
		style.SetColor(clr)
	}
	r.name.SetBaseStyle(&style)
	r.size.SetBaseStyle(&style)
	r.mod.SetBaseStyle(&style)
	r.crt.SetBaseStyle(&style)
	r.layoutRow(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *androidFileRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	s := r.layoutRow(context).Measure(context, constraints)
	if s.Y < u {
		s.Y = u
	}
	return s
}
