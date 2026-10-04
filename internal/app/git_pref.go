package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type gitTabsPref struct {
	Active int      `json:"active"`
	Paths  []string `json:"paths"`
}

func loadGitRepoPath() string {
	path, err := gitRepoPrefPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveGitRepoPath(repo string) error {
	path, err := gitRepoPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(repo+"\n"), 0o644)
}

func gitRepoPrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "git-repo"), nil
}

func gitTabsPrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "git-tabs.json"), nil
}

// loadGitTabs returns the saved tab paths and active index.
// A missing tab file falls back to the legacy single-repo preference.
func loadGitTabs() (paths []string, active int) {
	path, err := gitTabsPrefPath()
	if err != nil {
		return nil, 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if legacy := loadGitRepoPath(); legacy != "" {
			return []string{legacy}, 0
		}
		return nil, 0
	}
	var pref gitTabsPref
	if err := json.Unmarshal(data, &pref); err != nil {
		if legacy := loadGitRepoPath(); legacy != "" {
			return []string{legacy}, 0
		}
		return nil, 0
	}
	if pref.Active < 0 || pref.Active >= len(pref.Paths) {
		pref.Active = 0
	}
	return pref.Paths, pref.Active
}

func saveGitTabs(paths []string, active int) error {
	path, err := gitTabsPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if active < 0 || active >= len(paths) {
		active = 0
	}
	data, err := json.Marshal(gitTabsPref{Active: active, Paths: paths})
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func canonicalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved == "" {
		return path
	}
	return filepath.Clean(resolved)
}
