package mtpfs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nus/dogubako/internal/usbhost"
)

var (
	_ Client = (*live)(nil)
	_ Client = (*Mem)(nil)
)

type live struct {
	mu       sync.Mutex
	sessions map[string]*openDev
	listFn   func() ([]usbhost.Info, error)
	openFn   func(id string) (usbhost.Conn, error)
}

type openDev struct {
	mu    sync.Mutex
	info  usbhost.Info
	sess  *session
	nodes map[string]node
}

type node struct {
	handle    uint32
	storageID uint32
	storage   bool
	isDir     bool
	size      int64
	mod       time.Time
	name      string
}

func newLive() *live {
	return &live{sessions: map[string]*openDev{}}
}

// Default returns the USB MTP client. On non-macOS hosts, Devices reports ErrUnsupported.
func Default() Client {
	return newLive()
}

func (c *live) listUSB() ([]usbhost.Info, error) {
	if c.listFn != nil {
		return c.listFn()
	}
	return usbhost.List()
}

func (c *live) openUSB(id string) (usbhost.Conn, error) {
	if c.openFn != nil {
		return c.openFn(id)
	}
	return usbhost.Open(id)
}

func (c *live) Devices(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	infos, err := c.listUSB()
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{}
	out := make([]Device, 0, len(infos))
	for _, info := range infos {
		keep[info.ID] = true
		out = append(out, Device{
			Serial:  info.ID,
			State:   "online",
			Model:   info.Product,
			Product: info.Product,
		})
	}

	c.mu.Lock()
	var stale []*openDev
	for id, d := range c.sessions {
		if !keep[id] {
			stale = append(stale, d)
			delete(c.sessions, id)
		}
	}
	c.mu.Unlock()
	for _, d := range stale {
		d.mu.Lock()
		d.sess.close()
		d.mu.Unlock()
	}
	return out, nil
}

func (c *live) device(ctx context.Context, serial string) (*openDev, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if d, ok := c.sessions[serial]; ok && d.sess != nil && !d.sess.broken {
		c.mu.Unlock()
		return d, nil
	}
	c.mu.Unlock()

	conn, err := c.openUSB(serial)
	if err != nil {
		return nil, err
	}
	sess := &session{t: conn}
	if err := sess.open(ctx); err != nil {
		sess.close()
		return nil, err
	}
	d := &openDev{
		info:  conn.Info(),
		sess:  sess,
		nodes: map[string]node{"/": {storage: true, isDir: true, name: "/"}},
	}
	c.mu.Lock()
	if existing, ok := c.sessions[serial]; ok && existing.sess != nil && !existing.sess.broken {
		c.mu.Unlock()
		sess.close()
		return existing, nil
	}
	old := c.sessions[serial]
	c.sessions[serial] = d
	c.mu.Unlock()
	if old != nil && old != d {
		old.mu.Lock()
		old.sess.close()
		old.sess = nil
		old.mu.Unlock()
	}
	return d, nil
}

func sessionBroken(s *session) bool {
	return s != nil && s.broken
}

func (c *live) invalidate(serial string, d *openDev) {
	if d.sess != nil {
		d.sess.close()
		d.sess = nil
	}
	c.mu.Lock()
	if c.sessions[serial] == d {
		delete(c.sessions, serial)
	}
	c.mu.Unlock()
}

func doWith[T any](c *live, ctx context.Context, serial string, retry bool, fn func(*openDev) (T, error)) (T, error) {
	var zero T
	var last T
	var lastErr error
	attempts := 1
	if retry {
		attempts = 2
	}
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		d, err := c.device(ctx, serial)
		if err != nil {
			return zero, err
		}
		d.mu.Lock()
		if d.sess == nil || d.sess.broken {
			c.invalidate(serial, d)
			d.mu.Unlock()
			lastErr = fmt.Errorf("MTP session closed")
			continue
		}
		last, err = fn(d)
		broken := sessionBroken(d.sess)
		if broken {
			c.invalidate(serial, d)
		}
		d.mu.Unlock()
		if err == nil {
			return last, nil
		}
		lastErr = err
		if !broken {
			return last, err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("MTP session closed")
	}
	return last, retryExhausted(lastErr)
}

func doVoid(c *live, ctx context.Context, serial string, retry bool, fn func(*openDev) error) error {
	_, err := doWith(c, ctx, serial, retry, func(d *openDev) (struct{}, error) {
		return struct{}{}, fn(d)
	})
	return err
}

