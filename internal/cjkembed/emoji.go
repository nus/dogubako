package cjkembed

import (
	"fmt"
	"os"
	"strings"

	"github.com/gogpu/gg/text"
)

func registerEmoji() {
	src, path, err := openEmoji(defaultEmojiPaths())
	if err != nil || src == nil || path == "" {
		return
	}
	if err := registerLoaded("emoji", "Emoji", path); err != nil {
		logFontErr("emoji", err)
	}
}

func pickEmojiFace(sources []*text.FontSource) *text.FontSource {
	var fallback *text.FontSource
	for _, src := range sources {
		if src == nil {
			continue
		}
		if fallback == nil {
			fallback = src
		}
		fam := strings.ToLower(src.Name())
		if strings.Contains(fam, "emoji") || strings.Contains(fam, "color") {
			return src
		}
	}
	return fallback
}

func openEmoji(paths []string) (*text.FontSource, string, error) {
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
