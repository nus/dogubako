// Package cjkembed registers a CJK face for gogpu/ui.
//
// Linux loads Noto Sans CJK from the system font directories. macOS opens
// Hiragino Sans from /System/Library/Fonts. Other platforms do not add a
// CJK face. Color emoji fonts are not registered; file-tree icons are drawn
// as vector glyphs instead.
//
// gogpu/ui has no per-glyph fallback, so widgets that should show Japanese
// must set FontFamily to [FamilyName] after [Register]. When Register cannot
// load a CJK face, FamilyName is empty (the default Latin font) and
// [Available] is false so the app can keep the UI in English.
package cjkembed
