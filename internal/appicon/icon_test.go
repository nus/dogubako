package appicon

import (
	"bytes"
	"image/png"
	"testing"
)

func TestPNGDecodes(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatalf("size %v", img.Bounds())
	}
}

func TestWindowRGBASize(t *testing.T) {
	img, err := WindowRGBA()
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != windowIconSize || img.Bounds().Dy() != windowIconSize {
		t.Fatalf("window icon size %v, want %d", img.Bounds(), windowIconSize)
	}
}

func TestWindowRGBADoesNotShare1024Master(t *testing.T) {
	win, err := WindowRGBA()
	if err != nil {
		t.Fatal(err)
	}
	full, err := RGBA()
	if err != nil {
		t.Fatal(err)
	}
	if win.Bounds().Dx() != windowIconSize {
		t.Fatalf("window = %v", win.Bounds())
	}
	if full.Bounds().Dx() != 1024 {
		t.Fatalf("master = %v", full.Bounds())
	}
	if win == full {
		t.Fatal("window icon must not alias the 1024px master")
	}
}

func TestRGBALightCorner(t *testing.T) {
	img, err := RGBA()
	if err != nil {
		t.Fatal(err)
	}
	c := img.RGBAAt(0, 0)
	if c.R < 200 || c.G < 200 || c.B < 200 || c.A < 200 {
		t.Fatalf("expected light corner, got %+v", c)
	}
	mid := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if mid.R > 240 && mid.G > 240 && mid.B > 240 {
		t.Fatalf("expected toolbox in the center, got %+v", mid)
	}
}
