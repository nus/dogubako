package app

import (
	"image"
	"strings"

	"github.com/gogpu/ui/core/checkbox"
	"github.com/gogpu/ui/core/datatable"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/core/radio"
	"github.com/gogpu/ui/core/slider"
	"github.com/gogpu/ui/core/splitview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
)

func (s *Shell) buildRoot() widget.Widget {
	host := newKeyHost(s, s.buildShell())
	return host
}

func (s *Shell) buildShell() widget.Widget {
	return splitview.New(
		splitview.First(s.buildSidebar()),
		splitview.Second(newModeHost(s)),
		splitview.OrientationOpt(splitview.Horizontal),
		splitview.FixedFirst(220),
		splitview.MinFirst(180),
		splitview.MinSecond(400),
		splitview.PainterOpt(material3.SplitViewPainter{Theme: s.theme}),
	)
}

func (s *Shell) buildSidebar() widget.Widget {
	title := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.AppTitle)
	})).FontSize(18).Bold().Align(primitives.TextAlignCenter)

	tools := listview.New(
		listview.ItemCountFn(func() int { return len(Tools) }),
		listview.FixedItemHeight(40),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.SelectedIndexSignal(s.toolSel),
		listview.PainterOpt(material3.ListViewPainter{Theme: s.theme}),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			tool := Tools[ctx.Index]
			color := widget.RGBA8(33, 33, 33, 255)
			if ctx.Selected {
				color = widget.RGBA8(47, 129, 255, 255)
			}
			t := s.txt(tool.Title(s.model.Lang())).FontSize(14).Color(color)
			if ctx.Selected {
				t = t.Bold()
			}
			return primitives.Box(t).PaddingXY(12, 8)
		}),
		listview.OnItemClick(func(index int) {
			if index < 0 || index >= len(Tools) {
				return
			}
			s.model.SetMode(Tools[index].ID)
			s.bump()
		}),
	)

	setLang := func(lang i18n.Lang) {
		s.model.SetLang(lang)
		if s.gpu != nil {
			s.gpu.SetTitle(i18n.T(s.model.Lang(), i18n.AppTitle))
		}
		s.reloadUI()
	}
	lang := primitives.HBox(
		s.btnFn(func() string { return "日本語" }, func() { setLang(i18n.JA) }, func() bool { return s.model.Lang() == i18n.JA }, nil),
		s.btnFn(func() string { return "English" }, func() { setLang(i18n.EN) }, func() bool { return s.model.Lang() == i18n.EN }, nil),
	).Gap(6)

	return primitives.Box(
		title,
		primitives.Expanded(primitives.Box(tools).PaddingXY(4, 4)),
		primitives.Box(lang).PaddingXY(8, 8),
	).
		Padding(8).
		Gap(8).
		Background(widget.RGBA8(245, 247, 250, 255))
}

