// Package cjkembed registers CJK and emoji faces for Guigui.
//
// Linux builds embed Noto Sans CJK. macOS opens Hiragino Sans from
// /System/Library/Fonts. Color emoji uses Apple Color Emoji on macOS and
// Noto Color Emoji on Linux when those system fonts exist.
// Other platforms do not add a CJK face.
package cjkembed
