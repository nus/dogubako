package adbfs

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codeskyblue/go-adbkit/adb"
)

const (
	sIFMT  = 0o170000
	sIFDIR = 0o040000
)

func fileTypeDir(mode uint32) bool {
	return mode&sIFMT == sIFDIR
}

func unixTime(sec uint32) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(int64(sec), 0)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type tcpConnector struct {
	host string
	port int
}

func (c tcpConnector) ConnectionContext(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", net.JoinHostPort(c.host, strconv.Itoa(c.port)))
}

type live struct {
	client *adb.Client
}

var (
	defaultOnce sync.Once
	defaultCl   Client
)

// Default returns a process-wide client that talks to the local ADB server.
// It never invokes the adb binary; the server must already be running.
func Default() Client {
	defaultOnce.Do(func() {
		defaultCl = New()
	})
	return defaultCl
}

// New connects to the ADB server on ANDROID_ADB_SERVER_HOST/PORT (defaults
// 127.0.0.1:5037) using the ADB wire protocol only.
func New() Client {
	host := os.Getenv("ANDROID_ADB_SERVER_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 5037
	if s := os.Getenv("ANDROID_ADB_SERVER_PORT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			port = n
		}
	}
	return &live{client: adb.NewClientWithConnector(tcpConnector{host: host, port: port})}
}

func (c *live) device(serial string) *adb.Device {
	return c.client.Device(adb.DeviceWithSerial(serial))
}

func (c *live) Devices(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload, err := c.client.SendHostCommand("host:devices-l")
	if err != nil {
		return nil, fmt.Errorf("adb server: %w", err)
	}
	return parseDevicesL(string(payload)), nil
}

func parseDevicesL(payload string) []Device {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil
	}
	lines := strings.Split(payload, "\n")
	out := make([]Device, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{Serial: fields[0], State: fields[1]}
		for _, f := range fields[2:] {
			key, val, ok := strings.Cut(f, ":")
			if !ok {
				continue
			}
			switch key {
			case "model":
				d.Model = strings.ReplaceAll(val, "_", " ")
			case "product":
				d.Product = val
			}
		}
		out = append(out, d)
	}
	return out
}

func (c *live) Stat(ctx context.Context, serial, p string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	p = Clean(p)
	dev := c.device(serial)
	sync, err := dev.NewSyncService()
	if err != nil {
		return Entry{}, err
	}
	defer sync.Close()
	st, err := sync.Stat(p)
	if err != nil {
		return Entry{}, err
	}
	entries := []Entry{{
		Name:    Base(p),
		Path:    p,
		IsDir:   fileTypeDir(st.Mode),
		Size:    int64(st.Size),
		ModTime: unixTime(st.Time),
	}}
	c.enrichBirthTimes(ctx, dev, entries)
	return entries[0], nil
}

func (c *live) List(ctx context.Context, serial, dir string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir = Clean(dir)
	dev := c.device(serial)
	sync, err := dev.NewSyncService()
	if err != nil {
		return nil, err
	}
	dents, err := sync.Readdir(dir)
	sync.Close()
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(dents))
	for _, d := range dents {
		if d.Name == "" {
			continue
		}
		entries = append(entries, Entry{
			Name:    d.Name,
			Path:    Join(dir, d.Name),
			IsDir:   fileTypeDir(d.Mode),
			Size:    int64(d.Size),
			ModTime: unixTime(d.Mtime),
		})
	}
	sortEntries(entries)
	c.enrichBirthTimes(ctx, dev, entries)
	return entries, nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

func (c *live) enrichBirthTimes(ctx context.Context, dev *adb.Device, entries []Entry) {
	if len(entries) == 0 {
		return
	}
	const batch = 32
	byPath := make(map[string]*Entry, len(entries))
	for i := range entries {
		byPath[entries[i].Path] = &entries[i]
	}
	for start := 0; start < len(entries); start += batch {
		if ctx.Err() != nil {
			return
		}
		end := start + batch
		if end > len(entries) {
			end = len(entries)
		}
		args := make([]string, 0, end-start)
		for _, e := range entries[start:end] {
			args = append(args, shellQuote(e.Path))
		}
		cmd := "stat -c '%n|%W' " + strings.Join(args, " ")
		out, err := dev.RunCommandContext(ctx, cmd)
		if err != nil {
			return
		}
		applyBirthOutput(out, byPath)
	}
}

func applyBirthOutput(out string, byPath map[string]*Entry) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, birth, ok := strings.Cut(line, "|")
		if !ok {
			continue
		}
		e, ok := byPath[name]
		if !ok {
			e, ok = byPath[Clean(name)]
			if !ok {
				continue
			}
		}
		sec, err := strconv.ParseInt(strings.TrimSpace(birth), 10, 64)
		if err != nil || sec <= 0 {
			continue
		}
		e.CrtTime = time.Unix(sec, 0)
	}
}

func (c *live) PullFile(ctx context.Context, serial, remote, local string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remote = Clean(remote)
	dev := c.device(serial)
	sync, err := dev.NewSyncService()
	if err != nil {
		return err
	}
	defer sync.Close()
	r, err := sync.Pull(remote)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return err
	}
	f, err := os.Create(local)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *live) PushFile(ctx context.Context, serial, local, remote string, perm os.FileMode, mtime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remote = Clean(remote)
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()

	mode := uint32(perm) & 0o777
	if mode == 0 {
		mode = 0o644
	}
	opts := adb.SyncPushOptions{Mode: mode}
	if !mtime.IsZero() {
		opts.Mtime = mtime.Unix()
	} else {
		opts.Mtime = time.Now().Unix()
	}

	dev := c.device(serial)
	sync, err := dev.NewSyncService()
	if err != nil {
		return err
	}
	defer sync.Close()
	return sync.Push(f, remote, opts)
}

func (c *live) MkdirAll(ctx context.Context, serial, p string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p = Clean(p)
	if p == "/" {
		return nil
	}
	dev := c.device(serial)
	out, err := dev.RunCommandContext(ctx, "mkdir -p "+shellQuote(p)+" && printf OK")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "OK") {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = "mkdir failed"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
