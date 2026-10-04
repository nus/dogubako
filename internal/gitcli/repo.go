package gitcli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/format/reflog"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/utils/merkletrie"
)

const defaultMaxLog = 500

// Repo is a git working tree opened with go-git.
type Repo struct {
	Dir string
	g   *gogit.Repository
}

// Open resolves path to a work tree root. Operations use go-git, not the git binary.
func Open(ctx context.Context, path string) (*Repo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	return &Repo{Dir: wt.Filesystem().Root(), g: repo}, nil
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
	hash, branch, detached, hasHead := r.headRef()
	s.Branch = branch
	s.Detached = detached || branch == ""
	if hasHead {
		s.HEAD = hash.String()
	}

	locals, remoteBranches, tags, decos, err := r.listRefs(branch, s.Detached)
	if err != nil {
		return s, err
	}
	s.Locals = locals
	s.RemoteBranches = remoteBranches
	s.Tags = tags
	s.Remotes = r.remoteConfigs()

	max := opts.MaxCommits
	if max <= 0 {
		max = defaultMaxLog
	}
	if hasHead || len(locals) > 0 || len(remoteBranches) > 0 || len(tags) > 0 {
		tips := make([]plumbing.Hash, 0, 1+len(locals)+len(remoteBranches)+len(tags))
		if hasHead {
			tips = append(tips, hash)
		}
		for _, ref := range locals {
			tips = append(tips, plumbing.NewHash(ref.Hash))
		}
		for _, ref := range tags {
			tips = append(tips, plumbing.NewHash(ref.Hash))
		}
		for _, ref := range remoteBranches {
			if opts.HiddenRemotes[ref.Remote] {
				continue
			}
			tips = append(tips, plumbing.NewHash(ref.Hash))
		}
		commits, err := r.topo(ctx, tips, nil, max)
		if err != nil {
			return s, err
		}
		s.Commits = make([]Commit, len(commits))
		for i, c := range commits {
			s.Commits[i] = toCommit(c, decos[c.Hash.String()])
			if len(opts.HiddenRemotes) > 0 {
				s.Commits[i].Decorations = filterDecorations(s.Commits[i].Decorations, opts.HiddenRemotes)
			}
		}
	}

	s.Status, err = r.status(ctx)
	if err != nil {
		return s, err
	}
	if s.Status.Branch != "" && !s.Status.Detached {
		s.Branch = s.Status.Branch
	}
	s.Detached = s.Status.Detached || s.Branch == ""
	s.RefDigest, err = r.refDigest()
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
	hash, _, _, hasHead := r.headRef()
	if hasHead {
		st.Head = hash.String()
	}
	var err error
	st.Refs, err = r.refDigest()
	if err != nil {
		return State{}, err
	}
	st.Status, err = r.status(ctx)
	return st, err
}

func (r *Repo) refDigest() (string, error) {
	iter, err := r.g.References()
	if err != nil {
		return "", err
	}
	var lines []string
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		target := ref.Hash().String()
		if ref.Type() == plumbing.SymbolicReference {
			target = ref.Target().String()
		}
		lines = append(lines, ref.Name().String()+" "+target)
		return nil
	})
	if err != nil && !isStop(err) {
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
	hash, branch, detached, hasHead := r.headRef()
	st := Status{Branch: branch, Detached: detached || branch == ""}
	wt, err := r.wt()
	if err != nil {
		return st, err
	}
	gs, err := wt.Status()
	if err != nil {
		return st, err
	}
	paths := make([]string, 0, len(gs))
	for p := range gs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fs := gs[p]
		if fs == nil {
			continue
		}
		if fs.Staging == gogit.Unmodified && fs.Worktree == gogit.Unmodified {
			continue
		}
		st.Entries = append(st.Entries, StatusEntry{
			Code: string([]byte{byte(fs.Staging), byte(fs.Worktree)}),
			Path: p,
		})
	}
	if hasHead && branch != "" && !detached {
		st.Ahead, st.Behind = r.aheadBehind(branch, hash)
	}
	return st, nil
}

