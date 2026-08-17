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

func TestDogubakoLinksCJKOnlyOnLinux(t *testing.T) {
	root := moduleRoot(t)
	for _, tc := range []struct {
		goos string
		want bool
	}{
		{"linux", true},
		{"darwin", false},
		{"windows", false},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			linked := listsCJKFont(t, root, tc.goos)
			if linked != tc.want {
				t.Fatalf("cjkfont linked=%v, want %v", linked, tc.want)
			}
		})
	}
}

func listsCJKFont(t *testing.T, root, goos string) bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/dogubako")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list GOOS=%s: %v\n%s", goos, err, out)
	}
	const pkg = "github.com/guigui-gui/guigui/basicwidget/cjkfont"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == pkg {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
