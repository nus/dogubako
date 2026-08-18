package adbfs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Pull copies remote (file or directory) from the device to local.
// Directories are created under local as local/basename(remote) when local
// already exists as a directory; otherwise local is the destination path.
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
			dest = filepath.Join(local, "android-root")
		}
	}
	return pullEntry(ctx, c, serial, st, dest)
}

func pullEntry(ctx context.Context, c Client, serial string, st Entry, dest string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !st.IsDir {
		if err := c.PullFile(ctx, serial, st.Path, dest); err != nil {
			return 0, fmt.Errorf("%s: %w", st.Path, err)
		}
		return 1, nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, err
	}
	children, err := c.List(ctx, serial, st.Path)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", st.Path, err)
	}
	n := 0
	for _, child := range children {
		k, err := pullEntry(ctx, c, serial, child, filepath.Join(dest, child.Name))
		n += k
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Push copies local (file or directory) onto the device at remote.
// If remote exists as a directory, the source basename is created inside it.
func Push(ctx context.Context, c Client, serial, local, remote string) (int, error) {
	remote = Clean(remote)
	info, err := os.Lstat(local)
	if err != nil {
		return 0, err
	}
	dest := remote
	if st, err := c.Stat(ctx, serial, remote); err == nil && st.IsDir {
		dest = Join(remote, filepath.Base(local))
	} else if err == nil && st.IsDir == false && info.IsDir() {
		return 0, fmt.Errorf("cannot copy a folder onto a file: %s", remote)
	}
	return pushWalk(ctx, c, serial, local, dest, info)
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
	return 1, nil
}
