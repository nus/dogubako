package app

import (
	"image"
	"strings"

	"github.com/gogpu/ui/core/checkbox"
	"github.com/gogpu/ui/core/datatable"
	"github.com/gogpu/ui/core/listview"
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
	side := primitives.Box(s.buildSidebar()).
		Width(sidebarWidth).
		MinWidthValue(sidebarWidth).
		MaxWidthValue(sidebarWidth).
		Background(widget.RGBA8(245, 247, 250, 255)).
		BorderStyle(1, widget.RGBA8(220, 224, 230, 255))
	return primitives.HBox(side, primitives.Expanded(newModeHost(s)))
}

func (s *Shell) buildSidebar() widget.Widget {
	title := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.AppTitle)
	})).FontSize(14).Bold().Align(primitives.TextAlignCenter)

	setLang := func(lang i18n.Lang) {
		if lang == i18n.JA && !cjkembed.Available() {
			return
		}
		s.model.SetLang(lang)
		if s.gpu != nil {
			s.gpu.SetTitle(i18n.T(s.model.Lang(), i18n.AppTitle))
		}
		s.reloadUI()
	}

	items := make([]widget.Widget, 0, len(Tools))
	for i, tool := range Tools {
		i, tool := i, tool
		items = append(items, s.navItem(
			func() string { return tool.Title(s.model.Lang()) },
			func() bool { return s.model.Mode() == tool.ID },
			func() {
				s.model.SetMode(tool.ID)
				s.toolSel.Set(i)
				s.bump()
			},
		))
	}

	lang := s.segBarFill(
		segItem{
			label: func() string {
				if cjkembed.Available() {
					return "日本語"
				}
				return "Japanese"
			},
			selected: func() bool { return s.model.Lang() == i18n.JA },
			onClick:  func() { setLang(i18n.JA) },
			disabled: func() bool { return !cjkembed.Available() },
		},
		segItem{
			label:    func() string { return "English" },
			selected: func() bool { return s.model.Lang() == i18n.EN },
			onClick:  func() { setLang(i18n.EN) },
		},
	)

	return primitives.Box(
		primitives.Box(title).Height(unit),
		primitives.Expanded(primitives.VBox(items...).Gap(2).PaddingXY(4, 4)),
		lang,
	).Padding(6).Gap(6)
}

func (s *Shell) buildImageTool() widget.Widget {
	open := s.btn(i18n.OpenFile, func() { s.startOpen() }, false, nil)
	paste := s.btn(i18n.PasteClipboard, func() { s.pasteClipboard() }, false, nil)
	hint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.InputHint, ShortcutLabel("V"))
	})).FontSize(13).Color(widget.RGBA8(90, 90, 90, 255))
	toolbar := primitives.HBox(open, paste, primitives.Expanded(hint)).Gap(8).CrossAlign(primitives.CrossAxisCenter)

	tabs := make([]segItem, 0, len(ImageFeatures))
	for _, feature := range ImageFeatures {
		feature := feature
		tabs = append(tabs, segItem{
			label:    func() string { return feature.Title(s.model.Lang()) },
			selected: func() bool { return s.model.Image().Feature() == feature },
			onClick: func() {
				s.model.Image().SetFeature(feature)
				s.bump()
			},
		})
	}
	features := s.segBar(tabs...)

	save := s.btn(i18n.SaveFile, func() { s.startSave() }, true, func() bool { return !s.model.Image().HasSource() })
	copyBtn := s.btn(i18n.CopyClipboard, func() { s.copyClipboard() }, false, func() bool { return !s.model.Image().HasSource() })
	outHint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.OutputHint)
	})).FontSize(13).Color(widget.RGBA8(90, 90, 90, 255))
	output := primitives.HBox(
		save, copyBtn,
		s.imageFormatSeg(),
		s.imageQualitySlider(),
		primitives.Expanded(outHint),
	).Gap(8).CrossAlign(primitives.CrossAxisCenter)

	status := s.txt("").ContentSignal(s.computed(func() string {
		return s.model.Image().StatusText(s.model.Lang())
	})).FontSize(13)

	return primitives.VBox(
		toolbar,
		features,
		primitives.Expanded(newImageFeatureHost(s)),
		output,
		status,
	).Padding(12).Gap(12)
}