func (s *Shell) buildImageTool() widget.Widget {
	before := newSourcePreview(s.rev)
	after := newDestPreview(s.rev)
	before.OnCropChanged(func(rect image.Rectangle) {
		s.model.Image().SetCrop(rect)
		s.bump()
	})
	s.rev.Get() // keep computed readers consistent
	previewSync := state.NewComputed(func() uint64 {
		v := s.rev.Get()
		img := s.model.Image()
		before.SetImage(img.SourcePreview(), img.SourceSize())
		before.SetCrop(img.Crop(), img.CropEnabled())
		if out, err := img.Processed(); err == nil && out != nil {
			after.SetImage(img.ResultPreview())
		} else {
			after.SetImage(nil)
		}
		return v
	})
	_ = previewSync

	open := s.btn(i18n.OpenFile, func() { s.startOpen() }, false, nil)
	paste := s.btn(i18n.PasteClipboard, func() { s.pasteClipboard() }, false, nil)
	hint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.InputHint, ShortcutLabel("V"))
	})).FontSize(13).Color(widget.RGBA8(90, 90, 90, 255))

	toolbar := primitives.HBox(open, paste, primitives.Expanded(hint)).Gap(8)

	beforeLabel := s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if img.HasSource() {
			sz := img.SourceSize()
			return i18n.T(s.model.Lang(), i18n.BeforeSize, sz.X, sz.Y)
		}
		return i18n.T(s.model.Lang(), i18n.Before)
	})).Bold().FontSize(14)

	afterLabel := s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return i18n.T(s.model.Lang(), i18n.AfterSize, out.Bounds().Dx(), out.Bounds().Dy(), img.Format())
		}
		return i18n.T(s.model.Lang(), i18n.After)
	})).Bold().FontSize(14)

	afterEmpty := s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.AfterEmpty)
	})).FontSize(13).Color(widget.RGBA8(120, 120, 120, 255)).Align(primitives.TextAlignCenter)

	previews := primitives.HBox(
		primitives.Expanded(primitives.VBox(beforeLabel, primitives.Expanded(before)).Gap(6)),
		primitives.Expanded(primitives.VBox(afterLabel, primitives.Expanded(after), afterEmpty).Gap(6)),
	).Gap(12)

	save := s.btn(i18n.SaveFile, func() { s.startSave() }, true, func() bool { return !s.model.Image().HasSource() })
	copyBtn := s.btn(i18n.CopyClipboard, func() { s.copyClipboard() }, false, func() bool { return !s.model.Image().HasSource() })
	outHint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.OutputHint)
	})).FontSize(13).Color(widget.RGBA8(90, 90, 90, 255))
	output := primitives.HBox(save, copyBtn, primitives.Expanded(outHint)).Gap(8)

	status := s.txt("").ContentSignal(s.computed(func() string {
		return s.model.Image().StatusText(s.model.Lang())
	})).FontSize(13)

	return primitives.VBox(
		toolbar,
		primitives.Expanded(previews),
		s.buildImageForms(),
		output,
		status,
	).Padding(12).Gap(10)
}

