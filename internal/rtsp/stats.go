package rtsp

import (
	"fmt"
	"time"
)

// Direction is whether an RTSP log line was sent, received, or informational.
type Direction int

const (
	DirSend Direction = iota
	DirRecv
	DirInfo
)

func (d Direction) Arrow() string {
	switch d {
	case DirSend:
		return "→"
	case DirRecv:
		return "←"
	default:
		return "·"
	}
}

// LogEntry is one RTSP request, response, or note.
type LogEntry struct {
	Time time.Time
	Dir  Direction
	Text string
}

func (e LogEntry) Summary() string {
	first, _, _ := splitFirstLine(e.Text)
	if first == "" {
		first = e.Text
	}
	if len(first) > 96 {
		first = first[:93] + "..."
	}
	return fmt.Sprintf("%s %s %s", e.Time.Format("15:04:05.000"), e.Dir.Arrow(), first)
}

func splitFirstLine(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			return line, s[i+1:], true
		}
	}
	return s, "", false
}

// NetworkStats is RTP/TCP traffic since Play started.
type NetworkStats struct {
	Bytes      uint64
	Packets    uint64
	Lost       uint64
	BitrateBps uint64
	JitterMs   float64
}

// DecodeStats is video decode counters for the current session.
type DecodeStats struct {
	Codec        Codec
	Width        int
	Height       int
	Frames       uint64
	Errors       uint64
	FPS          float64
	LastDecodeMs float64
}

const maxLogEntries = 200

func FormatBytes(n uint64) string {
	const kb = 1024
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < kb*kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(kb*kb))
	}
}

func FormatBitrate(bps uint64) string {
	switch {
	case bps < 1000:
		return fmt.Sprintf("%d bps", bps)
	case bps < 1_000_000:
		return fmt.Sprintf("%.1f kbps", float64(bps)/1000)
	default:
		return fmt.Sprintf("%.2f Mbps", float64(bps)/1e6)
	}
}