func (s *Shell) buildClipFeature() widget.Widget {
	before := newSourcePreview(s.rev)
	after := newDestPreview(s.rev)
	after.SetEmptyHint(func() string {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.AfterEmpty)
	})
	before.OnCropChanged(func(rect image.Rectangle) {
		s.model.Image().SetCrop(rect)
		s.bump()
	})
	before.SetProvider(func() sourcePreviewFrame {
		img := s.model.Image()
		return sourcePreviewFrame{
			image:       img.SourcePreview(),
			size:        img.SourceSize(),
			crop:        img.Crop(),
			cropEnabled: img.CropEnabled(),
		}
	})
	after.SetProvider(func() image.Image {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return img.ResultPreview()
		}
		return nil
	})

	beforeLabel := s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if img.HasSource() {
			sz := img.SourceSize()
			return i18n.T(s.model.Lang(), i18n.BeforeSize, sz.X, sz.Y)
		}
		return i18n.T(s.model.Lang(), i18n.Before)
	})).Bold().FontSize(14)

	afterLabel := s.imageAfterLabel()
	previews := primitives.HBox(
		primitives.Expanded(primitives.VBox(beforeLabel, primitives.Expanded(before)).Gap(6)),
		primitives.Expanded(primitives.VBox(afterLabel, primitives.Expanded(after)).Gap(6)),
	).Gap(unit)

	return primitives.VBox(
		primitives.Expanded(previews),
		s.buildClipForms(),
	).Gap(12)
}

func (s *Shell) buildPaintFeature() widget.Widget {
	canvas := newPaintCanvas(s.rev)
	canvas.SetEmptyHint(func() string {
		if s.model.Image().HasSource() {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.AfterEmpty)
	})
	canvas.SetProvider(func() paintCanvasFrame {
		img := s.model.Image()
		return paintCanvasFrame{
			image:   img.SourcePreview(),
			size:    img.SourceSize(),
			overlay: img.PaintLayer(),
		}
	})
	canvas.OnStroke(func(pt image.Point) {
		s.model.Image().BeginPaintStroke(pt)
		s.rev.Set(s.rev.Get() + 1)
	}, func(pt image.Point) {
		s.model.Image().ContinuePaintStroke(pt)
		s.rev.Set(s.rev.Get() + 1)
	}, func() {
		s.model.Image().EndPaintStroke()
		s.bump()
	})

	after := newDestPreview(s.rev)
	after.SetEmptyHint(func() string {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.AfterEmpty)
	})
	after.SetProvider(func() image.Image {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return img.ResultPreview()
		}
		return nil
	})

	canvasLabel := s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if img.HasSource() {
			sz := img.SourceSize()
			return i18n.T(s.model.Lang(), i18n.PaintCanvasSize, sz.X, sz.Y)
		}
		return i18n.T(s.model.Lang(), i18n.PaintCanvas)
	})).Bold().FontSize(14)
	afterLabel := s.imageAfterLabel()
	previews := primitives.HBox(
		primitives.Expanded(primitives.VBox(canvasLabel, primitives.Expanded(canvas)).Gap(6)),
		primitives.Expanded(primitives.VBox(afterLabel, primitives.Expanded(after)).Gap(6)),
	).Gap(unit)

	return primitives.VBox(
		primitives.Expanded(previews),
		s.buildPaintForms(),
	).Gap(12)
}

func (s *Shell) imageAfterLabel() widget.Widget {
	return s.txt("").ContentSignal(s.computed(func() string {
		img := s.model.Image()
		if out, err := img.Processed(); err == nil && out != nil {
			return i18n.T(s.model.Lang(), i18n.AfterSize, out.Bounds().Dx(), out.Bounds().Dy(), img.Format())
		}
		return i18n.T(s.model.Lang(), i18n.After)
	})).Bold().FontSize(14)
}

func (s *Shell) imageFormatSeg() widget.Widget {
	has := func() bool { return !s.model.Image().HasSource() }
	return primitives.Box(s.segBar(
		segItem{
			label:    func() string { return "PNG" },
			selected: func() bool { return s.model.Image().Format() == imageproc.FormatPNG },
			disabled: has,
			onClick: func() {
				s.model.Image().SetFormat(imageproc.FormatPNG)
				s.bump()
			},
		},
		segItem{
			label:    func() string { return "JPEG" },
			selected: func() bool { return s.model.Image().Format() == imageproc.FormatJPEG },
			disabled: has,
			onClick: func() {
				s.model.Image().SetFormat(imageproc.FormatJPEG)
				s.bump()
			},
		},
	))
}

