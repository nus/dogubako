//go:build !darwin

package openh264

import (
	"bytes"
	"context"
	"image"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

func TestNewDecoderDisabled(t *testing.T) {
	t.Setenv(EnvName, "0")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := NewDecoder(ctx); err == nil {
		t.Fatal("expected disabled")
	}
}

func TestEnsureLibrarySkippedInTest(t *testing.T) {
	resetLibraryForTest()
	t.Setenv(EnvName, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_, err := EnsureLibrary(context.Background())
	if err == nil {
		t.Fatal("go test should not download into a fresh cache")
	}
}

func TestDecoderRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("download")
	}
	resetLibraryForTest()
	t.Setenv(EnvName, "download")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	path, err := EnsureLibrary(ctx)
	if err != nil {
		t.Skipf("cisco binary: %v", err)
	}
	dec, err := NewDecoder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	stream, err := encodeSolidI420(path, 16, 16, 81, 90, 240)
	if err != nil {
		t.Fatal(err)
	}
	if len(stream) < 8 {
		t.Fatalf("encoded stream too short: %d", len(stream))
	}
	t.Logf("annex-b %d bytes %x", len(stream), stream[:min(32, len(stream))])
	var frames int
	if err := splitAnnexBForTest(stream, func(nal []byte) error {
		img, err := dec.Decode(nal)
		if err != nil {
			t.Logf("decode: %v", err)
			return nil
		}
		if img == nil {
			return nil
		}
		frames++
		if img.Bounds() != image.Rect(0, 0, 16, 16) {
			t.Fatalf("bounds = %v", img.Bounds())
		}
		if img.NRGBAAt(8, 8).A != 255 {
			t.Fatalf("pixel = %+v", img.NRGBAAt(8, 8))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if frames == 0 {
		if img, err := dec.flush(); err != nil {
			t.Fatal(err)
		} else if img != nil {
			frames++
			if img.Bounds() != image.Rect(0, 0, 16, 16) {
				t.Fatalf("flushed bounds = %v", img.Bounds())
			}
		}
	}
	if frames == 0 {
		t.Fatal("no decoded frame")
	}
}

func splitAnnexBForTest(src []byte, emit func([]byte) error) error {
	start := -1
	for i := 0; i+2 < len(src); i++ {
		if src[i] != 0 || src[i+1] != 0 || src[i+2] != 1 {
			continue
		}
		at := i
		if i > 0 && src[i-1] == 0 {
			at = i - 1
		}
		if start >= 0 && at-start >= 4 {
			if err := emit(src[start:at]); err != nil {
				return err
			}
		}
		start = at
		i += 2
	}
	if start >= 0 && start < len(src) {
		return emit(src[start:])
	}
	return nil
}

func encodeSolidI420(libPath string, w, h int, yVal, uVal, vVal byte) ([]byte, error) {
	hnd, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	var create func(**isvcEncoder) int
	var destroy func(*isvcEncoder)
	purego.RegisterLibFunc(&create, hnd, "WelsCreateSVCEncoder")
	purego.RegisterLibFunc(&destroy, hnd, "WelsDestroySVCEncoder")
	var enc *isvcEncoder
	if ret := create(&enc); ret != 0 || enc == nil {
		return nil, errCreateEncoder
	}
	defer destroy(enc)

	param := sEncParamBase{
		IUsageType:     0,
		IPicWidth:      int32(w),
		IPicHeight:     int32(h),
		ITargetBitrate: 500000,
		IRCMode:        0,
		FMaxFrameRate:  30,
	}
	if enc.initialize(&param) != 0 {
		return nil, errInitEncoder
	}
	defer enc.uninitialize()
	enc.forceIntra()

	y := bytes.Repeat([]byte{yVal}, w*h)
	u := bytes.Repeat([]byte{uVal}, w*h/4)
	v := bytes.Repeat([]byte{vVal}, w*h/4)
	pic := sSourcePicture{
		IColorFormat: videoFormatI420,
		IStride:      [4]int32{int32(w), int32(w / 2), int32(w / 2)},
		IPicWidth:    int32(w),
		IPicHeight:   int32(h),
	}
	pic.PData[0] = &y[0]
	pic.PData[1] = &u[0]
	pic.PData[2] = &v[0]
	var out []byte
	for i := 0; i < 3; i++ {
		var info sFrameBSInfo
		if enc.encodeFrame(&pic, &info) != 0 {
			return nil, errEncodeFrame
		}
		out = append(out, collectAnnexB(&info)...)
	}
	if len(out) == 0 {
		return nil, errEncodeFrame
	}
	return out, nil
}

type errString string

func (e errString) Error() string { return string(e) }

const (
	errCreateEncoder errString = "openh264: WelsCreateSVCEncoder"
	errInitEncoder   errString = "openh264: encoder Initialize"
	errEncodeFrame   errString = "openh264: EncodeFrame"
)

type isvcEncoderVtbl struct {
	Initialize          *[0]byte
	InitializeExt       *[0]byte
	GetDefaultParams    *[0]byte
	Uninitialize        *[0]byte
	EncodeFrame         *[0]byte
	EncodeParameterSets *[0]byte
	ForceIntraFrame     *[0]byte
	SetOption           *[0]byte
	GetOption           *[0]byte
}

type isvcEncoder struct {
	vtbl *isvcEncoderVtbl
}

type sEncParamBase struct {
	IUsageType     uint32
	IPicWidth      int32
	IPicHeight     int32
	ITargetBitrate int32
	IRCMode        int32
	FMaxFrameRate  float32
}

type sSourcePicture struct {
	IColorFormat int32
	IStride      [4]int32
	PData        [4]*uint8
	IPicWidth    int32
	IPicHeight   int32
	UiTimeStamp  int64
}

type sLayerBSInfo struct {
	UiTemporalId     uint8
	UiSpatialId      uint8
	UiQualityId      uint8
	EFrameType       uint32
	UiLayerType      uint8
	ISubSeqId        int32
	INalCount        int32
	PNalLengthInByte *int32
	PBsBuf           *uint8
}

type sFrameBSInfo struct {
	ILayerNum         int32
	SLayerInfo        [128]sLayerBSInfo
	EFrameType        uint32
	IFrameSizeInBytes int32
	UiTimeStamp       int64
}

func (v *isvcEncoder) initialize(p *sEncParamBase) int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.Initialize)),
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(p)),
	)
	return int(r1)
}

