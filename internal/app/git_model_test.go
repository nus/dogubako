package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

func waitGit(t *testing.T, m *GitModel) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		m.Drain()
		if !m.anyBusy() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for git model")
}

func TestSplitMergedBranch(t *testing.T) {
	local, remote, ok := splitMergedBranch("main origin")
	if !ok || local != "main" || remote != "origin" {
		t.Fatalf("main = %q %q ok=%v", local, remote, ok)
	}
	local, remote, ok = splitMergedBranch("feature/foo origin upstream")
	if !ok || local != "feature/foo" || remote != "origin upstream" {
		t.Fatalf("nested = %q %q ok=%v", local, remote, ok)
	}
	if _, _, ok := splitMergedBranch("origin/feature"); ok {
		t.Fatal("a remote-only name is not a merged badge")
	}
}

func TestMergedBranchRefs(t *testing.T) {
	refs, ok := mergedBranchRefs(gitcli.Decoration{Kind: "head", Name: "main origin", HEAD: true}, "abc")
	if !ok || len(refs) != 2 {
		t.Fatalf("refs = %#v ok=%v", refs, ok)
	}
	if refs[0].Name != "main" || refs[0].Remote != "" || !refs[0].Current || refs[0].Hash != "abc" {
		t.Fatalf("local = %#v", refs[0])
	}
	if refs[1].Name != "origin/main" || refs[1].Remote != "origin" || refs[1].LocalName() != "main" {
		t.Fatalf("origin = %#v", refs[1])
	}

	refs, ok = mergedBranchRefs(gitcli.Decoration{Kind: "head", Name: "feature/foo origin upstream"}, "def")
	if !ok || len(refs) != 3 {
		t.Fatalf("multi = %#v ok=%v", refs, ok)
	}
	if refs[0].Name != "feature/foo" || refs[0].Current {
		t.Fatalf("local = %#v", refs[0])
	}
	if refs[1].Name != "origin/feature/foo" || refs[1].Remote != "origin" || refs[1].LocalName() != "feature/foo" {
		t.Fatalf("origin = %#v", refs[1])
	}
	if refs[2].Name != "upstream/feature/foo" || refs[2].Remote != "upstream" {
		t.Fatalf("upstream = %#v", refs[2])
	}
	if _, ok := mergedBranchRefs(gitcli.Decoration{Kind: "remote", Name: "origin/feature"}, "abc"); ok {
		t.Fatal("a remote-only badge is not split")
	}
	if _, ok := mergedBranchRefs(gitcli.Decoration{Kind: "head", Name: "main"}, "abc"); ok {
		t.Fatal("a local-only badge is not split")
	}
}

func TestBadgeHitRegions(t *testing.T) {
	ends := mergedBadgeEnds(300, 100, 120, []int{40, 50}, 10)
	if len(ends) != 3 || ends[0] != 100 || ends[1] != 165 || ends[2] != 300 {
		t.Fatalf("ends = %v", ends)
	}
	refs := []gitcli.Ref{{Name: "main"}, {Name: "origin/main", Remote: "origin"}, {Name: "upstream/main", Remote: "upstream"}}
	got, ok := refAtX(50, refs, ends)
	if !ok || got.Name != "main" {
		t.Fatalf("left = %#v", got)
	}
	got, ok = refAtX(100, refs, ends)
	if !ok || got.Remote != "origin" {
		t.Fatalf("origin = %#v", got)
	}
	got, ok = refAtX(164, refs, ends)
	if !ok || got.Remote != "origin" {
		t.Fatalf("origin edge = %#v", got)
	}
	got, ok = refAtX(165, refs, ends)
	if !ok || got.Remote != "upstream" {
		t.Fatalf("upstream = %#v", got)
	}
	got, ok = refAtX(299, refs, ends)
	if !ok || got.Remote != "upstream" {
		t.Fatalf("right = %#v", got)
	}
}

