//go:build !linux || nocjk

// Production macOS (and memstat -tags nocjk on Linux) do not embed Noto Sans CJK.
// The default Inter face has no kana/kanji glyphs.

package main

const cjkEnabled = false
