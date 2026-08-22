package cjkembed

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

var (
	theHiraginoFace *text.GoTextFaceSource
	theLocales      []language.Tag
)

var jaBase = language.MustParseBase("ja")

func registerHiragino(src *text.GoTextFaceSource) {
	theHiraginoFace = src
	basicwidget.RegisterFaceSourceEntries(appendHiraginoEntries)
}

func appendHiraginoEntries(context *guigui.Context, adder *basicwidget.FaceSourceEntryAdder) {
	if theHiraginoFace == nil {
		return
	}
	theLocales = slices.Delete(theLocales, 0, len(theLocales))
	theLocales = context.AppendLocales(theLocales)
	jaPrimary := japanesePrimary(theLocales)
	theLocales = slices.Delete(theLocales, 0, len(theLocales))

	if jaPrimary {
		// Prefer Hiragino for glyphs that differ between CJK and Western fonts.
		// See https://www.unicode.org/L2/L2014/14006-sv-western-vs-cjk.pdf
		adder.Add(basicwidget.FaceSourceEntry{
			FaceSource:    basicwidget.DefaultFaceSourceEntry().FaceSource,
			UnicodeRanges: westernSafeRanges,
		})
		adder.Add(basicwidget.FaceSourceEntry{FaceSource: theHiraginoFace})
		return
	}
	adder.Add(basicwidget.DefaultFaceSourceEntry())
	adder.Add(basicwidget.FaceSourceEntry{FaceSource: theHiraginoFace})
}

func japanesePrimary(locales []language.Tag) bool {
	if len(locales) == 0 {
		return false
	}
	base, conf := locales[0].Base()
	return conf != language.No && base == jaBase
}

// westernSafeRanges is Inter coverage that should win over CJK when Japanese
// is the primary locale, matching guigui/basicwidget/cjkfont.
var westernSafeRanges = []basicwidget.UnicodeRange{
	{Min: 0x0000, Max: 0x2013},
	{Min: 0x2016, Max: 0x2017},
	{Min: 0x201a, Max: 0x201b},
	{Min: 0x201e, Max: 0x2025},
	{Min: 0x2027, Max: 0x2e39},
	{Min: 0x2e3c, Max: 0x7fffffff},
}
