package sshcfg

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseHosts(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := "" +
		"# comment\n" +
		"Host alpha\n" +
		"  HostName 10.0.0.1\n" +
		"Host *.example.com beta\n" +
		"Host gamma delta\n" +
		"Host = quoted\n" +
		"Host \"my host\"\n" +
		"Include config.d/*\n" +
		"Include missing\n" +
		"Match host *\n" +
		"  Host extra-not-a-host\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	extra := "Host fromextra\nHost alpha\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config.d", "extra"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseFile(filepath.Join(sshDir, "config"), home, 0, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "beta", "gamma", "delta", "quoted", "my host", "fromextra", "extra-not-a-host"}
	if !slices.Equal(got, want) {
		t.Fatalf("hosts = %#v", got)
	}
}

func TestKeywordEquals(t *testing.T) {
	key, args := keyword([]string{"Host=foo", "bar"})
	if key != "Host" || !slices.Equal(args, []string{"foo", "bar"}) {
		t.Fatalf("key=%q args=%#v", key, args)
	}
}
