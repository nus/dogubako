// Package cjkembed registers a CJK face for gogpu/ui.
//
// Linux prefers a Japanese-only Noto Sans CJK OTF when present, and
// otherwise extracts one face from the Super OTC. macOS opens Hiragino
// Sans from /System/Library/Fonts and extracts that face from the TTC.
// Other platforms do not add a CJK face. Color emoji fonts are not
// registered; file-tree icons are drawn as vector glyphs instead.
//
// gogpu/ui has no per-glyph fallback, so widgets that should show Japanese
// must set FontFamily to [FamilyName] after [Register]. When Register cannot
// load a CJK face, FamilyName is empty (the default Latin font) and
// [Available] is false so the app can keep the UI in English.
package cjkembed
