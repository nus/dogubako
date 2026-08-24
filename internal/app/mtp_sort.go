package app

import (
	"cmp"
	"slices"
	"strings"

	"github.com/nus/dogubako/internal/mtpfs"
)

func sortMTPEntries(entries []mtpfs.Entry, col int, desc bool) {
	if len(entries) < 2 {
		return
	}
	slices.SortStableFunc(entries, func(a, b mtpfs.Entry) int {
		return compareMTPEntries(a, b, col, desc)
	})
}

func compareMTPEntries(a, b mtpfs.Entry, col int, desc bool) int {
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
