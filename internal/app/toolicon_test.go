package app

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestToolGlyphsDraw(t *testing.T) {
	img := ebiten.NewImage(32, 32)
	fg := color.NRGBA{R: 0xc4, G: 0x3a, B: 0x1a, A: 0xff}
	for _, id := range []ToolID{ToolImage, ToolScreenshot, ToolAndroid, ToolStopwatch, ToolGit, ToolMTP, ToolID("missing")} {
		drawToolGlyph(img, img.Bounds(), id, fg)
	}
	img.Deallocate()
}

func TestToolGlyphsTinyBounds(t *testing.T) {
	img := ebiten.NewImage(3, 8)
	fg := color.NRGBA{A: 0xff}
	for _, id := range []ToolID{ToolImage, ToolScreenshot, ToolAndroid, ToolStopwatch, ToolGit, ToolMTP} {
		drawToolGlyph(img, img.Bounds(), id, fg)
	}
	img.Deallocate()
}
