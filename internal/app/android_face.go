package app

import (
	"os"
	"path/filepath"
	"strings"
)

// AndroidFace is the pane shown inside the Android tool.
type AndroidFace string

const (
	AndroidFaceFiles  AndroidFace = "files"
	AndroidFaceScreen AndroidFace = "screen"
)

// androidFacePref remembers whether Android opens on files or the screen.
type androidFacePref struct {
	face   AndroidFace
	loaded bool
}

func (p *androidFacePref) current() AndroidFace {
	if !p.loaded {
		p.face = loadAndroidFace()
		p.loaded = true
	}
	if p.face != AndroidFaceScreen {
		return AndroidFaceFiles
	}
	return p.face
}

func (p *androidFacePref) set(face AndroidFace) {
	if face != AndroidFaceScreen {
		face = AndroidFaceFiles
	}
	if p.current() == face {
		return
	}
	p.face = face
	p.loaded = true
	_ = saveAndroidFace(face)
}

func loadAndroidFace() AndroidFace {
	path, err := androidFacePath()
	if err != nil {
		return AndroidFaceFiles
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AndroidFaceFiles
	}
	if AndroidFace(strings.TrimSpace(string(data))) == AndroidFaceScreen {
		return AndroidFaceScreen
	}
	return AndroidFaceFiles
}

func saveAndroidFace(face AndroidFace) error {
	path, err := androidFacePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(string(face)+"\n"), 0o644)
}

func androidFacePath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "android-face"), nil
}

// AndroidFace is the pane shown the next time Android is opened.
func (m *Model) AndroidFace() AndroidFace {
	m.migrateAndroidMode()
	return m.androidFace.current()
}

// SetAndroidFace switches the Android pane and remembers the choice.
// Leaving the screen pane stops the live preview.
func (m *Model) SetAndroidFace(face AndroidFace) {
	if face != AndroidFaceScreen {
		face = AndroidFaceFiles
	}
	cur := m.AndroidFace()
	if cur == face {
		return
	}
	if m.Mode() == ToolAndroid && cur == AndroidFaceScreen {
		m.androidShot.StopLive()
	}
	m.androidFace.set(face)
}

// migrateAndroidMode turns a saved android-shot tool into Android on the screen pane.
// After this, the tool id on disk is android.
func (m *Model) migrateAndroidMode() {
	if m.androidMigrated {
		return
	}
	m.androidMigrated = true
	m.bindAndroid()
	if m.mode.current() != ToolAndroidShot {
		return
	}
	m.androidFace.set(AndroidFaceScreen)
	m.mode.set(ToolAndroid)
}

// bindAndroid shares one device list between the file and screen panes.
func (m *Model) bindAndroid() {
	if m.adbBound {
		return
	}
	m.adbBound = true
	m.adb = &adbDevices{}
	m.android.shared = m.adb
	m.androidShot.shared = m.adb
	m.adb.files = &m.android
	m.adb.shot = &m.androidShot
}

// SelectAndroidDevice selects the same device for the file and screen panes.
func (m *Model) SelectAndroidDevice(serial string) {
	m.bindAndroid()
	if m.android.Busy() || m.androidShot.Busy() {
		return
	}
	m.android.SelectDevice(serial)
}

// AndroidScreenVisible reports whether the live preview should be running.
func (m *Model) AndroidScreenVisible() bool {
	return m.Mode() == ToolAndroid && m.AndroidFace() == AndroidFaceScreen
}