// Detail loads the full message and name-status for hash.
func (r *Repo) Detail(ctx context.Context, hash string) (CommitDetail, error) {
	if err := ctx.Err(); err != nil {
		return CommitDetail{}, err
	}
	c, err := r.commitObj(hash)
	if err != nil {
		return CommitDetail{}, err
	}
	files, err := r.nameStatus(c)
	if err != nil {
		files = nil
	}
	diffText, err := r.commitDiff(ctx, c)
	if err != nil {
		diffText = ""
	}
	decos := r.decorationsFor(c.Hash)
	return CommitDetail{
		Commit: toCommit(c, decos),
		Body:   strings.TrimSpace(c.Message),
		Files:  files,
		Diff:   diffText,
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
	wt, err := r.wt()
	if err != nil {
		return err
	}
	if stageAll {
		if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
			return err
		}
	}
	msg := message
	opts := &gogit.CommitOptions{}
	if amend {
		opts.Amend = true
		if msg == "" {
			head, err := r.headCommit()
			if err != nil {
				return err
			}
			msg = strings.TrimSpace(head.Message)
		}
	}
	_, err = wt.Commit(ensureNL(msg), opts)
	return err
}

// Stage adds paths to the index. A missing worktree file is recorded as a deletion.
func (r *Repo) Stage(ctx context.Context, paths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths")
	}
	wt, err := r.wt()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := wt.Add(p); err != nil {
			return err
		}
	}
	return nil
}

// Unstage restores the index for paths from HEAD and leaves the worktree as it is.
func (r *Repo) Unstage(ctx context.Context, paths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths")
	}
	wt, err := r.wt()
	if err != nil {
		return err
	}
	return wt.Restore(&gogit.RestoreOptions{Staged: true, Files: paths})
}

// Fetch downloads objects from every remote.
func (r *Repo) Fetch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remotes, err := r.g.Remotes()
	if err != nil {
		return err
	}
	for _, remote := range remotes {
		err := remote.FetchContext(ctx, &gogit.FetchOptions{
			RemoteName: remote.Config().Name,
			Prune:      true,
			Tags:       plumbing.TagFollowing,
		})
		if err = ignoreUpToDate(err); err != nil {
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
	remote, merge, ok, err := r.upstream()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no upstream configured")
	}
	wt, err := r.wt()
	if err != nil {
		return err
	}
	err = wt.PullContext(ctx, &gogit.PullOptions{
		RemoteName:    remote,
		ReferenceName: merge,
	})
	return ignoreUpToDate(err)
}

// Push sends the current branch. Sets upstream to origin when missing.
func (r *Repo) Push(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, branch, detached, ok := r.headRef()
	if !ok {
		return fmt.Errorf("no commits")
	}
	if detached || branch == "" {
		return fmt.Errorf("detached HEAD")
	}
	remoteName, merge, hasUp, err := r.upstream()
	if err != nil {
		return err
	}
	if !hasUp {
		remoteName = gogit.DefaultRemoteName
		merge = plumbing.NewBranchReferenceName(branch)
	}
	remote, err := r.g.Remote(remoteName)
	if err != nil {
		return err
	}
	spec := config.RefSpec(fmt.Sprintf("%s:%s", plumbing.NewBranchReferenceName(branch), merge))
	err = remote.PushContext(ctx, &gogit.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{spec},
	})
	if err = ignoreUpToDate(err); err != nil {
		return err
	}
	if !hasUp {
		return r.setUpstream(branch, remoteName, merge)
	}
	return nil
}

// CreateTag adds a lightweight tag at hash.
func (r *Repo) CreateTag(ctx context.Context, name, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("invalid tag name")
	}
	h, err := r.resolve(hash)
	if err != nil {
		return err
	}
	_, err = r.g.CreateTag(name, h, nil)
	if errors.Is(err, gogit.ErrTagExists) {
		return fmt.Errorf("tag %s already exists", name)
	}
	return err
}

// PushTag sends one local tag to the upstream remote, or to origin when HEAD has no upstream.
func (r *Repo) PushTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	refName, err := tagRefName(name)
	if err != nil {
		return err
	}
	spec := config.RefSpec(refName.String() + ":" + refName.String())
	return r.pushRefSpec(ctx, spec)
}

// DeleteTag removes a local tag. The commit it pointed at stays.
func (r *Repo) DeleteTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	refName, err := tagRefName(name)
	if err != nil {
		return err
	}
	return r.g.DeleteTag(refName.Short())
}

