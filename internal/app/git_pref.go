package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/nus/dogubako/internal/gitcli"
)

const gitRecentMax = 12

type gitTabsPref struct {
	Active int      `json:"active"`
	Paths  []string `json:"paths"`
	Recent []string `json:"recent,omitempty"`
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

// loadGitTabs returns the saved tab paths, active index, and recent repositories.
// A missing tab file falls back to the legacy single-repo preference.
func loadGitTabs() (paths []string, active int, recent []string) {
	path, err := gitTabsPrefPath()
	if err != nil {
		return nil, 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if legacy := loadGitRepoPath(); legacy != "" {
			return []string{legacy}, 0, nil
		}
		return nil, 0, nil
	}
	var pref gitTabsPref
	if err := json.Unmarshal(data, &pref); err != nil {
		if legacy := loadGitRepoPath(); legacy != "" {
			return []string{legacy}, 0, nil
		}
		return nil, 0, nil
	}
	if pref.Active < 0 || pref.Active >= len(pref.Paths) {
		pref.Active = 0
	}
	return pref.Paths, pref.Active, normalizeGitRecent(pref.Recent)
}

func saveGitTabs(paths []string, active int, recent []string) error {
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
	data, err := json.Marshal(gitTabsPref{Active: active, Paths: paths, Recent: recent})
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func normalizeGitRecent(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		p = canonicalPath(p)
		if p == "" || p == "." || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) == gitRecentMax {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// seedGitRecent builds a recent list from open tabs, with the active tab first.
func seedGitRecent(paths []string, active int) []string {
	ordered := make([]string, 0, len(paths))
	if active >= 0 && active < len(paths) {
		ordered = append(ordered, paths[active])
	}
	for i, p := range paths {
		if i == active {
			continue
		}
		ordered = append(ordered, p)
	}
	return normalizeGitRecent(ordered)
}

func canonicalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if loc := gitcli.ParseLoc(path); loc.IsRemote() {
		if loc.Dir == "" || loc.Dir == "." {
			return ""
		}
		return loc.Key()
	}
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved == "" {
		return path
	}
	return filepath.Clean(resolved)
}
