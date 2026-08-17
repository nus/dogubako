//go:build linux

package memstat

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

func fillRSS(s *Snapshot) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return
	}
	rss, peak, ok := parseStatusRSS(data)
	if !ok {
		return
	}
	s.RSS = rss
	s.RSSPeak = peak
}

func parseStatusRSS(data []byte) (rss, peak uint64, ok bool) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	var haveRSS, havePeak bool
	for sc.Scan() {
		line := sc.Text()
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		n, parsed := parseKib(val)
		if !parsed {
			continue
		}
		switch key {
		case "VmRSS":
			rss = n
			haveRSS = true
		case "VmHWM":
			peak = n
			havePeak = true
		}
	}
	return rss, peak, haveRSS && havePeak
}

func parseKib(val string) (uint64, bool) {
	val = strings.TrimSpace(val)
	val = strings.TrimSuffix(val, "kB")
	val = strings.TrimSpace(val)
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, false
	}
	return n * 1024, true
}
