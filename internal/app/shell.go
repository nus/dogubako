package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gogpu/gogpu"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/dnd"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/appicon"
	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/clipimg"
	"github.com/nus/dogubako/internal/dialog"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
	"github.com/nus/dogubako/internal/userdir"
)

const (
	windowWidth  = 1100
	windowHeight = 760
	windowMinW   = 800
	windowMinH   = 560

	// unit matches guigui's UnitSize at 1x (~24px). Sidebar was 8*u,
	// number fields 5*u, screenshot file list 16*u, device list 4*u.
	unit            float32 = 24
	sidebarWidth            = 8 * unit
	fieldWidth              = 5 * unit
	screenshotListW         = 16 * unit
	deviceListH             = 4 * unit
	formRowH        float32 = 32
	fieldH          float32 = 28
)

// Shell is the application controller: model, window, and widget tree.
type Shell struct {
	model Model
	gpu   *gogpu.App
	ui    *uiapp.App
	theme *material3.Theme

	rev state.Signal[uint64]

	pendingOpen           <-chan dialog.FileResult
	pendingSave           <-chan dialog.FileResult
	pendingScreenshotSave <-chan dialog.FileResult
	pendingAndroidPull    <-chan dialog.FileResult
	pendingAndroidPush    <-chan dialog.FileResult
	pendingCapture        <-chan capture.Result
	captureCancel         context.CancelFunc
	captureHidden         bool

	mu     sync.Mutex
	closed bool

	scaleSig  state.Signal[string]
	widthSig  state.Signal[string]
	heightSig state.Signal[string]
	cropXSig  state.Signal[string]
	cropYSig  state.Signal[string]
	cropWSig  state.Signal[string]
	cropHSig  state.Signal[string]
	angleSig  state.Signal[string]
	delaySig  state.Signal[string]
	quality   state.Signal[float32]
	keepAsp   state.Signal[bool]
	cropOn    state.Signal[bool]
	hideWin   state.Signal[bool]
	fmtSig    state.Signal[string]
	modeSig   state.Signal[string]
	shotMode  state.Signal[string]
	langSig   state.Signal[string]
	toolSel   state.Signal[int]
	shotSel   state.Signal[int]
	devSel    state.Signal[int]
	adbSel    state.Signal[int]
}

// Run starts the desktop application.
func Run() error {
	cjkOK := cjkembed.Register()
	lang := sessionLang(cjkOK, i18n.Load())

	theme := material3.New(widget.Hex(0x2F81FF))
	s := &Shell{
		theme:     theme,
		rev:       state.NewSignal[uint64](0),
		scaleSig:  state.NewSignal("100"),
		widthSig:  state.NewSignal("0"),
		heightSig: state.NewSignal("0"),
		cropXSig:  state.NewSignal("0"),
		cropYSig:  state.NewSignal("0"),
		cropWSig:  state.NewSignal("0"),
		cropHSig:  state.NewSignal("0"),
		angleSig:  state.NewSignal("0"),
		delaySig:  state.NewSignal("1"),
		quality:   state.NewSignal[float32](float32(imageproc.DefaultJPEGQuality)),
		keepAsp:   state.NewSignal(true),
		cropOn:    state.NewSignal(false),
		hideWin:   state.NewSignal(true),
		fmtSig:    state.NewSignal(string(imageproc.FormatPNG)),
		modeSig:   state.NewSignal(string(ToolImage)),
		shotMode:  state.NewSignal(string(capture.ModeFull)),
		langSig:   state.NewSignal(string(lang)),
		toolSel:   state.NewSignal(0),
		shotSel:   state.NewSignal(-1),
		devSel:    state.NewSignal(-1),
		adbSel:    state.NewSignal(-1),
	}
	s.model.applyLang(lang, false)

	cfg := gogpu.DefaultConfig().
		WithTitle(i18n.T(s.model.Lang(), i18n.AppTitle)).
		WithSize(windowWidth, windowHeight).
		WithMinSize(windowMinW, windowMinH)
	if img, err := appicon.WindowRGBA(); err == nil {
		cfg = cfg.WithIcon(img)
	}
	if p := os.Getenv("DOGUBAKO_OPEN"); p != "" {
		_ = s.model.Image().LoadPath(p)
		s.syncFields()
	}
	if t := os.Getenv("DOGUBAKO_TOOL"); t != "" {
		s.model.SetMode(ToolID(t))
	}
	gpuApp := gogpu.NewApp(cfg)
	uiApp := uiapp.New(
		uiapp.WithWindowProvider(gpuApp),
		uiapp.WithPlatformProvider(gpuApp),
		uiapp.WithEventSource(gpuApp.EventSource()),
		uiapp.WithTheme(theme.AsTheme()),
	)
	s.gpu = gpuApp
	s.ui = uiApp
	uiApp.SetRoot(s.buildRoot())

	gpuApp.OnUpdate(func(float64) { s.tick() })
	go s.watchPending()

	mgr := uiApp.Window().DndManager()
	mgr.RegisterTarget(&fileDropTarget{shell: s}, geometry.NewRect(0, 0, 4000, 3000))

	err := runDesktop(gpuApp, uiApp)
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return err
}

