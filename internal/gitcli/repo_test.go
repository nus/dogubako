package gitcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

func gitAt(t *testing.T, dir string, args ...string) string {
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
	return string(out)
}

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	gitAt(t, dir, "init", "-b", "main")
	gitAt(t, dir, "config", "user.name", "Test")
	gitAt(t, dir, "config", "user.email", "test@example.com")
	gitAt(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func writeCommit(t *testing.T, dir, name, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", name)
	gitAt(t, dir, "commit", "-m", msg)
}

func TestOpenSnapshotCommitAmend(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	writeCommit(t, dir, "b.txt", "second")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Dir == "" {
		t.Fatal("empty repo dir")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" || len(snap.Commits) != 2 {
		t.Fatalf("snap branch=%s commits=%d", snap.Branch, len(snap.Commits))
	}
	if snap.HEAD == "" || snap.Graph[0].Commit.Hash != snap.HEAD {
		t.Fatalf("HEAD %s vs first %s", snap.HEAD, snap.Graph[0].Commit.Hash)
	}
	if snap.Lanes != 1 {
		t.Fatalf("lanes = %d", snap.Lanes)
	}

	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Status.Dirty() || len(snap.Commits) != 2 || snap.Graph[0].Commit.Hash != Uncommitted || snap.Graph[1].Commit.Hash != snap.HEAD {
		t.Fatalf("dirty graph head=%s first=%s commits=%d dirty=%v", snap.HEAD, snap.Graph[0].Commit.Hash, len(snap.Commits), snap.Status.Dirty())
	}
	if err := repo.Commit(ctx, "third", true, false); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Commits) != 3 || snap.Commits[0].Subject != "third" {
		t.Fatalf("after commit: %+v", snap.Commits)
	}
	if err := repo.Commit(ctx, "third amended", false, true); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Commits[0].Subject != "third amended" {
		t.Fatalf("amend subject = %q", snap.Commits[0].Subject)
	}
	if !hasDecoration(snap.Commits[0].Decorations, "head", "main") {
		t.Fatalf("decorations = %#v", snap.Commits[0].Decorations)
	}

	d, err := repo.Detail(ctx, snap.HEAD)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Body, "third amended") {
		t.Fatalf("body = %q", d.Body)
	}
	if len(d.Files) == 0 {
		t.Fatal("expected files")
	}
	if !strings.Contains(d.Diff, "c.txt") || !strings.Contains(d.Diff, "+c") {
		t.Fatalf("diff = %q", d.Diff)
	}
	if d.Committer != "Test" || d.CommitterEmail != "test@example.com" {
		t.Fatalf("committer = %q <%s>", d.Committer, d.CommitterEmail)
	}
}

func TestCreateTag(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(ctx, "v1.0", snap.HEAD); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tags) != 1 || snap.Tags[0].Name != "v1.0" || snap.Tags[0].Hash != snap.HEAD {
		t.Fatalf("tags = %#v head = %s", snap.Tags, snap.HEAD)
	}
	if err := repo.CreateTag(ctx, "v1.0", snap.HEAD); err == nil {
		t.Fatal("expected duplicate tag to fail")
	}
	if err := repo.CreateTag(ctx, "bad name", snap.HEAD); err == nil {
		t.Fatal("expected invalid tag name to fail")
	}
}

func TestPushTag(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(ctx, "v1.0", snap.HEAD); err != nil {
		t.Fatal(err)
	}
	if err := repo.PushTag(ctx, "v1.0"); err != nil {
		t.Fatal(err)
	}
	out := gitAt(t, remote, "show-ref", "--tags")
	if !strings.Contains(out, "refs/tags/v1.0") {
		t.Fatalf("remote tags = %s", out)
	}
}

func TestDeleteTag(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	gitAt(t, dir, "tag", "v1.0")
	gitAt(t, dir, "tag", "-a", "v1.1", "-m", "annotated")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(ctx, "v1.0"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(ctx, "v1.1"); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tags) != 0 {
		t.Fatalf("tags = %#v", snap.Tags)
	}
	if err := repo.DeleteTag(ctx, "v1.0"); err == nil {
		t.Fatal("expected missing tag to fail")
	}
	if err := repo.DeleteTag(ctx, "bad name"); err == nil {
		t.Fatal("expected invalid tag name to fail")
	}
}

func TestDeleteRemoteTag(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(ctx, "v1.0", snap.HEAD); err != nil {
		t.Fatal(err)
	}
	if err := repo.PushTag(ctx, "v1.0"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteRemoteTag(ctx, "v1.0"); err != nil {
		t.Fatal(err)
	}
	out := gitAt(t, remote, "for-each-ref", "--format=%(refname)", "refs/tags")
	if strings.Contains(out, "refs/tags/v1.0") {
		t.Fatalf("remote tags = %s", out)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tags) != 1 || snap.Tags[0].Name != "v1.0" {
		t.Fatalf("local tags = %#v", snap.Tags)
	}
}

func TestSnapshotListsTags(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	gitAt(t, dir, "tag", "v1.0")
	gitAt(t, dir, "tag", "-a", "v1.1", "-m", "annotated")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tags) != 2 || snap.Tags[0].Name != "v1.0" || snap.Tags[1].Name != "v1.1" {
		t.Fatalf("tags = %#v", snap.Tags)
	}
	if snap.Tags[0].Hash == "" || snap.Tags[0].Hash != snap.HEAD {
		t.Fatalf("lightweight tag hash = %q head = %q", snap.Tags[0].Hash, snap.HEAD)
	}
	if snap.Tags[1].Hash != snap.HEAD {
		t.Fatalf("annotated tag hash = %q head = %q", snap.Tags[1].Hash, snap.HEAD)
	}
	if !commitHasDecoration(snap.Commits, "tag", "v1.0") || !commitHasDecoration(snap.Commits, "tag", "v1.1") {
		t.Fatal("tag decorations missing")
	}
}

