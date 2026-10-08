package app

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/sshcfg"
	"github.com/nus/dogubako/internal/sshremote"
)

const sshOpTimeout = 25 * time.Second

// sshPick is the SSH host and directory chooser for an empty Git tab.
type sshPick struct {
	open      bool
	hosts     []string
	connected bool
	dest      string
	port      string
	dir       string
	entries   []sshremote.Entry
	isRepo    bool
	errKey    i18n.Key
	errText   string
	listGen   uint64
	seq       uint64
	pending   <-chan sshResult
	cancel    context.CancelFunc
}

type sshResult struct {
	seq     uint64
	dir     string
	entries []sshremote.Entry
	isRepo  bool
	err     error
}

// BeginSSH opens the chooser and loads Host aliases from ~/.ssh/config.
func (m *GitModel) BeginSSH() {
	m.ensure()
	if m.ssh.open {
		return
	}
	hosts, err := sshcfg.UserHosts()
	m.ssh = sshPick{open: true, hosts: hosts, listGen: m.nextSSHGen()}
	if err != nil {
		m.ssh.errKey = i18n.GitSSHFailed
		m.ssh.errText = err.Error()
	}
	m.extraGen++
}

// CancelSSH closes the chooser.
func (m *GitModel) CancelSSH() {
	m.ensure()
	m.stopSSH()
	m.ssh = sshPick{}
	m.extraGen++
}

// SSHBack returns from the directory list to the destination chooser.
func (m *GitModel) SSHBack() {
	m.ensure()
	if !m.ssh.open {
		return
	}
	m.stopSSH()
	m.ssh.connected = false
	m.ssh.dir = ""
	m.ssh.entries = nil
	m.ssh.isRepo = false
	m.ssh.errKey = ""
	m.ssh.errText = ""
	m.ssh.listGen = m.nextSSHGen()
	m.extraGen++
}

func (m *GitModel) SSHActive() bool { return m.ssh.open }

func (m *GitModel) SSHConnected() bool { return m.ssh.open && m.ssh.connected }

func (m *GitModel) SSHBusy() bool { return m.ssh.pending != nil }

func (m *GitModel) SSHHosts() []string { return slices.Clone(m.ssh.hosts) }

func (m *GitModel) SSHDir() string { return m.ssh.dir }

func (m *GitModel) SSHEntries() []sshremote.Entry { return slices.Clone(m.ssh.entries) }

func (m *GitModel) SSHIsRepo() bool { return m.ssh.isRepo }

func (m *GitModel) SSHListGen() uint64 { return m.ssh.listGen }

// SSHTarget is the destination label, including a port when one was entered.
func (m *GitModel) SSHTarget() string {
	if m.ssh.port == "" {
		return m.ssh.dest
	}
	return m.ssh.dest + ":" + m.ssh.port
}

// SSHCanUp reports whether the remote directory has a parent.
func (m *GitModel) SSHCanUp() bool {
	return m.ssh.connected && m.ssh.dir != "" && m.ssh.dir != "/"
}

// SSHStatus is the chooser's status line.
func (m *GitModel) SSHStatus(lang i18n.Lang) string {
	if m.ssh.pending != nil {
		return i18n.T(lang, i18n.GitSSHConnecting)
	}
	if m.ssh.errKey == "" {
		return ""
	}
	if m.ssh.errText != "" {
		return i18n.T(lang, m.ssh.errKey, m.ssh.errText)
	}
	return i18n.T(lang, m.ssh.errKey)
}

// SSHConnect logs in to dest and lists the remote home directory.
// An empty port uses the ssh default or the port from ssh config.
func (m *GitModel) SSHConnect(dest, port string) {
	m.ensure()
	if !m.ssh.open || m.ssh.pending != nil {
		return
	}
	dest = strings.TrimSpace(dest)
	port = strings.TrimSpace(port)
	if !sshremote.ValidDest(dest) {
		m.setSSHErr(i18n.GitSSHBadDest, "")
		return
	}
	if !sshremote.ValidPort(port) {
		m.setSSHErr(i18n.GitSSHBadPort, "")
		return
	}
	m.ssh.dest = dest
	m.ssh.port = port
	m.ssh.errKey = ""
	m.ssh.errText = ""
	target := sshremote.Target{Dest: dest, Port: port}
	m.startSSH(func(ctx context.Context) sshResult {
		dir, err := sshremote.Home(ctx, target)
		if err != nil {
			return sshResult{err: err}
		}
		entries, isRepo, err := sshremote.List(ctx, target, dir)
		if err != nil {
			return sshResult{err: err}
		}
		return sshResult{dir: dir, entries: entries, isRepo: isRepo}
	})
}

