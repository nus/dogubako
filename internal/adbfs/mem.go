package adbfs

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
	mu      sync.Mutex
	Devs    []Device
	DevErr  error
	nodes   map[string]*memNode
	Fail    map[string]error // path -> error for Stat/List/Pull/Push/Mkdir
	Shot    map[string][]byte
	ShotErr error
}

type memNode struct {
	isDir bool
	size  int64
	mod   time.Time
	crt   time.Time
	data  []byte
	perm  os.FileMode
}

// NewMem returns an empty device filesystem. Call AddDevice then MkdirAll/PushFile.
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
	out := append([]Device(nil), m.Devs...)
	return out, nil
}

func (m *Mem) fail(path string) error {
	if m.Fail == nil {
		return nil
	}
	if err := m.Fail[Clean(path)]; err != nil {
		return err
	}
	return nil
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
	prefix := dir
	if prefix != "/" {
		prefix += "/"
	}
	var entries []Entry
	for p, child := range m.nodes {
		if p == dir {
			continue
		}
		parent := Parent(p)
		if parent != dir {
			continue
		}
		if dir == "/" && strings.Count(strings.TrimPrefix(p, "/"), "/") > 0 {
			continue
		}
		_ = prefix
		entries = append(entries, child.entry(p))
	}
	sortEntries(entries)
	return entries, nil
}

func (n *memNode) entry(p string) Entry {
	return Entry{
		Name:    Base(p),
		Path:    p,
		IsDir:   n.isDir,
		Size:    n.size,
		ModTime: n.mod,
		CrtTime: n.crt,
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
	remote = Clean(remote)
	data, err := os.ReadFile(local)
	if err != nil {
		return err
	}
	if mtime.IsZero() {
		mtime = time.Now()
	}
	if perm == 0 {
		perm = 0o644
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
		crt:   mtime,
		data:  append([]byte(nil), data...),
		perm:  perm,
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
	cur := "/"
	if _, ok := m.nodes[cur]; !ok {
		m.nodes[cur] = &memNode{isDir: true, mod: time.Unix(1, 0)}
	}
	if path == "/" {
		return nil
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range parts {
		cur = Join(cur, part)
		if n, ok := m.nodes[cur]; ok {
			if !n.isDir {
				return fmt.Errorf("ENOTDIR: %s", cur)
			}
			continue
		}
		m.nodes[cur] = &memNode{isDir: true, mod: time.Now(), crt: time.Now()}
	}
	return nil
}

// PutDir creates an empty directory.
func (m *Mem) PutDir(path string, mod, crt time.Time) {
	path = Clean(path)
	_ = m.MkdirAll(context.Background(), "", path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := m.nodes[path]; n != nil {
		if !mod.IsZero() {
			n.mod = mod
		}
		if !crt.IsZero() {
			n.crt = crt
		}
	}
}

// PutFile creates a regular file, creating parents as needed.
func (m *Mem) PutFile(path string, data []byte, mod, crt time.Time) {
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
		crt:   crt,
		data:  append([]byte(nil), data...),
		perm:  0o644,
	}
}

func (m *Mem) Screencap(ctx context.Context, serial string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ShotErr != nil {
		return nil, m.ShotErr
	}
	if m.Shot != nil {
		if data, ok := m.Shot[serial]; ok {
			return append([]byte(nil), data...), nil
		}
	}
	return nil, fmt.Errorf("no screenshot")
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
