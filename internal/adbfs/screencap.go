package adbfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net"
	"os"
)

// Android PIXEL_FORMAT_* values written by screencap.
const (
	pixelFormatRGBA8888 = 1
	pixelFormatRGBX8888 = 2
	pixelFormatRGB888   = 3
	pixelFormatRGB565   = 4
	pixelFormatBGRA8888 = 5
)

const maxFramebufferEdge = 16384

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

var screencapRemotes = []string{
	"/data/local/tmp/dogubako-screencap.png",
	"/sdcard/dogubako-screencap.png",
}

// Screencap captures the device display as PNG. It prefers the binary-safe
// exec protocol, then shell stdout (with the old CRLF workaround), then
// writing a temp file on the device and pulling it.
func (c *live) Screencap(ctx context.Context, serial string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serial == "" {
		return nil, fmt.Errorf("no device")
	}
	var lastErr error
	if data, err := c.execScreencap(ctx, serial); err == nil {
		if png := usablePNG(data); len(png) > 0 {
			return png, nil
		}
		lastErr = fmt.Errorf("exec screencap produced no PNG")
	} else {
		lastErr = err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err := c.device(serial).RunCommandContext(ctx, "screencap -p")
	if err == nil {
		if png := usablePNG([]byte(out)); len(png) > 0 {
			return png, nil
		}
		lastErr = fmt.Errorf("shell screencap produced no PNG")
	} else {
		lastErr = err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data, err := c.screencapPull(ctx, serial); err == nil {
		return data, nil
	} else {
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("screencap failed")
	}
	return nil, lastErr
}

func (c *live) ScreencapImage(ctx context.Context, serial string) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serial == "" {
		return nil, fmt.Errorf("no device")
	}
	var lastErr error
	if data, err := c.execCommand(ctx, serial, "screencap"); err == nil {
		if img, err := decodeScreencapBytes(data); err == nil {
			return img, nil
		} else {
			lastErr = err
		}
	} else {
		lastErr = err
	}
	data, err := c.Screencap(ctx, serial)
	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil && lastErr != nil {
		return nil, lastErr
	}
	return img, err
}

func (c *live) execScreencap(ctx context.Context, serial string) ([]byte, error) {
	return c.execCommand(ctx, serial, "screencap -p")
}

func (c *live) execCommand(ctx context.Context, serial, cmd string) ([]byte, error) {
	transport, err := c.device(serial).TransportContext(ctx)
	if err != nil {
		return nil, err
	}
	defer transport.Close()
	status, err := transport.SendCommand("exec:" + cmd)
	if err != nil {
		return nil, err
	}
	if status != "OKAY" {
		return nil, fmt.Errorf("exec %s: %s", cmd, status)
	}
	return readConn(ctx, transport)
}

func (c *live) screencapPull(ctx context.Context, serial string) ([]byte, error) {
	dev := c.device(serial)
	var lastErr error
	for _, remote := range screencapRemotes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := dev.RunCommandContext(ctx, "screencap -p "+shellQuote(remote)); err != nil {
			lastErr = err
			continue
		}
		f, err := os.CreateTemp("", "dogubako-adbcap-*.png")
		if err != nil {
			return nil, err
		}
		local := f.Name()
		_ = f.Close()
		pullErr := c.PullFile(ctx, serial, remote, local)
		_, _ = dev.RunCommandContext(ctx, "rm -f "+shellQuote(remote))
		if pullErr != nil {
			_ = os.Remove(local)
			lastErr = pullErr
			continue
		}
		data, err := os.ReadFile(local)
		_ = os.Remove(local)
		if err != nil {
			lastErr = err
			continue
		}
		if png := usablePNG(data); len(png) > 0 {
			return png, nil
		}
		lastErr = fmt.Errorf("screencap file is not a PNG")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("screencap file pull failed")
	}
	return nil, lastErr
}

func readConn(ctx context.Context, conn net.Conn) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(conn)
		ch <- result{data, err}
	}()
	select {
	case <-ctx.Done():
		_ = conn.Close()
		res := <-ch
		if res.err == nil {
			res.err = ctx.Err()
		}
		return res.data, res.err
	case res := <-ch:
		return res.data, res.err
	}
}