// SSHEnter lists a subdirectory of the current remote directory.
func (m *GitModel) SSHEnter(name string) {
	m.ensure()
	if !m.ssh.connected || m.ssh.pending != nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return
	}
	m.listSSH(joinRemote(m.ssh.dir, name))
}

// SSHUp lists the parent directory.
func (m *GitModel) SSHUp() {
	m.ensure()
	if !m.SSHCanUp() || m.ssh.pending != nil {
		return
	}
	parent := parentRemote(m.ssh.dir)
	if parent == m.ssh.dir {
		return
	}
	m.listSSH(parent)
}

// SSHGo lists an absolute remote directory.
func (m *GitModel) SSHGo(dir string) {
	m.ensure()
	if !m.ssh.connected || m.ssh.pending != nil {
		return
	}
	dir = strings.TrimSpace(dir)
	if !strings.HasPrefix(dir, "/") {
		m.setSSHErr(i18n.GitSSHBadDir, "")
		return
	}
	m.listSSH(cleanRemote(dir))
}

// SSHChoose opens the current remote directory as a repository.
func (m *GitModel) SSHChoose() {
	m.ensure()
	if !m.ssh.connected || m.ssh.dir == "" || m.ssh.pending != nil {
		return
	}
	key := gitcli.Loc{Dest: m.ssh.dest, Port: m.ssh.port, Dir: m.ssh.dir}.Key()
	m.CancelSSH()
	m.Open(key)
}

func (m *GitModel) listSSH(dir string) {
	m.ssh.errKey = ""
	m.ssh.errText = ""
	target := sshremote.Target{Dest: m.ssh.dest, Port: m.ssh.port}
	m.startSSH(func(ctx context.Context) sshResult {
		entries, isRepo, err := sshremote.List(ctx, target, dir)
		if err != nil {
			return sshResult{err: err}
		}
		return sshResult{dir: dir, entries: entries, isRepo: isRepo}
	})
}

func (m *GitModel) startSSH(fn func(context.Context) sshResult) {
	m.stopSSH()
	ctx, cancel := context.WithTimeout(context.Background(), sshOpTimeout)
	m.ssh.cancel = cancel
	m.ssh.seq++
	seq := m.ssh.seq
	ch := make(chan sshResult, 1)
	m.ssh.pending = ch
	m.extraGen++
	go func() {
		res := fn(ctx)
		res.seq = seq
		ch <- res
	}()
}

func (m *GitModel) stopSSH() {
	if m.ssh.cancel != nil {
		m.ssh.cancel()
		m.ssh.cancel = nil
	}
	m.ssh.seq++
	m.ssh.pending = nil
}

func (m *GitModel) drainSSH() {
	if m.ssh.pending == nil {
		return
	}
	select {
	case res := <-m.ssh.pending:
		m.ssh.pending = nil
		if m.ssh.cancel != nil {
			m.ssh.cancel()
			m.ssh.cancel = nil
		}
		if res.seq != m.ssh.seq {
			m.extraGen++
			return
		}
		if res.err != nil {
			m.setSSHErr(i18n.GitSSHFailed, res.err.Error())
			guigui.RequestRebuild()
			return
		}
		m.ssh.errKey = ""
		m.ssh.errText = ""
		m.ssh.connected = true
		m.ssh.dir = res.dir
		m.ssh.entries = res.entries
		m.ssh.isRepo = res.isRepo
		m.ssh.listGen = m.nextSSHGen()
		m.extraGen++
		guigui.RequestRebuild()
	default:
	}
}

func (m *GitModel) setSSHErr(key i18n.Key, text string) {
	m.ssh.errKey = key
	m.ssh.errText = text
	m.ssh.listGen = m.nextSSHGen()
	m.extraGen++
}

func (m *GitModel) nextSSHGen() uint64 {
	m.sshGen++
	return m.sshGen
}

func joinRemote(dir, name string) string {
	return path.Join(dir, name)
}

func parentRemote(dir string) string {
	dir = path.Clean(dir)
	if dir == "/" {
		return "/"
	}
	return path.Dir(dir)
}

func cleanRemote(dir string) string {
	dir = path.Clean(dir)
	if !strings.HasPrefix(dir, "/") {
		return "/"
	}
	return dir
}
