package gitcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nus/dogubako/internal/sshremote"
)

const (
	defaultMaxLog = 500
	maxWalk       = 20000
)

// Repo is a git working tree. Operations run the git binary in Dir.
// Dest is an ssh destination when the tree is on another host. Port is set
// only for an explicit destination, not for a Host alias from ssh config.
type Repo struct {
	Dir  string
	Dest string
	Port string
}

func (r *Repo) loc() Loc {
	return Loc{Dest: r.Dest, Port: r.Port, Dir: r.Dir}
}

// Key identifies the repository for tabs and the recent list.
func (r *Repo) Key() string { return r.loc().Key() }

// IsRemote reports whether git runs over ssh.
func (r *Repo) IsRemote() bool { return r.Dest != "" }

// Open resolves path to a work tree root. path may be a local directory or a remote Loc key.
func Open(ctx context.Context, repoPath string) (*Repo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	loc := ParseLoc(repoPath)
	if !loc.IsRemote() {
		loc.Dir = strings.TrimSpace(repoPath)
	}
	if strings.TrimSpace(loc.Dir) == "" {
		return nil, fmt.Errorf("not a git repository")
	}
	inside, err := gitOutput(ctx, loc, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		if err == nil {
			err = fmt.Errorf("not a work tree")
		}
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	top, err := gitOutput(ctx, loc, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	top = strings.TrimSpace(top)
	if loc.IsRemote() {
		top = path.Clean(top)
	} else {
		top = filepath.Clean(top)
	}
	return &Repo{Dir: top, Dest: loc.Dest, Port: loc.Port}, nil
}

// SnapshotOpts selects which refs seed the commit graph.
type SnapshotOpts struct {
	HiddenRemotes map[string]bool
	MaxCommits    int
}

// Snapshot is the graph, branches, remotes, and working-tree status.
type Snapshot struct {
	HEAD           string
	Branch         string
	Detached       bool
	Commits        []Commit
	Graph          []GraphRow
	Lanes          int
	Locals         []Ref
	Remotes        []Remote
	RemoteBranches []Ref
	Tags           []Ref
	Status         Status
	RefDigest      string
}

// State is the part of a repository that the graph must follow.
// It is cheaper than a full Snapshot and is safe to poll.
type State struct {
	Head   string
	Refs   string
	Status Status
}

func (s Snapshot) RefList() []Ref {
	out := make([]Ref, 0, len(s.Locals)+len(s.RemoteBranches))
	out = append(out, s.Locals...)
	out = append(out, s.RemoteBranches...)
	return out
}

// Snapshot loads refs and a bounded commit graph.
func (r *Repo) Snapshot(ctx context.Context, opts SnapshotOpts) (Snapshot, error) {
	var s Snapshot
	if err := ctx.Err(); err != nil {
		return s, err
	}
	hash, branch, detached, hasHead, err := r.head(ctx)
	if err != nil {
		return s, err
	}
	s.Branch = branch
	s.Detached = detached || branch == ""
	if hasHead {
		s.HEAD = hash
	}

	s.Remotes = r.remoteConfigs(ctx)
	locals, remoteBranches, tags, decos, err := r.listRefs(ctx, branch, detached, hash, s.Remotes)
	if err != nil {
		return s, err
	}
	s.Locals = locals
	s.RemoteBranches = remoteBranches
	s.Tags = tags

	max := opts.MaxCommits
	if max <= 0 {
		max = defaultMaxLog
	}
	var tips []string
	if hasHead {
		tips = append(tips, hash)
	}
	for _, ref := range locals {
		tips = append(tips, ref.Hash)
	}
	for _, ref := range tags {
		tips = append(tips, ref.Hash)
	}
	for _, ref := range remoteBranches {
		if opts.HiddenRemotes[ref.Remote] {
			continue
		}
		tips = append(tips, ref.Hash)
	}
	if len(tips) > 0 {
		commits, err := r.history(ctx, tips, max)
		if err != nil {
			return s, err
		}
		for i := range commits {
			commits[i].Decorations = decos[commits[i].Hash]
			if len(opts.HiddenRemotes) > 0 {
				commits[i].Decorations = filterDecorations(commits[i].Decorations, opts.HiddenRemotes)
			}
		}
		s.Commits = commits
	}

	s.Status, err = r.status(ctx)
	if err != nil {
		return s, err
	}
	if s.Status.Branch != "" && !s.Status.Detached {
		s.Branch = s.Status.Branch
	}
	s.Detached = s.Status.Detached || s.Branch == ""
	s.RefDigest, err = r.refDigest(ctx)
	if err != nil {
		return s, err
	}
	ApplyWorkTree(&s)
	return s, nil
}

// State reads HEAD, refs, and the working tree without walking history.
func (r *Repo) State(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	var st State
	hash, _, _, hasHead, err := r.head(ctx)
	if err != nil {
		return State{}, err
	}
	if hasHead {
		st.Head = hash
	}
	st.Refs, err = r.refDigest(ctx)
	if err != nil {
		return State{}, err
	}
	st.Status, err = r.status(ctx)
	return st, err
}

func (r *Repo) refDigest(ctx context.Context) (string, error) {
	out, err := r.output(ctx, "for-each-ref", "--format=%(refname) %(if)%(symref)%(then)%(symref)%(else)%(objectname)%(end)")
	if err != nil {
		return "", err
	}
	var lines []string
	for _, line := range splitLines(out) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	sym, symErr := r.output(ctx, "symbolic-ref", "--quiet", "HEAD")
	if symErr == nil {
		lines = append(lines, "HEAD "+strings.TrimSpace(sym))
	} else if err := ctx.Err(); err != nil {
		return "", err
	} else if h, hashErr := r.output(ctx, "rev-parse", "--verify", "--quiet", "HEAD"); hashErr == nil {
		lines = append(lines, "HEAD "+strings.TrimSpace(h))
	} else if err := ctx.Err(); err != nil {
		return "", err
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

// ApplyWorkTree rebuilds the graph lanes from the commits already loaded,
// including the gray uncommitted node when the worktree is dirty.
func ApplyWorkTree(s *Snapshot) {
	s.Graph = LayoutGraph(s.Commits)
	s.Lanes = LaneCount(s.Graph)
	if s.Status.Dirty() && s.HEAD != "" && len(s.Commits) > 0 {
		s.Graph = WithUncommitted(s.Commits, s.HEAD)
		s.Lanes = LaneCount(s.Graph)
	}
}

func (r *Repo) Status(ctx context.Context) (Status, error) {
	return r.status(ctx)
}

func (r *Repo) status(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	out, err := r.output(ctx, "status", "--porcelain=v1", "-b", "-uall")
	if err != nil {
		return Status{}, err
	}
	st := parseStatus(out)
	if st.Detached {
		st.Branch = ""
	}
	sort.Slice(st.Entries, func(i, j int) bool { return st.Entries[i].Path < st.Entries[j].Path })
	return st, nil
}

// Detail loads the full message and name-status for hash.
func (r *Repo) Detail(ctx context.Context, hash string) (CommitDetail, error) {
	if err := ctx.Err(); err != nil {
		return CommitDetail{}, err
	}
	meta, err := r.output(ctx, "log", "-1", "--no-notes", "--no-decorate", "--pretty=tformat:%H%x1f%P%x1f%an%x1f%ae%x1f%aI%x1f%cI%x1f%s", hash)
	if err != nil {
		return CommitDetail{}, err
	}
	set := parseHistory(meta)
	var c *histCommit
	for _, item := range set {
		c = item
		break
	}
	if c == nil {
		return CommitDetail{}, fmt.Errorf("unknown revision %s", hash)
	}
	body, err := r.output(ctx, "log", "-1", "--format=%B", "--no-notes", c.Hash)
	if err != nil {
		return CommitDetail{}, err
	}
	files, err := r.output(ctx, "diff-tree", "--no-commit-id", "--name-status", "-r", "--root", c.Hash)
	if err != nil {
		return CommitDetail{}, err
	}
	diff, err := r.output(ctx, "show", "--no-notes", "--no-color", "--format=", "--patch", c.Hash)
	if err != nil {
		return CommitDetail{}, err
	}
	headHash, branch, detached, _, _ := r.head(ctx)
	rems := r.remoteConfigs(ctx)
	_, _, _, decos, err := r.listRefs(ctx, branch, detached, headHash, rems)
	if err != nil {
		decos = nil
	}
	c.Decorations = decos[c.Hash]
	return CommitDetail{
		Commit: c.Commit,
		Body:   strings.TrimSpace(body),
		Files:  parseNameStatus(files),
		Diff:   diff,
	}, nil
}

// Commit creates a commit. amend rewrites HEAD.
func (r *Repo) Commit(ctx context.Context, message string, stageAll, amend bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	message = strings.TrimSpace(message)
	if !amend && message == "" {
		return fmt.Errorf("commit message is empty")
	}
	if stageAll {
		if err := r.run(ctx, "add", "-A"); err != nil {
			return err
		}
	}
	if amend && message == "" {
		return r.run(ctx, "commit", "--amend", "--no-edit")
	}
	args := []string{"commit"}
	if amend {
		args = append(args, "--amend")
	}
	args = append(args, "-m", message)
	return r.run(ctx, args...)
}

// Stage adds paths to the index. A missing worktree file is recorded as a deletion.
func (r *Repo) Stage(ctx context.Context, paths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths")
	}
	args := append([]string{"add", "--"}, paths...)
	return r.run(ctx, args...)
}

// Unstage restores the index for paths from HEAD and leaves the worktree as it is.
func (r *Repo) Unstage(ctx context.Context, paths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths")
	}
	args := append([]string{"reset", "-q", "HEAD", "--"}, paths...)
	return r.run(ctx, args...)
}

// DiffWork returns the patch for path. staged reads the index; otherwise the worktree.
// An untracked file is diffed against /dev/null.
func (r *Repo) DiffWork(ctx context.Context, path string, staged bool) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	args := []string{"diff", "--no-color", "--patch"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, err := r.diffOutput(ctx, args...)
	if err != nil || staged || strings.TrimSpace(out) != "" {
		return out, err
	}
	if !r.exists(ctx, path) {
		return "", nil
	}
	return r.diffOutput(ctx, "diff", "--no-color", "--no-index", "--patch", "--", "/dev/null", path)
}

// ApplyIndex applies patch to the index and leaves the worktree as it is.
// reverse undoes a staged patch.
func (r *Repo) ApplyIndex(ctx context.Context, patch string, reverse bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("empty patch")
	}
	if !strings.HasSuffix(patch, "\n") {
		patch += "\n"
	}
	args := []string{"apply", "--cached", "--recount", "--unidiff-zero", "--whitespace=nowarn"}
	if reverse {
		args = append(args, "--reverse")
	}
	args = append(args, "-")
	cmd := gitCmd(ctx, r.loc(), args...)
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return errors.New(msg)
	}
	return nil
}

