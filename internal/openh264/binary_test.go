package openh264

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

func TestPercent(t *testing.T) {
	if got := Percent(0, 100); got != 0 {
		t.Fatalf("0/100 = %d", got)
	}
	if got := Percent(50, 100); got != 50 {
		t.Fatalf("50/100 = %d", got)
	}
	if got := Percent(100, 100); got != 100 {
		t.Fatalf("100/100 = %d", got)
	}
	if got := Percent(9, 0); got != 0 {
		t.Fatalf("unknown total = %d", got)
	}
	if got := Percent(200, 100); got != 100 {
		t.Fatalf("over = %d", got)
	}
}

func TestCountingReaderReportsPercent(t *testing.T) {
	src := bytes.Repeat([]byte{1}, 100)
	var got []int
	r := &countingReader{
		r:       bytes.NewReader(src),
		total:   100,
		lastPct: -1,
		report: func(done, total int64) {
			got = append(got, Percent(done, total))
		},
	}
	buf := make([]byte, 25)
	for {
		_, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(got) == 0 || got[len(got)-1] != 100 {
		t.Fatalf("progress = %v", got)
	}
	if got[0] != 25 {
		t.Fatalf("first = %d", got[0])
	}
}

func TestDownloadLibraryReportsProgress(t *testing.T) {
	raw := bytes.Repeat([]byte("openh264-test"), 400)
	cmd := exec.Command("bzip2", "-c")
	cmd.Stdin = bytes.NewReader(raw)
	payload, err := cmd.Output()
	if err != nil {
		t.Skipf("bzip2: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = rw.Write(payload)
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "libopenh264-test.so")
	var last int
	reports := 0
	err = downloadLibrary(context.Background(), srv.URL, dest, func(done, total int64) {
		reports++
		last = Percent(done, total)
		if total != int64(len(payload)) {
			t.Errorf("total = %d want %d", total, len(payload))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if reports == 0 || last != 100 {
		t.Fatalf("reports=%d last=%d", reports, last)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("decompressed len=%d want=%d", len(got), len(raw))
	}
}
