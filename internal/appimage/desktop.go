// Package appimage installs and removes the application-menu entry for an AppImage.
package appimage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nus/dogubako/internal/i18n"
)

const (
	installFlag   = "--install-desktop"
	uninstallFlag = "--uninstall-desktop"

	desktopFile = "dogubako.desktop"
	iconFile    = "dogubako.png"
)

// OptionLines are the desktop-entry commands. Tool-subcommand help should
// include them; these flags are not tool names.
func OptionLines() []string {
	return []string{
		"dogubako " + installFlag,
		"dogubako " + uninstallFlag,
	}
}

// Usage is the help text for the desktop-entry commands.
func Usage(lang i18n.Lang) string {
	lines := []string{i18n.T(lang, i18n.DesktopUsageHeader)}
	for _, line := range OptionLines() {
		lines = append(lines, "  "+line)
	}
	return strings.Join(lines, "\n")
}

// Dispatch runs --install-desktop or --uninstall-desktop.
//
// handled is false when neither flag is present, including when args is
// empty. The caller then opens the window, and other arguments are left for
// a future tool-subcommand parser. The AppImage runtime removes
// --appimage-extract-and-run before dogubako sees argv, leaving APPIMAGE and
// APPDIR set, so that launch is handled here as a plain install or uninstall.
//
// On success the result is written to stdout and code is 0. On failure the
// reason or the usage text is written to stderr and code is 1. The window is
// not opened in either case.
func Dispatch(args []string, stdout, stderr io.Writer) (code int, handled bool) {
	install, uninstall, extra := classify(args)
	if !install && !uninstall {
		return 0, false
	}
	lang := i18n.Load()
	if (install && uninstall) || extra {
		fmt.Fprintln(stderr, Usage(lang))
		return 1, true
	}
	if install {
		path, err := installDesktop(lang)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1, true
		}
		fmt.Fprintln(stdout, path)
		return 0, true
	}
	removed, err := uninstallDesktop(lang)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1, true
	}
	if removed == 0 {
		fmt.Fprintln(stdout, i18n.T(lang, i18n.DesktopNothing))
		return 0, true
	}
	fmt.Fprintln(stdout, i18n.T(lang, i18n.DesktopRemoved))
	return 0, true
}

func classify(args []string) (install, uninstall, extra bool) {
	for _, arg := range args {
		switch arg {
		case installFlag:
			if install {
				extra = true
			}
			install = true
		case uninstallFlag:
			if uninstall {
				extra = true
			}
			uninstall = true
		default:
			extra = true
		}
	}
	return install, uninstall, extra
}

func installDesktop(lang i18n.Lang) (string, error) {
	appPath, err := appImageFile(lang)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(appPath, "\r\n") {
		return "", errors.New(i18n.T(lang, i18n.DesktopBadPath))
	}
	appdir := strings.TrimSpace(os.Getenv("APPDIR"))
	if appdir == "" {
		return "", errors.New(i18n.T(lang, i18n.DesktopNeedAppDir))
	}
	entryPath := filepath.Join(appdir, desktopFile)
	entry, err := os.ReadFile(entryPath)
	if err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopReadEntry, err))
	}
	icon256, err := os.ReadFile(iconSource(appdir, "256x256"))
	if err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopReadIcon, err))
	}
	icon512, err := os.ReadFile(iconSource(appdir, "512x512"))
	if err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopReadIcon, err))
	}
	root, err := dataHome()
	if err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopWriteFile, err))
	}
	desktopPath, dst256, dst512 := installedPaths(root)
	rewritten := rewriteDesktop(string(entry), quoteExec(appPath), appPath)
	if err := writeFile(desktopPath, []byte(rewritten)); err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopWriteFile, err))
	}
	if err := writeFile(dst256, icon256); err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopWriteFile, err))
	}
	if err := writeFile(dst512, icon512); err != nil {
		return "", errors.New(i18n.T(lang, i18n.DesktopWriteFile, err))
	}
	return desktopPath, nil
}

