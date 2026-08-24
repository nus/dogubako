package app

import (
	"fmt"
	"image"
	"time"

	"github.com/nus/dogubako/internal/i18n"
)

const (
	stopwatchCentisecond = 10 * time.Millisecond
	// stopwatchDisplayGlyphs is the width of "00:00:00.00".
	stopwatchDisplayGlyphs = 11
	stopwatchGlyphEmWidth  = 0.62
	stopwatchMinFontSize   = 24
)

// StopwatchModel is a start / pause / reset timer.
type StopwatchModel struct {
	generation  uint64
	running     bool
	accumulated time.Duration
	started     time.Time
	status      statusMsg
}

func (m *StopwatchModel) Generation() uint64 { return m.generation }
func (m *StopwatchModel) Running() bool      { return m.running }

func (m *StopwatchModel) Elapsed() time.Duration {
	return m.elapsedAt(time.Now())
}

func (m *StopwatchModel) Display() string {
	return FormatStopwatch(m.Elapsed())
}

func (m *StopwatchModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *StopwatchModel) SetStatus(key i18n.Key, args ...any) {
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

// CanReset reports whether Reset would change state.
func (m *StopwatchModel) CanReset() bool {
	return m.running || m.accumulated > 0
}

// CanCopy reports whether the elapsed time is frozen and can be copied.
func (m *StopwatchModel) CanCopy() bool {
	return !m.running && m.accumulated > 0
}

// Toggle starts a stopped watch or pauses a running one.
func (m *StopwatchModel) Toggle() {
	m.toggleAt(time.Now())
}

func (m *StopwatchModel) Reset() {
	if !m.CanReset() {
		return
	}
	m.running = false
	m.accumulated = 0
	m.started = time.Time{}
	m.status = statusMsg{}
	m.generation++
}

func (m *StopwatchModel) toggleAt(now time.Time) {
	m.status = statusMsg{}
	if m.running {
		m.accumulated += now.Sub(m.started)
		if m.accumulated < 0 {
			m.accumulated = 0
		}
		m.running = false
		m.generation++
		return
	}
	m.started = now
	m.running = true
	m.generation++
}

func (m *StopwatchModel) elapsedAt(now time.Time) time.Duration {
	d := m.accumulated
	if m.running {
		d += now.Sub(m.started)
	}
	if d < 0 {
		return 0
	}
	return d
}

// DisplayTicks is elapsed time in centiseconds, for UI rebuild keys.
func (m *StopwatchModel) DisplayTicks() int64 {
	return int64(m.Elapsed() / stopwatchCentisecond)
}

// FormatStopwatch renders elapsed time as HH:MM:SS.cc.
func FormatStopwatch(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	cs := d / stopwatchCentisecond
	hours := int64(cs / 360000)
	minutes := int64((cs / 6000) % 60)
	seconds := int64((cs / 100) % 60)
	frac := int64(cs % 100)
	return fmt.Sprintf("%02d:%02d:%02d.%02d", hours, minutes, seconds, frac)
}

// stopwatchFontSize is about 1/3 of the panel height, capped so the time
// string still fits the panel width.
func stopwatchFontSize(panel image.Point) float64 {
	if panel.Y <= 0 {
		return stopwatchMinFontSize
	}
	size := float64(panel.Y) / 3
	if panel.X > 0 {
		byWidth := float64(panel.X) / (stopwatchDisplayGlyphs * stopwatchGlyphEmWidth)
		if byWidth < size {
			size = byWidth
		}
	}
	if size < stopwatchMinFontSize {
		return stopwatchMinFontSize
	}
	return size
}
