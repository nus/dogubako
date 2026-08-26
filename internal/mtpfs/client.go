package mtpfs

import (
	"context"
	"io"
	"os"
	"time"
)

// Device is a connected MTP responder.
type Device struct {
	Serial  string
	State   string
	Model   string
	Product string
}

// Label is a human-readable device name for the UI.
func (d Device) Label() string {
	name := d.Model
	if name == "" {
		name = d.Product
	}
	if name == "" {
		name = d.Serial
	}
	if d.State != "" && d.State != "online" {
		return name + " (" + d.State + ")"
	}
	if name != d.Serial && d.Serial != "" {
		return name + "  " + d.Serial
	}
	return name
}

// Online reports whether the device is ready for file operations.
func (d Device) Online() bool {
	return d.State == "online" || d.State == ""
}

// Entry is a file or directory on a device.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// ListProgressFunc is called while listing a directory so the UI can show
// files as they arrive. entries is a snapshot of the list so far.
type ListProgressFunc func(loaded, total int, entries []Entry)

type listProgressKey struct{}

// WithListProgress attaches a listing progress callback to ctx.
func WithListProgress(ctx context.Context, fn ListProgressFunc) context.Context {
	if fn == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, listProgressKey{}, fn)
}

func reportListProgress(ctx context.Context, loaded, total int, entries []Entry) {
	fn, _ := ctx.Value(listProgressKey{}).(ListProgressFunc)
	if fn == nil {
		return
	}
	fn(loaded, total, entries)
}

// CopyProgressFunc is called while copying so the UI can show progress.
// copied/total are completed files; copiedBytes/totalBytes are payload bytes.
type CopyProgressFunc func(copied, total int, copiedBytes, totalBytes int64)

type copyProgressKey struct{}

type copyProgressState struct {
	fn          CopyProgressFunc
	copied      int
	total       int
	copiedBytes int64
	totalBytes  int64
	lastPct     int
}

// WithCopyProgress attaches a copy progress callback to ctx.
func WithCopyProgress(ctx context.Context, fn CopyProgressFunc) context.Context {
	if fn == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, copyProgressKey{}, &copyProgressState{fn: fn, lastPct: -1})
}

func copyProgressFrom(ctx context.Context) *copyProgressState {
	st, _ := ctx.Value(copyProgressKey{}).(*copyProgressState)
	return st
}

func setCopyTotals(ctx context.Context, files int, bytes int64) {
	st := copyProgressFrom(ctx)
	if st == nil {
		return
	}
	st.total = files
	st.totalBytes = bytes
	st.emit(true)
}

func addCopyBytes(ctx context.Context, n int64) {
	st := copyProgressFrom(ctx)
	if st == nil || n <= 0 {
		return
	}
	st.copiedBytes += n
	if st.totalBytes > 0 && st.copiedBytes > st.totalBytes {
		st.copiedBytes = st.totalBytes
	}
	st.emit(false)
}

func addCopyFile(ctx context.Context) {
	st := copyProgressFrom(ctx)
	if st == nil {
		return
	}
	st.copied++
	st.emit(true)
}

func (st *copyProgressState) emit(force bool) {
	if st.fn == nil {
		return
	}
	pct := byteOrFilePercent(st.copied, st.total, st.copiedBytes, st.totalBytes)
	if !force && pct == st.lastPct {
		return
	}
	st.lastPct = pct
	st.fn(st.copied, st.total, st.copiedBytes, st.totalBytes)
}

func byteOrFilePercent(copied, total int, copiedBytes, totalBytes int64) int {
	if totalBytes > 0 {
		return listPercent64(copiedBytes, totalBytes)
	}
	return listPercent64(int64(copied), int64(total))
}

func listPercent64(loaded, total int64) int {
	if total <= 0 {
		return 0
	}
	if loaded >= total {
		return 100
	}
	if loaded <= 0 {
		return 0
	}
	return int(loaded * 100 / total)
}

type progressWriter struct {
	ctx context.Context
	w   io.Writer
}

func (p progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		addCopyBytes(p.ctx, int64(n))
	}
	return n, err
}

// Client talks to MTP devices over USB bulk transfers.
type Client interface {
	Devices(ctx context.Context) ([]Device, error)
	Stat(ctx context.Context, serial, path string) (Entry, error)
	List(ctx context.Context, serial, path string) ([]Entry, error)
	PullFile(ctx context.Context, serial, remote, local string) error
	PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error
	MkdirAll(ctx context.Context, serial, path string) error
}
