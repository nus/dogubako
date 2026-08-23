package openh264

import (
	"fmt"
	"image"
	"unsafe"
)

func (info *sBufferInfo) sys() *sSysMEMBuffer {
	return (*sSysMEMBuffer)(unsafe.Pointer(&info.UsrData))
}

func i420ToNRGBA(yPlane, uPlane, vPlane *byte, w, h, yStride, cStride int) (*image.NRGBA, error) {
	if w <= 0 || h <= 0 || w > maxFrameEdge || h > maxFrameEdge {
		return nil, fmt.Errorf("openh264: bad frame size %dx%d", w, h)
	}
	if yStride < w || cStride < (w+1)/2 {
		return nil, fmt.Errorf("openh264: bad stride y=%d c=%d w=%d", yStride, cStride, w)
	}
	if yPlane == nil || uPlane == nil || vPlane == nil {
		return nil, fmt.Errorf("openh264: missing YUV plane")
	}
	y := unsafe.Slice(yPlane, yStride*h)
	u := unsafe.Slice(uPlane, cStride*((h+1)/2))
	v := unsafe.Slice(vPlane, cStride*((h+1)/2))
	return packI420(y, u, v, w, h, yStride, cStride), nil
}

func packI420(y, u, v []byte, w, h, yStride, cStride int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for row := 0; row < h; row++ {
		ys := row * yStride
		cs := (row / 2) * cStride
		di := row * dst.Stride
		for col := 0; col < w; col++ {
			yi := int(y[ys+col])
			ui := int(u[cs+col/2])
			vi := int(v[cs+col/2])
			// BT.601 limited range.
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
