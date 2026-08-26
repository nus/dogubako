//go:build darwin

package usbhost

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

const (
	// One bulk URB. Larger requests need the kernel to wire more pages;
	// 64KiB matches the MTP read chunk.
	maxBulkIO = 64 * 1024
	// Defer electrical suspend after the pipes go idle, so a pause between
	// URBs (disk write, scheduling) does not suspend the device.
	idleKeepAwakeSec = 120.0
	// Requests at least this large use an ioData buffer. Command containers
	// are far smaller and are cheaper to hand over as ordinary memory.
	ioBufferMinSize = 4 * 1024
)

type conn struct {
	mu           sync.Mutex
	info         Info
	iface        objc.ID
	bulkOut      objc.ID
	bulkIn       objc.ID
	outAddr      int
	inAddr       int
	maxPacketOut int
	ioOut        ioBuffer
	ioIn         ioBuffer
	interest     objc.Block
	ioBlock      objc.Block
	ioDone       chan ioResult
	closed       bool
}

type ioResult struct {
	n      int
	status uint32
}

// ioBuffer caches a buffer from ioDataWithCapacity:. Its length is fixed, so
// one buffer serves a single request size and is reallocated when that changes.
type ioBuffer struct {
	data objc.ID
	size int
}

func open(id string) (Conn, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newPool()
	defer pool.Send(sel_drain)

	svc, info, err := findService(id)
	if err != nil {
		return nil, err
	}
	defer ioObjectRelease(svc)

	c, err := openService(svc, info)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func openService(svc uint32, info Info) (*conn, error) {
	var nserr objc.ID
	interest := objc.NewBlock(func(_ objc.Block, host objc.ID, messageType uint32, arg unsafe.Pointer) {
		if messageType == usbHostMessageDeviceIsRequestingClose {
			debugf("interest refuse close 0x%08x (another client tried to seize the device)", messageType)
			return
		}
		debugf("interest msg=0x%08x", messageType)
	})
	iface := objc.ID(class_IOUSBHostInterface).Send(sel_alloc)
	iface = iface.Send(sel_initWithIOService, uintptr(svc), uintptr(initOptionsSeize), uintptr(0), unsafe.Pointer(&nserr), interest)
	if iface == 0 {
		interest.Release()
		return nil, nsError(nserr)
	}

	fail := func(err error) (*conn, error) {
		interest.Release()
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, err
	}

	cfgPtr := objc.Send[unsafe.Pointer](iface, sel_configurationDesc)
	ifacePtr := objc.Send[unsafe.Pointer](iface, sel_interfaceDesc)
	if cfgPtr == nil || ifacePtr == nil {
		return fail(fmt.Errorf("missing USB descriptors"))
	}
	ifaceNum := int(*(*byte)(unsafe.Add(ifacePtr, 2)))
	alt := int(*(*byte)(unsafe.Add(ifacePtr, 3)))
	cfgLen := int(*(*uint16)(unsafe.Add(cfgPtr, 2)))
	if cfgLen < 9 {
		return fail(fmt.Errorf("invalid configuration descriptor"))
	}
	config := unsafe.Slice((*byte)(cfgPtr), cfgLen)
	ep, ok := parseEndpoints(config, ifaceNum, alt)
	if !ok {
		return fail(fmt.Errorf("MTP bulk endpoints not found"))
	}

	bulkOut, err := copyPipe(iface, ep.bulkOut)
	if err != nil {
		return fail(err)
	}
	bulkIn, err := copyPipe(iface, ep.bulkIn)
	if err != nil {
		bulkOut.Send(sel_release)
		return fail(err)
	}

	c := &conn{
		info:         info,
		iface:        iface,
		bulkOut:      bulkOut,
		bulkIn:       bulkIn,
		outAddr:      ep.bulkOut,
		inAddr:       ep.bulkIn,
		maxPacketOut: ep.maxPacketOut,
		interest:     interest,
		ioDone:       make(chan ioResult, 2),
	}
	c.ioBlock = objc.NewBlock(func(_ objc.Block, status int32, n uintptr) {
		select {
		case c.ioDone <- ioResult{n: int(n), status: uint32(status)}:
		default:
			// Never block the IOUSBHost completion queue.
		}
	})
	c.setIdleTimeout(idleKeepAwakeSec)
	gotIdle := 0.0
	if sel_idleTimeout != 0 {
		gotIdle = objc.Send[float64](c.iface, sel_idleTimeout)
	}
	debugf("opened %s idleTimeout=%.0fs (property=%.3fs) in=0x%02x out=0x%02x mps=%d", info.ID, idleKeepAwakeSec, gotIdle, ep.bulkIn, ep.bulkOut, c.MaxPacketOut())
	runtime.SetFinalizer(c, (*conn).finalize)
	return c, nil
}

func (c *conn) setIdleTimeout(seconds float64) {
	for _, obj := range []objc.ID{c.iface, c.bulkIn, c.bulkOut} {
		if obj == 0 || sel_setIdleTimeout == 0 {
			continue
		}
		var nserr objc.ID
		ok := objc.Send[bool](obj, sel_setIdleTimeout, seconds, unsafe.Pointer(&nserr))
		if !ok {
			debugf("setIdleTimeout err=%v", nsError(nserr))
		}
	}
}

func copyPipe(iface objc.ID, addr int) (objc.ID, error) {
	var nserr objc.ID
	pipe := iface.Send(sel_copyPipe, uintptr(addr), unsafe.Pointer(&nserr))
	if pipe == 0 {
		return 0, nsError(nserr)
	}
	return pipe, nil
}

func (c *conn) Info() Info { return c.info }

func (c *conn) MaxPacketOut() int {
	if c.maxPacketOut <= 0 {
		return 512
	}
	return c.maxPacketOut
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func (c *conn) finalize() {
	_ = c.Close()
}

func (c *conn) closeLocked() error {
	if c.closed {
		return nil
	}
	c.closed = true
	runtime.SetFinalizer(c, nil)
	c.abortPipe(c.bulkIn)
	c.abortPipe(c.bulkOut)
	c.ioOut.release()
	c.ioIn.release()
	if c.bulkOut != 0 {
		c.abortPipe(c.bulkOut)
		c.bulkOut.Send(sel_release)
		c.bulkOut = 0
	}
	if c.bulkIn != 0 {
		c.abortPipe(c.bulkIn)
		c.bulkIn.Send(sel_release)
		c.bulkIn = 0
	}
	if c.iface != 0 {
		c.iface.Send(sel_destroy)
		c.iface.Send(sel_release)
		c.iface = 0
	}
	if c.ioBlock != 0 {
		c.ioBlock.Release()
		c.ioBlock = 0
	}
	if c.interest != 0 {
		c.interest.Release()
		c.interest = 0
	}
	return nil
}

func (c *conn) Write(p []byte, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("usb connection closed")
	}
	if len(p) == 0 {
		return c.sendZLPLocked(timeout)
	}
	if err := c.sendLocked(c.bulkOut, p, timeout); err != nil {
		return err
	}
	// A data phase whose length is an exact multiple of the packet size has to
	// be closed with a zero-length packet, or the responder keeps waiting.
	return c.maybeZLPLocked(int64(len(p)), timeout)
}

func (c *conn) Read(max int, timeout time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("usb connection closed")
	}
	if max <= 0 || max > maxBulkIO {
		max = maxBulkIO
	}
	return c.recvLocked(c.bulkIn, max, timeout)
}

