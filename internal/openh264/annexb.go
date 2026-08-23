package openh264

import "io"

// SplitAnnexB reads an H.264 Annex-B byte stream and calls emit once per NAL
// unit, including the start code.
func SplitAnnexB(r io.Reader, emit func([]byte) error) error {
	const keep = 3
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	start := -1
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				if start < 0 {
					start = indexStartCode(buf, 0)
					if start < 0 {
						if len(buf) > keep {
							copy(buf, buf[len(buf)-keep:])
							buf = buf[:keep]
						}
						break
					}
					if start > 0 {
						buf = buf[start:]
						start = 0
					}
				}
				next := indexStartCode(buf, start+3)
				if next < 0 {
					break
				}
				if err := emitNAL(buf[start:next], emit); err != nil {
					return err
				}
				buf = buf[next:]
				start = 0
			}
		}
		if err == io.EOF {
			if start >= 0 && start < len(buf) {
				return emitNAL(buf[start:], emit)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func emitNAL(nal []byte, emit func([]byte) error) error {
	if len(nal) < 4 {
		return nil
	}
	return emit(append([]byte(nil), nal...))
}

func indexStartCode(b []byte, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+2 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if i > 0 && b[i-1] == 0 {
				return i - 1
			}
			return i
		}
	}
	return -1
}
