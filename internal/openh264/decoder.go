//go:build !darwin

package openh264

import (
	"context"
	"fmt"
	"image"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	libMu     sync.Mutex
	libErr    error
	libReady  bool
	libHandle uintptr

	welsCreateDecoder  func(**isvcDecoder) int
	welsDestroyDecoder func(*isvcDecoder)
)

// Decoder is a single OpenH264 ISVCDecoder instance.
type Decoder struct {
	dec *isvcDecoder
}

// NewDecoder downloads (if needed) and initializes an OpenH264 decoder.
func NewDecoder(ctx context.Context) (*Decoder, error) {
	return NewDecoderProgress(ctx, nil)
}

// NewDecoderProgress is NewDecoder with optional download progress.
func NewDecoderProgress(ctx context.Context, progress ProgressFunc) (*Decoder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !Enabled() {
		return nil, fmt.Errorf("openh264: disabled")
	}
	if err := loadLibrary(ctx, progress); err != nil {
		return nil, err
	}
	var dec *isvcDecoder
	if ret := welsCreateDecoder(&dec); ret != 0 || dec == nil || dec.vtbl == nil {
		return nil, fmt.Errorf("openh264: WelsCreateDecoder=%d", ret)
	}
	param := sDecodingParam{
		UiTargetDqLayer: 255,
		EEcActiveIdc:    errorConSliceCopy,
		SVideoProperty: sVideoProperty{
			Size:         uint32(unsafe.Sizeof(sVideoProperty{})),
			EVideoBsType: videoBitstreamAVC,
		},
	}
	if ret := dec.initialize(&param); ret != cmResultSuccess {
		welsDestroyDecoder(dec)
		return nil, fmt.Errorf("openh264: Initialize=%d", ret)
	}
	return &Decoder{dec: dec}, nil
}

// Close releases the decoder.
func (d *Decoder) Close() {
	if d == nil || d.dec == nil {
		return
	}
	d.dec.uninitialize()
	welsDestroyDecoder(d.dec)
	d.dec = nil
}

// Decode feeds one Annex-B NAL (or access unit). A nil image means no picture
// is ready yet.
func (d *Decoder) Decode(annexB []byte) (*image.NRGBA, error) {
	if d == nil || d.dec == nil {
		return nil, fmt.Errorf("openh264: closed")
	}
	if len(annexB) == 0 {
		return nil, nil
	}
	var dst [3]*byte
	var info sBufferInfo
	ret := d.dec.decodeFrameNoDelay(annexB, &dst, &info)
	if info.IBufferStatus != 1 {
		if ret == dsErrorFree || ret == dsFramePending || ret == dsNoParamSets {
			return nil, nil
		}
		if ret == dsBitstreamError || ret == dsInvalidArgument {
			return nil, fmt.Errorf("openh264: decode status 0x%x", ret)
		}
		return nil, nil
	}
	sys := info.sys()
	w, h := int(sys.IWidth), int(sys.IHeight)
	y, u, v := dst[0], dst[1], dst[2]
	if y == nil {
		y, u, v = (*byte)(info.PDst[0]), (*byte)(info.PDst[1]), (*byte)(info.PDst[2])
	}
	img, err := i420ToNRGBA(y, u, v, w, h, int(sys.IStride[0]), int(sys.IStride[1]))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (d *Decoder) flush() (*image.NRGBA, error) {
	if d == nil || d.dec == nil {
		return nil, fmt.Errorf("openh264: closed")
	}
	var dst [3]*byte
	var info sBufferInfo
	_ = d.dec.decodeFrame2(nil, &dst, &info)
	if info.IBufferStatus != 1 {
		return nil, nil
	}
	sys := info.sys()
	return i420ToNRGBA(dst[0], dst[1], dst[2], int(sys.IWidth), int(sys.IHeight), int(sys.IStride[0]), int(sys.IStride[1]))
}

func resetLibraryForTest() {
	libMu.Lock()
	defer libMu.Unlock()
	if libHandle != 0 {
		_ = purego.Dlclose(libHandle)
		libHandle = 0
	}
	welsCreateDecoder = nil
	welsDestroyDecoder = nil
	libReady = false
	libErr = nil
}

func loadLibrary(ctx context.Context, progress ProgressFunc) error {
	libMu.Lock()
	defer libMu.Unlock()
	if libReady {
		return libErr
	}
	path, err := EnsureLibraryProgress(ctx, progress)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		libReady = true
		libErr = err
		return err
	}
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		libReady = true
		libErr = fmt.Errorf("openh264: dlopen: %w", err)
		return libErr
	}
	if _, err := purego.Dlsym(h, "WelsCreateDecoder"); err != nil {
		_ = purego.Dlclose(h)
		libReady = true
		libErr = fmt.Errorf("openh264: %w", err)
		return libErr
	}
	libHandle = h
	purego.RegisterLibFunc(&welsCreateDecoder, h, "WelsCreateDecoder")
	purego.RegisterLibFunc(&welsDestroyDecoder, h, "WelsDestroyDecoder")
	libReady = true
	return nil
}

func (v *isvcDecoder) initialize(p *sDecodingParam) int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.Initialize)),
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(p)),
	)
	return int(r1)
}

func (v *isvcDecoder) uninitialize() int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.Uninitialize)),
		uintptr(unsafe.Pointer(v)),
	)
	return int(r1)
}

func (v *isvcDecoder) decodeFrame2(src []byte, dst *[3]*byte, info *sBufferInfo) int {
	var pSrc uintptr
	if len(src) > 0 {
		pSrc = uintptr(unsafe.Pointer(&src[0]))
	}
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.DecodeFrame2)),
		uintptr(unsafe.Pointer(v)),
		pSrc,
		uintptr(len(src)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(unsafe.Pointer(info)),
	)
	return int(r1)
}

func (v *isvcDecoder) decodeFrameNoDelay(src []byte, dst *[3]*byte, info *sBufferInfo) int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.DecodeFrameNoDelay)),
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(len(src)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(unsafe.Pointer(info)),
	)
	return int(r1)
}