func (r *Repo) diffOutput(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cmd := gitCmd(ctx, r.loc(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil || exitCode(err) == 1 {
		return stdout.String(), nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = strings.TrimSpace(stdout.String())
	}
	if msg == "" {
		return "", err
	}
	return "", errors.New(msg)
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// Fetch downloads objects from every remote.
func (r *Repo) Fetch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	out, err := r.output(ctx, "remote")
	if err != nil {
		return err
	}
	for _, name := range splitLines(out) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := r.run(ctx, "fetch", "--prune", name); err != nil {
			return err
		}
	}
	return nil
}

// Pull fetches and fast-forwards the upstream of HEAD.
func (r *Repo) Pull(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, _, ok, err := r.upstream(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no upstream configured")
	}
	return r.run(ctx, "pull", "--ff-only")
}

// PullRef fetches the remote branch and fast-forwards HEAD onto it.
func (r *Repo) PullRef(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ref.Remote == "" {
		return fmt.Errorf("no remote")
	}
	branch := ref.LocalName()
	if branch == "" || branch == "HEAD" {
		return fmt.Errorf("no branch")
	}
	return r.run(ctx, "pull", "--ff-only", ref.Remote, branch)
}

// Push sends the current branch. Sets upstream to origin when missing.
func (r *Repo) Push(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, branch, detached, ok, err := r.head(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no commits")
	}
	if detached || branch == "" {
		return fmt.Errorf("detached HEAD")
	}
	remote, upstream, hasUp, err := r.upstream(ctx)
	if err != nil {
		return err
	}
	if !hasUp {
		return r.run(ctx, "push", "-u", "origin", branch)
	}
	return r.run(ctx, "push", remote, "HEAD:"+upstream)
}

// CreateTag adds a lightweight tag at hash.
func (r *Repo) CreateTag(ctx context.Context, name, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := r.tagRef(ctx, name)
	if err != nil {
		return err
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return fmt.Errorf("unknown revision")
	}
	resolved, err := r.output(ctx, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("unknown revision %s", hash)
	}
	return r.run(ctx, "tag", "--", strings.TrimPrefix(ref, "refs/tags/"), strings.TrimSpace(resolved))
}

// PushTag sends one local tag to the upstream remote, or to origin when HEAD has no upstream.
func (r *Repo) PushTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := r.tagRef(ctx, name)
	if err != nil {
		return err
	}
	remote, err := r.pushRemote(ctx)
	if err != nil {
		return err
	}
	return r.run(ctx, "push", remote, ref)
}

