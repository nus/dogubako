package h264

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	cfNumberSInt32Type = 3

	pixel32BGRA = 0x42475241 // 'BGRA'
	pixel32ARGB = 0x41524742 // 'ARGB'
	pixel32RGBA = 0x52474241 // 'RGBA'
	pixel420v   = 0x34323076 // '420v'
	pixel420f   = 0x34323066 // '420f'

	cvPixelBufferLockReadOnly = 1
	maxFrameEdge              = 16384
)

type outputCallbackRecord struct {
	callback uintptr
	refCon   uintptr
}

var (
	vtOnce sync.Once
	vtErr  error

	decodeCallback = purego.NewCallback(vtOutput)

	cfRelease           func(cf uintptr)
	cfNumberCreate      func(allocator uintptr, theType int64, valuePtr unsafe.Pointer) uintptr
	cfDictionaryCreate  func(allocator, keys, values uintptr, n int64, keyCb, valCb uintptr) uintptr
	kCFTypeKeyCallbacks uintptr
	kCFTypeValCallbacks uintptr
	kCFAllocatorNull    uintptr
	kCVPixelFormatKey   uintptr

	cmVideoFormatDescriptionCreateFromH264ParameterSets func(allocator uintptr, count uintptr, pointers, sizes unsafe.Pointer, nalHeaderLen int32, out *uintptr) int32
	cmBlockBufferCreateWithMemoryBlock                  func(structureAllocator uintptr, memoryBlock unsafe.Pointer, blockLength uintptr, blockAllocator uintptr, customBlockSource uintptr, offsetToData, dataLength uintptr, flags uint32, out *uintptr) int32
	cmSampleBufferCreateReady                           func(allocator, dataBuffer, format uintptr, numSamples, numSampleTimingEntries int64, sampleTimingArray uintptr, numSampleSizeEntries int64, sampleSizeArray unsafe.Pointer, out *uintptr) int32

	cvPixelBufferLockBaseAddress       func(buf uintptr, flags uint32) int32
	cvPixelBufferUnlockBaseAddress     func(buf uintptr, flags uint32) int32
	cvPixelBufferGetBaseAddress        func(buf uintptr) unsafe.Pointer
	cvPixelBufferGetBaseAddressOfPlane func(buf uintptr, plane uintptr) unsafe.Pointer
	cvPixelBufferGetBytesPerRow        func(buf uintptr) uintptr
	cvPixelBufferGetBytesPerRowOfPlane func(buf uintptr, plane uintptr) uintptr
	cvPixelBufferGetWidth              func(buf uintptr) uintptr
	cvPixelBufferGetHeight             func(buf uintptr) uintptr
	cvPixelBufferGetPixelFormatType    func(buf uintptr) uint32
	cvPixelBufferIsPlanar              func(buf uintptr) bool

	vtDecompressionSessionCreate                    func(allocator, format, decoderSpec, destAttrs, outputCallback uintptr, out *uintptr) int32
	vtDecompressionSessionDecodeFrame               func(session, sample uintptr, decodeFlags uint32, sourceFrameRefCon, infoFlagsOut uintptr) int32
	vtDecompressionSessionInvalidate                func(session uintptr)
	vtDecompressionSessionWaitForAsynchronousFrames func(session uintptr) int32
)

func loadVideoToolbox() error {
	vtOnce.Do(func() {
		vtErr = loadVideoToolboxLocked()
	})
	return vtErr
}

