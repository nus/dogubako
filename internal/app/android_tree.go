package app

import (
	"github.com/gogpu/ui/core/datatable"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"
)

const (
	// androidTableRowHeight is the DataTable row height used by the Android file tree.
	androidTableRowHeight float32 = 28
	// androidTableHeaderHeight matches gogpu/ui datatable.defaultHeaderHeight.
	androidTableHeaderHeight float32 = 36
	// androidTableScrollbarReserve matches gogpu/ui scrollview track width
	// (scrollbarWidth + 2*scrollbarPadding).
	androidTableScrollbarReserve float32 = 12
)

// activateAndroidTreeRow selects a visible tree row and toggles directories.
func activateAndroidTreeRow(m *AndroidModel, row int) {
	rows := m.Rows()
	if row < 0 || row >= len(rows) {
		return
	}
	r := rows[row]
	m.SelectPath(r.Entry.Path)
	if r.Entry.IsDir {
		m.ToggleExpand(r.Entry.Path)
	}
}

// androidTreeRetogglePath returns the directory to ToggleExpand when a click
// lands on a row that is already selected. gogpu DataTable skips OnRowSelect
// in that case, so expand/collapse would otherwise stick open.
func androidTreeRetogglePath(selected string, rows []AndroidTreeRow, row int) string {
	if row < 0 || row >= len(rows) {
		return ""
	}
	r := rows[row]
	if !r.Entry.IsDir || r.Entry.Path != selected {
		return ""
	}
	return r.Entry.Path
}

func androidTreeHitsScrollbar(localX, tableW float32) bool {
	return tableW > androidTableScrollbarReserve && localX >= tableW-androidTableScrollbarReserve
}

// androidTreeHitRow maps table-local coordinates to a data row index.
// Header clicks, empty space, and the vertical scrollbar return -1.
func androidTreeHitRow(localX, localY, tableW, scrollY float32, rowCount int) int {
	if androidTreeHitsScrollbar(localX, tableW) {
		return -1
	}
	if localY < androidTableHeaderHeight {
		return -1
	}
	contentY := localY - androidTableHeaderHeight + scrollY
	if contentY < 0 || androidTableRowHeight <= 0 {
		return -1
	}
	row := int(contentY / androidTableRowHeight)
	if row < 0 || row >= rowCount {
		return -1
	}
	return row
}

// androidTreeTable wraps the file-tree DataTable so a second click on the
// already-selected folder still collapses it.
type androidTreeTable struct {
	widget.WidgetBase
	shell   *Shell
	table   *datatable.Widget
	scrollY state.Signal[float32]
}

func newAndroidTreeTable(s *Shell, table *datatable.Widget, scrollY state.Signal[float32]) *androidTreeTable {
	t := &androidTreeTable{shell: s, table: table, scrollY: scrollY}
	t.SetVisible(true)
	t.SetEnabled(true)
	if table != nil {
		t.AddChild(table)
	}
	return t
}

func (t *androidTreeTable) Layout(ctx widget.Context, cons geometry.Constraints) geometry.Size {
	if t.table == nil {
		size := cons.Constrain(geometry.Sz(0, 0))
		t.SetBounds(geometry.FromPointSize(t.Position(), size))
		return size
	}
	size := widget.LayoutChild(t.table, ctx, cons)
	t.table.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	t.SetBounds(geometry.FromPointSize(t.Position(), size))
	return size
}

func (t *androidTreeTable) Draw(ctx widget.Context, canvas widget.Canvas) {
	if t.table == nil {
		return
	}
	canvas.PushTransform(t.Bounds().Min)
	widget.StampScreenOrigin(t.table, canvas)
	widget.DrawChild(t.table, ctx, canvas)
	canvas.PopTransform()
}

func (t *androidTreeTable) Event(ctx widget.Context, e event.Event) bool {
	retoggle := ""
	if me, ok := e.(*event.MouseEvent); ok && me.MouseType == event.MousePress && me.Button == event.ButtonLeft {
		retoggle = t.retogglePath(me)
	}
	handled := false
	if t.table != nil {
		handled = t.table.Event(ctx, e)
	}
	if retoggle != "" && t.shell != nil {
		t.shell.model.Android().ToggleExpand(retoggle)
		t.shell.bump()
	}
	return handled
}

func (t *androidTreeTable) retogglePath(me *event.MouseEvent) string {
	if t.shell == nil {
		return ""
	}
	m := t.shell.model.Android()
	if m.Busy() {
		return ""
	}
	b := t.Bounds()
	if t.table != nil && !t.table.Bounds().IsEmpty() {
		b = t.table.Bounds()
	}
	scrollY := float32(0)
	if t.scrollY != nil {
		scrollY = t.scrollY.Get()
	}
	local := me.Position.Sub(b.Min)
	rows := m.Rows()
	row := androidTreeHitRow(local.X, local.Y, b.Width(), scrollY, len(rows))
	return androidTreeRetogglePath(m.Selected(), rows, row)
}
