package app

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
)

var (
	eventOpenFile = guigui.GenerateEventKey()
	eventSaveFile = guigui.GenerateEventKey()
	eventPaste    = guigui.GenerateEventKey()
	eventCopy     = guigui.GenerateEventKey()
)

// ImageTool is the first workspace tool: resize, crop, and JPEG/PNG convert.
type ImageTool struct {
	guigui.DefaultWidget

	openButton  basicwidget.Button
	pasteButton basicwidget.Button
	hint        basicwidget.Text

	beforeLabel   basicwidget.Text
	beforePreview sourcePreview
	afterLabel    basicwidget.Text
	afterImage    destPreview
	afterEmpty    basicwidget.Text

	resizePanel editPanel
	cropPanel   editPanel
	formatPanel editPanel

	sectionSize     basicwidget.Text
	scaleText       basicwidget.Text
	scaleInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	widthText       basicwidget.Text
	widthInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	heightText      basicwidget.Text
	heightInput     guigui.WidgetWithSize[*basicwidget.NumberInput]
	keepAspectText  basicwidget.Text
	keepAspect      basicwidget.Toggle
	resetSizeButton basicwidget.Button

	sectionCrop     basicwidget.Text
	cropEnabledText basicwidget.Text
	cropEnabled     basicwidget.Toggle
	cropXText       basicwidget.Text
	cropXInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	cropYText       basicwidget.Text
	cropYInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	cropWText       basicwidget.Text
	cropWInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	cropHText       basicwidget.Text
	cropHInput      guigui.WidgetWithSize[*basicwidget.NumberInput]
	resetCropButton basicwidget.Button

	sectionOut  basicwidget.Text
	formatText  basicwidget.Text
	format      basicwidget.SegmentedControl[imageproc.Format]
	qualityText basicwidget.Text
	quality     basicwidget.Slider
	saveButton  basicwidget.Button
	copyButton  basicwidget.Button
	outputHint  basicwidget.Text

	status basicwidget.Text

	toolbarItems []guigui.LinearLayoutItem
	previewItems []guigui.LinearLayoutItem
	beforeItems  []guigui.LinearLayoutItem
	afterItems   []guigui.LinearLayoutItem
	editItems    []guigui.LinearLayoutItem
	outputItems  []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
	beforeLayout guigui.LinearLayout
	afterLayout  guigui.LinearLayout
	previewRow   guigui.LinearLayout
	toolbar      guigui.LinearLayout
	editRow      guigui.LinearLayout
	outputRow    guigui.LinearLayout
}

func (t *ImageTool) OnOpenFile(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventOpenFile, f)
}

func (t *ImageTool) OnSaveFile(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventSaveFile, f)
}

func (t *ImageTool) OnPaste(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventPaste, f)
}

func (t *ImageTool) OnCopy(f func(context *guigui.Context)) {
	guigui.SetEventHandler(t, eventCopy, f)
}