func (c *live) Stat(ctx context.Context, serial, path string) (Entry, error) {
	return doWith(c, ctx, serial, true, func(d *openDev) (Entry, error) {
		path = Clean(path)
		if path == "/" {
			return Entry{Name: "/", Path: "/", IsDir: true}, nil
		}
		if n, ok := d.nodes[path]; ok {
			return n.entry(path), nil
		}
		if _, err := d.listPath(ctx, Parent(path)); err != nil {
			return Entry{}, err
		}
		n, ok := d.nodes[path]
		if !ok {
			return Entry{}, fmt.Errorf("ENOENT: %s", path)
		}
		return n.entry(path), nil
	})
}

func (c *live) List(ctx context.Context, serial, dir string) ([]Entry, error) {
	return doWith(c, ctx, serial, true, func(d *openDev) ([]Entry, error) {
		return d.listPath(ctx, Clean(dir))
	})
}

func (d *openDev) listPath(ctx context.Context, dir string) ([]Entry, error) {
	dir = Clean(dir)
	if dir == "/" {
		return d.listStorages(ctx)
	}
	n, ok := d.nodes[dir]
	if !ok {
		if _, err := d.listPath(ctx, Parent(dir)); err != nil {
			return nil, err
		}
		n, ok = d.nodes[dir]
		if !ok {
			return nil, fmt.Errorf("ENOENT: %s", dir)
		}
	}
	if !n.isDir {
		return nil, fmt.Errorf("ENOTDIR: %s", dir)
	}
	parentHandle := n.handle
	if n.storage {
		parentHandle = handleRoot
	}
	if objs, err := d.sess.objectPropList(ctx, parentHandle); err == nil {
		entries := d.entriesFromProps(dir, n, parentHandle, objs)
		if len(entries) > 0 {
			reportListProgress(ctx, len(entries), len(entries), entries)
			return entries, nil
		}
		handles, herr := d.sess.objectHandles(ctx, n.storageID, parentHandle)
		if herr != nil || len(handles) == 0 {
			reportListProgress(ctx, 0, 0, entries)
			return entries, nil
		}
		return d.entriesFromInfos(ctx, dir, n, handles)
	} else if !isPropListUnsupported(err) {
		return nil, err
	}
	handles, err := d.sess.objectHandles(ctx, n.storageID, parentHandle)
	if err != nil {
		return nil, err
	}
	return d.entriesFromInfos(ctx, dir, n, handles)
}

func (d *openDev) entriesFromProps(dir string, n node, parentHandle uint32, objs []propObject) []Entry {
	entries := make([]Entry, 0, len(objs))
	used := map[string]int{}
	for _, p := range objs {
		if !isPropChild(p, parentHandle, n.storage) {
			continue
		}
		name := p.name
		if name == "" {
			name = fmt.Sprintf("object-%08x", p.handle)
		}
		name = uniqueName(used, name)
		path := Join(dir, name)
		isDir := p.hasFormat && p.format == fmtAssociation
		size := int64(0)
		if p.hasSize && p.size <= uint64(^uint64(0)>>1) {
			size = int64(p.size)
		}
		mod := parseModTime(p.modDate)
		d.nodes[path] = node{
			handle:    p.handle,
			storageID: n.storageID,
			isDir:     isDir,
			size:      size,
			mod:       mod,
			name:      name,
		}
		entries = append(entries, d.nodes[path].entry(path))
	}
	return entries
}

func (d *openDev) entriesFromInfos(ctx context.Context, dir string, n node, handles []uint32) ([]Entry, error) {
	entries := make([]Entry, 0, len(handles))
	used := map[string]int{}
	total := len(handles)
	reportListProgress(ctx, 0, total, entries)
	for i, h := range handles {
		if err := ctx.Err(); err != nil {
			reportListProgress(ctx, i, total, entries)
			return entries, err
		}
		info, err := d.sess.objectInfo(ctx, h)
		if err != nil {
			if d.sess.broken || !isAnyResponse(err) {
				return entries, err
			}
			continue
		}
		name := info.filename
		if name == "" {
			name = fmt.Sprintf("object-%08x", h)
		}
		name = uniqueName(used, name)
		p := Join(dir, name)
		isDir := info.format == fmtAssociation
		size := int64(info.size)
		if !isDir && info.size == objectSizeMax32 {
			size = int64(objectSizeMax32)
		}
		mod := parseModTime(info.modDate)
		d.nodes[p] = node{
			handle:    h,
			storageID: n.storageID,
			isDir:     isDir,
			size:      size,
			mod:       mod,
			name:      name,
		}
		entries = append(entries, d.nodes[p].entry(p))
		if (i+1)%32 == 0 || i+1 == total {
			reportListProgress(ctx, i+1, total, entries)
		}
	}
	return entries, nil
}

