package cjkembed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func TestDefaultHiraginoPaths(t *testing.T) {
	paths := defaultHiraginoPaths()
	if len(paths) == 0 {
		t.Fatal("no candidate paths")
	}
	if !strings.HasSuffix(paths[0], "ヒラギノ角ゴシック W3.ttc") {
		t.Fatalf("first path = %q", paths[0])
	}
	for _, p := range paths {
		if filepath.Dir(p) != hiraginoFontDir {
			t.Fatalf("dir = %q", filepath.Dir(p))
		}
	}
}

func TestExpandPathNorms(t *testing.T) {
	p := filepath.Join(hiraginoFontDir, "ヒラギノ角ゴシック W3.ttc")
	got := expandPathNorms([]string{p})
	nfc := norm.NFC.String(p)
	nfd := norm.NFD.String(p)
	if nfc == nfd {
		t.Fatal("expected NFC and NFD to differ for ヒラギノ")
	}
	var haveNFC, haveNFD bool
	for _, g := range got {
		if g == nfc {
			haveNFC = true
		}
		if g == nfd {
			haveNFD = true
		}
	}
	if !haveNFC || !haveNFD {
		t.Fatalf("norms = %q", got)
	}
}

func TestOpenHiraginoMissing(t *testing.T) {
	_, _, err := openHiragino([]string{filepath.Join(t.TempDir(), "missing.ttc")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAndPickTTF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GoRegular.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	src, gotPath, err := openHiragino([]string{
		filepath.Join(dir, "missing.ttf"),
		path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q", gotPath)
	}
	if src == nil || src.Metadata().Family == "" {
		t.Fatalf("metadata = %+v", src.Metadata())
	}
}

func TestSkipAndPreferFamily(t *testing.T) {
	if !skipHiraginoFamily("Hiragino Sans GB") {
		t.Fatal("expected to skip GB")
	}
	if !skipHiraginoFamily("Hiragino Mincho ProN") {
		t.Fatal("expected to skip Mincho")
	}
	if skipHiraginoFamily("Hiragino Sans") {
		t.Fatal("should not skip Hiragino Sans")
	}
	if !preferHiraginoFamily("Hiragino Sans") {
		t.Fatal("expected to prefer Sans")
	}
	if !preferHiraginoFamily("ヒラギノ角ゴシック") {
		t.Fatal("expected to prefer 角ゴ")
	}
}

func TestJapanesePrimary(t *testing.T) {
	if japanesePrimary(nil) {
		t.Fatal("empty")
	}
	ja := language.MustParse("ja-JP")
	en := language.MustParse("en-US")
	if !japanesePrimary([]language.Tag{ja, en}) {
		t.Fatal("ja first")
	}
	if japanesePrimary([]language.Tag{en, ja}) {
		t.Fatal("en first")
	}
}
