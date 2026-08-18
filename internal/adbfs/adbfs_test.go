package adbfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanJoinParent(t *testing.T) {
	if got := Clean("sdcard/DCIM"); got != "/sdcard/DCIM" {
		t.Fatalf("clean = %q", got)
	}
	if got := Join("/sdcard", "DCIM", "a.jpg"); got != "/sdcard/DCIM/a.jpg" {
		t.Fatalf("join = %q", got)
	}
	if got := Parent("/sdcard/DCIM"); got != "/sdcard" {
		t.Fatalf("parent = %q", got)
	}
	if got := Parent("/"); got != "/" {
		t.Fatalf("parent of / = %q", got)
	}
	if got := Base("/sdcard/a.txt"); got != "a.txt" {
		t.Fatalf("base = %q", got)
	}
	if !InDir("/sdcard", "/sdcard/DCIM/a.jpg") || InDir("/sdcard", "/storage") {
		t.Fatal("InDir")
	}
}

func TestParseDevicesL(t *testing.T) {
	in := `
emulator-5554          device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1
R5CT00XXXX             unauthorized usb:1-1 transport_id:2
Pixel_serial           device usb:2-1 product:panther model:Pixel_7 device:panther
`
	got := parseDevicesL(in)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Serial != "emulator-5554" || got[0].State != "device" || got[0].Model != "sdk gphone64 arm64" {
		t.Fatalf("emu = %+v", got[0])
	}
	if got[1].Online() {
		t.Fatal("unauthorized should be offline")
	}
	if got[2].Label() != "Pixel 7  Pixel_serial" {
		t.Fatalf("label = %q", got[2].Label())
	}
}

func TestApplyBirthOutput(t *testing.T) {
	mod := time.Unix(100, 0)
	entries := []Entry{
		{Path: "/sdcard/a.txt", ModTime: mod},
		{Path: "/sdcard/b.txt", ModTime: mod},
	}
	byPath := map[string]*Entry{
		entries[0].Path: &entries[0],
		entries[1].Path: &entries[1],
	}
	applyBirthOutput("/sdcard/a.txt|1700000000\n/sdcard/b.txt|0\n", byPath)
	if entries[0].CrtTime.Unix() != 1700000000 {
		t.Fatalf("a crt = %v", entries[0].CrtTime)
	}
	if !entries[1].CrtTime.IsZero() {
		t.Fatalf("b should stay zero: %v", entries[1].CrtTime)
	}
}

func TestMemPullPush(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := NewMem(Device{Serial: "dev1", State: "device", Model: "Pixel"})
	fs.PutDir("/sdcard/DCIM", now, now.Add(-time.Hour))
	fs.PutFile("/sdcard/DCIM/a.txt", []byte("hello"), now, now.Add(-time.Minute))

	ctx := context.Background()
	ents, err := fs.List(ctx, "dev1", "/sdcard")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || !ents[0].IsDir || ents[0].Name != "DCIM" {
		t.Fatalf("list sdcard = %+v", ents)
	}

	dir := t.TempDir()
	n, err := Pull(ctx, fs, "dev1", "/sdcard/DCIM", dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pulled files = %d", n)
	}
	got, err := os.ReadFile(filepath.Join(dir, "DCIM", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q", got)
	}

	src := t.TempDir()
	sub := filepath.Join(src, "frompc")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("pc"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err = Push(ctx, fs, "dev1", sub, "/sdcard")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pushed files = %d", n)
	}
	data, ok := fs.FileData("/sdcard/frompc/b.txt")
	if !ok || string(data) != "pc" {
		t.Fatalf("pushed = %q ok=%v", data, ok)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("quote = %s", got)
	}
}
