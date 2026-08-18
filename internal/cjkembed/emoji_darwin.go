//go:build darwin

package cjkembed

import (
	"fmt"
	"os"
	"path/filepath"
)

func defaultEmojiPaths() []string {
	dir := "/System/Library/Fonts"
	names := []string{
		"Apple Color Emoji.ttc",
		"Apple Color Emoji.ttf",
	}
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = filepath.Join(dir, name)
	}
	return out
}

func init() {
	src, _, err := openEmoji(defaultEmojiPaths())
	if err != nil {
		fmt.Fprintf(os.Stderr, "dogubako: emoji: %v\n", err)
		return
	}
	registerEmoji(src)
}
