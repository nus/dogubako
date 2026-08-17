//go:build linux

package memstat

import "testing"

func TestParseStatusRSS(t *testing.T) {
	const status = "" +
		"Name:\tdogubako\n" +
		"VmPeak:\t  401000 kB\n" +
		"VmSize:\t  400000 kB\n" +
		"VmHWM:\t   210000 kB\n" +
		"VmRSS:\t   200000 kB\n"
	rss, peak, ok := parseStatusRSS([]byte(status))
	if !ok {
		t.Fatal("parseStatusRSS returned !ok")
	}
	if rss != 200000*1024 {
		t.Fatalf("rss = %d", rss)
	}
	if peak != 210000*1024 {
		t.Fatalf("peak = %d", peak)
	}
}
