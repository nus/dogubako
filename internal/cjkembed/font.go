package cjkembed

import (
	"fmt"
	"os"
	"strings"

	"github.com/gogpu/gg/text"
	"github.com/gogpu/ui/plugin"
	"golang.org/x/text/unicode/norm"
)

const maxCollectionFaces = 32

func logFontErr(kind string, err error) {
	fmt.Fprintf(os.Stderr, "dogubako: %s: %v\n", kind, err)
}

func loadFamily(name string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty font")
	}
	return plugin.NewDefaultPluginContext().Assets.LoadFont(name, data)
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

func loadFaceSources(path string) ([]*text.FontSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sources []*text.FontSource
	for i := 0; i < maxCollectionFaces; i++ {
		src, err := text.NewFontSource(data, text.WithCollectionIndex(i))
		if err != nil {
			if i == 0 {
				return nil, err
			}
			break
		}
		sources = append(sources, src)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no faces in %s", path)
	}
	return sources, nil
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

func pickHiraginoFace(sources []*text.FontSource) *text.FontSource {
	var fallback *text.FontSource
	for _, src := range sources {
		if src == nil {
			continue
		}
		fam := src.Name()
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

func openCJKFont(paths []string) (*text.FontSource, string, error) {
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
	return nil, "", fmt.Errorf("cjk font not found")
}

func openHiragino(paths []string) (*text.FontSource, string, error) {
	return openCJKFont(paths)
}

func registerLoaded(kind, family, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		logFontErr(kind, err)
		return
	}
	if err := loadFamily(family, data); err != nil {
		logFontErr(kind, err)
	}
}
