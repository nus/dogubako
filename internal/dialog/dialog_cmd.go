package dialog

import (
	"path/filepath"
	"strings"
)

func zenityOpenArgs(title string, filter *FileFilter) (string, []string) {
	args := []string{"--file-selection", "--title=" + title}
	if filter != nil {
		args = append(args, "--file-filter="+zenityFilter(filter))
	}
	return "zenity", args
}

func zenitySaveArgs(title, suggested string, filter *FileFilter) (string, []string) {
	args := []string{"--file-selection", "--save", "--confirm-overwrite", "--title=" + title}
	if suggested != "" {
		args = append(args, "--filename="+suggested)
	}
	if filter != nil {
		args = append(args, "--file-filter="+zenityFilter(filter))
	}
	return "zenity", args
}

func kdialogOpenArgs(title string, filter *FileFilter) (string, []string) {
	args := []string{"--getopenfilename", ".", kdialogFilter(filter), "--title", title}
	return "kdialog", args
}

func kdialogSaveArgs(title, suggested string, filter *FileFilter) (string, []string) {
	start := suggested
	if start == "" {
		start = "."
	}
	args := []string{"--getsavefilename", start, kdialogFilter(filter), "--title", title}
	return "kdialog", args
}

func zenityDirArgs(title string) (string, []string) {
	return "zenity", []string{"--file-selection", "--directory", "--title=" + title}
}

func kdialogDirArgs(title string) (string, []string) {
	return "kdialog", []string{"--getexistingdirectory", ".", "--title", title}
}

func zenityFilter(filter *FileFilter) string {
	if filter == nil || len(filter.Extensions) == 0 {
		return "All files | *"
	}
	patterns := make([]string, 0, len(filter.Extensions))
	for _, ext := range filter.Extensions {
		ext = strings.TrimPrefix(ext, ".")
		if ext == "" {
			continue
		}
		patterns = append(patterns, "*."+ext)
	}
	name := filter.Description
	if name == "" {
		name = "Files"
	}
	return name + " | " + strings.Join(patterns, " ")
}

func kdialogFilter(filter *FileFilter) string {
	if filter == nil || len(filter.Extensions) == 0 {
		return "*"
	}
	patterns := make([]string, 0, len(filter.Extensions))
	for _, ext := range filter.Extensions {
		ext = strings.TrimPrefix(ext, ".")
		if ext == "" {
			continue
		}
		patterns = append(patterns, "*."+ext)
	}
	name := filter.Description
	if name == "" {
		name = "Files"
	}
	return name + " (" + strings.Join(patterns, " ") + ")"
}

func trimPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "file://")
	return filepath.Clean(s)
}
