package openh264

import (
	"bytes"
	"io"
	"testing"
)

func TestSplitAnnexB(t *testing.T) {
	nal1 := []byte{0, 0, 0, 1, 0x67, 0x42, 0xc0}
	nal2 := []byte{0, 0, 1, 0x68, 0xce}
	nal3 := []byte{0, 0, 0, 1, 0x65, 0x88, 0x80}
	src := concat(nal1, nal2, nal3)

	var got [][]byte
	if err := SplitAnnexB(bytes.NewReader(src), func(nal []byte) error {
		got = append(got, nal)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("nals = %d", len(got))
	}
	if !bytes.Equal(got[0], nal1) || !bytes.Equal(got[2], nal3) {
		t.Fatalf("nal mismatch: %#v", got)
	}
	if !bytes.Equal(got[1], []byte{0, 0, 1, 0x68, 0xce}) {
		t.Fatalf("3-byte start = %#v", got[1])
	}
}

func TestSplitAnnexBChunked(t *testing.T) {
	src := []byte{0, 0, 0, 1, 0x67, 1, 2, 0, 0, 0, 1, 0x65, 3}
	r := &chunkReader{data: src, size: 3}
	var n int
	if err := SplitAnnexB(r, func([]byte) error {
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("n = %d", n)
	}
}

func TestIndexStartCode(t *testing.T) {
	if got := indexStartCode([]byte{1, 0, 0, 0, 1, 2}, 0); got != 1 {
		t.Fatalf("4-byte = %d", got)
	}
	if got := indexStartCode([]byte{9, 0, 0, 1, 2}, 0); got != 1 {
		t.Fatalf("3-byte = %d", got)
	}
	if got := indexStartCode([]byte{0, 0}, 0); got != -1 {
		t.Fatalf("short = %d", got)
	}
}

type chunkReader struct {
	data []byte
	size int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := r.size
	if n > len(r.data) {
		n = len(r.data)
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func concat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}
