package clipimg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ReadPNG reads an image/png payload from the system clipboard.
func ReadPNG() ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		return readDarwin()
	case "linux":
		return readLinux()
	default:
		return nil, fmt.Errorf("clipboard images are not supported on %s", runtime.GOOS)
	}
}

// WritePNG writes an image/png payload to the system clipboard.
func WritePNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty png")
	}
	switch runtime.GOOS {
	case "darwin":
		return writeDarwin(data)
	case "linux":
		return writeLinux(data)
	default:
		return fmt.Errorf("clipboard images are not supported on %s", runtime.GOOS)
	}
}

func readLinux() ([]byte, error) {
	if out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o").Output(); err == nil && len(out) > 0 {
		return out, nil
	}
	if out, err := exec.Command("wl-paste", "--type", "image/png", "--no-newline").Output(); err == nil && len(out) > 0 {
		return out, nil
	}
	return nil, fmt.Errorf("clipboard has no png")
}

func writeLinux(data []byte) error {
	if err := runStdin(data, "xclip", "-selection", "clipboard", "-t", "image/png"); err == nil {
		return nil
	}
	if err := runStdin(data, "wl-copy", "--type", "image/png"); err == nil {
		return nil
	}
	return fmt.Errorf("clipboard write failed (install xclip or wl-clipboard)")
}

func runStdin(data []byte, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func writeDarwin(data []byte) error {
	f, err := os.CreateTemp("", "dogubako-clip-*.png")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	script := fmt.Sprintf(`set the clipboard to (read (POSIX file %q) as «class PNGf»)`, path)
	return exec.Command("osascript", "-e", script).Run()
}

func readDarwin() ([]byte, error) {
	f, err := os.CreateTemp("", "dogubako-clip-*.png")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	script := strings.Join([]string{
		`try`,
		`  set png_data to the clipboard as «class PNGf»`,
		fmt.Sprintf(`  set out to open for access POSIX file %q with write permission`, path),
		`  set eof out to 0`,
		`  write png_data to out`,
		`  close access out`,
		`on error`,
		`  try`,
		`    close access out`,
		`  end try`,
		`  error "no png"`,
		`end try`,
	}, "\n")
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return nil, fmt.Errorf("clipboard has no png")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("clipboard has no png")
	}
	return data, nil
}