func (c *conn) WriteStream(header []byte, r io.Reader, size int64, timeout time.Duration) error {
	return c.WriteStreamProgress(header, r, size, timeout, nil)
}

func (c *conn) WriteStreamProgress(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("usb connection closed")
	}
	total := int64(len(header)) + size
	if total < 0 {
		return fmt.Errorf("invalid stream size")
	}
	if total <= int64(maxBulkIO) {
		buf := make([]byte, total)
		copy(buf, header)
		if size > 0 {
			if _, err := io.ReadFull(r, buf[len(header):]); err != nil {
				return err
			}
		}
		if err := c.sendLocked(c.bulkOut, buf, timeout); err != nil {
			return err
		}
		if wrote != nil && size > 0 {
			wrote(size)
		}
		return c.maybeZLPLocked(total, timeout)
	}
	if err := c.writeStreamLocked(header, r, size, timeout, wrote); err != nil {
		return err
	}
	return c.maybeZLPLocked(total, timeout)
}

func (c *conn) maybeZLPLocked(total int64, timeout time.Duration) error {
	mps := c.MaxPacketOut()
	if mps <= 0 || total == 0 || total%int64(mps) != 0 {
		return nil
	}
	return c.sendZLPLocked(timeout)
}

func (c *conn) sendZLPLocked(timeout time.Duration) error {
	return c.sendLocked(c.bulkOut, nil, timeout)
}

func (c *conn) sendLocked(pipe objc.ID, p []byte, timeout time.Duration) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := newPool()
	defer pool.Send(sel_drain)

	if len(p) == 0 {
		// A nil buffer sends a zero-length packet.
		_, err := c.ioRequestN(pipe, 0, 0, timeout)
		return err
	}
	data, release, err := c.writeBufferLocked(p)
	if err != nil {
		return err
	}
	if release {
		defer data.Send(sel_release)
	}
	_, err = c.ioRequestN(pipe, data, len(p), timeout)
	return err
}

