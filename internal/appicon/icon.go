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

// WindowRGBA returns a downscaled copy for the native window icon.
func WindowRGBA() (*image.RGBA, error) {
	src, err := RGBA()
	if err != nil {
		return nil, err
	}
	if src.Bounds().Dx() == windowIconSize && src.Bounds().Dy() == windowIconSize {
		return src, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, windowIconSize, windowIconSize))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst, nil
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
