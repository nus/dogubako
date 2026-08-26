package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/mtpfs"
)

func waitMTP(t *testing.T, m *MTPModel) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.Drain()
		if !m.Busy() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for mtp model")
}

func TestMTPModelListsTreeAndCopy(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := mtpfs.NewMem(mtpfs.Device{Serial: "pixel", State: "online", Model: "Pixel 7"})
	fs.PutDir("/Internal/DCIM", now)
	fs.PutFile("/Internal/DCIM/a.txt", []byte("hello"), now)
	fs.PutFile("/Internal/note.txt", []byte("n"), now)

	var m MTPModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitMTP(t, &m)

	if m.Serial() != "pixel" {
		t.Fatalf("serial = %q", m.Serial())
	}
	if m.Root() != "/" {
		t.Fatalf("root = %q", m.Root())
	}
	rows := m.Rows()
	if len(rows) != 1 || rows[0].Entry.Name != "Internal" || !rows[0].Entry.IsDir {
		t.Fatalf("rows = %+v", rows)
	}

	m.ToggleExpand("/Internal")
	waitMTP(t, &m)
	rows = m.Rows()
	if len(rows) != 3 {
		t.Fatalf("expanded rows = %d %+v", len(rows), rows)
	}

	m.ToggleExpand("/Internal/DCIM")
	waitMTP(t, &m)
	m.SelectPath("/Internal/DCIM/a.txt")
	dir := t.TempDir()
	m.StartPull(dir)
	waitMTP(t, &m)
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("pulled = %q", got)
	}

	src := filepath.Join(t.TempDir(), "frompc.txt")
	if err := os.WriteFile(src, []byte("pc"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.SelectPath("/Internal")
	m.StartPush(src)
	waitMTP(t, &m)
	data, ok := fs.FileData("/Internal/frompc.txt")
	if !ok || string(data) != "pc" {
		t.Fatalf("pushed = %q ok=%v", data, ok)
	}
}

func TestMTPModelConnectError(t *testing.T) {
	fs := mtpfs.NewMem()
	fs.DevErr = context.DeadlineExceeded
	var m MTPModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitMTP(t, &m)
	if got := m.StatusText(i18n.JA); got == "" {
		t.Fatal("expected connect error")
	}
}

func TestMTPModelSortsByColumn(t *testing.T) {
	old := time.Unix(1_700_000_000, 0)
	mid := old.Add(time.Hour)
	neu := old.Add(2 * time.Hour)
	fs := mtpfs.NewMem(mtpfs.Device{Serial: "pixel", State: "online", Model: "Pixel 7"})
	fs.PutDir("/Internal/DCIM", mid)
	fs.PutFile("/Internal/note.txt", []byte("n"), neu)
	fs.PutFile("/Internal/big.bin", make([]byte, 2048), old)

	var m MTPModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitMTP(t, &m)
	m.ToggleExpand("/Internal")
	waitMTP(t, &m)

	names := func() []string {
		var out []string
		for _, r := range m.Rows() {
			if r.Depth == 1 {
				out = append(out, r.Entry.Name)
			}
		}
		return out
	}

	if got := names(); len(got) != 3 || got[0] != "DCIM" || got[1] != "big.bin" || got[2] != "note.txt" {
		t.Fatalf("default name asc = %v", got)
	}

	m.ToggleSort(androidSortName)
	if got := names(); got[0] != "DCIM" || got[1] != "note.txt" || got[2] != "big.bin" {
		t.Fatalf("name desc = %v", got)
	}
}

func TestListPercent(t *testing.T) {
	if got := listPercent(0, 0); got != 0 {
		t.Fatalf("unknown total = %d", got)
	}
	if got := listPercent(2, 8); got != 25 {
		t.Fatalf("2/8 = %d", got)
	}
	if got := listPercent(8, 8); got != 100 {
		t.Fatalf("8/8 = %d", got)
	}
	if got := listPercent(9, 8); got != 100 {
		t.Fatalf("over 100 = %d", got)
	}
}

func TestMTPModelListingProgressPercent(t *testing.T) {
	var m MTPModel
	ch := make(chan mtpListResult, 2)
	m.pendingList = ch
	m.children = map[string][]mtpfs.Entry{}

	ch <- mtpListResult{
		path:    "/",
		loaded:  2,
		total:   8,
		entries: []mtpfs.Entry{{Name: "a", Path: "/a"}},
	}
	m.Drain()
	if !m.Loading() {
		t.Fatal("expected loading")
	}
	if got := m.ListPercent(); got != 25 {
		t.Fatalf("percent = %d", got)
	}
	got := m.StatusText(i18n.JA)
	if got != "読み込んでいます… 25%（2 / 8 件）" {
		t.Fatalf("ja status = %q", got)
	}
	if en := m.StatusText(i18n.EN); en != "Loading… 25% (2 / 8)" {
		t.Fatalf("en status = %q", en)
	}

	ch <- mtpListResult{
		path:    "/",
		loaded:  8,
		total:   8,
		entries: []mtpfs.Entry{{Name: "a", Path: "/a"}, {Name: "b", Path: "/b"}},
		done:    true,
	}
	m.Drain()
	if m.Loading() {
		t.Fatal("listing should be done")
	}
	if got := m.ListPercent(); got != 0 {
		t.Fatalf("percent after done = %d", got)
	}
}

func TestMTPModelCopyProgressPercent(t *testing.T) {
	var m MTPModel
	ch := make(chan mtpCopyResult, 2)
	m.pendingCopy = ch

	ch <- mtpCopyResult{copied: 1, total: 4}
	m.Drain()
	if !m.Copying() {
		t.Fatal("expected copying")
	}
	if got := m.ProgressPercent(); got != 25 {
		t.Fatalf("percent = %d", got)
	}
	if got := m.StatusText(i18n.JA); got != "コピーしています… 25%（1 / 4 件）" {
		t.Fatalf("ja status = %q", got)
	}
	if got := m.StatusText(i18n.EN); got != "Copying… 25% (1 / 4)" {
		t.Fatalf("en status = %q", got)
	}

	ch <- mtpCopyResult{n: 4, dest: "/tmp", done: true}
	m.Drain()
	if m.Copying() {
		t.Fatal("copy should be done")
	}
	if got := m.ProgressPercent(); got != 0 {
		t.Fatalf("percent after done = %d", got)
	}
	if got := m.StatusText(i18n.JA); got != "コピーしました（4 件）: /tmp" {
		t.Fatalf("ja done = %q", got)
	}
}

func TestMTPModelCancelCopy(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := mtpfs.NewMem(mtpfs.Device{Serial: "pixel", State: "online", Model: "Pixel 7"})
	fs.PutFile("/Internal/a.txt", []byte("hello"), now)
	stall := &stallPullClient{Mem: fs, started: make(chan struct{})}

	var m MTPModel
	m.SetClient(stall)
	m.RefreshDevices()
	waitMTP(t, &m)
	m.ToggleExpand("/Internal")
	waitMTP(t, &m)
	m.SelectPath("/Internal/a.txt")
	m.StartPull(t.TempDir())

	select {
	case <-stall.started:
	case <-time.After(2 * time.Second):
		t.Fatal("copy did not start")
	}
	if !m.Copying() {
		t.Fatal("expected copying")
	}
	m.CancelCopy()
	waitMTP(t, &m)
	if m.Copying() {
		t.Fatal("copy should be cancelled")
	}
	if got := m.StatusText(i18n.JA); got != "コピーをキャンセルしました" {
		t.Fatalf("ja status = %q", got)
	}
	if got := m.StatusText(i18n.EN); got != "Copy cancelled" {
		t.Fatalf("en status = %q", got)
	}
	if _, _, ok := m.TakeRetryAlert(); ok {
		t.Fatal("cancel should not alert")
	}
}

func TestMTPModelCancelCopyIdle(t *testing.T) {
	var m MTPModel
	m.CancelCopy()
}

type stallPullClient struct {
	*mtpfs.Mem
	started chan struct{}
}

func (s *stallPullClient) PullFile(ctx context.Context, serial, remote, local string) error {
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestMTPModelCopyProgressSingleFileBytes(t *testing.T) {
	var m MTPModel
	ch := make(chan mtpCopyResult, 3)
	m.pendingCopy = ch

	ch <- mtpCopyResult{copied: 0, total: 1, copiedBytes: 0, totalBytes: 100}
	m.Drain()
	if got := m.ProgressPercent(); got != 0 {
		t.Fatalf("start percent = %d", got)
	}
	if got := m.StatusText(i18n.JA); got != "コピーしています… 0%" {
		t.Fatalf("ja start = %q", got)
	}

	ch <- mtpCopyResult{copied: 0, total: 1, copiedBytes: 42, totalBytes: 100}
	m.Drain()
	if got := m.ProgressPercent(); got != 42 {
		t.Fatalf("percent = %d", got)
	}
	if got := m.StatusText(i18n.JA); got != "コピーしています… 42%" {
		t.Fatalf("ja status = %q", got)
	}
	if got := m.StatusText(i18n.EN); got != "Copying… 42%" {
		t.Fatalf("en status = %q", got)
	}
}

func TestMTPModelRetryExhaustedQueuesAlert(t *testing.T) {
	fs := mtpfs.NewMem(mtpfs.Device{Serial: "pixel", State: "online", Model: "Pixel 7"})
	fs.Fail["/"] = mtpfs.RetryExhausted(context.DeadlineExceeded)

	var m MTPModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitMTP(t, &m)

	key, args, ok := m.TakeRetryAlert()
	if !ok {
		t.Fatal("expected a message-box alert after retry was exhausted")
	}
	if key != i18n.StatusMTPListFailed {
		t.Fatalf("alert key = %s", key)
	}
	if len(args) != 1 {
		t.Fatalf("alert args = %v", args)
	}
	if got := m.StatusText(i18n.JA); got == "" {
		t.Fatal("status bar should still show the error")
	}

	key, _, ok = m.TakeRetryAlert()
	if ok {
		t.Fatalf("alert should be consumed: %s", key)
	}
}

func TestMTPModelListErrorStaysOnStatusBar(t *testing.T) {
	fs := mtpfs.NewMem(mtpfs.Device{Serial: "pixel", State: "online", Model: "Pixel 7"})
	fs.Fail["/"] = context.Canceled

	var m MTPModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitMTP(t, &m)

	if _, _, ok := m.TakeRetryAlert(); ok {
		t.Fatal("plain list errors should stay on the status bar")
	}
	if got := m.StatusText(i18n.JA); got == "" {
		t.Fatal("expected list error status")
	}
}
