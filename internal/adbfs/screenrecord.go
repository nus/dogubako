package adbfs

import (
	"context"
	"fmt"
	"io"
)

const screenrecordH264 = "screenrecord --output-format=h264 --time-limit=180 -"

// ScreenrecordH264 starts a raw H.264 Annex-B stream of the device display
// via exec:screenrecord. The caller must Close the reader to stop recording.
func (c *live) ScreenrecordH264(ctx context.Context, serial string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if serial == "" {
		return nil, fmt.Errorf("no device")
	}
	transport, err := c.device(serial).TransportContext(ctx)
	if err != nil {
		return nil, err
	}
	status, err := transport.SendCommand("exec:" + screenrecordH264)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	if status != "OKAY" {
		_ = transport.Close()
		return nil, fmt.Errorf("exec screenrecord: %s", status)
	}
	return newCtxReadCloser(ctx, transport), nil
}

type ctxReadCloser struct {
	ctx    context.Context
	c      io.ReadCloser
	cancel context.CancelFunc
}

func newCtxReadCloser(ctx context.Context, c io.ReadCloser) io.ReadCloser {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()
	return &ctxReadCloser{ctx: ctx, c: c, cancel: cancel}
}

func (r *ctxReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.c.Read(p)
	if r.ctx.Err() != nil {
		return n, r.ctx.Err()
	}
	return n, err
}

func (r *ctxReadCloser) Close() error {
	r.cancel()
	return r.c.Close()
}
