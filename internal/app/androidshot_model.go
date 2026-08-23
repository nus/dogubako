package app

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/userdir"
)

const adbScreencapTimeout = 30 * time.Second

type androidShotResult struct {
	path      string
	cancelled bool
	err       error
}

// AndroidShotModel captures the screen of a connected Android device over ADB.
type AndroidShotModel struct {
	ScreenshotModel

	client adbfs.Client

	devices []adbfs.Device
	serial  string
	loaded  bool

	pendingDevices <-chan devicesResult
	pendingCapture <-chan androidShotResult
	captureCancel  context.CancelFunc
}

func (m *AndroidShotModel) SetClient(c adbfs.Client) {
	m.client = c
}

func (m *AndroidShotModel) Client() adbfs.Client {
	if m.client == nil {
		m.client = adbfs.Default()
	}
	return m.client
}

func (m *AndroidShotModel) Devices() []adbfs.Device { return m.devices }
func (m *AndroidShotModel) Serial() string          { return m.serial }

func (m *AndroidShotModel) Busy() bool {
	return m.pendingDevices != nil || m.pendingCapture != nil || m.Capturing()
}

func (m *AndroidShotModel) Online() bool {
	d := m.device(m.serial)
	return d.Serial != "" && d.Online()
}

func (m *AndroidShotModel) device(serial string) adbfs.Device {
	for _, d := range m.devices {
		if d.Serial == serial {
			return d
		}
	}
	return adbfs.Device{}
}

func (m *AndroidShotModel) EnsureLoaded() {
	m.ensureDest()
	if !m.delayInit {
		m.SetDelaySec(0)
	}
	if m.loaded || m.Busy() {
		return
	}
	m.RefreshDevices()
}

func (m *AndroidShotModel) ensureDest() {
	if m.destDir != "" || m.destErr != nil {
		return
	}
	m.prefix = "Android"
	dir, err := userdir.EnsureAndroidScreenshots()
	if err != nil {
		m.destErr = err
		return
	}
	m.destDir = dir
}

func (m *AndroidShotModel) Drain() {
	m.drainDevices()
	m.drainCapture()
}

func (m *AndroidShotModel) drainDevices() {
	if m.pendingDevices == nil {
		return
	}
	select {
	case res := <-m.pendingDevices:
		m.pendingDevices = nil
		m.applyDevices(res.devices, res.err)
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidShotModel) drainCapture() {
	if m.pendingCapture == nil {
		return
	}
	select {
	case res := <-m.pendingCapture:
		m.pendingCapture = nil
		m.captureCancel = nil
		m.SetCapturing(false)
		if res.cancelled || errors.Is(res.err, context.Canceled) {
			if res.path != "" {
				_ = os.Remove(res.path)
			}
			m.SetStatus(i18n.StatusCaptureCancelled)
			guigui.RequestRebuild()
			return
		}
		if res.err != nil {
			if res.path != "" {
				_ = os.Remove(res.path)
			}
			if errors.Is(res.err, context.DeadlineExceeded) {
				m.SetStatus(i18n.StatusCaptureTimeout)
			} else {
				m.SetStatus(i18n.StatusAdbCaptureFailed, res.err)
			}
			guigui.RequestRebuild()
			return
		}
		_ = m.ApplyCaptureFile(res.path)
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidShotModel) applyDevices(devs []adbfs.Device, err error) {
	m.devices = devs
	m.generation++
	if err != nil {
		m.serial = ""
		m.SetStatus(i18n.StatusAdbConnectFailed, err)
		return
	}
	if len(devs) == 0 {
		m.serial = ""
		m.SetStatus(i18n.StatusAdbNoDevices)
		return
	}
	if m.device(m.serial).Serial == "" {
		m.serial = firstOnlineSerial(devs)
		if m.serial == "" {
			m.serial = devs[0].Serial
		}
	}
	d := m.device(m.serial)
	if !d.Online() {
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	m.SetStatus(i18n.StatusAdbDeviceReady, d.Label())
}

func (m *AndroidShotModel) RefreshDevices() {
	if m.Busy() {
		return
	}
	m.loaded = true
	m.SetStatus(i18n.StatusAdbListing)
	ch := make(chan devicesResult, 1)
	m.pendingDevices = ch
	m.generation++
	client := m.Client()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adbListTimeout)
		defer cancel()
		devs, err := client.Devices(ctx)
		ch <- devicesResult{devices: devs, err: err}
	}()
}

func (m *AndroidShotModel) SelectDevice(serial string) {
	if m.Busy() || serial == "" || serial == m.serial {
		return
	}
	m.serial = serial
	m.generation++
	d := m.device(serial)
	if !d.Online() {
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	m.SetStatus(i18n.StatusAdbDeviceReady, d.Label())
}

func (m *AndroidShotModel) StartCapture() {
	if m.pendingDevices != nil {
		return
	}
	if !m.Online() {
		m.SetStatus(i18n.StatusAdbSelectOnline)
		return
	}
	m.cancelCapture()
	m.SetCapturing(true)
	m.SetStatus(i18n.StatusAdbCapturing)
	delay := time.Duration(m.DelaySec()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), delay+adbScreencapTimeout)
	m.captureCancel = cancel
	ch := make(chan androidShotResult, 1)
	m.pendingCapture = ch
	m.generation++
	client := m.Client()
	serial := m.serial
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				ch <- androidShotResult{cancelled: true, err: ctx.Err()}
				return
			case <-timer.C:
			}
		}
		data, err := client.Screencap(ctx, serial)
		if err != nil {
			ch <- androidShotResult{cancelled: errors.Is(err, context.Canceled), err: err}
			return
		}
		if err := ctx.Err(); err != nil {
			ch <- androidShotResult{cancelled: true, err: err}
			return
		}
		f, err := os.CreateTemp("", "dogubako-android-*.png")
		if err != nil {
			ch <- androidShotResult{err: err}
			return
		}
		path := f.Name()
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			_ = os.Remove(path)
			ch <- androidShotResult{err: writeErr}
			return
		}
		if closeErr != nil {
			_ = os.Remove(path)
			ch <- androidShotResult{err: closeErr}
			return
		}
		ch <- androidShotResult{path: path}
	}()
}

func (m *AndroidShotModel) cancelCapture() {
	if m.captureCancel != nil {
		m.captureCancel()
		m.captureCancel = nil
	}
	if m.pendingCapture == nil {
		return
	}
	stale := m.pendingCapture
	m.pendingCapture = nil
	go func() {
		res := <-stale
		if res.path != "" {
			_ = os.Remove(res.path)
		}
	}()
}
