// Package rtsp is a pure-Go RTSP client for MotionJPEG and H.264 video.
package rtsp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nus/dogubako/internal/h264"
)

const (
	firstFrameWait = 15 * time.Second
	statsTick      = 250 * time.Millisecond
	udpReadBuf     = 64 * 1024
)

// Snapshot is a consistent view of player state for the UI.
type Snapshot struct {
	Gen      uint64
	Playing  bool
	Image    image.Image
	Logs     []LogEntry
	Net      NetworkStats
	Dec      DecodeStats
	Download int // 0–100 while fetching OpenH264, else -1
	Err      error
}

// Player plays an RTSP URL (MotionJPEG or H.264) and exposes logs plus stats.
type Player struct {
	mu       sync.Mutex
	gen      uint64
	playing  bool
	img      image.Image
	logs     []LogEntry
	net      NetworkStats
	dec      DecodeStats
	download int
	err      error

	cancel context.CancelFunc
	done   chan struct{}

	byteCount atomic.Uint64
	frameAt   []time.Time
}

func NewPlayer() *Player {
	p := &Player{download: -1}
	return p
}

func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *Player) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	logs := append([]LogEntry(nil), p.logs...)
	return Snapshot{
		Gen:      p.gen,
		Playing:  p.playing,
		Image:    p.img,
		Logs:     logs,
		Net:      p.net,
		Dec:      p.dec,
		Download: p.download,
		Err:      p.err,
	}
}

func (p *Player) Start(parent context.Context, rawURL string) {
	p.Stop()
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	p.mu.Lock()
	p.cancel = cancel
	p.done = done
	p.playing = true
	p.err = nil
	p.img = nil
	p.logs = nil
	p.net = NetworkStats{}
	p.dec = DecodeStats{}
	p.download = -1
	p.byteCount.Store(0)
	p.frameAt = nil
	p.gen++
	p.mu.Unlock()
	go func() {
		defer close(done)
		err := p.play(ctx, rawURL)
		p.mu.Lock()
		p.playing = false
		if err != nil && ctx.Err() == nil {
			p.err = err
			p.appendLogLocked(DirInfo, err.Error())
		}
		p.gen++
		p.mu.Unlock()
	}()
}

func (p *Player) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.cancel = nil
	p.done = nil
	p.playing = false
	p.gen++
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (p *Player) play(ctx context.Context, rawURL string) error {
	u, err := parseRTSPURL(rawURL)
	if err != nil {
		return err
	}
	p.log(DirInfo, "connect "+hostPort(u))
	d := net.Dialer{Timeout: 10 * time.Second}
	var dialerCtx context.Context
	var cancelDial context.CancelFunc
	dialerCtx, cancelDial = context.WithTimeout(ctx, 10*time.Second)
	conn, err := d.DialContext(dialerCtx, "tcp", hostPort(u))
	cancelDial()
	if err != nil {
		return err
	}
	defer conn.Close()

	user, pass := credentials(u)
	c := &rtspConn{
		conn:  conn,
		br:    bufio.NewReaderSize(conn, 64*1024),
		bytes: &p.byteCount,
	}
	if user != "" {
		c.auth = &authState{user: user, pass: pass}
	}

	uri := requestURI(u)
	if err := p.handshake(ctx, c, uri, user, pass); err != nil {
		return err
	}

	resp, req, err := c.request("DESCRIBE", uri, map[string]string{"Accept": "application/sdp"})
	p.logExchange(req, resp, err)
	if err != nil {
		return err
	}
	resp, err = p.retryAuth(c, "DESCRIBE", uri, map[string]string{"Accept": "application/sdp"}, resp, user, pass)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("rtsp: DESCRIBE %d %s", resp.Status, resp.Reason)
	}
	base := contentBase(resp, uri)
	track, err := parseSDP(string(resp.Body), base)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.dec.Codec = track.Codec
	p.gen++
	p.mu.Unlock()
	p.log(DirInfo, fmt.Sprintf("track %s PT=%d %s", track.Codec.String(), track.PT, track.Control))

	setupURI := track.Control
	if setupURI == "" {
		setupURI = uri
	}
	interleaved, udpConn, err := p.setupMedia(c, setupURI)
	if err != nil {
		return err
	}
	if udpConn != nil {
		defer udpConn.Close()
	}

	playURI := uri
	resp, req, err = c.request("PLAY", playURI, map[string]string{"Range": "npt=0.000-"})
	p.logExchange(req, resp, err)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("rtsp: PLAY %d %s", resp.Status, resp.Reason)
	}
	keepEvery := sessionTimeout(resp.Header["Session"]) / 2
	if keepEvery <= 0 {
		keepEvery = 30 * time.Second
	}

	var dec h264.Decoder
	if track.Codec == CodecH264 {
		if !h264.Enabled() {
			return fmt.Errorf("h264: decoder disabled")
		}
		lastPct := -1
		dec, err = h264.NewDecoderProgress(ctx, func(done, total int64) {
			pct := h264.Percent(done, total)
			if pct == lastPct {
				return
			}
			lastPct = pct
			p.setDownload(pct)
		})
		if err != nil {
			return err
		}
		defer dec.Close()
		p.setDownload(-1)
		if len(track.SPS) > 0 {
			_, _ = dec.Decode(annexB(track.SPS))
		}
		if len(track.PPS) > 0 {
			_, _ = dec.Decode(annexB(track.PPS))
		}
	}

	p.log(DirInfo, "playing")
	return p.stream(ctx, c, udpConn, interleaved, track, dec, keepEvery)
}

