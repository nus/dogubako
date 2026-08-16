package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestValidateRuntime(t *testing.T) {
	if err := validateRuntime([]byte{0x7f, 'E', 'L', 'F', 0}); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntime([]byte("not elf")); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateAppDir(t *testing.T) {
	dir := t.TempDir()
	if err := validateAppDir(dir); err == nil {
		t.Fatal("expected missing AppRun")
	}
	if err := os.WriteFile(filepath.Join(dir, "AppRun"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateAppDir(dir); err == nil {
		t.Fatal("expected missing desktop")
	}
	if err := os.WriteFile(filepath.Join(dir, "app.desktop"), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateAppDir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAppImage(t *testing.T) {
	if _, err := exec.LookPath("mksquashfs"); err != nil {
		t.Skip("mksquashfs not installed")
	}

	dir := t.TempDir()
	appdir := filepath.Join(dir, "AppDir")
	if err := os.MkdirAll(filepath.Join(appdir, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdir, "AppRun"), []byte("#!/bin/sh\nexec echo ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdir, "demo.desktop"), []byte("[Desktop Entry]\nName=Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdir, "usr", "bin", "demo"), []byte("hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	runtimePath := filepath.Join(dir, "runtime")
	runtime := bytes.Repeat([]byte{0}, 64)
	copy(runtime, []byte{0x7f, 'E', 'L', 'F'})
	if err := os.WriteFile(runtimePath, runtime, 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "Demo.AppImage")
	if err := writeAppImage(out, appdir, runtimePath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !isELF(data) {
		t.Fatal("AppImage is not ELF")
	}
	offset := bytes.Index(data, []byte("hsqs"))
	if offset < 0 {
		t.Fatal("AppImage has no squashfs payload")
	}
	if offset < len(runtime) {
		t.Fatalf("squashfs offset %d overlaps runtime %d", offset, len(runtime))
	}

	if _, err := exec.LookPath("unsquashfs"); err != nil {
		return
	}
	extracted := filepath.Join(dir, "extracted")
	cmd := exec.Command("unsquashfs", "-o", strconv.Itoa(offset), "-d", extracted, out)
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unsquashfs: %v\n%s", err, outb)
	}
	if _, err := os.Stat(filepath.Join(extracted, "AppRun")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(extracted, "usr", "bin", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("payload %q", got)
	}
}

func TestWriteAppImageRejectsBadRuntime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AppRun"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.desktop"), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(dir, "runtime")
	if err := os.WriteFile(runtime, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAppImage(filepath.Join(dir, "out.AppImage"), dir, runtime); err == nil {
		t.Fatal("expected error")
	}
}
