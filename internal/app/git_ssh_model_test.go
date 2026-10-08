package app

import (
	"testing"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

func TestCanonicalRemotePath(t *testing.T) {
	raw := "ssh:dev::/home/me/../me/repo"
	want := gitcli.Loc{Dest: "dev", Dir: "/home/me/repo"}.Key()
	if got := canonicalPath(raw); got != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
	local := t.TempDir()
	if got := canonicalPath(local); got == "" {
		t.Fatal("local path cleared")
	}
}

func TestSSHConnectRejectsBadDest(t *testing.T) {
	m := &GitModel{}
	m.BeginSSH()
	m.SSHConnect("-oProxyCommand=x", "")
	if m.SSHBusy() {
		t.Fatal("invalid destination started a connection")
	}
	if m.SSHConnected() {
		t.Fatal("connected")
	}
	if m.SSHStatus(i18n.JA) == "" {
		t.Fatal("expected an error")
	}
	m.SSHConnect("dev", "nope")
	if m.SSHBusy() || m.SSHStatus(i18n.EN) == "" {
		t.Fatal("invalid port should be rejected")
	}
}

func TestRemotePathHelpers(t *testing.T) {
	if got := joinRemote("/home/me", "proj"); got != "/home/me/proj" {
		t.Fatalf("join = %q", got)
	}
	if got := parentRemote("/home/me/proj"); got != "/home/me" {
		t.Fatalf("parent = %q", got)
	}
	if got := parentRemote("/"); got != "/" {
		t.Fatalf("root parent = %q", got)
	}
	if got := cleanRemote("/home/me/../me/proj"); got != "/home/me/proj" {
		t.Fatalf("clean = %q", got)
	}
}

func TestTabLabelRemote(t *testing.T) {
	m := &GitModel{}
	m.tabs = []*gitSession{
		{owner: m, path: gitcli.Loc{Dest: "a", Dir: "/home/me/app"}.Key()},
		{owner: m, path: gitcli.Loc{Dest: "b", Dir: "/var/app"}.Key()},
	}
	m.active = 0
	m.gitSession = m.tabs[0]
	if got := m.TabLabel(0); got != "me/app" {
		t.Fatalf("label 0 = %q", got)
	}
	if got := m.TabLabel(1); got != "var/app" {
		t.Fatalf("label 1 = %q", got)
	}
}
