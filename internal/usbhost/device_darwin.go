//go:build darwin

package usbhost

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

const (
	// One bulk IO. The kernel has to wire (and for ordinary memory, bounce)
	// the whole buffer per request, and large requests are what fail with
	// "Unable to send IO" during SendObject on some controllers.
	maxBulkWriteChunk = 64 * 1024
	defaultReadSize   = 512 * 1024
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
	closed       bool
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
	iface := objc.ID(class_IOUSBHostInterface).Send(sel_alloc)
	// initWithIOService already takes exclusive ownership of the interface;
	// IOUSBHostObjectInitOptions has no seize bit.
	iface = iface.Send(sel_initWithIOService, uintptr(svc), uintptr(initOptionsNone), uintptr(0), unsafe.Pointer(&nserr), uintptr(0))
	if iface == 0 {
		return nil, nsError(nserr)
	}

	cfgPtr := objc.Send[unsafe.Pointer](iface, sel_configurationDesc)
	ifacePtr := objc.Send[unsafe.Pointer](iface, sel_interfaceDesc)
	if cfgPtr == nil || ifacePtr == nil {
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, fmt.Errorf("missing USB descriptors")
	}
	ifaceNum := int(*(*byte)(unsafe.Add(ifacePtr, 2)))
	alt := int(*(*byte)(unsafe.Add(ifacePtr, 3)))
	cfgLen := int(*(*uint16)(unsafe.Add(cfgPtr, 2)))
	if cfgLen < 9 {
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, fmt.Errorf("invalid configuration descriptor")
	}
	config := unsafe.Slice((*byte)(cfgPtr), cfgLen)
	ep, ok := parseEndpoints(config, ifaceNum, alt)
	if !ok {
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, fmt.Errorf("MTP bulk endpoints not found")
	}

	bulkOut, err := copyPipe(iface, ep.bulkOut)
	if err != nil {
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, err
	}
	bulkIn, err := copyPipe(iface, ep.bulkIn)
	if err != nil {
		bulkOut.Send(sel_release)
		iface.Send(sel_destroy)
		iface.Send(sel_release)
		return nil, err
	}

	c := &conn{
		info:         info,
		iface:        iface,
		bulkOut:      bulkOut,
		bulkIn:       bulkIn,
		outAddr:      ep.bulkOut,
		inAddr:       ep.bulkIn,
		maxPacketOut: ep.maxPacketOut,
	}
	runtime.SetFinalizer(c, (*conn).finalize)
	return c, nil
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
	c.ioOut.release()
	c.ioIn.release()
	if c.bulkOut != 0 {
		c.bulkOut.Send(sel_release)
		c.bulkOut = 0
	}
	if c.bulkIn != 0 {
		c.bulkIn.Send(sel_release)
		c.bulkIn = 0
	}
	if c.iface != 0 {
		c.iface.Send(sel_destroy)
		c.iface.Send(sel_release)
		c.iface = 0
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
	if max <= 0 {
		max = defaultReadSize
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
	if total <= int64(maxBulkWriteChunk) {
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
	if max >= ioBufferMinSize {
		if data := c.ioBufferLocked(&c.ioIn, max); data != 0 {
			return data, false
		}
	}
	return objc.ID(class_NSMutableData).Send(sel_alloc).Send(sel_initWithLength, uintptr(max)), true
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

func (c *conn) ioRequestN(pipe objc.ID, data objc.ID, length int, timeout time.Duration) (int, error) {
	var transferred uint64
	var nserr objc.ID
	ok := objc.Send[bool](pipe, sel_sendIORequest, data, unsafe.Pointer(&transferred), timeout.Seconds(), unsafe.Pointer(&nserr))
	runtime.KeepAlive(transferred)
	if !ok {
		if !isTimeout(nserr) {
			c.clearStall(pipe)
		}
		return 0, fmt.Errorf("USB %s, %d bytes: %w", c.pipeLabel(pipe), length, nsError(nserr))
	}
	return int(transferred), nil
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
	chunk := maxBulkWriteChunk
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
