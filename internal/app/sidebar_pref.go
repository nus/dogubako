package app

import (
	"os"
	"path/filepath"
	"strings"
)

// sidebarPref remembers whether the left menu is folded.
type sidebarPref struct {
	collapsedValue bool
	loaded         bool
}

func (p *sidebarPref) collapsed() bool {
	if !p.loaded {
		p.collapsedValue = loadSidebarCollapsed()
		p.loaded = true
	}
	return p.collapsedValue
}

func (p *sidebarPref) setCollapsed(collapsed bool) {
	if p.collapsed() == collapsed {
		return
	}
	p.collapsedValue = collapsed
	p.loaded = true
	_ = saveSidebarCollapsed(collapsed)
}

func loadSidebarCollapsed() bool {
	path, err := sidebarPrefPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.ToLower(strings.TrimSpace(string(data))) == "collapsed"
}

func saveSidebarCollapsed(collapsed bool) error {
	path, err := sidebarPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	val := "expanded"
	if collapsed {
		val = "collapsed"
	}
	return os.WriteFile(path, []byte(val+"\n"), 0o644)
}

func sidebarPrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "sidebar"), nil
}

var userConfigDir = os.UserConfigDir

func overrideSidebarConfigDir(fn func() (string, error)) (restore func()) {
	orig := userConfigDir
	userConfigDir = fn
	return func() { userConfigDir = orig }
}
