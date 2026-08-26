package mtpfs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type copyCounts struct {
	files int
	bytes int64
}

func (c copyCounts) add(o copyCounts) copyCounts {
	return copyCounts{files: c.files + o.files, bytes: c.bytes + o.bytes}
}

// Pull copies remote (file or directory) from the device to local.
func Pull(ctx context.Context, c Client, serial, remote, local string) (int, error) {
	remote = Clean(remote)
	st, err := c.Stat(ctx, serial, remote)
	if err != nil {
		return 0, err
	}
	dest := local
	if fi, err := os.Stat(local); err == nil && fi.IsDir() {
		dest = filepath.Join(local, Base(remote))
		if remote == "/" {
			dest = filepath.Join(local, "mtp-root")
		}
	}
	cache := map[string][]Entry{}
	counts, err := countPull(ctx, c, serial, st, cache)
	if err != nil {
		return 0, err
	}
	setCopyTotals(ctx, counts.files, counts.bytes)
	return pullEntry(ctx, c, serial, st, dest, cache)
}

func countPull(ctx context.Context, c Client, serial string, st Entry, cache map[string][]Entry) (copyCounts, error) {
	if err := ctx.Err(); err != nil {
		return copyCounts{}, err
	}
	if !st.IsDir {
		return copyCounts{files: 1, bytes: st.Size}, nil
	}
	children, err := c.List(ctx, serial, st.Path)
	if err != nil {
		return copyCounts{}, fmt.Errorf("%s: %w", st.Path, err)
	}
	cache[st.Path] = children
	var n copyCounts
	for _, child := range children {
		k, err := countPull(ctx, c, serial, child, cache)
		n = n.add(k)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func pullEntry(ctx context.Context, c Client, serial string, st Entry, dest string, cache map[string][]Entry) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !st.IsDir {
		if err := c.PullFile(ctx, serial, st.Path, dest); err != nil {
			return 0, fmt.Errorf("%s: %w", st.Path, err)
		}
		addCopyFile(ctx)
		return 1, nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, err
	}
	children, ok := cache[st.Path]
	if !ok {
		var err error
		children, err = c.List(ctx, serial, st.Path)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", st.Path, err)
		}
	}
	n := 0
	for _, child := range children {
		k, err := pullEntry(ctx, c, serial, child, filepath.Join(dest, child.Name), cache)
		n += k
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Push copies local (file or directory) onto the device at remote.
func Push(ctx context.Context, c Client, serial, local, remote string) (int, error) {
	remote = Clean(remote)
	info, err := os.Lstat(local)
	if err != nil {
		return 0, err
	}
	dest := remote
	if st, err := c.Stat(ctx, serial, remote); err == nil && st.IsDir {
		dest = Join(remote, filepath.Base(local))
	} else if err == nil && !st.IsDir && info.IsDir() {
		return 0, fmt.Errorf("cannot copy a folder onto a file: %s", remote)
	}
	counts, err := countLocalFiles(local, info)
	if err != nil {
		return 0, err
	}
	setCopyTotals(ctx, counts.files, counts.bytes)
	return pushWalk(ctx, c, serial, local, dest, info)
}

func countLocalFiles(local string, info fs.FileInfo) (copyCounts, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return copyCounts{}, nil
	}
	if info.IsDir() {
		entries, err := os.ReadDir(local)
		if err != nil {
			return copyCounts{}, err
		}
		var n copyCounts
		for _, e := range entries {
			child := filepath.Join(local, e.Name())
			fi, err := os.Lstat(child)
			if err != nil {
				return n, err
			}
			k, err := countLocalFiles(child, fi)
			n = n.add(k)
			if err != nil {
				return n, err
			}
		}
		return n, nil
	}
	if info.Mode().IsRegular() {
		return copyCounts{files: 1, bytes: info.Size()}, nil
	}
	return copyCounts{}, nil
}

func pushWalk(ctx context.Context, c Client, serial, local, remote string, info fs.FileInfo) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, nil
	}
	if info.IsDir() {
		if err := c.MkdirAll(ctx, serial, remote); err != nil {
			return 0, fmt.Errorf("%s: %w", remote, err)
		}
		entries, err := os.ReadDir(local)
		if err != nil {
			return 0, err
		}
		n := 0
		for _, e := range entries {
			childLocal := filepath.Join(local, e.Name())
			fi, err := os.Lstat(childLocal)
			if err != nil {
				return n, err
			}
			k, err := pushWalk(ctx, c, serial, childLocal, Join(remote, e.Name()), fi)
			n += k
			if err != nil {
				return n, err
			}
		}
		return n, nil
	}
	if !info.Mode().IsRegular() {
		return 0, nil
	}
	perm := info.Mode().Perm()
	if err := c.PushFile(ctx, serial, local, remote, perm, info.ModTime()); err != nil {
		return 0, fmt.Errorf("%s: %w", remote, err)
	}
	addCopyFile(ctx)
	return 1, nil
}

// closeAndRemoveIncomplete closes f and deletes local when the pull failed,
// so a cancelled or failed copy never leaves a truncated file.
func closeAndRemoveIncomplete(f *os.File, local string, err error) error {
	if f != nil {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		_ = os.Remove(local)
	}
	return err
}
