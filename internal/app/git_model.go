package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

const (
	gitLoadTimeout = 30 * time.Second
	gitOpTimeout   = 3 * time.Minute
)

type gitLoadResult struct {
	repo  *gitcli.Repo
	snap  gitcli.Snapshot
	quiet bool
	err   error
}

type gitOpResult struct {
	okKey  i18n.Key
	errKey i18n.Key
	err    error
}

type gitDetailResult struct {
	hash   string
	detail gitcli.CommitDetail
	err    error
}

type gitWorkResult struct {
	key  string
	diff string
	err  error
}

type gitWorkSlot struct {
	key     string
	diff    string
	err     error
	pending <-chan gitWorkResult
}

// GitModel holds one repository per tab. The embedded session is the active tab.
type GitModel struct {
	*gitSession
	tabs     []*gitSession
	active   int
	loaded   bool
	extraGen uint64
	recent   []string
}

// gitSession is the state of a single repository tab.
type gitSession struct {
	owner      *GitModel
	generation uint64

	path     string
	repo     *gitcli.Repo
	snap     gitcli.Snapshot
	hidden   map[string]bool
	sel      string
	detail   gitcli.CommitDetail
	draft    string
	stageAll bool

	status statusMsg

	pendingLoad   <-chan gitLoadResult
	pendingOp     <-chan gitOpResult
	pendingDetail <-chan gitDetailResult
	pendingStatus <-chan gitStatusResult
	lastPoll      time.Time

	workSlots [2]gitWorkSlot

	// reveal is a commit hash the graph should scroll into view. Empty means none.
	reveal string
}

type gitStatusResult struct {
	state gitcli.State
	err   error
}

func (m *gitSession) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *gitSession) SetStatus(key i18n.Key, args ...any) {
	if m.status.key == key && fmt.Sprint(m.status.args...) == fmt.Sprint(args...) {
		return
	}
	m.status.key = key
	if len(args) == 0 {
		m.status.args = nil
	} else {
		m.status.args = append([]any(nil), args...)
	}
	m.generation++
}

func (m *gitSession) Path() string { return m.path }
func (m *gitSession) HasRepo() bool {
	return m.repo != nil && m.path != ""
}
func (m *gitSession) Snapshot() gitcli.Snapshot { return m.snap }
func (m *gitSession) Selected() string          { return m.sel }
func (m *gitSession) Detail() gitcli.CommitDetail {
	return m.detail
}
func (m *gitSession) Draft() string { return m.draft }
func (m *gitSession) SetDraft(s string) {
	if m.draft == s {
		return
	}
	m.draft = s
	m.generation++
}
func (m *gitSession) StageAll() bool { return m.stageAll }
func (m *gitSession) SetStageAll(v bool) {
	if m.stageAll == v {
		return
	}
	m.stageAll = v
	m.generation++
}

func (m *gitSession) RemoteVisible(name string) bool {
	return !m.hidden[name]
}

func (m *gitSession) SetRemoteVisible(name string, visible bool) {
	if m.hidden == nil {
		m.hidden = map[string]bool{}
	}
	hide := !visible
	if m.hidden[name] == hide {
		return
	}
	if hide {
		m.hidden[name] = true
	} else {
		delete(m.hidden, name)
	}
	m.generation++
	m.reload()
}

func (m *gitSession) Busy() bool {
	return m.pendingLoad != nil || m.pendingOp != nil
}

func (m *gitSession) open(path string) {
	if m.Busy() || strings.TrimSpace(path) == "" {
		return
	}
	m.path = canonicalPath(path)
	m.SetStatus(i18n.StatusGitLoading)
	ch := make(chan gitLoadResult, 1)
	m.pendingLoad = ch
	m.generation++
	if m.owner != nil {
		m.owner.saveTabs()
	}
	hidden := cloneHidden(m.hidden)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitLoadTimeout)
		defer cancel()
		repo, err := gitcli.Open(ctx, path)
		if err != nil {
			ch <- gitLoadResult{err: err}
			return
		}
		snap, err := repo.Snapshot(ctx, gitcli.SnapshotOpts{HiddenRemotes: hidden})
		ch <- gitLoadResult{repo: repo, snap: snap, err: err}
	}()
}

func (m *gitSession) Reload() {
	if !m.HasRepo() {
		if m.path != "" {
			m.open(m.path)
		}
		return
	}
	m.reload()
}

func (m *gitSession) reload() {
	m.loadSnap(false)
}

// refresh reloads the graph without replacing the status line.
// Polling uses it when another Git operation changed the repository.
func (m *gitSession) refresh() {
	m.loadSnap(true)
}

