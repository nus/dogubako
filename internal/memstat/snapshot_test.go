package memstat

import (
	"runtime"
	"strings"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1536, "1.5 KiB"},
		{3 * 1024 * 1024, "3.0 MiB"},
		{2 * 1024 * 1024 * 1024, "2.0 GiB"},
	}
	for _, tc := range cases {
		if got := FormatBytes(tc.n); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestCapture(t *testing.T) {
	s := Capture("test", true)
	if s.Label != "test" {
		t.Fatalf("label = %q", s.Label)
	}
	if s.Sys == 0 {
		t.Fatal("sys is 0")
	}
	if runtime.GOOS == "linux" && s.RSS == 0 {
		t.Fatal("expected RSS on Linux")
	}
	text := Format(s)
	if !strings.Contains(text, "heap_inuse") {
		t.Fatalf("format missing heap_inuse: %s", text)
	}
	if !strings.Contains(text, "rss") {
		t.Fatalf("format missing rss: %s", text)
	}
}
