package app

import (
	"image"
	"image/color"

	"github.com/nus/dogubako/internal/imageproc"
)

func (m *ImageModel) PaintColor() color.NRGBA {
	m.ensurePaintDefaults()
	return m.paintColor
}

func (m *ImageModel) SetPaintColor(c color.NRGBA) {
	m.ensurePaintDefaults()
	if c.A == 0 {
		c.A = 255
	}
	m.paintColor = c
	m.generation++
}

func (m *ImageModel) BrushSize() int {
	m.ensurePaintDefaults()
	return m.brushSize
}

func (m *ImageModel) SetBrushSize(size int) {
	m.ensurePaintDefaults()
	size = imageproc.ClampBrushSize(size)
	if m.brushSize == size {
		return
	}
	m.brushSize = size
	m.generation++
}

func (m *ImageModel) PaintTool() PaintTool {
	if m.paintTool == PaintEraser {
		return PaintEraser
	}
	return PaintBrush
}

func (m *ImageModel) SetPaintTool(tool PaintTool) {
	if tool != PaintEraser {
		tool = PaintBrush
	}
	if m.PaintTool() == tool {
		return
	}
	m.paintTool = tool
	m.generation++
}

func (m *ImageModel) HasPaint() bool { return m.paintUsed }

func (m *ImageModel) PaintLayer() *image.NRGBA { return m.paintLayer }

func (m *ImageModel) CanUndoPaint() bool { return len(m.paintUndo) > 0 }

func (m *ImageModel) BeginPaintStroke(pt image.Point) {
	if m.source == nil {
		return
	}
	m.ensurePaintLayer()
	m.pushPaintUndo()
	m.stroking = true
	m.strokeLast = pt
	m.stampPaint(pt)
}

func (m *ImageModel) ContinuePaintStroke(pt image.Point) {
	if !m.stroking || m.paintLayer == nil {
		return
	}
	imageproc.StrokeBrush(m.paintLayer, m.strokeLast, pt, m.BrushSize(), m.PaintColor(), m.PaintTool() == PaintEraser)
	m.strokeLast = pt
	if m.PaintTool() != PaintEraser {
		m.paintUsed = true
	}
	m.generation++
}

func (m *ImageModel) EndPaintStroke() {
	if !m.stroking {
		return
	}
	m.stroking = false
	m.paintUsed = paintLayerUsed(m.paintLayer)
	m.markDirty()
}

func (m *ImageModel) UndoPaint() {
	if len(m.paintUndo) == 0 {
		return
	}
	last := m.paintUndo[len(m.paintUndo)-1]
	m.paintUndo = m.paintUndo[:len(m.paintUndo)-1]
	m.paintLayer = last
	m.paintUsed = paintLayerUsed(last)
	m.stroking = false
	m.markDirty()
}

func (m *ImageModel) ClearPaint() {
	if m.source == nil || (!m.paintUsed && m.paintLayer == nil) {
		return
	}
	m.ensurePaintLayer()
	m.pushPaintUndo()
	clearNRGBA(m.paintLayer)
	m.paintUsed = false
	m.stroking = false
	m.markDirty()
}

func (m *ImageModel) stampPaint(pt image.Point) {
	erase := m.PaintTool() == PaintEraser
	imageproc.StampBrush(m.paintLayer, pt, m.BrushSize(), m.PaintColor(), erase)
	if !erase {
		m.paintUsed = true
	}
	m.generation++
}

func (m *ImageModel) ensurePaintDefaults() {
	if m.brushSize == 0 {
		m.brushSize = imageproc.DefaultBrushSize
	}
	if m.paintColor.A == 0 && m.paintColor.R == 0 && m.paintColor.G == 0 && m.paintColor.B == 0 {
		m.paintColor = color.NRGBA{A: 255}
	}
	if m.paintTool == "" {
		m.paintTool = PaintBrush
	}
}

func (m *ImageModel) ensurePaintLayer() {
	m.ensurePaintDefaults()
	if m.source == nil {
		return
	}
	size := m.source.Bounds().Size()
	if m.paintLayer != nil && m.paintLayer.Bounds().Dx() == size.X && m.paintLayer.Bounds().Dy() == size.Y {
		return
	}
	m.paintLayer = image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
}

func (m *ImageModel) resetPaintLayer() {
	m.paintLayer = nil
	m.paintUsed = false
	m.paintUndo = nil
	m.stroking = false
}

func (m *ImageModel) pushPaintUndo() {
	if m.paintLayer == nil {
		return
	}
	m.paintUndo = append(m.paintUndo, imageproc.CloneNRGBA(m.paintLayer))
	if extra := len(m.paintUndo) - imageproc.MaxPaintUndo; extra > 0 {
		m.paintUndo = m.paintUndo[extra:]
	}
}

func paintLayerUsed(layer *image.NRGBA) bool {
	if layer == nil {
		return false
	}
	for i := 3; i < len(layer.Pix); i += 4 {
		if layer.Pix[i] != 0 {
			return true
		}
	}
	return false
}

func clearNRGBA(img *image.NRGBA) {
	if img == nil {
		return
	}
	for i := range img.Pix {
		img.Pix[i] = 0
	}
}