func (m *gitSession) loadSnap(quiet bool) {
	if m.Busy() || m.repo == nil {
		return
	}
	if !quiet {
		m.SetStatus(i18n.StatusGitLoading)
	}
	ch := make(chan gitLoadResult, 1)
	m.pendingLoad = ch
	m.generation++
	repo := m.repo
	hidden := cloneHidden(m.hidden)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitLoadTimeout)
		defer cancel()
		snap, err := repo.Snapshot(ctx, gitcli.SnapshotOpts{HiddenRemotes: hidden})
		ch <- gitLoadResult{repo: repo, snap: snap, err: err, quiet: quiet}
	}()
}

func (m *gitSession) Drain() {
	m.drainLoad()
	m.drainOp()
	m.drainDetail()
	m.drainWork()
	m.drainStatus()
}

func (m *gitSession) PollStatus() {
	if m.pendingStatus != nil || m.Busy() || m.repo == nil {
		return
	}
	now := time.Now()
	if now.Sub(m.lastPoll) < time.Second {
		return
	}
	m.lastPoll = now
	ch := make(chan gitStatusResult, 1)
	m.pendingStatus = ch
	repo := m.repo
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		st, err := repo.State(ctx)
		ch <- gitStatusResult{state: st, err: err}
	}()
}

func (m *gitSession) drainStatus() {
	if m.pendingStatus == nil {
		return
	}
	select {
	case res := <-m.pendingStatus:
		m.pendingStatus = nil
		if res.err != nil {
			return
		}
		m.applyWatched(res.state)
	default:
	}
}

// applyWatched copies a polled state into the view.
// A new commit, checkout, or branch edit reloads the graph.
// A worktree edit updates the file list and the uncommitted node.
func (m *gitSession) applyWatched(st gitcli.State) {
	if st.Head != m.snap.HEAD || st.Refs != m.snap.RefDigest {
		m.refresh()
		return
	}
	if statusEqual(m.snap.Status, st.Status) {
		return
	}
	wasDirty := m.snap.Status.Dirty()
	m.snap.Status = st.Status
	if st.Status.Branch != "" && !st.Status.Detached {
		m.snap.Branch = st.Status.Branch
	}
	m.snap.Detached = st.Status.Detached || m.snap.Branch == ""
	if wasDirty != st.Status.Dirty() {
		gitcli.ApplyWorkTree(&m.snap)
	}
	m.generation++
	guigui.RequestRebuild()
}

func decorationsFor(snap gitcli.Snapshot, hash string) []gitcli.Decoration {
	for _, c := range snap.Commits {
		if c.Hash == hash {
			return c.Decorations
		}
	}
	return nil
}

