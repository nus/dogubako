package openh264

import (
	"image/color"
	"testing"
)

func TestPackI420BlackAndRed(t *testing.T) {
	w, h := 2, 2
	y := []byte{16, 16, 16, 16}
	u := []byte{128}
	v := []byte{128}
	img := packI420(y, u, v, w, h, 2, 1)
	if c := img.NRGBAAt(0, 0); c.R > 2 || c.G > 2 || c.B > 2 || c.A != 255 {
		t.Fatalf("black = %+v", c)
	}

	// Limited-range red-ish: Y=81, U=90, V=240
	y = []byte{81, 81, 81, 81}
	u = []byte{90}
	v = []byte{240}
	img = packI420(y, u, v, w, h, 2, 1)
	c := img.NRGBAAt(1, 1)
	if c.R < 200 || c.G > 80 || c.B > 80 {
		t.Fatalf("red = %+v", c)
	}
	if img.NRGBAAt(0, 0) != (color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}) {
		t.Fatal("expected uniform")
	}
}

func TestI420RejectsBadSize(t *testing.T) {
	var z byte
	if _, err := i420ToNRGBA(&z, &z, &z, 0, 10, 16, 8); err == nil {
		t.Fatal("expected bad size")
	}
}