func (s *Shell) buildImageForms() widget.Widget {
	m3 := s.theme
	has := func() bool { return s.model.Image().HasSource() }
	cropOn := func() bool { return s.model.Image().HasSource() && s.model.Image().CropEnabled() }

	scale := s.intField(s.scaleSig, 1, 1000, func(v int) { s.model.Image().SetScalePercent(v) }, has)
	width := s.intField(s.widthSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetWidth(v) }, has)
	height := s.intField(s.heightSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetHeight(v) }, has)
	keep := s.check(i18n.KeepAspect, s.keepAsp, func() bool { return !has() }, func(v bool) { s.model.Image().SetKeepAspect(v); s.bump() })
	resetSize := s.btn(i18n.ResetSize, func() { s.model.Image().ResetSize(); s.bump() }, false, func() bool { return !has() })

	cropEn := s.check(i18n.CropEnable, s.cropOn, func() bool { return !has() }, func(v bool) { s.model.Image().SetCropEnabled(v); s.bump() })
	cropX := s.intField(s.cropXSig, 0, imageproc.MaxDimension, func(v int) { s.model.Image().SetCropX(v) }, cropOn)
	cropY := s.intField(s.cropYSig, 0, imageproc.MaxDimension, func(v int) { s.model.Image().SetCropY(v) }, cropOn)
	cropW := s.intField(s.cropWSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetCropWidth(v) }, cropOn)
	cropH := s.intField(s.cropHSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetCropHeight(v) }, cropOn)
	resetCrop := s.btn(i18n.ResetCrop, func() { s.model.Image().ResetCrop(); s.bump() }, false, func() bool { return !cropOn() })

	angle := s.intField(s.angleSig, 0, 359, func(v int) { s.model.Image().SetRotateDegrees(v) }, has)
	rot90 := s.btn(i18n.Rotate90, func() { s.model.Image().Rotate90(); s.bump() }, false, func() bool { return !has() })
	resetRot := s.btn(i18n.ResetRotate, func() { s.model.Image().ResetRotate(); s.bump() }, false, func() bool {
		return !has() || s.model.Image().RotateDegrees() == 0
	})

	format := radio.NewGroup(
		radio.Items(
			radio.ItemDef{Value: string(imageproc.FormatPNG), Label: "PNG"},
			radio.ItemDef{Value: string(imageproc.FormatJPEG), Label: "JPEG"},
		),
		radio.SelectedSignal(s.fmtSig),
		radio.DirectionOpt(radio.Horizontal),
		radio.GroupDisabledFn(func() bool { return !has() }),
		radio.GroupPainter(material3.RadioPainter{Theme: m3}),
		radio.OnChange(func(v string) {
			s.model.Image().SetFormat(imageproc.Format(v))
			s.bump()
		}),
	)
	qual := slider.New(
		slider.Min(1), slider.Max(100), slider.Step(1),
		slider.ValueSignal(s.quality),
		slider.DisabledFn(func() bool {
			return !has() || s.model.Image().Format() != imageproc.FormatJPEG
		}),
		slider.PainterOpt(material3.SliderPainter{Theme: m3}),
		slider.OnChange(func(v float32) {
			s.model.Image().SetJPEGQuality(int(v + 0.5))
			s.bump()
		}),
	)
	qualLabel := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.JPEGQuality, s.model.Image().JPEGQuality())
	})).FontSize(12)

	card := func(title i18n.Key, children ...widget.Widget) widget.Widget {
		items := []widget.Widget{
			s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), title) })).Bold().FontSize(13),
		}
		items = append(items, children...)
		return primitives.Box(items...).
			Padding(10).Gap(6).
			Background(widget.RGBA8(250, 250, 252, 255)).
			Rounded(8).
			BorderStyle(1, widget.RGBA8(220, 224, 230, 255))
	}

	return primitives.HBox(
		primitives.Expanded(card(i18n.Resize,
			s.labeled(i18n.ScalePercent, scale),
			s.labeled(i18n.WidthPx, width),
			s.labeled(i18n.HeightPx, height),
			keep, resetSize,
		)),
		primitives.Expanded(card(i18n.Crop,
			cropEn,
			s.labeled(i18n.CropX, cropX),
			s.labeled(i18n.CropY, cropY),
			s.labeled(i18n.Width, cropW),
			s.labeled(i18n.Height, cropH),
			resetCrop,
		)),
		primitives.Expanded(card(i18n.Rotate,
			s.labeled(i18n.RotateAngle, angle),
			rot90, resetRot,
		)),
		primitives.Expanded(card(i18n.Format,
			s.labeled(i18n.OutputFormat, format),
			qualLabel, qual,
		)),
	).Gap(8)
}

