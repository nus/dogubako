package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteImageISO9660(t *testing.T) {
	src := t.TempDir()
	app := filepath.Join(src, "Dogubako.app", "Contents", "MacOS")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(app, "dogubako")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(src, "Dogubako.app", "Contents", "Info.plist")
	if err := os.WriteFile(plist, []byte("<plist></plist>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/Applications", filepath.Join(src, "Applications")); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "Dogubako.dmg")
	now := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	if err := writeImage(out, src, "道具箱", now); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 19*sectorSize {
		t.Fatalf("image too small: %d", len(data))
	}
	if len(data)%sectorSize != 0 {
		t.Fatalf("image not sector-aligned: %d", len(data))
	}

	pvd := data[16*sectorSize : 17*sectorSize]
	if pvd[0] != 1 || !bytes.Equal(pvd[1:6], []byte("CD001")) {
		t.Fatalf("missing PVD: type=%d id=%q", pvd[0], pvd[1:6])
	}
	volID := strings.TrimSpace(string(pvd[40:72]))
	if volID != "DOGUBAKO" {
		t.Fatalf("PVD volume id: %q", volID)
	}
	svd := data[17*sectorSize : 18*sectorSize]
	if svd[0] != 2 || !bytes.Equal(svd[88:91], []byte{0x25, 0x2F, 0x45}) {
		t.Fatalf("missing Joliet SVD")
	}
	term := data[18*sectorSize : 19*sectorSize]
	if term[0] != 255 {
		t.Fatalf("missing terminator: %d", term[0])
	}

	total := binary.LittleEndian.Uint32(pvd[80:84])
	if int(total)*sectorSize != len(data) {
		t.Fatalf("volume size %d sectors, file %d bytes", total, len(data))
	}
	if !bytes.Contains(data, []byte("dogubako")) {
		t.Fatal("Rock Ridge name for executable missing")
	}
	if !bytes.Contains(data, []byte("Info.plist")) {
		t.Fatal("Rock Ridge name for Info.plist missing")
	}
	if !bytes.Contains(data, []byte("Applications")) {
		t.Fatal("Applications symlink name missing")
	}
	if !bytes.Contains(data, []byte("#!/bin/sh\necho ok\n")) {
		t.Fatal("payload missing")
	}
}

func TestISOVolumeID(t *testing.T) {
	if got := isoVolumeID("道具箱"); got != "DOGUBAKO" {
		t.Fatalf("japanese: %q", got)
	}
	if got := isoVolumeID("Dogubako"); got != "DOGUBAKO" {
		t.Fatalf("ascii: %q", got)
	}
}

func TestISOSanitizeAndNames(t *testing.T) {
	if got := isoSanitize("Info.plist"); got != "INFO.PLIST" {
		t.Fatalf("sanitize: %q", got)
	}
	used := map[string]int{}
	n := &node{name: "Info.plist"}
	if got := uniqueISOName(n, used); got != "INFO.PLIST;1" {
		t.Fatalf("iso name: %q", got)
	}
	n2 := &node{name: "Info.plist"}
	if got := uniqueISOName(n2, used); got == "INFO.PLIST;1" {
		t.Fatalf("expected unique collision name, got %q", got)
	}
}

func TestDirRecordLenEven(t *testing.T) {
	for idLen := 1; idLen < 40; idLen++ {
		for suLen := 0; suLen < 60; suLen++ {
			n := dirRecordLen(idLen, suLen)
			if n%2 != 0 {
				t.Fatalf("odd record len id=%d su=%d -> %d", idLen, suLen, n)
			}
			if n < 33+idLen+suLen {
				t.Fatalf("record shorter than contents")
			}
		}
	}
}

func TestSymlinkSLRoot(t *testing.T) {
	rec := symlinkSL("/Applications")
	if rec[0] != 'S' || rec[1] != 'L' {
		t.Fatalf("SL header: %q", rec[:2])
	}
	if !bytes.Contains(rec, []byte("Applications")) {
		t.Fatalf("missing target: %q", rec)
	}
	if rec[5] != 0x08 {
		t.Fatalf("expected root component, got 0x%02x", rec[5])
	}
}
