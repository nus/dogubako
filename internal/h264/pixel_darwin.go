package h264

import (
	"fmt"
	"image"
	"unsafe"
)

func copyPixelBuffer(buf uintptr) (*image.NRGBA, error) {
	w := int(cvPixelBufferGetWidth(buf))
	h := int(cvPixelBufferGetHeight(buf))
	if w <= 0 || h <= 0 || w > maxFrameEdge || h > maxFrameEdge {
		return nil, fmt.Errorf("videotoolbox: bad frame size %dx%d", w, h)
	}
	if st := cvPixelBufferLockBaseAddress(buf, cvPixelBufferLockReadOnly); st != 0 {
		return nil, fmt.Errorf("videotoolbox: lock pixel buffer=%d", st)
	}
	defer cvPixelBufferUnlockBaseAddress(buf, cvPixelBufferLockReadOnly)

	format := cvPixelBufferGetPixelFormatType(buf)
	if cvPixelBufferIsPlanar(buf) || format == pixel420v || format == pixel420f {
		return copyNV12(buf, w, h)
	}
	base := cvPixelBufferGetBaseAddress(buf)
	if base == nil {
		return nil, fmt.Errorf("videotoolbox: missing pixel base")
	}
	stride := int(cvPixelBufferGetBytesPerRow(buf))
	if stride < w*4 {
		return nil, fmt.Errorf("videotoolbox: bad stride %d w=%d", stride, w)
	}
	src := unsafe.Slice((*byte)(base), stride*h)
	switch format {
	case pixel32BGRA:
		return packBGRA(src, w, h, stride, 2, 1, 0), nil
	case pixel32ARGB:
		return packBGRA(src, w, h, stride, 1, 2, 3), nil
	case pixel32RGBA:
		return packBGRA(src, w, h, stride, 0, 1, 2), nil
	default:
		return nil, fmt.Errorf("videotoolbox: pixel format 0x%x", format)
	}
}

func packBGRA(src []byte, w, h, stride, ri, gi, bi int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for row := 0; row < h; row++ {
		si := row * stride
		di := row * dst.Stride
		for col := 0; col < w; col++ {
			dst.Pix[di+0] = src[si+ri]
			dst.Pix[di+1] = src[si+gi]
			dst.Pix[di+2] = src[si+bi]
			dst.Pix[di+3] = 255
			si += 4
			di += 4
		}
	}
	return dst
}

func copyNV12(buf uintptr, w, h int) (*image.NRGBA, error) {
	yPtr := cvPixelBufferGetBaseAddressOfPlane(buf, 0)
	uvPtr := cvPixelBufferGetBaseAddressOfPlane(buf, 1)
	if yPtr == nil || uvPtr == nil {
		return nil, fmt.Errorf("videotoolbox: missing NV12 planes")
	}
	yStride := int(cvPixelBufferGetBytesPerRowOfPlane(buf, 0))
	uvStride := int(cvPixelBufferGetBytesPerRowOfPlane(buf, 1))
	if yStride < w || uvStride < w {
		return nil, fmt.Errorf("videotoolbox: bad NV12 stride y=%d uv=%d w=%d", yStride, uvStride, w)
	}
	y := unsafe.Slice((*byte)(yPtr), yStride*h)
	uv := unsafe.Slice((*byte)(uvPtr), uvStride*((h+1)/2))
	return packNV12(y, uv, w, h, yStride, uvStride), nil
}

func packNV12(y, uv []byte, w, h, yStride, uvStride int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for row := 0; row < h; row++ {
		ys := row * yStride
		us := (row / 2) * uvStride
		di := row * dst.Stride
		for col := 0; col < w; col++ {
			yi := int(y[ys+col])
			ui := int(uv[us+(col&^1)])
			vi := int(uv[us+(col&^1)+1])
			c := yi - 16
			d := ui - 128
			e := vi - 128
			r := (298*c + 409*e + 128) >> 8
			g := (298*c - 100*d - 208*e + 128) >> 8
			b := (298*c + 516*d + 128) >> 8
			dst.Pix[di] = clampByte(r)
			dst.Pix[di+1] = clampByte(g)
			dst.Pix[di+2] = clampByte(b)
			dst.Pix[di+3] = 255
			di += 4
		}
	}
	return dst
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