func TestSnapshotHidesRemoteRefs(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feature")
	gitAt(t, dir, "push", "-u", "origin", "feature")
	gitAt(t, dir, "checkout", "main")
	gitAt(t, dir, "branch", "-D", "feature")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	shown, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := repo.Snapshot(ctx, SnapshotOpts{HiddenRemotes: map[string]bool{"origin": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(shown.RemoteBranches) == 0 {
		t.Fatal("expected remote branches when shown")
	}
	hasFeature := false
	for _, c := range shown.Commits {
		if c.Subject == "feature" {
			hasFeature = true
		}
	}
	if !hasFeature {
		t.Fatal("expected feature commit when remotes shown")
	}
	if !commitHasDecoration(shown.Commits, "remote", "origin/feature") {
		t.Fatal("expected origin/feature on the feature commit")
	}
	if !commitHasDecoration(shown.Commits, "head", "main") {
		t.Fatal("expected local main decoration")
	}
	for _, c := range hidden.Commits {
		if c.Subject == "feature" {
			t.Fatal("feature commit should be hidden with origin")
		}
	}
}

func TestCheckoutCarriesLocalChanges(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feat")
	gitAt(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, Ref{Name: "feature"}); err != nil {
		t.Fatalf("checkout err = %v", err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "feature" || snap.Detached || !snap.Status.Dirty() {
		t.Fatalf("branch = %s detached=%v dirty=%v", snap.Branch, snap.Detached, snap.Status.Dirty())
	}
	edited, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(edited) != "base\nedited\n" {
		t.Fatalf("a.txt = %q", edited)
	}
	if _, err := os.Stat(filepath.Join(dir, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckoutOverwriteKeepsBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "a.txt", "feature")
	gitAt(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	checkoutErr := repo.Checkout(ctx, Ref{Name: "feature"})
	if !errors.Is(checkoutErr, ErrLocalChanges) {
		t.Fatalf("checkout err = %v", checkoutErr)
	}
	if !strings.Contains(checkoutErr.Error(), "a.txt") {
		t.Fatalf("checkout err = %v", checkoutErr)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" || snap.Detached {
		t.Fatalf("branch = %s detached=%v", snap.Branch, snap.Detached)
	}
	if len(snap.Graph) == 0 || snap.Graph[0].Commit.Hash != Uncommitted {
		t.Fatal("uncommitted is not the first row")
	}
	if !grayReaches(snap.Graph, Uncommitted, snap.HEAD) {
		t.Fatalf("gray line does not reach HEAD %s", snap.HEAD)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "local\n" {
		t.Fatalf("a.txt = %q", body)
	}
}

func TestCheckoutWouldOverwriteMessage(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"error: Your local changes to the following files would be overwritten by checkout:\n\ta.txt", true},
		{"error: The following untracked working tree files would be overwritten by checkout:\n\ta.txt", true},
		{"error: 次のファイルへのローカルの変更はチェックアウトで上書きされます:\n\ta.txt", true},
		{"error: 以下の追跡されていない作業ツリーのファイルは、チェックアウトで上書きされます:\n\ta.txt", true},
		{"fatal: invalid reference: missing", false},
	}
	for _, tc := range cases {
		if got := checkoutWouldOverwrite(errors.New(tc.msg)); got != tc.want {
			t.Fatalf("%q: got %v", tc.msg, got)
		}
	}
}

func TestCheckoutTag(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "tag", "v1.0")
	writeCommit(t, dir, "b.txt", "next")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Detached || len(snap.Tags) != 1 || snap.Tags[0].Name != "v1.0" {
		t.Fatalf("before = detached %v tags %#v", snap.Detached, snap.Tags)
	}
	tag := snap.Tags[0]
	if tag.Hash == snap.HEAD {
		t.Fatal("tag still points at HEAD")
	}
	if err := repo.Checkout(ctx, tag); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Detached || snap.HEAD != tag.Hash || snap.Branch != "" {
		t.Fatalf("after = detached %v branch %q head %s tag %s", snap.Detached, snap.Branch, snap.HEAD, tag.Hash)
	}
	status := gitAt(t, dir, "status")
	if !strings.Contains(status, "HEAD detached at v1.0") {
		t.Fatalf("status = %s", status)
	}
	reflog := gitAt(t, dir, "reflog", "-1")
	if !strings.Contains(reflog, "checkout: moving from main to v1.0") {
		t.Fatalf("reflog = %s", reflog)
	}

	next := snap
	gitAt(t, dir, "tag", "v1.1", "main")
	snap, err = repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var later Ref
	for _, tg := range snap.Tags {
		if tg.Name == "v1.1" {
			later = tg
		}
	}
	if later.Hash == "" || later.Hash == next.HEAD {
		t.Fatalf("v1.1 = %#v detached %s", later, next.HEAD)
	}
	if err := repo.Checkout(ctx, later); err != nil {
		t.Fatal(err)
	}
	status = gitAt(t, dir, "status")
	if !strings.Contains(status, "HEAD detached at v1.1") {
		t.Fatalf("status = %s", status)
	}
	reflog = gitAt(t, dir, "reflog", "-1")
	if !strings.Contains(reflog, "checkout: moving from "+tag.Hash+" to v1.1") {
		t.Fatalf("reflog = %s", reflog)
	}
}

func TestPullRefFastForward(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feature")
	gitAt(t, dir, "push", "-u", "origin", "feature")
	gitAt(t, dir, "reset", "--hard", "HEAD~1")
	gitAt(t, dir, "checkout", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, Ref{Name: "feature"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PullRef(ctx, Ref{Name: "origin/feature", Remote: "origin"}); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "feature" {
		t.Fatalf("branch = %s", snap.Branch)
	}
	if !commitHasDecoration(snap.Commits, "head", "feature") {
		t.Fatal("expected feature at HEAD")
	}
	headSubject := ""
	for _, c := range snap.Commits {
		if c.Hash == snap.HEAD {
			headSubject = c.Subject
		}
	}
	if headSubject != "feature" {
		t.Fatalf("HEAD subject = %q", headSubject)
	}
	reflog := gitAt(t, dir, "reflog", "-1", "feature")
	if !strings.Contains(reflog, "pull") || strings.Contains(reflog, "reset:") {
		t.Fatalf("reflog = %s", reflog)
	}
}

func TestPullRefDiverged(t *testing.T) {
	dir := divergedMain(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	before := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	origin := strings.TrimSpace(gitAt(t, dir, "rev-parse", "origin/main"))
	if before == origin {
		t.Fatal("expected local main and origin/main to differ")
	}
	// An edit outside the rewritten file stays in the worktree.
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("same\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.PullRef(ctx, Ref{Name: "origin/main", Remote: "origin"}); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	origin = strings.TrimSpace(gitAt(t, dir, "rev-parse", "origin/main"))
	if head != origin {
		t.Fatalf("HEAD %s origin %s", head, origin)
	}
	if branch := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "main" {
		t.Fatalf("branch = %s", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("local-only file remains: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "rewritten\n" {
		t.Fatalf("a.txt = %q", body)
	}
	same, err := os.ReadFile(filepath.Join(dir, "c.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(same) != "same\nedited\n" {
		t.Fatalf("c.txt = %q", same)
	}
	reflog := gitAt(t, dir, "reflog", "-1")
	if !strings.Contains(reflog, "reset: moving to refs/remotes/origin/main") {
		t.Fatalf("reflog = %s", reflog)
	}
}

func TestPullRefDivergedFromOtherBranch(t *testing.T) {
	dir := divergedMain(t)
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feature")
	feature := strings.TrimSpace(gitAt(t, dir, "rev-parse", "feature"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, Ref{Name: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PullRef(ctx, Ref{Name: "origin/main", Remote: "origin"}); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	origin := strings.TrimSpace(gitAt(t, dir, "rev-parse", "origin/main"))
	main := strings.TrimSpace(gitAt(t, dir, "rev-parse", "main"))
	if head != origin || main != origin {
		t.Fatalf("HEAD %s main %s origin %s", head, main, origin)
	}
	if branch := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "main" {
		t.Fatalf("branch = %s", branch)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "feature")); got != feature {
		t.Fatalf("feature moved %s -> %s", feature, got)
	}
}

func TestPullRefDivergedKeepsOverlappingEdit(t *testing.T) {
	dir := divergedMain(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\nlocaledit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PullRef(ctx, Ref{Name: "origin/main", Remote: "origin"}); err == nil {
		t.Fatal("expected pull to refuse an overlapping edit")
	}
	after := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	if after != before {
		t.Fatalf("HEAD moved %s -> %s", before, after)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "base\nlocaledit\n" {
		t.Fatalf("a.txt = %q", body)
	}
}

func TestPullRefMissingBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	before := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PullRef(ctx, Ref{Name: "origin/missing", Remote: "origin"}); err == nil {
		t.Fatal("expected missing branch to fail")
	}
	after := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	if after != before {
		t.Fatalf("HEAD moved %s -> %s", before, after)
	}
}

func TestRefusedFastForward(t *testing.T) {
	if !refusedFastForward(errors.New("hint: diverging\nfatal: Not possible to fast-forward, aborting.")) {
		t.Fatal("english refusal")
	}
	if !refusedFastForward(errors.New("fatal: 早送りできません。中止します。")) {
		t.Fatal("japanese refusal")
	}
	if refusedFastForward(errors.New("fatal: couldn't find remote ref missing")) {
		t.Fatal("missing ref treated as divergence")
	}
	if refusedFastForward(nil) {
		t.Fatal("nil")
	}
}

// divergedMain is a clean repository checked out on main. origin/main was
// rewritten, and main has a commit origin does not. c.txt is the same in both.
func divergedMain(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	writeCommit(t, dir, "c.txt", "same")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	writeCommit(t, dir, "b.txt", "local")

	other := t.TempDir()
	gitAt(t, other, "clone", remote, ".")
	gitAt(t, other, "config", "user.name", "Test")
	gitAt(t, other, "config", "user.email", "test@example.com")
	gitAt(t, other, "config", "commit.gpgsign", "false")
	writeCommit(t, other, "a.txt", "rewritten")
	gitAt(t, other, "push", "--force", "origin", "main")
	gitAt(t, dir, "fetch", "origin")
	return dir
}

func TestCheckoutSwitch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feat")
	gitAt(t, dir, "checkout", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, Ref{Name: "feature"}); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "feature" {
		t.Fatalf("branch = %s", snap.Branch)
	}
}

func TestStateMatchesSnapshotAndSeesBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := repo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Head != snap.HEAD || st.Refs != snap.RefDigest {
		t.Fatalf("state head=%s refs differ=%v", st.Head, st.Refs != snap.RefDigest)
	}
	gitAt(t, dir, "checkout", "-b", "feature")
	next, err := repo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Refs == st.Refs {
		t.Fatal("creating a branch did not change the ref digest")
	}
	if next.Status.Branch != "feature" {
		t.Fatalf("branch = %s", next.Status.Branch)
	}
}

func TestCreateBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	base := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	writeCommit(t, dir, "b.txt", "next")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateBranch(ctx, "topic", base); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" || snap.HEAD == base {
		t.Fatalf("branch=%s head=%s base=%s", snap.Branch, snap.HEAD, base)
	}
	var found bool
	for _, ref := range snap.Locals {
		if ref.Name != "topic" {
			continue
		}
		found = true
		if ref.Hash != base || ref.Current {
			t.Fatalf("topic = %#v", ref)
		}
	}
	if !found {
		t.Fatal("topic missing")
	}
	if err := repo.CreateBranch(ctx, "topic", base); err == nil {
		t.Fatal("created a duplicate branch")
	}
	if err := repo.CreateBranch(ctx, "bad name", base); err == nil {
		t.Fatal("accepted an invalid name")
	}
	if err := repo.CreateBranch(ctx, "other", "deadbeef"); err == nil {
		t.Fatal("accepted an unknown revision")
	}
	again, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Branch != "main" {
		t.Fatalf("branch after rejected create = %s", again.Branch)
	}
}

func TestRenameBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	gitAt(t, dir, "checkout", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RenameBranch(ctx, Ref{Name: "feature"}, "topic"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RenameBranch(ctx, Ref{Name: "main", Current: true}, "trunk"); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "trunk" || snap.HEAD != before.HEAD {
		t.Fatalf("branch=%s head=%s before=%s", snap.Branch, snap.HEAD, before.HEAD)
	}
	names := map[string]bool{}
	for _, ref := range snap.Locals {
		names[ref.Name] = ref.Current
	}
	if _, ok := names["feature"]; ok {
		t.Fatal("feature still present")
	}
	if _, ok := names["main"]; ok {
		t.Fatal("main still present")
	}
	if current, ok := names["topic"]; !ok || current {
		t.Fatalf("topic = present %v current %v", ok, current)
	}
	if current, ok := names["trunk"]; !ok || !current {
		t.Fatalf("trunk = present %v current %v", ok, current)
	}
	if err := repo.RenameBranch(ctx, Ref{Name: "trunk", Current: true}, "topic"); err == nil {
		t.Fatal("renamed onto an existing branch")
	}
	if err := repo.RenameBranch(ctx, Ref{Name: "trunk", Current: true}, "bad name"); err == nil {
		t.Fatal("accepted an invalid name")
	}
	if err := repo.RenameBranch(ctx, Ref{Name: "origin/topic", Remote: "origin"}, "other"); err == nil {
		t.Fatal("renamed a remote-tracking branch")
	}
	again, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Branch != "trunk" {
		t.Fatalf("branch after rejected rename = %s", again.Branch)
	}
}

func TestDeleteBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feat")
	gitAt(t, dir, "checkout", "main")
	gitAt(t, dir, "update-ref", "refs/remotes/origin/feature", "HEAD")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBranch(ctx, Ref{Name: "main"}); !errors.Is(err, ErrDeleteCurrent) {
		t.Fatalf("delete current = %v", err)
	}
	if err := repo.DeleteBranch(ctx, Ref{Name: "feature"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBranch(ctx, Ref{Name: "origin/feature", Remote: "origin"}); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" {
		t.Fatalf("branch = %s", snap.Branch)
	}
	for _, ref := range snap.Locals {
		if ref.Name == "feature" {
			t.Fatal("local feature still present")
		}
	}
	for _, ref := range snap.RemoteBranches {
		if ref.Name == "origin/feature" {
			t.Fatal("remote feature still present")
		}
	}
}

func TestStageUnstage(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	writeCommit(t, dir, "b.txt", "second")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Stage(ctx, []string{"a.txt", "b.txt", "c.txt"}); err != nil {
		t.Fatal(err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertEntry(t, st, "a.txt", true, false)
	assertEntry(t, st, "b.txt", true, false)
	assertEntry(t, st, "c.txt", true, false)

	if err := repo.Unstage(ctx, []string{"a.txt", "c.txt"}); err != nil {
		t.Fatal(err)
	}
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertEntry(t, st, "a.txt", false, true)
	assertEntry(t, st, "b.txt", true, false)
	assertEntry(t, st, "c.txt", false, true)
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "changed\n" {
		t.Fatalf("worktree changed: %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file came back: %v", err)
	}
	if branch := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "main" {
		t.Fatalf("branch = %q", branch)
	}
}

func TestDiscardPaths(t *testing.T) {
	if got := discardPaths([]string{"a.txt", "a.txt", " ", "."}, false); len(got) != 1 || got[0] != "a.txt" {
		t.Fatalf("worktree paths = %#v", got)
	}
	got := discardPaths([]string{"old.txt -> new.txt"}, true)
	if len(got) != 2 || got[0] != "old.txt" || got[1] != "new.txt" {
		t.Fatalf("staged rename = %#v", got)
	}
	got = discardPaths([]string{"old.txt -> new.txt"}, false)
	if len(got) != 1 || got[0] != "new.txt" {
		t.Fatalf("worktree rename = %#v", got)
	}
}

func TestDiscard(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	writeCommit(t, dir, "gone.txt", "keep")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "a.txt")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Discard(ctx, []string{"a.txt", "new.txt", "gone.txt"}, false); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "staged\n" {
		t.Fatalf("worktree discard = %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatalf("deleted file not restored: %v", err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertEntry(t, st, "a.txt", true, false)

	if err := repo.Discard(ctx, []string{"a.txt"}, true); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "base\n" {
		t.Fatalf("staged discard = %q", body)
	}
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Dirty() {
		t.Fatalf("status after staged discard: %v", st.Entries)
	}
}

func TestDiscardAddedAndRename(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "old.txt", "from")
	gitAt(t, dir, "mv", "old.txt", "new.txt")
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "added.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var rename string
	for _, e := range st.Entries {
		if strings.Contains(e.Path, "new.txt") && strings.Contains(e.Path, "old.txt") {
			rename = e.Path
		}
	}
	if rename == "" {
		t.Fatalf("rename entry missing: %v", st.Entries)
	}
	if err := repo.Discard(ctx, []string{rename, "added.txt"}, true); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "old.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "from\n" {
		t.Fatalf("renamed file = %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("rename destination remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "added.txt")); !os.IsNotExist(err) {
		t.Fatalf("added file remains: %v", err)
	}
}

func TestDiscardWithoutHEAD(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "a.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Discard(ctx, []string{"a.txt"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("added file remains: %v", err)
	}
}

func TestStageSelectedLines(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\nbeta\ngamma\nomega\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "a.txt")
	gitAt(t, dir, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\nBETA\ngamma\ndelta\nomega\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	unstaged, err := repo.DiffWork(ctx, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(unstaged, "+delta\n")
	if start < 0 {
		t.Fatalf("diff missing delta:\n%s", unstaged)
	}
	patch, ok := PatchForLines(unstaged, start, start+len("+delta\n"), false)
	if !ok {
		t.Fatal("no patch")
	}
	if err := repo.ApplyIndex(ctx, patch, false); err != nil {
		t.Fatalf("apply %v\n%s", err, patch)
	}
	staged, err := repo.DiffWork(ctx, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(staged, "+delta\n") {
		t.Fatalf("staged diff:\n%s", staged)
	}
	if strings.Contains(staged, "+BETA\n") || strings.Contains(staged, "-beta\n") {
		t.Fatalf("replacement was staged:\n%s", staged)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "alpha\nBETA\ngamma\ndelta\nomega\n" {
		t.Fatalf("worktree changed: %q", body)
	}

	newDiff, err := repo.DiffWork(ctx, "c.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	start = strings.Index(newDiff, "+one\n")
	if start < 0 {
		t.Fatalf("new file diff:\n%s", newDiff)
	}
	patch, ok = PatchForLines(newDiff, start, start+len("+one\n"), false)
	if !ok {
		t.Fatal("no new-file patch")
	}
	if err := repo.ApplyIndex(ctx, patch, false); err != nil {
		t.Fatalf("apply new %v\n%s", err, patch)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertEntry(t, st, "c.txt", true, true)

	cached, err := repo.DiffWork(ctx, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	start = strings.Index(cached, "+delta\n")
	if start < 0 {
		t.Fatalf("cached diff:\n%s", cached)
	}
	patch, ok = PatchForLines(cached, start, start+len("+delta\n"), true)
	if !ok {
		t.Fatal("no unstage patch")
	}
	if err := repo.ApplyIndex(ctx, patch, true); err != nil {
		t.Fatalf("unstage %v\n%s", err, patch)
	}
	left, err := repo.DiffWork(ctx, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(left) != "" {
		t.Fatalf("delta still staged:\n%s", left)
	}
}

func assertEntry(t *testing.T, st Status, path string, staged, unstaged bool) {
	t.Helper()
	for _, e := range st.Entries {
		if e.Path != path {
			continue
		}
		if e.Staged() != staged || e.Unstaged() != unstaged {
			t.Fatalf("%s staged=%v unstaged=%v code=%q", path, e.Staged(), e.Unstaged(), e.Code)
		}
		return
	}
	t.Fatalf("missing %s", path)
}

func hasDecoration(ds []Decoration, kind, name string) bool {
	for _, d := range ds {
		if d.Kind == kind && d.Name == name {
			return true
		}
	}
	return false
}

func TestSnapshotHistoryPages(t *testing.T) {
	dir := initRepo(t)
	for i := 0; i < 6; i++ {
		commitEmptyAt(t, dir, fmt.Sprintf("c%d", i), fmt.Sprintf("2020-01-01 00:00:%02d +0000", i))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.Snapshot(ctx, SnapshotOpts{MaxCommits: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Commits) != 3 || !page.MoreCommits {
		t.Fatalf("page commits=%d more=%v", len(page.Commits), page.MoreCommits)
	}
	if page.Commits[0].Subject != "c5" || page.Commits[2].Subject != "c3" {
		t.Fatalf("page subjects = %q %q %q", page.Commits[0].Subject, page.Commits[1].Subject, page.Commits[2].Subject)
	}
	all, err := repo.Snapshot(ctx, SnapshotOpts{MaxCommits: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Commits) != 6 || all.MoreCommits {
		t.Fatalf("all commits=%d more=%v", len(all.Commits), all.MoreCommits)
	}
	for i := range page.Commits {
		if page.Commits[i].Hash != all.Commits[i].Hash {
			t.Fatalf("prefix mismatch at %d", i)
		}
	}
	def, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Commits) != 6 || def.MoreCommits {
		t.Fatalf("default commits=%d more=%v", len(def.Commits), def.MoreCommits)
	}
}

func commitEmptyAt(t *testing.T, dir, msg, when string) {
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

func TestSnapshotListsOtherWorktrees(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "README", "root")
	other := filepath.Join(t.TempDir(), "feature")
	gitAt(t, dir, "worktree", "add", "-b", "feature", other)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Worktrees) != 1 || snap.Worktrees[0].Branch != "feature" {
		t.Fatalf("worktrees = %+v", snap.Worktrees)
	}
	if !sameWorktree(snap.Worktrees[0].Path, other, false) {
		t.Fatalf("path = %q want %q", snap.Worktrees[0].Path, other)
	}
	st, err := repo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Worktrees != WorktreeKey(snap.Worktrees) {
		t.Fatalf("state digest = %q snap %q", st.Worktrees, WorktreeKey(snap.Worktrees))
	}

	linked, err := Open(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	fromOther, err := linked.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fromOther.Worktrees) != 1 || fromOther.Worktrees[0].Branch != "main" {
		t.Fatalf("from feature = %+v", fromOther.Worktrees)
	}
}

func TestSnapshotListsSubmodules(t *testing.T) {
	parent := initRepo(t)
	writeCommit(t, parent, "README", "parent")
	const hash = "0123456789abcdef0123456789abcdef01234567"
	gitAt(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+hash+",vendor/lib")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Submodules) != 1 || snap.Submodules[0].Path != "vendor/lib" || snap.Submodules[0].Hash != hash {
		t.Fatalf("submodules = %#v", snap.Submodules)
	}
	if !strings.Contains(snap.RefDigest, "gitlink "+hash+" vendor/lib") {
		t.Fatalf("digest = %q", snap.RefDigest)
	}
}

func TestWholeFileDiffIncludesSkippedLines(t *testing.T) {
	dir := initRepo(t)
	var b strings.Builder
	for i := 1; i <= 15; i++ {
		switch i {
		case 1:
			b.WriteString("HEAD_LINE\n")
		case 8:
			b.WriteString("MIDDLE\n")
		case 15:
			b.WriteString("TAIL_LINE\n")
		default:
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", "a.txt")
	gitAt(t, dir, "commit", "-m", "base")
	changed := strings.ReplaceAll(b.String(), "MIDDLE\n", "CHANGED\n")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := repo.DiffWork(ctx, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(narrow, "HEAD_LINE") || strings.Contains(narrow, "TAIL_LINE") {
		t.Fatalf("narrow diff already includes the ends:\n%s", narrow)
	}
	wide, err := repo.DiffWorkWhole(ctx, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wide, " HEAD_LINE\n") || !strings.Contains(wide, " TAIL_LINE\n") || !strings.Contains(wide, "+CHANGED\n") {
		t.Fatalf("wide diff:\n%s", wide)
	}

	gitAt(t, dir, "add", "a.txt")
	gitAt(t, dir, "commit", "-m", "change")
	hash := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	shown, err := repo.ShowFileWhole(ctx, hash, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown, " HEAD_LINE\n") || !strings.Contains(shown, " TAIL_LINE\n") || !strings.Contains(shown, "+CHANGED\n") {
		t.Fatalf("commit diff:\n%s", shown)
	}
}

func TestPushArgs(t *testing.T) {
	cases := []struct {
		name                     string
		mode                     PushMode
		hasUp                    bool
		remote, upstream, branch string
		want                     []string
	}{
		{"normal missing", PushNormal, false, "", "", "main", []string{"push", "-u", "origin", "main"}},
		{"lease missing", PushForceWithLease, false, "", "", "main", []string{"push", "--force-with-lease", "-u", "origin", "main"}},
		{"force missing", PushForce, false, "", "", "topic", []string{"push", "--force", "-u", "origin", "topic"}},
		{"normal upstream", PushNormal, true, "origin", "main", "main", []string{"push", "origin", "HEAD:main"}},
		{"lease upstream", PushForceWithLease, true, "origin", "main", "main", []string{"push", "--force-with-lease", "origin", "HEAD:main"}},
		{"force other remote", PushForce, true, "upstream", "feature", "local", []string{"push", "--force", "upstream", "HEAD:feature"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pushArgs(tc.mode, tc.hasUp, tc.remote, tc.upstream, tc.branch)
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("args = %#v, want %#v", got, tc.want)
			}
			if strings.Contains(strings.Join(got, " "), "--force-with-lease=") {
				t.Fatalf("lease must not pin a hash: %#v", got)
			}
		})
	}
}

func TestPushBranchArgs(t *testing.T) {
	cases := []struct {
		name                     string
		mode                     PushMode
		hasUp                    bool
		remote, upstream, branch string
		want                     []string
	}{
		{"normal missing", PushNormal, false, "", "", "feature/side", []string{"push", "-u", "origin", "feature/side"}},
		{"lease missing", PushForceWithLease, false, "", "", "main", []string{"push", "--force-with-lease", "-u", "origin", "main"}},
		{"force missing", PushForce, false, "", "", "topic", []string{"push", "--force", "-u", "origin", "topic"}},
		{"normal upstream", PushNormal, true, "origin", "main", "main", []string{"push", "origin", "main:main"}},
		{"lease other name", PushForceWithLease, true, "origin", "topic", "feature", []string{"push", "--force-with-lease", "origin", "feature:topic"}},
		{"force other remote", PushForce, true, "upstream", "topic", "feature", []string{"push", "--force", "upstream", "feature:topic"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pushBranchArgs(tc.mode, tc.hasUp, tc.remote, tc.upstream, tc.branch)
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("args = %#v, want %#v", got, tc.want)
			}
			if strings.Contains(strings.Join(got, " "), "HEAD:") {
				t.Fatalf("branch push must not use HEAD: %#v", got)
			}
			if strings.Contains(strings.Join(got, " "), "--force-with-lease=") {
				t.Fatalf("lease must not pin a hash: %#v", got)
			}
		})
	}
}

func TestPushArgsOnRemoteGit(t *testing.T) {
	lease := pushArgs(PushForceWithLease, true, "origin", "main", "main")
	cmd := gitCmd(context.Background(), Loc{Dest: "dev", Dir: "/work/app"}, lease...)
	if len(cmd.Args) == 0 || cmd.Args[0] != "ssh" {
		t.Fatalf("args = %#v", cmd.Args)
	}
	remote := cmd.Args[len(cmd.Args)-1]
	leaseAt := strings.Index(remote, "'--force-with-lease'")
	originAt := strings.Index(remote, "'origin'")
	headAt := strings.Index(remote, "'HEAD:main'")
	if leaseAt < 0 || originAt < 0 || headAt < 0 || leaseAt > originAt || originAt > headAt {
		t.Fatalf("remote = %q", remote)
	}
	if strings.Contains(remote, "--force-with-lease=") {
		t.Fatalf("pinned lease: %q", remote)
	}

	force := pushArgs(PushForce, false, "", "", "main")
	cmd = gitCmd(context.Background(), Loc{Dest: "dev", Dir: "/work/app"}, force...)
	remote = cmd.Args[len(cmd.Args)-1]
	forceAt := strings.Index(remote, "'--force'")
	uAt := strings.Index(remote, "'-u'")
	originAt = strings.Index(remote, "'origin'")
	branchAt := strings.Index(remote, "'main'")
	if forceAt < 0 || uAt < 0 || originAt < 0 || branchAt < 0 || forceAt > uAt || uAt > originAt || originAt > branchAt {
		t.Fatalf("remote = %q", remote)
	}
	if strings.Contains(remote, "force-with-lease") {
		t.Fatalf("lease flag on force push: %q", remote)
	}
}

func TestPushBranchArgsOnRemoteGit(t *testing.T) {
	lease := pushBranchArgs(PushForceWithLease, true, "upstream", "topic", "feature")
	cmd := gitCmd(context.Background(), Loc{Dest: "dev", Dir: "/work/app"}, lease...)
	if len(cmd.Args) == 0 || cmd.Args[0] != "ssh" {
		t.Fatalf("args = %#v", cmd.Args)
	}
	remote := cmd.Args[len(cmd.Args)-1]
	leaseAt := strings.Index(remote, "'--force-with-lease'")
	remoteAt := strings.Index(remote, "'upstream'")
	specAt := strings.Index(remote, "'feature:topic'")
	if leaseAt < 0 || remoteAt < 0 || specAt < 0 || leaseAt > remoteAt || remoteAt > specAt {
		t.Fatalf("remote = %q", remote)
	}
	if strings.Contains(remote, "HEAD:") || strings.Contains(remote, "--force-with-lease=") {
		t.Fatalf("remote = %q", remote)
	}

	normal := pushBranchArgs(PushNormal, false, "", "", "feature/side")
	cmd = gitCmd(context.Background(), Loc{Dest: "dev", Dir: "/work/app"}, normal...)
	remote = cmd.Args[len(cmd.Args)-1]
	uAt := strings.Index(remote, "'-u'")
	originAt := strings.Index(remote, "'origin'")
	branchAt := strings.Index(remote, "'feature/side'")
	if uAt < 0 || originAt < 0 || branchAt < 0 || uAt > originAt || originAt > branchAt {
		t.Fatalf("remote = %q", remote)
	}
}

func TestPushRejectsEmptyAndDetached(t *testing.T) {
	dir := initRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []PushMode{PushNormal, PushForceWithLease, PushForce} {
		err := repo.PushMode(ctx, mode)
		if err == nil || !strings.Contains(err.Error(), "no commits") {
			t.Fatalf("mode %d empty: %v", mode, err)
		}
	}
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "--detach")
	for _, mode := range []PushMode{PushNormal, PushForceWithLease, PushForce} {
		err := repo.PushMode(ctx, mode)
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("mode %d detached: %v", mode, err)
		}
	}
}

func TestPushSetsUpstream(t *testing.T) {
	modes := []struct {
		name string
		mode PushMode
	}{
		{"normal", PushNormal},
		{"lease", PushForceWithLease},
		{"force", PushForce},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeCommit(t, dir, "a.txt", "base")
			remote := t.TempDir()
			gitAt(t, remote, "init", "--bare", "-b", "main")
			gitAt(t, dir, "remote", "add", "origin", remote)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			repo, err := Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.PushMode(ctx, tc.mode); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "@{upstream}")); got != "origin/main" {
				t.Fatalf("upstream = %q", got)
			}
			want := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
			if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != want {
				t.Fatalf("remote = %s want %s", got, want)
			}
		})
	}
}

func TestPushKeepsUpstreamDestination(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main:topic")
	writeCommit(t, dir, "b.txt", "next")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "@{upstream}")); got != "origin/topic" {
		t.Fatalf("upstream = %q", got)
	}
	if err := repo.PushMode(ctx, PushForce); err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "topic")); got != want {
		t.Fatalf("topic = %s want %s", got, want)
	}
	if out, err := exec.Command("git", "-C", remote, "rev-parse", "--verify", "--quiet", "main").CombinedOutput(); err == nil {
		t.Fatalf("push created main: %s", out)
	}
}

func TestPushForceModes(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	writeCommit(t, dir, "b.txt", "local")

	other := t.TempDir()
	gitAt(t, other, "clone", remote, ".")
	gitAt(t, other, "config", "user.name", "Test")
	gitAt(t, other, "config", "user.email", "test@example.com")
	gitAt(t, other, "config", "commit.gpgsign", "false")
	writeCommit(t, other, "a.txt", "rewritten")
	gitAt(t, other, "push", "--force", "origin", "main")
	gitAt(t, dir, "fetch", "origin")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	remoteBefore := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main"))
	if err := repo.Push(ctx); err == nil {
		t.Fatal("normal push should refuse a diverged history")
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != remoteBefore {
		t.Fatalf("refused push moved remote %s -> %s", remoteBefore, got)
	}
	if err := repo.PushMode(ctx, PushForceWithLease); err != nil {
		t.Fatal(err)
	}
	local := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != local {
		t.Fatalf("lease remote = %s want %s", got, local)
	}

	writeCommit(t, other, "d.txt", "later")
	gitAt(t, other, "push", "--force", "origin", "main")
	moved := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main"))
	if err := repo.PushMode(ctx, PushForceWithLease); err == nil {
		t.Fatal("lease should refuse a remote that moved after fetch")
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != moved {
		t.Fatalf("refused lease moved remote %s -> %s", moved, got)
	}
	if err := repo.PushMode(ctx, PushForce); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != local {
		t.Fatalf("force remote = %s want %s", got, local)
	}
}

func TestPushBranchSetsUpstream(t *testing.T) {
	modes := []struct {
		name string
		mode PushMode
	}{
		{"normal", PushNormal},
		{"lease", PushForceWithLease},
		{"force", PushForce},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeCommit(t, dir, "a.txt", "base")
			remote := t.TempDir()
			gitAt(t, remote, "init", "--bare", "-b", "main")
			gitAt(t, dir, "remote", "add", "origin", remote)
			gitAt(t, dir, "checkout", "-b", "feature/side")
			writeCommit(t, dir, "b.txt", "side")
			gitAt(t, dir, "checkout", "main")

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			repo, err := Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.PushBranch(ctx, "feature/side", tc.mode); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
				t.Fatalf("HEAD = %q", got)
			}
			if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "feature/side@{upstream}")); got != "origin/feature/side" {
				t.Fatalf("upstream = %q", got)
			}
			want := strings.TrimSpace(gitAt(t, dir, "rev-parse", "feature/side"))
			if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "feature/side")); got != want {
				t.Fatalf("remote = %s want %s", got, want)
			}
			if _, err := exec.Command("git", "-C", remote, "rev-parse", "--verify", "--quiet", "main").CombinedOutput(); err == nil {
				t.Fatalf("push created main")
			}
		})
	}
}

func TestPushBranchUsesOwnUpstream(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	origin := t.TempDir()
	gitAt(t, origin, "init", "--bare", "-b", "main")
	other := t.TempDir()
	gitAt(t, other, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", origin)
	gitAt(t, dir, "remote", "add", "upstream", other)
	gitAt(t, dir, "push", "-u", "origin", "main")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "b.txt", "feature")
	gitAt(t, dir, "push", "-u", "upstream", "feature:topic")
	gitAt(t, dir, "checkout", "main")
	writeCommit(t, dir, "c.txt", "main-local")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	mainHead := strings.TrimSpace(gitAt(t, dir, "rev-parse", "main"))
	featureHead := strings.TrimSpace(gitAt(t, dir, "rev-parse", "feature"))
	originMain := strings.TrimSpace(gitAt(t, origin, "rev-parse", "main"))
	if err := repo.PushBranch(ctx, "feature", PushNormal); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
		t.Fatalf("HEAD = %q", got)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "main")); got != mainHead {
		t.Fatalf("main moved %s -> %s", mainHead, got)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "feature")); got != featureHead {
		t.Fatalf("feature moved %s -> %s", featureHead, got)
	}
	if got := strings.TrimSpace(gitAt(t, other, "rev-parse", "topic")); got != featureHead {
		t.Fatalf("topic = %s want %s", got, featureHead)
	}
	if got := strings.TrimSpace(gitAt(t, origin, "rev-parse", "main")); got != originMain {
		t.Fatalf("origin/main = %s want %s", got, originMain)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "feature@{upstream}")); got != "upstream/topic" {
		t.Fatalf("feature upstream = %q", got)
	}
	if out, err := exec.Command("git", "-C", origin, "rev-parse", "--verify", "--quiet", "feature").CombinedOutput(); err == nil {
		t.Fatalf("push created origin/feature: %s", out)
	}

	gitAt(t, dir, "checkout", "feature")
	writeCommit(t, dir, "d.txt", "feature-next")
	next := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))
	if err := repo.PushBranch(ctx, "feature", PushNormal); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "feature" {
		t.Fatalf("HEAD = %q", got)
	}
	if got := strings.TrimSpace(gitAt(t, other, "rev-parse", "topic")); got != next {
		t.Fatalf("checked-out push topic = %s want %s", got, next)
	}
	if got := strings.TrimSpace(gitAt(t, origin, "rev-parse", "main")); got != originMain {
		t.Fatalf("checked-out push moved origin/main %s -> %s", originMain, got)
	}
}

