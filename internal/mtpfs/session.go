package mtpfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	cmdTimeout   = 20 * time.Second
	dataTimeout  = 60 * time.Second
	readChunk    = 512 * 1024
	partialChunk = 256 * 1024
)

type transport interface {
	Write(p []byte, timeout time.Duration) error
	Read(max int, timeout time.Duration) ([]byte, error)
	WriteStream(header []byte, r io.Reader, size int64, timeout time.Duration) error
	WriteStreamProgress(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error
	Close() error
}

type session struct {
	t       transport
	tx      uint32
	pending []byte
}

func (s *session) close() {
	if s == nil || s.t == nil {
		return
	}
	_, _ = s.runCommand(context.Background(), opCloseSession, nil)
	_ = s.t.Close()
	s.t = nil
}

func (s *session) nextTx() uint32 {
	id := s.tx
	s.tx++
	return id
}

func (s *session) open(ctx context.Context) error {
	_, err := s.runCommand(ctx, opOpenSession, []uint32{1})
	if err != nil {
		if isResponse(err, respSessionAlreadyOpen) {
			_, _ = s.runCommand(ctx, opCloseSession, nil)
			_, err = s.runCommand(ctx, opOpenSession, []uint32{1})
			if isResponse(err, respSessionAlreadyOpen) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	// Some responders only fully initialize after DeviceInfo is read.
	_, _ = s.receiveData(ctx, opGetDeviceInfo, nil)
	return nil
}

type responseError struct {
	code uint16
}

func (e responseError) Error() string {
	if name := responseName(e.code); name != "" {
		return fmt.Sprintf("MTP %s (0x%04x)", name, e.code)
	}
	return fmt.Sprintf("MTP response 0x%04x", e.code)
}

func responseName(code uint16) string {
	switch code {
	case respOK:
		return "OK"
	case respOpNotSupported:
		return "operation_not_supported"
	case respParamNotSupported:
		return "parameter_not_supported"
	case respInvalidHandle:
		return "invalid_object_handle"
	case respInvalidParent:
		return "invalid_parent"
	case respInvalidParameter:
		return "invalid_parameter"
	case respSessionAlreadyOpen:
		return "session_already_open"
	default:
		return ""
	}
}

func isPropListUnsupported(err error) bool {
	return isResponse(err, respOpNotSupported) ||
		isResponse(err, respParamNotSupported) ||
		isResponse(err, respInvalidParameter) ||
		isResponse(err, respIncompleteTransfer) ||
		isResponse(err, respInvalidObjectProp)
}

func isResponse(err error, code uint16) bool {
	var re responseError
	return err != nil && errorAs(err, &re) && re.code == code
}

func errorAs(err error, target *responseError) bool {
	e, ok := err.(responseError)
	if !ok {
		return false
	}
	*target = e
	return true
}

func (s *session) runCommand(ctx context.Context, op uint16, params []uint32) ([]uint32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tx := s.nextTx()
	if err := s.t.Write(encodeCommand(op, tx, params), cmdTimeout); err != nil {
		return nil, err
	}
	h, payload, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(h, op); err != nil {
		return nil, err
	}
	return paramsFrom(payload), nil
}

func (s *session) receiveData(ctx context.Context, op uint16, params []uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tx := s.nextTx()
	if err := s.t.Write(encodeCommand(op, tx, params), cmdTimeout); err != nil {
		return nil, err
	}
	h, payload, err := s.readContainer(ctx, dataTimeout)
	if err != nil {
		return nil, err
	}
	if h.typ == containerResponse {
		if err := checkResponse(h, op); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if h.typ != containerData {
		return nil, fmt.Errorf("MTP: expected data container, got type %d", h.typ)
	}
	rh, rpayload, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(rh, op); err != nil {
		return nil, err
	}
	_ = rpayload
	return payload, nil
}

func (s *session) sendData(ctx context.Context, op uint16, params []uint32, payload []byte) ([]uint32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tx := s.nextTx()
	if err := s.t.Write(encodeCommand(op, tx, params), cmdTimeout); err != nil {
		return nil, err
	}
	if err := s.t.Write(encodeData(op, tx, payload), dataTimeout); err != nil {
		return nil, err
	}
	h, body, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(h, op); err != nil {
		return nil, err
	}
	return paramsFrom(body), nil
}

func (s *session) sendFile(ctx context.Context, op uint16, r io.Reader, size int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := s.nextTx()
	if err := s.t.Write(encodeCommand(op, tx, nil), cmdTimeout); err != nil {
		return err
	}
	phase := int64(containerHeaderSize) + size
	lenField := uint32(phase)
	if phase > int64(^uint32(0)) {
		lenField = lengthUnknown
	}
	header := make([]byte, containerHeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], lenField)
	binary.LittleEndian.PutUint16(header[4:6], containerData)
	binary.LittleEndian.PutUint16(header[6:8], op)
	binary.LittleEndian.PutUint32(header[8:12], tx)
	wrote := func(n int64) { addCopyBytes(ctx, n) }
	if err := s.t.WriteStreamProgress(header, r, size, dataTimeout, wrote); err != nil {
		return err
	}
	h, _, err := s.readContainer(ctx, dataTimeout)
	if err != nil {
		return err
	}
	return checkResponse(h, op)
}

func (s *session) getObjectTo(ctx context.Context, handle uint32, size int64, w io.Writer) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if size == int64(objectSizeMax32) {
		if sz, err := s.objectSize64(ctx, handle); err == nil && sz >= 0 {
			size = sz
		}
	}
	if size == 0 {
		return 0, nil
	}
	if size > 0 && size != int64(objectSizeMax32) {
		n, err := s.getPartialTo(ctx, handle, size, w)
		if err == nil {
			return n, nil
		}
		if n == 0 && (isResponse(err, respOpNotSupported) || isResponse(err, respParamNotSupported)) {
			return s.getObjectFullTo(ctx, handle, w)
		}
		return n, err
	}
	return s.getObjectFullTo(ctx, handle, w)
}

func (s *session) getPartialTo(ctx context.Context, handle uint32, size int64, w io.Writer) (int64, error) {
	var (
		offset  uint32
		written int64
	)
	for written < size {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		left := size - written
		want := uint32(partialChunk)
		if left < int64(want) {
			want = uint32(left)
		}
		if want == 0 {
			break
		}
		b, err := s.receiveData(ctx, opGetPartialObject, []uint32{handle, offset, want})
		if err != nil {
			return written, err
		}
		if len(b) == 0 {
			break
		}
		n, err := w.Write(b)
		written += int64(n)
		if err != nil {
			return written, err
		}
		next := uint64(offset) + uint64(len(b))
		if next > uint64(^uint32(0)) {
			return written, fmt.Errorf("MTP file is larger than 4GiB")
		}
		offset = uint32(next)
		if uint32(len(b)) < want {
			break
		}
	}
	return written, nil
}

func (s *session) getObjectFullTo(ctx context.Context, handle uint32, w io.Writer) (int64, error) {
	tx := s.nextTx()
	if err := s.t.Write(encodeCommand(opGetObject, tx, []uint32{handle}), cmdTimeout); err != nil {
		return 0, err
	}
	for {
		if err := s.fillAtLeast(ctx, dataTimeout, containerHeaderSize); err != nil {
			return 0, err
		}
		h, err := decodeHeader(s.pending)
		if err != nil {
			return 0, err
		}
		if h.typ == containerEvent {
			if err := s.skipContainer(ctx, dataTimeout, h); err != nil {
				return 0, err
			}
			continue
		}
		if h.typ == containerResponse {
			rh, _, err := s.takeContainer(ctx, cmdTimeout)
			if err != nil {
				return 0, err
			}
			return 0, checkResponse(rh, opGetObject)
		}
		if h.typ != containerData {
			return 0, fmt.Errorf("MTP: expected data for GetObject, got type %d", h.typ)
		}
		s.consume(containerHeaderSize)
		var written int64
		if h.length == lengthUnknown || int(h.length) < containerHeaderSize {
			return s.copyUntilResponse(ctx, w)
		}
		remaining := int64(h.length) - containerHeaderSize
		for remaining > 0 {
			if err := ctx.Err(); err != nil {
				return written, err
			}
			if len(s.pending) == 0 {
				before := len(s.pending)
				if err := s.readMore(ctx, dataTimeout); err != nil {
					return written, err
				}
				if len(s.pending) == before {
					continue
				}
			}
			take := len(s.pending)
			if int64(take) > remaining {
				take = int(remaining)
			}
			k, err := w.Write(s.pending[:take])
			s.consume(k)
			written += int64(k)
			remaining -= int64(k)
			if err != nil {
				return written, err
			}
			if k == 0 {
				break
			}
		}
		rh, _, err := s.takeContainer(ctx, cmdTimeout)
		if err != nil {
			return written, err
		}
		return written, checkResponse(rh, opGetObject)
	}
}

func (s *session) copyUntilResponse(ctx context.Context, w io.Writer) (int64, error) {
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		if len(s.pending) == 0 {
			before := len(s.pending)
			if err := s.readMore(ctx, dataTimeout); err != nil {
				return written, err
			}
			if len(s.pending) == before {
				continue
			}
		}
		if written > 0 && isResponsePrefix(s.pending) {
			break
		}
		n, err := w.Write(s.pending)
		s.consume(n)
		written += int64(n)
		if err != nil {
			return written, err
		}
		if n == 0 {
			break
		}
	}
	rh, _, err := s.takeContainer(ctx, cmdTimeout)
	if err != nil {
		return written, err
	}
	return written, checkResponse(rh, opGetObject)
}

func isResponsePrefix(b []byte) bool {
	if len(b) < containerHeaderSize {
		return false
	}
	h, err := decodeHeader(b)
	if err != nil {
		return false
	}
	return h.typ == containerResponse && h.length >= containerHeaderSize && h.length <= 32
}

func (s *session) readMore(ctx context.Context, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	chunk, err := s.t.Read(readChunk, timeout)
	if err != nil {
		return err
	}
	if len(chunk) > 0 {
		s.pending = append(s.pending, chunk...)
	}
	return nil
}

func (s *session) consume(n int) []byte {
	if n < 0 {
		n = 0
	}
	if n > len(s.pending) {
		n = len(s.pending)
	}
	out := append([]byte(nil), s.pending[:n]...)
	s.pending = s.pending[n:]
	if len(s.pending) == 0 {
		s.pending = nil
	}
	return out
}

func (s *session) fillAtLeast(ctx context.Context, timeout time.Duration, n int) error {
	for len(s.pending) < n {
		before := len(s.pending)
		if err := s.readMore(ctx, timeout); err != nil {
			return err
		}
		if len(s.pending) == before {
			continue
		}
	}
	return nil
}

func (s *session) takeContainer(ctx context.Context, timeout time.Duration) (containerHeader, []byte, error) {
	if err := s.fillAtLeast(ctx, timeout, containerHeaderSize); err != nil {
		return containerHeader{}, nil, err
	}
	h, err := decodeHeader(s.pending)
	if err != nil {
		return containerHeader{}, nil, err
	}
	if h.typ == containerEvent {
		if err := s.skipContainer(ctx, timeout, h); err != nil {
			return containerHeader{}, nil, err
		}
		return s.takeContainer(ctx, timeout)
	}
	total := int(h.length)
	if h.length == lengthUnknown || total < containerHeaderSize {
		buf := s.consume(len(s.pending))
		var payload []byte
		if len(buf) > containerHeaderSize {
			payload = append([]byte(nil), buf[containerHeaderSize:]...)
		}
		return h, payload, nil
	}
	if err := s.fillAtLeast(ctx, timeout, total); err != nil {
		return containerHeader{}, nil, err
	}
	end := min(total, len(s.pending))
	buf := s.consume(end)
	var payload []byte
	if end > containerHeaderSize {
		payload = append([]byte(nil), buf[containerHeaderSize:]...)
	}
	return h, payload, nil
}

func (s *session) skipContainer(ctx context.Context, timeout time.Duration, h containerHeader) error {
	total := int(h.length)
	if h.length == lengthUnknown || total < containerHeaderSize {
		s.consume(containerHeaderSize)
		return nil
	}
	if err := s.fillAtLeast(ctx, timeout, total); err != nil {
		return err
	}
	s.consume(min(total, len(s.pending)))
	return nil
}

func (s *session) readContainerFrom(ctx context.Context, buf []byte, timeout time.Duration) (containerHeader, []byte, error) {
	if len(buf) > 0 {
		s.pending = append(buf, s.pending...)
	}
	return s.takeContainer(ctx, timeout)
}

func (s *session) readUntilHeader(ctx context.Context) ([]byte, error) {
	if err := s.fillAtLeast(ctx, dataTimeout, containerHeaderSize); err != nil {
		return nil, err
	}
	h, err := decodeHeader(s.pending)
	if err != nil {
		return nil, err
	}
	if h.typ == containerEvent {
		if err := s.skipContainer(ctx, dataTimeout, h); err != nil {
			return nil, err
		}
		return s.readUntilHeader(ctx)
	}
	need := int(h.length)
	if h.length == lengthUnknown || need < containerHeaderSize || h.typ == containerData {
		buf := s.pending
		s.pending = nil
		return buf, nil
	}
	if err := s.fillAtLeast(ctx, dataTimeout, need); err != nil {
		return nil, err
	}
	buf := s.pending
	s.pending = nil
	return buf, nil
}

func (s *session) readContainer(ctx context.Context, timeout time.Duration) (containerHeader, []byte, error) {
	return s.takeContainer(ctx, timeout)
}

func checkResponse(h containerHeader, op uint16) error {
	if h.typ != containerResponse {
		return fmt.Errorf("MTP 0x%04x: expected response, got type %d", op, h.typ)
	}
	if h.code != respOK {
		return responseError{code: h.code}
	}
	return nil
}

func paramsFrom(payload []byte) []uint32 {
	n := len(payload) / 4
	out := make([]uint32, n)
	for i := 0; i < n; i++ {
		out[i] = binary.LittleEndian.Uint32(payload[4*i:])
	}
	return out
}

func (s *session) storageIDs(ctx context.Context) ([]uint32, error) {
	b, err := s.receiveData(ctx, opGetStorageIDs, nil)
	if err != nil {
		return nil, err
	}
	r := byteReader{b: b}
	return r.u32Array()
}

func (s *session) storageInfo(ctx context.Context, id uint32) (storageInfo, error) {
	b, err := s.receiveData(ctx, opGetStorageInfo, []uint32{id})
	if err != nil {
		return storageInfo{}, err
	}
	return parseStorageInfo(b)
}

func (s *session) objectHandles(ctx context.Context, storage, parent uint32) ([]uint32, error) {
	b, err := s.receiveData(ctx, opGetObjectHandles, []uint32{storage, formatAll, parent})
	if err != nil && parent == handleRoot && (isResponse(err, respInvalidParent) || isResponse(err, respInvalidParameter)) {
		// PTP spec uses 0 for the storage root; Android uses 0xFFFFFFFF.
		b, err = s.receiveData(ctx, opGetObjectHandles, []uint32{storage, formatAll, handleAll})
	}
	if err != nil {
		return nil, err
	}
	r := byteReader{b: b}
	return r.u32Array()
}

func (s *session) objectPropList(ctx context.Context, parent uint32) ([]propObject, error) {
	b, err := s.receiveData(ctx, opGetObjectPropList, []uint32{parent, formatAll, propAll, 0, depthDirect})
	if err == nil {
		return parseObjectPropList(b)
	}
	if !isPropListUnsupported(err) {
		return nil, err
	}
	return s.objectPropListPieces(ctx, parent)
}

func (s *session) objectPropListPieces(ctx context.Context, parent uint32) ([]propObject, error) {
	codes := []uint32{propObjectFileName, propObjectFormat, propObjectSize, propDateModified, propParentObject}
	merged := map[uint32]*propObject{}
	ok := false
	for _, code := range codes {
		b, err := s.receiveData(ctx, opGetObjectPropList, []uint32{parent, formatAll, code, 0, depthDirect})
		if err != nil {
			continue
		}
		objs, err := parseObjectPropList(b)
		if err != nil {
			continue
		}
		ok = true
		mergePropObjects(merged, objs)
	}
	if !ok {
		return nil, responseError{code: respOpNotSupported}
	}
	return propMapSlice(merged), nil
}

func (s *session) objectInfo(ctx context.Context, handle uint32) (objectInfo, error) {
	b, err := s.receiveData(ctx, opGetObjectInfo, []uint32{handle})
	if err != nil {
		return objectInfo{}, err
	}
	return parseObjectInfo(b)
}

func (s *session) objectSize64(ctx context.Context, handle uint32) (int64, error) {
	b, err := s.receiveData(ctx, opGetObjectPropValue, []uint32{handle, propObjectSize})
	if err != nil {
		return 0, err
	}
	r := byteReader{b: b}
	v, err := r.u64()
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

func (s *session) createFolder(ctx context.Context, storage, parent uint32, name string) (uint32, error) {
	payload := encodeObjectInfo(storage, parent, fmtAssociation, 0, name, assocGenericFolder)
	resp, err := s.sendData(ctx, opSendObjectInfo, []uint32{storage, parent}, payload)
	if err != nil {
		return 0, err
	}
	if len(resp) >= 3 {
		return resp[2], nil
	}
	return 0, nil
}

func (s *session) sendObjectFile(ctx context.Context, storage, parent uint32, name string, local string, size int64) error {
	infoSize := uint32(size)
	if size > int64(objectSizeMax32) {
		infoSize = objectSizeMax32
	}
	payload := encodeObjectInfo(storage, parent, fmtUndefined, infoSize, name, 0)
	if _, err := s.sendData(ctx, opSendObjectInfo, []uint32{storage, parent}, payload); err != nil {
		return err
	}
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.sendFile(ctx, opSendObject, f, size)
}
