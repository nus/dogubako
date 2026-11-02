package app

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Tool icons are stroked vectors in the item text color. Color emoji fonts
// are not used; Apple Color Emoji alone is ~180MB on disk and can take over
// 1GB in RSS. The shapes are the same on macOS and Ubuntu.

func glyphStroke(r image.Rectangle) float32 {
	sw := float32(min(r.Dx(), r.Dy())) * 0.08
	if sw < 1 {
		sw = 1
	}
	return sw
}

func strokeGlyphPath(dst *ebiten.Image, p *vector.Path, sw float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, p, &vector.StrokeOptions{
		Width:      sw,
		LineCap:    vector.LineCapRound,
		LineJoin:   vector.LineJoinRound,
		MiterLimit: 4,
	}, op)
}

func strokeRoundRect(dst *ebiten.Image, x, y, w, h, rad, sw float32, clr color.Color) {
	if w <= sw || h <= sw {
		return
	}
	x += sw / 2
	y += sw / 2
	w -= sw
	h -= sw
	if rad > w/2 {
		rad = w / 2
	}
	if rad > h/2 {
		rad = h / 2
	}
	if rad < 0 {
		rad = 0
	}
	var p vector.Path
	p.MoveTo(x+rad, y)
	p.LineTo(x+w-rad, y)
	p.QuadTo(x+w, y, x+w, y+rad)
	p.LineTo(x+w, y+h-rad)
	p.QuadTo(x+w, y+h, x+w-rad, y+h)
	p.LineTo(x+rad, y+h)
	p.QuadTo(x, y+h, x, y+h-rad)
	p.LineTo(x, y+rad)
	p.QuadTo(x, y, x+rad, y)
	p.Close()
	strokeGlyphPath(dst, &p, sw, clr)
}

func drawToolGlyph(dst *ebiten.Image, bounds image.Rectangle, id ToolID, fg color.Color) {
	switch id {
	case ToolImage:
		drawFrameGlyph(dst, bounds, fg)
	case ToolScreenshot:
		drawScreenshotGlyph(dst, bounds, fg)
	case ToolAndroid:
		drawPhoneGlyph(dst, bounds, fg)
	case ToolStopwatch:
		drawStopwatchGlyph(dst, bounds, fg)
	case ToolGit:
		r := iconInner(bounds)
		sw := float32(1.25)
		if r.Dx() >= 4 && r.Dy() >= 4 {
			sw = glyphStroke(r)
		}
		drawBranchMark(dst, bounds, fg, sw)
	case ToolMTP:
		drawMTPGlyph(dst, bounds, fg)
	}
}

// drawFrameGlyph is a picture frame: a thick border and a peak inside it.
func drawFrameGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	w := float32(r.Dx())
	h := float32(r.Dy())
	sw := glyphStroke(r)
	border := w * 0.16
	if border < sw {
		border = sw
	}
	vector.StrokeRect(dst, x+border/2, y+border/2, w-border, h-border, border, fg, true)
	inset := border + sw*0.5
	if w-2*inset < 4 || h-2*inset < 4 {
		return
	}
	bot := y + h - inset
	vector.StrokeLine(dst, x+inset, bot, x+w*0.40, y+h*0.42, sw, fg, true)
	vector.StrokeLine(dst, x+w*0.40, y+h*0.42, x+w-inset, bot, sw, fg, true)
}

// drawScreenshotGlyph is a screen with crop corners at the top-left and bottom-right.
func drawScreenshotGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	w := float32(r.Dx())
	h := float32(r.Dy())
	sw := glyphStroke(r)
	sx := x + w*0.22
	sy := y + h*0.22
	swd := w * 0.56
	sh := h * 0.56
	vector.StrokeRect(dst, sx, sy, swd, sh, sw, fg, true)
	vector.StrokeLine(dst, sx, sy+sh*0.28, sx+swd, sy+sh*0.28, sw, fg, true)
	arm := w * 0.36
	var tl vector.Path
	tl.MoveTo(x+w*0.02, y+arm)
	tl.LineTo(x+w*0.02, y+h*0.02)
	tl.LineTo(x+arm, y+h*0.02)
	strokeGlyphPath(dst, &tl, sw*1.35, fg)
	var br vector.Path
	br.MoveTo(x+w-arm, y+h*0.98)
	br.LineTo(x+w*0.98, y+h*0.98)
	br.LineTo(x+w*0.98, y+h-arm)
	strokeGlyphPath(dst, &br, sw*1.35, fg)
}

// drawPhoneGlyph is a portrait handset, distinct from the landscape MTP device.
func drawPhoneGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	w := float32(r.Dx())
	h := float32(r.Dy())
	sw := glyphStroke(r)
	pw := w * 0.52
	ph := h * 0.94
	px := x + (w-pw)/2
	py := y + (h-ph)/2
	strokeRoundRect(dst, px, py, pw, ph, pw*0.18, sw, fg)
	vector.StrokeLine(dst, x+w*0.40, y+h*0.16, x+w*0.60, y+h*0.16, sw, fg, true)
	vector.StrokeLine(dst, x+w*0.40, y+h*0.84, x+w*0.60, y+h*0.84, sw, fg, true)
}

func drawStopwatchGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	w := float32(r.Dx())
	h := float32(r.Dy())
	sw := glyphStroke(r)
	cx := x + w*0.50
	cy := y + h*0.58
	rad := w * 0.34
	if rad < 2 {
		rad = 2
	}
	btnW := w * 0.16
	btnH := h * 0.16
	top := cy - rad
	vector.FillRect(dst, cx-btnW/2, top-btnH*0.55, btnW, btnH, fg, true)
	vector.StrokeCircle(dst, cx, cy, rad, sw, fg, true)
	vector.StrokeLine(dst, cx, cy, cx, cy-rad*0.55, sw, fg, true)
	vector.StrokeLine(dst, cx, cy, cx+rad*0.42, cy+rad*0.12, sw, fg, true)
	vector.FillCircle(dst, cx, cy, sw*0.65, fg, true)
}

// drawMTPGlyph is a landscape device overlapping an outline folder.
// The handset is wider than the Android phone, and the folder is a stroke
// rather than the filled file-tree folder.
func drawMTPGlyph(dst *ebiten.Image, bounds image.Rectangle, fg color.Color) {
	r := iconInner(bounds)
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Min.Y)
	w := float32(r.Dx())
	h := float32(r.Dy())
	sw := glyphStroke(r)
	dx := x + w*0.30
	dy := y
	dw := w * 0.70
	dh := h * 0.46
	strokeRoundRect(dst, dx, dy, dw, dh, dw*0.12, sw, fg)
	padX := dw * 0.14
	padY := dh * 0.24
	vector.StrokeRect(dst, dx+padX, dy+padY, dw-2*padX, dh-2*padY, sw, fg, true)

	fx := x
	fy := y + h*0.40
	fw := w * 0.64
	fh := h * 0.58
	tabH := fh * 0.28
	tabW := fw * 0.46
	var p vector.Path
	p.MoveTo(fx, fy+tabH)
	p.LineTo(fx, fy+fh)
	p.LineTo(fx+fw, fy+fh)
	p.LineTo(fx+fw, fy+tabH)
	p.LineTo(fx+tabW, fy+tabH)
	p.LineTo(fx+tabW, fy)
	p.LineTo(fx, fy)
	p.Close()
	strokeGlyphPath(dst, &p, sw, fg)
}
