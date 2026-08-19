//go:build linux

package cjkembed

import "path/filepath"

func defaultEmojiPaths() []string {
	return []string{
		filepath.Join("/usr/share/fonts/truetype/noto-color-emoji/NotoColorEmoji.ttf"),
		filepath.Join("/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf"),
		filepath.Join("/usr/share/fonts/google-noto-emoji/NotoColorEmoji.ttf"),
		filepath.Join("/usr/share/fonts/noto/NotoColorEmoji.ttf"),
	}
}