func (s *Shell) watchPending() {
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		done := s.closed
		s.mu.Unlock()
		if done {
			return
		}
		if s.hasPending() && s.gpu != nil {
			s.gpu.RequestRedraw()
		}
	}
}

func (s *Shell) hasPending() bool {
	return s.pendingOpen != nil || s.pendingSave != nil || s.pendingScreenshotSave != nil ||
		s.pendingAndroidPull != nil || s.pendingAndroidPush != nil || s.pendingCapture != nil ||
		s.model.Android().Busy()
}

func (s *Shell) bump() {
	s.syncFields()
	s.rev.Set(s.rev.Get() + 1)
	if s.gpu != nil {
		s.gpu.RequestRedraw()
	}
}

// forceFullRepaint asks the window to redraw every pixel. FrameworkManaged
// otherwise clips to dirty widgets' previous ScreenBounds, so a tool switch
// would leave the old panel on screen.
func (s *Shell) forceFullRepaint() {
	if s.ui == nil {
		return
	}
	s.ui.Window().SetRenderMode(uiapp.RenderModeFrameworkManaged)
}

func (s *Shell) reloadUI() {
	s.syncFields()
	if s.ui != nil {
		s.ui.SetRoot(s.buildRoot())
	}
	s.bump()
}

func (s *Shell) syncFields() {
	img := s.model.Image()
	s.scaleSig.Set(strconv.Itoa(img.ScalePercent()))
	s.widthSig.Set(strconv.Itoa(img.Width()))
	s.heightSig.Set(strconv.Itoa(img.Height()))
	c := img.Crop()
	s.cropXSig.Set(strconv.Itoa(c.Min.X))
	s.cropYSig.Set(strconv.Itoa(c.Min.Y))
	s.cropWSig.Set(strconv.Itoa(max(1, c.Dx())))
	s.cropHSig.Set(strconv.Itoa(max(1, c.Dy())))
	s.angleSig.Set(strconv.Itoa(img.RotateDegrees()))
	s.quality.Set(float32(img.JPEGQuality()))
	s.keepAsp.Set(img.KeepAspect())
	s.cropOn.Set(img.CropEnabled())
	s.fmtSig.Set(string(img.Format()))

	shot := s.model.Screenshot()
	s.delaySig.Set(strconv.Itoa(shot.DelaySec()))
	s.hideWin.Set(shot.HideWindow())
	s.shotMode.Set(string(shot.Mode()))

	s.langSig.Set(string(s.model.Lang()))
	s.modeSig.Set(string(s.model.Mode()))
	s.toolSel.Set(toolIndex(s.model.Mode()))
}

func (s *Shell) tick() {
	s.drainDialogs()
	s.drainCapture()
	s.model.Android().Drain()
	s.model.Screenshot().RefreshFiles()
	gen := s.model.Image().Generation() + s.model.Screenshot().Generation() + s.model.Android().Generation()
	if gen > s.rev.Get() {
		s.syncFields()
		s.rev.Set(gen)
	}
}

