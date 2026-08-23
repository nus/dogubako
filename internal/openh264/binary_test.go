package openh264

import (
	"runtime"
	"strings"
	"testing"
)

func TestBinaryFileName(t *testing.T) {
	name, err := BinaryFileName()
	if err != nil {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			return
		}
		t.Fatal(err)
	}
	if !strings.Contains(name, version) {
		t.Fatalf("name = %q", name)
	}
	url, err := BinaryURL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, cdnBase+"/") || !strings.HasSuffix(url, ".bz2") {
		t.Fatalf("url = %q", url)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv(EnvName, "0")
	if Enabled() {
		t.Fatal("expected disabled")
	}
	t.Setenv(EnvName, "off")
	if Enabled() {
		t.Fatal("expected off")
	}
	t.Setenv(EnvName, "")
	if !Enabled() {
		t.Fatal("empty should allow")
	}
}

func TestAllowDownloadInTest(t *testing.T) {
	t.Setenv(EnvName, "")
	if allowDownload() {
		t.Fatal("go test should not download unless asked")
	}
	t.Setenv(EnvName, "download")
	if !allowDownload() {
		t.Fatal("download should opt in")
	}
	t.Setenv(EnvName, "0")
	if allowDownload() {
		t.Fatal("disabled should not download")
	}
}
