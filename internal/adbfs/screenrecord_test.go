package adbfs

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/nus/dogubako/internal/openh264"
)

func TestMemScreenrecordH264(t *testing.T) {
	nal := []byte{0, 0, 0, 1, 0x67, 0x42}
	fs := NewMem(Device{Serial: "pixel", State: "device"})
	if _, err := fs.ScreenrecordH264(context.Background(), "pixel"); err == nil {
		t.Fatal("expected missing stream")
	}

	fs.H264 = map[string][]byte{"pixel": nal}
	r, err := fs.ScreenrecordH264(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, nal) {
		t.Fatalf("got %#v", got)
	}
	got[0] = 9
	again, err := fs.ScreenrecordH264(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(again)
	_ = again.Close()
	if data[0] == 9 {
		t.Fatal("ScreenrecordH264 should copy")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fs.ScreenrecordH264(canceled, "pixel"); err == nil {
		t.Fatal("expected canceled")
	}
}

func TestSplitAnnexBFromMemStream(t *testing.T) {
	src := []byte{0, 0, 0, 1, 0x67, 1, 0, 0, 0, 1, 0x65, 2}
	fs := NewMem(Device{Serial: "pixel", State: "device"})
	fs.H264 = map[string][]byte{"pixel": src}
	r, err := fs.ScreenrecordH264(context.Background(), "pixel")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var n int
	if err := openh264.SplitAnnexB(r, func([]byte) error {
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("n = %d", n)
	}
}
