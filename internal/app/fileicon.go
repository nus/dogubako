package app

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

// fileKindIcon draws a folder or file glyph. Color emoji fonts are not used;
// Apple Color Emoji alone is ~180MB on disk and can take over 1GB in RSS.
type fileKindIcon struct {
	guigui.DefaultWidget

	folder bool
}

func (k *fileKindIcon) SetFolder(folder bool) {
	k.folder = folder
}

func (k *fileKindIcon) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteBool(k.folder)
}

func (k *fileKindIcon) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	fg := color.Color(color.NRGBA{R: 0xc8, G: 0xc8, B: 0xc8, A: 0xff})
	if v, ok := context.Env(k, basicwidget.EnvKeyListItemColorType); ok {
		if ct, ok := v.(basicwidget.ListItemColorType); ok {
			if c := ct.TextColor(context); c != nil {
				fg = c
			}
		}
	}
	if k.folder {
		drawFolderGlyph(dst, b, fg)
		return
	}
	drawFileGlyph(dst, b, fg)
}

func iconInner(b image.Rectangle) image.Rectangle {
	pad := max(1, min(b.Dx(), b.Dy())/6)
	return image.Rect(b.Min.X+pad, b.Min.Y+pad, b.Max.X-pad, b.Max.Y-pad)
}

func drawFolderGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	tabH := max(2, r.Dy()/5)
	tabW := r.Dx() / 2
	body := image.Rect(r.Min.X, r.Min.Y+tabH, r.Max.X, r.Max.Y)
	tab := image.Rect(r.Min.X, r.Min.Y, r.Min.X+tabW, r.Min.Y+tabH+1)
	fill := color.NRGBA{R: 0xe6, G: 0xc2, B: 0x6a, A: 0xff}
	if n, ok := fg.(color.NRGBA); ok && n.A > 0 {
		fill.A = n.A
	}
	vector.FillRect(dst, float32(tab.Min.X), float32(tab.Min.Y), float32(tab.Dx()), float32(tab.Dy()), fill, false)
	vector.FillRect(dst, float32(body.Min.X), float32(body.Min.Y), float32(body.Dx()), float32(body.Dy()), fill, false)
	vector.StrokeRect(dst, float32(body.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(body.Dx())-1, float32(r.Dy())-1, 1, fg, false)
}

func drawFileGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	fold := max(3, r.Dx()/3)
	page := color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	if n, ok := fg.(color.NRGBA); ok && n.A > 0 {
		page.A = n.A
	}
	vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), page, false)
	vector.FillRect(dst, float32(r.Max.X-fold), float32(r.Min.Y), float32(fold), float32(fold), color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}, false)
	vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(r.Dx())-1, float32(r.Dy())-1, 1, fg, false)
}