func decoKey(ds []gitcli.Decoration) string {
	if len(ds) == 0 {
		return ""
	}
	parts := make([]string, len(ds))
	for i, d := range ds {
		head := "0"
		if d.HEAD {
			head = "1"
		}
		parts[i] = d.Kind + "\x00" + d.Name + "\x00" + head
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func statusEqual(a, b gitcli.Status) bool {
	if a.Branch != b.Branch || a.Detached != b.Detached || a.Ahead != b.Ahead || a.Behind != b.Behind {
		return false
	}
	if len(a.Entries) != len(b.Entries) {
		return false
	}
	for i := range a.Entries {
		if a.Entries[i] != b.Entries[i] {
			return false
		}
	}
	return true
}

func (m *gitSession) drainLoad() {
	if m.pendingLoad == nil {
		return
	}
	select {
	case res := <-m.pendingLoad:
		m.pendingLoad = nil
		if res.err != nil {
			if m.HasRepo() {
				m.SetStatus(i18n.StatusGitOpFailed, res.err)
			} else {
				m.SetStatus(i18n.StatusGitOpenFailed, res.err)
			}
			guigui.RequestRebuild()
			return
		}
		m.repo = res.repo
		m.path = canonicalPath(res.repo.Dir)
		if m.owner != nil {
			m.owner.saveTabs()
		}
		m.applySnap(res.snap)
		if !res.quiet {
			m.SetStatus(i18n.StatusGitOpened, m.path, len(m.snap.Commits))
		}
		guigui.RequestRebuild()
	default:
	}
}

func (m *gitSession) applySnap(snap gitcli.Snapshot) {
	m.snap = snap
	if m.sel == "" && snap.HEAD != "" {
		m.sel = snap.HEAD
		m.startDetail(snap.HEAD)
	} else if m.sel != "" {
		found := m.sel == gitcli.Uncommitted && snap.Status.Dirty()
		if !found {
			for _, c := range snap.Commits {
				if c.Hash == m.sel {
					found = true
					break
				}
			}
		}
		if !found && snap.HEAD != "" {
			m.sel = snap.HEAD
			m.startDetail(snap.HEAD)
		} else if found && m.sel != gitcli.Uncommitted && (m.detail.Hash != m.sel || decoKey(m.detail.Decorations) != decoKey(decorationsFor(snap, m.sel))) {
			m.startDetail(m.sel)
		}
	}
	m.generation++
}

func (m *gitSession) drainOp() {
	if m.pendingOp == nil {
		return
	}
	select {
	case res := <-m.pendingOp:
		m.pendingOp = nil
		if res.err != nil {
			var status statusError
			if errors.As(res.err, &status) {
				m.SetStatus(status.key, status.err)
				if status.reload {
					m.reload()
				}
			} else {
				key := res.errKey
				if key == "" {
					key = i18n.StatusGitOpFailed
				}
				m.SetStatus(key, res.err)
			}
		} else {
			m.SetStatus(res.okKey)
			m.reload()
		}
		guigui.RequestRebuild()
	default:
	}
}

func (m *gitSession) drainDetail() {
	if m.pendingDetail == nil {
		return
	}
	select {
	case res := <-m.pendingDetail:
		m.pendingDetail = nil
		if res.err != nil || res.hash != m.sel {
			return
		}
		m.detail = res.detail
		m.generation++
		guigui.RequestRebuild()
	default:
	}
}

func (m *gitSession) SelectCommit(hash string) {
	if hash == "" || hash == m.sel {
		return
	}
	m.sel = hash
	m.generation++
	if hash == gitcli.Uncommitted {
		m.pendingDetail = nil
		m.detail = gitcli.CommitDetail{}
		return
	}
	m.startDetail(hash)
}

// RevealCommit selects hash and asks the graph to bring that row into view.
func (m *gitSession) RevealCommit(hash string) {
	if hash == "" || hash == gitcli.Uncommitted {
		return
	}
	m.reveal = hash
	m.SelectCommit(hash)
	guigui.RequestRebuild()
}

// TakeReveal returns the hash waiting to be scrolled into view, once.
func (m *gitSession) TakeReveal() string {
	h := m.reveal
	m.reveal = ""
	return h
}

func (m *gitSession) startDetail(hash string) {
	if m.repo == nil || hash == "" {
		return
	}
	ch := make(chan gitDetailResult, 1)
	m.pendingDetail = ch
	repo := m.repo
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitLoadTimeout)
		defer cancel()
		d, err := repo.Detail(ctx, hash)
		ch <- gitDetailResult{hash: hash, detail: d, err: err}
	}()
}

func (m *gitSession) SelectedCommit() (gitcli.Commit, bool) {
	for _, c := range m.snap.Commits {
		if c.Hash == m.sel {
			return c, true
		}
	}
	return gitcli.Commit{}, false
}

func (m *gitSession) CanCommit() bool {
	if !m.HasRepo() || m.Busy() {
		return false
	}
	if strings.TrimSpace(m.draft) == "" {
		return false
	}
	if m.StageAll() {
		return m.snap.Status.Dirty()
	}
	return m.snap.Status.HasStaged()
}

func (m *gitSession) CanAmend() bool {
	return m.HasRepo() && !m.Busy() && m.snap.HEAD != ""
}

func (m *gitSession) CanPushPull() bool {
	return m.HasRepo() && !m.Busy()
}

func (m *gitSession) startOp(ok, fail i18n.Key, fn func(ctx context.Context, repo *gitcli.Repo) error) {
	if m.Busy() || m.repo == nil {
		return
	}
	m.SetStatus(i18n.StatusGitWorking)
	ch := make(chan gitOpResult, 1)
	m.pendingOp = ch
	m.generation++
	repo := m.repo
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
		defer cancel()
		err := fn(ctx, repo)
		ch <- gitOpResult{okKey: ok, errKey: fail, err: err}
	}()
}

func (m *gitSession) DoStage(paths []string) {
	if !m.HasRepo() || m.Busy() || len(paths) == 0 {
		return
	}
	paths = append([]string(nil), paths...)
	m.startOp(i18n.StatusGitStageOk, i18n.StatusGitStageFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Stage(ctx, paths)
	})
}

