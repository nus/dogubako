package app

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/imageproc"
)

// ToolID identifies a workspace tool shown in the sidebar.
type ToolID string

const (
	ToolImage      ToolID = "image"
	ToolScreenshot ToolID = "screenshot"
	ToolAndroid    ToolID = "android"
)

// Tool is a sidebar entry.
type Tool struct {
	ID ToolID
}

// Tools is the ordered sidebar catalog. Add new tools here.
var Tools = []Tool{
	{ID: ToolImage},
	{ID: ToolScreenshot},
	{ID: ToolAndroid},
}

// Title returns the localized sidebar label for this tool.
func (t Tool) Title(lang i18n.Lang) string {
	switch t.ID {
	case ToolImage:
		return i18n.T(lang, i18n.ToolImage)
	case ToolScreenshot:
		return i18n.T(lang, i18n.ToolScreenshot)
	case ToolAndroid:
		return i18n.T(lang, i18n.ToolAndroid)
	default:
		return string(t.ID)
	}
}

// Model is the application-wide state provided to widgets via Env.
type Model struct {
	lang       i18n.Lang
	mode       ToolID
	image      ImageModel
	screenshot ScreenshotModel
	android    AndroidModel
}

func (m *Model) Lang() i18n.Lang {
	if m.lang == "" {
		m.lang = i18n.Load()
	}
	return m.lang
}

func (m *Model) SetLang(lang i18n.Lang) {
	lang = i18n.Normalize(lang)
	if m.Lang() == lang {
		return
	}
	m.applyLang(lang, true)
}

func (m *Model) applyLang(lang i18n.Lang, persist bool) {
	m.lang = i18n.Normalize(lang)
	if persist {
		_ = i18n.Save(m.lang)
	}
}

// sessionLang picks the UI language for this process.
// Without a CJK face, Japanese strings would be drawn with Inter, so
// the session stays English. The saved preference is not overwritten.
func sessionLang(cjkOK bool, saved i18n.Lang) i18n.Lang {
	if !cjkOK {
		return i18n.EN
	}
	return i18n.Normalize(saved)
}

func (m *Model) Mode() ToolID {
	if m.mode == "" {
		return ToolImage
	}
	return m.mode
}

func (m *Model) SetMode(mode ToolID) {
	m.mode = mode
}

func (m *Model) Image() *ImageModel {
	return &m.image
}

func (m *Model) Screenshot() *ScreenshotModel {
	return &m.screenshot
}

func (m *Model) Android() *AndroidModel {
	return &m.android
}

// ImageFeature is a sub-mode of the image tool. The main panel switches UI
// for each feature (clip vs paint).
type ImageFeature string

const (
	ImageClip  ImageFeature = "clip"
	ImagePaint ImageFeature = "paint"
)

// ImageFeatures is the ordered catalog of image-tool functions.
var ImageFeatures = []ImageFeature{ImageClip, ImagePaint}

// Title returns the localized label for this image-tool function.
func (f ImageFeature) Title(lang i18n.Lang) string {
	switch f {
	case ImagePaint:
		return i18n.T(lang, i18n.ImagePaint)
	default:
		return i18n.T(lang, i18n.ImageClip)
	}
}

// PaintTool is the active brush in paint mode.
type PaintTool string

const (
	PaintBrush  PaintTool = "brush"
	PaintEraser PaintTool = "eraser"
)

// ImageModel holds the first tool: clip (resize, rotate, crop) and paint.
type ImageModel struct {
	generation uint64

	source     image.Image
	sourceName string

	feature ImageFeature

	// keepAspect defaults to true until the user changes it.
	keepAspect            bool
	keepAspectInitialized bool

	params imageproc.Params

	paintLayer *image.NRGBA
	paintUsed  bool
	paintUndo  []*image.NRGBA
	paintColor color.NRGBA
	brushSize  int
	paintTool  PaintTool
	strokeLast image.Point
	stroking   bool

	processed  image.Image
	processErr error
	dirty      bool

	srcPreview image.Image
	dstPreview image.Image

	status statusMsg
}

type statusMsg struct {
	key  i18n.Key
	args []any
}

const previewMaxEdge = 2048

func (m *ImageModel) Generation() uint64 { return m.generation }

