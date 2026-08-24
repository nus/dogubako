package h264

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

func TestNALPayloadAndAVCC(t *testing.T) {
	nal := []byte{0, 0, 0, 1, 0x67, 0x42}
	if got := nalType(nal); got != nalSPS {
		t.Fatalf("type = %d", got)
	}
	if !bytes.Equal(nalPayload(nal), []byte{0x67, 0x42}) {
		t.Fatalf("payload = %x", nalPayload(nal))
	}
	avcc := toAVCC(nal)
	if len(avcc) != 6 || avcc[3] != 2 || avcc[4] != 0x67 {
		t.Fatalf("avcc = %x", avcc)
	}
	short := []byte{0, 0, 1, 0x65, 1}
	if got := nalType(short); got != nalIDR {
		t.Fatalf("idr type = %d", got)
	}
	if !isVCL(nalIDR) || isVCL(nalSPS) {
		t.Fatal("vcl classification")
	}
}

func TestPercent(t *testing.T) {
	if got := Percent(0, 100); got != 0 {
		t.Fatalf("0/100 = %d", got)
	}
	if got := Percent(50, 100); got != 50 {
		t.Fatalf("50/100 = %d", got)
	}
	if got := Percent(100, 100); got != 100 {
		t.Fatalf("100/100 = %d", got)
	}
	if got := Percent(9, 0); got != 0 {
		t.Fatalf("unknown total = %d", got)
	}
	if got := Percent(200, 100); got != 100 {
		t.Fatalf("over = %d", got)
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
