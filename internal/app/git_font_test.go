package app

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/nus/dogubako/internal/gitcli"
)

func TestGitEmojiFontLoaded(t *testing.T) {
	if gitDiffFont() == nil || gitMonoSource == nil || gitEmojiSource == nil {
		t.Fatal("monospace or emoji face missing")
	}
}

func TestGitEmojiFontHasNoColorTables(t *testing.T) {
	zr, err := gzip.NewReader(bytes.NewReader(notoEmojiGz))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	zr.Close()
	if err != nil {
		t.Fatal(err)
	}
	tags := sfntTableTags(t, data)
	for _, bad := range []string{"COLR", "CPAL", "CBDT", "CBLC", "sbix", "SVG ", "EBDT", "EBLC"} {
		if _, ok := tags[bad]; ok {
			t.Fatalf("color table %q is embedded", bad)
		}
	}
}

func sfntTableTags(t *testing.T, data []byte) map[string]struct{} {
	t.Helper()
	if len(data) < 12 {
		t.Fatal("font too short")
	}
	n := int(binary.BigEndian.Uint16(data[4:6]))
	tags := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		off := 12 + i*16
		if off+4 > len(data) {
			t.Fatal("table record past end")
		}
		tags[string(data[off:off+4])] = struct{}{}
	}
	return tags
}

func TestGitEmojiKeepsMonoMetrics(t *testing.T) {
	mono := &text.GoTextFace{Source: gitMonoSource, Size: 16}
	multi := gitMonoEmojiFace(t, 16)
	mm := mono.Metrics()
	em := multi.Metrics()
	if em.HAscent > mm.HAscent+0.01 || em.HDescent > mm.HDescent+0.01 {
		t.Fatalf("line metrics grew: mono ascent %v descent %v, combined ascent %v descent %v", mm.HAscent, mm.HDescent, em.HAscent, em.HDescent)
	}
}

func TestGitEmojiVisibleAndMonoSymbolsStay(t *testing.T) {
	mono := &text.GoTextFace{Source: gitMonoSource, Size: 32}
	emoji := &text.GoTextFace{Source: gitEmojiSource, Size: 32}
	multi := gitMonoEmojiFace(t, 32)
	for _, s := range []string{"😀", "🚀", "✨", "✅", "❌", "❤", "⭐", "❤️"} {
		got := text.Advance(s, multi)
		want := text.Advance(s, emoji)
		if got <= 0 || got != want {
			t.Fatalf("%s advance multi %v emoji %v", s, got, want)
		}
	}
	for _, s := range []string{"★", "✓", "♥", "☀", "⚠"} {
		if got, want := text.Advance(s, multi), text.Advance(s, mono); got != want {
			t.Fatalf("%s uses a different face: multi %v mono %v", s, got, want)
		}
		bare := text.AppendLazyGlyphs(nil, s, multi, nil)
		with := text.AppendLazyGlyphs(nil, s+"\uFE0F", multi, nil)
		if len(bare) == 0 || len(with) == 0 || with[0].GID != bare[0].GID {
			t.Fatalf("%s + VS16 changed the monospace glyph", s)
		}
		if got := text.Advance(s+"\uFE0F", multi); got != text.Advance(s, multi) {
			t.Fatalf("%s + VS16 width %v, bare %v", s, got, text.Advance(s, multi))
		}
	}
	if text.Advance("\uFE0F", emoji) != 0 || text.Advance("\u200D", emoji) != 0 {
		t.Fatal("VS16 or ZWJ takes width")
	}
	if text.Advance("★\u200D", multi) != text.Advance("★", multi) {
		t.Fatal("ZWJ after a monospace glyph takes width")
	}
	// A digit keycap may stay separate. The variation selector still takes no width.
	if text.Advance("1\uFE0F", multi) != text.Advance("1", multi) {
		t.Fatal("VS16 after a digit takes width")
	}
}

func TestGitEmojiSequencesAreOneCluster(t *testing.T) {
	multi := gitMonoEmojiFace(t, 32)
	one := text.Advance("😀", multi)
	for _, s := range []string{"🇯🇵", "👍🏻", "👨‍👩‍👧"} {
		full := text.Advance(s, multi)
		if full <= 0 || full > one*1.35 {
			t.Fatalf("%s width %v, one emoji %v", s, full, one)
		}
		// A byte inside the sequence belongs to the one cluster, so the
		// caret snaps back to the cluster's leading edge.
		if mid := text.AdvanceAt(s, 4, multi); mid != 0 {
			t.Fatalf("%s split at byte 4: advance %v, full %v", s, mid, full)
		}
	}
	// Two regional indicators that are not yet ligated would be wider than
	// one cell. A lone indicator is itself one glyph.
	if text.Advance("🇯", multi) <= 0 {
		t.Fatal("regional indicator has no glyph")
	}
}

func TestGitEmojiIsMonochromeAndFitsTheLine(t *testing.T) {
	const size = 64.0
	face := gitMonoEmojiFace(t, size)
	mono := &text.GoTextFace{Source: gitMonoSource, Size: size}
	mm := mono.Metrics()
	// LineHeight / FontSize is 18/12. The slack around the face is split
	// above and below the baseline. Emoji has to stay inside that line.
	line := size * 3 / 2
	pad := (line - (mm.HAscent + mm.HDescent)) / 2
	if pad < 0 {
		t.Fatalf("monospace metrics exceed the line: ascent %v descent %v", mm.HAscent, mm.HDescent)
	}
	roomAbove := pad + mm.HAscent
	roomBelow := pad + mm.HDescent
	for _, s := range []string{"😀", "🚀", "✨", "✅", "❌", "❤", "⭐", "🇯🇵", "👍🏻", "👨‍👩‍👧"} {
		glyphs := text.AppendLazyGlyphs(nil, s, face, nil)
		if len(glyphs) != 1 {
			t.Fatalf("%s shaped to %d glyphs", s, len(glyphs))
		}
		g := glyphs[0]
		if g.Colored() {
			t.Fatalf("%s is a color glyph", s)
		}
		if g.EndIndexInBytes-g.StartIndexInBytes != len(s) {
			t.Fatalf("%s glyph covers bytes [%d,%d)", s, g.StartIndexInBytes, g.EndIndexInBytes)
		}
		if g.ImageBounds.Empty() {
			t.Fatalf("%s has no outline", s)
		}
		above := g.OriginY - float64(g.ImageBounds.Min.Y)
		below := float64(g.ImageBounds.Max.Y) - g.OriginY
		if above > roomAbove || below > roomBelow {
			t.Fatalf("%s sticks out of the line: above %v below %v", s, above, below)
		}
	}
}

func TestHistorySearchMatchesEmoji(t *testing.T) {
	rows := []gitcli.GraphRow{{
		Commit: gitcli.Commit{Hash: "abc123", Subject: "ship 🚀"},
	}}
	got := historyMatchHashes(rows, "🚀")
	if len(got) != 1 || got[0] != "abc123" {
		t.Fatalf("matches = %v", got)
	}
}

func gitMonoEmojiFace(t *testing.T, size float64) text.Face {
	t.Helper()
	if gitDiffFont() == nil || gitMonoSource == nil || gitEmojiSource == nil {
		t.Fatal("faces missing")
	}
	face, err := text.NewMultiFace(
		&text.GoTextFace{Source: gitMonoSource, Size: size},
		&text.GoTextFace{Source: gitEmojiSource, Size: size},
	)
	if err != nil {
		t.Fatal(err)
	}
	return face
}
