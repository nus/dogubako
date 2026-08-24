package mtpfs

import (
	"path"
	"strings"
)

// Clean returns a cleaned absolute Unix-style MTP path.
func Clean(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	cleaned := path.Clean(p)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

// Join joins MTP path elements with slashes.
func Join(elem ...string) string {
	if len(elem) == 0 {
		return "/"
	}
	parts := make([]string, 0, len(elem))
	for _, e := range elem {
		e = strings.ReplaceAll(e, "\\", "/")
		if e == "" {
			continue
		}
		parts = append(parts, e)
	}
	if len(parts) == 0 {
		return "/"
	}
	return Clean(path.Join(parts...))
}

// Base returns the last element of an MTP path.
func Base(p string) string {
	p = Clean(p)
	if p == "/" {
		return "/"
	}
	return path.Base(p)
}

// Parent returns the parent directory of an MTP path.
func Parent(p string) string {
	p = Clean(p)
	if p == "/" {
		return "/"
	}
	return path.Dir(p)
}
