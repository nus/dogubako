package i18n

import (
	"strings"
	"testing"
	"unicode"
)

func TestCatalogsComplete(t *testing.T) {
	ja := catalogs[JA]
	en := catalogs[EN]
	if len(ja) == 0 || len(en) == 0 {
		t.Fatal("empty catalog")
	}
	for key, val := range ja {
		if strings.TrimSpace(val) == "" {
			t.Errorf("empty JA string for %s", key)
		}
		if _, ok := en[key]; !ok {
			t.Errorf("missing EN: %s", key)
		}
	}
	for key, val := range en {
		if strings.TrimSpace(val) == "" {
			t.Errorf("empty EN string for %s", key)
		}
		if _, ok := ja[key]; !ok {
			t.Errorf("missing JA: %s", key)
		}
	}
}

func TestFormatVerbsMatch(t *testing.T) {
	for key, ja := range catalogs[JA] {
		en := catalogs[EN][key]
		if verbs(ja) != verbs(en) {
			t.Errorf("%s verbs: ja %q vs en %q", key, ja, en)
		}
	}
}

func TestT(t *testing.T) {
	if got := T(JA, AppTitle); got != "道具箱" {
		t.Fatalf("ja title = %q", got)
	}
	if got := T(EN, AppTitle); got != "Dogubako" {
		t.Fatalf("en title = %q", got)
	}
	if got := T(JA, StatusLoaded, "a.png", 10, 20); got != "a.png を読み込みました（10×20）" {
		t.Fatalf("ja loaded = %q", got)
	}
	if got := T(EN, StatusLoaded, "a.png", 10, 20); got != "Loaded a.png (10×20)" {
		t.Fatalf("en loaded = %q", got)
	}
	if got := T("fr", AppTitle); got != "道具箱" {
		t.Fatalf("unknown lang should fall back: %q", got)
	}
}

func TestParseLocale(t *testing.T) {
	cases := map[string]Lang{
		"ja_JP.UTF-8": JA,
		"ja-JP":       JA,
		"en_US.UTF-8": EN,
		"en":          EN,
		"C":           JA,
		"":            JA,
		"fr_FR":       JA,
	}
	for in, want := range cases {
		if got := ParseLocale(in); got != want {
			t.Errorf("ParseLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetect(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_GB.UTF-8")
	if got := Detect(); got != EN {
		t.Fatalf("detect en = %q", got)
	}
	t.Setenv("LANG", "ja_JP.UTF-8")
	if got := Detect(); got != JA {
		t.Fatalf("detect ja = %q", got)
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	restore := OverrideUserConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)

	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ja_JP.UTF-8")
	if got := Load(); got != JA {
		t.Fatalf("unsaved default = %q", got)
	}
	if err := Save(EN); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got != EN {
		t.Fatalf("saved en = %q", got)
	}
	if err := Save(JA); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got != JA {
		t.Fatalf("saved ja = %q", got)
	}
}

func verbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && !unicode.IsLetter(rune(s[j])) && s[j] != '%' {
			j++
		}
		if j < len(s) {
			b.WriteByte('%')
			b.WriteByte(s[j])
		}
	}
	return b.String()
}
