package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/i18n"
)

func waitAndroid(t *testing.T, m *AndroidModel) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.Drain()
		if !m.Busy() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for android model")
}

func TestAndroidModelListsTreeAndCopy(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	created := now.Add(-time.Hour)
	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel 7"})
	fs.PutDir("/sdcard/DCIM", now, created)
	fs.PutFile("/sdcard/DCIM/a.txt", []byte("hello"), now, created)
	fs.PutFile("/sdcard/note.txt", []byte("n"), now, created)

	var m AndroidModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroid(t, &m)

	if m.Serial() != "pixel" {
		t.Fatalf("serial = %q", m.Serial())
	}
	if m.Root() != "/sdcard" {
		t.Fatalf("root = %q", m.Root())
	}
	rows := m.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if !rows[0].Entry.IsDir || rows[0].Entry.Name != "DCIM" {
		t.Fatalf("first row = %+v", rows[0].Entry)
	}
	if rows[0].Entry.CrtTime.IsZero() || rows[0].Entry.ModTime.IsZero() {
		t.Fatal("expected timestamps")
	}

	m.ToggleExpand("/sdcard/DCIM")
	waitAndroid(t, &m)
	rows = m.Rows()
	if len(rows) != 3 {
		t.Fatalf("expanded rows = %d", len(rows))
	}
	if rows[1].Depth != 1 || rows[1].Entry.Name != "a.txt" {
		t.Fatalf("child = %+v depth=%d", rows[1].Entry, rows[1].Depth)
	}

	m.SelectPath("/sdcard/DCIM/a.txt")
	dir := t.TempDir()
	m.StartPull(dir)
	waitAndroid(t, &m)
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("pulled = %q", got)
	}
	if got := m.StatusText(i18n.EN); got == "" {
		t.Fatal("expected copy status")
	}

	src := filepath.Join(t.TempDir(), "frompc.txt")
	if err := os.WriteFile(src, []byte("pc"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.SelectPath("/sdcard")
	m.StartPush(src)
	waitAndroid(t, &m)
	data, ok := fs.FileData("/sdcard/frompc.txt")
	if !ok || string(data) != "pc" {
		t.Fatalf("pushed = %q ok=%v", data, ok)
	}
}

func TestAndroidModelOfflineDevice(t *testing.T) {
	fs := adbfs.NewMem(adbfs.Device{Serial: "x", State: "unauthorized"})
	var m AndroidModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroid(t, &m)
	if m.StatusText(i18n.EN) == "" {
		t.Fatal("expected offline status")
	}
	if len(m.Rows()) != 0 {
		t.Fatal("should not list offline device")
	}
}

func TestAndroidModelConnectError(t *testing.T) {
	fs := adbfs.NewMem()
	fs.DevErr = context.DeadlineExceeded
	var m AndroidModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroid(t, &m)
	if got := m.StatusText(i18n.JA); got == "" {
		t.Fatal("expected connect error")
	}
}

func TestFormatFileSizeAndTime(t *testing.T) {
	if got := formatFileSize(100, true); got != "—" {
		t.Fatalf("dir size = %q", got)
	}
	if got := formatFileSize(512, false); got != "512 B" {
		t.Fatalf("bytes = %q", got)
	}
	if got := formatFileTime(time.Time{}); got != "—" {
		t.Fatalf("zero time = %q", got)
	}
	ts := time.Date(2026, 8, 18, 12, 30, 0, 0, time.Local)
	if got := formatFileTime(ts); got != "2026-08-18 12:30" {
		t.Fatalf("time = %q", got)
	}
}

func TestAndroidModelSortsByColumn(t *testing.T) {
	old := time.Unix(1_700_000_000, 0)
	mid := old.Add(time.Hour)
	neu := old.Add(2 * time.Hour)
	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel 7"})
	fs.PutDir("/sdcard/DCIM", mid, mid)
	fs.PutFile("/sdcard/note.txt", []byte("n"), neu, neu)
	fs.PutFile("/sdcard/big.bin", make([]byte, 2048), old, old)

	var m AndroidModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroid(t, &m)

	names := func() []string {
		var out []string
		for _, r := range m.Rows() {
			out = append(out, r.Entry.Name)
		}
		return out
	}

	if got := names(); len(got) != 3 || got[0] != "DCIM" || got[1] != "big.bin" || got[2] != "note.txt" {
		t.Fatalf("default name asc = %v", got)
	}

	m.ToggleSort(androidSortName)
	if !m.SortDesc() || m.SortCol() != androidSortName {
		t.Fatalf("name toggle desc col=%d desc=%v", m.SortCol(), m.SortDesc())
	}
	if got := names(); got[0] != "DCIM" || got[1] != "note.txt" || got[2] != "big.bin" {
		t.Fatalf("name desc = %v", got)
	}

	m.ToggleSort(androidSortSize)
	if m.SortDesc() || m.SortCol() != androidSortSize {
		t.Fatalf("size asc col=%d desc=%v", m.SortCol(), m.SortDesc())
	}
	if got := names(); got[0] != "DCIM" || got[1] != "note.txt" || got[2] != "big.bin" {
		t.Fatalf("size asc = %v", got)
	}

	m.ToggleSort(androidSortSize)
	if got := names(); got[0] != "big.bin" || got[1] != "note.txt" || got[2] != "DCIM" {
		t.Fatalf("size desc = %v", got)
	}

	m.ToggleSort(androidSortMod)
	if got := names(); got[0] != "big.bin" || got[1] != "DCIM" || got[2] != "note.txt" {
		t.Fatalf("mod asc = %v", got)
	}
	m.ToggleSort(androidSortMod)
	if got := names(); got[0] != "note.txt" || got[1] != "DCIM" || got[2] != "big.bin" {
		t.Fatalf("mod desc = %v", got)
	}

	m.ToggleSort(0)
	if m.SortCol() != androidSortMod || !m.SortDesc() {
		t.Fatal("invalid column should be ignored")
	}
}