func (s *Shell) computed(fn func() string) state.ReadonlySignal[string] {
	return state.NewComputed(func() string {
		_ = s.rev.Get()
		return fn()
	}, s.rev.AsReadonly())
}

func (s *Shell) handleKey(e *event.KeyEvent) bool {
	if e.KeyType != event.KeyPress {
		return false
	}
	if !shortcutHeld(e) {
		return false
	}
	switch s.model.Mode() {
	case ToolScreenshot:
		switch e.Key {
		case event.KeyS:
			_ = s.model.Screenshot().SaveDefault()
			s.bump()
			return true
		case event.KeyC:
			s.copyScreenshot()
			return true
		}
	case ToolAndroid:
		return false
	default:
		switch e.Key {
		case event.KeyO:
			s.startOpen()
			return true
		case event.KeyS:
			s.startSave()
			return true
		case event.KeyV:
			s.pasteClipboard()
			return true
		case event.KeyC:
			s.copyClipboard()
			return true
		}
	}
	return false
}

func (s *Shell) startOpen() {
	if s.pendingOpen != nil {
		return
	}
	lang := s.model.Lang()
	s.pendingOpen = dialog.OpenFileAsync(i18n.T(lang, i18n.DialogOpen), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImages),
		Extensions:  []string{"png", "jpg", "jpeg", "gif"},
	})
}

func (s *Shell) startSave() {
	if s.pendingSave != nil || !s.model.Image().HasSource() {
		return
	}
	ext := "png"
	if s.model.Image().Format() == imageproc.FormatJPEG {
		ext = "jpg"
	}
	lang := s.model.Lang()
	s.pendingSave = dialog.SaveFileAsync(i18n.T(lang, i18n.DialogSave), s.model.Image().SuggestedFilename(), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImage),
		Extensions:  []string{ext},
	})
}

func (s *Shell) pasteClipboard() {
	data, err := clipimg.ReadPNG()
	if err != nil {
		s.model.Image().SetStatus(i18n.StatusClipboardReadFailed)
		s.bump()
		return
	}
	_ = s.model.Image().LoadClipboardPNG(data)
	s.bump()
}

func (s *Shell) copyClipboard() {
	if !s.model.Image().HasSource() {
		s.model.Image().SetStatus(i18n.StatusNoImageToCopy)
		s.bump()
		return
	}
	data, err := s.model.Image().ExportPNG()
	if err != nil {
		s.bump()
		return
	}
	if err := clipimg.WritePNG(data); err != nil {
		s.model.Image().SetStatus(i18n.StatusClipboardCopyFailed)
		s.bump()
		return
	}
	s.model.Image().SetStatus(i18n.StatusClipboardCopied)
	s.bump()
}

func (s *Shell) drainDialogs() {
	if s.pendingOpen != nil {
		select {
		case res := <-s.pendingOpen:
			s.pendingOpen = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					s.setDialogStatus(true, res.Err)
				}
				break
			}
			_ = s.model.Image().LoadPath(res.Path)
			s.bump()
		default:
		}
	}
	if s.pendingSave != nil {
		select {
		case res := <-s.pendingSave:
			s.pendingSave = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					s.setDialogStatus(false, res.Err)
				}
				break
			}
			_ = s.model.Image().SavePath(res.Path)
			s.bump()
		default:
		}
	}
	if s.pendingScreenshotSave != nil {
		select {
		case res := <-s.pendingScreenshotSave:
			s.pendingScreenshotSave = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					s.setScreenshotDialogStatus(res.Err)
				}
				break
			}
			_ = s.model.Screenshot().SavePath(res.Path)
			s.bump()
		default:
		}
	}
	if s.pendingAndroidPull != nil {
		select {
		case res := <-s.pendingAndroidPull:
			s.pendingAndroidPull = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					s.setAndroidDialogStatus(res.Err)
				}
				break
			}
			s.model.Android().StartPull(res.Path)
			s.bump()
		default:
		}
	}
	if s.pendingAndroidPush != nil {
		select {
		case res := <-s.pendingAndroidPush:
			s.pendingAndroidPush = nil
			if res.Cancelled || res.Err != nil {
				if res.Err != nil {
					s.setAndroidDialogStatus(res.Err)
				}
				break
			}
			s.model.Android().StartPush(res.Path)
			s.bump()
		default:
		}
	}
}

