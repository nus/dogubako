package app

import (
	"context"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/adbfs"
)

// adbDevices is the device list shared by the Android file and screen panes.
type adbDevices struct {
	devices []adbfs.Device
	serial  string
	pending <-chan devicesResult
	fetched bool

	// follow is the pane whose status and reload should run when the fetch returns.
	follow AndroidFace
	// keepRoot re-lists the current directory instead of probing roots again.
	keepRoot bool

	files *AndroidModel
	shot  *AndroidShotModel
}

func (d *adbDevices) refresh(client adbfs.Client, follow AndroidFace, keepRoot bool) {
	if d == nil || d.pending != nil {
		return
	}
	d.follow = follow
	d.keepRoot = keepRoot
	ch := make(chan devicesResult, 1)
	d.pending = ch
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adbListTimeout)
		defer cancel()
		devs, err := client.Devices(ctx)
		ch <- devicesResult{devices: devs, err: err}
	}()
}

func (d *adbDevices) drain() {
	if d == nil || d.pending == nil {
		return
	}
	select {
	case res := <-d.pending:
		d.pending = nil
		d.fetched = true
		prev := d.assign(res.devices, res.err)
		primaryFiles := d.follow == AndroidFaceFiles
		if d.files != nil {
			d.files.onSharedDevices(res.err, prev, primaryFiles)
		}
		if d.shot != nil {
			d.shot.onSharedDevices(res.err, prev, !primaryFiles)
		}
		guigui.RequestRebuild()
	default:
	}
}

func (d *adbDevices) assign(devs []adbfs.Device, err error) (prev string) {
	prev = d.serial
	d.devices = devs
	if err != nil || len(devs) == 0 {
		d.serial = ""
		return prev
	}
	if !adbSerialIn(devs, d.serial) {
		d.serial = firstOnlineSerial(devs)
		if d.serial == "" {
			d.serial = devs[0].Serial
		}
	}
	return prev
}

func adbSerialIn(devs []adbfs.Device, serial string) bool {
	if serial == "" {
		return false
	}
	for _, d := range devs {
		if d.Serial == serial {
			return true
		}
	}
	return false
}
