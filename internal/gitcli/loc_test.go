package gitcli

import (
	"context"
	"strings"
	"testing"
)

func TestLocRoundTrip(t *testing.T) {
	cases := []Loc{
		{Dest: "dev", Dir: "/home/me/repo"},
		{Dest: "user@host", Port: "2222", Dir: "/home/me/repo"},
		{Dest: "my host", Dir: "/home/foo:bar/repo"},
		{Dest: "user@host", Dir: "/tmp/a b"},
	}
	for _, loc := range cases {
		got := ParseLoc(loc.Key())
		if got != loc {
			t.Fatalf("ParseLoc(%q) = %#v, want %#v", loc.Key(), got, loc)
		}
	}
}

func TestParseLocLocal(t *testing.T) {
	got := ParseLoc("/Users/me/proj")
	if got.IsRemote() || got.Dir != "/Users/me/proj" {
		t.Fatalf("local = %#v", got)
	}
}

func TestLocKeyCleansDir(t *testing.T) {
	got := (Loc{Dest: "dev", Dir: "/home/me/../me/repo"}).Key()
	want := (Loc{Dest: "dev", Dir: "/home/me/repo"}).Key()
	if got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}

func TestGitCmdRemote(t *testing.T) {
	cmd := gitCmd(context.Background(), Loc{Dest: "dev", Dir: "/work/app"}, "status")
	if len(cmd.Args) == 0 || cmd.Args[0] != "ssh" {
		t.Fatalf("args = %#v", cmd.Args)
	}
	remote := cmd.Args[len(cmd.Args)-1]
	if !strings.Contains(remote, "git") || !strings.Contains(remote, "/work/app") || !strings.Contains(remote, "status") {
		t.Fatalf("remote = %q", remote)
	}
	if cmd.Args[len(cmd.Args)-2] != "dev" {
		t.Fatalf("dest = %q", cmd.Args[len(cmd.Args)-2])
	}
	local := gitCmd(context.Background(), Loc{Dir: "/work/app"}, "status")
	if local.Args[0] != "git" {
		t.Fatalf("local args = %#v", local.Args)
	}
}

func TestLocDisplay(t *testing.T) {
	if got := (Loc{Dest: "dev", Dir: "/work"}).Display(); got != "dev:/work" {
		t.Fatalf("display = %q", got)
	}
	if got := (Loc{Dest: "user@host", Port: "2222", Dir: "/work"}).Display(); got != "user@host:2222:/work" {
		t.Fatalf("display = %q", got)
	}
}
