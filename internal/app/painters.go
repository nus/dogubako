package app

import (
	"github.com/gogpu/ui/core/datatable"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

// compactTFPainter keeps Material 3 colors but uses guigui-like field metrics
// so 28px rows are not clipped by the default 16/12 padding.
type compactTFPainter struct {
	inner material3.TextFieldPainter
}

func newCompactTFPainter(theme *material3.Theme) compactTFPainter {
	return compactTFPainter{inner: material3.TextFieldPainter{Theme: theme}}
}

func (p compactTFPainter) PaintTextField(canvas widget.Canvas, st *textfield.PaintState) {
	p.inner.PaintTextField(canvas, st)
}

func (compactTFPainter) ContentPadding() (float32, float32) { return 8, 4 }
func (compactTFPainter) TextFieldFontSize() float32         { return 13 }
func (compactTFPainter) TextFieldCursorWidth() float32      { return 1 }
func (compactTFPainter) TextFieldCornerRadius() float32     { return 4 }

// quietListPainter keeps Material 3 selection colors but skips the English
// "No items" empty state, matching the old guigui lists.
type quietListPainter struct {
	inner material3.ListViewPainter
}

func newQuietListPainter(theme *material3.Theme) quietListPainter {
	return quietListPainter{inner: material3.ListViewPainter{Theme: theme}}
}

func (p quietListPainter) PaintDivider(canvas widget.Canvas, state listview.DividerState) {
	p.inner.PaintDivider(canvas, state)
}

func (p quietListPainter) PaintEmptyState(widget.Canvas, geometry.Rect) {}

func (p quietListPainter) PaintItemBackground(canvas widget.Canvas, state listview.ItemPaintState) {
	p.inner.PaintItemBackground(canvas, state)
}

func (p quietListPainter) PaintSelection(canvas widget.Canvas, state listview.ItemPaintState) {
	p.inner.PaintSelection(canvas, state)
}

// quietTablePainter keeps Material 3 table chrome but skips "No data".
type quietTablePainter struct {
	inner   material3.DataTablePainter
	fileRow func(int) (depth int, isDir, expanded bool, ok bool)
}

func newQuietTablePainter(theme *material3.Theme, fileRow func(int) (depth int, isDir, expanded bool, ok bool)) quietTablePainter {
	return quietTablePainter{inner: material3.DataTablePainter{Theme: theme}, fileRow: fileRow}
}

func (p quietTablePainter) PaintHeader(canvas widget.Canvas, bounds geometry.Rect, state datatable.HeaderPaintState) {
	p.inner.PaintHeader(canvas, bounds, state)
}

func (p quietTablePainter) PaintHeaderCell(canvas widget.Canvas, bounds geometry.Rect, state datatable.HeaderCellPaintState) {
	if bounds.IsEmpty() {
		return
	}
	if state.Hovered && state.Sortable && !state.Disabled {
		canvas.DrawRect(bounds, widget.RGBA8(0, 0, 0, 10))
	}
	label := state.Title
	if ind := state.SortDir.Indicator(); ind != "" {
		label = state.Title + " " + ind
	}
	fg := widget.RGBA8(28, 27, 31, 255)
	if state.Disabled {
		fg = widget.RGBA8(28, 27, 31, 97)
	}
	textBounds := geometry.NewRect(bounds.Min.X+12, bounds.Min.Y, bounds.Width()-24, bounds.Height())
	drawStyledLabel(canvas, label, textBounds, 13, fg, state.Align)
}

func (p quietTablePainter) PaintRow(canvas widget.Canvas, state datatable.RowPaintState) {
	p.inner.PaintRow(canvas, state)
}

func (p quietTablePainter) PaintCell(canvas widget.Canvas, state datatable.CellPaintState) {
	if p.fileRow != nil && state.ColIndex == 0 {
		depth, isDir, expanded, ok := p.fileRow(state.RowIndex)
		if ok {
			drawAndroidNameGlyphs(canvas, state.Bounds, depth, isDir, expanded, state.Disabled, state.Value)
			return
		}
	}
	p.inner.PaintCell(canvas, state)
}

func (p quietTablePainter) PaintEmptyState(widget.Canvas, geometry.Rect) {}

var (
	_ textfield.Painter       = compactTFPainter{}
	_ textfield.LayoutMetrics = compactTFPainter{}
)
