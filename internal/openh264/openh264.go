// Package openh264 loads Cisco's prebuilt OpenH264 shared library via
// ebitengine/purego (CGO_ENABLED=0) and decodes Annex-B H.264.
//
// The binary is downloaded at runtime from Cisco so MPEG-LA fees stay on
// Cisco's license. The library is never compiled from source or statically
// linked. End users can disable it with DOGUBAKO_OPENH264=0, or delete the
// cached file under the user cache directory.
package openh264

import (
	"os"
	"strings"
	"testing"
)

const (
	// Attribution is the credit required by Cisco's binary license.
	Attribution = "OpenH264 Video Codec provided by Cisco Systems, Inc."

	// EnvName controls whether the Cisco binary is used.
	//
	//	unset  — download/load the codec (disabled automatically in go test)
	//	0/off  — never use OpenH264
	//	path   — load this shared library
	//	1      — allow a download even during go test
	EnvName = "DOGUBAKO_OPENH264"

	version = "2.6.0"
	cdnBase = "https://ciscobinary.openh264.org"
)

// ProgressFunc reports a download. total is 0 when the size is unknown.
type ProgressFunc func(downloaded, total int64)

// Percent is downloaded/total as 0–100. It is 0 when total is unknown.
func Percent(downloaded, total int64) int {
	if total <= 0 || downloaded <= 0 {
		return 0
	}
	if downloaded >= total {
		return 100
	}
	return int(downloaded * 100 / total)
}

// Enabled reports whether H.264 preview may try OpenH264.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvName))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}

func envLibraryPath() string {
	v := strings.TrimSpace(os.Getenv(EnvName))
	switch strings.ToLower(v) {
	case "", "0", "1", "off", "false", "no", "on", "true", "yes", "download":
		return ""
	}
	return v
}

func allowDownload() bool {
	if !Enabled() {
		return false
	}
	if envLibraryPath() != "" {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvName)))
	if testing.Testing() {
		return v == "1" || v == "download"
	}
	return true
}
