package gitcli

import (
	"net/url"
	"path"
	"strings"
)

// Loc is a working tree on this machine or on an SSH host.
// Dest is a Host alias from ssh config, or an explicit destination such as user@host.
// Port is set only for an explicit destination; a config Host keeps Port empty
// so ssh uses the port written in the config.
type Loc struct {
	Dest string
	Port string
	Dir  string
}

// IsRemote reports whether git runs on another host.
func (l Loc) IsRemote() bool { return l.Dest != "" }

// Key is the string stored for tabs and recent repositories.
// Local locations are the directory path. Remote locations use an ssh: prefix.
func (l Loc) Key() string {
	if !l.IsRemote() {
		return l.Dir
	}
	return "ssh:" + url.PathEscape(l.Dest) + ":" + url.PathEscape(l.Port) + ":" + path.Clean(l.Dir)
}

// Display is the path shown in the UI.
func (l Loc) Display() string {
	if !l.IsRemote() {
		return l.Dir
	}
	head := l.Dest
	if l.Port != "" {
		head += ":" + l.Port
	}
	return head + ":" + l.Dir
}

// ParseLoc decodes a tab path. Paths without the ssh: prefix are local.
func ParseLoc(s string) Loc {
	s = strings.TrimSpace(s)
	const prefix = "ssh:"
	if !strings.HasPrefix(s, prefix) {
		return Loc{Dir: s}
	}
	rest := s[len(prefix):]
	destEsc, rest, ok := strings.Cut(rest, ":")
	if !ok {
		return Loc{Dir: s}
	}
	portEsc, dir, ok := strings.Cut(rest, ":")
	if !ok || !strings.HasPrefix(dir, "/") {
		return Loc{Dir: s}
	}
	dest, err1 := url.PathUnescape(destEsc)
	port, err2 := url.PathUnescape(portEsc)
	if err1 != nil || err2 != nil || dest == "" {
		return Loc{Dir: s}
	}
	return Loc{Dest: dest, Port: port, Dir: path.Clean(dir)}
}
