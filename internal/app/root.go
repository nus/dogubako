package app

import (
	"errors"
	"path/filepath"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/clipboard"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/dialog"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
	"github.com/nus/dogubako/internal/userdir"
)

var EnvKeyModel = guigui.GenerateEnvKey()

// Root is the application shell: a left tool menu and a main tool panel.
type Root struct {
	guigui.DefaultWidget

	background     basicwidget.Background
	sidebar        Sidebar
	imageTool      ImageTool
	screenshotTool ScreenshotTool

	model Model

	pendingOpen           <-chan dialog.FileResult
	pendingSave           <-chan dialog.FileResult
	pendingScreenshotSave <-chan dialog.FileResult
	pendingCapture        <-chan capture.Result
	captureMinimized      bool

	layoutItems []guigui.LinearLayoutItem
}

func (r *Root) Env(context *guigui.Context, key guigui.EnvKey, source *guigui.EnvSource) (any, bool) {
	switch key {
	case EnvKeyModel:
		return &r.model, true
	default:
		return nil, false
	}
}

func (r *Root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(string(r.model.Mode()))
	w.WriteString(string(r.model.Lang()))
	w.WriteUint64(r.model.Image().Generation())
	w.WriteUint64(r.model.Screenshot().Generation())
	w.WriteBool(r.pendingCapture != nil)
}

func (r *Root) contentWidget() guigui.Widget {
	switch r.model.Mode() {
	case ToolScreenshot:
		return &r.screenshotTool
	default:
		return &r.imageTool
	}
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.sidebar)
	if content := r.contentWidget(); content != nil {
		adder.AddWidget(content)
	}
	context.SetButtonInputReceptive(r, true)
	ebiten.SetWindowTitle(i18n.T(r.model.Lang(), i18n.AppTitle))

	r.imageTool.OnOpenFile(func(context *guigui.Context) {
		r.startOpen()
	})
	r.imageTool.OnSaveFile(func(context *guigui.Context) {
		r.startSave()
	})
	r.imageTool.OnPaste(func(context *guigui.Context) {
		r.pasteClipboard()
	})
	r.imageTool.OnCopy(func(context *guigui.Context) {
		r.copyClipboard()
	})
	r.screenshotTool.OnCapture(func(context *guigui.Context) {
		r.startCapture()
	})
	r.screenshotTool.OnSaveAs(func(context *guigui.Context) {
		r.startScreenshotSave()
	})
	r.screenshotTool.OnCopy(func(context *guigui.Context) {
		r.copyScreenshot()
	})
	r.screenshotTool.OnSendToImage(func(context *guigui.Context) {
		r.sendScreenshotToImage()
	})
	r.screenshotTool.OnShowFolder(func(context *guigui.Context) {
		r.showScreenshotFolder()
	})
	return nil
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(context)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	r.layoutItems = append(r.layoutItems,
		guigui.LinearLayoutItem{
			Widget: &r.sidebar,
			Size:   guigui.FixedSize(8 * u),
		},
		guigui.LinearLayoutItem{
			Widget: r.contentWidget(),
			Size:   guigui.FlexibleSize(1),
		},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.layoutItems,
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *Root) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	r.drainDialogs()
	r.drainCapture()
	if files := ebiten.DroppedFiles(); files != nil && r.model.Mode() == ToolImage {
		_ = r.model.Image().LoadDropped(files)
	}
	return nil
}

func (r *Root) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !shortcutModifierPressed(context) {
		return guigui.HandleInputResult{}
	}
	switch r.model.Mode() {
	case ToolScreenshot:
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyS):
			_ = r.model.Screenshot().SaveDefault()
			return guigui.HandleInputByWidget(r)
		case inpututil.IsKeyJustPressed(ebiten.KeyC):
			r.copyScreenshot()
			return guigui.HandleInputByWidget(r)
		}
	default:
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyO):
			r.startOpen()
			return guigui.HandleInputByWidget(r)
		case inpututil.IsKeyJustPressed(ebiten.KeyS):
			r.startSave()
			return guigui.HandleInputByWidget(r)
		case inpututil.IsKeyJustPressed(ebiten.KeyV):
			r.pasteClipboard()
			return guigui.HandleInputByWidget(r)
		case inpututil.IsKeyJustPressed(ebiten.KeyC):
			r.copyClipboard()
			return guigui.HandleInputByWidget(r)
		}
	}
	return guigui.HandleInputResult{}
}

func (r *Root) startOpen() {
	if r.pendingOpen != nil {
		return
	}
	lang := r.model.Lang()
	r.pendingOpen = dialog.OpenFileAsync(i18n.T(lang, i18n.DialogOpen), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImages),
		Extensions:  []string{"png", "jpg", "jpeg", "gif"},
	})
}

func (r *Root) startSave() {
	if r.pendingSave != nil || !r.model.Image().HasSource() {
		return
	}
	ext := "png"
	if r.model.Image().Format() == imageproc.FormatJPEG {
		ext = "jpg"
	}
	lang := r.model.Lang()
	r.pendingSave = dialog.SaveFileAsync(i18n.T(lang, i18n.DialogSave), r.model.Image().SuggestedFilename(), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImage),
		Extensions:  []string{ext},
	})
}

func (r *Root) pasteClipboard() {
	contents, err := clipboard.Read()
	if err != nil {
		r.model.Image().SetStatus(i18n.StatusClipboardReadFailed)
		return
	}
	_ = r.model.Image().LoadClipboardPNG(contents.PNG)
}

