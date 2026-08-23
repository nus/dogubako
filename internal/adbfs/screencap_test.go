package adbfs

import (
	"bytes"
	"context"
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