// DeleteTag removes a local tag. The commit it pointed at stays.
func (r *Repo) DeleteTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := r.tagRef(ctx, name)
	if err != nil {
		return err
	}
	return r.run(ctx, "tag", "-d", "--", strings.TrimPrefix(ref, "refs/tags/"))
}

// DeleteRemoteTag removes a tag from the upstream remote, or from origin when HEAD has no upstream.
// The local tag stays.
func (r *Repo) DeleteRemoteTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := r.tagRef(ctx, name)
	if err != nil {
		return err
	}
	remote, err := r.pushRemote(ctx)
	if err != nil {
		return err
	}
	return r.run(ctx, "push", remote, ":"+ref)
}

func (r *Repo) tagRef(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("invalid tag name")
	}
	ref := "refs/tags/" + name
	if err := r.run(ctx, "check-ref-format", ref); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("invalid tag name")
	}
	return ref, nil
}

func (r *Repo) pushRemote(ctx context.Context) (string, error) {
	remote, _, ok, err := r.upstream(ctx)
	if err != nil {
		return "", err
	}
	if !ok || remote == "" {
		return "origin", nil
	}
	return remote, nil
}

// ErrDeleteCurrent is returned when deleting the branch that HEAD points at.
var ErrDeleteCurrent = errors.New("cannot delete the checked-out branch")

