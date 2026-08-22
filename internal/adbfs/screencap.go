package adbfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
)

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

func (c *live) execScreencap(ctx context.Context, serial string) ([]byte, error) {
	transport, err := c.device(serial).TransportContext(ctx)
	if err != nil {
		return nil, err
	}
	defer transport.Close()
	status, err := transport.SendCommand("exec:screencap -p")
	if err != nil {
		return nil, err
	}
	if status != "OKAY" {
		return nil, fmt.Errorf("exec screencap: %s", status)
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