func (m *gitSession) DoUnstage(paths []string) {
	if !m.HasRepo() || m.Busy() || len(paths) == 0 {
		return
	}
	paths = append([]string(nil), paths...)
	m.startOp(i18n.StatusGitUnstageOk, i18n.StatusGitUnstageFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Unstage(ctx, paths)
	})
}

// DoApplyPatch stages or unstages the lines described by patch.
func (m *gitSession) DoApplyPatch(patch string, reverse bool) {
	if !m.HasRepo() || m.Busy() || strings.TrimSpace(patch) == "" {
		return
	}
	okKey, errKey := i18n.StatusGitStageOk, i18n.StatusGitStageFailed
	if reverse {
		okKey, errKey = i18n.StatusGitUnstageOk, i18n.StatusGitUnstageFailed
	}
	m.startOp(okKey, errKey, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.ApplyIndex(ctx, patch, reverse)
	})
}

// WorkDiff returns the patch for path. ready is false while the diff is loading.
// Staged and unstaged diffs are cached separately.
func (m *gitSession) WorkDiff(path string, staged bool) (string, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" || m.repo == nil {
		return "", true, nil
	}
	slot := &m.workSlots[0]
	if staged {
		slot = &m.workSlots[1]
	}
	key := m.workDiffKey(path, staged)
	if slot.key == key {
		return slot.diff, true, slot.err
	}
	if slot.pending == nil {
		m.startWorkDiff(slot, path, staged, key)
	}
	return "", false, nil
}

func (m *gitSession) workDiffKey(path string, staged bool) string {
	side := "0"
	if staged {
		side = "1"
	}
	var b strings.Builder
	b.WriteString(path)
	b.WriteByte(0)
	b.WriteString(side)
	b.WriteByte(0)
	for _, e := range m.snap.Status.Entries {
		b.WriteString(e.Code)
		b.WriteByte(' ')
		b.WriteString(e.Path)
		b.WriteByte('\n')
	}
	if info, err := os.Stat(filepath.Join(m.repo.Dir, filepath.FromSlash(path))); err == nil {
		fmt.Fprintf(&b, "%d %d", info.ModTime().UnixNano(), info.Size())
	}
	return b.String()
}

func (m *gitSession) startWorkDiff(slot *gitWorkSlot, path string, staged bool, key string) {
	if m.repo == nil {
		return
	}
	ch := make(chan gitWorkResult, 1)
	slot.pending = ch
	repo := m.repo
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitLoadTimeout)
		defer cancel()
		diff, err := repo.DiffWork(ctx, path, staged)
		ch <- gitWorkResult{key: key, diff: diff, err: err}
	}()
}

func (m *gitSession) drainWork() {
	rebuilt := false
	for i := range m.workSlots {
		slot := &m.workSlots[i]
		if slot.pending == nil {
			continue
		}
		select {
		case res := <-slot.pending:
			slot.pending = nil
			slot.key = res.key
			slot.diff = res.diff
			slot.err = res.err
			rebuilt = true
		default:
		}
	}
	if rebuilt {
		m.generation++
		guigui.RequestRebuild()
	}
}

func (m *gitSession) DoCommit() {
	if !m.CanCommit() {
		if strings.TrimSpace(m.draft) == "" {
			m.SetStatus(i18n.StatusGitNeedMessage)
		} else {
			m.SetStatus(i18n.StatusGitNoChanges)
		}
		return
	}
	msg := m.draft
	stage := m.StageAll()
	m.startOp(i18n.StatusGitCommitOk, i18n.StatusGitCommitFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Commit(ctx, msg, stage, false)
	})
}

func (m *gitSession) DoAmend() {
	if !m.CanAmend() {
		return
	}
	msg := m.draft
	stage := m.StageAll()
	m.startOp(i18n.StatusGitAmendOk, i18n.StatusGitAmendFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Commit(ctx, msg, stage, true)
	})
}

func (m *gitSession) DoFetch() {
	if !m.CanPushPull() {
		return
	}
	m.startOp(i18n.StatusGitFetchOk, i18n.StatusGitFetchFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Fetch(ctx)
	})
}

func (m *gitSession) DoPull() {
	if !m.CanPushPull() {
		return
	}
	m.startOp(i18n.StatusGitPullOk, i18n.StatusGitPullFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Pull(ctx)
	})
}

func (m *gitSession) DoPush() {
	if !m.CanPushPull() {
		return
	}
	m.startOp(i18n.StatusGitPushOk, i18n.StatusGitPushFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Push(ctx)
	})
}

