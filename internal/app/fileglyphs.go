package app

import (
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/nus/dogubako/internal/cjkembed"
)

const (
	androidNamePadX    float32 = 12
	androidNameIndent  float32 = 12
	androidNameExpandW float32 = 12
	androidNameIconW   float32 = 16
	androidNameGap     float32 = 4
)

type androidNameSlots struct {
	Expand geometry.Rect
	Icon   geometry.Rect
	Text   geometry.Rect
}

func androidNameSlotsOf(bounds geometry.Rect, depth int) androidNameSlots {
	if depth < 0 {
		depth = 0
	}
	x := bounds.Min.X + androidNamePadX + float32(depth)*androidNameIndent
	midY := bounds.Min.Y + bounds.Height()/2
	slot := func(w, h float32) geometry.Rect {
		return geometry.NewRect(x, midY-h/2, w, h)
	}
	exp := slot(androidNameExpandW, androidNameExpandW)
	x += androidNameExpandW + androidNameGap
	icon := slot(androidNameIconW, androidNameIconW)
	x += androidNameIconW + androidNameGap
	textW := bounds.Max.X - androidNamePadX - x
	if textW < 0 {
		textW = 0
	}
	text := geometry.NewRect(x, bounds.Min.Y, textW, bounds.Height())
	return androidNameSlots{Expand: exp, Icon: icon, Text: text}
}

func drawExpandGlyph(canvas widget.Canvas, r geometry.Rect, open bool, color widget.Color) {
	if r.IsEmpty() {
		return
	}
	cx := r.Min.X + r.Width()/2
	cy := r.Min.Y + r.Height()/2
	s := r.Width() * 0.28
	if s < 2 {
		s = 2
	}
	if open {
		canvas.DrawLine(geometry.Pt(cx-s, cy-s*0.45), geometry.Pt(cx, cy+s*0.55), color, 1.6)
		canvas.DrawLine(geometry.Pt(cx, cy+s*0.55), geometry.Pt(cx+s, cy-s*0.45), color, 1.6)
		return
	}
	canvas.DrawLine(geometry.Pt(cx-s*0.45, cy-s), geometry.Pt(cx+s*0.55, cy), color, 1.6)
	canvas.DrawLine(geometry.Pt(cx+s*0.55, cy), geometry.Pt(cx-s*0.45, cy+s), color, 1.6)
}

func drawFolderGlyph(canvas widget.Canvas, r geometry.Rect, open bool) {
	if r.IsEmpty() {
		return
	}
	tabH := r.Height() * 0.28
	tabW := r.Width() * 0.45
	tab := geometry.NewRect(r.Min.X, r.Min.Y+1, tabW, tabH+1)
	body := geometry.NewRect(r.Min.X, r.Min.Y+tabH*0.55, r.Width(), r.Height()-tabH*0.55)
	tabFill := widget.RGBA8(214, 154, 42, 255)
	bodyFill := widget.RGBA8(240, 186, 64, 255)
	if open {
		tabFill = widget.RGBA8(196, 138, 32, 255)
		bodyFill = widget.RGBA8(247, 206, 96, 255)
	}
	canvas.DrawRoundRect(tab, tabFill, 2)
	canvas.DrawRoundRect(body, bodyFill, 2)
	if open {
		lip := geometry.NewRect(r.Min.X+1.5, r.Min.Y+r.Height()*0.48, r.Width()-3, r.Height()*0.42)
		canvas.DrawRoundRect(lip, widget.RGBA8(255, 228, 150, 255), 1.5)
	}
}

func drawFileGlyph(canvas widget.Canvas, r geometry.Rect) {
	if r.IsEmpty() {
		return
	}
	body := widget.RGBA8(245, 247, 250, 255)
	edge := widget.RGBA8(148, 158, 174, 255)
	foldFill := widget.RGBA8(226, 232, 240, 255)
	canvas.DrawRoundRect(r, body, 2)
	canvas.StrokeRoundRect(r, edge, 2, 1)
	fold := r.Width() * 0.34
	if fold < 4 {
		fold = 4
	}
	foldR := geometry.NewRect(r.Max.X-fold, r.Min.Y, fold, fold)
	canvas.DrawRect(foldR, foldFill)
	canvas.DrawLine(geometry.Pt(r.Max.X-fold, r.Min.Y), geometry.Pt(r.Max.X-fold, r.Min.Y+fold), edge, 1)
	canvas.DrawLine(geometry.Pt(r.Max.X-fold, r.Min.Y+fold), geometry.Pt(r.Max.X, r.Min.Y+fold), edge, 1)
	lineX0 := r.Min.X + 3
	lineX1 := r.Max.X - 3
	if lineX1 > r.Max.X-fold-1 {
		lineX1 = r.Max.X - fold - 1
	}
	if lineX1 > lineX0+2 {
		y0 := r.Min.Y + fold + 2
		for i := 0; i < 3; i++ {
			y := y0 + float32(i)*3.2
			if y >= r.Max.Y-2 {
				break
			}
			canvas.DrawLine(geometry.Pt(lineX0, y), geometry.Pt(lineX1, y), widget.RGBA8(186, 194, 206, 255), 1)
		}
	}
}

func drawAndroidNameGlyphs(canvas widget.Canvas, bounds geometry.Rect, depth int, isDir, expanded, disabled bool, name string) {
	if bounds.IsEmpty() {
		return
	}
	canvas.PushClip(bounds)
	slots := androidNameSlotsOf(bounds, depth)
	mark := widget.RGBA8(90, 98, 110, 255)
	text := widget.RGBA8(33, 33, 33, 255)
	if disabled {
		mark = widget.RGBA8(90, 98, 110, 140)
		text = widget.RGBA8(33, 33, 33, 140)
	}
	if isDir {
		drawExpandGlyph(canvas, slots.Expand, expanded, mark)
		drawFolderGlyph(canvas, slots.Icon, expanded)
	} else {
		drawFileGlyph(canvas, slots.Icon)
	}
	if name != "" {
		drawStyledLabel(canvas, name, slots.Text, 13, text, widget.TextAlignLeft)
	}
	canvas.PopClip()
}

func drawStyledLabel(canvas widget.Canvas, s string, bounds geometry.Rect, size float32, color widget.Color, align widget.TextAlign) {
	if s == "" || bounds.IsEmpty() {
		return
	}
	style := widget.TextStyle{
		FontFamily: cjkembed.FamilyName,
		FontSize:   size,
		Color:      color,
		Align:      align,
	}
	if sd, ok := canvas.(widget.StyledTextDrawer); ok {
		sd.DrawStyledText(s, bounds, style)
		return
	}
	canvas.DrawText(s, bounds, size, color, false, align)
}
