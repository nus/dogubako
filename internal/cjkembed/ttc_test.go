package cjkembed

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/gg/text"
	"golang.org/x/image/font/gofont/goregular"
)

func TestExtractCollectionFaceLeavesSingleFont(t *testing.T) {
	got, err := extractCollectionFace(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(goregular.TTF) {
		t.Fatalf("len = %d, want %d", len(got), len(goregular.TTF))
	}
	for i := range got {
		if got[i] != goregular.TTF[i] {
			t.Fatalf("byte %d changed", i)
		}
	}
}

func TestExtractCollectionFaceFromWrappedTTF(t *testing.T) {
	ttc := wrapAsTTC(t, goregular.TTF)
	got, err := extractCollectionFace(ttc, 0)
	if err != nil {
		t.Fatal(err)
	}
	src, err := text.NewFontSource(got)
	if err != nil {
		t.Fatal(err)
	}
	if src.Name() == "" {
		t.Fatal("extracted face has no name")
	}
	if _, err := extractCollectionFace(ttc, 1); err == nil {
		t.Fatal("expected out-of-range index to fail")
	}
}

func wrapAsTTC(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	if len(sfnt) < 12 {
		t.Fatal("sfnt too small")
	}
	const header = 16
	out := make([]byte, header+len(sfnt))
	copy(out[0:4], ttcTag)
	binary.BigEndian.PutUint32(out[4:8], 0x00010000)
	binary.BigEndian.PutUint32(out[8:12], 1)
	binary.BigEndian.PutUint32(out[12:16], uint32(header))
	copy(out[header:], sfnt)
	numTables := int(binary.BigEndian.Uint16(sfnt[4:6]))
	for i := 0; i < numTables; i++ {
		rec := out[header+12+16*i:]
		off := binary.BigEndian.Uint32(rec[8:12])
		binary.BigEndian.PutUint32(rec[8:12], off+uint32(header))
	}
	return out
}
