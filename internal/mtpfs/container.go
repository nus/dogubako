package mtpfs

import (
	"encoding/binary"
	"fmt"
)

const (
	containerHeaderSize = 12

	containerCommand  = 1
	containerData     = 2
	containerResponse = 3
	containerEvent    = 4

	opGetDeviceInfo      = 0x1001
	opOpenSession        = 0x1002
	opCloseSession       = 0x1003
	opGetStorageIDs      = 0x1004
	opGetStorageInfo     = 0x1005
	opGetObjectHandles   = 0x1007
	opGetObjectInfo      = 0x1008
	opGetObject          = 0x1009
	opSendObjectInfo     = 0x100c
	opSendObject         = 0x100d
	opGetPartialObject   = 0x101b
	opGetObjectPropValue = 0x9803
	opGetObjectPropList  = 0x9805
	opGetPartialObject64 = 0x95c1

	respOK                 = 0x2001
	respOpNotSupported     = 0x2005
	respParamNotSupported  = 0x2006
	respInvalidHandle      = 0x2008
	respInvalidParent      = 0x200a
	respInvalidParameter   = 0x201d
	respSessionAlreadyOpen = 0x201e
	respIncompleteTransfer = 0x2009
	respInvalidObjectProp  = 0xa801

	fmtUndefined   = 0x3000
	fmtAssociation = 0x3001

	assocGenericFolder = 0x0001

	propObjectFormat   = 0xdc02
	propObjectSize     = 0xdc04
	propObjectFileName = 0xdc07
	propDateModified   = 0xdc09
	propParentObject   = 0xdc0b
	propAll            = 0xffffffff
	depthDirect        = 1

	dtUint16 = 0x0004
	dtUint32 = 0x0006
	dtUint64 = 0x0008
	dtString = 0xffff

	// Android / libmtp: 0xFFFFFFFF is the storage root (children with no parent).
	// 0 means "all objects on the storage" and can time out listing a phone.
	handleRoot      = 0xffffffff
	handleAll       = 0
	formatAll       = 0
	lengthUnknown   = 0xffffffff
	objectSizeMax32 = 0xffffffff
)

type containerHeader struct {
	length        uint32
	typ           uint16
	code          uint16
	transactionID uint32
}

func encodeCommand(op uint16, tx uint32, params []uint32) []byte {
	if len(params) > 5 {
		params = params[:5]
	}
	b := make([]byte, containerHeaderSize+4*len(params))
	binary.LittleEndian.PutUint32(b[0:4], uint32(len(b)))
	binary.LittleEndian.PutUint16(b[4:6], containerCommand)
	binary.LittleEndian.PutUint16(b[6:8], op)
	binary.LittleEndian.PutUint32(b[8:12], tx)
	for i, p := range params {
		binary.LittleEndian.PutUint32(b[12+4*i:], p)
	}
	return b
}

func encodeData(op uint16, tx uint32, payload []byte) []byte {
	b := make([]byte, containerHeaderSize+len(payload))
	binary.LittleEndian.PutUint32(b[0:4], uint32(len(b)))
	binary.LittleEndian.PutUint16(b[4:6], containerData)
	binary.LittleEndian.PutUint16(b[6:8], op)
	binary.LittleEndian.PutUint32(b[8:12], tx)
	copy(b[12:], payload)
	return b
}

func decodeHeader(b []byte) (containerHeader, error) {
	if len(b) < containerHeaderSize {
		return containerHeader{}, fmt.Errorf("truncated MTP container header")
	}
	return containerHeader{
		length:        binary.LittleEndian.Uint32(b[0:4]),
		typ:           binary.LittleEndian.Uint16(b[4:6]),
		code:          binary.LittleEndian.Uint16(b[6:8]),
		transactionID: binary.LittleEndian.Uint32(b[8:12]),
	}, nil
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) remaining() int { return len(r.b) - r.i }

func (r *byteReader) u8() (byte, error) {
	if r.i >= len(r.b) {
		return 0, fmt.Errorf("truncated MTP dataset")
	}
	v := r.b[r.i]
	r.i++
	return v, nil
}

func (r *byteReader) u16() (uint16, error) {
	if r.remaining() < 2 {
		return 0, fmt.Errorf("truncated MTP dataset")
	}
	v := binary.LittleEndian.Uint16(r.b[r.i:])
	r.i += 2
	return v, nil
}

func (r *byteReader) u32() (uint32, error) {
	if r.remaining() < 4 {
		return 0, fmt.Errorf("truncated MTP dataset")
	}
	v := binary.LittleEndian.Uint32(r.b[r.i:])
	r.i += 4
	return v, nil
}

func (r *byteReader) u64() (uint64, error) {
	if r.remaining() < 8 {
		return 0, fmt.Errorf("truncated MTP dataset")
	}
	v := binary.LittleEndian.Uint64(r.b[r.i:])
	r.i += 8
	return v, nil
}

func (r *byteReader) skip(n int) error {
	if r.remaining() < n {
		return fmt.Errorf("truncated MTP dataset")
	}
	r.i += n
	return nil
}

func (r *byteReader) mtpString() (string, error) {
	n, err := r.u8()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	need := int(n) * 2
	if r.remaining() < need {
		return "", fmt.Errorf("truncated MTP string")
	}
	u := make([]uint16, n)
	for i := 0; i < int(n); i++ {
		u[i] = binary.LittleEndian.Uint16(r.b[r.i+2*i:])
	}
	r.i += need
	if len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16Decode(u)), nil
}