func (r *Root) copyClipboard() {
	if !r.model.Image().HasSource() {
		r.model.Image().SetStatus(i18n.StatusNoImageToCopy)
		return
	}
	data, err := r.model.Image().ExportPNG()
	if err != nil {
		return
	}
	if err := clipboard.Write(clipboard.Contents{PNG: data}); err != nil {
		r.model.Image().SetStatus(i18n.StatusClipboardCopyFailed)
		return
	}
	r.model.Image().SetStatus(i18n.StatusClipboardCopied)
}

func (r *Root) drainDialogs() {
	if r.pendingOpen != nil {
		select {
		case res := <-r.pendingOpen:
			r.pendingOpen = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					r.setDialogStatus(true, res.Err)
				}
				break
			}
			_ = r.model.Image().LoadPath(res.Path)
		default:
		}
	}
	if r.pendingSave != nil {
		select {
		case res := <-r.pendingSave:
			r.pendingSave = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					r.setDialogStatus(false, res.Err)
				}
				break
			}
			_ = r.model.Image().SavePath(res.Path)
		default:
		}
	}
	if r.pendingScreenshotSave != nil {
		select {
		case res := <-r.pendingScreenshotSave:
			r.pendingScreenshotSave = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					r.setScreenshotDialogStatus(res.Err)
				}
				break
			}
			_ = r.model.Screenshot().SavePath(res.Path)
		default:
		}
	}
}

func (r *Root) startCapture() {
	shot := r.model.Screenshot()
	if shot.Capturing() || r.pendingCapture != nil {
		return
	}
	shot.SetCapturing(true)
	shot.SetStatus(i18n.StatusCaptureInProgress)
	delay := time.Duration(shot.DelaySec()) * time.Second
	if shot.HideWindow() {
		ebiten.MinimizeWindow()
		r.captureMinimized = true
		if delay < 400*time.Millisecond {
			delay = 400 * time.Millisecond
		}
	}
	r.pendingCapture = capture.Async(shot.Mode(), delay)
}

func (r *Root) drainCapture() {
	if r.pendingCapture == nil {
		return
	}
	select {
	case res := <-r.pendingCapture:
		r.pendingCapture = nil
		if r.captureMinimized {
			ebiten.RestoreWindow()
			r.captureMinimized = false
		}
		shot := r.model.Screenshot()
		shot.SetCapturing(false)
		if res.Cancelled || errors.Is(res.Err, capture.ErrCancelled) {
			shot.SetStatus(i18n.StatusCaptureCancelled)
			return
		}
		if res.Err != nil {
			if errors.Is(res.Err, capture.ErrNoTool) {
				shot.SetStatus(i18n.StatusCaptureNoTool)
				return
			}
			shot.SetStatus(i18n.StatusCaptureFailed, res.Err)
			return
		}
		_ = shot.ApplyCapture(res.Image)
	default:
	}
}

func (r *Root) startScreenshotSave() {
	if r.pendingScreenshotSave != nil || !r.model.Screenshot().HasImage() {
		return
	}
	lang := r.model.Lang()
	r.pendingScreenshotSave = dialog.SaveFileAsync(i18n.T(lang, i18n.DialogSave), r.model.Screenshot().SuggestedSavePath(), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImage),
		Extensions:  []string{"png"},
	})
}

func (r *Root) copyScreenshot() {
	data, err := r.model.Screenshot().ExportPNG()
	if err != nil {
		return
	}
	if err := clipboard.Write(clipboard.Contents{PNG: data}); err != nil {
		r.model.Screenshot().SetStatus(i18n.StatusClipboardCopyFailed)
		return
	}
	r.model.Screenshot().SetStatus(i18n.StatusClipboardCopied)
}

func (r *Root) sendScreenshotToImage() {
	shot := r.model.Screenshot()
	img := shot.Image()
	if img == nil {
		shot.SetStatus(i18n.StatusNoCaptureToSave)
		return
	}
	name := filepath.Base(shot.SelectedPath())
	if name == "" || name == "." {
		name = shot.SuggestedFilename()
	}
	_ = r.model.Image().LoadImage(img, name)
	r.model.SetMode(ToolImage)
}

func (r *Root) showScreenshotFolder() {
	dir := r.model.Screenshot().DestDir()
	if dir == "" {
		r.model.Screenshot().SetStatus(i18n.StatusDestFailed, r.model.Screenshot().DestErr())
		return
	}
	if err := userdir.OpenInFileManager(dir); err != nil {
		r.model.Screenshot().SetStatus(i18n.StatusFolderOpenFailed, err)
	}
}

func (r *Root) setScreenshotDialogStatus(err error) {
	if errors.Is(err, dialog.ErrNoFileDialog) {
		r.model.Screenshot().SetStatus(i18n.StatusNoFileDialog)
		return
	}
	r.model.Screenshot().SetStatus(i18n.StatusSaveDialogFailed, err)
}

func (r *Root) setDialogStatus(open bool, err error) {
	if errors.Is(err, dialog.ErrNoFileDialog) {
		r.model.Image().SetStatus(i18n.StatusNoFileDialog)
		return
	}
	if open {
		r.model.Image().SetStatus(i18n.StatusOpenFailed, err)
		return
	}
	r.model.Image().SetStatus(i18n.StatusSaveDialogFailed, err)
}

func shortcutModifierPressed(context *guigui.Context) bool {
	return ebiten.IsKeyPressed(context.KeyBindingMode().ShortcutModifierKey())
}

func ShortcutLabel(context *guigui.Context, key string) string {
	if context.KeyBindingMode() == guigui.KeyBindingModeCommand {
		return "⌘" + key
	}
	return "Ctrl+" + key
}
