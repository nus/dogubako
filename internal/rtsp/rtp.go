package rtsp

import (
	"encoding/binary"
	"fmt"
	"time"
)

const rtpHeaderSize = 12

type rtpPacket struct {
	Padding    bool
	Marker     bool
	PT         byte
	Seq        uint16
	Timestamp  uint32
	SSRC       uint32
	Payload    []byte
	Arrival    time.Time
	PayloadOff int
}

func parseRTP(b []byte) (rtpPacket, error) {
	if len(b) < rtpHeaderSize {
		return rtpPacket{}, fmt.Errorf("rtp: short packet (%d)", len(b))
	}
	if b[0]>>6 != 2 {
		return rtpPacket{}, fmt.Errorf("rtp: version %d", b[0]>>6)
	}
	cc := int(b[0] & 0x0f)
	ext := b[0]&0x10 != 0
	pad := b[0]&0x80 != 0
	off := rtpHeaderSize + 4*cc
	if len(b) < off {
		return rtpPacket{}, fmt.Errorf("rtp: short CSRC")
	}
	if ext {
		if len(b) < off+4 {
			return rtpPacket{}, fmt.Errorf("rtp: short extension")
		}
		n := int(binary.BigEndian.Uint16(b[off+2:])) * 4
		off += 4 + n
		if len(b) < off {
			return rtpPacket{}, fmt.Errorf("rtp: truncated extension")
		}
	}
	end := len(b)
	if pad {
		if end == 0 {
			return rtpPacket{}, fmt.Errorf("rtp: empty padding")
		}
		p := int(b[end-1])
		if p == 0 || p > end-off {
			return rtpPacket{}, fmt.Errorf("rtp: bad padding")
		}
		end -= p
	}
	if off > end {
		return rtpPacket{}, fmt.Errorf("rtp: header longer than packet")
	}
	return rtpPacket{
		Padding:    pad,
		Marker:     b[1]&0x80 != 0,
		PT:         b[1] & 0x7f,
		Seq:        binary.BigEndian.Uint16(b[2:]),
		Timestamp:  binary.BigEndian.Uint32(b[4:]),
		SSRC:       binary.BigEndian.Uint32(b[8:]),
		Payload:    b[off:end],
		PayloadOff: off,
	}, nil
}

type seqTracker struct {
	init    bool
	last    uint16
	lost    uint64
	packets uint64
}

func (s *seqTracker) Push(seq uint16) {
	s.packets++
	if !s.init {
		s.init = true
		s.last = seq
		return
	}
	expect := s.last + 1
	if seq != expect {
		gap := uint16(seq - expect)
		if gap < 0x8000 {
			s.lost += uint64(gap)
		}
	}
	s.last = seq
}

// RFC 3550 A.8 interarrival jitter, in clock units.
type jitter struct {
	clock  uint32
	init   bool
	prevTS uint32
	prevAt time.Time
	j      float64
}

func (j *jitter) Push(ts uint32, at time.Time) {
	if j.clock == 0 {
		j.clock = 90000
	}
	if !j.init {
		j.init = true
		j.prevTS = ts
		j.prevAt = at
		return
	}
	arrival := at.Sub(j.prevAt).Seconds() * float64(j.clock)
	transit := arrival - float64(int32(ts-j.prevTS))
	if transit < 0 {
		transit = -transit
	}
	j.j += (transit - j.j) / 16
	j.prevTS = ts
	j.prevAt = at
}

func (j *jitter) Milliseconds() float64 {
	if j.clock == 0 {
		return 0
	}
	return j.j * 1000 / float64(j.clock)
}
