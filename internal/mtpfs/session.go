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
	readChunk    = 64 * 1024 // matches usbhost bulk IN cap
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
	broken  bool
}

func (s *session) close() {
	if s == nil || s.t == nil {
		return
	}
	if !s.broken {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = s.runCommand(ctx, opCloseSession, nil)
		cancel()
	}
	_ = s.t.Close()
	s.t = nil
}

func (s *session) kill(err error) error {
	if s != nil {
		s.broken = true
	}
	return err
}

func isAnyResponse(err error) bool {
	var re responseError
	return errorAs(err, &re)
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
			_, closeErr := s.runCommand(ctx, opCloseSession, nil)
			if s.broken {
				return closeErr
			}
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
	if _, err := s.receiveData(ctx, opGetDeviceInfo, nil); err != nil && s.broken {
		return err
	}
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
		return nil, s.kill(err)
	}
	h, payload, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := s.checkResponse(h, op, tx); err != nil {
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
		return nil, s.kill(err)
	}
	h, payload, err := s.readContainer(ctx, dataTimeout)
	if err != nil {
		return nil, err
	}
	if h.typ == containerResponse {
		if err := s.checkResponse(h, op, tx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if h.typ != containerData {
		return nil, s.kill(fmt.Errorf("MTP: expected data container, got type %d", h.typ))
	}
	if err := s.checkTx(h, tx, op); err != nil {
		return nil, err
	}
	rh, rpayload, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := s.checkResponse(rh, op, tx); err != nil {
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
		return nil, s.kill(err)
	}
	if err := s.t.Write(encodeData(op, tx, payload), dataTimeout); err != nil {
		return nil, s.kill(err)
	}
	h, body, err := s.readContainer(ctx, cmdTimeout)
	if err != nil {
		return nil, err
	}
	if err := s.checkResponse(h, op, tx); err != nil {
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
		return s.kill(err)
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
		return s.kill(err)
	}
	h, _, err := s.readContainer(ctx, dataTimeout)
	if err != nil {
		return err
	}
	return s.checkResponse(h, op, tx)
}

func (s *session) getObjectTo(ctx context.Context, handle uint32, size int64, w io.Writer) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if size <= 0 || size == int64(objectSizeMax32) {
		if sz, err := s.objectSize64(ctx, handle); err == nil && sz >= 0 {
			size = sz
		} else if s.broken {
			return 0, err
		}
	}
	// Full-file GetObject is one bulk data phase. GetPartialObject would
	// round-trip a command every 256KiB.
	return s.getObjectFullTo(ctx, handle, w)
}

func isPartialObjectUnsupported(err error) bool {
	return isResponse(err, respOpNotSupported) ||
		isResponse(err, respParamNotSupported) ||
		isResponse(err, respInvalidParameter) ||
		isResponse(err, respIncompleteTransfer)
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
		return 0, s.kill(err)
	}
	for {
		if err := s.fillAtLeast(ctx, dataTimeout, containerHeaderSize); err != nil {
			return 0, err
		}
		h, err := decodeHeader(s.pending)
		if err != nil {
			return 0, s.kill(err)
		}
		if err := s.validateHeader(h); err != nil {
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
			return 0, s.checkResponse(rh, opGetObject, tx)
		}
		if h.typ != containerData {
			return 0, s.kill(fmt.Errorf("MTP: expected data for GetObject, got type %d", h.typ))
		}
		if err := s.checkTx(h, tx, opGetObject); err != nil {
			return 0, err
		}
		s.consume(containerHeaderSize)
		var written int64
		if h.length == lengthUnknown || int(h.length) < containerHeaderSize {
			return s.copyUntilResponse(ctx, tx, w)
		}
		remaining := int64(h.length) - containerHeaderSize
		for remaining > 0 {
			if err := ctx.Err(); err != nil {
				return written, s.kill(err)
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
				// Local write failed but the device is still sending object data.
				return written, s.kill(err)
			}
			if k == 0 {
				break
			}
		}
		rh, _, err := s.takeContainer(ctx, cmdTimeout)
		if err != nil {
			return written, err
		}
		return written, s.checkResponse(rh, opGetObject, tx)
	}
}

