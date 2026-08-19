package cjkembed

import (
	"path/filepath"

	"golang.org/x/text/language"
)

// FamilyName is the gogpu/ui font family used for CJK text.
const FamilyName = "CJK"

const hiraginoFontDir = "/System/Library/Fonts"

var jaBase = language.MustParseBase("ja")

// Register loads system CJK (and emoji, when present) into the gogpu/ui
// font registry. Call once before creating the widget tree.
func Register() {
	registerCJK()
	registerEmoji()
}

func registerCJK() {
	src, path, err := openCJKFont(defaultCJKPaths())
	if err != nil || src == nil || path == "" {
		if err != nil && len(defaultCJKPaths()) > 0 {
			logFontErr("cjk", err)
		}
		return
	}
	registerLoaded("cjk", FamilyName, path)
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
