package dialog

import (
	"errors"

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

// OpenFileAsync shows a native open dialog on a new goroutine.
func OpenFileAsync(title string, filter *FileFilter) <-chan FileResult {
	ch := make(chan FileResult, 1)
	go func() {
		if title == "" {
			title = "Open"
		}
		b := nativedialog.File().Title(title)
		if filter != nil {
			b = b.Filter(filter.Description, filter.Extensions...)
		}
		path, err := b.Load()
		ch <- toFileResult(path, err)
	}()
	return ch
}

// SaveFileAsync shows a native save dialog on a new goroutine.
func SaveFileAsync(title, suggested string, filter *FileFilter) <-chan FileResult {
	ch := make(chan FileResult, 1)
	go func() {
		if title == "" {
			title = "Save As"
		}
		b := nativedialog.File().Title(title)
		if suggested != "" {
			b = b.SetStartFile(suggested)
		}
		if filter != nil {
			b = b.Filter(filter.Description, filter.Extensions...)
		}
		path, err := b.Save()
		ch <- toFileResult(path, err)
	}()
	return ch
}

func toFileResult(path string, err error) FileResult {
	if errors.Is(err, nativedialog.ErrCancelled) {
		return FileResult{Cancelled: true}
	}
	return FileResult{Path: path, Err: err}
}
