package main

import (
	"bytes"
	"encoding/binary"
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
