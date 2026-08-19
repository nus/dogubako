package dialog

import (
	"errors"
	"os/exec"
)

// FileResult is the outcome of an asynchronous file dialog.
type FileResult struct {
	Path      string
	Cancelled bool
	Err       error
}

// FileFilter restricts the files an open dialog offers.
type FileFilter struct {
	Description string
	Extensions  []string
}

// ErrNoFileDialog is returned when neither a native nor an external file dialog is available.
var ErrNoFileDialog = errors.New("file dialog unavailable")

// OpenFileAsync shows a file-open dialog on a new goroutine.
// macOS uses osascript; Ubuntu uses zenity, then kdialog.
func OpenFileAsync(title string, filter *FileFilter) <-chan FileResult {
	ch := make(chan FileResult, 1)
	go func() {
		if title == "" {
			title = "Open"
		}
		ch <- openFileSync(title, filter)
	}()
	return ch
}

// SaveFileAsync shows a file-save dialog on a new goroutine.
func SaveFileAsync(title, suggested string, filter *FileFilter) <-chan FileResult {
	ch := make(chan FileResult, 1)
	go func() {
		if title == "" {
			title = "Save As"
		}
		ch <- saveFileSync(title, suggested, filter)
	}()
	return ch
}

// OpenDirectoryAsync shows a folder-selection dialog on a new goroutine.
func OpenDirectoryAsync(title string) <-chan FileResult {
	ch := make(chan FileResult, 1)
	go func() {
		if title == "" {
			title = "Select Folder"
		}
		ch <- openDirectorySync(title)
	}()
	return ch
}

func openFileSync(title string, filter *FileFilter) FileResult {
	if res, ok := tryOSDialog(osaOpen, title, "", filter); ok {
		return res
	}
	if res, ok := tryExternal(zenityOpenArgs(title, filter)); ok {
		return res
	}
	if res, ok := tryExternal(kdialogOpenArgs(title, filter)); ok {
		return res
	}
	return FileResult{Err: ErrNoFileDialog}
}

func saveFileSync(title, suggested string, filter *FileFilter) FileResult {
	if res, ok := tryOSDialog(osaSave, title, suggested, filter); ok {
		return res
	}
	if res, ok := tryExternal(zenitySaveArgs(title, suggested, filter)); ok {
		return res
	}
	if res, ok := tryExternal(kdialogSaveArgs(title, suggested, filter)); ok {
		return res
	}
	return FileResult{Err: ErrNoFileDialog}
}

func openDirectorySync(title string) FileResult {
	if res, ok := tryOSDialog(osaDir, title, "", nil); ok {
		return res
	}
	if res, ok := tryExternal(zenityDirArgs(title)); ok {
		return res
	}
	if res, ok := tryExternal(kdialogDirArgs(title)); ok {
		return res
	}
	return FileResult{Err: ErrNoFileDialog}
}

func tryExternal(name string, args []string) (FileResult, bool) {
	if _, err := execLookPath(name); err != nil {
		return FileResult{}, false
	}
	out, err := execCommand(name, args...)
	res := resultFromCmd(out, err)
	if res.Err != nil && isCommandMissing(res.Err) {
		return FileResult{}, false
	}
	return res, true
}

func resultFromCmd(out []byte, err error) FileResult {
	if err == nil {
		path := trimPath(string(out))
		if path == "" {
			return FileResult{Cancelled: true}
		}
		return FileResult{Path: path}
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return FileResult{Cancelled: true}
	}
	return FileResult{Err: err}
}

func isCommandMissing(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// execLookPath and execCommand are vars so tests can stub them.
var (
	execLookPath = exec.LookPath
	execCommand  = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)