// DeleteRemoteTag removes a tag from the upstream remote, or from origin when HEAD has no upstream.
// The local tag stays.
func (r *Repo) DeleteRemoteTag(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	refName, err := tagRefName(name)
	if err != nil {
		return err
	}
	return r.pushRefSpec(ctx, config.RefSpec(":"+refName.String()))
}

func tagRefName(name string) (plumbing.ReferenceName, error) {
	refName := plumbing.NewTagReferenceName(strings.TrimSpace(name))
	if err := refName.Validate(); err != nil {
		return "", err
	}
	return refName, nil
}

func (r *Repo) pushRemoteName() (string, error) {
	remoteName, _, hasUp, err := r.upstream()
	if err != nil {
		return "", err
	}
	if !hasUp {
		remoteName = gogit.DefaultRemoteName
	}
	return remoteName, nil
}

func (r *Repo) pushRefSpec(ctx context.Context, spec config.RefSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	remoteName, err := r.pushRemoteName()
	if err != nil {
		return err
	}
	remote, err := r.g.Remote(remoteName)
	if err != nil {
		return err
	}
	err = remote.PushContext(ctx, &gogit.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{spec},
	})
	return ignoreUpToDate(err)
}

// ErrDeleteCurrent is returned when deleting the branch that HEAD points at.
var ErrDeleteCurrent = errors.New("cannot delete the checked-out branch")

// DeleteBranch removes a local branch or a remote-tracking branch.
// The checked-out branch is left in place.
func (r *Repo) DeleteBranch(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, local, err := deleteRefName(ref)
	if err != nil {
		return err
	}
	_, branch, detached, ok := r.headRef()
	if local && ((ref.Current) || (ok && !detached && branch == ref.Name)) {
		return ErrDeleteCurrent
	}
	if err := r.g.Storer.RemoveReference(name); err != nil {
		return err
	}
	if local {
		_ = r.clearBranchConfig(ref.Name)
	}
	return nil
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
	if newName == "" || newName == ref.Name {
		return fmt.Errorf("invalid branch name")
	}
	oldName := plumbing.NewBranchReferenceName(ref.Name)
	if err := oldName.Validate(); err != nil {
		return err
	}
	nextName := plumbing.NewBranchReferenceName(newName)
	if err := nextName.Validate(); err != nil {
		return err
	}
	cur, err := r.g.Reference(oldName, true)
	if err != nil {
		return err
	}
	if _, err := r.g.Reference(nextName, false); err == nil {
		return fmt.Errorf("branch %s already exists", newName)
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}
	if err := r.g.Storer.SetReference(plumbing.NewHashReference(nextName, cur.Hash())); err != nil {
		return err
	}
	head, err := r.g.Reference(plumbing.HEAD, false)
	if err != nil {
		return err
	}
	if head.Type() == plumbing.SymbolicReference && head.Target() == oldName {
		if err := r.g.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, nextName)); err != nil {
			return err
		}
	}
	if err := r.g.Storer.RemoveReference(oldName); err != nil {
		return err
	}
	return r.renameBranchConfig(ref.Name, newName)
}

func deleteRefName(ref Ref) (plumbing.ReferenceName, bool, error) {
	if ref.Name == "" {
		return "", false, fmt.Errorf("no branch")
	}
	if ref.Remote == "" {
		name := plumbing.NewBranchReferenceName(ref.Name)
		if err := name.Validate(); err != nil {
			return "", false, err
		}
		return name, true, nil
	}
	local := ref.LocalName()
	if local == "" || local == "HEAD" {
		return "", false, fmt.Errorf("no branch")
	}
	name := plumbing.NewRemoteReferenceName(ref.Remote, local)
	if err := name.Validate(); err != nil {
		return "", false, err
	}
	return name, false, nil
}

// ErrLocalChanges is returned when checkout would overwrite a dirty worktree.
// go-git moves HEAD before it notices the dirty tree, so checkout must not
// start in that state.
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
	head, err := r.g.Reference(plumbing.HEAD, false)
	if err != nil {
		return err
	}
	saved := cloneHead(head)
	if err := r.checkout(ctx, ref); err != nil {
		var keep keepHeadError
		if !errors.As(err, &keep) {
			_ = r.g.Storer.SetReference(saved)
		}
		return err
	}
	return nil
}