func decodeScreencapBytes(data []byte) (image.Image, error) {
	if img, err := decodeFramebuffer(data); err == nil {
		return img, nil
	} else if png := usablePNG(data); len(png) > 0 {
		img, _, decErr := image.Decode(bytes.NewReader(png))
		if decErr == nil {
			return img, nil
		}
		return nil, err
	} else {
		return nil, err
	}
}

func decodeFramebuffer(data []byte) (*image.NRGBA, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("short framebuffer")
	}
	w := int(binary.LittleEndian.Uint32(data[0:4]))
	h := int(binary.LittleEndian.Uint32(data[4:8]))
	format := binary.LittleEndian.Uint32(data[8:12])
	if w <= 0 || h <= 0 || w > maxFramebufferEdge || h > maxFramebufferEdge {
		return nil, fmt.Errorf("bad framebuffer size %dx%d", w, h)
	}
	bpp := bytesPerPixel(format)
	if bpp == 0 {
		return nil, fmt.Errorf("unsupported pixel format %d", format)
	}
	need := w * h * bpp
	header, err := framebufferHeaderLen(len(data), need)
	if err != nil {
		return nil, err
	}
	return packFramebuffer(data[header:header+need], w, h, format)
}

func framebufferHeaderLen(total, need int) (int, error) {
	switch {
	case total == 12+need:
		return 12, nil
	case total == 16+need:
		return 16, nil
	case total > 16+need:
		return 16, nil
	case total > 12+need:
		return 12, nil
	default:
		return 0, fmt.Errorf("framebuffer size %d want %d", total, need)
	}
}

func bytesPerPixel(format uint32) int {
	switch format {
	case pixelFormatRGBA8888, pixelFormatRGBX8888, pixelFormatBGRA8888:
		return 4
	case pixelFormatRGB888:
		return 3
	case pixelFormatRGB565:
		return 2
	default:
		return 0
	}
}

func packFramebuffer(pix []byte, w, h int, format uint32) (*image.NRGBA, error) {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	switch format {
	case pixelFormatRGBA8888, pixelFormatRGBX8888:
		if format == pixelFormatRGBA8888 {
			copy(dst.Pix, pix)
			return dst, nil
		}
		for i := 0; i+3 < len(pix) && i+3 < len(dst.Pix); i += 4 {
			dst.Pix[i] = pix[i]
			dst.Pix[i+1] = pix[i+1]
			dst.Pix[i+2] = pix[i+2]
			dst.Pix[i+3] = 255
		}
		return dst, nil
	case pixelFormatBGRA8888:
		for i := 0; i+3 < len(pix) && i+3 < len(dst.Pix); i += 4 {
			dst.Pix[i] = pix[i+2]
			dst.Pix[i+1] = pix[i+1]
			dst.Pix[i+2] = pix[i]
			dst.Pix[i+3] = pix[i+3]
		}
		return dst, nil
	case pixelFormatRGB888:
		di := 0
		for i := 0; i+2 < len(pix) && di+3 < len(dst.Pix); i += 3 {
			dst.Pix[di] = pix[i]
			dst.Pix[di+1] = pix[i+1]
			dst.Pix[di+2] = pix[i+2]
			dst.Pix[di+3] = 255
			di += 4
		}
		return dst, nil
	case pixelFormatRGB565:
		di := 0
		for i := 0; i+1 < len(pix) && di+3 < len(dst.Pix); i += 2 {
			v := binary.LittleEndian.Uint16(pix[i:])
			dst.Pix[di] = uint8(((v >> 11) & 0x1f) * 255 / 31)
			dst.Pix[di+1] = uint8(((v >> 5) & 0x3f) * 255 / 63)
			dst.Pix[di+2] = uint8((v & 0x1f) * 255 / 31)
			dst.Pix[di+3] = 255
			di += 4
		}
		return dst, nil
	default:
		return nil, fmt.Errorf("unsupported pixel format %d", format)
	}
}

func usablePNG(data []byte) []byte {
	if p := pngPayload(data); p != nil {
		return p
	}
	return pngPayload(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
}

func pngPayload(data []byte) []byte {
	i := bytes.Index(data, pngMagic)
	if i < 0 {
		return nil
	}
	data = data[i:]
	iend := []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D'}
	j := bytes.LastIndex(data, iend)
	if j >= 0 && j+12 <= len(data) {
		return data[:j+12]
	}
	if len(data) < len(pngMagic) {
		return nil
	}
	return data
}