func TestPushBranchWhileDetached(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	writeCommit(t, dir, "b.txt", "next")
	want := strings.TrimSpace(gitAt(t, dir, "rev-parse", "main"))
	gitAt(t, dir, "checkout", "--detach", "HEAD~1")
	detached := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PushMode(ctx, PushNormal); err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("toolbar push: %v", err)
	}
	if err := repo.PushBranch(ctx, "main", PushNormal); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != want {
		t.Fatalf("remote = %s want %s", got, want)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "HEAD")); got != detached {
		t.Fatalf("HEAD = %s want detached %s", got, detached)
	}
	if err := exec.Command("git", "-C", dir, "symbolic-ref", "--quiet", "HEAD").Run(); err == nil {
		t.Fatal("push attached HEAD")
	}
}

func TestPushBranchForceModes(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	remote := t.TempDir()
	gitAt(t, remote, "init", "--bare", "-b", "main")
	gitAt(t, dir, "remote", "add", "origin", remote)
	gitAt(t, dir, "push", "-u", "origin", "main")
	writeCommit(t, dir, "b.txt", "local")
	gitAt(t, dir, "checkout", "-b", "topic")

	other := t.TempDir()
	gitAt(t, other, "clone", remote, ".")
	gitAt(t, other, "config", "user.name", "Test")
	gitAt(t, other, "config", "user.email", "test@example.com")
	gitAt(t, other, "config", "commit.gpgsign", "false")
	writeCommit(t, other, "a.txt", "rewritten")
	gitAt(t, other, "push", "--force", "origin", "main")
	gitAt(t, dir, "fetch", "origin")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	remoteBefore := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main"))
	local := strings.TrimSpace(gitAt(t, dir, "rev-parse", "main"))
	if err := repo.PushBranch(ctx, "main", PushNormal); err == nil {
		t.Fatal("normal push should refuse a diverged history")
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != remoteBefore {
		t.Fatalf("refused push moved remote %s -> %s", remoteBefore, got)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "topic" {
		t.Fatalf("HEAD = %q", got)
	}
	if err := repo.PushBranch(ctx, "main", PushForceWithLease); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != local {
		t.Fatalf("lease remote = %s want %s", got, local)
	}

	writeCommit(t, other, "d.txt", "later")
	gitAt(t, other, "push", "--force", "origin", "main")
	moved := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main"))
	if err := repo.PushBranch(ctx, "main", PushForceWithLease); err == nil {
		t.Fatal("lease should refuse a remote that moved after fetch")
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != moved {
		t.Fatalf("refused lease moved remote %s -> %s", moved, got)
	}
	if err := repo.PushBranch(ctx, "main", PushForce); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "rev-parse", "main")); got != local {
		t.Fatalf("force remote = %s want %s", got, local)
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "topic" {
		t.Fatalf("HEAD = %q", got)
	}
}

func TestPushBranchRejectsMissing(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "  ", "no-such"} {
		err := repo.PushBranch(ctx, name, PushForce)
		if err == nil || !strings.Contains(err.Error(), "no branch") {
			t.Fatalf("branch %q: %v", name, err)
		}
	}
	if got := strings.TrimSpace(gitAt(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
		t.Fatalf("HEAD = %q", got)
	}
}

func commitHasDecoration(cs []Commit, kind, name string) bool {
	for _, c := range cs {
		if hasDecoration(c.Decorations, kind, name) {
			return true
		}
	}
	return false
}
