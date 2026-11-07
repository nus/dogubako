package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// textScalePercents are the UI text sizes, as a percentage of the default.
// Each step is 25. AppScale is the percentage divided by 100.
var textScalePercents = []int{75, 100, 125, 150, 175, 200}

const textScaleDefaultPercent = 100

// textScalePref remembers the UI text size. An unreadable or off-step file
// stays on disk and the session uses 100%.
type textScalePref struct {
	percent int
	loaded  bool
}

func (p *textScalePref) current() int {
	if !p.loaded {
		p.percent = loadTextScale()
		p.loaded = true
	}
	if !knownTextScale(p.percent) {
		return textScaleDefaultPercent
	}
	return p.percent
}

func (p *textScalePref) set(percent int) {
	if !knownTextScale(percent) || p.current() == percent {
		return
	}
	p.percent = percent
	p.loaded = true
	_ = saveTextScale(percent)
}

// TextScalePercent is the saved UI text size: 75, 100, 125, 150, 175, or 200.
func (m *Model) TextScalePercent() int {
	return m.textScale.current()
}

// TextAppScale is the AppScale for TextScalePercent. Device scale is separate.
func (m *Model) TextAppScale() float64 {
	return appScaleFromPercent(m.TextScalePercent())
}

// StepTextScale moves one step toward smaller (dir -1) or larger (dir 1).
// The ends do not wrap, and a no-op does not write the setting.
func (m *Model) StepTextScale(dir int) {
	next, ok := stepTextScale(m.TextScalePercent(), dir)
	if !ok {
		return
	}
	m.textScale.set(next)
}

// CanShrinkText reports whether a smaller step exists.
func (m *Model) CanShrinkText() bool {
	_, ok := stepTextScale(m.TextScalePercent(), -1)
	return ok
}

// CanGrowText reports whether a larger step exists.
func (m *Model) CanGrowText() bool {
	_, ok := stepTextScale(m.TextScalePercent(), 1)
	return ok
}

// SavedAppScale is the text size to apply before the first frame.
// A missing or unusable setting is 1. The window size is unchanged.
func SavedAppScale() float64 {
	return appScaleFromPercent(loadTextScale())
}

func appScaleFromPercent(percent int) float64 {
	if !knownTextScale(percent) {
		percent = textScaleDefaultPercent
	}
	return float64(percent) / 100
}

func stepTextScale(percent, dir int) (int, bool) {
	if dir != -1 && dir != 1 {
		return percent, false
	}
	for i, step := range textScalePercents {
		if step != percent {
			continue
		}
		j := i + dir
		if j < 0 || j >= len(textScalePercents) {
			return percent, false
		}
		return textScalePercents[j], true
	}
	return percent, false
}

func knownTextScale(percent int) bool {
	for _, step := range textScalePercents {
		if step == percent {
			return true
		}
	}
	return false
}

func textScaleLabel(percent int) string {
	return strconv.Itoa(percent) + "%"
}

func loadTextScale() int {
	path, err := textScalePath()
	if err != nil {
		return textScaleDefaultPercent
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return textScaleDefaultPercent
	}
	percent, ok := parseTextScale(string(data))
	if !ok {
		return textScaleDefaultPercent
	}
	return percent
}

func parseTextScale(data string) (int, bool) {
	s := strings.TrimSpace(data)
	for _, step := range textScalePercents {
		if s == strconv.Itoa(step) {
			return step, true
		}
	}
	return textScaleDefaultPercent, false
}

func saveTextScale(percent int) error {
	if !knownTextScale(percent) {
		return os.ErrInvalid
	}
	path, err := textScalePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(percent)+"\n"), 0o644)
}

func textScalePath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "text-scale"), nil
}