func loadVideoToolboxLocked() error {
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("videotoolbox: CoreFoundation: %w", err)
	}
	cm, err := purego.Dlopen("/System/Library/Frameworks/CoreMedia.framework/CoreMedia", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("videotoolbox: CoreMedia: %w", err)
	}
	cv, err := purego.Dlopen("/System/Library/Frameworks/CoreVideo.framework/CoreVideo", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("videotoolbox: CoreVideo: %w", err)
	}
	vt, err := purego.Dlopen("/System/Library/Frameworks/VideoToolbox.framework/VideoToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("videotoolbox: VideoToolbox: %w", err)
	}

	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	purego.RegisterLibFunc(&cfNumberCreate, cf, "CFNumberCreate")
	purego.RegisterLibFunc(&cfDictionaryCreate, cf, "CFDictionaryCreate")

	kCFTypeKeyCallbacks, err = purego.Dlsym(cf, "kCFTypeDictionaryKeyCallBacks")
	if err != nil {
		return fmt.Errorf("videotoolbox: %w", err)
	}
	kCFTypeValCallbacks, err = purego.Dlsym(cf, "kCFTypeDictionaryValueCallBacks")
	if err != nil {
		return fmt.Errorf("videotoolbox: %w", err)
	}
	kCFAllocatorNull, err = cfConst(cf, "kCFAllocatorNull")
	if err != nil {
		return err
	}
	kCVPixelFormatKey, err = cfConst(cv, "kCVPixelBufferPixelFormatTypeKey")
	if err != nil {
		return err
	}

	purego.RegisterLibFunc(&cmVideoFormatDescriptionCreateFromH264ParameterSets, cm, "CMVideoFormatDescriptionCreateFromH264ParameterSets")
	purego.RegisterLibFunc(&cmBlockBufferCreateWithMemoryBlock, cm, "CMBlockBufferCreateWithMemoryBlock")
	purego.RegisterLibFunc(&cmSampleBufferCreateReady, cm, "CMSampleBufferCreateReady")

	purego.RegisterLibFunc(&cvPixelBufferLockBaseAddress, cv, "CVPixelBufferLockBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferUnlockBaseAddress, cv, "CVPixelBufferUnlockBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferGetBaseAddress, cv, "CVPixelBufferGetBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferGetBaseAddressOfPlane, cv, "CVPixelBufferGetBaseAddressOfPlane")
	purego.RegisterLibFunc(&cvPixelBufferGetBytesPerRow, cv, "CVPixelBufferGetBytesPerRow")
	purego.RegisterLibFunc(&cvPixelBufferGetBytesPerRowOfPlane, cv, "CVPixelBufferGetBytesPerRowOfPlane")
	purego.RegisterLibFunc(&cvPixelBufferGetWidth, cv, "CVPixelBufferGetWidth")
	purego.RegisterLibFunc(&cvPixelBufferGetHeight, cv, "CVPixelBufferGetHeight")
	purego.RegisterLibFunc(&cvPixelBufferGetPixelFormatType, cv, "CVPixelBufferGetPixelFormatType")
	purego.RegisterLibFunc(&cvPixelBufferIsPlanar, cv, "CVPixelBufferIsPlanar")

	purego.RegisterLibFunc(&vtDecompressionSessionCreate, vt, "VTDecompressionSessionCreate")
	purego.RegisterLibFunc(&vtDecompressionSessionDecodeFrame, vt, "VTDecompressionSessionDecodeFrame")
	purego.RegisterLibFunc(&vtDecompressionSessionInvalidate, vt, "VTDecompressionSessionInvalidate")
	purego.RegisterLibFunc(&vtDecompressionSessionWaitForAsynchronousFrames, vt, "VTDecompressionSessionWaitForAsynchronousFrames")
	return nil
}

func cfConst(lib uintptr, name string) (uintptr, error) {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0, fmt.Errorf("videotoolbox: %s: %w", name, err)
	}
	return **(**uintptr)(unsafe.Pointer(&addr)), nil
}

func release(p uintptr) {
	if p != 0 {
		cfRelease(p)
	}
}

func pixelFormatAttrs() uintptr {
	pix := uint32(pixel32BGRA)
	num := cfNumberCreate(0, cfNumberSInt32Type, unsafe.Pointer(&pix))
	if num == 0 {
		return 0
	}
	keys := [1]uintptr{kCVPixelFormatKey}
	vals := [1]uintptr{num}
	dict := cfDictionaryCreate(0, uintptr(unsafe.Pointer(&keys[0])), uintptr(unsafe.Pointer(&vals[0])), 1, kCFTypeKeyCallbacks, kCFTypeValCallbacks)
	release(num)
	return dict
}
