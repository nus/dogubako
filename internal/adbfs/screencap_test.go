package adbfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestUsablePNGFindsMagicAndStripsCRLF(t *testing.T) {
	src := tinyPNG(t)
	if got := usablePNG(src); !bytes.Equal(got, src) {
		t.Fatalf("plain png len=%d want=%d", len(got), len(src))
	}

	wrapped := append([]byte("noise\n"), append(src, []byte("\nOK\n")...)...)
	if got := usablePNG(wrapped); !bytes.Equal(got, src) {
		t.Fatalf("wrapped png mismatch (len=%d)", len(got))
	}

	corrupted := bytes.ReplaceAll(src, []byte("\n"), []byte("\r\n"))
	if bytes.Equal(corrupted, src) {
		t.Fatal("expected CRLF corruption")
	}
	if pngPayload(corrupted) != nil {
		t.Fatal("corrupted payload should not match PNG magic")
	}
	got := usablePNG(corrupted)
	if !bytes.Equal(got, src) {
		t.Fatalf("CRLF repair mismatch (len=%d want=%d)", len(got), len(src))
	}
}

func TestMemScreencap(t *testing.T) {
	png := tinyPNG(t)
	fs := NewMem(Device{Serial: "pixel", State: "device"})
	fs.Shot = map[string][]byte{"pixel": png}

	got, err := fs.Screencap(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("len=%d want=%d", len(got), len(png))
	}
	got[0] = 0
	again, err := fs.Screencap(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	if again[0] == 0 {
		t.Fatal("Screencap should copy")
	}

	if _, err := fs.Screencap(context.Background(), "missing"); err == nil {
		t.Fatal("expected error")
	}

	fs.ShotErr = context.DeadlineExceeded
	if _, err := fs.Screencap(context.Background(), "pixel"); err == nil {
		t.Fatal("expected ShotErr")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	fs.ShotErr = nil
	if _, err := fs.Screencap(canceled, "pixel"); err == nil {
		t.Fatal("expected canceled")
	}
}

func TestMemScreencapImage(t *testing.T) {
	png := tinyPNG(t)
	fs := NewMem(Device{Serial: "pixel", State: "device"})
	fs.Shot = map[string][]byte{"pixel": png}

	img, err := fs.ScreencapImage(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v", img.Bounds())
	}
}

func TestDecodeFramebufferRGBA12(t *testing.T) {
	data := rawFramebuffer(2, 1, pixelFormatRGBA8888, []byte{
		255, 0, 0, 255,
		0, 255, 0, 128,
	})
	img, err := decodeFramebuffer(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.NRGBAAt(0, 0) != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("p0 = %+v", img.NRGBAAt(0, 0))
	}
	if img.NRGBAAt(1, 0) != (color.NRGBA{G: 255, A: 128}) {
		t.Fatalf("p1 = %+v", img.NRGBAAt(1, 0))
	}
}

func TestDecodeFramebufferRGBA16Header(t *testing.T) {
	pix := []byte{10, 20, 30, 255}
	data := make([]byte, 16+len(pix))
	binary.LittleEndian.PutUint32(data[0:], 1)
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint32(data[8:], pixelFormatRGBA8888)
	binary.LittleEndian.PutUint32(data[12:], 1) // colorspace
	copy(data[16:], pix)
	img, err := decodeFramebuffer(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.NRGBAAt(0, 0) != (color.NRGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Fatalf("pixel = %+v", img.NRGBAAt(0, 0))
	}
}

func TestDecodeFramebufferBGRAAndRGB(t *testing.T) {
	bgra, err := decodeFramebuffer(rawFramebuffer(1, 1, pixelFormatBGRA8888, []byte{1, 2, 3, 4}))
	if err != nil {
		t.Fatal(err)
	}
	if bgra.NRGBAAt(0, 0) != (color.NRGBA{R: 3, G: 2, B: 1, A: 4}) {
		t.Fatalf("bgra = %+v", bgra.NRGBAAt(0, 0))
	}

	rgb, err := decodeFramebuffer(rawFramebuffer(1, 1, pixelFormatRGB888, []byte{9, 8, 7}))
	if err != nil {
		t.Fatal(err)
	}
	if rgb.NRGBAAt(0, 0) != (color.NRGBA{R: 9, G: 8, B: 7, A: 255}) {
		t.Fatalf("rgb = %+v", rgb.NRGBAAt(0, 0))
	}

	rgbx, err := decodeFramebuffer(rawFramebuffer(1, 1, pixelFormatRGBX8888, []byte{1, 2, 3, 0}))
	if err != nil {
		t.Fatal(err)
	}
	if rgbx.NRGBAAt(0, 0) != (color.NRGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatalf("rgbx = %+v", rgbx.NRGBAAt(0, 0))
	}

	rgb565 := make([]byte, 2)
	// red: 0b11111 000000 00000
	binary.LittleEndian.PutUint16(rgb565, 0xf800)
	got, err := decodeFramebuffer(rawFramebuffer(1, 1, pixelFormatRGB565, rgb565))
	if err != nil {
		t.Fatal(err)
	}
	if c := got.NRGBAAt(0, 0); c.R < 250 || c.G != 0 || c.B != 0 || c.A != 255 {
		t.Fatalf("rgb565 = %+v", c)
	}
}

func TestDecodeFramebufferRejectsBadInput(t *testing.T) {
	if _, err := decodeFramebuffer([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected short")
	}
	if _, err := decodeFramebuffer(rawFramebuffer(0, 1, pixelFormatRGBA8888, nil)); err == nil {
		t.Fatal("expected bad size")
	}
	if _, err := decodeFramebuffer(rawFramebuffer(1, 1, 99, []byte{0, 0, 0, 0})); err == nil {
		t.Fatal("expected bad format")
	}
}

func TestDecodeScreencapBytesFallsBackToPNG(t *testing.T) {
	src := tinyPNG(t)
	img, err := decodeScreencapBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v", img.Bounds())
	}
}

func rawFramebuffer(w, h, format uint32, pix []byte) []byte {
	data := make([]byte, 12+len(pix))
	binary.LittleEndian.PutUint32(data[0:], w)
	binary.LittleEndian.PutUint32(data[4:], h)
	binary.LittleEndian.PutUint32(data[8:], format)
	copy(data[12:], pix)
	return data
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 1, color.NRGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
