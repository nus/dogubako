package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
	"github.com/nus/dogubako/internal/userdir"
)

const maxCaptureDelaySec = 10

// ScreenshotModel holds the screen-capture tool state.
type ScreenshotModel struct {
	generation uint64

	image   image.Image
	preview *ebiten.Image

	mode      capture.Mode
	delaySec  int
	delayInit bool
	hide      bool
	hideInit  bool

	capturing bool
	destDir   string
	destErr   error
	lastSaved string

	status statusMsg
}

func (m *ScreenshotModel) Generation() uint64 { return m.generation }

func (m *ScreenshotModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *ScreenshotModel) SetStatus(key i18n.Key, args ...any) {
	if m.status.key == key && fmt.Sprint(m.status.args...) == fmt.Sprint(args...) {
		return
	}
	m.status.key = key
	if len(args) == 0 {
		m.status.args = nil
	} else {
		m.status.args = append([]any(nil), args...)
	}
	m.generation++
}

func (m *ScreenshotModel) HasImage() bool     { return m.image != nil }
func (m *ScreenshotModel) Image() image.Image { return m.image }

func (m *ScreenshotModel) Size() image.Point {
	if m.image == nil {
		return image.Point{}
	}
	return m.image.Bounds().Size()
}

func (m *ScreenshotModel) Preview() *ebiten.Image {
	if m.preview == nil && m.image != nil {
		m.preview = previewImage(m.image)
	}
	return m.preview
}

func (m *ScreenshotModel) SetImage(img image.Image) {
	m.image = img
	m.preview = nil
	m.generation++
	if img != nil {
		m.SetStatus(i18n.StatusCaptured, img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func (m *ScreenshotModel) Mode() capture.Mode {
	return capture.Normalize(m.mode)
}

func (m *ScreenshotModel) SetMode(mode capture.Mode) {
	mode = capture.Normalize(mode)
	if m.Mode() == mode {
		return
	}
	m.mode = mode
	m.generation++
}

func (m *ScreenshotModel) DelaySec() int {
	if !m.delayInit {
		return 1
	}
	return m.delaySec
}

func (m *ScreenshotModel) SetDelaySec(v int) {
	if v < 0 {
		v = 0
	}
	if v > maxCaptureDelaySec {
		v = maxCaptureDelaySec
	}
	m.delayInit = true
	if m.delaySec == v {
		return
	}
	m.delaySec = v
	m.generation++
}

func (m *ScreenshotModel) HideWindow() bool {
	if !m.hideInit {
		return true
	}
	return m.hide
}

func (m *ScreenshotModel) SetHideWindow(hide bool) {
	m.hideInit = true
	if m.hide == hide {
		return
	}
	m.hide = hide
	m.generation++
}

func (m *ScreenshotModel) Capturing() bool { return m.capturing }

func (m *ScreenshotModel) SetCapturing(capturing bool) {
	if m.capturing == capturing {
		return
	}
	m.capturing = capturing
	m.generation++
}

func (m *ScreenshotModel) DestDir() string {
	m.resolveDest()
	return m.destDir
}

func (m *ScreenshotModel) DestErr() error {
	m.resolveDest()
	return m.destErr
}

func (m *ScreenshotModel) resolveDest() {
	if m.destDir != "" || m.destErr != nil {
		return
	}
	dir, err := userdir.EnsureScreenshots()
	if err != nil {
		m.destErr = err
		return
	}
	m.destDir = dir
}

func (m *ScreenshotModel) SuggestedFilename() string {
	return userdir.Filename(time.Now())
}

func (m *ScreenshotModel) SuggestedSavePath() string {
	name := m.SuggestedFilename()
	dir := m.DestDir()
	if dir == "" {
		return name
	}
	return filepath.Join(dir, name)
}

func (m *ScreenshotModel) SaveDefault() error {
	path, err := userdir.SuggestedPath(time.Now())
	if err != nil {
		m.SetStatus(i18n.StatusDestFailed, err)
		return err
	}
	return m.SavePath(path)
}

func (m *ScreenshotModel) SavePath(path string) error {
	if m.image == nil {
		m.SetStatus(i18n.StatusNoCaptureToSave)
		return fmt.Errorf("no capture")
	}
	data, err := imageproc.EncodePNG(m.image)
	if err != nil {
		m.SetStatus(i18n.StatusEncodeFailed, err)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.SetStatus(i18n.StatusSaveFailed, err)
		return err
	}
	m.lastSaved = path
	m.SetStatus(i18n.StatusSaved, path)
	return nil
}

func (m *ScreenshotModel) ExportPNG() ([]byte, error) {
	if m.image == nil {
		m.SetStatus(i18n.StatusNoCaptureToCopy)
		return nil, fmt.Errorf("no capture")
	}
	data, err := imageproc.EncodePNG(m.image)
	if err != nil {
		m.SetStatus(i18n.StatusEncodeFailed, err)
		return nil, err
	}
	return data, nil
}

func (m *ScreenshotModel) LastSaved() string { return m.lastSaved }