func (s *Shell) buildScreenshotTool() widget.Widget {
	m3 := s.theme
	busy := func() bool { return s.model.Screenshot().Capturing() }

	setMode := func(mode capture.Mode) {
		s.model.Screenshot().SetMode(mode)
		s.bump()
	}
	mode := primitives.HBox(
		s.btnFn(func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotFull) }, func() { setMode(capture.ModeFull) }, func() bool { return s.model.Screenshot().Mode() == capture.ModeFull }, busy),
		s.btnFn(func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotWindow) }, func() { setMode(capture.ModeWindow) }, func() bool { return s.model.Screenshot().Mode() == capture.ModeWindow }, busy),
		s.btnFn(func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotRegion) }, func() { setMode(capture.ModeRegion) }, func() bool { return s.model.Screenshot().Mode() == capture.ModeRegion }, busy),
	).Gap(6)
	delay := s.intField(s.delaySig, 0, maxCaptureDelaySec, func(v int) { s.model.Screenshot().SetDelaySec(v) }, func() bool { return !busy() })
	hide := s.check(i18n.ScreenshotHideWindow, s.hideWin, busy, func(v bool) { s.model.Screenshot().SetHideWindow(v) })
	cap := s.btnFn(func() string {
		if s.model.Screenshot().Capturing() {
			return i18n.T(s.model.Lang(), i18n.ScreenshotCaptureRetry)
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotCapture)
	}, func() { s.startCapture() }, func() bool { return true }, nil)
	toolbar := primitives.HBox(
		s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotMode) })).FontSize(13),
		mode,
		s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotDelay) })).FontSize(13),
		delay,
		hide,
		primitives.Expanded(primitives.Box()),
		cap,
	).Gap(8)

	listLabel := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.ScreenshotFiles)
	})).Bold().FontSize(14)
	files := listview.New(
		listview.ItemCountFn(func() int { return len(s.model.Screenshot().Files()) }),
		listview.FixedItemHeight(48),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.SelectedIndexSignal(s.shotSel),
		listview.PainterOpt(material3.ListViewPainter{Theme: m3}),
		listview.DisabledFn(busy),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			files := s.model.Screenshot().Files()
			if ctx.Index < 0 || ctx.Index >= len(files) {
				return primitives.Box()
			}
			f := files[ctx.Index]
			thumb := s.model.Screenshot().Thumbnail(f.Path)
			row := []widget.Widget{
				s.txt(f.Name).FontSize(13),
			}
			if thumb != nil {
				preview := newDestPreview(nil)
				preview.SetSource(thumb)
				row = []widget.Widget{
					primitives.Box(preview).Width(40).Height(40),
					s.txt(f.Name).FontSize(13),
				}
			}
			return primitives.HBox(row...).Gap(8).PaddingXY(8, 4).Height(48)
		}),
		listview.OnItemClick(func(index int) {
			files := s.model.Screenshot().Files()
			if index < 0 || index >= len(files) {
				return
			}
			_ = s.model.Screenshot().LoadPath(files[index].Path)
			s.bump()
		}),
	)

	previewLabel := s.txt("").ContentSignal(s.computed(func() string {
		shot := s.model.Screenshot()
		if shot.HasImage() {
			sz := shot.Size()
			return i18n.T(s.model.Lang(), i18n.ScreenshotPreviewSz, sz.X, sz.Y)
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotPreview)
	})).Bold().FontSize(14)
	preview := newDestPreview(s.rev)
	sync := state.NewComputed(func() uint64 {
		v := s.rev.Get()
		preview.SetSource(s.model.Screenshot().Image())
		return v
	})
	_ = sync
	empty := s.txt("").ContentSignal(s.computed(func() string {
		if s.model.Screenshot().HasImage() {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotEmpty)
	})).Align(primitives.TextAlignCenter).Color(widget.RGBA8(120, 120, 120, 255))

	body := primitives.HBox(
		primitives.Box(listLabel, primitives.Expanded(files)).Width(320).Gap(6),
		primitives.Expanded(primitives.VBox(previewLabel, primitives.Expanded(preview), empty).Gap(6)),
	).Gap(12)

	saveAs := s.btn(i18n.ScreenshotSaveAs, func() { s.startScreenshotSave() }, false, func() bool { return !s.model.Screenshot().HasImage() || busy() })
	copyBtn := s.btn(i18n.ScreenshotCopy, func() { s.copyScreenshot() }, false, func() bool { return !s.model.Screenshot().HasImage() || busy() })
	send := s.btn(i18n.ScreenshotSendImage, func() { s.sendScreenshotToImage() }, false, func() bool { return !s.model.Screenshot().HasImage() || busy() })
	folder := s.btn(i18n.ScreenshotShowFolder, func() { s.showScreenshotFolder() }, false, func() bool { return s.model.Screenshot().DestDir() == "" || busy() })
	dest := s.txt("").ContentSignal(s.computed(func() string {
		d := s.model.Screenshot().DestDir()
		if d == "" {
			d = "—"
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotDest, d)
	})).FontSize(12)
	status := s.txt("").ContentSignal(s.computed(func() string {
		return s.model.Screenshot().StatusText(s.model.Lang())
	})).FontSize(13)

	return primitives.VBox(
		toolbar,
		primitives.Expanded(body),
		primitives.HBox(saveAs, copyBtn, send, folder, primitives.Expanded(dest)).Gap(8),
		status,
	).Padding(12).Gap(10)
}