func TestBranchSwitchRef(t *testing.T) {
	local, ok := branchSwitchRef(gitcli.Decoration{Kind: "head", Name: "main origin", HEAD: true}, "abc")
	if !ok || local.Name != "main" || local.Remote != "" || !local.Current || local.Hash != "abc" {
		t.Fatalf("local = %#v ok=%v", local, ok)
	}
	nested, ok := branchSwitchRef(gitcli.Decoration{Kind: "head", Name: "feature/foo"}, "def")
	if !ok || nested.Name != "feature/foo" || nested.Remote != "" {
		t.Fatalf("nested = %#v ok=%v", nested, ok)
	}
	remote, ok := branchSwitchRef(gitcli.Decoration{Kind: "remote", Name: "origin/feature"}, "ghi")
	if !ok || remote.Name != "origin/feature" || remote.Remote != "origin" || remote.LocalName() != "feature" {
		t.Fatalf("remote = %#v ok=%v", remote, ok)
	}
	tag, ok := branchSwitchRef(gitcli.Decoration{Kind: "tag", Name: "v1.0"}, "abc")
	if !ok || !tag.IsTag() || tag.Name != "v1.0" || tag.Hash != "abc" {
		t.Fatalf("tag = %#v ok=%v", tag, ok)
	}
	if _, ok := branchSwitchRef(gitcli.Decoration{Kind: "remote", Name: "origin/HEAD"}, "abc"); ok {
		t.Fatal("remote HEAD should not switch")
	}
}

func TestRefDecorations(t *testing.T) {
	got := refDecorations([]gitcli.Decoration{
		{Kind: "head", Name: "main", HEAD: true},
		{Kind: "remote", Name: "origin/main"},
		{Kind: "tag", Name: "v1.0"},
		{Kind: "head", Name: "HEAD"},
		{Kind: "other", Name: "HEAD", HEAD: true},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d %#v", len(got), got)
	}
	if got[0].Name != "main origin" || !got[0].HEAD || got[0].Kind != "head" {
		t.Fatalf("head = %#v", got[0])
	}
	if got[1].Name != "v1.0" || got[1].Kind != "tag" {
		t.Fatalf("tag = %#v", got[1])
	}
	if len(refDecorations(nil)) != 0 {
		t.Fatal("empty")
	}

	separate := refDecorations([]gitcli.Decoration{
		{Kind: "head", Name: "main"},
		{Kind: "remote", Name: "origin/main"},
		{Kind: "remote", Name: "upstream/main"},
		{Kind: "remote", Name: "origin/feature"},
		{Kind: "head", Name: "feature/foo"},
		{Kind: "remote", Name: "origin/feature/foo"},
	})
	if len(separate) != 3 {
		t.Fatalf("separate = %#v", separate)
	}
	if separate[0].Name != "main origin upstream" || separate[0].Kind != "head" {
		t.Fatalf("main = %#v", separate[0])
	}
	if separate[1].Name != "feature/foo origin" {
		t.Fatalf("feature = %#v", separate[1])
	}
	if separate[2].Name != "origin/feature" || separate[2].Kind != "remote" {
		t.Fatalf("unmatched = %#v", separate[2])
	}
}

func TestGitModelOpenCommitAmend(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "first")

	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.Open(dir)
	waitGit(t, &m)
	if !m.HasRepo() {
		t.Fatalf("status = %s", m.StatusText(i18n.EN))
	}
	if m.Snapshot().Branch != "main" {
		t.Fatalf("branch = %s", m.Snapshot().Branch)
	}
	if len(m.Snapshot().Graph) != 1 {
		t.Fatalf("graph = %d", len(m.Snapshot().Graph))
	}
	if m.Selected() == "" {
		t.Fatal("expected HEAD selected")
	}

	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Reload()
	waitGit(t, &m)
	m.SetDraft("second")
	m.SetStageAll(true)
	if !m.CanCommit() {
		t.Fatal("expected can commit")
	}
	m.DoCommit()
	waitGit(t, &m)
	if len(m.Snapshot().Commits) != 2 {
		t.Fatalf("commits = %d status=%s", len(m.Snapshot().Commits), m.StatusText(i18n.EN))
	}
	m.SetDraft("second amended")
	m.DoAmend()
	waitGit(t, &m)
	if m.Snapshot().Commits[0].Subject != "second amended" {
		t.Fatalf("subject = %q status=%s", m.Snapshot().Commits[0].Subject, m.StatusText(i18n.EN))
	}
}