// keepHeadError means HEAD already points at the checked-out branch.
type keepHeadError struct{ err error }

func (e keepHeadError) Error() string { return e.err.Error() }
func (e keepHeadError) Unwrap() error { return e.err }

func cloneHead(ref *plumbing.Reference) *plumbing.Reference {
	if ref.Type() == plumbing.SymbolicReference {
		return plumbing.NewSymbolicReference(ref.Name(), ref.Target())
	}
	return plumbing.NewHashReference(ref.Name(), ref.Hash())
}

func (r *Repo) checkout(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wt, err := r.wt()
	if err != nil {
		return err
	}
	if ref.IsTag() {
		// git switch --detach <tag>: HEAD becomes the peeled commit, and the
		// reflog names the tag so status says "detached at <tag>".
		from := r.detachFromLabel()
		old, _, _, _ := r.headRef()
		target := ref.Hash
		if target == "" {
			target = ref.Name
		}
		hash, err := r.resolve(target)
		if err != nil {
			return err
		}
		if err := wt.Checkout(&gogit.CheckoutOptions{Hash: hash}); err != nil {
			return err
		}
		if err := r.noteDetach(old, hash, from, ref.Name); err != nil {
			return keepHeadError{err: err}
		}
		return nil
	}
	if ref.Remote == "" {
		return wt.Checkout(&gogit.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(ref.Name),
		})
	}
	local := ref.LocalName()
	if _, err := r.g.Reference(plumbing.NewBranchReferenceName(local), true); err == nil {
		return wt.Checkout(&gogit.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(local),
		})
	}
	hash, err := r.resolve(ref.Hash)
	if err != nil {
		return err
	}
	if err := wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(local),
		Hash:   hash,
		Create: true,
	}); err != nil {
		return err
	}
	if err := r.setUpstream(local, ref.Remote, plumbing.NewBranchReferenceName(local)); err != nil {
		return keepHeadError{err: err}
	}
	return nil
}

func (r *Repo) wt() (*gogit.Worktree, error) {
	return r.g.Worktree()
}

// detachFromLabel is the name git records as the previous position when detaching.
// A branch is its short name. An already detached HEAD is the full hash.
func (r *Repo) detachFromLabel() string {
	ref, err := r.g.Reference(plumbing.HEAD, false)
	if err != nil || ref == nil {
		return "HEAD"
	}
	if ref.Type() == plumbing.SymbolicReference {
		if short := ref.Target().Short(); short != "" {
			return short
		}
	}
	if !ref.Hash().IsZero() {
		return ref.Hash().String()
	}
	return "HEAD"
}

func (r *Repo) noteDetach(old, new plumbing.Hash, from, to string) error {
	logs, ok := r.g.Storer.(storer.ReflogStorer)
	if !ok {
		return fmt.Errorf("repository has no reflog")
	}
	name, email := r.reflogIdentity()
	return logs.AppendReflog(plumbing.HEAD, &reflog.Entry{
		OldHash: old,
		NewHash: new,
		Committer: reflog.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
		Message: fmt.Sprintf("checkout: moving from %s to %s", from, to),
	})
}

func (r *Repo) reflogIdentity() (string, string) {
	cfg, err := r.g.Config()
	if err != nil {
		return "unknown", "unknown"
	}
	name, email := cfg.Committer.Name, cfg.Committer.Email
	if name == "" {
		name = cfg.User.Name
	}
	if email == "" {
		email = cfg.User.Email
	}
	if name == "" {
		name = "unknown"
	}
	if email == "" {
		email = "unknown"
	}
	return name, email
}

func (r *Repo) headRef() (hash plumbing.Hash, branch string, detached, ok bool) {
	ref, err := r.g.Reference(plumbing.HEAD, false)
	if err != nil {
		return plumbing.ZeroHash, "", true, false
	}
	if ref.Type() == plumbing.SymbolicReference {
		branch = ref.Target().Short()
		resolved, err := r.g.Reference(ref.Target(), true)
		if err != nil {
			return plumbing.ZeroHash, branch, false, false
		}
		return resolved.Hash(), branch, false, true
	}
	h := ref.Hash()
	return h, "", true, !h.IsZero()
}

