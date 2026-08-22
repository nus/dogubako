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
	if img.Bounds().Dx() > windowIconEdge || img.Bounds().Dy() > windowIconEdge {
		t.Fatalf("window icon too large: %v", img.Bounds())
	}
}
