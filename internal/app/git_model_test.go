package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLocalBranchByName(t *testing.T) {
	locals := []gitcli.Ref{
		{Name: "main", Current: true},
		{Name: "feature/foo"},
	}
	got, ok := localBranchByName(locals, "feature/foo")
	if !ok || got.Name != "feature/foo" || got.Current {
		t.Fatalf("nested = %#v ok=%v", got, ok)
	}
	got, ok = localBranchByName(locals, "main")
	if !ok || !got.Current {
		t.Fatalf("main = %#v ok=%v", got, ok)
	}
	if _, ok := localBranchByName(locals, "feature"); ok {
		t.Fatal("partial name matched")
	}
	if _, ok := localBranchByName(locals, ""); ok {
		t.Fatal("empty name matched")
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
	if m.Draft() != "" {
		t.Fatalf("draft = %q", m.Draft())
	}
	m.SetDraft("second amended")
	m.DoAmend()
	waitGit(t, &m)
	if m.Snapshot().Commits[0].Subject != "second amended" {
		t.Fatalf("subject = %q status=%s", m.Snapshot().Commits[0].Subject, m.StatusText(i18n.EN))
	}
}

func TestGitModelHistoryPages(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	for i := 0; i < 5; i++ {
		gitCommitEmpty(t, dir, "c"+string(rune('0'+i)), "2020-01-01 00:00:0"+string(rune('0'+i))+" +0000")
	}

	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.ensure()
	m.histPage = 2
	m.Open(dir)
	waitGit(t, &m)
	if len(m.Snapshot().Commits) != 2 || !m.HasMoreHistory() {
		t.Fatalf("first page commits=%d more=%v status=%s", len(m.Snapshot().Commits), m.HasMoreHistory(), m.StatusText(i18n.EN))
	}
	if m.Snapshot().Commits[0].Subject != "c4" {
		t.Fatalf("newest = %q", m.Snapshot().Commits[0].Subject)
	}
	rev := m.GraphRev()
	if rev == 0 {
		t.Fatal("opening a repository did not publish a graph")
	}
	m.SelectCommit(m.Snapshot().Commits[1].Hash)
	if m.GraphRev() != rev {
		t.Fatal("selecting a commit replaced the graph")
	}
	m.LoadMoreHistory()
	waitGit(t, &m)
	if len(m.Snapshot().Commits) != 4 || !m.HasMoreHistory() {
		t.Fatalf("second page commits=%d more=%v", len(m.Snapshot().Commits), m.HasMoreHistory())
	}
	if m.GraphRev() == rev {
		t.Fatal("the next page did not publish a new graph")
	}
	if m.Snapshot().Commits[0].Subject != "c4" || m.Snapshot().Commits[3].Subject != "c1" {
		t.Fatalf("subjects = %q … %q", m.Snapshot().Commits[0].Subject, m.Snapshot().Commits[3].Subject)
	}
	m.LoadMoreHistory()
	waitGit(t, &m)
	if len(m.Snapshot().Commits) != 5 || m.HasMoreHistory() {
		t.Fatalf("end commits=%d more=%v status=%s", len(m.Snapshot().Commits), m.HasMoreHistory(), m.StatusText(i18n.EN))
	}
	m.LoadMoreHistory()
	waitGit(t, &m)
	if len(m.Snapshot().Commits) != 5 || m.HasMoreHistory() {
		t.Fatalf("extra load commits=%d more=%v", len(m.Snapshot().Commits), m.HasMoreHistory())
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

func TestOpenSubmoduleKeepsTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	remote := initRepo(t, "lib")
	parent := initRepo(t, "app")
	gitRun(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", "--", remote, "vendor/lib")
	gitRun(t, parent, "commit", "-m", "add submodule")

	var m GitModel
	m.Open(parent)
	waitGit(t, &m)
	if len(m.Snapshot().Submodules) != 1 || m.Snapshot().Submodules[0].Path != "vendor/lib" {
		t.Fatalf("submodules = %#v", m.Snapshot().Submodules)
	}
	m.OpenSubmodule(m.Snapshot().Submodules[0].Path)
	waitGit(t, &m)
	if m.TabCount() != 2 {
		t.Fatalf("tabs = %d path = %s parent = %s", m.TabCount(), m.Path(), canonicalPath(parent))
	}
	if canonicalPath(m.Path()) == canonicalPath(parent) {
		t.Fatalf("active path collapsed to parent %s", m.Path())
	}
	if !strings.HasSuffix(filepath.ToSlash(m.Path()), "vendor/lib") {
		t.Fatalf("active path = %s", m.Path())
	}
}

func TestOpenUninitializedSubmoduleKeepsTab(t *testing.T) {
	requireGitBin(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	lib := initRepo(t, "lib")
	parent := initRepo(t, "app")
	gitRun(t, parent, "config", "-f", ".gitmodules", "submodule.lib.path", "vendor/lib")
	gitRun(t, parent, "config", "-f", ".gitmodules", "submodule.lib.url", lib)
	gitRun(t, parent, "add", ".gitmodules")
	hash := strings.TrimSpace(gitOutput(t, lib, "rev-parse", "HEAD"))
	gitRun(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+hash+",vendor/lib")
	gitRun(t, parent, "commit", "-m", "add submodule")
	if err := os.MkdirAll(filepath.Join(parent, "vendor", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}

	var m GitModel
	m.Open(parent)
	waitGit(t, &m)
	m.OpenSubmodule("vendor/lib")
	waitGit(t, &m)
	if m.TabCount() != 2 {
		t.Fatalf("tabs = %d path = %s err = %s", m.TabCount(), m.Path(), m.StatusText(i18n.EN))
	}
	if !strings.HasSuffix(filepath.ToSlash(m.Path()), "vendor/lib") {
		t.Fatalf("active path = %s", m.Path())
	}
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

func TestGitTabStep(t *testing.T) {
	right, ok := gitTabStep(true, true, false, false, false)
	if !ok || right != 1 {
		t.Fatalf("ctrl+tab = %d ok=%v", right, ok)
	}
	left, ok := gitTabStep(true, true, true, false, false)
	if !ok || left != -1 {
		t.Fatalf("ctrl+shift+tab = %d ok=%v", left, ok)
	}
	for _, c := range []struct {
		name                     string
		tab, control, shift, alt bool
		command                  bool
	}{
		{name: "tab alone", tab: true},
		{name: "shift+tab", tab: true, shift: true},
		{name: "alt", tab: true, control: true, alt: true},
		{name: "shift+alt", tab: true, control: true, shift: true, alt: true},
		{name: "command", tab: true, control: true, command: true},
		{name: "shift+command", tab: true, control: true, shift: true, command: true},
		{name: "control held", control: true},
		{name: "command alone", tab: true, command: true},
	} {
		if step, ok := gitTabStep(c.tab, c.control, c.shift, c.alt, c.command); ok {
			t.Fatalf("%s stepped %d", c.name, step)
		}
	}
}

func TestSelectNeighborWrapsInTabOrder(t *testing.T) {
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.tabs = []*gitSession{
		{path: "/repo-a"},
		{path: "/repo-b"},
		{path: "/repo-c"},
	}
	m.active = 2
	m.SelectNeighbor(1)
	if m.ActiveTab() != 0 || m.Path() != "/repo-a" {
		t.Fatalf("right wrap active=%d path=%q", m.ActiveTab(), m.Path())
	}
	m.SelectNeighbor(1)
	if m.ActiveTab() != 1 || m.Path() != "/repo-b" {
		t.Fatalf("right active=%d path=%q", m.ActiveTab(), m.Path())
	}
	m.SelectNeighbor(-1)
	if m.ActiveTab() != 0 || m.Path() != "/repo-a" {
		t.Fatalf("left active=%d path=%q", m.ActiveTab(), m.Path())
	}
	m.SelectNeighbor(-1)
	if m.ActiveTab() != 2 || m.Path() != "/repo-c" {
		t.Fatalf("left wrap active=%d path=%q", m.ActiveTab(), m.Path())
	}
	m.SelectNeighbor(0)
	if m.ActiveTab() != 2 || m.TabCount() != 3 {
		t.Fatalf("zero step active=%d count=%d", m.ActiveTab(), m.TabCount())
	}
	if m.tabs[0].path != "/repo-a" || m.tabs[1].path != "/repo-b" || m.tabs[2].path != "/repo-c" {
		t.Fatalf("order = %q %q %q", m.tabs[0].path, m.tabs[1].path, m.tabs[2].path)
	}
	paths, active, _ := loadGitTabs()
	if active != 2 || len(paths) != 3 || paths[0] != "/repo-a" || paths[1] != "/repo-b" || paths[2] != "/repo-c" {
		t.Fatalf("saved paths=%q active=%d", paths, active)
	}

	m.tabs[0].pendingLoad = make(chan gitLoadResult)
	m.SelectNeighbor(1)
	if m.ActiveTab() != 0 || m.Path() != "/repo-a" || !m.Busy() {
		t.Fatalf("busy active=%d path=%q busy=%v", m.ActiveTab(), m.Path(), m.Busy())
	}

	var one GitModel
	one.tabs = []*gitSession{{path: "/only"}}
	one.SelectNeighbor(1)
	one.SelectNeighbor(-1)
	if one.ActiveTab() != 0 || one.TabCount() != 1 || one.Path() != "/only" {
		t.Fatalf("single active=%d count=%d path=%q", one.ActiveTab(), one.TabCount(), one.Path())
	}
}

func TestNewTabReusesIdleEmpty(t *testing.T) {
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.ensure()
	m.tabs[0].path = "/repo-a"
	m.NewTab()
	if m.TabCount() != 2 || m.ActiveTab() != 1 || m.Path() != "" {
		t.Fatalf("append count=%d active=%d path=%q", m.TabCount(), m.ActiveTab(), m.Path())
	}
	paths, active, _ := loadGitTabs()
	if len(paths) != 2 || active != 1 || paths[0] != "/repo-a" || paths[1] != "" {
		t.Fatalf("saved paths=%q active=%d", paths, active)
	}

	m.SelectTab(0)
	m.NewTab()
	if m.TabCount() != 2 || m.ActiveTab() != 1 || m.Path() != "" {
		t.Fatalf("reuse count=%d active=%d path=%q", m.TabCount(), m.ActiveTab(), m.Path())
	}
	m.NewTab()
	if m.TabCount() != 2 || m.ActiveTab() != 1 {
		t.Fatalf("stay count=%d active=%d", m.TabCount(), m.ActiveTab())
	}

	m.tabs[1].pendingLoad = make(chan gitLoadResult)
	m.SelectTab(0)
	m.NewTab()
	if m.TabCount() != 3 || m.ActiveTab() != 2 || m.Path() != "" {
		t.Fatalf("busy count=%d active=%d path=%q", m.TabCount(), m.ActiveTab(), m.Path())
	}
	paths, active, _ = loadGitTabs()
	if len(paths) != 3 || active != 2 || paths[2] != "" {
		t.Fatalf("busy saved paths=%q active=%d", paths, active)
	}
}

func TestGitRecentRepos(t *testing.T) {
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
	if got := m.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(dirB) || got[1] != canonicalPath(dirA) {
		t.Fatalf("recent = %v", got)
	}
	m.Open(dirA)
	if got := m.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(dirA) || got[1] != canonicalPath(dirB) {
		t.Fatalf("reopen recent = %v", got)
	}
	m.CloseTab(0)
	m.CloseTab(0)
	if m.Path() != "" {
		t.Fatalf("path = %s", m.Path())
	}
	if got := m.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(dirA) {
		t.Fatalf("after close recent = %v", got)
	}

	var again GitModel
	again.EnsureLoaded()
	waitGit(t, &again)
	if got := again.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(dirA) || got[1] != canonicalPath(dirB) {
		t.Fatalf("restored recent = %v", got)
	}
}

func TestOpenRecentClosesEmptyTab(t *testing.T) {
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
	m.Open(dirA)
	if m.TabCount() != 2 || m.ActiveTab() != 0 {
		t.Fatalf("repo tab switch count=%d active=%d", m.TabCount(), m.ActiveTab())
	}

	m.NewTab()
	if m.TabCount() != 3 || m.Path() != "" {
		t.Fatalf("new tab count=%d path=%q", m.TabCount(), m.Path())
	}
	m.Open(dirA)
	if m.TabCount() != 2 || m.ActiveTab() != 0 || filepath.Base(m.Path()) != filepath.Base(dirA) {
		t.Fatalf("after recent jump count=%d active=%d path=%s", m.TabCount(), m.ActiveTab(), m.Path())
	}
	if filepath.Base(m.tabs[1].path) != filepath.Base(dirB) {
		t.Fatalf("kept path = %s", m.tabs[1].path)
	}

	if err := saveGitTabs([]string{"", dirB}, 0, nil); err != nil {
		t.Fatal(err)
	}
	var leading GitModel
	leading.EnsureLoaded()
	waitGit(t, &leading)
	if leading.TabCount() != 2 || leading.Path() != "" {
		t.Fatalf("leading empty count=%d path=%q", leading.TabCount(), leading.Path())
	}
	leading.Open(dirB)
	if leading.TabCount() != 1 || leading.ActiveTab() != 0 || filepath.Base(leading.Path()) != filepath.Base(dirB) {
		t.Fatalf("leading close count=%d active=%d path=%s", leading.TabCount(), leading.ActiveTab(), leading.Path())
	}
}

func TestGitRecentSeedsFromTabs(t *testing.T) {
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	a := filepath.Join(cfg, "a")
	b := filepath.Join(cfg, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveGitTabs([]string{a, b}, 1, nil); err != nil {
		t.Fatal(err)
	}
	var m GitModel
	m.EnsureLoaded()
	if got := m.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(b) || got[1] != canonicalPath(a) {
		t.Fatalf("recent = %v", got)
	}
	waitGit(t, &m)
	if got := m.RecentPaths(); len(got) != 2 || got[0] != canonicalPath(b) || got[1] != canonicalPath(a) {
		t.Fatalf("recent after load = %v", got)
	}
}

func TestGitRecentForget(t *testing.T) {
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	a := filepath.Join(cfg, "a")
	b := filepath.Join(cfg, "b")
	var m GitModel
	m.ensure()
	m.rememberRepo(a)
	m.rememberRepo(b)
	m.forgetRecent(a)
	if got := m.RecentPaths(); len(got) != 1 || got[0] != canonicalPath(b) {
		t.Fatalf("recent = %v", got)
	}
	m.forgetRecent(canonicalPath(b))
	if m.RecentPaths() != nil {
		t.Fatalf("recent = %v", m.RecentPaths())
	}
	m.forgetRecent(a)
	if m.RecentPaths() != nil {
		t.Fatal("forgetting a missing path changed the list")
	}

	var again GitModel
	again.EnsureLoaded()
	if again.RecentPaths() != nil {
		t.Fatalf("restored = %v", again.RecentPaths())
	}
}

func TestGitRecentCap(t *testing.T) {
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	var m GitModel
	m.ensure()
	var last string
	for i := 0; i < gitRecentMax+5; i++ {
		last = filepath.Join(cfg, "repo", strings.Repeat("n", i+1))
		m.rememberRepo(last)
	}
	got := m.RecentPaths()
	if len(got) != gitRecentMax {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] != canonicalPath(last) {
		t.Fatalf("front = %s", got[0])
	}
	var again GitModel
	again.EnsureLoaded()
	if restored := again.RecentPaths(); len(restored) != gitRecentMax || restored[0] != canonicalPath(last) {
		t.Fatalf("restored = %v", restored)
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

func TestFormatCommitMetaParents(t *testing.T) {
	when := time.Date(2026, 10, 11, 9, 17, 39, 0, time.FixedZone("JST", 9*3600))
	text, ranges, hashes := formatCommitMeta(i18n.EN, gitcli.CommitDetail{
		Commit: gitcli.Commit{
			Hash:    "0123456789abcdef",
			Parents: []string{"aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"},
			Author:  "Yota",
			Email:   "a@b.c",
			When:    when,
		},
		Committer:      "GitHub",
		CommitterEmail: "noreply@github.com",
		Body:           "subject\n\nCo-authored-by: Cursor <cursoragent@cursor.com>",
	})
	if !strings.Contains(text, "Commit: 0123456789abcdef") {
		t.Fatalf("commit line missing: %q", text)
	}
	if !strings.Contains(text, "Author: Yota <a@b.c>") || !strings.Contains(text, "Committer: GitHub <noreply@github.com>") {
		t.Fatalf("people missing: %q", text)
	}
	if !strings.Contains(text, "Co-authored-by: Cursor <cursoragent@cursor.com>") {
		t.Fatalf("body missing: %q", text)
	}
	if len(ranges) != 2 || len(hashes) != 2 {
		t.Fatalf("ranges %d hashes %d", len(ranges), len(hashes))
	}
	if hashes[0] != "aaaaaaaaaaaaaaaa" || hashes[1] != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("hashes = %#v", hashes)
	}
	if text[ranges[0].StartInBytes:ranges[0].EndInBytes] != hashes[0] {
		t.Fatalf("first parent range = %q", text[ranges[0].StartInBytes:ranges[0].EndInBytes])
	}
	if text[ranges[1].StartInBytes:ranges[1].EndInBytes] != hashes[1] {
		t.Fatalf("second parent range = %q", text[ranges[1].StartInBytes:ranges[1].EndInBytes])
	}
	got := formatCommitWhenIn(when)
	if got != "Sun Oct 11 2026 09:17:39 GMT+0900 (JST)" {
		t.Fatalf("when = %q", got)
	}
	empty, emptyRanges, emptyHashes := formatCommitMeta(i18n.EN, gitcli.CommitDetail{})
	if len(emptyRanges) != 0 || len(emptyHashes) != 0 || !strings.Contains(empty, "Commit: ") {
		t.Fatalf("empty = %q ranges %d", empty, len(emptyRanges))
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

func gitCommitEmpty(t *testing.T, dir, msg, when string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_DATE="+when,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
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
	gitOutput(t, dir, args...)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
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
	return strings.TrimSpace(string(out))
}

func TestHighlightCommitLeavesDetail(t *testing.T) {
	var m GitModel
	m.ensure()
	m.sel = "aaa"
	m.detail.Hash = "aaa"
	before := m.generation
	m.HighlightCommit("bbb")
	if m.Selected() != "bbb" || m.Detail().Hash != "aaa" {
		t.Fatalf("selected=%s detail=%s", m.Selected(), m.Detail().Hash)
	}
	if m.generation == before {
		t.Fatal("generation did not advance")
	}
	gen := m.generation
	m.HighlightCommit("bbb")
	if m.generation != gen {
		t.Fatal("repeat highlight changed generation")
	}
	m.HighlightCommit("")
	if m.Selected() != "bbb" {
		t.Fatalf("empty highlight selected=%s", m.Selected())
	}
}

func TestGitModelRebase(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "rebase.autoStash", "false")
	writeModelFile(t, dir, "a.txt", "base\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "base")
	gitRun(t, dir, "checkout", "-b", "feature")
	writeModelFile(t, dir, "b.txt", "feat\n")
	gitRun(t, dir, "add", "b.txt")
	gitRun(t, dir, "commit", "-m", "feat")
	gitRun(t, dir, "checkout", "main")
	writeModelFile(t, dir, "c.txt", "main\n")
	gitRun(t, dir, "add", "c.txt")
	gitRun(t, dir, "commit", "-m", "main")
	onto := gitOutput(t, dir, "rev-parse", "HEAD")
	gitRun(t, dir, "checkout", "feature")
	old := gitOutput(t, dir, "rev-parse", "HEAD")

	m := openGitModel(t, dir)
	m.DoRebase(onto)
	waitGit(t, m)
	snap := m.Snapshot()
	if snap.Branch != "feature" || snap.Detached || snap.HEAD == "" || snap.HEAD == old {
		t.Fatalf("branch=%s detached=%v head=%s old=%s status=%s", snap.Branch, snap.Detached, snap.HEAD, old, m.StatusText(i18n.EN))
	}
	if m.StatusText(i18n.EN) != "Rebased" || m.StatusText(i18n.JA) != "rebase しました" {
		t.Fatalf("en=%q ja=%q", m.StatusText(i18n.EN), m.StatusText(i18n.JA))
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.txt")); err != nil {
		t.Fatal(err)
	}

	parent := ""
	for _, c := range snap.Commits {
		if c.Hash == snap.HEAD && len(c.Parents) > 0 {
			parent = c.Parents[0]
		}
	}
	if parent == "" {
		t.Fatal("no parent")
	}
	head := snap.HEAD
	m.DoRebase(parent)
	waitGit(t, m)
	if m.Snapshot().HEAD != head {
		t.Fatalf("HEAD moved to %s", m.Snapshot().HEAD)
	}
	if m.StatusText(i18n.EN) != "Already based on this commit" || m.StatusText(i18n.JA) != "すでにこのコミットの上にあります" {
		t.Fatalf("en=%q ja=%q", m.StatusText(i18n.EN), m.StatusText(i18n.JA))
	}

	m.DoRebase(m.Snapshot().HEAD)
	waitGit(t, m)
	if m.Snapshot().HEAD != head || m.StatusText(i18n.EN) != "Already based on this commit" {
		t.Fatalf("head=%s status=%s", m.Snapshot().HEAD, m.StatusText(i18n.EN))
	}
}

func TestGitModelRebaseDirty(t *testing.T) {
	requireGitBin(t)
	dir, onto := modelRebaseFeature(t)
	writeModelFile(t, dir, "b.txt", "local\n")
	head := gitOutput(t, dir, "rev-parse", "HEAD")

	m := openGitModel(t, dir)
	m.DoRebase(onto)
	waitGit(t, m)
	if m.Snapshot().HEAD != head {
		t.Fatalf("HEAD = %s, want %s", m.Snapshot().HEAD, head)
	}
	if !strings.HasPrefix(m.StatusText(i18n.EN), "Could not rebase: ") || !strings.HasPrefix(m.StatusText(i18n.JA), "rebase に失敗しました: ") {
		t.Fatalf("en=%q ja=%q", m.StatusText(i18n.EN), m.StatusText(i18n.JA))
	}
	body, err := os.ReadFile(filepath.Join(dir, "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "local\n" {
		t.Fatalf("b.txt = %q", body)
	}
	if stash := gitOutput(t, dir, "stash", "list"); stash != "" {
		t.Fatalf("stash = %q", stash)
	}
}

func TestGitModelRebaseConflict(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "rebase.autoStash", "false")
	writeModelFile(t, dir, "a.txt", "base\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "base")
	gitRun(t, dir, "checkout", "-b", "feature")
	writeModelFile(t, dir, "a.txt", "feature\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "feature")
	gitRun(t, dir, "checkout", "main")
	writeModelFile(t, dir, "a.txt", "main\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "main")
	onto := gitOutput(t, dir, "rev-parse", "HEAD")
	gitRun(t, dir, "checkout", "feature")

	m := openGitModel(t, dir)
	m.DoRebase(onto)
	waitGit(t, m)
	if !strings.HasPrefix(m.StatusText(i18n.EN), "Could not rebase: ") {
		t.Fatalf("status = %q", m.StatusText(i18n.EN))
	}
	if !modelRebaseInProgress(dir) {
		t.Fatal("conflict was aborted")
	}
}

func TestGitModelRebaseDetached(t *testing.T) {
	requireGitBin(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	writeModelFile(t, dir, "a.txt", "base\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "base")
	parent := gitOutput(t, dir, "rev-parse", "HEAD")
	writeModelFile(t, dir, "b.txt", "next\n")
	gitRun(t, dir, "add", "b.txt")
	gitRun(t, dir, "commit", "-m", "next")
	gitRun(t, dir, "checkout", "--detach")

	m := openGitModel(t, dir)
	if !m.Snapshot().Detached {
		t.Fatal("expected detached")
	}
	head := m.Snapshot().HEAD
	status := m.StatusText(i18n.EN)
	m.DoRebase(parent)
	waitGit(t, m)
	if m.Snapshot().HEAD != head || m.StatusText(i18n.EN) != status {
		t.Fatalf("head=%s status=%s", m.Snapshot().HEAD, m.StatusText(i18n.EN))
	}
}

func openGitModel(t *testing.T, dir string) *GitModel {
	t.Helper()
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)
	var m GitModel
	m.Open(dir)
	waitGit(t, &m)
	if !m.HasRepo() {
		t.Fatalf("status = %s", m.StatusText(i18n.EN))
	}
	return &m
}

func modelRebaseFeature(t *testing.T) (dir, onto string) {
	t.Helper()
	dir = t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "rebase.autoStash", "false")
	writeModelFile(t, dir, "a.txt", "base\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "base")
	gitRun(t, dir, "checkout", "-b", "feature")
	writeModelFile(t, dir, "b.txt", "feat\n")
	gitRun(t, dir, "add", "b.txt")
	gitRun(t, dir, "commit", "-m", "feat")
	gitRun(t, dir, "checkout", "main")
	writeModelFile(t, dir, "c.txt", "main\n")
	gitRun(t, dir, "add", "c.txt")
	gitRun(t, dir, "commit", "-m", "main")
	onto = gitOutput(t, dir, "rev-parse", "HEAD")
	gitRun(t, dir, "checkout", "feature")
	return dir, onto
}

func TestPlanWorktreeTabs(t *testing.T) {
	main, feature, other := "/main", "/feature", "/other"
	check := func(name string, paths []string, active int, removed, parent string, parentOpen bool, want worktreeTabAction) {
		t.Helper()
		got := planWorktreeTabs(paths, active, removed, parent, parentOpen)
		if got != want {
			t.Fatalf("%s = %+v", name, got)
		}
	}
	check("show parent tab", []string{main, feature}, 1, feature, main, true, worktreeTabAction{Close: 1, Reopen: -1, Select: 0})
	check("parent already showing", []string{main, feature}, 0, feature, main, true, worktreeTabAction{Close: 1, Reopen: -1, Select: -1})
	check("move the only tab", []string{feature}, 0, feature, main, true, worktreeTabAction{Close: -1, Reopen: 0, Select: -1})
	check("leave the other tab", []string{main, feature, other}, 2, feature, main, true, worktreeTabAction{Close: 1, Reopen: -1, Select: -1})
	check("bare last tab", []string{feature}, 0, feature, "/bare.git", false, worktreeTabAction{Close: 0, Blank: true, Reopen: -1, Select: -1})
	check("bare with a neighbor", []string{other, feature}, 1, feature, "/bare.git", false, worktreeTabAction{Close: 1, Reopen: -1, Select: -1})
	check("not open", []string{main}, 0, feature, main, true, worktreeTabAction{Close: -1, Reopen: -1, Select: -1})

	hostA := "ssh:a::/feature"
	hostB := "ssh:b::/feature"
	parentA := "ssh:a::/main"
	check("other host stays", []string{hostA, hostB}, 0, hostA, parentA, true, worktreeTabAction{Close: -1, Reopen: 0, Select: -1})
	if got := planWorktreeTabs([]string{hostA, hostB}, 1, hostA, parentA, true); got.Reopen != 0 || got.Close != -1 {
		t.Fatalf("viewing the other host = %+v", got)
	}
}

func TestWorktreeKeyKeepsHost(t *testing.T) {
	a := worktreeKey("ssh:host::/repo", "/repo/feature")
	b := worktreeKey("ssh:other::/repo", "/repo/feature")
	if a == b || !strings.Contains(a, "/repo/feature") || !strings.Contains(b, "/repo/feature") {
		t.Fatalf("a=%s b=%s", a, b)
	}
	if got := worktreeKey("/repo", "/repo/feature"); got != canonicalPath("/repo/feature") {
		t.Fatalf("local = %s", got)
	}
}

func TestOpenHereReusesTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	dir := initRepo(t, "main")
	other := filepath.Join(t.TempDir(), "feature")
	gitRun(t, dir, "worktree", "add", "-b", "feature", other)

	var m GitModel
	m.Open(dir)
	waitGit(t, &m)
	m.OpenHere(other)
	waitGit(t, &m)
	if m.TabCount() != 1 || canonicalPath(m.Path()) != canonicalPath(other) {
		t.Fatalf("tabs=%d path=%s", m.TabCount(), m.Path())
	}
	m.Open(dir)
	waitGit(t, &m)
	if m.TabCount() != 1 || canonicalPath(m.Path()) != canonicalPath(dir) {
		t.Fatalf("same parent tabs=%d path=%s", m.TabCount(), m.Path())
	}
	unrelated := initRepo(t, "other")
	m.Open(unrelated)
	waitGit(t, &m)
	if m.TabCount() != 2 {
		t.Fatalf("tabs = %d", m.TabCount())
	}
	m.SelectTab(0)
	m.OpenHere(unrelated)
	waitGit(t, &m)
	if m.TabCount() != 2 || canonicalPath(m.Path()) != canonicalPath(unrelated) {
		t.Fatalf("switch tabs=%d path=%s active=%d", m.TabCount(), m.Path(), m.ActiveTab())
	}
}

func TestCloseWorktree(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	dir := initRepo(t, "main")
	other := filepath.Join(t.TempDir(), "feature")
	gitRun(t, dir, "worktree", "add", "-b", "feature", other)
	if err := os.WriteFile(filepath.Join(other, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var m GitModel
	m.Open(dir)
	waitGit(t, &m)
	m.Open(other)
	waitGit(t, &m)
	feature := worktreeNamed(m.Snapshot().Worktrees, "feature")
	if feature.Path == "" || feature.Main {
		t.Fatalf("feature = %+v", feature)
	}
	mainWT := worktreeNamed(m.Snapshot().Worktrees, "main")
	m.DoCloseWorktree(mainWT, true)
	waitGit(t, &m)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.ConsoleText(), "worktree remove") {
		t.Fatalf("main ran git: %q", m.ConsoleText())
	}

	m.DoCloseWorktree(feature, false)
	waitGit(t, &m)
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("dirty worktree = %v", err)
	}
	if m.TabCount() != 1 || canonicalPath(m.Path()) != canonicalPath(other) {
		t.Fatalf("after failure tabs=%d path=%s", m.TabCount(), m.Path())
	}
	if !strings.Contains(m.StatusText(i18n.EN), "Could not close worktree") {
		t.Fatalf("status = %q", m.StatusText(i18n.EN))
	}
	if !strings.Contains(m.ConsoleText(), "git worktree remove ") || strings.Contains(m.ConsoleText(), "git worktree remove --force") {
		t.Fatalf("console = %q", m.ConsoleText())
	}

	m.DoCloseWorktree(feature, true)
	waitGit(t, &m)
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatalf("forced remove: %v", err)
	}
	if m.TabCount() != 1 || canonicalPath(m.Path()) != canonicalPath(dir) {
		t.Fatalf("tabs=%d path=%s", m.TabCount(), m.Path())
	}
	if !strings.Contains(gitOutput(t, dir, "branch"), "feature") {
		t.Fatal("branch was deleted")
	}
	if !strings.Contains(m.ConsoleText(), "git worktree remove --force --force") {
		t.Fatalf("console = %q", m.ConsoleText())
	}
	if m.StatusText(i18n.EN) != "Closed worktree" {
		t.Fatalf("status = %q", m.StatusText(i18n.EN))
	}
}

func TestCloseWorktreeMovesOnlyTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	dir := initRepo(t, "main")
	other := filepath.Join(t.TempDir(), "feature")
	gitRun(t, dir, "worktree", "add", "-b", "feature", other)

	var m GitModel
	m.Open(other)
	waitGit(t, &m)
	m.DoCloseWorktree(worktreeNamed(m.Snapshot().Worktrees, "feature"), false)
	waitGit(t, &m)
	if m.TabCount() != 1 || canonicalPath(m.Path()) != canonicalPath(dir) {
		t.Fatalf("tabs=%d path=%s", m.TabCount(), m.Path())
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatalf("removed: %v", err)
	}
	if !strings.Contains(m.ConsoleText(), "git worktree remove ") || strings.Contains(m.ConsoleText(), "git worktree remove --force") {
		t.Fatalf("console = %q", m.ConsoleText())
	}
}

func TestCloseBareParentWorktreeBlanksTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	seed := initRepo(t, "seed")
	bare := filepath.Join(t.TempDir(), "bare.git")
	feature := filepath.Join(t.TempDir(), "feature")
	gitRun(t, seed, "clone", "--bare", seed, bare)
	gitRun(t, bare, "worktree", "add", "-b", "feature", feature, "main")

	var m GitModel
	m.Open(feature)
	waitGit(t, &m)
	wt := worktreeNamed(m.Snapshot().Worktrees, "feature")
	if wt.Path == "" || wt.Main || wt.Bare {
		t.Fatalf("feature = %+v trees=%+v", wt, m.Snapshot().Worktrees)
	}
	m.DoCloseWorktree(wt, false)
	waitGit(t, &m)
	if m.TabCount() != 1 || m.Path() != "" {
		t.Fatalf("tabs=%d path=%q", m.TabCount(), m.Path())
	}
	if _, err := os.Stat(feature); !os.IsNotExist(err) {
		t.Fatalf("feature remains: %v", err)
	}
	if strings.Contains(m.Path(), "bare") {
		t.Fatalf("opened bare path %s", m.Path())
	}
	if !strings.Contains(m.ConsoleText(), "git worktree remove") {
		t.Fatalf("console = %q", m.ConsoleText())
	}
	if !strings.Contains(gitOutput(t, bare, "branch"), "feature") {
		t.Fatal("branch was deleted")
	}
}

func worktreeNamed(trees []gitcli.Worktree, branch string) gitcli.Worktree {
	for _, wt := range trees {
		if wt.Branch == branch {
			return wt
		}
	}
	return gitcli.Worktree{}
}

func writeModelFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func modelRebaseInProgress(dir string) bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(dir, ".git", name)); err == nil {
			return true
		}
	}
	return false
}