func (r *Repo) headCommit() (*object.Commit, error) {
	ref, err := r.g.Head()
	if err != nil {
		return nil, err
	}
	return r.g.CommitObject(ref.Hash())
}

func (r *Repo) commitObj(hash string) (*object.Commit, error) {
	h, err := r.resolve(hash)
	if err != nil {
		return nil, err
	}
	return r.g.CommitObject(h)
}

func (r *Repo) resolve(name string) (plumbing.Hash, error) {
	if name == "HEAD" {
		h, _, _, ok := r.headRef()
		if !ok {
			return plumbing.ZeroHash, fmt.Errorf("no commits")
		}
		return h, nil
	}
	if h := plumbing.NewHash(name); !h.IsZero() {
		if _, err := r.g.CommitObject(h); err == nil {
			return h, nil
		}
	}
	candidates := []plumbing.ReferenceName{
		plumbing.ReferenceName(name),
		plumbing.NewBranchReferenceName(name),
		plumbing.ReferenceName("refs/remotes/" + name),
		plumbing.NewTagReferenceName(name),
	}
	for _, cand := range candidates {
		ref, err := r.g.Reference(cand, true)
		if err != nil {
			continue
		}
		h, ok := r.peel(ref)
		if ok {
			return h, nil
		}
	}
	return plumbing.ZeroHash, fmt.Errorf("unknown revision %s", name)
}

func (r *Repo) peel(ref *plumbing.Reference) (plumbing.Hash, bool) {
	h := ref.Hash()
	if ref.Type() == plumbing.SymbolicReference {
		resolved, err := r.g.Reference(ref.Name(), true)
		if err != nil {
			return plumbing.ZeroHash, false
		}
		h = resolved.Hash()
	}
	for range 5 {
		obj, err := r.g.Object(plumbing.AnyObject, h)
		if err != nil {
			return plumbing.ZeroHash, false
		}
		switch o := obj.(type) {
		case *object.Commit:
			return o.Hash, true
		case *object.Tag:
			h = o.Target
		default:
			return plumbing.ZeroHash, false
		}
	}
	return plumbing.ZeroHash, false
}

func (r *Repo) listRefs(branch string, detached bool) (locals, remoteBranches, tags []Ref, decos map[string][]Decoration, err error) {
	decos = map[string][]Decoration{}
	known := map[string]bool{}
	for _, rm := range r.remoteConfigs() {
		known[rm.Name] = true
	}
	iter, err := r.g.References()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()
		switch {
		case name.IsBranch():
			h, ok := r.peel(ref)
			if !ok {
				return nil
			}
			short := name.Short()
			locals = append(locals, Ref{
				Name:    short,
				Full:    name.String(),
				Hash:    h.String(),
				Current: !detached && short == branch,
			})
			d := Decoration{Kind: "head", Name: short, HEAD: !detached && short == branch}
			decos[h.String()] = append(decos[h.String()], d)
		case name.IsRemote():
			short := name.Short()
			remote, branchName, ok := splitRemote(short, known)
			if !ok || branchName == "HEAD" || strings.HasSuffix(short, "/HEAD") {
				return nil
			}
			h, peeled := r.peel(ref)
			if !peeled {
				return nil
			}
			remoteBranches = append(remoteBranches, Ref{
				Name:   short,
				Full:   name.String(),
				Hash:   h.String(),
				Remote: remote,
			})
			decos[h.String()] = append(decos[h.String()], Decoration{Kind: "remote", Name: short})
		case name.IsTag():
			h, ok := r.peel(ref)
			if !ok {
				return nil
			}
			short := name.Short()
			tags = append(tags, Ref{
				Name: short,
				Full: name.String(),
				Hash: h.String(),
			})
			decos[h.String()] = append(decos[h.String()], Decoration{Kind: "tag", Name: short})
		}
		return nil
	})
	if err != nil && !isStop(err) {
		return nil, nil, nil, nil, err
	}
	if detached {
		if h, _, _, ok := r.headRef(); ok {
			decos[h.String()] = append(decos[h.String()], Decoration{Kind: "other", Name: "HEAD", HEAD: true})
		}
	}
	sort.Slice(locals, func(i, j int) bool { return locals[i].Name < locals[j].Name })
	sort.Slice(remoteBranches, func(i, j int) bool { return remoteBranches[i].Name < remoteBranches[j].Name })
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return locals, remoteBranches, tags, decos, nil
}

