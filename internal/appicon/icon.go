// Package appicon holds the application icon used by the window and packaging.
package appicon

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"sync"
)

//go:embed icon.png
var pngBytes []byte

var (
	once    sync.Once
	master  *image.RGBA
	loadErr error
)

// PNG returns the original icon file bytes.
func PNG() []byte {
	return pngBytes
}

// RGBA returns the decoded icon as a top-left-origin RGBA image.
func RGBA() (*image.RGBA, error) {
	once.Do(func() {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			loadErr = fmt.Errorf("decode icon: %w", err)
			return
		}
		master = toRGBA(img)
	})
	return master, loadErr
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok && rgba.Bounds().Min.Eq(image.Pt(0, 0)) {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}
