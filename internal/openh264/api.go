package openh264

// C ABI types for OpenH264 2.6 decoder (64-bit). Layout follows Cisco's
// codec_api.h / codec_def.h as emitted by cgo -godefs on linux/amd64.

const (
	dsErrorFree       = 0x0
	dsFramePending    = 0x1
	dsBitstreamError  = 0x4
	dsNoParamSets     = 0x10
	dsInvalidArgument = 0x1000

	errorConSliceCopy = 0x2
	videoBitstreamAVC = 0x0
	cmResultSuccess   = 0x0

	videoFormatI420 = 0x17
	maxFrameEdge    = 16384
)

type isvcDecoderVtbl struct {
	Initialize         *[0]byte
	Uninitialize       *[0]byte
	DecodeFrame        *[0]byte
	DecodeFrameNoDelay *[0]byte
	DecodeFrame2       *[0]byte
	FlushFrame         *[0]byte
	DecodeParser       *[0]byte
	DecodeFrameEx      *[0]byte
	SetOption          *[0]byte
	GetOption          *[0]byte
}

type isvcDecoder struct {
	vtbl *isvcDecoderVtbl
}

type sVideoProperty struct {
	Size         uint32
	EVideoBsType uint32
}

type sDecodingParam struct {
	PFileNameRestructed *int8
	UiCpuLoad           uint32
	UiTargetDqLayer     uint8
	EEcActiveIdc        uint32
	BParseOnly          bool
	SVideoProperty      sVideoProperty
}

type sSysMEMBuffer struct {
	IWidth  int32
	IHeight int32
	IFormat int32
	IStride [2]int32
}

type sBufferInfo struct {
	IBufferStatus     int32
	UiInBsTimeStamp   uint64
	UiOutYuvTimeStamp uint64
	UsrData           [20]byte
	PDst              [3]*uint8
}
