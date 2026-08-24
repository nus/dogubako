package h264

import (
	"context"
	"testing"
	"time"
)

func TestNewDecoderLoadsVideoToolbox(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dec, err := NewDecoder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	img, err := dec.Decode(nil)
	if err != nil || img != nil {
		t.Fatalf("empty: img=%v err=%v", img, err)
	}
	img, err = dec.Decode([]byte{0, 0, 0, 1, 0x67, 0x42, 0x00, 0x0a})
	if err != nil {
		t.Fatalf("sps: %v", err)
	}
	if img != nil {
		t.Fatal("sps should not emit a picture")
	}
	img, err = dec.Decode([]byte{0, 0, 0, 1, 0x65, 0x88, 0x80})
	if err != nil {
		t.Fatalf("idr without session: %v", err)
	}
	if img != nil {
		t.Fatal("expected no picture before PPS")
	}
}

func TestVideoToolboxAcceptsBaselineParameterSets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dec, err := NewDecoder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	// Baseline 16x16 SPS/PPS from the common Hello264 test stream.
	sps := []byte{0, 0, 0, 1, 0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
	pps := []byte{0, 0, 0, 1, 0x68, 0xce, 0x38, 0x80}
	if _, err := dec.Decode(sps); err != nil {
		t.Fatalf("sps: %v", err)
	}
	if _, err := dec.Decode(pps); err != nil {
		t.Fatalf("pps: %v", err)
	}
	vt := dec.(*vtDecoder)
	if vt.session == 0 || vt.format == 0 {
		t.Fatal("expected VideoToolbox session from SPS/PPS")
	}
	// Exercise DecodeFrame / block buffers. A stub IDR will not yield a picture.
	img, err := dec.Decode([]byte{0, 0, 0, 1, 0x65, 0x88, 0x84, 0x21, 0xa0})
	if img != nil {
		t.Fatal("stub IDR should not decode to an image")
	}
	if err != nil {
		t.Logf("stub idr: %v", err)
	}
}

func TestNewDecoderRespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewDecoder(ctx); err == nil {
		t.Fatal("expected canceled")
	}
}