func (m *ImageModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *ImageModel) SetStatus(key i18n.Key, args ...any) {
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

func (m *ImageModel) HasSource() bool     { return m.source != nil }
func (m *ImageModel) Source() image.Image { return m.source }
func (m *ImageModel) SourceName() string  { return m.sourceName }

func (m *ImageModel) SourceSize() image.Point {
	if m.source == nil {
		return image.Point{}
	}
	return m.source.Bounds().Size()
}

func (m *ImageModel) BaseSize() image.Point {
	if m.source == nil {
		return image.Point{}
	}
	size := m.source.Bounds().Size()
	if m.params.CropEnabled {
		c := m.params.Crop.Intersect(m.source.Bounds())
		if !c.Empty() {
			size = c.Size()
		}
	}
	return imageproc.RotatedSize(size, m.params.RotateDegrees)
}

func (m *ImageModel) Params() imageproc.Params { return m.params }

func (m *ImageModel) Feature() ImageFeature {
	if m.feature == "" {
		return ImageClip
	}
	return m.feature
}

func (m *ImageModel) SetFeature(feature ImageFeature) {
	if feature != ImagePaint {
		feature = ImageClip
	}
	if m.Feature() == feature {
		return
	}
	m.feature = feature
	m.generation++
}

func (m *ImageModel) KeepAspect() bool {
	if !m.keepAspectInitialized {
		return true
	}
	return m.keepAspect
}

func (m *ImageModel) Width() int  { return m.params.Width }
func (m *ImageModel) Height() int { return m.params.Height }

func (m *ImageModel) ScalePercent() int {
	base := m.BaseSize()
	if base.X <= 0 {
		return 100
	}
	return max(1, int(float64(m.params.Width)*100/float64(base.X)+0.5))
}

func (m *ImageModel) CropEnabled() bool { return m.params.CropEnabled }

func (m *ImageModel) Crop() image.Rectangle {
	if m.source == nil {
		return image.Rectangle{}
	}
	if m.params.Crop.Empty() {
		return m.source.Bounds()
	}
	return m.params.Crop
}

func (m *ImageModel) Format() imageproc.Format {
	m.ensureDefaults()
	return m.params.Format
}

func (m *ImageModel) JPEGQuality() int {
	m.ensureDefaults()
	return m.params.JPEGQuality
}

func (m *ImageModel) Processed() (image.Image, error) {
	m.recompute()
	return m.processed, m.processErr
}

func (m *ImageModel) LoadPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return m.LoadReader(f, filepath.Base(path))
}

func (m *ImageModel) LoadReader(r io.Reader, name string) error {
	img, format, err := imageproc.Decode(r)
	if err != nil {
		m.SetStatus(i18n.StatusLoadFailed, err)
		return err
	}
	m.setSource(img, name, format)
	m.SetStatus(i18n.StatusLoaded, name, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}

func (m *ImageModel) LoadDropped(fsys fs.FS) error {
	if fsys == nil {
		return nil
	}
	var loadErr error
	found := false
	walkErr := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !isImageFilename(path) {
			return nil
		}
		f, err := fsys.Open(path)
		if err != nil {
			loadErr = err
			return fs.SkipAll
		}
		defer f.Close()
		name := filepath.Base(path)
		if named, ok := f.(interface{ Name() string }); ok && named.Name() != "" {
			name = filepath.Base(named.Name())
		}
		loadErr = m.LoadReader(f, name)
		found = true
		return fs.SkipAll
	})
	if walkErr != nil && loadErr == nil {
		loadErr = walkErr
	}
	if !found && loadErr == nil {
		m.SetStatus(i18n.StatusDropNoImage)
		return fmt.Errorf("no image in dropped files")
	}
	return loadErr
}

func (m *ImageModel) LoadPaths(paths []string) error {
	for _, p := range paths {
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err := m.LoadDropped(os.DirFS(p))
			if err == nil {
				return nil
			}
			continue
		}
		if !isImageFilename(p) {
			continue
		}
		return m.LoadPath(p)
	}
	m.SetStatus(i18n.StatusDropNoImage)
	return fmt.Errorf("no image in dropped files")
}

