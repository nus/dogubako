package app

import (
	"fmt"
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nus/dogubako/internal/imageproc"
)

// ToolID identifies a workspace tool shown in the sidebar.
type ToolID string

const (
	ToolImage ToolID = "image"
)

// Tool is a sidebar entry.
type Tool struct {
	ID    ToolID
	Title string
}

// Tools is the ordered sidebar catalog. Add new tools here.
var Tools = []Tool{
	{ID: ToolImage, Title: "画像"},
}

// Model is the application-wide state provided to widgets via Env.
type Model struct {
	mode  ToolID
	image ImageModel
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

// ImageModel holds the first tool: resize, crop, and JPEG/PNG conversion.
type ImageModel struct {
	generation uint64

	source     image.Image
	sourceName string

	// keepAspect defaults to true until the user changes it.
	keepAspect            bool
	keepAspectInitialized bool

	params imageproc.Params

	processed  image.Image
	processErr error
	dirty      bool

	srcPreview *ebiten.Image
	dstPreview *ebiten.Image

	status string
}

const previewMaxEdge = 2048

func (m *ImageModel) Generation() uint64 { return m.generation }
func (m *ImageModel) Status() string     { return m.status }

func (m *ImageModel) SetStatus(status string) {
	if m.status == status {
		return
	}
	m.status = status
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
	if m.params.CropEnabled {
		c := m.params.Crop.Intersect(m.source.Bounds())
		if !c.Empty() {
			return c.Size()
		}
	}
	return m.source.Bounds().Size()
}

func (m *ImageModel) Params() imageproc.Params { return m.params }

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
		m.SetStatus(fmt.Sprintf("読み込みに失敗しました: %v", err))
		return err
	}
	m.setSource(img, name, format)
	m.SetStatus(fmt.Sprintf("%s を読み込みました（%d×%d）", name, img.Bounds().Dx(), img.Bounds().Dy()))
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
		m.SetStatus("ドロップされたファイルに画像がありません")
		return fmt.Errorf("no image in dropped files")
	}
	return loadErr
}

func (m *ImageModel) LoadClipboardPNG(png []byte) error {
	if len(png) == 0 {
		m.SetStatus("クリップボードに画像がありません")
		return fmt.Errorf("clipboard has no png")
	}
	img, format, err := imageproc.DecodeBytes(png)
	if err != nil {
		m.SetStatus(fmt.Sprintf("クリップボードの画像を読めません: %v", err))
		return err
	}
	m.setSource(img, "clipboard.png", format)
	m.SetStatus(fmt.Sprintf("クリップボードから貼り付けました（%d×%d）", img.Bounds().Dx(), img.Bounds().Dy()))
	return nil
}

func (m *ImageModel) SavePath(path string) error {
	img, err := m.Processed()
	if err != nil {
		m.SetStatus(fmt.Sprintf("書き出せません: %v", err))
		return err
	}
	data, err := imageproc.Encode(img, m.Format(), m.JPEGQuality())
	if err != nil {
		m.SetStatus(fmt.Sprintf("エンコードに失敗しました: %v", err))
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.SetStatus(fmt.Sprintf("保存に失敗しました: %v", err))
		return err
	}
	m.SetStatus(fmt.Sprintf("保存しました: %s", path))
	return nil
}

func (m *ImageModel) ExportPNG() ([]byte, error) {
	img, err := m.Processed()
	if err != nil {
		m.SetStatus(fmt.Sprintf("書き出せません: %v", err))
		return nil, err
	}
	data, err := imageproc.EncodePNG(img)
	if err != nil {
		m.SetStatus(fmt.Sprintf("エンコードに失敗しました: %v", err))
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
	out, err := imageproc.Apply(m.source, m.params)
	m.processed = out
	m.processErr = err
}

func (m *ImageModel) SourcePreview() *ebiten.Image {
	m.recompute()
	if m.srcPreview == nil && m.source != nil {
		m.srcPreview = previewImage(m.source)
	}
	return m.srcPreview
}

func (m *ImageModel) ResultPreview() *ebiten.Image {
	m.recompute()
	if m.dstPreview == nil && m.processed != nil && m.processErr == nil {
		m.dstPreview = previewImage(m.processed)
	}
	return m.dstPreview
}

func previewImage(src image.Image) *ebiten.Image {
	if src == nil {
		return nil
	}
	size := src.Bounds().Size()
	if size.X <= previewMaxEdge && size.Y <= previewMaxEdge {
		return ebiten.NewImageFromImage(src)
	}
	scale := float64(previewMaxEdge) / float64(max(size.X, size.Y))
	w := max(1, int(float64(size.X)*scale))
	h := max(1, int(float64(size.Y)*scale))
	return ebiten.NewImageFromImage(imageproc.Resize(src, w, h))
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