func (s *session) copyUntilResponse(ctx context.Context, tx uint32, w io.Writer) (int64, error) {
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, s.kill(err)
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
		if data, _, ok := splitTrailingResponse(s.pending, tx); ok {
			if len(data) > 0 {
				n, err := w.Write(data)
				s.consume(n)
				written += int64(n)
				if err != nil {
					return written, s.kill(err)
				}
			}
			break
		}
		n, err := w.Write(s.pending)
		s.consume(n)
		written += int64(n)
		if err != nil {
			return written, s.kill(err)
		}
		if n == 0 {
			break
		}
	}
	rh, _, err := s.takeContainer(ctx, cmdTimeout)
	if err != nil {
		return written, err
	}
	return written, s.checkResponse(rh, opGetObject, tx)
}

// splitTrailingResponse reports whether b ends with an MTP response container.
// tx 0 accepts any transaction id.
func splitTrailingResponse(b []byte, tx uint32) (data, resp []byte, ok bool) {
	if len(b) < containerHeaderSize {
		return b, nil, false
	}
	max := 32
	if len(b) < max {
		max = len(b)
	}
	for n := containerHeaderSize; n <= max; n++ {
		off := len(b) - n
		h, err := decodeHeader(b[off:])
		if err != nil {
			continue
		}
		if h.typ != containerResponse || int(h.length) != n {
			continue
		}
		if tx != 0 && h.transactionID != tx {
			continue
		}
		return b[:off], b[off:], true
	}
	return b, nil, false
}

func (s *session) readMore(ctx context.Context, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return s.kill(err)
	}
	chunk, err := s.t.Read(readChunk, timeout)
	if err != nil {
		return s.kill(err)
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
		return containerHeader{}, nil, s.kill(err)
	}
	if err := s.validateHeader(h); err != nil {
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

const maxDataContainer = 128 << 20

func (s *session) validateHeader(h containerHeader) error {
	switch h.typ {
	case containerCommand, containerResponse:
		if h.length < containerHeaderSize || h.length > 32 {
			return s.kill(fmt.Errorf("MTP: invalid container length %d type %d", h.length, h.typ))
		}
	case containerEvent:
		if h.length != lengthUnknown && (h.length < containerHeaderSize || h.length > 64) {
			return s.kill(fmt.Errorf("MTP: invalid event length %d", h.length))
		}
	case containerData:
		if h.length != lengthUnknown && (h.length < containerHeaderSize || int(h.length) > maxDataContainer) {
			return s.kill(fmt.Errorf("MTP: invalid data container length %d", h.length))
		}
	default:
		return s.kill(fmt.Errorf("MTP: invalid container type %d", h.typ))
	}
	return nil
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

func checkResponse(h containerHeader, op uint16, tx uint32) error {
	if h.typ != containerResponse {
		return fmt.Errorf("MTP 0x%04x: expected response, got type %d", op, h.typ)
	}
	if h.transactionID != tx {
		return fmt.Errorf("MTP 0x%04x: transaction mismatch got %d want %d", op, h.transactionID, tx)
	}
	if h.code != respOK {
		return responseError{code: h.code}
	}
	return nil
}

func (s *session) checkResponse(h containerHeader, op uint16, tx uint32) error {
	err := checkResponse(h, op, tx)
	if err == nil || isAnyResponse(err) {
		return err
	}
	return s.kill(err)
}

func (s *session) checkTx(h containerHeader, tx uint32, op uint16) error {
	if h.transactionID == tx {
		return nil
	}
	return s.kill(fmt.Errorf("MTP 0x%04x: transaction mismatch got %d want %d", op, h.transactionID, tx))
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
			if s.broken || !isAnyResponse(err) {
				return nil, err
			}
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
