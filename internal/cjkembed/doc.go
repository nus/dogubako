// Package cjkembed registers CJK faces for Guigui.
//
// Linux builds embed Noto Sans CJK. macOS opens Hiragino Sans from
// /System/Library/Fonts. Color emoji fonts are not loaded; they are
// hundreds of megabytes and folder/file icons are drawn as vectors instead.
// Other platforms do not add a CJK face.
package cjkembed