func uninstallDesktop(lang i18n.Lang) (int, error) {
	if _, err := appImageFile(lang); err != nil {
		return 0, err
	}
	root, err := dataHome()
	if err != nil {
		return 0, errors.New(i18n.T(lang, i18n.DesktopRemoveFile, err))
	}
	desktopPath, dst256, dst512 := installedPaths(root)
	removed := 0
	for _, path := range []string{desktopPath, dst256, dst512} {
		err := os.Remove(path)
		if err == nil {
			removed++
			continue
		}
		if os.IsNotExist(err) {
			continue
		}
		return removed, errors.New(i18n.T(lang, i18n.DesktopRemoveFile, err))
	}
	return removed, nil
}

// appImageFile resolves APPIMAGE to an absolute path of an existing file.
// A relative value is resolved from the current working directory.
func appImageFile(lang i18n.Lang) (string, error) {
	raw := strings.TrimSpace(os.Getenv("APPIMAGE"))
	if raw == "" {
		return "", errors.New(i18n.T(lang, i18n.DesktopNeedAppImage))
	}
	path := raw
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", errors.New(i18n.T(lang, i18n.DesktopAppImageCheck, err))
		}
		path = abs
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New(i18n.T(lang, i18n.DesktopAppImageMissing, path))
		}
		return "", errors.New(i18n.T(lang, i18n.DesktopAppImageCheck, err))
	}
	if !fi.Mode().IsRegular() {
		return "", errors.New(i18n.T(lang, i18n.DesktopAppImageMissing, path))
	}
	return path, nil
}

// dataHome returns XDG_DATA_HOME, or ~/.local/share when it is unset, empty,
// or not an absolute path (the XDG Base Directory rule).
func dataHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func installedPaths(root string) (desktop, icon256, icon512 string) {
	desktop = filepath.Join(root, "applications", desktopFile)
	icon256 = iconDest(root, "256x256")
	icon512 = iconDest(root, "512x512")
	return desktop, icon256, icon512
}

func iconSource(appdir, size string) string {
	return filepath.Join(appdir, "usr", "share", "icons", "hicolor", size, "apps", iconFile)
}

func iconDest(root, size string) string {
	return filepath.Join(root, "icons", "hicolor", size, "apps", iconFile)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return os.Chmod(path, 0o644)
}

// rewriteDesktop keeps every field from the AppImage's dogubako.desktop and
// sets Exec and TryExec to the absolute AppImage path. Exec is one quoted
// desktop-entry argument. TryExec is that path alone, not a command line.
func rewriteDesktop(content, execValue, tryValue string) string {
	lines := splitLines(content)
	execAt := -1
	hasTry := false
	for i, line := range lines {
		switch desktopKey(line) {
		case "Exec":
			if execAt < 0 {
				execAt = i
			}
			lines[i] = "Exec=" + execValue
		case "TryExec":
			hasTry = true
			lines[i] = "TryExec=" + tryValue
		}
	}
	if execAt < 0 {
		lines = append(lines, "Exec="+execValue)
		execAt = len(lines) - 1
	}
	if !hasTry {
		lines = insertLine(lines, execAt+1, "TryExec="+tryValue)
	}
	return strings.Join(lines, "\n") + "\n"
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") && len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func desktopKey(line string) string {
	line = strings.TrimRight(line, "\r")
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return ""
	}
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(key)
}

func insertLine(lines []string, at int, line string) []string {
	if at >= len(lines) {
		return append(lines, line)
	}
	lines = append(lines, "")
	copy(lines[at+1:], lines[at:])
	lines[at] = line
	return lines
}

// quoteExec quotes one Exec argument using desktop-entry rules.
// The whole path is double-quoted, a literal % is written as %%, and
// backslash, quote, and the escaped whitespace sequences are escaped.
func quoteExec(path string) string {
	var b strings.Builder
	b.Grow(len(path) + 2)
	b.WriteByte('"')
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '%':
			b.WriteString(`%%`)
		default:
			b.WriteByte(path[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}
