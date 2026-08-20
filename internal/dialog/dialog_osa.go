package dialog

import (
	"errors"
	"os/exec"
	"strings"
)

type osaKind int

const (
	osaOpen osaKind = iota
	osaSave
	osaDir
)

func tryOSDialog(kind osaKind, title, suggested string, filter *FileFilter) (FileResult, bool) {
	if _, err := execLookPath("osascript"); err != nil {
		return FileResult{}, false
	}
	out, err := execOSA(osaScript(kind, title, suggested, filter))
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return FileResult{Cancelled: true}, true
		}
		if errorsIsCancel(err) {
			return FileResult{Cancelled: true}, true
		}
		if isCommandMissing(err) {
			return FileResult{}, false
		}
		return FileResult{Err: err}, false
	}
	path := trimPath(string(out))
	if path == "" {
		return FileResult{Cancelled: true}, true
	}
	return FileResult{Path: path}, true
}

func osaScript(kind osaKind, title, suggested string, filter *FileFilter) string {
	prompt := osaQuote(title)
	var cmd string
	switch kind {
	case osaSave:
		cmd = "choose file name with prompt " + prompt
		if suggested != "" {
			cmd += " default name " + osaQuote(suggested)
		}
	case osaDir:
		cmd = "choose folder with prompt " + prompt
	default:
		cmd = "choose file with prompt " + prompt
		if types := osaTypeList(filter); types != "" {
			cmd += " of type " + types
		}
	}
	return strings.Join([]string{
		"try",
		"  POSIX path of (" + cmd + ")",
		"on error number -128",
		`  error "cancelled" number 1`,
		"end try",
	}, "\n")
}

func osaQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return `"` + s + `"`
}

func osaTypeList(filter *FileFilter) string {
	if filter == nil || len(filter.Extensions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(filter.Extensions))
	for _, ext := range filter.Extensions {
		ext = strings.TrimPrefix(strings.ToLower(ext), ".")
		if ext == "" || ext == "*" {
			return ""
		}
		parts = append(parts, osaQuote(ext))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func errorsIsCancel(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "cancelled")
}

var execOSA = func(script string) ([]byte, error) {
	cmd := exec.Command("osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	return cmd.Output()
}