// DeleteBranch removes a local branch or a remote-tracking branch.
// The checked-out branch is left in place.
func (r *Repo) DeleteBranch(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ref.Name == "" {
		return fmt.Errorf("no branch")
	}
	if ref.Remote == "" {
		_, branch, detached, ok, err := r.head(ctx)
		if err != nil {
			return err
		}
		if ref.Current || (ok && !detached && branch == ref.Name) {
			return ErrDeleteCurrent
		}
		return r.run(ctx, "branch", "-D", "--", ref.Name)
	}
	local := ref.LocalName()
	if local == "" || local == "HEAD" {
		return fmt.Errorf("no branch")
	}
	return r.run(ctx, "branch", "-D", "-r", "--", ref.Name)
}

// CreateBranch adds a local branch at hash. HEAD stays where it is.
func (r *Repo) CreateBranch(ctx context.Context, name, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("invalid branch name")
	}
	if err := r.run(ctx, "check-ref-format", "--branch", name); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("invalid branch name")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" || hash == Uncommitted {
		return fmt.Errorf("unknown revision")
	}
	resolved, err := r.output(ctx, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("unknown revision %s", hash)
	}
	return r.run(ctx, "branch", "--", name, strings.TrimSpace(resolved))
}

// RenameBranch renames a local branch. The checked-out branch can be renamed,
// and HEAD follows it. Remote-tracking branches are left unchanged.
func (r *Repo) RenameBranch(ctx context.Context, ref Ref, newName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ref.Remote != "" {
		return fmt.Errorf("cannot rename a remote-tracking branch")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || newName == ref.Name || ref.Name == "" {
		return fmt.Errorf("invalid branch name")
	}
	if err := r.run(ctx, "check-ref-format", "--branch", newName); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("invalid branch name")
	}
	return r.run(ctx, "branch", "-m", "--", ref.Name, newName)
}