func (c *conn) recvLocked(pipe objc.ID, max int, timeout time.Duration) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := newPool()
	defer pool.Send(sel_drain)

	data, release := c.readBufferLocked(max)
	if data == 0 {
		return nil, fmt.Errorf("allocate USB read buffer")
	}
	if release {
		defer data.Send(sel_release)
	}

	n, err := c.ioRequestN(pipe, data, max, timeout)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return []byte{}, nil
	}
	ptr := objc.Send[unsafe.Pointer](data, sel_mutableBytes)
	if ptr == nil {
		return nil, fmt.Errorf("USB read buffer pointer is nil")
	}
	out := make([]byte, n)
	copy(out, unsafe.Slice((*byte)(ptr), n))
	return out, nil
}

// writeBufferLocked returns a buffer holding p, and whether the caller owns it.
func (c *conn) writeBufferLocked(p []byte) (objc.ID, bool, error) {
	if len(p) >= ioBufferMinSize {
		if data := c.ioBufferLocked(&c.ioOut, len(p)); data != 0 {
			if !copyIntoData(data, p) {
				return 0, false, fmt.Errorf("USB write buffer pointer is nil")
			}
			return data, false, nil
		}
	}
	data, err := dataWithBytes(p)
	if err != nil {
		return 0, false, err
	}
	return data, true, nil
}

func (c *conn) readBufferLocked(max int) (objc.ID, bool) {
	// Reuse one NSMutableData. A completed IN may shrink length to the
	// short-packet size; restore it so the next URB is still `max` bytes.
	if c.ioIn.data != 0 && c.ioIn.size == max {
		c.ioIn.data.Send(sel_setLength, uintptr(max))
		return c.ioIn.data, false
	}
	c.ioIn.release()
	data := objc.ID(class_NSMutableData).Send(sel_alloc).Send(sel_initWithLength, uintptr(max))
	if data == 0 {
		return 0, true
	}
	c.ioIn.data = data
	c.ioIn.size = max
	return data, false
}

// ioBufferLocked returns a cached IOBufferMemoryDescriptor-backed buffer of
// exactly size bytes. The kernel can DMA straight out of these, while ordinary
// NSMutableData has to be bounced for every request, which is where large bulk
// transfers fail. Returns 0 when the framework cannot provide one.
func (c *conn) ioBufferLocked(b *ioBuffer, size int) objc.ID {
	if b.data != 0 && b.size == size {
		return b.data
	}
	b.release()
	var nserr objc.ID
	data := objc.Send[objc.ID](c.iface, sel_ioDataWithCapacity, uintptr(size), unsafe.Pointer(&nserr))
	if data == 0 {
		return 0
	}
	// The name is not alloc/new/copy, so the buffer is autoreleased and would
	// go away when the enclosing pool drains.
	data.Send(sel_retain)
	b.data = data
	b.size = size
	return data
}

func (b *ioBuffer) release() {
	if b.data != 0 {
		b.data.Send(sel_release)
	}
	b.data = 0
	b.size = 0
}

func (c *conn) abortPipe(pipe objc.ID) {
	if pipe == 0 || sel_abort == 0 {
		return
	}
	var nserr objc.ID
	_ = objc.Send[bool](pipe, sel_abort, unsafe.Pointer(&nserr))
}

func (c *conn) ioRequestN(pipe objc.ID, data objc.ID, length int, timeout time.Duration) (int, error) {
	n, code, err := c.submitIO(pipe, data, length, timeout)
	if err != nil {
		if shouldClearStallCode(code) {
			c.clearStall(pipe)
		}
		return 0, fmt.Errorf("USB %s, %d bytes: %w", c.pipeLabel(pipe), length, err)
	}
	return n, nil
}

func (c *conn) submitIO(pipe objc.ID, data objc.ID, length int, timeout time.Duration) (int, uint32, error) {
	dataLen := 0
	if data != 0 && sel_length != 0 {
		dataLen = int(objc.Send[uintptr](data, sel_length))
	}
	select {
	case <-c.ioDone:
	default:
	}
	var nserr objc.ID
	t0 := time.Now()
	ok := objc.Send[bool](pipe, sel_enqueueIORequest, data, timeout.Seconds(), unsafe.Pointer(&nserr), c.ioBlock)
	if !ok {
		err := nsError(nserr)
		debugf("%s req=%d nsdata.len=%d xfer=0 dur=%s err=%v", c.pipeLabel(pipe), length, dataLen, time.Since(t0), err)
		return 0, nsErrorCode(nserr), err
	}
	return c.waitIO(pipe, length, dataLen, timeout, t0)
}

