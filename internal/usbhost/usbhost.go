package usbhost

import (
	"errors"
	"io"
	"time"
)

// ErrUnsupported is returned when USB host access is not available on this OS.
var ErrUnsupported = errors.New("MTP over USB is only available on macOS")

// ErrNotFound means no matching MTP interface was found.
var ErrNotFound = errors.New("no MTP USB interface found")

// ErrBusy means another process owns the USB interface.
var ErrBusy = errors.New("USB interface is in use; close Image Capture or Android File Transfer and try again")

// Info is a connected MTP USB interface.
type Info struct {
	ID           string
	VendorID     int
	ProductID    int
	LocationID   uint32
	Serial       string
	Product      string
	Manufacturer string
	Interface    int
}

// Label is a human-readable name for the UI.
func (i Info) Label() string {
	name := i.Product
	if name == "" {
		name = i.ID
	}
	if i.Serial != "" && i.Serial != name {
		return name + "  " + i.Serial
	}
	return name
}

// Conn is an opened MTP bulk pipe pair.
type Conn interface {
	Info() Info
	MaxPacketOut() int
	Write(p []byte, timeout time.Duration) error
	Read(max int, timeout time.Duration) ([]byte, error)
	WriteStream(header []byte, r io.Reader, size int64, timeout time.Duration) error
	WriteStreamProgress(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error
	Close() error
}

// List returns MTP USB interfaces currently attached.
func List() ([]Info, error) {
	return list()
}

// Open seizes the MTP interface identified by Info.ID and opens bulk pipes.
func Open(id string) (Conn, error) {
	return open(id)
}
