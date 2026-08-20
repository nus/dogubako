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

// windowIconSize is small enough for gogpu's X11 _NET_WM_ICON property.
// ChangeProperty encodes the request length as uint16; a 1024×1024 CARDINAL
// buffer overflows that field and desynchronizes the X11 connection so the
// window never maps.
const windowIconSize = 128

//go:embed icon.png
var pngBytes []byte

var (
	fullOnce sync.Once
	master   *image.RGBA
	fullErr  error

	winOnce sync.Once
	window  *image.RGBA
	winErr  error
)

// PNG returns the original icon file bytes.
func PNG() []byte {
	return pngBytes
}

// RGBA returns the decoded icon as a top-left-origin RGBA image.
func RGBA() (*image.RGBA, error) {
	fullOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			fullErr = fmt.Errorf("decode icon: %w", err)
			return
		}
		master = toRGBA(img)
	})
	return master, fullErr
}

// WindowRGBA returns a downscaled copy for the native window icon.
// The 1024px master is not kept; only this 128px bitmap is cached at runtime.
func WindowRGBA() (*image.RGBA, error) {
	winOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			winErr = fmt.Errorf("decode icon: %w", err)
			return
		}
		if img.Bounds().Dx() == windowIconSize && img.Bounds().Dy() == windowIconSize {
			window = toRGBA(img)
			return
		}
		window = image.NewRGBA(image.Rect(0, 0, windowIconSize, windowIconSize))
		xdraw.CatmullRom.Scale(window, window.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	})
	return window, winErr
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
