package sshremote

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"":    "''",
		"a":   "'a'",
		"a'b": `'a'\''b'`,
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	if !ValidDest("dev") || !ValidDest("user@host") || !ValidDest("my host") {
		t.Fatal("expected valid destinations")
	}
	if ValidDest("") || ValidDest("-oProxyCommand=x") || ValidDest("a\nb") {
		t.Fatal("expected invalid destinations")
	}
	if !ValidPort("") || !ValidPort("22") || ValidPort("0") || ValidPort("65536") || ValidPort("x") {
		t.Fatal("port validation")
	}
}

func TestCommandArgs(t *testing.T) {
	cmd := Command(context.Background(), Target{Dest: "dev", Port: "2222"}, "true")
	if cmd.Args[0] != "ssh" || cmd.Args[len(cmd.Args)-2] != "dev" || cmd.Args[len(cmd.Args)-1] != "true" {
		t.Fatalf("args = %#v", cmd.Args)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, part := range []string{"BatchMode=yes", "StrictHostKeyChecking=accept-new", "ConnectTimeout=15", "-p 2222"} {
		if !strings.Contains(joined, part) {
			t.Fatalf("args missing %q: %#v", part, cmd.Args)
		}
	}
	plain := Command(context.Background(), Target{Dest: "alias"}, "true")
	if slices.Contains(plain.Args, "-p") {
		t.Fatalf("config host should not pass -p: %#v", plain.Args)
	}
}

func TestParseList(t *testing.T) {
	entries, repo, err := ParseList("REPO\n/home/me/repo/sub\n/home/me/repo/.hidden\n")
	if err != nil {
		t.Fatal(err)
	}
	if !repo {
		t.Fatal("expected repo")
	}
	if len(entries) != 2 || entries[0].Name != "sub" || entries[1].Name != ".hidden" {
		t.Fatalf("entries = %#v", entries)
	}
	if _, _, err := ParseList("DIR\n"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseList("ERR not a directory\n"); err == nil {
		t.Fatal("expected error")
	}
}