func (s *Shell) startCapture() {
	shot := s.model.Screenshot()
	s.cancelPendingCapture()
	shot.SetCapturing(true)
	shot.SetStatus(i18n.StatusCaptureInProgress)
	delay := time.Duration(shot.DelaySec()) * time.Second
	if shot.HideWindow() {
		s.hideAppWindow()
		s.captureHidden = true
		if delay < 400*time.Millisecond {
			delay = 400 * time.Millisecond
		}
	} else {
		s.restoreCaptureWindow()
	}
	ctx, cancel := context.WithTimeout(context.Background(), delay+capture.Timeout(shot.Mode()))
	s.captureCancel = cancel
	s.pendingCapture = capture.Async(ctx, shot.Mode(), delay)
	s.bump()
}

func (s *Shell) cancelPendingCapture() {
	if s.captureCancel != nil {
		s.captureCancel()
		s.captureCancel = nil
	}
	if s.pendingCapture == nil {
		return
	}
	stale := s.pendingCapture
	s.pendingCapture = nil
	go func() { <-stale }()
}

func (s *Shell) drainCapture() {
	if s.pendingCapture == nil {
		return
	}
	select {
	case res := <-s.pendingCapture:
		s.pendingCapture = nil
		if s.captureCancel != nil {
			s.captureCancel()
			s.captureCancel = nil
		}
		s.restoreCaptureWindow()
		shot := s.model.Screenshot()
		shot.SetCapturing(false)
		if res.Cancelled || errors.Is(res.Err, capture.ErrCancelled) {
			shot.SetStatus(i18n.StatusCaptureCancelled)
			s.bump()
			return
		}
		if res.Err != nil {
			if errors.Is(res.Err, capture.ErrTimeout) {
				shot.SetStatus(i18n.StatusCaptureTimeout)
			} else if errors.Is(res.Err, capture.ErrNoTool) {
				shot.SetStatus(i18n.StatusCaptureNoTool)
			} else {
				shot.SetStatus(i18n.StatusCaptureFailed, res.Err)
			}
			s.bump()
			return
		}
		_ = shot.ApplyCapture(res.Image)
		s.bump()
	default:
	}
}

func (s *Shell) startScreenshotSave() {
	if s.pendingScreenshotSave != nil || !s.model.Screenshot().HasImage() {
		return
	}
	lang := s.model.Lang()
	s.pendingScreenshotSave = dialog.SaveFileAsync(i18n.T(lang, i18n.DialogSave), s.model.Screenshot().SuggestedSavePath(), &dialog.FileFilter{
		Description: i18n.T(lang, i18n.FilterImage),
		Extensions:  []string{"png"},
	})
}

func (s *Shell) copyScreenshot() {
	data, err := s.model.Screenshot().ExportPNG()
	if err != nil {
		s.bump()
		return
	}
	if err := clipimg.WritePNG(data); err != nil {
		s.model.Screenshot().SetStatus(i18n.StatusClipboardCopyFailed)
		s.bump()
		return
	}
	s.model.Screenshot().SetStatus(i18n.StatusClipboardCopied)
	s.bump()
}

func (s *Shell) sendScreenshotToImage() {
	shot := s.model.Screenshot()
	img := shot.Image()
	if img == nil {
		shot.SetStatus(i18n.StatusNoCaptureToSave)
		s.bump()
		return
	}
	name := filepath.Base(shot.SelectedPath())
	if name == "" || name == "." {
		name = shot.SuggestedFilename()
	}
	_ = s.model.Image().LoadImage(img, name)
	s.model.SetMode(ToolImage)
	s.bump()
}

func (s *Shell) showScreenshotFolder() {
	shot := s.model.Screenshot()
	path := shot.RevealPath()
	if path == "" {
		shot.SetStatus(i18n.StatusDestFailed, shot.DestErr())
		s.bump()
		return
	}
	if err := userdir.OpenInFileManager(path); err != nil {
		shot.SetStatus(i18n.StatusFolderOpenFailed, err)
	}
	s.bump()
}

