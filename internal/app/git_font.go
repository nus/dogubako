package app

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/guigui-gui/guigui/basicwidget"
)

// Noto Sans Mono CJK JP Regular (SIL Open Font License 1.1).
// Source: https://github.com/notofonts/noto-cjk/releases/tag/Sans2.004
// The license text is mono/OFL.txt.
//
//go:embed mono/NotoSansMonoCJKjp-Regular.otf.gz
var notoSansMonoCJKJPGz []byte

// Noto Emoji Regular (SIL Open Font License 1.1), a monochrome outline face.
// Source: https://github.com/google/fonts/tree/main/ofl/notoemoji
// The variable font is instanced at weight 400. Vertical metrics are fitted
// to Noto Sans Mono CJK JP so a line's box does not grow. The license text
// is mono/OFL-NotoEmoji.txt. Color tables are not included.
//
//go:embed mono/NotoEmoji-Regular.ttf.gz
var notoEmojiGz []byte

var (
	gitMonoOnce    sync.Once
	gitMonoSource  *text.GoTextFaceSource
	gitEmojiSource *text.GoTextFaceSource
	gitMonoFamily  *basicwidget.FontFamily
)

func gitDiffFont() *basicwidget.FontFamily {
	gitMonoOnce.Do(func() {
		mono, err := loadGzipFace(notoSansMonoCJKJPGz)
		if err != nil {
			return
		}
		gitMonoSource = mono
		entries := []basicwidget.FaceSourceEntry{{FaceSource: mono}}
		if emoji, err := loadGzipFace(notoEmojiGz); err == nil {
			gitEmojiSource = emoji
			// Mono wins when both faces have a glyph. Emoji is only the
			// fallback for code points the monospace face does not cover.
			entries = append(entries, basicwidget.FaceSourceEntry{FaceSource: emoji})
		}
		gitMonoFamily = basicwidget.NewFontFamily(entries, nil)
	})
	return gitMonoFamily
}

func loadGzipFace(gz []byte) (*text.GoTextFaceSource, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	zr.Close()
	if err != nil {
		return nil, err
	}
	return text.NewGoTextFaceSource(bytes.NewReader(data))
}

// useGitMono sets the shared monospace family on style when the font is available.
// Search fields, filters, the SSH form, and name dialogs do not call this.
func useGitMono(style *basicwidget.TextStyle) {
	if family := gitDiffFont(); family != nil {
		style.SetFontFamily(family)
	}
}
