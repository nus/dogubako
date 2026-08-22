package cjkembed

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/unicode/norm"
)

const hiraginoFontDir = "/System/Library/Fonts"

// defaultHiraginoPaths is the usual macOS location of Hiragino Sans, preferred
// weight first. Each weight is a separate .ttc.
func defaultHiraginoPaths() []string {
	names := []string{
		"ヒラギノ角ゴシック W3.ttc",
		"ヒラギノ角ゴシック W4.ttc",
		"ヒラギノ角ゴシック W5.ttc",
		"ヒラギノ角ゴシック W6.ttc",
		"ヒラギノ角ゴシック W2.ttc",
	}
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = filepath.Join(hiraginoFontDir, name)
	}
	return out
}

func expandPathNorms(paths []string) []string {
	out := make([]string, 0, len(paths)*2)
	seen := make(map[string]struct{}, len(paths)*2)
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range paths {
		add(p)
		add(norm.NFC.String(p))
		add(norm.NFD.String(p))
	}
	return out
}

func loadFaceSources(path string) ([]*text.GoTextFaceSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(data)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttc", ".otc":
		return text.NewGoTextFaceSourcesFromCollection(r)
	default:
		src, err := text.NewGoTextFaceSource(r)
		if err != nil {
			return nil, err
		}
		return []*text.GoTextFaceSource{src}, nil
	}
}

func skipHiraginoFamily(family string) bool {
	fam := strings.ToLower(family)
	switch {
	case strings.Contains(fam, "mincho"), strings.Contains(fam, "明朝"):
		return true
	case strings.Contains(fam, "gb"), strings.Contains(fam, "cns"):
		return true
	case strings.Contains(fam, " tc"), strings.HasSuffix(fam, "tc"):
		return true
	default:
		return false
	}
}

func preferHiraginoFamily(family string) bool {
	fam := strings.ToLower(family)
	return strings.Contains(fam, "sans") || strings.Contains(fam, "角ゴ")
}

func pickHiraginoFace(sources []*text.GoTextFaceSource) *text.GoTextFaceSource {
	var fallback *text.GoTextFaceSource
	for _, src := range sources {
		if src == nil {
			continue
		}
		fam := src.Metadata().Family
		if skipHiraginoFamily(fam) {
			continue
		}
		if preferHiraginoFamily(fam) {
			return src
		}
		if fallback == nil {
			fallback = src
		}
	}
	if fallback != nil {
		return fallback
	}
	if len(sources) > 0 {
		return sources[0]
	}
	return nil
}

func openHiragino(paths []string) (*text.GoTextFaceSource, string, error) {
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
		face := pickHiraginoFace(sources)
		if face == nil {
			continue
		}
		return face, path, nil
	}
	if firstErr != nil {
		return nil, "", firstErr
	}
	return nil, "", fmt.Errorf("hiragino font not found")
}