func (p *Player) handshake(ctx context.Context, c *rtspConn, uri, user, pass string) error {
	_ = ctx
	resp, req, err := c.request("OPTIONS", uri, nil)
	p.logExchange(req, resp, err)
	if err != nil {
		return err
	}
	if resp.Status == 401 {
		_, err = p.retryAuth(c, "OPTIONS", uri, nil, resp, user, pass)
		return err
	}
	return nil
}

func (p *Player) retryAuth(c *rtspConn, method, uri string, extra map[string]string, resp *rtspResponse, user, pass string) (*rtspResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("rtsp: empty response")
	}
	if resp.Status != 401 {
		if resp.Status >= 400 {
			return resp, fmt.Errorf("rtsp: %s %d %s", method, resp.Status, resp.Reason)
		}
		return resp, nil
	}
	if user == "" {
		return resp, fmt.Errorf("rtsp: authentication required")
	}
	www := headerAuth(resp)
	st := parseWWWAuth(www, user, pass)
	if st == nil {
		return resp, fmt.Errorf("rtsp: unsupported authentication")
	}
	c.auth = st
	p.log(DirInfo, "auth "+www)
	next, req, err := c.request(method, uri, extra)
	p.logExchange(req, next, err)
	if err != nil {
		return nil, err
	}
	if next.Status == 401 {
		return next, fmt.Errorf("rtsp: authentication failed")
	}
	if next.Status >= 400 {
		return next, fmt.Errorf("rtsp: %s %d %s", method, next.Status, next.Reason)
	}
	return next, nil
}

func (p *Player) setupMedia(c *rtspConn, setupURI string) (interleaved bool, udp net.PacketConn, err error) {
	resp, req, err := c.request("SETUP", setupURI, map[string]string{
		"Transport": "RTP/AVP/TCP;unicast;interleaved=0-1",
	})
	p.logExchange(req, resp, err)
	if err != nil {
		return false, nil, err
	}
	if resp.Status == 200 {
		tr := strings.ToUpper(resp.Header["Transport"])
		if tr == "" || strings.Contains(tr, "TCP") || strings.Contains(tr, "INTERLEAVED") {
			return true, nil, nil
		}
	}
	l, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		if resp.Status != 200 {
			return false, nil, fmt.Errorf("rtsp: SETUP %d %s", resp.Status, resp.Reason)
		}
		return false, nil, err
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
	transport := fmt.Sprintf("RTP/AVP;unicast;client_port=%d-%d", port, port+1)
	resp, req, err = c.request("SETUP", setupURI, map[string]string{"Transport": transport})
	p.logExchange(req, resp, err)
	if err != nil {
		_ = l.Close()
		return false, nil, err
	}
	if resp.Status != 200 {
		_ = l.Close()
		return false, nil, fmt.Errorf("rtsp: SETUP %d %s", resp.Status, resp.Reason)
	}
	return false, l, nil
}

