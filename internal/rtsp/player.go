package rtsp

import (
	"image"
	"strings"
	"time"
)

func (p *Player) log(dir Direction, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.appendLogLocked(dir, text)
	p.gen++
}

func (p *Player) logExchange(req string, resp *rtspResponse, err error) {
	if req != "" {
		p.log(DirSend, strings.TrimRight(req, "\r\n"))
	}
	if resp != nil {
		p.log(DirRecv, strings.TrimRight(resp.Raw, "\r\n"))
	}
	if err != nil {
		p.log(DirInfo, err.Error())
	}
}

func (p *Player) appendLogLocked(dir Direction, text string) {
	p.logs = append(p.logs, LogEntry{Time: time.Now(), Dir: dir, Text: text})
	if len(p.logs) > maxLogEntries {
		p.logs = append([]LogEntry(nil), p.logs[len(p.logs)-maxLogEntries:]...)
	}
}

func (p *Player) setDownload(pct int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.download = pct
	p.gen++
}

func (p *Player) pushFrame(img image.Image, codec Codec, decodeMs float64) {
	if img == nil {
		return
	}
	sz := img.Bounds().Size()
	now := time.Now()
	p.mu.Lock()
	p.img = img
	p.dec.Codec = codec
	p.dec.Width = sz.X
	p.dec.Height = sz.Y
	p.dec.Frames++
	p.dec.LastDecodeMs = decodeMs
	p.frameAt = append(p.frameAt, now)
	p.trimFPSLocked(now)
	p.download = -1
	p.gen++
	p.mu.Unlock()
}

func (p *Player) decodeErr() {
	p.mu.Lock()
	p.dec.Errors++
	p.gen++
	p.mu.Unlock()
}

func (p *Player) noteDecodeTime(ms float64) {
	p.mu.Lock()
	p.dec.LastDecodeMs = ms
	p.mu.Unlock()
}

func (p *Player) noteNet(seq seqTracker, jit jitter) {
	p.mu.Lock()
	p.net.Bytes = p.byteCount.Load()
	p.net.Packets = seq.packets
	p.net.Lost = seq.lost
	p.net.JitterMs = jit.Milliseconds()
	p.mu.Unlock()
}

func (p *Player) setBitrate(bps uint64) {
	p.mu.Lock()
	p.net.BitrateBps = bps
	p.net.Bytes = p.byteCount.Load()
	p.gen++
	p.mu.Unlock()
}

func (p *Player) refreshFPS() {
	p.mu.Lock()
	p.trimFPSLocked(time.Now())
	p.net.Bytes = p.byteCount.Load()
	p.gen++
	p.mu.Unlock()
}

func (p *Player) trimFPSLocked(now time.Time) {
	cut := now.Add(-time.Second)
	i := 0
	for i < len(p.frameAt) && p.frameAt[i].Before(cut) {
		i++
	}
	if i > 0 {
		p.frameAt = append([]time.Time(nil), p.frameAt[i:]...)
	}
	p.dec.FPS = float64(len(p.frameAt))
}