func (m *gitSession) DoCreateTag(hash, name string, push bool) {
	if !m.HasRepo() || m.Busy() || hash == "" || hash == gitcli.Uncommitted {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.startOp(i18n.StatusGitTagOk, i18n.StatusGitTagFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		if err := repo.CreateTag(ctx, name, hash); err != nil {
			return err
		}
		if !push {
			return nil
		}
		if err := repo.PushTag(ctx, name); err != nil {
			return statusError{key: i18n.StatusGitTagPushFailed, err: err, reload: true}
		}
		return nil
	})
}

func (m *gitSession) DoDeleteTag(name string) {
	if !m.HasRepo() || m.Busy() {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.startOp(i18n.StatusGitTagDeleted, i18n.StatusGitTagDeleteErr, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.DeleteTag(ctx, name)
	})
}

func (m *gitSession) DoPushTag(name string) {
	if !m.HasRepo() || m.Busy() {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.startOp(i18n.StatusGitTagPushed, i18n.StatusGitTagPushErr, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.PushTag(ctx, name)
	})
}

func (m *gitSession) DoDeleteRemoteTag(name string) {
	if !m.HasRepo() || m.Busy() {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.startOp(i18n.StatusGitTagRemoteOk, i18n.StatusGitTagRemoteErr, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.DeleteRemoteTag(ctx, name)
	})
}

// statusError is an operation failure that uses its own status text.
// reload asks the session to read the repository again after a partial success.
type statusError struct {
	key    i18n.Key
	err    error
	reload bool
}

func (e statusError) Error() string { return e.err.Error() }
func (e statusError) Unwrap() error { return e.err }

func (m *gitSession) DoCreateBranch(hash, name string) {
	if !m.HasRepo() || m.Busy() || hash == "" || hash == gitcli.Uncommitted {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.startOp(i18n.StatusGitBranchOk, i18n.StatusGitBranchFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.CreateBranch(ctx, name, hash)
	})
}

func (m *gitSession) DoRenameBranch(ref gitcli.Ref, name string) {
	if !m.HasRepo() || m.Busy() || ref.Name == "" || ref.Remote != "" {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || name == ref.Name {
		return
	}
	m.startOp(i18n.StatusGitRenameOk, i18n.StatusGitRenameFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.RenameBranch(ctx, ref, name)
	})
}

func (m *gitSession) DoDeleteBranch(ref gitcli.Ref) {
	if !m.HasRepo() || m.Busy() || ref.Name == "" {
		return
	}
	if ref.Current {
		m.SetStatus(i18n.StatusGitDeleteCurrent)
		return
	}
	m.startOp(i18n.StatusGitDeleteOk, i18n.StatusGitDeleteFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.DeleteBranch(ctx, ref)
	})
}

func (m *gitSession) DoCheckout(ref gitcli.Ref) {
	if !m.HasRepo() || m.Busy() || ref.Current || ref.Name == "" {
		return
	}
	if m.snap.Status.Dirty() {
		m.SetStatus(i18n.StatusGitCheckoutDirty)
		return
	}
	okKey, errKey := i18n.StatusGitCheckoutOk, i18n.StatusGitCheckoutFailed
	if ref.IsTag() {
		okKey, errKey = i18n.StatusGitTagSwitchOk, i18n.StatusGitTagSwitchErr
	}
	m.startOp(okKey, errKey, func(ctx context.Context, repo *gitcli.Repo) error {
		return repo.Checkout(ctx, ref)
	})
}

// DoCheckoutAndPull switches to an existing local branch, then fast-forwards it
// from the remote branch the user asked to switch to.
func (m *gitSession) DoCheckoutAndPull(local, remote gitcli.Ref) {
	if !m.HasRepo() || m.Busy() || local.Name == "" || local.Remote != "" || remote.Remote == "" {
		return
	}
	if !local.Current && m.snap.Status.Dirty() {
		m.SetStatus(i18n.StatusGitCheckoutDirty)
		return
	}
	switched := !local.Current
	okKey := i18n.StatusGitPullOk
	if switched {
		okKey = i18n.StatusGitCheckoutPullOk
	}
	m.startOp(okKey, i18n.StatusGitCheckoutFailed, func(ctx context.Context, repo *gitcli.Repo) error {
		if switched {
			if err := repo.Checkout(ctx, local); err != nil {
				return err
			}
		}
		if err := repo.PullRef(ctx, remote); err != nil {
			return statusError{key: i18n.StatusGitPullFailed, err: err, reload: true}
		}
		return nil
	})
}

func cloneHidden(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]bool, len(src))
	for k, v := range src {
		if v {
			out[k] = true
		}
	}
	return out
}