func (s *Shell) imageQualitySlider() widget.Widget {
	qual := newHSlider(s, s.quality, 1, 100, 1, func() bool {
		return !s.model.Image().HasSource() || s.model.Image().Format() != imageproc.FormatJPEG
	}, func(v float32) {
		s.model.Image().SetJPEGQuality(int(v + 0.5))
	})
	label := s.txt("").ContentSignal(state.NewComputed(func() string {
		_ = s.rev.Get()
		q := int(s.quality.Get() + 0.5)
		return i18n.T(s.model.Lang(), i18n.JPEGQuality, q)
	}, s.rev.AsReadonly(), s.quality.AsReadonly())).FontSize(13).Color(widget.RGBA8(70, 70, 70, 255))
	return primitives.HBox(
		label,
		primitives.Box(qual).Width(fieldWidth).Height(formRowH),
	).Gap(8).Height(formRowH).CrossAlign(primitives.CrossAxisCenter)
}

func (s *Shell) buildClipForms() widget.Widget {
	has := func() bool { return s.model.Image().HasSource() }
	cropOn := func() bool { return s.model.Image().HasSource() && s.model.Image().CropEnabled() }

	scale := s.intField(s.scaleSig, 1, 1000, func(v int) { s.model.Image().SetScalePercent(v) }, has)
	width := s.intField(s.widthSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetWidth(v) }, has)
	height := s.intField(s.heightSig, 1, imageproc.MaxDimension, func(v int) { s.model.Image().SetHeight(v) }, has)
	keep := s.toggle(s.keepAsp, func() bool { return !has() }, func(v bool) { s.model.Image().SetKeepAspect(v); s.bump() })
	resetSize := s.btn(i18n.ResetSize, func() { s.model.Image().ResetSize(); s.bump() }, false, func() bool { return !has() })

	cropEn := s.toggle(s.cropOn, func() bool { return !has() }, func(v bool) { s.model.Image().SetCropEnabled(v); s.bump() })
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

	return newEqualRow(8,
		s.formPanel(i18n.Resize,
			s.formRow(i18n.ScalePercent, scale),
			s.formRow(i18n.WidthPx, width),
			s.formRow(i18n.HeightPx, height),
			s.formRow(i18n.KeepAspect, keep),
			s.formEnd(resetSize),
		),
		s.formPanel(i18n.Crop,
			s.formRow(i18n.CropEnable, cropEn),
			s.formRow(i18n.CropX, cropX),
			s.formRow(i18n.CropY, cropY),
			s.formRow(i18n.Width, cropW),
			s.formRow(i18n.Height, cropH),
			s.formEnd(resetCrop),
		),
		s.formPanel(i18n.Rotate,
			s.formRow(i18n.RotateAngle, angle),
			s.formEnd(rot90),
			s.formEnd(resetRot),
		),
	)
}

