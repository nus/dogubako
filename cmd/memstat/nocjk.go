//go:build nocjk

// This build is for memory comparison only. The default Inter face has no
// kana/kanji glyphs, and Guigui does not fall back to OS fonts, so Japanese
// UI text renders as .notdef tofu.

package main

const cjkEnabled = false
