package openh264

import (
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const maxLibraryBytes = 20 << 20

// BinaryFileName is the Cisco prebuilt name for this OS/arch (uncompressed).
func BinaryFileName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "libopenh264-" + version + "-linux64.8.so", nil
		case "arm64":
			return "libopenh264-" + version + "-linux-arm64.8.so", nil
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "libopenh264-" + version + "-mac-x64.dylib", nil
		case "arm64":
			return "libopenh264-" + version + "-mac-arm64.dylib", nil
		}
	}
	return "", fmt.Errorf("openh264: no Cisco binary for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// BinaryURL is the Cisco CDN URL of the bzip2-compressed library.
func BinaryURL() (string, error) {
	name, err := BinaryFileName()
	if err != nil {
		return "", err
	}
	return cdnBase + "/" + name + ".bz2", nil
}

func cachePath() (string, error) {
	if p := envLibraryPath(); p != "" {
		return p, nil
	}
	name, err := BinaryFileName()
	if err != nil {
		return "", err
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogubako", "openh264", version, name), nil
}

// EnsureLibrary returns a local path to the Cisco shared library, downloading
// it into the user cache when needed.
func EnsureLibrary(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !Enabled() {
		return "", fmt.Errorf("openh264: disabled")
	}
	path, err := cachePath()
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return path, nil
	}
	if !allowDownload() {
		return "", fmt.Errorf("openh264: library not cached")
	}
	url, err := BinaryURL()
	if err != nil {
		return "", err
	}
	if err := downloadLibrary(ctx, url, path); err != nil {
		return "", err
	}
	return path, nil
}

func downloadLibrary(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dogubako")
	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("openh264: download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("openh264: download: HTTP %s", res.Status)
	}

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(bzip2.NewReader(res.Body), maxLibraryBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("openh264: decompress: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n > maxLibraryBytes {
		return fmt.Errorf("openh264: library too large")
	}
	if n == 0 {
		return fmt.Errorf("openh264: empty library")
	}
	return os.Rename(tmp, dest)
}
