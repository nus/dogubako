package cjkembed

import (
	"fmt"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

var theEmojiFace *text.GoTextFaceSource

// emojiUnicodeRanges covers pictographs used as UI icons (folder/file emoji).
var emojiUnicodeRanges = []basicwidget.UnicodeRange{
	{Min: 0x1f000, Max: 0x1faff},
}

func registerEmoji(src *text.GoTextFaceSource) {
	theEmojiFace = src
	basicwidget.RegisterFaceSourceEntries(appendEmojiEntries)
}

func appendEmojiEntries(context *guigui.Context, adder *basicwidget.FaceSourceEntryAdder) {
	if theEmojiFace == nil {
		return
	}
	adder.SetPriority(basicwidget.FaceSourceEntryPriorityHigh)
	adder.Add(basicwidget.FaceSourceEntry{
		FaceSource:    theEmojiFace,
		UnicodeRanges: emojiUnicodeRanges,
	})
}

func pickEmojiFace(sources []*text.GoTextFaceSource) *text.GoTextFaceSource {
	var fallback *text.GoTextFaceSource
	for _, src := range sources {
		if src == nil {
			continue
		}
		if fallback == nil {
			fallback = src
		}
		fam := strings.ToLower(src.Metadata().Family)
		if strings.Contains(fam, "emoji") || strings.Contains(fam, "color") {
			return src
		}
	}
	return fallback
}

func openEmoji(paths []string) (*text.GoTextFaceSource, string, error) {
	var firstErr error
	for _, path := range expandPathNorms(paths) {
		sources, err := loadFaceSources(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		face := pickEmojiFace(sources)
		if face == nil {
			continue
		}
		return face, path, nil
	}
	if firstErr != nil {
		return nil, "", firstErr
	}
	return nil, "", fmt.Errorf("emoji font not found")
}
