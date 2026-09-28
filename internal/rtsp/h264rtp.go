package rtsp

import "fmt"

const (
	nalSTAPA = 24
	nalFUA   = 28
)

type h264Depacketizer struct {
	fu     []byte
	fuType byte
}

func annexB(nal []byte) []byte {
	if len(nal) == 0 {
		return nil
	}
	out := make([]byte, 4+len(nal))
	out[0], out[1], out[2], out[3] = 0, 0, 0, 1
	copy(out[4:], nal)
	return out
}

// Push returns zero or more Annex-B NAL units from one RTP H.264 payload (RFC 6184).
func (d *h264Depacketizer) Push(payload []byte) ([][]byte, error) {
	if len(payload) < 1 {
		return nil, nil
	}
	nalType := payload[0] & 0x1f
	nri := payload[0] & 0x60
	switch {
	case nalType >= 1 && nalType <= 23:
		d.fu = nil
		return [][]byte{annexB(payload)}, nil
	case nalType == nalSTAPA:
		d.fu = nil
		return splitSTAPA(payload[1:])
	case nalType == nalFUA:
		return d.pushFU(nri, payload[1:])
	default:
		d.fu = nil
		return nil, nil
	}
}

func splitSTAPA(b []byte) ([][]byte, error) {
	var nals [][]byte
	for len(b) >= 2 {
		n := int(b[0])<<8 | int(b[1])
		b = b[2:]
		if n <= 0 || n > len(b) {
			return nals, fmt.Errorf("h264: bad STAP-A size")
		}
		nals = append(nals, annexB(b[:n]))
		b = b[n:]
	}
	return nals, nil
}

func (d *h264Depacketizer) pushFU(nri byte, rest []byte) ([][]byte, error) {
	if len(rest) < 2 {
		d.fu = nil
		return nil, fmt.Errorf("h264: short FU-A")
	}
	hdr := rest[0]
	start := hdr&0x80 != 0
	end := hdr&0x40 != 0
	t := hdr & 0x1f
	body := rest[1:]
	if start {
		d.fuType = t
		d.fu = append(d.fu[:0], nri|t)
		d.fu = append(d.fu, body...)
		if end {
			nal := d.fu
			d.fu = nil
			return [][]byte{annexB(nal)}, nil
		}
		return nil, nil
	}
	if d.fu == nil || t != d.fuType {
		d.fu = nil
		return nil, nil
	}
	d.fu = append(d.fu, body...)
	if end {
		nal := d.fu
		d.fu = nil
		return [][]byte{annexB(nal)}, nil
	}
	return nil, nil
}
