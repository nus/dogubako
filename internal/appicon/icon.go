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

	xdraw "golang.org/x/image/draw"
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

const windowIconEdge = 128

// RGBA returns a window-sized icon. The embedded master is 1024px; keeping
// that raster would cost 4MB for a title-bar glyph.
func RGBA() (*image.RGBA, error) {
	once.Do(func() {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			loadErr = fmt.Errorf("decode icon: %w", err)
			return
		}
		master = scaleRGBA(img, windowIconEdge)
	})
	return master, loadErr
}

func scaleRGBA(src image.Image, edge int) *image.RGBA {
	b := src.Bounds()
	if b.Dx() <= edge && b.Dy() <= edge {
		return toRGBA(src)
	}
	s := float64(edge) / float64(max(b.Dx(), b.Dy()))
	w := max(1, int(float64(b.Dx())*s))
	h := max(1, int(float64(b.Dy())*s))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
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