func (t *ImageTool) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.openButton)
	adder.AddWidget(&t.pasteButton)
	adder.AddWidget(&t.hint)
	adder.AddWidget(&t.beforeLabel)
	adder.AddWidget(&t.beforePreview)
	adder.AddWidget(&t.afterLabel)
	adder.AddWidget(&t.resizePanel)
	adder.AddWidget(&t.cropPanel)
	adder.AddWidget(&t.formatPanel)
	adder.AddWidget(&t.saveButton)
	adder.AddWidget(&t.copyButton)
	adder.AddWidget(&t.outputHint)
	adder.AddWidget(&t.status)

	v, ok := context.Env(t, EnvKeyModel)
	if !ok {
		return nil
	}
	appModel := v.(*Model)
	model := appModel.Image()
	lang := appModel.Lang()
	u := basicwidget.UnitSize(context)
	fieldW := 5 * u
	has := model.HasSource()

	t.openButton.SetText(i18n.T(lang, i18n.OpenFile))
	t.openButton.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventOpenFile)
	})
	t.pasteButton.SetText(i18n.T(lang, i18n.PasteClipboard))
	t.pasteButton.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventPaste)
	})
	t.hint.SetValue(i18n.T(lang, i18n.InputHint, ShortcutLabel(context, "V")))
	t.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	srcSize := model.SourceSize()
	setBoldText(&t.beforeLabel, true)
	if has {
		t.beforeLabel.SetValue(i18n.T(lang, i18n.BeforeSize, srcSize.X, srcSize.Y))
	} else {
		t.beforeLabel.SetValue(i18n.T(lang, i18n.Before))
	}
	t.beforePreview.SetImage(model.SourcePreview(), srcSize)
	t.beforePreview.SetCrop(model.Crop(), model.CropEnabled())
	t.beforePreview.OnCropChanged(func(rect image.Rectangle) {
		model.SetCrop(rect)
	})

	setBoldText(&t.afterLabel, true)
	if out, err := model.Processed(); err == nil && out != nil {
		t.afterLabel.SetValue(i18n.T(lang, i18n.AfterSize, out.Bounds().Dx(), out.Bounds().Dy(), model.Format()))
		t.afterImage.SetImage(model.ResultPreview())
		adder.AddWidget(&t.afterImage)
	} else {
		t.afterLabel.SetValue(i18n.T(lang, i18n.After))
		t.afterImage.SetImage(nil)
		t.afterEmpty.SetMultiline(true)
		t.afterEmpty.SetWrapMode(basicwidget.WrapModeNormal)
		t.afterEmpty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		t.afterEmpty.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		t.afterEmpty.SetValue(i18n.T(lang, i18n.AfterEmpty))
		adder.AddWidget(&t.afterEmpty)
	}

	t.configureForms(context, model, lang, fieldW)

	t.saveButton.SetText(i18n.T(lang, i18n.SaveFile))
	t.saveButton.SetType(basicwidget.ButtonTypePrimary)
	t.saveButton.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventSaveFile)
	})
	context.SetEnabled(&t.saveButton, has)
	t.copyButton.SetText(i18n.T(lang, i18n.CopyClipboard))
	t.copyButton.OnDown(func(context *guigui.Context) {
		guigui.DispatchEvent(t, eventCopy)
	})
	context.SetEnabled(&t.copyButton, has)
	t.outputHint.SetValue(i18n.T(lang, i18n.OutputHint))
	t.outputHint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	t.status.SetValue(model.StatusText(lang))
	t.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	return nil
}

