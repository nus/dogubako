package h264

import (
	"bytes"
	"fmt"
	"image"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	handleSeq atomic.Uintptr
	decoders  sync.Map // uintptr -> *vtDecoder
)

type vtDecoder struct {
	handle  uintptr
	cbRec   outputCallbackRecord
	session uintptr
	format  uintptr
	sps     []byte
	pps     []byte
	closed  atomic.Bool

	mu  sync.Mutex
	img *image.NRGBA
	err error
}

func newVTDecoder() (*vtDecoder, error) {
	if err := loadVideoToolbox(); err != nil {
		return nil, err
	}
	d := &vtDecoder{handle: handleSeq.Add(1)}
	d.cbRec = outputCallbackRecord{callback: decodeCallback, refCon: d.handle}
	decoders.Store(d.handle, d)
	return d, nil
}

func (d *vtDecoder) Close() {
	if d == nil || !d.closed.CompareAndSwap(false, true) {
		return
	}
	decoders.Delete(d.handle)
	if d.session != 0 {
		_ = vtDecompressionSessionWaitForAsynchronousFrames(d.session)
		vtDecompressionSessionInvalidate(d.session)
		release(d.session)
		d.session = 0
	}
	release(d.format)
	d.format = 0
}

func (d *vtDecoder) Decode(annexB []byte) (*image.NRGBA, error) {
	if d == nil || d.closed.Load() {
		return nil, fmt.Errorf("videotoolbox: closed")
	}
	if len(annexB) == 0 {
		return nil, nil
	}
	t := nalType(annexB)
	payload := nalPayload(annexB)
	if len(payload) == 0 {
		return nil, nil
	}
	switch t {
	case nalSPS, nalPPS:
		return nil, d.applyParameterSet(t, payload)
	case nalSEI, nalAUD:
		return nil, nil
	}
	if !isVCL(t) || d.session == 0 {
		return nil, nil
	}
	return d.decodeAVCC(toAVCC(annexB))
}

func (d *vtDecoder) applyParameterSet(kind byte, payload []byte) error {
	payload = bytes.Clone(payload)
	switch kind {
	case nalSPS:
		if bytes.Equal(d.sps, payload) {
			return nil
		}
		d.sps = payload
	case nalPPS:
		if bytes.Equal(d.pps, payload) {
			return nil
		}
		d.pps = payload
	}
	if len(d.sps) == 0 || len(d.pps) == 0 {
		return nil
	}
	return d.recreateSession()
}

func (d *vtDecoder) recreateSession() error {
	sets := [2]uintptr{
		uintptr(unsafe.Pointer(&d.sps[0])),
		uintptr(unsafe.Pointer(&d.pps[0])),
	}
	sizes := [2]uintptr{uintptr(len(d.sps)), uintptr(len(d.pps))}
	var format uintptr
	st := cmVideoFormatDescriptionCreateFromH264ParameterSets(0, 2, unsafe.Pointer(&sets[0]), unsafe.Pointer(&sizes[0]), 4, &format)
	runtime.KeepAlive(d.sps)
	runtime.KeepAlive(d.pps)
	if st != 0 || format == 0 {
		return fmt.Errorf("videotoolbox: format description=%d", st)
	}
	attrs := pixelFormatAttrs()
	var session uintptr
	st = vtDecompressionSessionCreate(0, format, 0, attrs, uintptr(unsafe.Pointer(&d.cbRec)), &session)
	release(attrs)
	if (st != 0 || session == 0) && attrs != 0 {
		st = vtDecompressionSessionCreate(0, format, 0, 0, uintptr(unsafe.Pointer(&d.cbRec)), &session)
	}
	if st != 0 || session == 0 {
		release(format)
		return fmt.Errorf("videotoolbox: session create=%d", st)
	}
	if d.session != 0 {
		_ = vtDecompressionSessionWaitForAsynchronousFrames(d.session)
		vtDecompressionSessionInvalidate(d.session)
		release(d.session)
	}
	release(d.format)
	d.format = format
	d.session = session
	runtime.KeepAlive(d.cbRec)
	return nil
}

func (d *vtDecoder) decodeAVCC(avcc []byte) (*image.NRGBA, error) {
	if len(avcc) < 5 {
		return nil, nil
	}
	d.mu.Lock()
	d.img = nil
	d.err = nil
	d.mu.Unlock()

	var block uintptr
	st := cmBlockBufferCreateWithMemoryBlock(0, unsafe.Pointer(&avcc[0]), uintptr(len(avcc)), kCFAllocatorNull, 0, 0, uintptr(len(avcc)), 0, &block)
	if st != 0 || block == 0 {
		return nil, fmt.Errorf("videotoolbox: block buffer=%d", st)
	}
	size := uintptr(len(avcc))
	var sample uintptr
	st = cmSampleBufferCreateReady(0, block, d.format, 1, 0, 0, 1, unsafe.Pointer(&size), &sample)
	release(block)
	if st != 0 || sample == 0 {
		return nil, fmt.Errorf("videotoolbox: sample buffer=%d", st)
	}
	st = vtDecompressionSessionDecodeFrame(d.session, sample, 0, 0, 0)
	_ = vtDecompressionSessionWaitForAsynchronousFrames(d.session)
	release(sample)
	runtime.KeepAlive(avcc)
	runtime.KeepAlive(size)
	runtime.KeepAlive(d.cbRec)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return nil, d.err
	}
	if st != 0 && d.img == nil {
		return nil, fmt.Errorf("videotoolbox: DecodeFrame=%d", st)
	}
	return d.img, nil
}

func vtOutput(refCon, _, status, _, imageBuffer uintptr) {
	v, ok := decoders.Load(refCon)
	if !ok {
		return
	}
	v.(*vtDecoder).onFrame(status, imageBuffer)
}

func (d *vtDecoder) onFrame(status, imageBuffer uintptr) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed.Load() {
		return
	}
	if int32(status) != 0 {
		d.err = fmt.Errorf("videotoolbox: decode status %d", int32(status))
		return
	}
	if imageBuffer == 0 {
		return
	}
	img, err := copyPixelBuffer(imageBuffer)
	if err != nil {
		d.err = err
		return
	}
	d.img = img
	d.err = nil
}
