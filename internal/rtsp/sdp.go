package rtsp

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// Codec is a video codec advertised in SDP.
type Codec string

const (
	CodecUnknown Codec = ""
	CodecH264    Codec = "H264"
	CodecJPEG    Codec = "JPEG"
)

func (c Codec) String() string {
	switch c {
	case CodecH264:
		return "H.264"
	case CodecJPEG:
		return "MotionJPEG"
	default:
		return "—"
	}
}

type mediaTrack struct {
	Control  string
	PT       byte
	Codec    Codec
	Clock    uint32
	Fmtp     string
	SPS, PPS []byte // H.264 parameter sets from sprop-parameter-sets
}

func parseSDP(sdp, contentBase string) (mediaTrack, error) {
	var track mediaTrack
	inVideo := false
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "m=") {
			inVideo = strings.HasPrefix(line, "m=video")
			if !inVideo || track.Codec != CodecUnknown {
				if inVideo && track.Codec != CodecUnknown {
					inVideo = false
				}
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				if pt, err := strconv.Atoi(fields[3]); err == nil && pt >= 0 && pt < 128 {
					track.PT = byte(pt)
					if pt == 26 {
						track.Codec = CodecJPEG
						track.Clock = 90000
					}
				}
			}
			continue
		}
		if !inVideo || !strings.HasPrefix(line, "a=") {
			continue
		}
		attr, val, _ := strings.Cut(strings.TrimPrefix(line, "a="), ":")
		attr = strings.ToLower(strings.TrimSpace(attr))
		val = strings.TrimSpace(val)
		switch attr {
		case "control":
			track.Control = val
		case "rtpmap":
			pt, rest, ok := matchPayloadType(val, track.PT)
			if !ok {
				continue
			}
			if track.PT == 0 {
				track.PT = pt
			}
			name, clock := parseRtpmap(rest)
			switch strings.ToUpper(name) {
			case "H264":
				track.Codec = CodecH264
			case "JPEG", "MJPEG":
				track.Codec = CodecJPEG
			}
			track.Clock = clock
		case "fmtp":
			_, rest, ok := matchPayloadType(val, track.PT)
			if !ok {
				track.Fmtp = val
			} else {
				track.Fmtp = rest
			}
			sps, pps := parseSprop(track.Fmtp)
			track.SPS, track.PPS = sps, pps
		}
	}
	if track.Codec == CodecUnknown {
		return mediaTrack{}, fmt.Errorf("sdp: no H.264 or JPEG video track")
	}
	if track.Clock == 0 {
		track.Clock = 90000
	}
	track.Control = resolveControl(contentBase, track.Control)
	return track, nil
}

func parseRtpmap(rest string) (name string, clock uint32) {
	name, rate, _ := strings.Cut(rest, "/")
	name = strings.TrimSpace(name)
	clock = 90000
	if n, err := strconv.Atoi(strings.TrimSpace(strings.Split(rate, "/")[0])); err == nil && n > 0 {
		clock = uint32(n)
	}
	return name, clock
}

func matchPayloadType(val string, trackPT byte) (pt byte, rest string, ok bool) {
	ptStr, rest, found := strings.Cut(val, " ")
	if !found {
		return 0, val, false
	}
	n, err := strconv.Atoi(ptStr)
	if err != nil || n < 0 || n > 127 {
		return 0, rest, false
	}
	pt = byte(n)
	if trackPT != 0 && pt != trackPT {
		return pt, rest, false
	}
	return pt, rest, true
}

func parseSprop(fmtp string) (sps, pps []byte) {
	for _, part := range strings.Split(fmtp, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(strings.ToLower(k)) != "sprop-parameter-sets" {
			continue
		}
		sets := strings.Split(v, ",")
		if len(sets) >= 1 {
			sps, _ = base64.StdEncoding.DecodeString(strings.TrimSpace(sets[0]))
		}
		if len(sets) >= 2 {
			pps, _ = base64.StdEncoding.DecodeString(strings.TrimSpace(sets[1]))
		}
		return sps, pps
	}
	return nil, nil
}

func resolveControl(base, control string) string {
	control = strings.TrimSpace(control)
	if control == "*" || control == "" {
		return strings.TrimSpace(base)
	}
	if strings.Contains(control, "://") {
		return control
	}
	base = strings.TrimSpace(base)
	if strings.HasSuffix(base, "/") {
		if strings.HasPrefix(control, "/") {
			return base + strings.TrimPrefix(control, "/")
		}
		return base + control
	}
	if strings.HasPrefix(control, "/") {
		// Absolute path on the same origin: keep scheme/host from base.
		if i := strings.Index(base, "://"); i >= 0 {
			rest := base[i+3:]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				return base[:i+3+slash] + control
			}
			return base + control
		}
		return control
	}
	if base == "" {
		return control
	}
	return base + "/" + control
}