func TestGitModelToggleRemoteReloads(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "first")
	remote := t.TempDir()
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "push", "-u", "origin", "main")

	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.Open(dir)
	waitGit(t, &m)
	if len(m.Snapshot().Remotes) != 1 {
		t.Fatalf("remotes = %#v", m.Snapshot().Remotes)
	}
	if !m.RemoteVisible("origin") {
		t.Fatal("origin should start visible")
	}
	m.SetRemoteVisible("origin", false)
	waitGit(t, &m)
	if m.RemoteVisible("origin") {
		t.Fatal("origin should be hidden")
	}
	for _, c := range m.Snapshot().Commits {
		for _, d := range c.Decorations {
			if d.Kind == "remote" {
				t.Fatalf("hidden decoration %#v", d)
			}
		}
	}
}

func TestGitPollReflectsExternalChanges(t *testing.T) {
	requireGitBin(t)
	dir := initRepo(t, "a")
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.Open(dir)
	waitGit(t, &m)

	gitRun(t, dir, "checkout", "-b", "feature")
	waitGitView(t, &m, func() bool {
		snap := m.Snapshot()
		if snap.Branch != "feature" {
			return false
		}
		for _, ref := range snap.Locals {
			if ref.Name == "feature" && ref.Current {
				return true
			}
		}
		return false
	})

	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitGitView(t, &m, func() bool {
		snap := m.Snapshot()
		return snap.Status.Dirty() && len(snap.Graph) > 1 && snap.Graph[0].Commit.Hash == gitcli.Uncommitted && snap.Graph[1].Commit.Hash == snap.HEAD
	})
}

func waitGitView(t *testing.T, m *GitModel, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		m.lastPoll = time.Time{}
		m.PollStatus()
		m.Drain()
		if done() && !m.anyBusy() && m.pendingStatus == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for git view: branch=%s dirty=%v graph=%s", m.Snapshot().Branch, m.Snapshot().Status.Dirty(), graphHeads(m.Snapshot()))
}

func graphHeads(snap gitcli.Snapshot) string {
	parts := make([]string, len(snap.Graph))
	for i, row := range snap.Graph {
		parts[i] = row.Commit.Hash
	}
	return strings.Join(parts, " ")
}

func TestGitTabsKeepRepos(t *testing.T) {
	requireGitBin(t)
	dirA := initRepo(t, "a")
	dirB := initRepo(t, "b")

	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.Open(dirA)
	waitGit(t, &m)
	m.Open(dirB)
	waitGit(t, &m)
	if m.TabCount() != 2 {
		t.Fatalf("tabs = %d", m.TabCount())
	}
	if m.ActiveTab() != 1 || filepath.Base(m.Path()) != filepath.Base(dirB) {
		t.Fatalf("active = %d path = %s", m.ActiveTab(), m.Path())
	}
	m.Open(dirA)
	waitGit(t, &m)
	if m.TabCount() != 2 || m.ActiveTab() != 0 || filepath.Base(m.Path()) != filepath.Base(dirA) {
		t.Fatalf("reuse tabs=%d active=%d path=%s", m.TabCount(), m.ActiveTab(), m.Path())
	}
	m.SelectTab(1)
	if filepath.Base(m.Path()) != filepath.Base(dirB) {
		t.Fatalf("switched path = %s", m.Path())
	}
	m.NewTab()
	if m.TabCount() != 3 || m.Path() != "" || m.TabLabel(m.ActiveTab()) != "" {
		t.Fatalf("new tab count=%d path=%q label=%q", m.TabCount(), m.Path(), m.TabLabel(m.ActiveTab()))
	}
	m.CloseTab(m.ActiveTab())
	if m.TabCount() != 2 {
		t.Fatalf("after close tabs = %d", m.TabCount())
	}

	var again GitModel
	again.EnsureLoaded()
	waitGit(t, &again)
	if again.TabCount() != 2 {
		t.Fatalf("restored tabs = %d", again.TabCount())
	}
	if filepath.Base(again.tabs[0].path) != filepath.Base(dirA) || filepath.Base(again.tabs[1].path) != filepath.Base(dirB) {
		t.Fatalf("restored paths %q %q", again.tabs[0].path, again.tabs[1].path)
	}
	if again.ActiveTab() != 1 {
		t.Fatalf("restored active = %d", again.ActiveTab())
	}
}