func (s *Shell) buildAndroidTool() widget.Widget {
	m3 := s.theme
	s.model.Android().EnsureLoaded()
	busy := func() bool { return s.model.Android().Busy() }
	online := func() bool {
		for _, d := range s.model.Android().Devices() {
			if d.Serial == s.model.Android().Serial() && d.Online() {
				return true
			}
		}
		return false
	}

	deviceLabel := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.AndroidDevices)
	})).Bold().FontSize(14)
	devices := listview.New(
		listview.ItemCountFn(func() int { return len(s.model.Android().Devices()) }),
		listview.FixedItemHeight(32),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.SelectedIndexSignal(s.devSel),
		listview.PainterOpt(material3.ListViewPainter{Theme: m3}),
		listview.DisabledFn(busy),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			devs := s.model.Android().Devices()
			if ctx.Index < 0 || ctx.Index >= len(devs) {
				return primitives.Box()
			}
			return primitives.Box(s.txt(devs[ctx.Index].Label()).FontSize(13)).PaddingXY(8, 4)
		}),
		listview.OnItemClick(func(index int) {
			devs := s.model.Android().Devices()
			if index < 0 || index >= len(devs) {
				return
			}
			s.model.Android().SelectDevice(devs[index].Serial)
			s.bump()
		}),
	)

	refresh := s.btn(i18n.AndroidRefresh, func() { s.model.Android().Reload(); s.bump() }, false, busy)
	up := s.btn(i18n.AndroidUp, func() { s.model.Android().GoUp(); s.bump() }, false, func() bool { return busy() || !s.model.Android().CanGoUp() })
	path := s.txt("").ContentSignal(s.computed(func() string {
		return s.model.Android().Root()
	})).FontSize(13)
	toolbar := primitives.HBox(refresh, up, primitives.Expanded(path)).Gap(8)

	table := datatable.New(
		datatable.Columns([]datatable.Column{
			{Key: "name", Title: i18n.T(s.model.Lang(), i18n.AndroidColName), Sortable: true, MinWidth: 180},
			{Key: "size", Title: i18n.T(s.model.Lang(), i18n.AndroidColSize), Sortable: true, Width: 110},
			{Key: "mod", Title: i18n.T(s.model.Lang(), i18n.AndroidColModified), Sortable: true, Width: 170},
		}),
		datatable.RowCountFn(func() int { return len(s.model.Android().Rows()) }),
		datatable.RowHeight(28),
		datatable.SelectionModeOpt(datatable.SelectionSingle),
		datatable.SelectedRowSignal(s.adbSel),
		datatable.DisabledFn(func() bool { return busy() || !online() }),
		datatable.PainterOpt(material3.DataTablePainter{Theme: m3}),
		datatable.CellValue(func(row int, col string) string {
			rows := s.model.Android().Rows()
			if row < 0 || row >= len(rows) {
				return ""
			}
			r := rows[row]
			switch col {
			case "name":
				mark := androidFileIcon
				exp := ""
				if r.Entry.IsDir {
					mark = androidFolderIcon
					if r.Expanded {
						exp = androidExpandOpen + " "
					} else {
						exp = androidExpandClosed + " "
					}
				}
				return strings.Repeat("  ", r.Depth) + exp + mark + " " + r.Entry.Name
			case "size":
				return formatFileSize(r.Entry.Size, r.Entry.IsDir)
			case "mod":
				return formatFileTime(r.Entry.ModTime)
			default:
				return ""
			}
		}),
		datatable.OnSort(func(col string, ascending bool) {
			id := androidSortName
			switch col {
			case "size":
				id = androidSortSize
			case "mod":
				id = androidSortMod
			}
			m := s.model.Android()
			if m.SortCol() != id {
				m.ToggleSort(id)
				if !ascending && !m.SortDesc() {
					m.ToggleSort(id)
				}
			} else if m.SortDesc() == ascending {
				m.ToggleSort(id)
			}
			s.bump()
		}),
		datatable.OnRowSelect(func(row int) {
			rows := s.model.Android().Rows()
			if row < 0 || row >= len(rows) {
				return
			}
			r := rows[row]
			s.model.Android().SelectPath(r.Entry.Path)
			if r.Entry.IsDir {
				s.model.Android().ToggleExpand(r.Entry.Path)
			}
			s.bump()
		}),
	)
	empty := s.txt("").ContentSignal(s.computed(func() string {
		if len(s.model.Android().Rows()) > 0 {
			return ""
		}
		if len(s.model.Android().Devices()) == 0 {
			return i18n.T(s.model.Lang(), i18n.AndroidNoDevices)
		}
		return i18n.T(s.model.Lang(), i18n.AndroidEmpty)
	})).Align(primitives.TextAlignCenter).Color(widget.RGBA8(120, 120, 120, 255))

	pull := s.btn(i18n.AndroidPull, func() { s.startAndroidPull() }, false, func() bool { return busy() || !s.model.Android().HasSelection() })
	pushFile := s.btn(i18n.AndroidPushFile, func() { s.startAndroidPush(false) }, false, func() bool { return busy() || !online() })
	pushDir := s.btn(i18n.AndroidPushFolder, func() { s.startAndroidPush(true) }, false, func() bool { return busy() || !online() })
	hint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.AndroidHint)
	})).FontSize(12).Color(widget.RGBA8(90, 90, 90, 255))
	status := s.txt("").ContentSignal(s.computed(func() string {
		return s.model.Android().StatusText(s.model.Lang())
	})).FontSize(13)

	return primitives.VBox(
		deviceLabel,
		primitives.Box(devices).Height(96),
		toolbar,
		primitives.Expanded(primitives.VBox(primitives.Expanded(table), empty)),
		primitives.HBox(pull, pushFile, pushDir, primitives.Expanded(hint)).Gap(8),
		status,
	).Padding(12).Gap(8)
}

