package main

import (
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreateUniversal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	arm := filepath.Join(dir, "arm64")
	amd := filepath.Join(dir, "amd64")
	buildDarwin(t, src, arm, "arm64")
	buildDarwin(t, src, amd, "amd64")

	out := filepath.Join(dir, "universal")
	if err := create(out, []string{arm, amd}); err != nil {
		t.Fatal(err)
	}

	fat, err := macho.OpenFat(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fat.Close()
	if len(fat.Arches) != 2 {
		t.Fatalf("arches: %d", len(fat.Arches))
	}
	seen := map[macho.Cpu]bool{}
	for _, a := range fat.Arches {
		seen[a.Cpu] = true
	}
	if !seen[macho.CpuAmd64] || !seen[macho.CpuArm64] {
		t.Fatalf("missing arch in %+v", seen)
	}
}

func TestCreateUsage(t *testing.T) {
	if err := create("", nil); err == nil {
		t.Fatal("expected error")
	}
	if err := create("out", []string{"only-one"}); err == nil {
		t.Fatal("expected error")
	}
}

func buildDarwin(t *testing.T, src, out, arch string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH="+arch)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", arch, err, out)
	}
}
