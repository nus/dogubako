package dialog

import (
	"errors"
	"fmt"
	"os/exec"

	nativedialog "github.com/hajimehoshi/dialog"
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

var errNoFileDialog = errors.New("ファイルダイアログを開けません。Ubuntu では libgtk-3-0 または zenity をインストールしてください")

// OpenFileAsync shows a file-open dialog on a new goroutine.
// On Ubuntu it uses GTK 3, then zenity, then kdialog.
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
// On Ubuntu it uses GTK 3, then zenity, then kdialog.
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

func openFileSync(title string, filter *FileFilter) FileResult {
	if res, ok := tryNative(func() (string, error) {
		b := nativedialog.File().Title(title)
		if filter != nil {
			b = b.Filter(filter.Description, filter.Extensions...)
		}
		return b.Load()
	}); ok {
		return res
	}
	if res, ok := tryExternal(zenityOpenArgs(title, filter)); ok {
		return res
	}
	if res, ok := tryExternal(kdialogOpenArgs(title, filter)); ok {
		return res
	}
	return FileResult{Err: errNoFileDialog}
}

func saveFileSync(title, suggested string, filter *FileFilter) FileResult {
	if res, ok := tryNative(func() (string, error) {
		b := nativedialog.File().Title(title)
		if suggested != "" {
			b = b.SetStartFile(suggested)
		}
		if filter != nil {
			b = b.Filter(filter.Description, filter.Extensions...)
		}
		return b.Save()
	}); ok {
		return res
	}
	if res, ok := tryExternal(zenitySaveArgs(title, suggested, filter)); ok {
		return res
	}
	if res, ok := tryExternal(kdialogSaveArgs(title, suggested, filter)); ok {
		return res
	}
	return FileResult{Err: errNoFileDialog}
}

func tryNative(fn func() (string, error)) (FileResult, bool) {
	var path string
	var err error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("native dialog: %v", rec)
			}
		}()
		path, err = fn()
	}()
	if err == nil {
		return FileResult{Path: path}, true
	}
	if errors.Is(err, nativedialog.ErrCancelled) {
		return FileResult{Cancelled: true}, true
	}
	// GTK missing or no display: let the caller try zenity/kdialog.
	return FileResult{Err: err}, false
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