func (t *ImageTool) configureForms(context *guigui.Context, model *ImageModel, lang i18n.Lang, fieldW int) {
	t.sectionSize.SetValue(i18n.T(lang, i18n.Resize))
	setBoldText(&t.sectionSize, true)

	t.scaleText.SetValue(i18n.T(lang, i18n.ScalePercent))
	configureIntInput(t.scaleInput.Widget(), 1, 1000, 1, model.ScalePercent(), func(v int, committed bool) {
		if committed {
			model.SetScalePercent(v)
		}
	})
	t.scaleInput.SetFixedWidth(fieldW)
	context.SetEnabled(&t.scaleInput, model.HasSource())

	t.widthText.SetValue(i18n.T(lang, i18n.WidthPx))
	configureIntInput(t.widthInput.Widget(), 1, imageproc.MaxDimension, 1, model.Width(), func(v int, committed bool) {
		if committed {
			model.SetWidth(v)
		}
	})
	t.widthInput.SetFixedWidth(fieldW)
	context.SetEnabled(&t.widthInput, model.HasSource())

	t.heightText.SetValue(i18n.T(lang, i18n.HeightPx))
	configureIntInput(t.heightInput.Widget(), 1, imageproc.MaxDimension, 1, model.Height(), func(v int, committed bool) {
		if committed {
			model.SetHeight(v)
		}
	})
	t.heightInput.SetFixedWidth(fieldW)
	context.SetEnabled(&t.heightInput, model.HasSource())

	t.keepAspectText.SetValue(i18n.T(lang, i18n.KeepAspect))
	t.keepAspect.SetValue(model.KeepAspect())
	t.keepAspect.OnValueChanged(func(context *guigui.Context, value bool) {
		model.SetKeepAspect(value)
	})
	context.SetEnabled(&t.keepAspect, model.HasSource())

	t.resetSizeButton.SetText(i18n.T(lang, i18n.ResetSize))
	t.resetSizeButton.OnDown(func(context *guigui.Context) {
		model.ResetSize()
	})
	context.SetEnabled(&t.resetSizeButton, model.HasSource())

	t.sectionCrop.SetValue(i18n.T(lang, i18n.Crop))
	setBoldText(&t.sectionCrop, true)

	t.cropEnabledText.SetValue(i18n.T(lang, i18n.CropEnable))
	t.cropEnabled.SetValue(model.CropEnabled())
	t.cropEnabled.OnValueChanged(func(context *guigui.Context, value bool) {
		model.SetCropEnabled(value)
	})
	context.SetEnabled(&t.cropEnabled, model.HasSource())

	crop := model.Crop()
	src := model.SourceSize()
	t.cropXText.SetValue(i18n.T(lang, i18n.CropX))
	configureIntInput(t.cropXInput.Widget(), 0, max(0, src.X-1), 1, crop.Min.X, func(v int, committed bool) {
		if committed {
			model.SetCropX(v)
		}
	})
	t.cropXInput.SetFixedWidth(fieldW)

	t.cropYText.SetValue(i18n.T(lang, i18n.CropY))
	configureIntInput(t.cropYInput.Widget(), 0, max(0, src.Y-1), 1, crop.Min.Y, func(v int, committed bool) {
		if committed {
			model.SetCropY(v)
		}
	})
	t.cropYInput.SetFixedWidth(fieldW)

	t.cropWText.SetValue(i18n.T(lang, i18n.Width))
	configureIntInput(t.cropWInput.Widget(), 1, max(1, src.X), 1, max(1, crop.Dx()), func(v int, committed bool) {
		if committed {
			model.SetCropWidth(v)
		}
	})
	t.cropWInput.SetFixedWidth(fieldW)

	t.cropHText.SetValue(i18n.T(lang, i18n.Height))
	configureIntInput(t.cropHInput.Widget(), 1, max(1, src.Y), 1, max(1, crop.Dy()), func(v int, committed bool) {
		if committed {
			model.SetCropHeight(v)
		}
	})
	t.cropHInput.SetFixedWidth(fieldW)

	cropOn := model.HasSource() && model.CropEnabled()
	context.SetEnabled(&t.cropXInput, cropOn)
	context.SetEnabled(&t.cropYInput, cropOn)
	context.SetEnabled(&t.cropWInput, cropOn)
	context.SetEnabled(&t.cropHInput, cropOn)

	t.resetCropButton.SetText(i18n.T(lang, i18n.ResetCrop))
	t.resetCropButton.OnDown(func(context *guigui.Context) {
		model.ResetCrop()
	})
	context.SetEnabled(&t.resetCropButton, cropOn)

	t.sectionOut.SetValue(i18n.T(lang, i18n.Format))
	setBoldText(&t.sectionOut, true)

	t.formatText.SetValue(i18n.T(lang, i18n.OutputFormat))
	t.format.SetItems([]basicwidget.SegmentedControlItem[imageproc.Format]{
		{Text: "PNG", Value: imageproc.FormatPNG},
		{Text: "JPEG", Value: imageproc.FormatJPEG},
	})
	t.format.SelectItemByValue(model.Format())
	t.format.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.format.ItemByIndex(index)
		if !ok {
			return
		}
		model.SetFormat(item.Value)
	})
	context.SetEnabled(&t.format, model.HasSource())

	t.quality.SetMinimumValue(1)
	t.quality.SetMaximumValue(100)
	t.quality.SetValue(model.JPEGQuality())
	t.quality.OnValueChanged(func(context *guigui.Context, value int) {
		model.SetJPEGQuality(value)
	})
	context.SetEnabled(&t.quality, model.HasSource() && model.Format() == imageproc.FormatJPEG)
	t.qualityText.SetValue(i18n.T(lang, i18n.JPEGQuality, model.JPEGQuality()))

	t.resizePanel.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &t.sectionSize},
		{PrimaryWidget: &t.scaleText, SecondaryWidget: &t.scaleInput},
		{PrimaryWidget: &t.widthText, SecondaryWidget: &t.widthInput},
		{PrimaryWidget: &t.heightText, SecondaryWidget: &t.heightInput},
		{PrimaryWidget: &t.keepAspectText, SecondaryWidget: &t.keepAspect},
		{SecondaryWidget: &t.resetSizeButton},
	})
	t.cropPanel.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &t.sectionCrop},
		{PrimaryWidget: &t.cropEnabledText, SecondaryWidget: &t.cropEnabled},
		{PrimaryWidget: &t.cropXText, SecondaryWidget: &t.cropXInput},
		{PrimaryWidget: &t.cropYText, SecondaryWidget: &t.cropYInput},
		{PrimaryWidget: &t.cropWText, SecondaryWidget: &t.cropWInput},
		{PrimaryWidget: &t.cropHText, SecondaryWidget: &t.cropHInput},
		{SecondaryWidget: &t.resetCropButton},
	})
	t.formatPanel.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &t.sectionOut},
		{PrimaryWidget: &t.formatText, SecondaryWidget: &t.format},
		{PrimaryWidget: &t.qualityText, SecondaryWidget: &t.quality},
	})
}

