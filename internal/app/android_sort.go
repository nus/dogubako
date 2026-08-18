package app

import (
	"cmp"
	"slices"
	"strings"

	"github.com/nus/dogubako/internal/adbfs"
)

func sortAndroidEntries(entries []adbfs.Entry, col int, desc bool) {
	if len(entries) < 2 {
		return
	}
	slices.SortStableFunc(entries, func(a, b adbfs.Entry) int {
		return compareAndroidEntries(a, b, col, desc)
	})
}

func compareAndroidEntries(a, b adbfs.Entry, col int, desc bool) int {
	if col == androidSortName && a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}
		return 1
	}
	var c int
	switch col {
	case androidSortSize:
		c = cmp.Compare(a.Size, b.Size)
	case androidSortMod:
		c = a.ModTime.Compare(b.ModTime)
	default:
		c = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}
	if desc {
		c = -c
	}
	if c != 0 {
		return c
	}
	if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); n != 0 {
		return n
	}
	return strings.Compare(a.Path, b.Path)
}
