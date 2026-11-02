package app

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

// sidebarTip is the collapsed tool name. basicwidget.TooltipArea waits half a
// second after the pointer enters; this opens on the hover tick itself.
type sidebarTip struct {
	guigui.DefaultWidget

	popup basicwidget.Popup
	body  sidebarTipBody

	text     string
	hovering bool
	wantOpen bool
	showArea image.Rectangle
}

func (t *sidebarTip) SetText(text string) {
	t.text = text
}

func (t *sidebarTip) Close() {
	t.hovering = false
	t.wantOpen = false
	if t.popup.IsOpen() {
		t.popup.SetOpen(false)
	}
}

func (t *sidebarTip) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(t.text)
	w.WriteBool(t.wantOpen)
	w.WriteBool(t.popup.IsOpen())
}

func (t *sidebarTip) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if t.popup.IsOpen() {
		adder.AddWidget(&t.popup)
	}
	t.popup.SetContent(&t.body)
	t.popup.SetModal(false)
	t.body.SetText(t.text)
	if t.wantOpen {
		t.wantOpen = false
		t.popup.SetOpen(true)
	}
	return nil
}

func (t *sidebarTip) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	tooltipSize := t.body.Measure(context, guigui.Constraints{})
	gap := basicwidget.UnitSize(context) / 8
	bounds := sidebarTipBounds(t.showArea, tooltipSize, context.AppBounds(), gap)
	layouter.LayoutWidget(&t.popup, bounds)
}

// sidebarTipBounds places the balloon to the right of the icon, centered on
// that row. It shifts only enough to stay inside the window.
func sidebarTipBounds(icon image.Rectangle, size image.Point, app image.Rectangle, gap int) image.Rectangle {
	x := icon.Max.X + gap
	y := icon.Min.Y + (icon.Dy()-size.Y)/2
	b := image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x+size.X, y+size.Y)}
	if b.Max.X > app.Max.X {
		b = b.Add(image.Pt(app.Max.X-b.Max.X, 0))
	}
	if b.Min.X < app.Min.X {
		b = b.Add(image.Pt(app.Min.X-b.Min.X, 0))
	}
	if b.Max.Y > app.Max.Y {
		b = b.Add(image.Pt(0, app.Max.Y-b.Max.Y))
	}
	if b.Min.Y < app.Min.Y {
		b = b.Add(image.Pt(0, app.Min.Y-b.Min.Y))
	}
	return b
}

func (t *sidebarTip) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (t *sidebarTip) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	area, hit := t.hitArea(context, widgetBounds)
	open, changed := sidebarTipTransition(hit, t.hovering)
	t.hovering = hit
	if !changed {
		return nil
	}
	if open {
		t.showArea = area
		t.wantOpen = true
		return nil
	}
	t.wantOpen = false
	if t.popup.IsOpen() {
		t.popup.SetOpen(false)
	}
	return nil
}

func (t *sidebarTip) hitArea(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (image.Rectangle, bool) {
	if !context.IsVisible(t) || !context.IsEnabled(t) {
		return image.Rectangle{}, false
	}
	bounds := widgetBounds.Bounds()
	if !image.Pt(ebiten.CursorPosition()).In(bounds) {
		return image.Rectangle{}, false
	}
	// Opening requires this widget to be the hit target. Once open, the
	// balloon can sit on the icon; that must not count as leaving.
	if !t.hovering && !widgetBounds.IsHitAtCursor() {
		return image.Rectangle{}, false
	}
	return bounds, true
}

// sidebarTipTransition opens the balloon on the tick the pointer enters and
// closes it on the tick the pointer leaves. There is no extra wait.
func sidebarTipTransition(hovering, wasHovering bool) (open, changed bool) {
	if hovering == wasHovering {
		return hovering, false
	}
	return hovering, true
}

type sidebarTipBody struct {
	guigui.DefaultWidget

	text  basicwidget.Text
	value string
	items []guigui.LinearLayoutItem
}

func (b *sidebarTipBody) SetText(value string) {
	b.value = value
}

func (b *sidebarTipBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&b.text)
	b.text.SetMultiline(true)
	b.text.SetValue(b.value)
	return nil
}

func (b *sidebarTipBody) layout(context *guigui.Context) guigui.LinearLayout {
	b.items = slices.Delete(b.items, 0, len(b.items))
	b.items = append(b.items, guigui.LinearLayoutItem{
		Widget: &b.text,
		Size:   guigui.FlexibleSize(1),
	})
	return guigui.LinearLayout{
		Items:   b.items,
		Padding: basicwidget.TooltipTextPadding(context),
	}
}

func (b *sidebarTipBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (b *sidebarTipBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return b.layout(context).Measure(context, constraints)
}