func (s *Shell) labeled(key i18n.Key, w widget.Widget) widget.Widget {
	return primitives.HBox(
		s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), key) })).FontSize(12).Color(widget.RGBA8(70, 70, 70, 255)),
		primitives.Expanded(w),
	).Gap(6)
}

func (s *Shell) intField(sig state.Signal[string], minV, maxV int, apply func(int), enabled func() bool) widget.Widget {
	tf := textfield.New(
		textfield.ValueSignal(sig),
		textfield.InputTypeOpt(textfield.TypeNumber),
		textfield.DisabledFn(func() bool { return enabled != nil && !enabled() }),
		textfield.PainterOpt(material3.TextFieldPainter{Theme: s.theme}),
		textfield.OnSubmit(func(text string) {
			n, ok := parseInt(strings.TrimSpace(text))
			if !ok {
				return
			}
			if n < minV {
				n = minV
			}
			if n > maxV {
				n = maxV
			}
			apply(n)
			s.bump()
		}),
		textfield.OnChange(func(text string) {
			n, ok := parseInt(strings.TrimSpace(text))
			if !ok {
				return
			}
			if n < minV || n > maxV {
				return
			}
			apply(n)
		}),
	)
	return primitives.Box(tf).Width(fieldWidth).Height(36)
}

func (s *Shell) txt(content string) *primitives.TextWidget {
	return primitives.Text(content).FontFamily(cjkembed.FamilyName)
}

func (s *Shell) check(key i18n.Key, sig state.Signal[bool], disabled func() bool, onToggle func(bool)) widget.Widget {
	box := checkbox.New(
		checkbox.CheckedSignal(sig),
		checkbox.DisabledFn(func() bool { return disabled != nil && disabled() }),
		checkbox.PainterOpt(material3.CheckboxPainter{Theme: s.theme}),
		checkbox.OnToggle(onToggle),
	)
	return primitives.HBox(
		box,
		s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), key) })).FontSize(13),
	).Gap(6)
}
