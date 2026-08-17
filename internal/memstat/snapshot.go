// Package memstat captures Go heap and (on Linux) process RSS for diagnosing
// GUI memory use.
package memstat

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Snapshot is a point-in-time view of process memory.
type Snapshot struct {
	Label string `json:"label"`

	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapInuse  uint64 `json:"heap_inuse"`
	HeapIdle   uint64 `json:"heap_idle"`
	HeapSys    uint64 `json:"heap_sys"`
	StackInuse uint64 `json:"stack_inuse"`
	Sys        uint64 `json:"sys"`
	TotalAlloc uint64 `json:"total_alloc"`
	Mallocs    uint64 `json:"mallocs"`
	NumGC      uint32 `json:"num_gc"`

	// RSS and RSSPeak are populated on Linux from /proc/self/status.
	// They are 0 when unavailable.
	RSS     uint64 `json:"rss"`
	RSSPeak uint64 `json:"rss_peak"`
}

// Capture records memory statistics. When gc is true, a GC cycle and
// FreeOSMemory run first so retained heap is easier to compare.
func Capture(label string, gc bool) Snapshot {
	if gc {
		runtime.GC()
		debug.FreeOSMemory()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s := Snapshot{
		Label:      label,
		HeapAlloc:  ms.HeapAlloc,
		HeapInuse:  ms.HeapInuse,
		HeapIdle:   ms.HeapIdle,
		HeapSys:    ms.HeapSys,
		StackInuse: ms.StackInuse,
		Sys:        ms.Sys,
		TotalAlloc: ms.TotalAlloc,
		Mallocs:    ms.Mallocs,
		NumGC:      ms.NumGC,
	}
	fillRSS(&s)
	return s
}

// FormatBytes renders n as a human-readable IEC byte size.
func FormatBytes(n uint64) string {
	const (
		kiB = 1024
		miB = 1024 * kiB
		giB = 1024 * miB
	)
	switch {
	case n >= giB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(giB))
	case n >= miB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(miB))
	case n >= kiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// Format is a multi-line human-readable report for one snapshot.
func Format(s Snapshot) string {
	var b strings.Builder
	if s.Label != "" {
		fmt.Fprintf(&b, "%s\n", s.Label)
	}
	fmt.Fprintf(&b, "  heap_alloc   %s\n", FormatBytes(s.HeapAlloc))
	fmt.Fprintf(&b, "  heap_inuse   %s\n", FormatBytes(s.HeapInuse))
	fmt.Fprintf(&b, "  heap_sys     %s\n", FormatBytes(s.HeapSys))
	fmt.Fprintf(&b, "  sys          %s\n", FormatBytes(s.Sys))
	if s.RSS > 0 {
		fmt.Fprintf(&b, "  rss          %s\n", FormatBytes(s.RSS))
		fmt.Fprintf(&b, "  rss_peak     %s\n", FormatBytes(s.RSSPeak))
	} else {
		fmt.Fprintf(&b, "  rss          (unavailable)\n")
	}
	fmt.Fprintf(&b, "  mallocs      %d\n", s.Mallocs)
	fmt.Fprintf(&b, "  num_gc       %d\n", s.NumGC)
	return b.String()
}