func (c *conn) waitIO(pipe objc.ID, length, dataLen int, timeout time.Duration, t0 time.Time) (int, uint32, error) {
	wait := timeout + time.Second
	if timeout <= 0 {
		wait = 2 * time.Minute
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case res := <-c.ioDone:
		if res.status != 0 {
			err := ioReturnErr(res.status)
			debugf("%s req=%d nsdata.len=%d xfer=%d dur=%s err=%v", c.pipeLabel(pipe), length, dataLen, res.n, time.Since(t0), err)
			return res.n, res.status, err
		}
		debugf("%s req=%d nsdata.len=%d xfer=%d dur=%s", c.pipeLabel(pipe), length, dataLen, res.n, time.Since(t0))
		return res.n, 0, nil
	case <-timer.C:
		debugf("%s req=%d nsdata.len=%d xfer=0 dur=%s err=%v", c.pipeLabel(pipe), length, dataLen, time.Since(t0), ioReturnErr(ioReturnTimeout))
		c.abortPipe(pipe)
		drain := time.NewTimer(time.Second)
		defer drain.Stop()
		select {
		case res := <-c.ioDone:
			if res.status != 0 {
				return res.n, res.status, ioReturnErr(res.status)
			}
			return res.n, ioReturnTimeout, ioReturnErr(ioReturnTimeout)
		case <-drain.C:
			return 0, ioReturnTimeout, ioReturnErr(ioReturnTimeout)
		}
	}
}

func (c *conn) pipeLabel(pipe objc.ID) string {
	switch pipe {
	case c.bulkOut:
		return fmt.Sprintf("bulk OUT 0x%02x", c.outAddr)
	case c.bulkIn:
		return fmt.Sprintf("bulk IN 0x%02x", c.inAddr)
	default:
		return "pipe"
	}
}

func copyIntoData(data objc.ID, p []byte) bool {
	ptr := objc.Send[unsafe.Pointer](data, sel_mutableBytes)
	if ptr == nil {
		return false
	}
	copy(unsafe.Slice((*byte)(ptr), len(p)), p)
	runtime.KeepAlive(p)
	return true
}

func (c *conn) clearStall(pipe objc.ID) {
	var nserr objc.ID
	_ = objc.Send[bool](pipe, sel_clearStall, unsafe.Pointer(&nserr))
}

func dataWithBytes(p []byte) (objc.ID, error) {
	if len(p) == 0 {
		return 0, nil
	}
	ptr := unsafe.Pointer(unsafe.SliceData(p))
	data := objc.ID(class_NSMutableData).Send(sel_alloc).Send(sel_initWithBytes, ptr, uintptr(len(p)))
	runtime.KeepAlive(p)
	if data == 0 {
		return 0, fmt.Errorf("allocate USB write buffer")
	}
	return data, nil
}

func (c *conn) writeStreamLocked(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error {
	// Chunk at a multiple of wMaxPacketSize so each URB except the last is
	// full packets. A short packet in the middle of SendObject wedges Android.
	// Do not LockOSThread around file reads; sendLocked pins the thread per URB.
	mps := c.MaxPacketOut()
	chunk := maxBulkIO
	if mps > 1 {
		chunk = (chunk / mps) * mps
		if chunk == 0 {
			chunk = mps
		}
	}

	total := int64(len(header)) + size
	var (
		sent    int64
		hdrOff  int
		fileOff int64
		buf     = make([]byte, chunk)
	)
	for sent < total {
		n := chunk
		if left := int(total - sent); left < n {
			n = left
		}
		filled := 0
		hdr := 0
		if hdrOff < len(header) {
			ncopy := copy(buf, header[hdrOff:])
			hdrOff += ncopy
			filled += ncopy
			hdr = ncopy
		}
		if filled < n && fileOff < size {
			want := n - filled
			if left := int(size - fileOff); left < want {
				want = left
			}
			got, err := io.ReadFull(r, buf[filled:filled+want])
			if err != nil {
				return err
			}
			filled += got
			fileOff += int64(got)
		}
		if err := c.sendLocked(c.bulkOut, buf[:filled], timeout); err != nil {
			return err
		}
		sent += int64(filled)
		if wrote != nil {
			if payload := int64(filled - hdr); payload > 0 {
				wrote(payload)
			}
		}
	}
	return nil
}

func debugf(format string, args ...any) {
	if os.Getenv("DOGUBAKO_MTP_DEBUG") == "" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if os.Getenv("DOGUBAKO_MTP_DEBUG") == "1" && !strings.Contains(msg, "err=") && !strings.Contains(msg, "opened ") && !strings.Contains(msg, "interest ") {
		return
	}
	fmt.Fprintf(os.Stderr, "usbhost: %s\n", msg)
}