// ErrLocalChanges is returned when checkout is refused because the worktree
// is dirty, including untracked files. HEAD is left unchanged.
var ErrLocalChanges = errors.New("local changes would be overwritten by checkout")

// Checkout switches to a local branch, or creates a local branch from a remote.
func (r *Repo) Checkout(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := r.status(ctx)
	if err != nil {
		return err
	}
	if st.Dirty() {
		return ErrLocalChanges
	}
	if ref.IsTag() {
		name := ref.Name
		if name == "" {
			name = ref.Hash
		}
		if name == "" {
			return fmt.Errorf("no tag")
		}
		return r.run(ctx, "checkout", "--detach", name)
	}
	if ref.Remote == "" {
		if ref.Name == "" {
			return fmt.Errorf("no branch")
		}
		return r.run(ctx, "switch", "--", ref.Name)
	}
	local := ref.LocalName()
	if local == "" || local == "HEAD" {
		return fmt.Errorf("no branch")
	}
	exists, err := r.hasRef(ctx, "refs/heads/"+local)
	if err != nil {
		return err
	}
	if exists {
		return r.run(ctx, "switch", "--", local)
	}
	track := ref.Name
	if !strings.Contains(track, "/") {
		track = ref.Remote + "/" + local
	}
	return r.run(ctx, "switch", "-c", local, "--track", track)
}

