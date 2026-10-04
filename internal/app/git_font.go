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

var (
	gitMonoOnce   sync.Once
	gitMonoSource *text.GoTextFaceSource
	gitMonoFamily *basicwidget.FontFamily
)

func gitDiffFont() *basicwidget.FontFamily {
	gitMonoOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(notoSansMonoCJKJPGz))
		if err != nil {
			return
		}
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			return
		}
		src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			return
		}
		gitMonoSource = src
		gitMonoFamily = basicwidget.NewFontFamily([]basicwidget.FaceSourceEntry{{
			FaceSource: src,
		}}, nil)
	})
	return gitMonoFamily
}

// useGitMono sets the shared monospace family on style when the font is available.
func useGitMono(style *basicwidget.TextStyle) {
	if family := gitDiffFont(); family != nil {
		style.SetFontFamily(family)
	}
}
