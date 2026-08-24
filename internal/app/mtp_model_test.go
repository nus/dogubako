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
