package mtpfs

import (
	"context"
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

// Client talks to MTP devices over USB bulk transfers.
type Client interface {
	Devices(ctx context.Context) ([]Device, error)
	Stat(ctx context.Context, serial, path string) (Entry, error)
	List(ctx context.Context, serial, path string) ([]Entry, error)
	PullFile(ctx context.Context, serial, remote, local string) error
	PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error
	MkdirAll(ctx context.Context, serial, path string) error
}
