package rtsp

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestParseSDP_H264(t *testing.T) {
	sps := []byte{0x67, 0x42, 0xc0, 0x1e}
	pps := []byte{0x68, 0xce, 0x38, 0x80}
	sdp := "v=0\r\n" +
		"m=video 0 RTP/AVP 96\r\n" +
		"a=rtpmap:96 H264/90000\r\n" +
		"a=fmtp:96 packetization-mode=1;sprop-parameter-sets=" +
		base64.StdEncoding.EncodeToString(sps) + "," + base64.StdEncoding.EncodeToString(pps) + "\r\n" +
		"a=control:trackID=0\r\n"
	tr, err := parseSDP(sdp, "rtsp://cam/stream/")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Codec != CodecH264 || tr.PT != 96 {
		t.Fatalf("track = %+v", tr)
	}
	if !bytes.Equal(tr.SPS, sps) || !bytes.Equal(tr.PPS, pps) {
		t.Fatalf("sprop sps=%x pps=%x", tr.SPS, tr.PPS)
	}
	if tr.Control != "rtsp://cam/stream/trackID=0" {
		t.Fatalf("control = %q", tr.Control)
	}
}

func TestParseSDP_JPEG(t *testing.T) {
	sdp := "m=video 0 RTP/AVP 26\r\na=control:/\r\n"
	tr, err := parseSDP(sdp, "rtsp://cam:554/live")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Codec != CodecJPEG || tr.PT != 26 {
		t.Fatalf("track = %+v", tr)
	}
}

func TestParseSDP_NoVideo(t *testing.T) {
	if _, err := parseSDP("m=audio 0 RTP/AVP 0\r\n", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveControl(t *testing.T) {
	if got := resolveControl("rtsp://h/a/", "track1"); got != "rtsp://h/a/track1" {
		t.Fatalf("got %q", got)
	}
	if got := resolveControl("rtsp://h/a", "/abs"); got != "rtsp://h/abs" {
		t.Fatalf("abs = %q", got)
	}
	if got := resolveControl("rtsp://h/a", "rtsp://h/a/t"); got != "rtsp://h/a/t" {
		t.Fatalf("abs url = %q", got)
	}
}

func TestH264SingleAndFUA(t *testing.T) {
	var d h264Depacketizer
	nals, err := d.Push([]byte{0x67, 0x42})
	if err != nil || len(nals) != 1 || !bytes.Equal(nals[0], []byte{0, 0, 0, 1, 0x67, 0x42}) {
		t.Fatalf("single = %v %v", nals, err)
	}
	nal := bytes.Repeat([]byte{0x65, 0x88, 0x80}, 10)
	ind := byte(0x7c)          // F | NRI=3 | type 28
	start := []byte{ind, 0x85} // S=1 type=5
	start = append(start, nal[1:8]...)
	mid := []byte{ind, 0x05}
	mid = append(mid, nal[8:20]...)
	end := []byte{ind, 0x45} // E=1
	end = append(end, nal[20:]...)
	var d2 h264Depacketizer
	if nals, err := d2.Push(start); err != nil || nals != nil {
		t.Fatalf("start: %v %v", nals, err)
	}
	if nals, err := d2.Push(mid); err != nil || nals != nil {
		t.Fatalf("mid: %v %v", nals, err)
	}
	nals, err = d2.Push(end)
	if err != nil || len(nals) != 1 {
		t.Fatalf("end: %v %v", nals, err)
	}
	if !bytes.HasPrefix(nals[0], []byte{0, 0, 0, 1, 0x65}) {
		t.Fatalf("reassembled = %x", nals[0][:8])
	}
	if len(nals[0]) != 4+len(nal) {
		t.Fatalf("len = %d want %d", len(nals[0]), 4+len(nal))
	}
}

func TestH264STAPA(t *testing.T) {
	var d h264Depacketizer
	payload := []byte{24, 0, 2, 0x67, 0x42, 0, 2, 0x68, 0xce}
	nals, err := d.Push(payload)
	if err != nil || len(nals) != 2 {
		t.Fatalf("stap-a %v %v", nals, err)
	}
}

func TestJPEGRawFallback(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	jpg := buf.Bytes()
	hdr := []byte{0, 0, 0, 0, 1, 80, 2, 2}
	payload := append(hdr, jpg...)
	var d jpegDepacketizer
	frame, err := d.Push(payload, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(frame))
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds().Dx() != 16 || out.Bounds().Dy() != 16 {
		t.Fatalf("size = %v", out.Bounds())
	}
}

func TestSeqTrackerLoss(t *testing.T) {
	var s seqTracker
	s.Push(1)
	s.Push(2)
	s.Push(5)
	if s.packets != 3 || s.lost != 2 {
		t.Fatalf("packets=%d lost=%d", s.packets, s.lost)
	}
}

func TestParseRTP(t *testing.T) {
	b := []byte{
		0x80, 26, 0x00, 0x07,
		0, 0, 0, 1,
		0, 0, 0, 2,
		9, 9, 9,
	}
	pkt, err := parseRTP(b)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PT != 26 || pkt.Seq != 7 || !bytes.Equal(pkt.Payload, []byte{9, 9, 9}) {
		t.Fatalf("%+v", pkt)
	}
}

func TestParseURL(t *testing.T) {
	u, err := parseRTSPURL("192.168.0.5/live")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "rtsp" || u.Hostname() != "192.168.0.5" || u.Port() != "554" {
		t.Fatalf("%s", u)
	}
	if _, err := parseRTSPURL("http://x"); err == nil {
		t.Fatal("expected scheme error")
	}
}

func TestDigestAuth(t *testing.T) {
	st := parseWWWAuth(`Digest realm="cam", nonce="abc123", qop="auth"`, "admin", "pass")
	if st == nil || st.mode != authDigest {
		t.Fatal("parse")
	}
	h := st.header("DESCRIBE", "rtsp://cam/stream")
	if !bytes.Contains([]byte(h), []byte("Digest username=")) {
		t.Fatalf("header = %s", h)
	}
}

func TestLogSummary(t *testing.T) {
	e := LogEntry{Dir: DirSend, Text: "OPTIONS rtsp://x RTSP/1.0\r\nCSeq: 1\r\n"}
	e.Time = e.Time.UTC()
	s := e.Summary()
	if !bytes.Contains([]byte(s), []byte("→")) || !bytes.Contains([]byte(s), []byte("OPTIONS")) {
		t.Fatalf("summary = %q", s)
	}
}

func TestFormatUnits(t *testing.T) {
	if got := FormatBytes(500); got != "500 B" {
		t.Fatalf("bytes = %q", got)
	}
	if got := FormatBitrate(2_000_000); got != "2.00 Mbps" {
		t.Fatalf("rate = %q", got)
	}
}
