package app

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/h264"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/rtsp"
)

// RTSPModel is the RTSP player: live video, protocol log, decode and net stats.
type RTSPModel struct {
	generation uint64
	url        string
	status     statusMsg

	player   *rtsp.Player
	snap     rtsp.Snapshot
	lastGen  uint64
	selected int
	follow   bool
}

func (m *RTSPModel) Generation() uint64 { return m.generation }

func (m *RTSPModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *RTSPModel) SetStatus(key i18n.Key, args ...any) {
	if m.status.key == key && fmt.Sprint(m.status.args...) == fmt.Sprint(args...) {
		return
	}
	m.status.key = key
	if len(args) == 0 {
		m.status.args = nil
	} else {
		m.status.args = append([]any(nil), args...)
	}
	m.generation++
}

func (m *RTSPModel) URL() string { return m.url }

func (m *RTSPModel) SetURL(url string) {
	if m.url == url {
		return
	}
	m.url = url
	m.generation++
}

func (m *RTSPModel) Playing() bool {
	return m.snap.Playing || (m.player != nil && m.player.Playing())
}

func (m *RTSPModel) HasImage() bool { return m.snap.Image != nil }

func (m *RTSPModel) Image() image.Image { return m.snap.Image }

func (m *RTSPModel) Preview() image.Image { return previewImage(m.snap.Image) }

func (m *RTSPModel) Size() image.Point {
	if m.snap.Dec.Width > 0 && m.snap.Dec.Height > 0 {
		return image.Pt(m.snap.Dec.Width, m.snap.Dec.Height)
	}
	if m.snap.Image == nil {
		return image.Point{}
	}
	return m.snap.Image.Bounds().Size()
}

func (m *RTSPModel) Logs() []rtsp.LogEntry { return m.snap.Logs }

func (m *RTSPModel) SelectedLog() int { return m.selected }

func (m *RTSPModel) SelectLog(i int) {
	logs := m.snap.Logs
	if i < 0 || i >= len(logs) {
		return
	}
	m.selected = i
	m.follow = i == len(logs)-1
	m.generation++
}

func (m *RTSPModel) SelectedLogText() string {
	logs := m.snap.Logs
	if m.selected < 0 || m.selected >= len(logs) {
		return ""
	}
	return logs[m.selected].Text
}

func (m *RTSPModel) Net() rtsp.NetworkStats { return m.snap.Net }
func (m *RTSPModel) Dec() rtsp.DecodeStats  { return m.snap.Dec }

func (m *RTSPModel) DownloadPercent() int {
	if m.snap.Download < 0 {
		return -1
	}
	return m.snap.Download
}

func (m *RTSPModel) DecodeStatsText(lang i18n.Lang) string {
	d := m.snap.Dec
	size := "—"
	if d.Width > 0 && d.Height > 0 {
		size = fmt.Sprintf("%d×%d", d.Width, d.Height)
	}
	codec := d.Codec.String()
	if codec == "" || codec == "—" {
		codec = "—"
	}
	return i18n.T(lang, i18n.RTSPDecodeStats, codec, size, d.Frames, d.FPS, d.LastDecodeMs, d.Errors)
}

func (m *RTSPModel) NetworkStatsText(lang i18n.Lang) string {
	n := m.snap.Net
	return i18n.T(lang, i18n.RTSPNetStats,
		rtsp.FormatBytes(n.Bytes),
		rtsp.FormatBitrate(n.BitrateBps),
		n.Packets,
		n.Lost,
		n.JitterMs,
	)
}

func (m *RTSPModel) Drain() {
	if m.player == nil {
		return
	}
	snap := m.player.Snapshot()
	if snap.Gen == m.lastGen {
		return
	}
	m.lastGen = snap.Gen
	m.snap = snap
	if m.follow && len(snap.Logs) > 0 {
		m.selected = len(snap.Logs) - 1
	} else if m.selected >= len(snap.Logs) {
		m.selected = max(0, len(snap.Logs)-1)
	}
	m.applyStatus(snap)
	m.generation++
	guigui.RequestRebuild()
}

func (m *RTSPModel) applyStatus(snap rtsp.Snapshot) {
	switch {
	case snap.Err != nil && !snap.Playing:
		m.SetStatus(i18n.StatusRTSPFailed, snap.Err)
	case snap.Download >= 0 && snap.Playing:
		m.SetStatus(i18n.StatusAdbOpenH264Download, snap.Download)
	case snap.Playing && snap.Image != nil:
		codec := snap.Dec.Codec.String()
		m.SetStatus(i18n.StatusRTSPPlaying, codec, snap.Dec.Width, snap.Dec.Height)
	case snap.Playing:
		m.SetStatus(i18n.StatusRTSPWaiting)
	}
}

func (m *RTSPModel) Connect() {
	url := strings.TrimSpace(m.url)
	if url == "" {
		m.SetStatus(i18n.StatusRTSPInvalidURL)
		return
	}
	if m.player == nil {
		m.player = rtsp.NewPlayer()
	}
	m.follow = true
	m.selected = 0
	m.SetStatus(i18n.StatusRTSPConnecting)
	m.player.Start(context.Background(), url)
	m.snap = m.player.Snapshot()
	m.lastGen = m.snap.Gen
	m.generation++
}

func (m *RTSPModel) Disconnect() {
	if m.player == nil {
		return
	}
	if !m.player.Playing() && !m.snap.Playing {
		return
	}
	m.player.Stop()
	m.snap = m.player.Snapshot()
	m.lastGen = m.snap.Gen
	m.SetStatus(i18n.StatusRTSPStopped)
	m.generation++
}

func (m *RTSPModel) Toggle() {
	if m.Playing() {
		m.Disconnect()
		return
	}
	m.Connect()
}

func (m *RTSPModel) Hint(lang i18n.Lang) string {
	hint := i18n.T(lang, i18n.RTSPHint)
	if a := h264.Attribution(); a != "" {
		hint += " " + a
	}
	return hint
}