func (s *Shell) buildPaintForms() widget.Widget {
	has := func() bool { return s.model.Image().HasSource() }
	brushSize := newHSlider(s, s.brushSig, 1, float32(imageproc.MaxBrushSize), 1, func() bool {
		return !has()
	}, func(v float32) {
		s.model.Image().SetBrushSize(int(v + 0.5))
	})
	r := s.intField(s.paintRSig, 0, 255, func(v int) {
		c := s.model.Image().PaintColor()
		c.R = uint8(v)
		s.model.Image().SetPaintColor(c)
	}, has)
	g := s.intField(s.paintGSig, 0, 255, func(v int) {
		c := s.model.Image().PaintColor()
		c.G = uint8(v)
		s.model.Image().SetPaintColor(c)
	}, has)
	b := s.intField(s.paintBSig, 0, 255, func(v int) {
		c := s.model.Image().PaintColor()
		c.B = uint8(v)
		s.model.Image().SetPaintColor(c)
	}, has)

	swatches := make([]widget.Widget, 0, len(paintPalette))
	for _, c := range paintPalette {
		swatches = append(swatches, s.colorSwatch(c))
	}

	return newEqualRow(8,
		s.formPanel(i18n.ImagePaint,
			s.formRow(i18n.PaintTool, primitives.Box(s.segBar(
				segItem{
					label:    func() string { return i18n.T(s.model.Lang(), i18n.PaintBrush) },
					selected: func() bool { return s.model.Image().PaintTool() == PaintBrush },
					disabled: func() bool { return !has() },
					onClick: func() {
						s.model.Image().SetPaintTool(PaintBrush)
						s.bump()
					},
				},
				segItem{
					label:    func() string { return i18n.T(s.model.Lang(), i18n.PaintEraser) },
					selected: func() bool { return s.model.Image().PaintTool() == PaintEraser },
					disabled: func() bool { return !has() },
					onClick: func() {
						s.model.Image().SetPaintTool(PaintEraser)
						s.bump()
					},
				},
			))),
			s.formSlider(s.brushSig, i18n.PaintSize, brushSize),
			s.formEnd(s.btn(i18n.PaintUndo, func() { s.model.Image().UndoPaint(); s.bump() }, false, func() bool {
				return !s.model.Image().CanUndoPaint()
			})),
			s.formEnd(s.btn(i18n.PaintClear, func() { s.model.Image().ClearPaint(); s.bump() }, false, func() bool {
				return !s.model.Image().HasPaint()
			})),
		),
		s.formPanel(i18n.PaintColor,
			primitives.HBox(swatches...).Gap(4).Height(formRowH).CrossAlign(primitives.CrossAxisCenter),
			s.formRow(i18n.PaintR, r),
			s.formRow(i18n.PaintG, g),
			s.formRow(i18n.PaintB, b),
		),
	)
}

