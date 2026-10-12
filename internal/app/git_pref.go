package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
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

func gitGraphWidthPrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "git-graph-width"), nil
}

func validGitGraphUnits(units float64) bool {
	return units > 0 && !math.IsNaN(units) && !math.IsInf(units, 0)
}

var (
	graphUnitsPrefReady bool
	graphUnitsPref      float64
)

// preferredGitGraphUnits is the commit-graph column width to use when a widget
// has not been given one yet. Zero keeps the width that fits the lanes.
func preferredGitGraphUnits() float64 {
	if !graphUnitsPrefReady {
		graphUnitsPref = loadGitGraphUnits()
		graphUnitsPrefReady = true
	}
	return graphUnitsPref
}

// preferGraphUnits returns units when it is a real dragged width, otherwise the
// width saved from the last session.
func preferGraphUnits(units float64) float64 {
	if validGitGraphUnits(units) {
		return units
	}
	return preferredGitGraphUnits()
}

func rememberGitGraphUnits(units float64) {
	if !validGitGraphUnits(units) {
		return
	}
	graphUnitsPref = units
	graphUnitsPrefReady = true
	_ = saveGitGraphUnits(units)
}

func resetGitGraphUnitsPref() {
	graphUnitsPrefReady = false
	graphUnitsPref = 0
}

type gitMetaWidthPref struct {
	Author float64 `json:"author,omitempty"`
	Hash   float64 `json:"hash,omitempty"`
	Date   float64 `json:"date,omitempty"`
}

func gitMetaWidthPrefPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "git-meta-widths.json"), nil
}

var (
	metaWidthPrefReady bool
	metaWidthPref      gitMetaWidthPref
)

// preferredGitMetaUnits is the saved author, commit-id, and date column widths.
// A zero keeps that column's default width.
func preferredGitMetaUnits() (author, hash, date float64) {
	if !metaWidthPrefReady {
		metaWidthPref = loadGitMetaUnits()
		metaWidthPrefReady = true
	}
	return metaWidthPref.Author, metaWidthPref.Hash, metaWidthPref.Date
}

// preferMetaUnits keeps a dragged width and fills any unset column from the last session.
func preferMetaUnits(author, hash, date float64) (float64, float64, float64) {
	savedAuthor, savedHash, savedDate := preferredGitMetaUnits()
	if !validGitGraphUnits(author) {
		author = savedAuthor
	}
	if !validGitGraphUnits(hash) {
		hash = savedHash
	}
	if !validGitGraphUnits(date) {
		date = savedDate
	}
	return author, hash, date
}

func rememberGitMetaUnits(author, hash, date float64) {
	pref := gitMetaWidthPref{}
	if validGitGraphUnits(author) {
		pref.Author = author
	}
	if validGitGraphUnits(hash) {
		pref.Hash = hash
	}
	if validGitGraphUnits(date) {
		pref.Date = date
	}
	metaWidthPref = pref
	metaWidthPrefReady = true
	_ = saveGitMetaUnits(pref)
}

func resetGitMetaUnitsPref() {
	metaWidthPrefReady = false
	metaWidthPref = gitMetaWidthPref{}
}

func loadGitMetaUnits() gitMetaWidthPref {
	path, err := gitMetaWidthPrefPath()
	if err != nil {
		return gitMetaWidthPref{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return gitMetaWidthPref{}
	}
	var pref gitMetaWidthPref
	if err := json.Unmarshal(data, &pref); err != nil {
		return gitMetaWidthPref{}
	}
	if !validGitGraphUnits(pref.Author) {
		pref.Author = 0
	}
	if !validGitGraphUnits(pref.Hash) {
		pref.Hash = 0
	}
	if !validGitGraphUnits(pref.Date) {
		pref.Date = 0
	}
	return pref
}

func saveGitMetaUnits(pref gitMetaWidthPref) error {
	if !validGitGraphUnits(pref.Author) && !validGitGraphUnits(pref.Hash) && !validGitGraphUnits(pref.Date) {
		return nil
	}
	path, err := gitMetaWidthPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(pref)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// loadGitGraphUnits returns the saved commit-graph column width in unit sizes.
// Missing or unusable values are zero, which keeps the width that fits the lanes.
func loadGitGraphUnits() float64 {
	path, err := gitGraphWidthPrefPath()
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	units, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil || !validGitGraphUnits(units) {
		return 0
	}
	return units
}

func saveGitGraphUnits(units float64) error {
	if !validGitGraphUnits(units) {
		return nil
	}
	path, err := gitGraphWidthPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text := strconv.FormatFloat(units, 'g', -1, 64)
	return os.WriteFile(path, []byte(text+"\n"), 0o644)
}