func (r *byteReader) u16Array() ([]uint16, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]uint16, n)
	for i := range out {
		out[i], err = r.u16()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *byteReader) u32Array() ([]uint32, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]uint32, n)
	for i := range out {
		out[i], err = r.u32()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func utf16Decode(u []uint16) []rune {
	runes := make([]rune, 0, len(u))
	for i := 0; i < len(u); i++ {
		r := rune(u[i])
		if r >= 0xd800 && r <= 0xdbff && i+1 < len(u) {
			low := rune(u[i+1])
			if low >= 0xdc00 && low <= 0xdfff {
				r = 0x10000 + ((r - 0xd800) << 10) + (low - 0xdc00)
				i++
			}
		}
		runes = append(runes, r)
	}
	return runes
}

type byteWriter struct {
	b []byte
}

func (w *byteWriter) u8(v byte) { w.b = append(w.b, v) }

func (w *byteWriter) u16(v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	w.b = append(w.b, buf[:]...)
}

func (w *byteWriter) u32(v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	w.b = append(w.b, buf[:]...)
}

func (w *byteWriter) u64(v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	w.b = append(w.b, buf[:]...)
}

func (w *byteWriter) mtpString(s string) {
	if s == "" {
		w.u8(0)
		return
	}
	u := utf16Encode(s)
	u = append(u, 0)
	if len(u) > 255 {
		u = u[:255]
		u[254] = 0
	}
	w.u8(byte(len(u)))
	for _, c := range u {
		w.u16(c)
	}
}

func utf16Encode(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if r > 0xffff {
			r -= 0x10000
			out = append(out, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

func encodeObjectInfo(storage, parent uint32, format uint16, size uint32, name string, assoc uint16) []byte {
	var w byteWriter
	w.u32(storage)
	w.u16(format)
	w.u16(0) // protection
	w.u32(size)
	w.u16(0) // thumb format
	w.u32(0)
	w.u32(0)
	w.u32(0)
	w.u32(0)
	w.u32(0)
	w.u32(0)
	w.u32(parent)
	w.u16(assoc)
	w.u32(0)
	w.u32(0)
	w.mtpString(name)
	w.mtpString("")
	w.mtpString("")
	w.mtpString("")
	return w.b
}

type objectInfo struct {
	storageID uint32
	format    uint16
	size      uint32
	parent    uint32
	filename  string
	modDate   string
}

func parseObjectInfo(b []byte) (objectInfo, error) {
	r := byteReader{b: b}
	var info objectInfo
	var err error
	if info.storageID, err = r.u32(); err != nil {
		return info, err
	}
	if info.format, err = r.u16(); err != nil {
		return info, err
	}
	if err = r.skip(2); err != nil { // protection
		return info, err
	}
	if info.size, err = r.u32(); err != nil {
		return info, err
	}
	if err = r.skip(2 + 4 + 4 + 4 + 4 + 4 + 4); err != nil { // thumb + image fields
		return info, err
	}
	if info.parent, err = r.u32(); err != nil {
		return info, err
	}
	if err = r.skip(2 + 4 + 4); err != nil { // association + seq
		return info, err
	}
	if info.filename, err = r.mtpString(); err != nil {
		return info, err
	}
	if _, err = r.mtpString(); err != nil { // capture date
		return info, err
	}
	if info.modDate, err = r.mtpString(); err != nil {
		return info, err
	}
	return info, nil
}

type storageInfo struct {
	description string
	volume      string
}

func parseStorageInfo(b []byte) (storageInfo, error) {
	r := byteReader{b: b}
	if err := r.skip(2 + 2 + 2 + 8 + 8 + 4); err != nil {
		return storageInfo{}, err
	}
	desc, err := r.mtpString()
	if err != nil {
		return storageInfo{}, err
	}
	vol, err := r.mtpString()
	if err != nil {
		return storageInfo{}, err
	}
	return storageInfo{description: desc, volume: vol}, nil
}

func parseDeviceInfo(b []byte) (manufacturer, model, serial string, err error) {
	r := byteReader{b: b}
	if err = r.skip(2 + 4 + 2); err != nil {
		return "", "", "", err
	}
	if _, err = r.mtpString(); err != nil { // vendor extension desc
		return "", "", "", err
	}
	if err = r.skip(2); err != nil { // functional mode
		return "", "", "", err
	}
	if _, err = r.u16Array(); err != nil { // operations
		return "", "", "", err
	}
	if _, err = r.u16Array(); err != nil {
		return "", "", "", err
	}
	if _, err = r.u16Array(); err != nil {
		return "", "", "", err
	}
	if _, err = r.u16Array(); err != nil {
		return "", "", "", err
	}
	if _, err = r.u16Array(); err != nil {
		return "", "", "", err
	}
	if manufacturer, err = r.mtpString(); err != nil {
		return "", "", "", err
	}
	if model, err = r.mtpString(); err != nil {
		return "", "", "", err
	}
	if _, err = r.mtpString(); err != nil { // version
		return "", "", "", err
	}
	serial, err = r.mtpString()
	return manufacturer, model, serial, err
}

func parseMTPDate(s string) (year, month, day, hour, min, sec int, ok bool) {
	// YYYYMMDDThhmmss[.s][Z or ±hhmm]
	if len(s) < 15 || s[8] != 'T' {
		return 0, 0, 0, 0, 0, 0, false
	}
	atoi := func(a string) int {
		n := 0
		for i := 0; i < len(a); i++ {
			if a[i] < '0' || a[i] > '9' {
				return -1
			}
			n = n*10 + int(a[i]-'0')
		}
		return n
	}
	year = atoi(s[0:4])
	month = atoi(s[4:6])
	day = atoi(s[6:8])
	hour = atoi(s[9:11])
	min = atoi(s[11:13])
	sec = atoi(s[13:15])
	if year < 0 || month < 1 || month > 12 || day < 1 || hour < 0 || min < 0 || sec < 0 {
		return 0, 0, 0, 0, 0, 0, false
	}
	return year, month, day, hour, min, sec, true
}