func (s *Shell) buildScreenshotTool() widget.Widget {
	m3 := s.theme
	busy := func() bool { return s.model.Screenshot().Capturing() }

	setMode := func(mode capture.Mode) {
		s.model.Screenshot().SetMode(mode)
		s.bump()
	}
	mode := s.segBar(
		segItem{
			label:    func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotFull) },
			selected: func() bool { return s.model.Screenshot().Mode() == capture.ModeFull },
			disabled: busy,
			onClick:  func() { setMode(capture.ModeFull) },
		},
		segItem{
			label:    func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotWindow) },
			selected: func() bool { return s.model.Screenshot().Mode() == capture.ModeWindow },
			disabled: busy,
			onClick:  func() { setMode(capture.ModeWindow) },
		},
		segItem{
			label:    func() string { return i18n.T(s.model.Lang(), i18n.ScreenshotRegion) },
			selected: func() bool { return s.model.Screenshot().Mode() == capture.ModeRegion },
			disabled: busy,
			onClick:  func() { setMode(capture.ModeRegion) },
		},
	)
	delay := s.intField(s.delaySig, 0, maxCaptureDelaySec, func(v int) { s.model.Screenshot().SetDelaySec(v) }, func() bool { return !busy() })
	hide := s.toggle(s.hideWin, busy, func(v bool) { s.model.Screenshot().SetHideWindow(v) })
	cap := s.btnFn(func() string {
		if s.model.Screenshot().Capturing() {
			return i18n.T(s.model.Lang(), i18n.ScreenshotCaptureRetry)
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotCapture)
	}, func() { s.startCapture() }, func() bool { return true }, nil)
	// 0.5 CJK em (~7px) between 対象 and 画面全体. Other toolbar gaps stay 8px.
	modeGroup := primitives.HBox(s.toolbarLabel(i18n.ScreenshotMode), mode).
		Gap(7).CrossAlign(primitives.CrossAxisCenter)
	delayGroup := primitives.HBox(s.toolbarLabel(i18n.ScreenshotDelay), delay).
		Gap(8).CrossAlign(primitives.CrossAxisCenter)
	hideGroup := primitives.HBox(hide, s.toolbarLabel(i18n.ScreenshotHideWindow)).
		Gap(6)
	toolbar := primitives.HBox(
		modeGroup,
		delayGroup,
		hideGroup,
		primitives.Expanded(primitives.Box()),
		cap,
	).Gap(8).CrossAlign(primitives.CrossAxisCenter)

	listLabel := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.ScreenshotFiles)
	})).Bold().FontSize(14)
	files := listview.New(
		listview.ItemCountFn(func() int { return len(s.model.Screenshot().Files()) }),
		listview.FixedItemHeight(48),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.SelectedIndexSignal(s.shotSel),
		listview.PainterOpt(newQuietListPainter(m3)),
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
					primitives.Box(preview).Width(48).Height(48),
					s.txt(f.Name).FontSize(13),
				}
			}
			return primitives.HBox(row...).Gap(8).PaddingXY(8, 0).Height(48).CrossAlign(primitives.CrossAxisCenter)
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
	preview.SetEmptyHint(func() string {
		if s.model.Screenshot().HasImage() {
			return ""
		}
		return i18n.T(s.model.Lang(), i18n.ScreenshotEmpty)
	})
	preview.SetProvider(func() image.Image {
		return s.model.Screenshot().Preview()
	})

	listCol := primitives.Box(
		listLabel,
		primitives.Expanded(primitives.Box(files).BorderStyle(1, widget.RGBA8(220, 224, 230, 255))),
	).Width(screenshotListW).MinWidthValue(screenshotListW).MaxWidthValue(screenshotListW).Gap(6)

	body := primitives.HBox(
		listCol,
		primitives.Expanded(primitives.VBox(previewLabel, primitives.Expanded(preview)).Gap(6)),
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
		primitives.HBox(saveAs, copyBtn, send, folder, primitives.Expanded(dest)).Gap(8).CrossAlign(primitives.CrossAxisCenter),
		status,
	).Padding(12).Gap(12)
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
		listview.FixedItemHeight(unit),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.SelectedIndexSignal(s.devSel),
		listview.PainterOpt(newQuietListPainter(m3)),
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
	toolbar := primitives.HBox(refresh, up, primitives.Expanded(path)).Gap(8).CrossAlign(primitives.CrossAxisCenter)

	adbScrollY := state.NewSignal[float32](0)
	table := datatable.New(
		datatable.Columns([]datatable.Column{
			{Key: "name", Title: i18n.T(s.model.Lang(), i18n.AndroidColName), Sortable: true, MinWidth: 180},
			{Key: "size", Title: i18n.T(s.model.Lang(), i18n.AndroidColSize), Sortable: true, Width: 110},
			{Key: "mod", Title: i18n.T(s.model.Lang(), i18n.AndroidColModified), Sortable: true, Width: 170},
		}),
		datatable.RowCountFn(func() int { return len(s.model.Android().Rows()) }),
		datatable.RowHeight(androidTableRowHeight),
		datatable.SelectionModeOpt(datatable.SelectionSingle),
		datatable.SelectedRowSignal(s.adbSel),
		datatable.ScrollYSignal(adbScrollY),
		datatable.DisabledFn(func() bool { return busy() || !online() }),
		datatable.PainterOpt(newQuietTablePainter(m3, func(row int) (int, bool, bool, bool) {
			rows := s.model.Android().Rows()
			if row < 0 || row >= len(rows) {
				return 0, false, false, false
			}
			r := rows[row]
			return r.Depth, r.Entry.IsDir, r.Expanded, true
		})),
		datatable.CellValue(func(row int, col string) string {
			rows := s.model.Android().Rows()
			if row < 0 || row >= len(rows) {
				return ""
			}
			r := rows[row]
			switch col {
			case "name":
				return r.Entry.Name
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
			activateAndroidTreeRow(s.model.Android(), row)
			s.bump()
		}),
	)
	tree := newAndroidTreeTable(s, table, adbScrollY)

	pull := s.btn(i18n.AndroidPull, func() { s.startAndroidPull() }, false, func() bool { return busy() || !s.model.Android().HasSelection() })
	pushFile := s.btn(i18n.AndroidPushFile, func() { s.startAndroidPush(false) }, false, func() bool { return busy() || !online() })
	pushDir := s.btn(i18n.AndroidPushFolder, func() { s.startAndroidPush(true) }, false, func() bool { return busy() || !online() })
	hint := s.txt("").ContentSignal(s.computed(func() string {
		return i18n.T(s.model.Lang(), i18n.AndroidHint)
	})).FontSize(12).Color(widget.RGBA8(90, 90, 90, 255))
	status := s.txt("").ContentSignal(s.computed(func() string {
		if msg := s.model.Android().StatusText(s.model.Lang()); msg != "" {
			return msg
		}
		if len(s.model.Android().Rows()) > 0 {
			return ""
		}
		if len(s.model.Android().Devices()) == 0 {
			return i18n.T(s.model.Lang(), i18n.AndroidNoDevices)
		}
		return i18n.T(s.model.Lang(), i18n.AndroidEmpty)
	})).FontSize(13)

	return primitives.VBox(
		deviceLabel,
		primitives.Box(devices).Height(deviceListH).BorderStyle(1, widget.RGBA8(220, 224, 230, 255)),
		toolbar,
		primitives.Expanded(tree),
		primitives.HBox(pull, pushFile, pushDir, primitives.Expanded(hint)).Gap(8).CrossAlign(primitives.CrossAxisCenter),
		status,
	).Padding(12).Gap(12)
}