func (p *Player) stream(ctx context.Context, c *rtspConn, udp net.PacketConn, interleaved bool, track mediaTrack, dec h264.Decoder, keepEvery time.Duration) error {
	var (
		h264d h264Depacketizer
		jpegt jpegDepacketizer
		seq   seqTracker
		jit   = jitter{clock: track.Clock}
		got   atomic.Bool
	)
	firstTimer := time.AfterFunc(firstFrameWait, func() {
		if !got.Load() {
			c.close()
			if udp != nil {
				_ = udp.Close()
			}
		}
	})
	defer firstTimer.Stop()

	keep := time.NewTicker(keepEvery)
	defer keep.Stop()
	tick := time.NewTicker(statsTick)
	defer tick.Stop()

	var lastWin time.Time
	var lastBytes uint64
	lastWin = time.Now()
	lastBytes = p.byteCount.Load()

	handleRTP := func(raw []byte, at time.Time) error {
		pkt, err := parseRTP(raw)
		if err != nil {
			return nil
		}
		if pkt.PT != track.PT && track.PT != 0 {
			return nil
		}
		seq.Push(pkt.Seq)
		jit.Push(pkt.Timestamp, at)
		p.noteNet(seq, jit)
		switch track.Codec {
		case CodecH264:
			if dec == nil {
				return fmt.Errorf("h264: no decoder")
			}
			nals, err := h264d.Push(pkt.Payload)
			if err != nil {
				p.decodeErr()
				return nil
			}
			for _, nal := range nals {
				start := time.Now()
				img, err := dec.Decode(nal)
				ms := time.Since(start).Seconds() * 1000
				if err != nil {
					p.decodeErr()
					continue
				}
				if img == nil {
					p.noteDecodeTime(ms)
					continue
				}
				got.Store(true)
				firstTimer.Stop()
				p.pushFrame(img, track.Codec, ms)
			}
		case CodecJPEG:
			frame, err := jpegt.Push(pkt.Payload, pkt.Marker)
			if err != nil {
				p.decodeErr()
				return nil
			}
			if frame == nil {
				return nil
			}
			start := time.Now()
			img, err := jpeg.Decode(bytes.NewReader(frame))
			ms := time.Since(start).Seconds() * 1000
			if err != nil {
				p.decodeErr()
				return nil
			}
			got.Store(true)
			firstTimer.Stop()
			p.pushFrame(img, track.Codec, ms)
		}
		return nil
	}

	errCh := make(chan error, 1)
	if udp != nil {
		go func() {
			buf := make([]byte, udpReadBuf)
			for {
				if err := ctx.Err(); err != nil {
					return
				}
				_ = udp.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, _, err := udp.ReadFrom(buf)
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						continue
					}
					if ctx.Err() != nil {
						return
					}
					select {
					case errCh <- err:
					default:
					}
					return
				}
				at := time.Now()
				pkt := append([]byte(nil), buf[:n]...)
				if err := handleRTP(pkt, at); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			c.setDeadline(3 * time.Second)
			_, _, _ = c.request("TEARDOWN", "*", nil)
			return ctx.Err()
		case err := <-errCh:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !got.Load() {
				return err
			}
			return err
		case <-keep.C:
			if interleaved {
				continue
			}
			c.setDeadline(10 * time.Second)
			resp, req, err := c.request("OPTIONS", "*", nil)
			if err == nil {
				p.logExchange(req, resp, nil)
			}
		case <-tick.C:
			now := time.Now()
			b := p.byteCount.Load()
			dt := now.Sub(lastWin).Seconds()
			if dt > 0 {
				bps := uint64(float64(b-lastBytes) / dt)
				p.setBitrate(bps)
			}
			lastWin = now
			lastBytes = b
			p.refreshFPS()
		default:
			if interleaved {
				c.setDeadline(2 * time.Second)
				ch, payload, resp, err := c.readInterleaved()
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						continue
					}
					if ctx.Err() != nil {
						return ctx.Err()
					}
					if !got.Load() {
						return err
					}
					if err == io.EOF {
						return fmt.Errorf("rtsp: connection closed")
					}
					return err
				}
				if resp != nil {
					p.log(DirRecv, resp.Raw)
					continue
				}
				if ch%2 == 1 {
					continue // RTCP
				}
				if err := handleRTP(payload, time.Now()); err != nil {
					return err
				}
				continue
			}
			// UDP: the receive loop is in a goroutine; just wait a bit.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-errCh:
				return err
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
}