func (s *Shell) startAndroidPull() {
	if s.pendingAndroidPull != nil || s.model.Android().Busy() {
		return
	}
	e, ok := s.model.Android().SelectedEntry()
	if !ok {
		s.model.Android().SetStatus(i18n.StatusAdbNoSelection)
		s.bump()
		return
	}
	lang := s.model.Lang()
	if e.IsDir {
		s.pendingAndroidPull = dialog.OpenDirectoryAsync(i18n.T(lang, i18n.DialogOpenDir))
		return
	}
	s.pendingAndroidPull = dialog.SaveFileAsync(i18n.T(lang, i18n.DialogSaveAny), e.Name, nil)
}

func (s *Shell) startAndroidPush(folder bool) {
	if s.pendingAndroidPush != nil || s.model.Android().Busy() {
		return
	}
	lang := s.model.Lang()
	if folder {
		s.pendingAndroidPush = dialog.OpenDirectoryAsync(i18n.T(lang, i18n.DialogOpenFolder))
		return
	}
	s.pendingAndroidPush = dialog.OpenFileAsync(i18n.T(lang, i18n.DialogOpenAny), nil)
}

func (s *Shell) setAndroidDialogStatus(err error) {
	if errors.Is(err, dialog.ErrNoFileDialog) {
		s.model.Android().SetStatus(i18n.StatusNoFileDialog)
		s.bump()
		return
	}
	s.model.Android().SetStatus(i18n.StatusSaveDialogFailed, err)
	s.bump()
}

func (s *Shell) setScreenshotDialogStatus(err error) {
	if errors.Is(err, dialog.ErrNoFileDialog) {
		s.model.Screenshot().SetStatus(i18n.StatusNoFileDialog)
		s.bump()
		return
	}
	s.model.Screenshot().SetStatus(i18n.StatusSaveDialogFailed, err)
	s.bump()
}

func (s *Shell) setDialogStatus(open bool, err error) {
	if errors.Is(err, dialog.ErrNoFileDialog) {
		s.model.Image().SetStatus(i18n.StatusNoFileDialog)
		s.bump()
		return
	}
	if open {
		s.model.Image().SetStatus(i18n.StatusOpenFailed, err)
		s.bump()
		return
	}
	s.model.Image().SetStatus(i18n.StatusSaveDialogFailed, err)
	s.bump()
}

func (s *Shell) hideAppWindow() {
	if w := s.gpu.PrimaryWindow(); w != nil {
		w.Hide()
	}
}

func (s *Shell) restoreCaptureWindow() {
	if !s.captureHidden {
		return
	}
	s.captureHidden = false
	if w := s.gpu.PrimaryWindow(); w != nil {
		w.Show()
	}
}

func shortcutHeld(e *event.KeyEvent) bool {
	if runtime.GOOS == "darwin" {
		return e.IsSuper()
	}
	return e.IsCtrl()
}

// ShortcutLabel returns the platform shortcut caption for key.
func ShortcutLabel(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + key
	}
	return "Ctrl+" + key
}

func toolIndex(mode ToolID) int {
	for i, t := range Tools {
		if t.ID == mode {
			return i
		}
	}
	return 0
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

type fileDropTarget struct {
	shell *Shell
}

func (t *fileDropTarget) CanAccept(data dnd.DragData) bool {
	return data.Kind == dnd.KindFile
}

func (t *fileDropTarget) DragEnter(_ dnd.DragData) {}

func (t *fileDropTarget) DragOver(_ dnd.DragData, _ geometry.Point) dnd.DropEffect {
	return dnd.DropCopy
}

func (t *fileDropTarget) DragLeave() {}

func (t *fileDropTarget) Drop(data dnd.DragData, _ geometry.Point) bool {
	payload, ok := data.Payload.(dnd.FilePayload)
	if !ok || len(payload.Paths) == 0 {
		return false
	}
	if t.shell.model.Mode() != ToolImage {
		t.shell.model.SetMode(ToolImage)
	}
	_ = t.shell.model.Image().LoadPaths(payload.Paths)
	t.shell.bump()
	return true
}
