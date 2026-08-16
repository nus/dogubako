// Command genicon writes the app icon as ICNS (macOS) or PNG (Linux).
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/nus/dogubako/internal/appicon"
)

func main() {
	out := flag.String("o", "dogubako.icns", "output .icns or .png path")
	size := flag.Int("size", 256, "png pixel size (ignored for icns)")
	flag.Parse()
	if err := writeIcon(*out, *size); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeIcon(path string, pngSize int) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return writePNG(path, pngSize)
	default:
		return writeICNS(path)
	}
}

func writePNG(path string, size int) error {
	if size < 16 || size > 1024 {
		return fmt.Errorf("png size must be 16..1024, got %d", size)
	}
	img, err := renderIcon(size)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func writeICNS(path string) error {
	master, err := renderIcon(1024)
	if err != nil {
		return err
	}
	entries := []struct {
		osType string
		size   int
	}{
		{"icp4", 16},
		{"icp5", 32},
		{"icp6", 64},
		{"ic07", 128},
		{"ic08", 256},
		{"ic09", 512},
		{"ic10", 1024},
		{"ic11", 32},
		{"ic12", 64},
		{"ic13", 256},
		{"ic14", 512},
	}

	var body []byte
	for _, e := range entries {
		pngBytes, err := encodePNG(scaleImage(master, e.size))
		if err != nil {
			return err
		}
		header := make([]byte, 8)
		copy(header[:4], e.osType)
		binary.BigEndian.PutUint32(header[4:], uint32(8+len(pngBytes)))
		body = append(body, header...)
		body = append(body, pngBytes...)
	}

	file := make([]byte, 8+len(body))
	copy(file[:4], "icns")
	binary.BigEndian.PutUint32(file[4:], uint32(len(file)))
	copy(file[8:], body)
	return os.WriteFile(path, file, 0o644)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf []byte
	w := &sliceWriter{buf: &buf}
	if err := png.Encode(w, img); err != nil {
		return nil, err
	}
	return buf, nil
}

type sliceWriter struct{ buf *[]byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

func renderIcon(size int) (*image.RGBA, error) {
	src, err := appicon.RGBA()
	if err != nil {
		return nil, err
	}
	return scaleImage(src, size), nil
}

func scaleImage(src *image.RGBA, size int) *image.RGBA {
	if src.Bounds().Dx() == size && src.Bounds().Dy() == size {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}
