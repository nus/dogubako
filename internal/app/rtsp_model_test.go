package app

import (
	"strings"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/rtsp"
)

func TestRTSPConnectEmptyURL(t *testing.T) {
	var m RTSPModel
	m.Connect()
	if m.Playing() {
		t.Fatal("empty url should not play")
	}
	if got := m.StatusText(i18n.EN); got != "Enter an RTSP URL" {
		t.Fatalf("status = %q", got)
	}
}

func TestRTSPSetURL(t *testing.T) {
	var m RTSPModel
	m.SetURL("rtsp://cam/stream")
	if m.URL() != "rtsp://cam/stream" {
		t.Fatalf("url = %q", m.URL())
	}
	gen := m.Generation()
	m.SetURL("rtsp://cam/stream")
	if m.Generation() != gen {
		t.Fatal("same url should not bump generation")
	}
}

func TestRTSPStatsText(t *testing.T) {
	var m RTSPModel
	m.snap.Dec = rtsp.DecodeStats{Codec: rtsp.CodecJPEG, Width: 16, Height: 8, Frames: 3, FPS: 2.5, LastDecodeMs: 1.5, Errors: 1}
	m.snap.Net = rtsp.NetworkStats{Bytes: 2048, Packets: 10, Lost: 1, BitrateBps: 8000, JitterMs: 0.5}
	dec := m.DecodeStatsText(i18n.EN)
	if !strings.Contains(dec, "MotionJPEG") || !strings.Contains(dec, "16×8") || !strings.Contains(dec, "3") {
		t.Fatalf("decode = %q", dec)
	}
	net := m.NetworkStatsText(i18n.JA)
	if !strings.Contains(net, "10") || !strings.Contains(net, "1") {
		t.Fatalf("net = %q", net)
	}
}

func TestRTSPDisconnectIdempotent(t *testing.T) {
	var m RTSPModel
	m.Disconnect()
	m.Disconnect()
}