func (s *Shell) formPanel(title i18n.Key, children ...widget.Widget) widget.Widget {
	items := []widget.Widget{
		s.txt("").ContentSignal(s.computed(func() string { return i18n.T(s.model.Lang(), title) })).Bold().FontSize(13),
	}
	items = append(items, children...)
	return primitives.Box(items...).
		Padding(8).Gap(2).
		Background(widget.RGBA8(250, 250, 252, 255)).
		Rounded(4).
		BorderStyle(1, widget.RGBA8(220, 224, 230, 255))
}

func (s *Shell) formRow(key i18n.Key, control widget.Widget) widget.Widget {
	return primitives.HBox(
		primitives.Expanded(s.txt("").ContentSignal(s.computed(func() string {
			return i18n.T(s.model.Lang(), key)
		})).FontSize(13).Color(widget.RGBA8(70, 70, 70, 255))),
		control,
	).Gap(8).Height(formRowH).CrossAlign(primitives.CrossAxisCenter)
}

func (s *Shell) formEnd(control widget.Widget) widget.Widget {
	return primitives.HBox(
		primitives.Expanded(primitives.Box()),
		control,
	).Height(formRowH).CrossAlign(primitives.CrossAxisCenter)
}

func (s *Shell) formSlider(sig state.Signal[float32], key i18n.Key, control widget.Widget) widget.Widget {
	label := s.txt("").ContentSignal(state.NewComputed(func() string {
		_ = s.rev.Get()
		v := int(sig.Get() + 0.5)
		return i18n.T(s.model.Lang(), key, v)
	}, s.rev.AsReadonly(), sig.AsReadonly())).FontSize(13).Color(widget.RGBA8(70, 70, 70, 255))
	return primitives.HBox(
		primitives.Expanded(label),
		primitives.Box(control).Width(fieldWidth).Height(formRowH),
	).Gap(8).Height(formRowH).CrossAlign(primitives.CrossAxisCenter)
}

func (s *Shell) intField(sig state.Signal[string], minV, maxV int, apply func(int), enabled func() bool) widget.Widget {
	disabled := func() bool { return enabled != nil && !enabled() }
	tf := textfield.New(
		textfield.ValueSignal(sig),
		textfield.InputTypeOpt(textfield.TypeNumber),
		textfield.DisabledFn(disabled),
		textfield.PainterOpt(newCompactTFPainter(s.theme)),
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
	nudge := func(delta int) {
		apply(stepIntValue(sig.Get(), minV, maxV, delta))
		s.bump()
	}
	spins := newSpinButtons(s, disabled, func() { nudge(1) }, func() { nudge(-1) })
	return primitives.HBox(
		primitives.Expanded(primitives.Box(tf).Height(fieldH)),
		primitives.Box(spins).Width(spinBtnW).MinWidthValue(spinBtnW).MaxWidthValue(spinBtnW).Height(fieldH),
	).Width(fieldWidth).Height(fieldH).Gap(2)
}

func (s *Shell) txt(content string) *primitives.TextWidget {
	return primitives.Text(content).FontFamily(cjkembed.FamilyName)
}

func (s *Shell) toggle(sig state.Signal[bool], disabled func() bool, onToggle func(bool)) widget.Widget {
	box := checkbox.New(
		checkbox.CheckedSignal(sig),
		checkbox.DisabledFn(func() bool { return disabled != nil && disabled() }),
		checkbox.PainterOpt(material3.CheckboxPainter{Theme: s.theme}),
		checkbox.OnToggle(onToggle),
	)
	// Padding so layout height matches toolbar buttons; the painter centers
	// the 18px box inside that height.
	return box.Padding((btnHeight - 18) / 2)
}
