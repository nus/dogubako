package gitcli

import (
	"strings"
	"time"
)

const fieldSep = "\x1f"

// Commit is one node in the log used by the graph.
type Commit struct {
	Hash        string
	Parents     []string
	Author      string
	Email       string
	When        time.Time
	Subject     string
	Decorations []Decoration
}

func (c Commit) Short() string {
	if len(c.Hash) >= 7 {
		return c.Hash[:7]
	}
	return c.Hash
}

// Decoration is a ref name git reports on a commit (%D).
type Decoration struct {
	Kind string // head, tag, remote, other
	Name string // main, origin/main, v1.0
	HEAD bool
}

// Ref is a local or remote-tracking branch.
type Ref struct {
	Name    string // short: main, origin/main
	Full    string // refs/heads/main
	Hash    string
	Current bool
	Remote  string // empty for local
}

func (r Ref) LocalName() string {
	if r.Remote == "" {
		return r.Name
	}
	return strings.TrimPrefix(r.Name, r.Remote+"/")
}

// IsTag reports whether the ref is a tag.
func (r Ref) IsTag() bool {
	return strings.HasPrefix(r.Full, "refs/tags/")
}

// Remote is a configured remote.
type Remote struct {
	Name string
	URL  string
}

// Status is porcelain v1 plus branch header.
type Status struct {
	Branch   string
	Detached bool
	Ahead    int
	Behind   int
	Entries  []StatusEntry
}

func (s Status) Dirty() bool {
	return len(s.Entries) > 0
}

func (s Status) HasStaged() bool {
	for _, e := range s.Entries {
		if e.Staged() {
			return true
		}
	}
	return false
}

// StatusEntry is one porcelain line.
type StatusEntry struct {
	Code string // two-letter XY
	Path string
}

func (e StatusEntry) Staged() bool {
	if len(e.Code) == 0 {
		return false
	}
	c := e.Code[0]
	return c != ' ' && c != '?' && c != '!'
}

func (e StatusEntry) Unstaged() bool {
	if len(e.Code) < 2 {
		return e.Code == "??"
	}
	c := e.Code[1]
	return c != ' '
}

func (e StatusEntry) Untracked() bool {
	return e.Code == "??"
}

func (e StatusEntry) String() string {
	return strings.TrimSpace(e.Code + " " + e.Path)
}

// CommitDetail is the bottom-panel payload for one commit.
type CommitDetail struct {
	Commit
	Body  string
	Files []string
	Diff  string
}

func parseLog(stdout string) []Commit {
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	if strings.TrimSpace(stdout) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	out := make([]Commit, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		c, ok := parseLogLine(line)
		if ok {
			out = append(out, c)
		}
	}
	return out
}

func parseLogLine(line string) (Commit, bool) {
	parts := strings.Split(line, fieldSep)
	if len(parts) < 7 {
		return Commit{}, false
	}
	var when time.Time
	if t, err := time.Parse(time.RFC3339, parts[4]); err == nil {
		when = t
	}
	var parents []string
	if p := strings.TrimSpace(parts[1]); p != "" {
		parents = strings.Fields(p)
	}
	return Commit{
		Hash:        parts[0],
		Parents:     parents,
		Author:      parts[2],
		Email:       parts[3],
		When:        when,
		Subject:     parts[5],
		Decorations: parseDecorations(parts[6]),
	}, true
}

func parseDecorations(s string) []Decoration {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []Decoration
	for _, raw := range strings.Split(s, ", ") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		d := Decoration{Kind: "other", Name: raw}
		if strings.HasPrefix(raw, "HEAD -> ") {
			d.HEAD = true
			d.Kind = "head"
			d.Name = strings.TrimPrefix(raw, "HEAD -> ")
		} else if raw == "HEAD" {
			d.HEAD = true
			d.Kind = "head"
			d.Name = "HEAD"
		} else if strings.HasPrefix(raw, "tag: ") {
			d.Kind = "tag"
			d.Name = strings.TrimPrefix(raw, "tag: ")
		} else if strings.Contains(raw, "/") {
			d.Kind = "remote"
			d.Name = raw
		} else {
			d.Kind = "head"
			d.Name = raw
		}
		out = append(out, d)
	}
	return out
}

func filterDecorations(ds []Decoration, hideRemotes map[string]bool) []Decoration {
	if len(hideRemotes) == 0 {
		return ds
	}
	out := ds[:0:0]
	for _, d := range ds {
		if d.Kind == "remote" {
			remote, _, ok := strings.Cut(d.Name, "/")
			if ok && hideRemotes[remote] {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

func parseRefs(stdout string) []Ref {
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	if strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []Ref
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		parts := strings.Split(line, "\x00")
		if len(parts) < 3 {
			continue
		}
		full := parts[0]
		hash := parts[1]
		head := parts[2] == "*"
		r := Ref{Full: full, Hash: hash, Current: head}
		switch {
		case strings.HasPrefix(full, "refs/heads/"):
			r.Name = strings.TrimPrefix(full, "refs/heads/")
		case strings.HasPrefix(full, "refs/remotes/"):
			rest := strings.TrimPrefix(full, "refs/remotes/")
			r.Name = rest
			if i := strings.IndexByte(rest, '/'); i > 0 {
				r.Remote = rest[:i]
			}
		default:
			continue
		}
		if r.Name == "HEAD" || strings.HasSuffix(r.Name, "/HEAD") {
			continue
		}
		out = append(out, r)
	}
	return out
}

func parseRemotes(stdout string) []Remote {
	var out []Remote
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if len(fields) >= 3 && fields[2] != "(fetch)" {
			continue
		}
		out = append(out, Remote{Name: fields[0], URL: fields[1]})
	}
	return out
}

func parseStatus(stdout string) Status {
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	var st Status
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			st = parseBranchHeader(strings.TrimPrefix(line, "## "))
			continue
		}
		if len(line) < 3 {
			continue
		}
		st.Entries = append(st.Entries, StatusEntry{
			Code: line[:2],
			Path: strings.TrimSpace(line[3:]),
		})
	}
	return st
}

func parseBranchHeader(s string) Status {
	var st Status
	if strings.HasPrefix(s, "HEAD (no branch)") || strings.HasPrefix(s, "HEAD (detached") {
		st.Detached = true
		st.Branch = "HEAD"
		return st
	}
	if strings.HasPrefix(s, "No commits yet on ") {
		st.Branch = strings.TrimPrefix(s, "No commits yet on ")
		if i := strings.Index(st.Branch, "..."); i >= 0 {
			st.Branch = st.Branch[:i]
		}
		return st
	}
	head, rest, _ := strings.Cut(s, "...")
	st.Branch = head
	if i := strings.Index(st.Branch, " "); i >= 0 {
		st.Branch = st.Branch[:i]
	}
	if a := extractCount(rest, "ahead "); a > 0 {
		st.Ahead = a
	}
	if b := extractCount(rest, "behind "); b > 0 {
		st.Behind = b
	}
	return st
}

func extractCount(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return 0
	}
	s = s[i+len(key):]
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func parseNameStatus(stdout string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
