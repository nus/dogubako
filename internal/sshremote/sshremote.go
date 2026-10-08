// Package sshremote runs non-interactive commands on an SSH destination.
package sshremote

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Target is an ssh destination. Port is empty when ssh should use its default
// or the port from the user's config (typical for a Host alias).
type Target struct {
	Dest string
	Port string
}

// ValidDest reports whether dest can be passed as a single ssh destination.
func ValidDest(dest string) bool {
	if dest == "" || strings.HasPrefix(dest, "-") {
		return false
	}
	return !strings.ContainsAny(dest, "\r\n\x00")
}

// ValidPort reports whether port is empty or a TCP port.
func ValidPort(port string) bool {
	if port == "" {
		return true
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

// Quote quotes s for a POSIX shell single-quoted string.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Command runs remote via ssh. remote is one shell command, already quoted
// for the remote user's login shell. Callers that need POSIX syntax should
// wrap it in sh -c.
func Command(ctx context.Context, t Target, remote string) *exec.Cmd {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=15",
		"-o", "ClearAllForwardings=yes",
	}
	args = append(args, controlArgs()...)
	if t.Port != "" {
		args = append(args, "-p", t.Port)
	}
	args = append(args, t.Dest, remote)
	return exec.CommandContext(ctx, "ssh", args...)
}

// controlArgs reuses one ssh connection for the burst of git commands a view needs.
func controlArgs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".ssh")
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return nil
	}
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=60",
		"-o", "ControlPath=" + filepath.Join(dir, "dogubako-%C"),
	}
}

func output(ctx context.Context, t Target, remote string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !ValidDest(t.Dest) || !ValidPort(t.Port) {
		return "", errors.New("invalid ssh destination")
	}
	cmd := Command(ctx, t, remote)
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

func sh(script string) string {
	return "sh -c " + Quote(script)
}

const homeScript = `
if [ -n "$HOME" ] && [ -d "$HOME" ]; then
  printf '%s\n' "$HOME"
else
  pwd
fi
`

// Home returns the remote login directory.
func Home(ctx context.Context, t Target) (string, error) {
	out, err := output(ctx, t, sh(homeScript))
	if err != nil {
		return "", err
	}
	dir := path.Clean(strings.TrimSpace(out))
	if !strings.HasPrefix(dir, "/") {
		return "", errors.New("remote home is not absolute")
	}
	return dir, nil
}

// Entry is a directory on the remote host.
type Entry struct {
	Name string
}

const listBody = `
if [ ! -d "$d" ]; then
  printf '%s\n' 'ERR not a directory'
  exit 1
fi
if git -C "$d" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  printf '%s\n' REPO
else
  printf '%s\n' DIR
fi
find "$d" -mindepth 1 -maxdepth 1 ! -name .git -print | while IFS= read -r p; do
  [ -d "$p" ] && printf '%s\n' "$p"
done | LC_ALL=C sort
`

// List returns subdirectories of dir and whether dir is inside a work tree.
func List(ctx context.Context, t Target, dir string) ([]Entry, bool, error) {
	dir = path.Clean(strings.TrimSpace(dir))
	if !strings.HasPrefix(dir, "/") {
		return nil, false, errors.New("directory must be absolute")
	}
	script := "d=" + Quote(dir) + "\n" + listBody
	out, err := output(ctx, t, sh(script))
	if err != nil {
		return nil, false, err
	}
	entries, isRepo, err := ParseList(out)
	return entries, isRepo, err
}

// ParseList parses the stdout of List.
func ParseList(out string) ([]Entry, bool, error) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, false, errors.New("empty listing")
	}
	lines := strings.Split(out, "\n")
	switch lines[0] {
	case "ERR not a directory":
		return nil, false, errors.New("not a directory")
	case "REPO", "DIR":
	default:
		return nil, false, errors.New("unexpected listing")
	}
	isRepo := lines[0] == "REPO"
	entries := make([]Entry, 0, len(lines)-1)
	seen := make(map[string]bool, len(lines)-1)
	for _, line := range lines[1:] {
		name := path.Base(strings.TrimSpace(line))
		if name == "" || name == "." || name == ".." || name == ".git" || seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, Entry{Name: name})
	}
	return entries, isRepo, nil
}
