package app

import (
	"context"
	"errors"
	"image"
	"os"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/userdir"
)

const (
	adbScreencapTimeout = 30 * time.Second
	adbLiveTimeout      = 15 * time.Second
	adbLiveMinInterval  = 80 * time.Millisecond
	adbLiveFailLimit    = 3
)

type androidShotResult struct {
	path      string
	cancelled bool
	err       error
}

type androidLiveResult struct {
	img image.Image
	err error
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

	live        bool
	liveStopped bool
	liveResume  bool
	liveFails   int
	pendingLive <-chan androidLiveResult
	liveCancel  context.CancelFunc
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

func (m *AndroidShotModel) Live() bool { return m.live }

func (m *AndroidShotModel) Drain() {
	m.drainDevices()
	m.drainCapture()
	m.drainLive()
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
			m.resumeLiveAfterCapture()
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
			m.resumeLiveAfterCapture()
			guigui.RequestRebuild()
			return
		}
		_ = m.ApplyCaptureFile(res.path)
		m.resumeLiveAfterCapture()
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidShotModel) drainLive() {
	if m.pendingLive == nil {
		return
	}
	select {
	case res := <-m.pendingLive:
		if !m.live {
			return
		}
		if res.err != nil {
			if errors.Is(res.err, context.Canceled) {
				return
			}
			m.liveFails++
			if m.liveFails >= adbLiveFailLimit {
				m.StopLive()
				m.liveStopped = true
				m.SetStatus(i18n.StatusAdbLiveFailed, res.err)
				guigui.RequestRebuild()
			}
			return
		}
		m.liveFails = 0
		m.applyLiveFrame(res.img)
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidShotModel) applyLiveFrame(img image.Image) {
	if img == nil {
		return
	}
	m.image = img
	m.preview = previewImage(img)
	m.imageSize = img.Bounds().Size()
	m.sourcePath = ""
	m.generation++
	switch m.status.key {
	case i18n.StatusSaved, i18n.StatusClipboardCopied:
	default:
		m.SetStatus(i18n.StatusAdbLive, m.imageSize.X, m.imageSize.Y)
	}
}

func (m *AndroidShotModel) applyDevices(devs []adbfs.Device, err error) {
	wasLive := m.live && !m.liveStopped
	prevSerial := m.serial
	m.devices = devs
	m.generation++
	if err != nil {
		m.serial = ""
		m.StopLive()
		m.SetStatus(i18n.StatusAdbConnectFailed, err)
		return
	}
	if len(devs) == 0 {
		m.serial = ""
		m.StopLive()
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
		m.StopLive()
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	if wasLive && (prevSerial != m.serial || !m.live) {
		m.StopLive()
		m.StartLive()
		return
	}
	if !m.live {
		m.SetStatus(i18n.StatusAdbDeviceReady, d.Label())
	}
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
	if m.pendingDevices != nil || m.pendingCapture != nil || serial == "" || serial == m.serial {
		return
	}
	resume := m.live && !m.liveStopped
	m.StopLive()
	m.serial = serial
	m.generation++
	d := m.device(serial)
	if !d.Online() {
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	m.SetStatus(i18n.StatusAdbDeviceReady, d.Label())
	if resume {
		m.StartLive()
	}
}

func (m *AndroidShotModel) StartCapture() {
	if m.pendingDevices != nil {
		return
	}
	if m.live && m.image != nil {
		_ = m.SaveDefault()
		return
	}
	if !m.Online() {
		m.SetStatus(i18n.StatusAdbSelectOnline)
		return
	}
	m.liveResume = m.live && !m.liveStopped
	m.StopLive()
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

func (m *AndroidShotModel) resumeLiveAfterCapture() {
	if !m.liveResume || m.liveStopped || !m.Online() {
		m.liveResume = false
		return
	}
	m.liveResume = false
	m.StartLive()
}

func (m *AndroidShotModel) SetLive(on bool) {
	if on {
		m.liveStopped = false
		m.StartLive()
		return
	}
	m.liveStopped = true
	m.StopLive()
	if m.HasImage() {
		m.SetStatus(i18n.StatusAdbLiveStopped)
		return
	}
	if m.Online() {
		d := m.device(m.serial)
		m.SetStatus(i18n.StatusAdbDeviceReady, d.Label())
	}
}

func (m *AndroidShotModel) EnsureLive() {
	if m.live || m.liveStopped || m.pendingCapture != nil || m.pendingDevices != nil || !m.Online() {
		return
	}
	m.StartLive()
}

func (m *AndroidShotModel) StartLive() {
	if m.live || m.pendingCapture != nil {
		return
	}
	if !m.Online() {
		m.SetStatus(i18n.StatusAdbSelectOnline)
		return
	}
	m.liveStopped = false
	m.liveFails = 0
	ctx, cancel := context.WithCancel(context.Background())
	m.liveCancel = cancel
	ch := make(chan androidLiveResult, 1)
	m.pendingLive = ch
	m.live = true
	m.generation++
	if !m.HasImage() {
		m.SetStatus(i18n.StatusAdbLiveStarting)
	}
	client := m.Client()
	serial := m.serial
	go liveLoop(ctx, client, serial, ch)
}

func liveLoop(ctx context.Context, client adbfs.Client, serial string, ch chan androidLiveResult) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		started := time.Now()
		frameCtx, cancel := context.WithTimeout(ctx, adbLiveTimeout)
		img, err := client.ScreencapImage(frameCtx, serial)
		cancel()
		res := androidLiveResult{img: img, err: err}
		if ctx.Err() != nil {
			return
		}
		sendLiveResult(ctx, ch, res)
		if wait := adbLiveMinInterval - time.Since(started); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func sendLiveResult(ctx context.Context, ch chan androidLiveResult, res androidLiveResult) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case <-ctx.Done():
	case ch <- res:
	}
}

func (m *AndroidShotModel) StopLive() {
	if m.liveCancel != nil {
		m.liveCancel()
		m.liveCancel = nil
	}
	if !m.live && m.pendingLive == nil {
		return
	}
	m.live = false
	m.pendingLive = nil
	m.generation++
}

func (m *AndroidShotModel) LoadPath(path string) error {
	if m.live {
		m.liveStopped = true
		m.StopLive()
	}
	return m.ScreenshotModel.LoadPath(path)
}
