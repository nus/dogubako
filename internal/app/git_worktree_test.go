package app

import (
	"path/filepath"
	"testing"
)

func TestSwitchWorktreeRestoresView(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	main := initRepo(t, "app")
	gitRun(t, main, "commit", "--allow-empty", "-m", "second")
	feature := filepath.Join(t.TempDir(), "feature")
	gitRun(t, main, "worktree", "add", "-b", "feature", feature)

	var m GitModel
	m.Open(main)
	waitGit(t, &m)
	older := ""
	for _, c := range m.Snapshot().Commits {
		if c.Hash != m.Snapshot().HEAD {
			older = c.Hash
			break
		}
	}
	if older == "" {
		t.Fatal("no older commit")
	}
	m.SelectCommit(older)
	m.SetDraft("wip on main")

	var tool GitTool
	tool.faceSess = m.gitSession
	tool.commitView = true
	tool.openHash = older
	tool.histFile = "app.txt"
	tool.histFrac = 0.4
	tool.refFilter = "feat"
	tool.histQuery = "second"
	tool.histSearchOpen = true
	tool.diffSplit = true
	tool.workPanes.unstaged.sel = []string{"app.txt"}
	tool.workPanes.diff.view.compact.scrollY = -48

	tool.openWorktree(&m, feature)
	waitGit(t, &m)
	tool.adoptFace(&m)
	if m.TabCount() != 1 || m.Path() != canonicalPath(feature) {
		t.Fatalf("count=%d path=%s", m.TabCount(), m.Path())
	}
	paths, active, _ := loadGitTabs()
	if active != 0 || len(paths) != 1 || paths[0] != canonicalPath(feature) {
		t.Fatalf("saved paths=%q active=%d", paths, active)
	}
	if m.Draft() != "" || tool.commitView || tool.refFilter != "" || tool.openHash != "" || tool.histQuery != "" || len(tool.workPanes.unstaged.sel) != 0 {
		t.Fatalf("new worktree kept the previous screen draft=%q view=%v filter=%q open=%q query=%q sel=%v", m.Draft(), tool.commitView, tool.refFilter, tool.openHash, tool.histQuery, tool.workPanes.unstaged.sel)
	}

	m.SetDraft("note")
	featureSel := m.Selected()
	tool.commitView = false
	tool.openHash = featureSel
	tool.workPanes.diff.view.compact.scrollY = -3

	tool.openWorktree(&m, main)
	tool.adoptFace(&m)
	if m.TabCount() != 1 || m.Path() != canonicalPath(main) {
		t.Fatalf("back count=%d path=%s", m.TabCount(), m.Path())
	}
	if m.Selected() != older || m.Draft() != "wip on main" {
		t.Fatalf("restored sel=%s draft=%q", m.Selected(), m.Draft())
	}
	if !tool.commitView || tool.openHash != older || tool.histFile != "app.txt" || tool.histFrac != 0.4 || tool.refFilter != "feat" || tool.histQuery != "second" || !tool.histSearchOpen || !tool.diffSplit {
		t.Fatalf("screen view=%v open=%s file=%s frac=%v filter=%q query=%q search=%v split=%v", tool.commitView, tool.openHash, tool.histFile, tool.histFrac, tool.refFilter, tool.histQuery, tool.histSearchOpen, tool.diffSplit)
	}
	if len(tool.workPanes.unstaged.sel) != 1 || tool.workPanes.unstaged.sel[0] != "app.txt" {
		t.Fatalf("files = %v", tool.workPanes.unstaged.sel)
	}
	if !tool.workHold.set || tool.workHold.compactY != -48 {
		t.Fatalf("hold = %+v", tool.workHold)
	}
	tool.workPanes.diff.holdPlace(tool.workHold)
	tool.workPanes.diff.view.Set("diff\n", false, true, false, false, "")
	if tool.workPanes.diff.view.compact.scrollY != -48 {
		t.Fatalf("scroll = %v", tool.workPanes.diff.view.compact.scrollY)
	}

	tool.openWorktree(&m, feature)
	tool.adoptFace(&m)
	if m.TabCount() != 1 || m.Draft() != "note" || m.Selected() != featureSel || tool.commitView || tool.openHash != featureSel {
		t.Fatalf("feature count=%d draft=%q sel=%s view=%v open=%s", m.TabCount(), m.Draft(), m.Selected(), tool.commitView, tool.openHash)
	}
}

func TestOpenSiblingWorktreeStaysInTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	main := initRepo(t, "app")
	feature := filepath.Join(t.TempDir(), "feature")
	gitRun(t, main, "worktree", "add", "-b", "feature", feature)

	var m GitModel
	m.Open(main)
	waitGit(t, &m)
	m.SetDraft("from main")
	m.Open(feature)
	waitGit(t, &m)
	if m.TabCount() != 1 || m.Path() != canonicalPath(feature) || m.Draft() != "" {
		t.Fatalf("count=%d path=%s draft=%q", m.TabCount(), m.Path(), m.Draft())
	}
	m.Open(main)
	if m.TabCount() != 1 || m.Path() != canonicalPath(main) || m.Draft() != "from main" {
		t.Fatalf("back count=%d path=%s draft=%q", m.TabCount(), m.Path(), m.Draft())
	}
}

func TestSameParentWorktreesShareOneTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	main := initRepo(t, "app")
	feature := filepath.Join(t.TempDir(), "feature")
	fix := filepath.Join(t.TempDir(), "fix")
	gitRun(t, main, "worktree", "add", "-b", "feature", feature)
	gitRun(t, main, "worktree", "add", "-b", "fix", fix)
	other := initRepo(t, "other")

	var m GitModel
	m.Open(main)
	m.SetDraft("main draft")
	m.Open(other)
	m.Open(feature)
	m.Open(fix)
	waitGit(t, &m)
	if m.TabCount() != 2 || m.ActiveTab() != 0 || m.Path() != canonicalPath(fix) {
		t.Fatalf("count=%d active=%d path=%s", m.TabCount(), m.ActiveTab(), m.Path())
	}
	if filepath.Base(m.tabs[1].path) != filepath.Base(other) {
		t.Fatalf("other tab = %s", m.tabs[1].path)
	}
	m.Open(main)
	if m.TabCount() != 2 || m.Path() != canonicalPath(main) || m.Draft() != "main draft" {
		t.Fatalf("parent count=%d path=%s draft=%q", m.TabCount(), m.Path(), m.Draft())
	}
	if filepath.Base(m.tabs[1].path) != filepath.Base(other) {
		t.Fatalf("other moved = %s", m.tabs[1].path)
	}
}

func TestBareWorktreesShareOneTab(t *testing.T) {
	requireGitBin(t)
	cfg := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return cfg, nil })
	t.Cleanup(restore)

	root := t.TempDir()
	bare := filepath.Join(root, "bare.git")
	gitRun(t, root, "init", "--bare", "-b", "main", bare)
	seed := filepath.Join(root, "seed")
	gitRun(t, root, "clone", bare, seed)
	gitRun(t, seed, "config", "user.name", "Test")
	gitRun(t, seed, "config", "user.email", "test@example.com")
	gitRun(t, seed, "config", "commit.gpgsign", "false")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "init")
	gitRun(t, seed, "push", "origin", "main")
	feature := filepath.Join(root, "feature")
	fix := filepath.Join(root, "fix")
	gitRun(t, bare, "worktree", "add", "-b", "feature", feature, "main")
	gitRun(t, bare, "worktree", "add", "-b", "fix", fix, "main")

	var m GitModel
	m.Open(feature)
	m.Open(fix)
	waitGit(t, &m)
	if m.TabCount() != 1 || m.Path() != canonicalPath(fix) {
		t.Fatalf("count=%d path=%s", m.TabCount(), m.Path())
	}
}