func initRepo(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", name+".txt")
	gitRun(t, dir, "commit", "-m", name)
	return dir
}

func TestParentLinksMapShortHashes(t *testing.T) {
	var body gitDetailBody
	body.setParents([]string{"0123456789abcdef", "abcdef0123456789"})
	if body.parentsText.Value() != "0123456  abcdef0" {
		t.Fatalf("text = %q", body.parentsText.Value())
	}
	if len(body.parentRanges) != 2 || len(body.parentHashes) != 2 {
		t.Fatalf("ranges %d hashes %d", len(body.parentRanges), len(body.parentHashes))
	}
	if body.parentRanges[0] != (basicwidget.TextRange{StartInBytes: 0, EndInBytes: 7}) {
		t.Fatalf("first range = %+v", body.parentRanges[0])
	}
	if body.parentRanges[1] != (basicwidget.TextRange{StartInBytes: 9, EndInBytes: 16}) {
		t.Fatalf("second range = %+v", body.parentRanges[1])
	}
	if body.parentHashes[1] != "abcdef0123456789" {
		t.Fatalf("hash = %q", body.parentHashes[1])
	}
	body.setParents(nil)
	if body.showParents || body.parentsText.Value() != "" || len(body.parentHashes) != 0 {
		t.Fatal("parents not cleared")
	}
}

func TestSelectRefRevealsBranchTip(t *testing.T) {
	m := &GitModel{gitSession: &gitSession{snap: gitcli.Snapshot{
		Locals: []gitcli.Ref{
			{Name: "main", Hash: "aaa"},
			{Name: "feature", Hash: "bbb"},
		},
		RemoteBranches: []gitcli.Ref{
			{Name: "origin/feature", Remote: "origin", Hash: "ccc"},
		},
		Tags: []gitcli.Ref{
			{Name: "v1.0", Hash: "ddd"},
		},
	}}}
	var tool GitTool
	tool.selectRef(m, "local:feature")
	if m.Selected() != "bbb" || m.TakeReveal() != "bbb" {
		t.Fatalf("local selected=%q reveal=%q", m.Selected(), m.reveal)
	}
	tool.selectRef(m, "remote:origin/feature")
	if m.Selected() != "ccc" || m.TakeReveal() != "ccc" {
		t.Fatalf("remote selected=%q reveal=%q", m.Selected(), m.reveal)
	}
	tool.selectRef(m, "local:feature")
	if m.Selected() != "bbb" || m.TakeReveal() != "bbb" {
		t.Fatal("selecting the branch again should scroll to its tip")
	}
	tool.selectRef(m, "tag:v1.0")
	if m.Selected() != "ddd" || m.TakeReveal() != "ddd" {
		t.Fatalf("tag selected=%q reveal=%q", m.Selected(), m.reveal)
	}
	tool.selectRef(m, "local:missing")
	if m.Selected() != "ddd" || m.TakeReveal() != "" {
		t.Fatal("unknown branch should leave the selection")
	}
}

func TestRevealCommitSelectsHash(t *testing.T) {
	m := &gitSession{}
	m.RevealCommit("")
	m.RevealCommit(gitcli.Uncommitted)
	if m.Selected() != "" || m.TakeReveal() != "" {
		t.Fatal("empty hash selected")
	}
	m.RevealCommit("abc123")
	if m.Selected() != "abc123" {
		t.Fatalf("selected = %q", m.Selected())
	}
	if got := m.TakeReveal(); got != "abc123" {
		t.Fatalf("reveal = %q", got)
	}
	if m.TakeReveal() != "" {
		t.Fatal("reveal not consumed")
	}
	m.RevealCommit("abc123")
	if m.Selected() != "abc123" || m.TakeReveal() != "abc123" {
		t.Fatal("same hash should stay selected and reveal again")
	}
}

func requireGitBin(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
