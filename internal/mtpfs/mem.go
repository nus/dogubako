package mtpfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Mem is an in-memory Client for tests.
type Mem struct {
	mu     sync.Mutex
	Devs   []Device
	DevErr error
	nodes  map[string]*memNode
	Fail   map[string]error
}

type memNode struct {
	isDir bool
	size  int64
	mod   time.Time
	data  []byte
}

// NewMem returns an empty device filesystem.
func NewMem(devs ...Device) *Mem {
	m := &Mem{
		Devs:  append([]Device(nil), devs...),
		nodes: map[string]*memNode{"/": {isDir: true, mod: time.Unix(1, 0)}},
		Fail:  map[string]error{},
	}
	return m
}

func (m *Mem) Devices(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DevErr != nil {
		return nil, m.DevErr
	}
	return append([]Device(nil), m.Devs...), nil
}

func (m *Mem) fail(path string) error {
	if m.Fail == nil {
		return nil
	}
	return m.Fail[Clean(path)]
}

func (m *Mem) Stat(ctx context.Context, serial, path string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	_ = serial
	path = Clean(path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(path); err != nil {
		return Entry{}, err
	}
	n, ok := m.nodes[path]
	if !ok {
		return Entry{}, fmt.Errorf("ENOENT: %s", path)
	}
	return n.entry(path), nil
}

func (m *Mem) List(ctx context.Context, serial, dir string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = serial
	dir = Clean(dir)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(dir); err != nil {
		return nil, err
	}
	n, ok := m.nodes[dir]
	if !ok {
		return nil, fmt.Errorf("ENOENT: %s", dir)
	}
	if !n.isDir {
		return nil, fmt.Errorf("ENOTDIR: %s", dir)
	}
	var entries []Entry
	for p, child := range m.nodes {
		if p == dir {
			continue
		}
		if Parent(p) != dir {
			continue
		}
		entries = append(entries, child.entry(p))
	}
	reportListProgress(ctx, len(entries), len(entries), entries)
	return entries, nil
}

func (n *memNode) entry(p string) Entry {
	return Entry{
		Name:    Base(p),
		Path:    p,
		IsDir:   n.isDir,
		Size:    n.size,
		ModTime: n.mod,
	}
}

func (m *Mem) PullFile(ctx context.Context, serial, remote, local string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = serial
	remote = Clean(remote)
	m.mu.Lock()
	n, ok := m.nodes[remote]
	errFail := m.fail(remote)
	m.mu.Unlock()
	if errFail != nil {
		return errFail
	}
	if !ok {
		return fmt.Errorf("ENOENT: %s", remote)
	}
	if n.isDir {
		return fmt.Errorf("EISDIR: %s", remote)
	}
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return err
	}
	return os.WriteFile(local, n.data, 0o644)
}

func (m *Mem) PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = serial
	_ = perm
	remote = Clean(remote)
	data, err := os.ReadFile(local)
	if err != nil {
		return err
	}
	if mtime.IsZero() {
		mtime = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(remote); err != nil {
		return err
	}
	parent := Parent(remote)
	if pn, ok := m.nodes[parent]; !ok || !pn.isDir {
		return fmt.Errorf("ENOENT: %s", parent)
	}
	m.nodes[remote] = &memNode{
		isDir: false,
		size:  int64(len(data)),
		mod:   mtime,
		data:  append([]byte(nil), data...),
	}
	return nil
}

func (m *Mem) MkdirAll(ctx context.Context, serial, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = serial
	path = Clean(path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail(path); err != nil {
		return err
	}
	if _, ok := m.nodes["/"]; !ok {
		m.nodes["/"] = &memNode{isDir: true, mod: time.Unix(1, 0)}
	}
	if path == "/" {
		return nil
	}
	cur := "/"
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		cur = Join(cur, part)
		if n, ok := m.nodes[cur]; ok {
			if !n.isDir {
				return fmt.Errorf("ENOTDIR: %s", cur)
			}
			continue
		}
		m.nodes[cur] = &memNode{isDir: true, mod: time.Now()}
	}
	return nil
}

// PutDir creates an empty directory.
func (m *Mem) PutDir(path string, mod time.Time) {
	path = Clean(path)
	_ = m.MkdirAll(context.Background(), "", path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := m.nodes[path]; n != nil && !mod.IsZero() {
		n.mod = mod
	}
}

// PutFile creates a regular file, creating parents as needed.
func (m *Mem) PutFile(path string, data []byte, mod time.Time) {
	path = Clean(path)
	_ = m.MkdirAll(context.Background(), "", Parent(path))
	if mod.IsZero() {
		mod = time.Unix(10, 0)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[path] = &memNode{
		isDir: false,
		size:  int64(len(data)),
		mod:   mod,
		data:  append([]byte(nil), data...),
	}
}

// FileData returns the stored bytes for path.
func (m *Mem) FileData(path string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[Clean(path)]
	if !ok || n.isDir {
		return nil, false
	}
	return append([]byte(nil), n.data...), true
}
