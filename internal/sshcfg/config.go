// Package sshcfg reads Host aliases from the user's OpenSSH config.
package sshcfg

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UserHosts returns Host aliases from ~/.ssh/config, following Include.
// Wildcard patterns are omitted. A missing config file is an empty list.
func UserHosts() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "config")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseFile(path, home, 0, map[string]bool{})
}

func parseFile(path, home string, depth int, seen map[string]bool) ([]string, error) {
	if depth > 10 {
		return nil, nil
	}
	path = filepath.Clean(path)
	if seen[path] {
		return nil, nil
	}
	seen[path] = true
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f, home, depth, seen)
}

func parse(r io.Reader, home string, depth int, seen map[string]bool) ([]string, error) {
	var hosts []string
	known := map[string]bool{}
	add := func(name string) {
		if name == "" || known[name] || isPattern(name) {
			return
		}
		known[name] = true
		hosts = append(hosts, name)
	}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := splitWords(sc.Text())
		if len(fields) == 0 {
			continue
		}
		key, args := keyword(fields)
		switch strings.ToLower(key) {
		case "host":
			for _, tok := range args {
				add(tok)
			}
		case "include":
			for _, tok := range args {
				more, err := includeHosts(tok, home, depth, seen)
				if err != nil {
					return nil, err
				}
				for _, name := range more {
					add(name)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func includeHosts(tok, home string, depth int, seen map[string]bool) ([]string, error) {
	pattern := expandPath(tok, home)
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(home, ".ssh", pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, nil
	}
	var hosts []string
	for _, match := range matches {
		fi, err := os.Stat(match)
		if err != nil || fi.IsDir() {
			continue
		}
		more, err := parseFile(match, home, depth+1, seen)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, more...)
	}
	return hosts, nil
}

func expandPath(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func keyword(fields []string) (string, []string) {
	key := fields[0]
	args := fields[1:]
	if i := strings.IndexByte(key, '='); i >= 0 {
		if key[i+1:] != "" {
			args = append([]string{key[i+1:]}, args...)
		}
		key = key[:i]
	}
	if len(args) > 0 && args[0] == "=" {
		args = args[1:]
	}
	return key, args
}

func isPattern(tok string) bool {
	return strings.ContainsAny(tok, "*?!")
}

func splitWords(line string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if c == '"' {
				inQuote = false
				continue
			}
			if c == '\\' && i+1 < len(line) {
				i++
				b.WriteByte(line[i])
				continue
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case '#':
			if b.Len() > 0 {
				out = append(out, b.String())
			}
			return out
		case '"':
			inQuote = true
		case ' ', '\t':
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteByte(c)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