func (r *Repo) decorationsFor(h plumbing.Hash) []Decoration {
	_, branch, detached, _ := r.headRef()
	_, _, _, decos, err := r.listRefs(branch, detached)
	if err != nil {
		return nil
	}
	return decos[h.String()]
}

func (r *Repo) remoteConfigs() []Remote {
	remotes, err := r.g.Remotes()
	if err != nil {
		return nil
	}
	out := make([]Remote, 0, len(remotes))
	for _, remote := range remotes {
		cfg := remote.Config()
		url := ""
		if len(cfg.URLs) > 0 {
			url = cfg.URLs[0]
		}
		out = append(out, Remote{Name: cfg.Name, URL: url})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Repo) upstream() (remote string, merge plumbing.ReferenceName, ok bool, err error) {
	_, branch, detached, has := r.headRef()
	if !has || detached || branch == "" {
		return "", "", false, nil
	}
	cfg, err := r.g.Config()
	if err != nil {
		return "", "", false, err
	}
	b := cfg.Branches[branch]
	if b == nil || b.Remote == "" || b.Merge == "" {
		return "", "", false, nil
	}
	return b.Remote, b.Merge, true, nil
}

func (r *Repo) setUpstream(branch, remote string, merge plumbing.ReferenceName) error {
	cfg, err := r.g.Config()
	if err != nil {
		return err
	}
	if cfg.Branches == nil {
		cfg.Branches = map[string]*config.Branch{}
	}
	b := cfg.Branches[branch]
	if b == nil {
		b = &config.Branch{Name: branch}
		cfg.Branches[branch] = b
	}
	b.Name = branch
	b.Remote = remote
	b.Merge = merge
	return r.g.SetConfig(cfg)
}

func (r *Repo) clearBranchConfig(branch string) error {
	cfg, err := r.g.Config()
	if err != nil {
		return err
	}
	if cfg.Branches == nil {
		return nil
	}
	if _, ok := cfg.Branches[branch]; !ok {
		return nil
	}
	delete(cfg.Branches, branch)
	return r.g.SetConfig(cfg)
}

func (r *Repo) renameBranchConfig(oldName, newName string) error {
	cfg, err := r.g.Config()
	if err != nil {
		return err
	}
	if cfg.Branches == nil {
		return nil
	}
	b := cfg.Branches[oldName]
	if b == nil {
		return nil
	}
	delete(cfg.Branches, oldName)
	b.Name = newName
	cfg.Branches[newName] = b
	return r.g.SetConfig(cfg)
}

func (r *Repo) aheadBehind(branch string, head plumbing.Hash) (ahead, behind int) {
	cfg, err := r.g.Config()
	if err != nil {
		return 0, 0
	}
	b := cfg.Branches[branch]
	if b == nil || b.Remote == "" || b.Merge == "" {
		return 0, 0
	}
	ref, err := r.g.Reference(plumbing.NewRemoteReferenceName(b.Remote, b.Merge.Short()), true)
	if err != nil {
		return 0, 0
	}
	local, err := r.g.CommitObject(head)
	if err != nil {
		return 0, 0
	}
	up, err := r.g.CommitObject(ref.Hash())
	if err != nil {
		return 0, 0
	}
	bases, err := local.MergeBase(up)
	if err != nil || len(bases) == 0 {
		return 0, 0
	}
	base := bases[0].Hash
	return countFirstParent(local, base), countFirstParent(up, base)
}

func (r *Repo) commitDiff(ctx context.Context, c *object.Commit) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var from *object.Tree
	if c.NumParents() == 0 {
		t, err := r.emptyTree()
		if err != nil {
			return "", err
		}
		from = t
	} else {
		p, err := c.Parent(0)
		if err != nil {
			return "", err
		}
		from, err = p.Tree()
		if err != nil {
			return "", err
		}
	}
	to, err := c.Tree()
	if err != nil {
		return "", err
	}
	patch, err := from.PatchContext(ctx, to)
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

func (r *Repo) nameStatus(c *object.Commit) ([]string, error) {
	var from *object.Tree
	if c.NumParents() == 0 {
		t, err := r.emptyTree()
		if err != nil {
			return nil, err
		}
		from = t
	} else {
		p, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		from, err = p.Tree()
		if err != nil {
			return nil, err
		}
	}
	to, err := c.Tree()
	if err != nil {
		return nil, err
	}
	changes, err := from.Diff(to)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, ch := range changes {
		action, err := ch.Action()
		if err != nil {
			return nil, err
		}
		letter := "M"
		name := ch.To.Name
		switch action {
		case merkletrie.Insert:
			letter = "A"
		case merkletrie.Delete:
			letter = "D"
			name = ch.From.Name
		}
		if name == "" {
			continue
		}
		files = append(files, letter+"\t"+name)
	}
	return files, nil
}

func (r *Repo) topo(ctx context.Context, tips []plumbing.Hash, skip map[plumbing.Hash]bool, limit int) ([]*object.Commit, error) {
	set, err := r.collect(ctx, tips, skip)
	if err != nil {
		return nil, err
	}
	return orderNewest(set, limit), nil
}

func (r *Repo) collect(ctx context.Context, tips []plumbing.Hash, skip map[plumbing.Hash]bool) (map[plumbing.Hash]*object.Commit, error) {
	out := map[plumbing.Hash]*object.Commit{}
	stack := append([]plumbing.Hash(nil), tips...)
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if h.IsZero() || skip[h] {
			continue
		}
		if _, ok := out[h]; ok {
			continue
		}
		c, err := r.g.CommitObject(h)
		if err != nil {
			continue
		}
		out[h] = c
		if len(out) >= 20000 {
			break
		}
		for _, p := range c.ParentHashes {
			if !skip[p] {
				stack = append(stack, p)
			}
		}
	}
	return out, nil
}