func (d *openDev) listStorages(ctx context.Context) ([]Entry, error) {
	ids, err := d.sess.storageIDs(ctx)
	if err != nil {
		return nil, err
	}
	used := map[string]int{}
	var entries []Entry
	for _, id := range ids {
		if id == 0 || id&0xffff == 0 {
			continue
		}
		info, err := d.sess.storageInfo(ctx, id)
		if err != nil {
			if d.sess.broken || !isAnyResponse(err) {
				return nil, err
			}
			continue
		}
		name := info.description
		if name == "" {
			name = info.volume
		}
		if name == "" {
			name = fmt.Sprintf("Storage-%08x", id)
		}
		name = uniqueName(used, name)
		p := Join("/", name)
		d.nodes[p] = node{
			handle:    0,
			storageID: id,
			storage:   true,
			isDir:     true,
			name:      name,
		}
		entries = append(entries, d.nodes[p].entry(p))
	}
	return entries, nil
}

func (n node) entry(path string) Entry {
	return Entry{
		Name:    n.name,
		Path:    path,
		IsDir:   n.isDir || n.storage,
		Size:    n.size,
		ModTime: n.mod,
	}
}

func uniqueName(used map[string]int, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	base := name
	for {
		if used[name] == 0 {
			used[name] = 1
			return name
		}
		used[base]++
		name = fmt.Sprintf("%s (%d)", base, used[base])
	}
}

func parseModTime(s string) time.Time {
	y, mo, d, h, mi, se, ok := parseMTPDate(s)
	if !ok {
		return time.Time{}
	}
	return time.Date(y, time.Month(mo), d, h, mi, se, 0, time.Local)
}

func (c *live) PullFile(ctx context.Context, serial, remote, local string) error {
	st, err := c.Stat(ctx, serial, remote)
	if err != nil {
		return err
	}
	if st.IsDir {
		return fmt.Errorf("EISDIR: %s", remote)
	}
	if err := os.MkdirAll(parentDir(local), 0o755); err != nil {
		return err
	}
	return doVoid(c, ctx, serial, true, func(d *openDev) error {
		n, ok := d.nodes[Clean(remote)]
		if !ok {
			if _, err := d.listPath(ctx, Parent(Clean(remote))); err != nil {
				return err
			}
			n, ok = d.nodes[Clean(remote)]
			if !ok {
				return fmt.Errorf("ENOENT: %s", remote)
			}
		}
		f, err := os.Create(local)
		if err != nil {
			return err
		}
		_, err = d.sess.getObjectTo(ctx, n.handle, n.size, progressWriter{ctx: ctx, w: f})
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(local)
			return err
		}
		return nil
	})
}

func (c *live) PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error {
	_ = perm
	_ = mtime
	st, err := os.Stat(local)
	if err != nil {
		return err
	}
	remote = Clean(remote)
	parent := Parent(remote)
	name := Base(remote)
	return doVoid(c, ctx, serial, false, func(d *openDev) error {
		pn, err := d.ensureDir(ctx, parent)
		if err != nil {
			return err
		}
		parentHandle := pn.handle
		if pn.storage {
			parentHandle = handleRoot
		}
		return d.sess.sendObjectFile(ctx, pn.storageID, parentHandle, name, local, st.Size())
	})
}

func (c *live) MkdirAll(ctx context.Context, serial, path string) error {
	return doVoid(c, ctx, serial, false, func(d *openDev) error {
		_, err := d.ensureDir(ctx, Clean(path))
		return err
	})
}

func (d *openDev) ensureDir(ctx context.Context, path string) (node, error) {
	path = Clean(path)
	if path == "/" {
		return node{}, fmt.Errorf("cannot create objects at device root; choose a storage")
	}
	if n, ok := d.nodes[path]; ok && n.isDir {
		return n, nil
	}
	parent := Parent(path)
	if parent == "/" {
		if _, err := d.listPath(ctx, "/"); err != nil {
			return node{}, err
		}
		n, ok := d.nodes[path]
		if !ok {
			return node{}, fmt.Errorf("ENOENT: %s", path)
		}
		return n, nil
	}
	pn, err := d.ensureDir(ctx, parent)
	if err != nil {
		return node{}, err
	}
	if _, err := d.listPath(ctx, parent); err != nil {
		return node{}, err
	}
	if n, ok := d.nodes[path]; ok {
		if !n.isDir {
			return node{}, fmt.Errorf("ENOTDIR: %s", path)
		}
		return n, nil
	}
	parentHandle := pn.handle
	if pn.storage {
		parentHandle = handleRoot
	}
	h, err := d.sess.createFolder(ctx, pn.storageID, parentHandle, Base(path))
	if err != nil {
		return node{}, err
	}
	n := node{
		handle:    h,
		storageID: pn.storageID,
		isDir:     true,
		name:      Base(path),
	}
	d.nodes[path] = n
	return n, nil
}

func parentDir(p string) string {
	i := strings.LastIndex(p, string(os.PathSeparator))
	if i <= 0 {
		return "."
	}
	return p[:i]
}
