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

type cjkFile struct {
	data  []byte
	index int
	path  string
	name  string
}

func loadCJKFile(path string) (*cjkFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	index, name, err := pickCollectionFace(data)
	if err != nil {
		return nil, err
	}
	return &cjkFile{data: data, index: index, path: path, name: name}, nil
}

func pickCollectionFace(data []byte) (int, string, error) {
	var fallback int
	var fallbackName string
	found := false
	for i := 0; i < maxCollectionFaces; i++ {
		src, err := text.NewFontSource(data, text.WithCollectionIndex(i))
		if err != nil {
			if i == 0 {
				return 0, "", err
			}
			break
		}
		fam := src.Name()
		if skipHiraginoFamily(fam) {
			continue
		}
		if preferHiraginoFamily(fam) {
			return i, fam, nil
		}
		if !found {
			fallback = i
			fallbackName = fam
			found = true
		}
	}
	if found {
		return fallback, fallbackName, nil
	}
	return 0, "", fmt.Errorf("no usable face")
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

func openCJKFont(paths []string) (*cjkFile, error) {
	var firstErr error
	for _, path := range expandPathNorms(paths) {
		file, err := loadCJKFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		return file, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("cjk font not found")
}

func openHiragino(paths []string) (*text.FontSource, string, error) {
	file, err := openCJKFont(paths)
	if err != nil {
		return nil, "", err
	}
	src, err := text.NewFontSource(file.data, text.WithCollectionIndex(file.index))
	if err != nil {
		return nil, "", err
	}
	return src, file.path, nil
}

func registerLoaded(file *cjkFile, family string) error {
	face, err := extractCollectionFace(file.data, file.index)
	if err != nil {
		face = file.data
	}
	return loadFamily(family, face)
}