func (m *ImageModel) LoadClipboardPNG(png []byte) error {
	if len(png) == 0 {
		m.SetStatus(i18n.StatusClipboardNoImage)
		return fmt.Errorf("clipboard has no png")
	}
	img, format, err := imageproc.DecodeBytes(png)
	if err != nil {
		m.SetStatus(i18n.StatusClipboardImageReadFailed, err)
		return err
	}
	m.setSource(img, "clipboard.png", format)
	m.SetStatus(i18n.StatusPasted, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}

func (m *ImageModel) LoadImage(img image.Image, name string) error {
	if img == nil {
		return fmt.Errorf("no image")
	}
	if name == "" {
		name = "screenshot.png"
	}
	m.setSource(img, name, imageproc.FormatPNG)
	m.SetStatus(i18n.StatusLoaded, name, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}

func (m *ImageModel) SavePath(path string) error {
	img, err := m.Processed()
	if err != nil {
		m.SetStatus(i18n.StatusExportFailed, err)
		return err
	}
	data, err := imageproc.Encode(img, m.Format(), m.JPEGQuality())
	if err != nil {
		m.SetStatus(i18n.StatusEncodeFailed, err)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.SetStatus(i18n.StatusSaveFailed, err)
		return err
	}
	m.SetStatus(i18n.StatusSaved, path)
	return nil
}

func (m *ImageModel) ExportPNG() ([]byte, error) {
	img, err := m.Processed()
	if err != nil {
		m.SetStatus(i18n.StatusExportFailed, err)
		return nil, err
	}
	data, err := imageproc.EncodePNG(img)
	if err != nil {
		m.SetStatus(i18n.StatusEncodeFailed, err)
		return nil, err
	}
	return data, nil
}

func (m *ImageModel) SuggestedFilename() string {
	base := strings.TrimSuffix(m.sourceName, filepath.Ext(m.sourceName))
	if base == "" {
		base = "image"
	}
	return base + "-edited" + imageproc.Extension(m.Format())
}

func (m *ImageModel) SetKeepAspect(keep bool) {
	m.ensureDefaults()
	m.keepAspect = keep
	m.keepAspectInitialized = true
	if keep {
		m.params.Height = imageproc.HeightForWidth(m.BaseSize(), max(1, m.params.Width))
	}
	m.markDirty()
}

func (m *ImageModel) SetWidth(width int) {
	m.ensureDefaults()
	m.params.Width = clampDim(width)
	if m.KeepAspect() {
		m.params.Height = imageproc.HeightForWidth(m.BaseSize(), m.params.Width)
	}
	m.markDirty()
}

func (m *ImageModel) SetHeight(height int) {
	m.ensureDefaults()
	m.params.Height = clampDim(height)
	if m.KeepAspect() {
		m.params.Width = imageproc.WidthForHeight(m.BaseSize(), m.params.Height)
	}
	m.markDirty()
}

func (m *ImageModel) SetScalePercent(percent int) {
	m.ensureDefaults()
	size := imageproc.ScaleSize(m.BaseSize(), percent)
	m.params.Width = size.X
	m.params.Height = size.Y
	m.markDirty()
}

func (m *ImageModel) ResetSize() {
	m.ensureDefaults()
	size := m.BaseSize()
	m.params.Width = max(1, size.X)
	m.params.Height = max(1, size.Y)
	m.markDirty()
}

func (m *ImageModel) SetCropEnabled(enabled bool) {
	m.ensureDefaults()
	m.params.CropEnabled = enabled
	if enabled && m.params.Crop.Empty() && m.source != nil {
		m.params.Crop = m.source.Bounds()
	}
	if m.KeepAspect() && m.params.Width > 0 {
		m.params.Height = imageproc.HeightForWidth(m.BaseSize(), m.params.Width)
	}
	m.markDirty()
}

func (m *ImageModel) SetCrop(rect image.Rectangle) {
	m.ensureDefaults()
	if m.source != nil {
		rect = rect.Intersect(m.source.Bounds())
	}
	if rect.Empty() {
		return
	}
	m.params.Crop = rect
	m.params.CropEnabled = true
	if m.KeepAspect() && m.params.Width > 0 {
		m.params.Height = imageproc.HeightForWidth(m.BaseSize(), m.params.Width)
	}
	m.markDirty()
}

func (m *ImageModel) SetCropX(x int) {
	c := m.Crop()
	m.SetCrop(image.Rect(x, c.Min.Y, x+c.Dx(), c.Max.Y))
}

func (m *ImageModel) SetCropY(y int) {
	c := m.Crop()
	m.SetCrop(image.Rect(c.Min.X, y, c.Max.X, y+c.Dy()))
}

func (m *ImageModel) SetCropWidth(w int) {
	c := m.Crop()
	m.SetCrop(image.Rect(c.Min.X, c.Min.Y, c.Min.X+max(1, w), c.Max.Y))
}

func (m *ImageModel) SetCropHeight(h int) {
	c := m.Crop()
	m.SetCrop(image.Rect(c.Min.X, c.Min.Y, c.Max.X, c.Min.Y+max(1, h)))
}

func (m *ImageModel) ResetCrop() {
	if m.source == nil {
		return
	}
	m.params.Crop = m.source.Bounds()
	if m.KeepAspect() && m.params.Width > 0 {
		m.params.Height = imageproc.HeightForWidth(m.BaseSize(), m.params.Width)
	}
	m.markDirty()
}

func (m *ImageModel) RotateDegrees() int {
	return imageproc.NormalizeDegrees(m.params.RotateDegrees)
}

func (m *ImageModel) SetRotateDegrees(degrees int) {
	m.ensureDefaults()
	degrees = imageproc.NormalizeDegrees(degrees)
	if m.params.RotateDegrees == degrees {
		return
	}
	percent := m.ScalePercent()
	m.params.RotateDegrees = degrees
	if m.source != nil {
		size := imageproc.ScaleSize(m.BaseSize(), percent)
		m.params.Width = size.X
		m.params.Height = size.Y
	}
	m.markDirty()
}

func (m *ImageModel) Rotate90() {
	m.SetRotateDegrees(m.RotateDegrees() + 90)
}

func (m *ImageModel) ResetRotate() {
	m.SetRotateDegrees(0)
}

func (m *ImageModel) SetFormat(format imageproc.Format) {
	m.ensureDefaults()
	if format != imageproc.FormatJPEG && format != imageproc.FormatPNG {
		format = imageproc.FormatPNG
	}
	m.params.Format = format
}

func (m *ImageModel) SetJPEGQuality(q int) {
	m.ensureDefaults()
	if q < 1 {
		q = 1
	}
	if q > 100 {
		q = 100
	}
	m.params.JPEGQuality = q
}

func (m *ImageModel) ensureDefaults() {
	if m.params.Format == "" {
		m.params.Format = imageproc.FormatPNG
	}
	if m.params.JPEGQuality == 0 {
		m.params.JPEGQuality = imageproc.DefaultJPEGQuality
	}
	if !m.keepAspectInitialized {
		m.keepAspect = true
		m.keepAspectInitialized = true
	}
}

func (m *ImageModel) setSource(img image.Image, name string, format imageproc.Format) {
	m.ensureDefaults()
	m.source = img
	m.sourceName = name
	if format == imageproc.FormatJPEG || format == imageproc.FormatPNG {
		m.params.Format = format
	}
	m.params.Crop = img.Bounds()
	m.params.Width = img.Bounds().Dx()
	m.params.Height = img.Bounds().Dy()
	m.params.RotateDegrees = 0
	m.resetPaintLayer()
	m.markDirty()
}

func (m *ImageModel) markDirty() {
	m.dirty = true
	m.generation++
}

func (m *ImageModel) recompute() {
	if !m.dirty {
		return
	}
	m.dirty = false
	m.ensureDefaults()
	if m.source == nil {
		m.processed = nil
		m.processErr = nil
		m.srcPreview = nil
		m.dstPreview = nil
		return
	}
	m.srcPreview = nil
	m.dstPreview = nil
	src := m.source
	if m.paintUsed && m.paintLayer != nil {
		src = imageproc.CompositeOver(m.source, m.paintLayer)
	}
	out, err := imageproc.Apply(src, m.params)
	m.processed = out
	m.processErr = err
}

func (m *ImageModel) SourcePreview() image.Image {
	m.recompute()
	if m.srcPreview == nil && m.source != nil {
		m.srcPreview = previewImage(m.source)
	}
	return m.srcPreview
}

func (m *ImageModel) ResultPreview() image.Image {
	m.recompute()
	if m.dstPreview == nil && m.processed != nil && m.processErr == nil {
		m.dstPreview = previewImage(m.processed)
	}
	return m.dstPreview
}

func previewImage(src image.Image) image.Image {
	if src == nil {
		return nil
	}
	size := src.Bounds().Size()
	if size.X <= previewMaxEdge && size.Y <= previewMaxEdge {
		return src
	}
	scale := float64(previewMaxEdge) / float64(max(size.X, size.Y))
	w := max(1, int(float64(size.X)*scale))
	h := max(1, int(float64(size.Y)*scale))
	return imageproc.Resize(src, w, h)
}

func clampDim(v int) int {
	if v < 1 {
		return 1
	}
	if v > imageproc.MaxDimension {
		return imageproc.MaxDimension
	}
	return v
}

func isImageFilename(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}
