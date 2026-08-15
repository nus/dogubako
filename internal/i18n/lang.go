package i18n

import (
	"os"
	"path/filepath"
	"strings"
)

// Lang is a UI language.
type Lang string

const (
	// JA is Japanese.
	JA Lang = "ja"
	// EN is English.
	EN Lang = "en"
	// Default is used when the locale is unknown.
	Default = JA
)

// Normalize maps an arbitrary value onto JA or EN.
func Normalize(lang Lang) Lang {
	if lang == EN {
		return EN
	}
	return JA
}

// ParseLocale maps a BCP 47 / POSIX locale (en_US.UTF-8, ja-JP, …) onto JA or EN.
func ParseLocale(s string) Lang {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "c" || s == "posix" {
		return Default
	}
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "-", "_")
	primary := s
	if i := strings.IndexByte(s, '_'); i >= 0 {
		primary = s[:i]
	}
	switch primary {
	case "en", "eng", "english":
		return EN
	case "ja", "jpn", "jp", "japanese":
		return JA
	default:
		return Default
	}
}

// Detect returns the OS UI language, defaulting to Japanese.
func Detect() Lang {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return ParseLocale(v)
		}
	}
	return Default
}

// Load returns the saved language, or Detect if none is stored.
func Load() Lang {
	path, err := configPath()
	if err != nil {
		return Detect()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Detect()
	}
	switch strings.ToLower(strings.TrimSpace(string(data))) {
	case string(EN):
		return EN
	case string(JA):
		return JA
	default:
		return Detect()
	}
}

// Save persists the language for the next launch.
func Save(lang Lang) error {
	lang = Normalize(lang)
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(string(lang)+"\n"), 0o644)
}

// OverrideUserConfigDir replaces os.UserConfigDir. Tests should defer the returned restore function.
func OverrideUserConfigDir(fn func() (string, error)) (restore func()) {
	orig := userConfigDir
	userConfigDir = fn
	return func() { userConfigDir = orig }
}

var userConfigDir = os.UserConfigDir

func configPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "lang"), nil
}
