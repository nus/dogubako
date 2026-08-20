package app

import (
	"testing"
	"time"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/nus/dogubako/internal/adbfs"
)

func TestAndroidTreeHitRow(t *testing.T) {
	const w, n = float32(400), 3
	if got := androidTreeHitRow(10, 10, w, 0, n); got != -1 {
		t.Fatalf("header = %d", got)
	}
	if got := androidTreeHitRow(10, androidTableHeaderHeight, w, 0, n); got != 0 {
		t.Fatalf("first row = %d", got)
	}
	if got := androidTreeHitRow(10, androidTableHeaderHeight+androidTableRowHeight, w, 0, n); got != 1 {
		t.Fatalf("second row = %d", got)
	}
	if got := androidTreeHitRow(10, androidTableHeaderHeight+androidTableRowHeight, w, androidTableRowHeight, n); got != 2 {
		t.Fatalf("scrolled row = %d", got)
	}
	if got := androidTreeHitRow(10, androidTableHeaderHeight+androidTableRowHeight*10, w, 0, n); got != -1 {
		t.Fatalf("past last row = %d", got)
	}
	if got := androidTreeHitRow(w-1, androidTableHeaderHeight+4, w, 0, n); got != -1 {
		t.Fatalf("scrollbar = %d", got)
	}
}

func TestAndroidTreeRetogglePath(t *testing.T) {
	rows := []AndroidTreeRow{
		{Entry: adbfs.Entry{Path: "/sdcard/DCIM", Name: "DCIM", IsDir: true}, Expanded: true},
		{Entry: adbfs.Entry{Path: "/sdcard/DCIM/a.txt", Name: "a.txt"}},
		{Entry: adbfs.Entry{Path: "/sdcard/note.txt", Name: "note.txt"}},
	}
	if got := androidTreeRetogglePath("/sdcard/DCIM", rows, 0); got != "/sdcard/DCIM" {
		t.Fatalf("selected dir = %q", got)
	}
	if got := androidTreeRetogglePath("/sdcard/DCIM/a.txt", rows, 0); got != "" {
		t.Fatalf("other selection on dir = %q", got)
	}
	if got := androidTreeRetogglePath("/sdcard/DCIM/a.txt", rows, 1); got != "" {
		t.Fatalf("selected file = %q", got)
	}
	if got := androidTreeRetogglePath("/sdcard/DCIM", rows, -1); got != "" {
		t.Fatalf("header = %q", got)
	}
}

func TestAndroidModelReclickSelectedDirCollapses(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel 7"})
	fs.PutDir("/sdcard/DCIM", now, now)
	fs.PutFile("/sdcard/DCIM/a.txt", []byte("hello"), now, now)
	fs.PutFile("/sdcard/note.txt", []byte("n"), now, now)

	var m AndroidModel
	m.SetClient(fs)
	m.RefreshDevices()
	waitAndroid(t, &m)

	activateAndroidTreeRow(&m, 0)
	waitAndroid(t, &m)
	if !m.Rows()[0].Expanded {
		t.Fatal("first click should expand")
	}
	if m.Selected() != "/sdcard/DCIM" {
		t.Fatalf("selected = %q", m.Selected())
	}
	if n := len(m.Rows()); n != 3 {
		t.Fatalf("expanded rows = %d", n)
	}

	path := androidTreeRetogglePath(m.Selected(), m.Rows(), 0)
	if path == "" {
		t.Fatal("reclick of selected dir should retoggle")
	}
	m.ToggleExpand(path)
	if m.Rows()[0].Expanded {
		t.Fatal("second click should collapse")
	}
	if n := len(m.Rows()); n != 2 {
		t.Fatalf("collapsed rows = %d", n)
	}
}

func TestAndroidTreeTableRetogglesSelectedDir(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := adbfs.NewMem(adbfs.Device{Serial: "pixel", State: "device", Model: "Pixel 7"})
	fs.PutDir("/sdcard/DCIM", now, now)
	fs.PutFile("/sdcard/note.txt", []byte("n"), now, now)

	s := &Shell{}
	s.model.Android().SetClient(fs)
	s.model.Android().RefreshDevices()
	waitAndroid(t, s.model.Android())
	activateAndroidTreeRow(s.model.Android(), 0)
	waitAndroid(t, s.model.Android())
	if !s.model.Android().Rows()[0].Expanded {
		t.Fatal("expected expanded dir")
	}

	host := newAndroidTreeTable(s, nil, nil)
	host.SetBounds(geometry.NewRect(0, 0, 400, 200))
	me := event.NewMouseEvent(event.MousePress, event.ButtonLeft, event.ButtonStateLeft,
		geometry.Pt(20, androidTableHeaderHeight+4), geometry.Pt(20, androidTableHeaderHeight+4), 0)
	if got := host.retogglePath(me); got != "/sdcard/DCIM" {
		t.Fatalf("retoggle = %q", got)
	}
}
