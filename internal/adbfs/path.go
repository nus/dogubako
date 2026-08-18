package adbfs

import (
	"path"
	"strings"
)

// Clean returns a cleaned absolute Unix path for Android.
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

// Join joins Android path elements with slashes.
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

// Base returns the last element of an Android path.
func Base(p string) string {
	p = Clean(p)
	if p == "/" {
		return "/"
	}
	return path.Base(p)
}

// Parent returns the parent directory of an Android path.
func Parent(p string) string {
	p = Clean(p)
	if p == "/" {
		return "/"
	}
	return path.Dir(p)
}

// InDir reports whether child is path dir itself or a descendant of dir.
func InDir(dir, child string) bool {
	dir = Clean(dir)
	child = Clean(child)
	if dir == "/" {
		return true
	}
	return child == dir || strings.HasPrefix(child, dir+"/")
}
