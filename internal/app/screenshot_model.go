package app

import (
	"fmt"
	"image"
	"io"
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
	maxCaptureDelaySec     = 10
	thumbMaxEdge           = 128
	screenshotPollInterval = time.Second
)

// ScreenshotModel holds the screen-capture tool state.
type ScreenshotModel struct {
	generation uint64

	image      image.Image
	preview    image.Image
	imageSize  image.Point
	sourcePath string

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
	prefix    string

	files          []ScreenshotFile
	thumbs         map[string]thumbEntry
	jumpToSelected bool
	listed         bool
	lastList       time.Time

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

func (m *ScreenshotModel) HasImage() bool {
	return m.image != nil || m.preview != nil || (m.imageSize.X > 0 && m.imageSize.Y > 0)
}

func (m *ScreenshotModel) Image() image.Image {
	if m.image != nil {
		return m.image
	}
	if m.sourcePath == "" {
		return nil
	}
	img, err := decodeImageFile(m.sourcePath)
	if err != nil {
		return nil
	}
	return img
}

func (m *ScreenshotModel) Size() image.Point {
	if m.imageSize.X > 0 && m.imageSize.Y > 0 {
		return m.imageSize
	}
	if m.image != nil {
		return m.image.Bounds().Size()
	}
	return image.Point{}
}

func (m *ScreenshotModel) Preview() image.Image {
	if m.preview != nil {
		return m.preview
	}
	if m.image != nil {
		m.preview = previewImage(m.image)
		return m.preview
	}
	if m.sourcePath != "" {
		if img, err := decodeImageFile(m.sourcePath); err == nil {
			m.imageSize = img.Bounds().Size()
			m.preview = previewImage(img)
		}
	}
	return m.preview
}

func (m *ScreenshotModel) SetImage(img image.Image) {
	m.image = img
	m.preview = nil
	m.sourcePath = ""
	if img != nil {
		m.imageSize = img.Bounds().Size()
		m.SetStatus(i18n.StatusCaptured, img.Bounds().Dx(), img.Bounds().Dy())
		return
	}
	m.imageSize = image.Point{}
	m.generation++
}

func (m *ScreenshotModel) releaseRaster() {
	m.image = nil
}

func (m *ScreenshotModel) adoptFile(path string, captured bool) error {
	img, err := decodeImageFile(path)
	if err != nil {
		m.SetStatus(i18n.StatusLoadFailed, err)
		return err
	}
	m.sourcePath = path
	m.selected = path
	m.lastSaved = path
	m.imageSize = img.Bounds().Size()
	m.preview = previewImage(img)
	m.rememberThumb(path, img)
	m.releaseRaster()
	if captured {
		m.jumpToSelected = true
		m.SetStatus(i18n.StatusSaved, path)
	} else {
		m.SetStatus(i18n.StatusLoaded, filepath.Base(path), m.imageSize.X, m.imageSize.Y)
	}
	m.RefreshFiles()
	return nil
}

// ApplyCapture stores img and writes it into the default screenshot folder.
func (m *ScreenshotModel) ApplyCapture(img image.Image) error {
	if img == nil {
		return fmt.Errorf("no image")
	}
	m.image = img
	m.preview = previewImage(img)
	m.imageSize = img.Bounds().Size()
	m.sourcePath = ""
	m.generation++
	return m.SaveDefault()
}

// ApplyCaptureFile moves a helper-written PNG into the screenshot folder
// and keeps only a preview in memory.
func (m *ScreenshotModel) ApplyCaptureFile(path string) error {
	if path == "" {
		return fmt.Errorf("no image")
	}
	dest, err := m.suggestedPath(time.Now())
	if err != nil {
		m.SetStatus(i18n.StatusDestFailed, err)
		_ = os.Remove(path)
		return err
	}
	if err := moveFile(path, dest); err != nil {
		m.SetStatus(i18n.StatusSaveFailed, err)
		_ = os.Remove(path)
		return err
	}
	return m.adoptFile(dest, true)
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

// RevealPath is the file to select in the OS file manager, or DestDir when none is selected.
func (m *ScreenshotModel) RevealPath() string {
	if p := m.SelectedPath(); p != "" {
		return p
	}
	return m.DestDir()
}

func (m *ScreenshotModel) TakeJumpToSelected() bool {
	if !m.jumpToSelected {
		return false
	}
	m.jumpToSelected = false
	return true
}

func (m *ScreenshotModel) RefreshFiles() {
	m.refreshFiles(true)
}

// PollFiles refreshes the folder at most once per second and decodes at most
// one thumbnail so a folder of large PNGs cannot spike RSS by gigabytes.
func (m *ScreenshotModel) PollFiles() {
	m.refreshFiles(false)
}

func (m *ScreenshotModel) refreshFiles(force bool) {
	if !force && m.listed && time.Since(m.lastList) < screenshotPollInterval {
		if m.ensureOneThumb() {
			m.generation++
		}
		return
	}
	dir := m.DestDir()
	if dir == "" {
		m.listed = true
		m.lastList = time.Now()
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
	m.listed = true
	m.lastList = time.Now()
	changed := !screenshotFilesEqual(m.files, next)
	if changed {
		m.files = next
		m.dropStaleThumbs()
	}
	if m.ensureOneThumb() {
		changed = true
	}
	if changed {
		m.generation++
	}
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

func (m *ScreenshotModel) dropStaleThumbs() {
	if m.thumbs == nil {
		return
	}
	live := make(map[string]struct{}, len(m.files))
	for _, f := range m.files {
		live[f.Path] = struct{}{}
	}
	for p := range m.thumbs {
		if _, ok := live[p]; !ok {
			delete(m.thumbs, p)
		}
	}
}

func (m *ScreenshotModel) ensureOneThumb() bool {
	if m.thumbs == nil {
		m.thumbs = make(map[string]thumbEntry)
	}
	for _, f := range m.files {
		if e, ok := m.thumbs[f.Path]; ok && e.img != nil && e.mod.Equal(f.ModTime) {
			continue
		}
		m.thumbs[f.Path] = thumbEntry{mod: f.ModTime, img: loadThumb(f.Path)}
		return true
	}
	return false
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
	if m.selected == path && m.HasImage() && m.preview != nil {
		return nil
	}
	return m.adoptFile(path, false)
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

func (m *ScreenshotModel) filename(t time.Time) string {
	prefix := m.prefix
	if prefix == "" {
		prefix = "Screenshot"
	}
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format(prefix + "-2006-01-02-150405.png")
}

func (m *ScreenshotModel) suggestedPath(t time.Time) (string, error) {
	dir := m.DestDir()
	if dir == "" {
		err := m.destErr
		if err == nil {
			err = fmt.Errorf("no dest")
		}
		return "", err
	}
	return userdir.UniquePath(dir, m.filename(t)), nil
}

func (m *ScreenshotModel) SuggestedFilename() string {
	return m.filename(time.Now())
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
	path, err := m.suggestedPath(time.Now())
	if err != nil {
		m.SetStatus(i18n.StatusDestFailed, err)
		return err
	}
	return m.SavePath(path)
}

func (m *ScreenshotModel) SavePath(path string) error {
	if !m.HasImage() {
		m.SetStatus(i18n.StatusNoCaptureToSave)
		return fmt.Errorf("no capture")
	}
	switch {
	case m.sourcePath != "" && m.sourcePath != path:
		if err := copyFile(m.sourcePath, path); err != nil {
			m.SetStatus(i18n.StatusSaveFailed, err)
			return err
		}
	case m.image != nil:
		data, err := imageproc.EncodePNG(m.image)
		if err != nil {
			m.SetStatus(i18n.StatusEncodeFailed, err)
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			m.SetStatus(i18n.StatusSaveFailed, err)
			return err
		}
	case m.sourcePath == path:
		// already on disk
	default:
		m.SetStatus(i18n.StatusNoCaptureToSave)
		return fmt.Errorf("no capture")
	}
	if m.preview == nil && m.image != nil {
		m.preview = previewImage(m.image)
	}
	if m.image != nil {
		m.rememberThumb(path, m.image)
	}
	m.releaseRaster()
	m.sourcePath = path
	m.lastSaved = path
	m.selected = path
	m.jumpToSelected = true
	m.SetStatus(i18n.StatusSaved, path)
	m.RefreshFiles()
	return nil
}

func (m *ScreenshotModel) ExportPNG() ([]byte, error) {
	if m.sourcePath != "" {
		data, err := os.ReadFile(m.sourcePath)
		if err != nil {
			m.SetStatus(i18n.StatusEncodeFailed, err)
			return nil, err
		}
		if len(data) > 0 {
			return data, nil
		}
	}
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

func decodeImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := imageproc.Decode(f)
	return img, err
}

func moveFile(src, dst string) error {
	if src == dst {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
