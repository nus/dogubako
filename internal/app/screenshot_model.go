package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nus/dogubako/internal/capture"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
	"github.com/nus/dogubako/internal/userdir"
)

const (
	maxCaptureDelaySec = 10
	thumbMaxEdge       = 128
)

// ScreenshotModel holds the screen-capture tool state.
type ScreenshotModel struct {
	generation uint64

	image   image.Image
	preview image.Image

	mode      capture.Mode
	delaySec  int
	delayInit bool
	hide      bool
	hideInit  bool

	capturing bool
	destDir   string
	destErr   error
	lastSaved string
	selected  string

	files          []ScreenshotFile
	thumbs         map[string]thumbEntry
	jumpToSelected bool

	status statusMsg
}

type thumbEntry struct {
	mod time.Time
	img *image.NRGBA
}

// ScreenshotFile is one image in the save folder.
type ScreenshotFile struct {
	Name    string
	Path    string
	ModTime time.Time
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

func (m *ScreenshotModel) Preview() image.Image {
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

// ApplyCapture stores img and writes it into the default screenshot folder.
func (m *ScreenshotModel) ApplyCapture(img image.Image) error {
	if img == nil {
		return fmt.Errorf("no image")
	}
	m.image = img
	m.preview = nil
	m.generation++
	return m.SaveDefault()
}

func (m *ScreenshotModel) Files() []ScreenshotFile {
	return m.files
}

func (m *ScreenshotModel) SelectedPath() string {
	if m.selected != "" {
		return m.selected
	}
	return m.lastSaved
}

func (m *ScreenshotModel) TakeJumpToSelected() bool {
	if !m.jumpToSelected {
		return false
	}
	m.jumpToSelected = false
	return true
}

func (m *ScreenshotModel) RefreshFiles() {
	dir := m.DestDir()
	if dir == "" {
		if len(m.files) == 0 {
			return
		}
		m.files = nil
		m.thumbs = nil
		m.generation++
		return
	}
	next, err := listScreenshotFiles(dir)
	if err != nil {
		next = nil
	}
	if screenshotFilesEqual(m.files, next) {
		m.ensureThumbs()
		return
	}
	m.files = next
	m.ensureThumbs()
	m.generation++
}

func (m *ScreenshotModel) Thumbnail(path string) image.Image {
	if path == "" || m.thumbs == nil {
		return nil
	}
	e, ok := m.thumbs[path]
	if !ok {
		return nil
	}
	return e.img
}

func (m *ScreenshotModel) ensureThumbs() {
	if m.thumbs == nil {
		m.thumbs = make(map[string]thumbEntry)
	}
	live := make(map[string]struct{}, len(m.files))
	for _, f := range m.files {
		live[f.Path] = struct{}{}
		if e, ok := m.thumbs[f.Path]; ok && e.img != nil && e.mod.Equal(f.ModTime) {
			continue
		}
		m.thumbs[f.Path] = thumbEntry{mod: f.ModTime, img: loadThumb(f.Path)}
	}
	for p := range m.thumbs {
		if _, ok := live[p]; !ok {
			delete(m.thumbs, p)
		}
	}
}

func (m *ScreenshotModel) rememberThumb(path string, img image.Image) {
	if path == "" || img == nil {
		return
	}
	if m.thumbs == nil {
		m.thumbs = make(map[string]thumbEntry)
	}
	mod := time.Now()
	if fi, err := os.Stat(path); err == nil {
		mod = fi.ModTime()
	}
	m.thumbs[path] = thumbEntry{mod: mod, img: makeThumb(img)}
}

func (m *ScreenshotModel) LoadPath(path string) error {
	if path == "" {
		return fmt.Errorf("no path")
	}
	if m.selected == path && m.image != nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		m.SetStatus(i18n.StatusLoadFailed, err)
		return err
	}
	defer f.Close()
	img, _, err := imageproc.Decode(f)
	if err != nil {
		m.SetStatus(i18n.StatusLoadFailed, err)
		return err
	}
	m.image = img
	m.preview = nil
	m.selected = path
	m.lastSaved = path
	m.SetStatus(i18n.StatusLoaded, filepath.Base(path), img.Bounds().Dx(), img.Bounds().Dy())
	return nil
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
	m.selected = path
	m.jumpToSelected = true
	m.rememberThumb(path, m.image)
	m.SetStatus(i18n.StatusSaved, path)
	m.RefreshFiles()
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

func listScreenshotFiles(dir string) ([]ScreenshotFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]ScreenshotFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isImageFilename(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, ScreenshotFile{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			ModTime: info.ModTime(),
		})
	}
	slices.SortFunc(files, func(a, b ScreenshotFile) int {
		if a.ModTime.After(b.ModTime) {
			return -1
		}
		if a.ModTime.Before(b.ModTime) {
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return files, nil
}

func screenshotFilesEqual(a, b []ScreenshotFile) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || !a[i].ModTime.Equal(b[i].ModTime) {
			return false
		}
	}
	return true
}

func loadThumb(path string) *image.NRGBA {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := imageproc.Decode(f)
	if err != nil {
		return nil
	}
	return makeThumb(img)
}

func makeThumb(img image.Image) *image.NRGBA {
	if img == nil {
		return nil
	}
	size := img.Bounds().Size()
	if size.X <= 0 || size.Y <= 0 {
		return nil
	}
	if size.X <= thumbMaxEdge && size.Y <= thumbMaxEdge {
		return imageproc.Resize(img, size.X, size.Y)
	}
	scale := float64(thumbMaxEdge) / float64(max(size.X, size.Y))
	w := max(1, int(float64(size.X)*scale+0.5))
	h := max(1, int(float64(size.Y)*scale+0.5))
	return imageproc.Resize(img, w, h)
}
