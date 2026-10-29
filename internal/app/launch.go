package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nus/dogubako/internal/appimage"
	"github.com/nus/dogubako/internal/i18n"
)

// Launch is the tool selected from the command line.
// An empty Tool keeps the tool saved from the last run.
// Path is absolute when git or image was given a path, and empty when it was omitted.
type Launch struct {
	Tool ToolID
	Path string
}

// ParseLaunch reads the arguments after the program name.
// openWindow is false when the process should exit with code and leave the window closed.
// Help is written to stdout. Unknown commands, extra arguments, an unavailable
// mtp tool, and a path of the wrong kind are written to stderr.
// A git path that exists must be a directory. An image path that exists must be a file.
// A missing path still opens the window, and the tool reports it the way the dialog does.
func ParseLaunch(args []string, stdout, stderr io.Writer) (launch Launch, code int, openWindow bool) {
	if len(args) == 0 {
		return Launch{}, 0, true
	}
	switch args[0] {
	case "-h", "--help", "help":
		if len(args) != 1 {
			return usageError(stderr)
		}
		fmt.Fprintln(stdout, cliUsage(i18n.Load()))
		return Launch{}, 0, false
	case string(ToolGit):
		return parseLaunchPath(ToolGit, args, stderr)
	case string(ToolImage):
		return parseLaunchPath(ToolImage, args, stderr)
	case string(ToolScreenshot), string(ToolAndroid), string(ToolAndroidShot), string(ToolStopwatch):
		if len(args) != 1 {
			return usageError(stderr)
		}
		return Launch{Tool: ToolID(args[0])}, 0, true
	case string(ToolMTP):
		if len(args) != 1 {
			return usageError(stderr)
		}
		if !knownTool(ToolMTP) {
			fmt.Fprintln(stderr, i18n.T(i18n.Load(), i18n.CLINoMTP))
			return Launch{}, 1, false
		}
		return Launch{Tool: ToolMTP}, 0, true
	default:
		return usageError(stderr)
	}
}

func parseLaunchPath(tool ToolID, args []string, stderr io.Writer) (Launch, int, bool) {
	if len(args) > 2 {
		return usageError(stderr)
	}
	if len(args) < 2 {
		return Launch{Tool: tool}, 0, true
	}
	if strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, i18n.T(i18n.Load(), i18n.CLINeedPath))
		return Launch{}, 1, false
	}
	path := absLaunchPath(args[1])
	if msg, ok := validateLaunchPath(tool, path, i18n.Load()); !ok {
		fmt.Fprintln(stderr, msg)
		return Launch{}, 1, false
	}
	return Launch{Tool: tool, Path: path}, 0, true
}

// validateLaunchPath checks an existing path against the tool.
// Git takes a directory. Image takes a regular file.
// A path that is not there is accepted, so the tool can show the same status as the dialog.
func validateLaunchPath(tool ToolID, path string, lang i18n.Lang) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true
		}
		return i18n.T(lang, i18n.CLIPathCheck, path, err), false
	}
	switch tool {
	case ToolGit:
		if info.IsDir() {
			return "", true
		}
		return i18n.T(lang, i18n.CLINotDirectory, path), false
	case ToolImage:
		if info.Mode().IsRegular() {
			return "", true
		}
		return i18n.T(lang, i18n.CLINotFile, path), false
	default:
		return "", true
	}
}

func usageError(stderr io.Writer) (Launch, int, bool) {
	fmt.Fprintln(stderr, cliUsage(i18n.Load()))
	return Launch{}, 1, false
}

// absLaunchPath resolves a relative path from the working directory at startup.
// An absolute path is cleaned. A path that cannot be made absolute is returned as given.
func absLaunchPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func cliUsage(lang i18n.Lang) string {
	lines := []string{
		i18n.T(lang, i18n.DesktopUsageHeader),
		"  dogubako",
		"  dogubako git [" + i18n.T(lang, i18n.CLIArgDirectory) + "]",
		"  dogubako image [" + i18n.T(lang, i18n.CLIArgFile) + "]",
		"  dogubako screenshot",
		"  dogubako android",
		"  dogubako android-shot",
		"  dogubako stopwatch",
	}
	if knownTool(ToolMTP) {
		lines = append(lines, "  dogubako mtp")
	}
	for _, line := range appimage.OptionLines() {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "  dogubako --help")
	return strings.Join(lines, "\n")
}

// ApplyLaunch selects the tool the way the sidebar does, and opens a path when one was given.
// Git follows the folder dialog: a directory inside a work tree resolves to the repository root.
// Image follows the file dialog. A path that cannot be opened still leaves that tool selected.
func (m *Model) ApplyLaunch(launch Launch) {
	switch launch.Tool {
	case "":
		return
	case ToolAndroid:
		m.SetAndroidFace(AndroidFaceFiles)
		m.SetMode(ToolAndroid)
	case ToolAndroidShot:
		m.SetMode(ToolAndroidShot)
	case ToolGit:
		m.SetMode(ToolGit)
		if launch.Path != "" {
			m.Git().EnsureLoaded()
			m.Git().Open(launch.Path)
		}
	case ToolImage:
		m.SetMode(ToolImage)
		if launch.Path != "" {
			_ = m.Image().LoadPath(launch.Path)
		}
	default:
		m.SetMode(launch.Tool)
	}
}

// ApplyLaunch selects the command-line tool before the window is shown.
func (r *Root) ApplyLaunch(launch Launch) {
	r.model.ApplyLaunch(launch)
}
