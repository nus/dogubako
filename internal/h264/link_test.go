package h264

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAttribution(t *testing.T) {
	a := Attribution()
	if runtime.GOOS == "darwin" {
		if a != "" {
			t.Fatalf("darwin attribution = %q", a)
		}
		return
	}
	if a == "" {
		t.Fatal("expected OpenH264 attribution")
	}
}

func TestEnabled(t *testing.T) {
	if runtime.GOOS == "darwin" && !Enabled() {
		t.Fatal("darwin should use VideoToolbox")
	}
}

func TestOpenH264NotLinkedOnDarwin(t *testing.T) {
	root := moduleRoot(t)
	if listsOpenH264(t, root, "darwin") {
		t.Fatal("darwin binary must not import internal/openh264")
	}
	if !listsOpenH264(t, root, "linux") {
		t.Fatal("linux binary should import internal/openh264")
	}
}

func TestPlatformDecoderFiles(t *testing.T) {
	root := moduleRoot(t)
	darwin := listGoFiles(t, root, "darwin", "./internal/h264")
	if !containsFile(darwin, "vt_darwin.go") || !containsFile(darwin, "codec_darwin.go") {
		t.Fatalf("darwin files = %q, want VideoToolbox", darwin)
	}
	if containsFile(darwin, "codec_other.go") {
		t.Fatalf("darwin files unexpectedly include codec_other.go: %q", darwin)
	}
	linux := listGoFiles(t, root, "linux", "./internal/h264")
	if !containsFile(linux, "codec_other.go") {
		t.Fatalf("linux files = %q, want codec_other.go", linux)
	}
	if containsFile(linux, "vt_darwin.go") || containsFile(linux, "codec_darwin.go") {
		t.Fatalf("linux files unexpectedly include VideoToolbox: %q", linux)
	}
}

func listsOpenH264(t *testing.T, root, goos string) bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/dogubako")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list GOOS=%s: %v\n%s", goos, err, out)
	}
	const pkg = "github.com/nus/dogubako/internal/openh264"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == pkg {
			return true
		}
	}
	return false
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

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