func configureIntInput(n *basicwidget.NumberInput, minV, maxV, step, value int, on func(int, bool)) {
	n.SetMinimumValue(minV)
	n.SetMaximumValue(maxV)
	n.SetStep(step)
	n.SetValue(value)
	n.OnValueChanged(func(context *guigui.Context, v int, committed bool) {
		on(v, committed)
	})
}

func (t *ImageTool) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	t.toolbarItems = slices.Delete(t.toolbarItems, 0, len(t.toolbarItems))
	t.toolbarItems = append(t.toolbarItems,
		guigui.LinearLayoutItem{Widget: &t.openButton},
		guigui.LinearLayoutItem{Widget: &t.pasteButton},
		guigui.LinearLayoutItem{Widget: &t.hint, Size: guigui.FlexibleSize(1)},
	)
	t.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     t.toolbarItems,
		Gap:       u / 2,
	}

	afterContent := guigui.Widget(&t.afterEmpty)
	if t.afterImage.HasImage() {
		afterContent = &t.afterImage
	}
	t.beforeItems = slices.Delete(t.beforeItems, 0, len(t.beforeItems))
	t.beforeItems = append(t.beforeItems,
		guigui.LinearLayoutItem{Widget: &t.beforeLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &t.beforePreview, Size: guigui.FlexibleSize(1)},
	)
	t.beforeLayout = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.beforeItems, Gap: u / 4}

	t.afterItems = slices.Delete(t.afterItems, 0, len(t.afterItems))
	t.afterItems = append(t.afterItems,
		guigui.LinearLayoutItem{Widget: &t.afterLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: afterContent, Size: guigui.FlexibleSize(1)},
	)
	t.afterLayout = guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: t.afterItems, Gap: u / 4}

	t.previewItems = slices.Delete(t.previewItems, 0, len(t.previewItems))
	t.previewItems = append(t.previewItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.beforeLayout},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.afterLayout},
	)
	t.previewRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.previewItems, Gap: u}

	t.outputItems = slices.Delete(t.outputItems, 0, len(t.outputItems))
	t.outputItems = append(t.outputItems,
		guigui.LinearLayoutItem{Widget: &t.saveButton},
		guigui.LinearLayoutItem{Widget: &t.copyButton},
		guigui.LinearLayoutItem{Widget: &t.outputHint, Size: guigui.FlexibleSize(1)},
	)
	t.outputRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.outputItems, Gap: u / 2}

	t.editItems = slices.Delete(t.editItems, 0, len(t.editItems))
	t.editItems = append(t.editItems,
		guigui.LinearLayoutItem{Widget: &t.resizePanel, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.cropPanel, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &t.formatPanel, Size: guigui.FlexibleSize(1)},
	)
	t.editRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: t.editItems, Gap: u / 2}

	t.layoutItems = slices.Delete(t.layoutItems, 0, len(t.layoutItems))
	t.layoutItems = append(t.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &t.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &t.previewRow},
		guigui.LinearLayoutItem{Layout: &t.editRow},
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

// editPanel is a bordered form group that reports the form's intrinsic height
// so the parent can keep every control on screen without scrolling.
type editPanel struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	form  basicwidget.Form
}

func (p *editPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.panel)
	p.panel.SetContent(&p.form)
	p.panel.SetAutoBorder(false)
	p.panel.SetBorders(basicwidget.PanelBorders{Start: true, Top: true, End: true, Bottom: true})
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	p.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleSecondary)
	return nil
}

func (p *editPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&p.panel, widgetBounds.Bounds())
}

func (p *editPanel) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return p.form.Measure(context, constraints)
}
