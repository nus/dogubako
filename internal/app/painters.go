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
	inner material3.DataTablePainter
}

func newQuietTablePainter(theme *material3.Theme) quietTablePainter {
	return quietTablePainter{inner: material3.DataTablePainter{Theme: theme}}
}

func (p quietTablePainter) PaintHeader(canvas widget.Canvas, bounds geometry.Rect, state datatable.HeaderPaintState) {
	p.inner.PaintHeader(canvas, bounds, state)
}

func (p quietTablePainter) PaintHeaderCell(canvas widget.Canvas, bounds geometry.Rect, state datatable.HeaderCellPaintState) {
	p.inner.PaintHeaderCell(canvas, bounds, state)
}

func (p quietTablePainter) PaintRow(canvas widget.Canvas, state datatable.RowPaintState) {
	p.inner.PaintRow(canvas, state)
}

func (p quietTablePainter) PaintCell(canvas widget.Canvas, state datatable.CellPaintState) {
	p.inner.PaintCell(canvas, state)
}

func (p quietTablePainter) PaintEmptyState(widget.Canvas, geometry.Rect) {}

var (
	_ textfield.Painter       = compactTFPainter{}
	_ textfield.LayoutMetrics = compactTFPainter{}
)
