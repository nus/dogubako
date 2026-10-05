package app

import (
	"os"
	"path/filepath"
	"strings"
)

// modePref remembers the tool shown in the main panel.
type modePref struct {
	id     ToolID
	loaded bool
}

func (p *modePref) current() ToolID {
	if !p.loaded {
		p.id = loadMode()
		p.loaded = true
	}
	if p.id == "" {
		return ToolImage
	}
	return p.id
}

func (p *modePref) set(id ToolID) {
	if !knownTool(id) {
		return
	}
	if p.current() == id {
		return
	}
	p.id = id
	p.loaded = true
	_ = saveMode(id)
}

func loadMode() ToolID {
	path, err := modePrefPath()
	if err != nil {
		return ToolImage
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolImage
	}
	id := ToolID(strings.TrimSpace(string(data)))
	if !knownTool(id) {
		return ToolImage
	}
	return id
}

func saveMode(id ToolID) error {
	path, err := modePrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(string(id)+"\n"), 0o644)
}

func modePrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "mode"), nil
}

func knownTool(id ToolID) bool {
	for _, tool := range Tools {
		if tool.ID == id {
			return true
		}
	}
	return false
}