func (r *Repo) hasRef(ctx context.Context, ref string) (bool, error) {
	_, err := r.output(ctx, "show-ref", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// head reports the current commit. ok is false when the branch has no commits yet.
func (r *Repo) head(ctx context.Context) (hash, branch string, detached, ok bool, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", false, false, err
	}
	sym, symErr := r.output(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if symErr == nil {
		branch = strings.TrimSpace(sym)
		h, hashErr := r.output(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
		if hashErr != nil {
			if err := ctx.Err(); err != nil {
				return "", "", false, false, err
			}
			return "", branch, false, false, nil
		}
		return strings.TrimSpace(h), branch, false, true, nil
	}
	if err := ctx.Err(); err != nil {
		return "", "", false, false, err
	}
	h, hashErr := r.output(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
	if hashErr != nil {
		if err := ctx.Err(); err != nil {
			return "", "", false, false, err
		}
		return "", "", true, false, symErr
	}
	return strings.TrimSpace(h), "", true, true, nil
}

func (r *Repo) listRefs(ctx context.Context, branch string, detached bool, head string, remotes []Remote) (locals, remoteBranches, tags []Ref, decos map[string][]Decoration, err error) {
	decos = map[string][]Decoration{}
	known := map[string]bool{}
	for _, rm := range remotes {
		known[rm.Name] = true
	}
	out, err := r.output(ctx, "for-each-ref", "--format=%(refname)%00%(objecttype)%00%(objectname)%00%(*objecttype)%00%(*objectname)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 3 {
			continue
		}
		full := parts[0]
		hash, ok := peeledCommit(parts)
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(full, "refs/heads/"):
			short := strings.TrimPrefix(full, "refs/heads/")
			if short == "" || short == "HEAD" {
				continue
			}
			locals = append(locals, Ref{
				Name:    short,
				Full:    full,
				Hash:    hash,
				Current: !detached && short == branch,
			})
		case strings.HasPrefix(full, "refs/remotes/"):
			short := strings.TrimPrefix(full, "refs/remotes/")
			remote, branchName, ok := splitRemote(short, known)
			if !ok || branchName == "" || branchName == "HEAD" || strings.HasSuffix(short, "/HEAD") {
				continue
			}
			remoteBranches = append(remoteBranches, Ref{
				Name:   short,
				Full:   full,
				Hash:   hash,
				Remote: remote,
			})
		case strings.HasPrefix(full, "refs/tags/"):
			short := strings.TrimPrefix(full, "refs/tags/")
			if short == "" {
				continue
			}
			tags = append(tags, Ref{
				Name: short,
				Full: full,
				Hash: hash,
			})
		}
	}
	sort.Slice(locals, func(i, j int) bool { return locals[i].Name < locals[j].Name })
	sort.Slice(remoteBranches, func(i, j int) bool { return remoteBranches[i].Name < remoteBranches[j].Name })
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	for _, ref := range locals {
		decos[ref.Hash] = append(decos[ref.Hash], Decoration{Kind: "head", Name: ref.Name, HEAD: ref.Current})
	}
	for _, ref := range remoteBranches {
		decos[ref.Hash] = append(decos[ref.Hash], Decoration{Kind: "remote", Name: ref.Name})
	}
	for _, ref := range tags {
		decos[ref.Hash] = append(decos[ref.Hash], Decoration{Kind: "tag", Name: ref.Name})
	}
	if detached && head != "" {
		decos[head] = append(decos[head], Decoration{Kind: "other", Name: "HEAD", HEAD: true})
	}
	return locals, remoteBranches, tags, decos, nil
}

func peeledCommit(parts []string) (string, bool) {
	objectType := parts[1]
	objectName := parts[2]
	if objectType == "commit" && objectName != "" {
		return objectName, true
	}
	if len(parts) >= 5 && objectType == "tag" && parts[3] == "commit" && parts[4] != "" {
		return parts[4], true
	}
	return "", false
}

func (r *Repo) remoteConfigs(ctx context.Context) []Remote {
	out, err := r.output(ctx, "remote", "-v")
	if err != nil {
		return nil
	}
	rems := parseRemotes(out)
	sort.Slice(rems, func(i, j int) bool { return rems[i].Name < rems[j].Name })
	return rems
}

func (r *Repo) upstream(ctx context.Context) (remote, branch string, ok bool, err error) {
	_, _, detached, has, err := r.head(ctx)
	if err != nil {
		return "", "", false, err
	}
	if !has || detached {
		return "", "", false, nil
	}
	name, err := r.output(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		if err := ctx.Err(); err != nil {
			return "", "", false, err
		}
		return "", "", false, nil
	}
	name = strings.TrimSpace(name)
	known := map[string]bool{}
	for _, rm := range r.remoteConfigs(ctx) {
		known[rm.Name] = true
	}
	remote, branch, ok = splitRemote(name, known)
	if !ok || remote == "" || branch == "" {
		return "", "", false, nil
	}
	return remote, branch, true, nil
}

func (r *Repo) history(ctx context.Context, tips []string, limit int) ([]Commit, error) {
	seen := map[string]bool{}
	args := []string{
		"log", "-n", strconv.Itoa(maxWalk),
		"--no-notes", "--no-decorate",
		"--pretty=tformat:%H%x1f%P%x1f%an%x1f%ae%x1f%aI%x1f%cI%x1f%s",
	}
	for _, tip := range tips {
		tip = strings.TrimSpace(tip)
		if tip == "" || seen[tip] {
			continue
		}
		seen[tip] = true
		args = append(args, tip)
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out, err := r.output(ctx, args...)
	if err != nil {
		return nil, err
	}
	return orderNewest(parseHistory(out), limit), nil
}

type histCommit struct {
	Commit
	committer time.Time
}

func parseHistory(stdout string) map[string]*histCommit {
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	stdout = strings.TrimSuffix(stdout, "\n")
	if strings.TrimSpace(stdout) == "" {
		return nil
	}
	set := map[string]*histCommit{}
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue
		}
		c, ok := parseHistoryLine(line)
		if !ok {
			continue
		}
		if _, exists := set[c.Hash]; exists {
			continue
		}
		set[c.Hash] = c
	}
	return set
}

func parseHistoryLine(line string) (*histCommit, bool) {
	parts := strings.Split(line, fieldSep)
	if len(parts) < 7 || parts[0] == "" {
		return nil, false
	}
	var parents []string
	if p := strings.TrimSpace(parts[1]); p != "" {
		parents = strings.Fields(p)
	}
	authorWhen, _ := time.Parse(time.RFC3339, parts[4])
	committerWhen, _ := time.Parse(time.RFC3339, parts[5])
	return &histCommit{
		Commit: Commit{
			Hash:    parts[0],
			Parents: parents,
			Author:  parts[2],
			Email:   parts[3],
			When:    authorWhen,
			Subject: strings.Join(parts[6:], fieldSep),
		},
		committer: committerWhen,
	}, true
}

func orderNewest(set map[string]*histCommit, limit int) []Commit {
	if len(set) == 0 {
		return nil
	}
	kids := map[string]int{}
	for _, c := range set {
		for _, p := range c.Parents {
			if _, ok := set[p]; ok {
				kids[p]++
			}
		}
	}
	var ready []string
	for h := range set {
		if kids[h] == 0 {
			ready = append(ready, h)
		}
	}
	var out []Commit
	for len(ready) > 0 && (limit <= 0 || len(out) < limit) {
		sort.Slice(ready, func(i, j int) bool {
			return newer(set[ready[i]], set[ready[j]])
		})
		h := ready[0]
		ready = ready[1:]
		out = append(out, set[h].Commit)
		for _, p := range set[h].Parents {
			if _, ok := set[p]; !ok {
				continue
			}
			kids[p]--
			if kids[p] == 0 {
				ready = append(ready, p)
			}
		}
	}
	return out
}

func newer(a, b *histCommit) bool {
	if a.committer.Equal(b.committer) {
		return a.Hash > b.Hash
	}
	return a.committer.After(b.committer)
}

func splitRemote(short string, known map[string]bool) (remote, branch string, ok bool) {
	best := ""
	for name := range known {
		if strings.HasPrefix(short, name+"/") && len(name) > len(best) {
			best = name
		}
	}
	if best != "" {
		return best, strings.TrimPrefix(short, best+"/"), true
	}
	remote, branch, ok = strings.Cut(short, "/")
	return remote, branch, ok
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (r *Repo) output(ctx context.Context, args ...string) (string, error) {
	return gitOutput(ctx, r.loc(), args...)
}

func (r *Repo) exists(ctx context.Context, rel string) bool {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" {
		return false
	}
	if !r.IsRemote() {
		_, err := os.Stat(filepath.Join(r.Dir, filepath.FromSlash(rel)))
		return err == nil
	}
	abs := path.Join(r.Dir, rel)
	remote := "sh -c " + sshremote.Quote("test -e "+sshremote.Quote(abs))
	cmd := sshremote.Command(ctx, sshremote.Target{Dest: r.Dest, Port: r.Port}, remote)
	return cmd.Run() == nil
}

func (r *Repo) run(ctx context.Context, args ...string) error {
	_, err := r.output(ctx, args...)
	return err
}

func gitOutput(ctx context.Context, loc Loc, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cmd := gitCmd(ctx, loc, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return "", err
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func gitCmd(ctx context.Context, loc Loc, args ...string) *exec.Cmd {
	full := make([]string, 0, len(args)+6)
	full = append(full, "--no-pager", "-c", "core.quotePath=false", "-c", "log.showSignature=false")
	if loc.Dir != "" {
		full = append(full, "-C", loc.Dir)
	}
	full = append(full, args...)
	if loc.Dest == "" {
		cmd := exec.CommandContext(ctx, "git", full...)
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GIT_EDITOR=true",
			"GIT_SEQUENCE_EDITOR=true",
		)
		return cmd
	}
	// sh -c keeps the command POSIX when the remote login shell is not sh.
	var b strings.Builder
	b.WriteString("env GIT_TERMINAL_PROMPT=0 GIT_EDITOR=true GIT_SEQUENCE_EDITOR=true git")
	for _, a := range full {
		b.WriteByte(' ')
		b.WriteString(sshremote.Quote(a))
	}
	remote := "sh -c " + sshremote.Quote(b.String())
	return sshremote.Command(ctx, sshremote.Target{Dest: loc.Dest, Port: loc.Port}, remote)
}
