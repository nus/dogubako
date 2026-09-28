package rtsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPlayerJPEGLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	jpg := tinyJPEG(t)
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveJPEGRTSP(ln, jpg)
	}()

	p := NewPlayer()
	url := "rtsp://" + ln.Addr().String() + "/stream"
	p.Start(context.Background(), url)
	defer p.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap := p.Snapshot()
		if snap.Image != nil && snap.Dec.Codec == CodecJPEG && snap.Dec.Frames >= 1 {
			if snap.Dec.Width != 16 || snap.Dec.Height != 16 {
				t.Fatalf("size = %dx%d", snap.Dec.Width, snap.Dec.Height)
			}
			if len(snap.Logs) < 4 {
				t.Fatalf("logs = %d", len(snap.Logs))
			}
			if snap.Net.Packets == 0 {
				t.Fatalf("expected rtp packets")
			}
			return
		}
		if snap.Err != nil {
			t.Fatalf("player err: %v", snap.Err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	snap := p.Snapshot()
	t.Fatalf("no frame: playing=%v err=%v logs=%d", snap.Playing, snap.Err, len(snap.Logs))
}

func TestPlayerInvalidURL(t *testing.T) {
	p := NewPlayer()
	p.Start(context.Background(), "http://example")
	defer p.Stop()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := p.Snapshot()
		if snap.Err != nil {
			return
		}
		if !snap.Playing && snap.Err == nil && snap.Gen > 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected url error")
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{G: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serveJPEGRTSP(ln net.Listener, jpg []byte) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	br := bufio.NewReader(conn)
	session := "1"
	cseq := 0
	playing := false
	for {
		req, headers, err := readTestRequest(br)
		if err != nil {
			if playing && (err == io.EOF || isClosed(err)) {
				return nil
			}
			return err
		}
		cseq = atoiDefault(headers["cseq"], cseq+1)
		method, uri, _ := strings.Cut(req, " ")
		uri, _, _ = strings.Cut(uri, " ")
		switch method {
		case "OPTIONS":
			writeTestResponse(conn, 200, cseq, session, "Public: OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN\r\n", nil)
		case "DESCRIBE":
			sdp := "v=0\r\n" +
				"s=loopback\r\n" +
				"m=video 0 RTP/AVP 26\r\n" +
				"a=rtpmap:26 JPEG/90000\r\n" +
				"a=control:" + uri + "/track\r\n"
			writeTestResponse(conn, 200, cseq, session, "Content-Type: application/sdp\r\nContent-Base: "+uri+"/\r\n", []byte(sdp))
		case "SETUP":
			writeTestResponse(conn, 200, cseq, session, "Transport: RTP/AVP/TCP;unicast;interleaved=0-1\r\n", nil)
		case "PLAY":
			writeTestResponse(conn, 200, cseq, session, "RTP-Info: url="+uri+"\r\n", nil)
			playing = true
			if err := writeJPEGFrame(conn, jpg); err != nil {
				return err
			}
		case "TEARDOWN":
			writeTestResponse(conn, 200, cseq, session, "", nil)
			return nil
		default:
			writeTestResponse(conn, 501, cseq, session, "", nil)
		}
	}
}

func readTestRequest(br *bufio.Reader) (string, map[string]string, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	req := strings.TrimSpace(line)
	hdr := make(map[string]string)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok {
			hdr[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	if cl := hdr["content-length"]; cl != "" {
		n := atoiDefault(cl, 0)
		if n > 0 {
			io.CopyN(io.Discard, br, int64(n))
		}
	}
	return req, hdr, nil
}

func writeTestResponse(w io.Writer, status, cseq int, session, extra string, body []byte) {
	var b strings.Builder
	fmt.Fprintf(&b, "RTSP/1.0 %d OK\r\n", status)
	fmt.Fprintf(&b, "CSeq: %d\r\n", cseq)
	fmt.Fprintf(&b, "Session: %s;timeout=60\r\n", session)
	b.WriteString(extra)
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(body))
	w.Write([]byte(b.String()))
	if len(body) > 0 {
		w.Write(body)
	}
}

func writeJPEGFrame(w io.Writer, jpg []byte) error {
	jpegHdr := []byte{0, 0, 0, 0, 1, 80, 2, 2}
	payload := append(jpegHdr, jpg...)
	rtp := make([]byte, 12+len(payload))
	rtp[0] = 0x80
	rtp[1] = 26 | 0x80 // PT 26 + marker
	binary.BigEndian.PutUint16(rtp[2:], 1)
	binary.BigEndian.PutUint32(rtp[4:], 90000)
	binary.BigEndian.PutUint32(rtp[8:], 1)
	copy(rtp[12:], payload)
	inter := make([]byte, 4+len(rtp))
	inter[0] = '$'
	inter[1] = 0
	binary.BigEndian.PutUint16(inter[2:], uint16(len(rtp)))
	copy(inter[4:], rtp)
	_, err := w.Write(inter)
	return err
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func isClosed(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "reset"))
}
