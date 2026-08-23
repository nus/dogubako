package adbfs

import (
	"context"
	"os"
	"time"
)

// Device is a connected Android device (or emulator).
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
		name = d.Serial
	}
	if d.State != "" && d.State != "device" {
		return name + " (" + d.State + ")"
	}
	if d.Model != "" && d.Serial != "" && d.Model != d.Serial {
		return name + "  " + d.Serial
	}
	return name
}

// Online reports whether the device is ready for file operations.
func (d Device) Online() bool {
	return d.State == "device"
}

// Entry is a file or directory on a device.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	CrtTime time.Time // zero when the filesystem does not expose birth time
}

// Client talks to Android devices over the ADB protocol (not the adb CLI).
type Client interface {
	Devices(ctx context.Context) ([]Device, error)
	Stat(ctx context.Context, serial, path string) (Entry, error)
	List(ctx context.Context, serial, path string) ([]Entry, error)
	PullFile(ctx context.Context, serial, remote, local string) error
	PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error
	MkdirAll(ctx context.Context, serial, path string) error
	// Screencap returns a PNG of the device display (screencap -p).
	Screencap(ctx context.Context, serial string) ([]byte, error)
}

// DefaultRoots are tried in order when a device is first opened.
var DefaultRoots = []string{"/sdcard", "/storage/emulated/0", "/"}
