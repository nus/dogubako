package h264

const (
	nalSlice = 1
	nalIDR   = 5
	nalSEI   = 6
	nalSPS   = 7
	nalPPS   = 8
	nalAUD   = 9
)

func nalPayload(annexB []byte) []byte {
	switch {
	case len(annexB) >= 4 && annexB[0] == 0 && annexB[1] == 0 && annexB[2] == 0 && annexB[3] == 1:
		return annexB[4:]
	case len(annexB) >= 3 && annexB[0] == 0 && annexB[1] == 0 && annexB[2] == 1:
		return annexB[3:]
	default:
		return annexB
	}
}

func nalType(annexB []byte) byte {
	p := nalPayload(annexB)
	if len(p) == 0 {
		return 0
	}
	return p[0] & 0x1f
}

func isVCL(t byte) bool {
	return t >= nalSlice && t <= nalIDR
}

func toAVCC(annexB []byte) []byte {
	p := nalPayload(annexB)
	if len(p) == 0 {
		return nil
	}
	n := uint32(len(p))
	out := make([]byte, 4+len(p))
	out[0] = byte(n >> 24)
	out[1] = byte(n >> 16)
	out[2] = byte(n >> 8)
	out[3] = byte(n)
	copy(out[4:], p)
	return out
}
