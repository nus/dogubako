package cjkembed

import (
	"fmt"
	"path/filepath"

	"golang.org/x/text/language"
)

const (
	cjkFamily       = "CJK"
	hiraginoFontDir = "/System/Library/Fonts"
)

// FamilyName is the gogpu/ui font family for Japanese text.
// Empty when Register did not find a CJK face; widgets then use the default font.
var FamilyName string

var (
	cjkLoaded   bool
	cjkPathList = defaultCJKPaths
	jaBase      = language.MustParseBase("ja")
)

// Available reports whether a CJK face was registered for this process.
func Available() bool { return cjkLoaded }

// Register loads system CJK (and emoji, when present) into the gogpu/ui
// font registry. Call once before creating the widget tree.
// It returns whether a CJK face was loaded.
func Register() bool {
	cjkLoaded = false
	FamilyName = ""
	ok := registerCJK()
	registerEmoji()
	return ok
}

func registerCJK() bool {
	src, path, err := openCJKFont(cjkPathList())
	if err != nil || src == nil || path == "" {
		if err != nil {
			logFontErr("cjk", fmt.Errorf("%w (using English UI)", err))
		}
		return false
	}
	if err := registerLoaded("cjk", cjkFamily, path); err != nil {
		logFontErr("cjk", fmt.Errorf("%w (using English UI)", err))
		return false
	}
	FamilyName = cjkFamily
	cjkLoaded = true
	return true
}

func japanesePrimary(locales []language.Tag) bool {
	if len(locales) == 0 {
		return false
	}
	base, conf := locales[0].Base()
	return conf != language.No && base == jaBase
}

// defaultHiraginoPaths is the usual macOS location of Hiragino Sans, preferred
// weight first. Each weight is a separate .ttc.
func defaultHiraginoPaths() []string {
	names := []string{
		"ヒラギノ角ゴシック W3.ttc",
		"ヒラギノ角ゴシック W4.ttc",
		"ヒラギノ角ゴシック W5.ttc",
		"ヒラギノ角ゴシック W6.ttc",
		"ヒラギノ角ゴシック W2.ttc",
	}
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = filepath.Join(hiraginoFontDir, name)
	}
	return out
}
