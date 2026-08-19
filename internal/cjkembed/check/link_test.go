package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDarwinCompilesHiraginoLoader(t *testing.T) {
	root := moduleRoot(t)
	darwin := listGoFiles(t, root, "darwin", "./internal/cjkembed")
	if !containsFile(darwin, "hiragino_darwin.go") {
		t.Fatalf("darwin files = %q, want hiragino_darwin.go", darwin)
	}
	linux := listGoFiles(t, root, "linux", "./internal/cjkembed")
	if containsFile(linux, "hiragino_darwin.go") {
		t.Fatalf("linux files unexpectedly include hiragino_darwin.go: %q", linux)
	}
	if !containsFile(linux, "embed_linux.go") {
		t.Fatalf("linux files = %q, want embed_linux.go", linux)
	}
	if !containsFile(darwin, "emoji_darwin.go") {
		t.Fatalf("darwin files = %q, want emoji_darwin.go", darwin)
	}
	if containsFile(linux, "emoji_darwin.go") {
		t.Fatalf("linux files unexpectedly include emoji_darwin.go: %q", linux)
	}
	if !containsFile(linux, "emoji_linux.go") {
		t.Fatalf("linux files = %q, want emoji_linux.go", linux)
	}
}

func listGoFiles(t *testing.T, root, goos, pkg string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{join .GoFiles \" \"}}", pkg)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list GOOS=%s %s: %v\n%s", goos, pkg, err, out)
	}
	return strings.Fields(string(out))
}

func containsFile(files []string, name string) bool {
	for _, f := range files {
		if f == name {
			return true
		}
	}
	return false
}

func TestDogubakoDoesNotLinkGuigui(t *testing.T) {
	root := moduleRoot(t)
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/dogubako")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list GOOS=%s: %v\n%s", goos, err, out)
			}
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "github.com/guigui-gui/guigui" || strings.HasPrefix(line, "github.com/guigui-gui/guigui/") {
					t.Fatalf("guigui still linked on %s: %s", goos, line)
				}
			}
		})
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
