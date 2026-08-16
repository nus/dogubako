package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteICNS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dogubako.icns")
	if err := writeICNS(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 16 {
		t.Fatalf("icns too small: %d", len(data))
	}
	if !bytes.Equal(data[:4], []byte("icns")) {
		t.Fatalf("magic: %q", data[:4])
	}
	size := binary.BigEndian.Uint32(data[4:])
	if int(size) != len(data) {
		t.Fatalf("size field %d != file %d", size, len(data))
	}
}

func TestWritePNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dogubako.png")
	if err := writePNG(path, 256); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("size %v", img.Bounds())
	}
}

func TestWriteIconExt(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "icon.PNG")
	if err := writeIcon(pngPath, 64); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatal(err)
	}
}

func TestWritePNGRejectsSize(t *testing.T) {
	if err := writePNG(filepath.Join(t.TempDir(), "x.png"), 8); err == nil {
		t.Fatal("expected error")
	}
}

func TestWritePNGUsesArtwork(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dogubako.png")
	if err := writePNG(path, 64); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	c := img.At(0, 0)
	r, g, b, a := c.RGBA()
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 || a>>8 < 200 {
		t.Fatalf("expected light corner, got %v", c)
	}
}
