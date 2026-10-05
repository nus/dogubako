package gitcli

import (
	"context"
	"errors"
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

func TestCheckoutDirtyKeepsBranch(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "base")
	gitAt(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "f.txt", "feat")
	gitAt(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, Ref{Name: "feature"}); !errors.Is(err, ErrLocalChanges) {
		t.Fatalf("checkout err = %v", err)
	}
	snap, err := repo.Snapshot(ctx, SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" || snap.Detached {
		t.Fatalf("branch = %s detached=%v", snap.Branch, snap.Detached)
	}
	found := false
	for i := range snap.Graph {
		if snap.Graph[i].Commit.Hash != Uncommitted {
			continue
		}
		if i+1 >= len(snap.Graph) || snap.Graph[i+1].Commit.Hash != snap.HEAD {
			t.Fatalf("uncommitted is not directly above HEAD")
		}
		u := snap.Graph[i]
		h := snap.Graph[i+1]
		if !edgeGray(u.Outgoing, u.Lane, h.Lane) || !edgeGray(h.Incoming, u.Lane, h.Lane) {
			t.Fatalf("sprout %#v -> %#v", u.Outgoing, h.Incoming)
		}
		found = true
	}
	if !found {
		t.Fatal("missing uncommitted node")
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

func commitHasDecoration(cs []Commit, kind, name string) bool {
	for _, c := range cs {
		if hasDecoration(c.Decorations, kind, name) {
			return true
		}
	}
	return false
}