func (r *Repo) emptyTree() (*object.Tree, error) {
	obj := &plumbing.MemoryObject{}
	t := &object.Tree{}
	if err := t.Encode(obj); err != nil {
		return nil, err
	}
	return &object.Tree{Hash: obj.Hash()}, nil
}

func orderNewest(set map[plumbing.Hash]*object.Commit, limit int) []*object.Commit {
	kids := map[plumbing.Hash]int{}
	for _, c := range set {
		for _, p := range c.ParentHashes {
			if _, ok := set[p]; ok {
				kids[p]++
			}
		}
	}
	var ready []plumbing.Hash
	for h := range set {
		if kids[h] == 0 {
			ready = append(ready, h)
		}
	}
	var out []*object.Commit
	for len(ready) > 0 && (limit <= 0 || len(out) < limit) {
		sort.Slice(ready, func(i, j int) bool {
			return newer(set[ready[i]], set[ready[j]])
		})
		h := ready[0]
		ready = ready[1:]
		out = append(out, set[h])
		for _, p := range set[h].ParentHashes {
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

func newer(a, b *object.Commit) bool {
	if a.Committer.When.Equal(b.Committer.When) {
		return a.Hash.String() > b.Hash.String()
	}
	return a.Committer.When.After(b.Committer.When)
}

func toCommit(c *object.Commit, decos []Decoration) Commit {
	parents := make([]string, len(c.ParentHashes))
	for i, p := range c.ParentHashes {
		parents[i] = p.String()
	}
	return Commit{
		Hash:        c.Hash.String(),
		Parents:     parents,
		Author:      c.Author.Name,
		Email:       c.Author.Email,
		When:        c.Author.When,
		Subject:     subjectOf(c.Message),
		Decorations: decos,
	}
}

func subjectOf(msg string) string {
	msg = strings.TrimRight(msg, "\n")
	line, _, _ := strings.Cut(msg, "\n")
	return strings.TrimSpace(line)
}

func ensureNL(s string) string {
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return "\n"
	}
	return s + "\n"
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

func countFirstParent(c *object.Commit, target plumbing.Hash) int {
	n := 0
	for c != nil && c.Hash != target && n < 100000 {
		n++
		if c.NumParents() == 0 {
			break
		}
		p, err := c.Parent(0)
		if err != nil {
			break
		}
		c = p
	}
	return n
}

func ignoreUpToDate(err error) error {
	if err == nil || errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func isStop(err error) bool {
	return errors.Is(err, storer.ErrStop)
}