func (v *isvcEncoder) uninitialize() int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.Uninitialize)),
		uintptr(unsafe.Pointer(v)),
	)
	return int(r1)
}

func (v *isvcEncoder) forceIntra() {
	purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.ForceIntraFrame)),
		uintptr(unsafe.Pointer(v)),
		1,
		^uintptr(0),
	)
}

func (v *isvcEncoder) encodeFrame(pic *sSourcePicture, info *sFrameBSInfo) int {
	r1, _, _ := purego.SyscallN(
		uintptr(unsafe.Pointer(v.vtbl.EncodeFrame)),
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(pic)),
		uintptr(unsafe.Pointer(info)),
	)
	return int(r1)
}

func collectAnnexB(info *sFrameBSInfo) []byte {
	var out []byte
	for i := int32(0); i < info.ILayerNum; i++ {
		layer := &info.SLayerInfo[i]
		if layer.INalCount <= 0 || layer.PNalLengthInByte == nil || layer.PBsBuf == nil {
			continue
		}
		lens := unsafe.Slice(layer.PNalLengthInByte, layer.INalCount)
		total := 0
		for _, n := range lens {
			total += int(n)
		}
		if total <= 0 {
			continue
		}
		out = append(out, unsafe.Slice(layer.PBsBuf, total)...)
	}
	return out
}
