package capture

import (
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/nus/dogubako/internal/imageproc"
)

// Mode selects how much of the screen to capture.
type Mode string

const (
	ModeFull   Mode = "full"
	ModeWindow Mode = "window"
	ModeRegion Mode = "region"
)

// Result is the outcome of an asynchronous capture.
type Result struct {
	Image     image.Image
	Cancelled bool
	Err       error
}

var (
	// ErrCancelled means the user dismissed an interactive picker.
	ErrCancelled = errors.New("capture cancelled")
	// ErrUnsupported is returned on operating systems this app does not target.
	ErrUnsupported = errors.New("screen capture is not supported on this OS")
	// ErrNoTool means no known screenshot helper was found on PATH.
	ErrNoTool = errors.New("no screenshot command found")
)

var (
	currentOS = runtime.GOOS
	lookPath  = exec.LookPath
	runCmd    = func(name string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		return cmd.CombinedOutput()
	}
)

// Normalize maps an unknown value onto ModeFull.
func Normalize(mode Mode) Mode {
	switch mode {
	case ModeWindow, ModeRegion:
		return mode
	default:
		return ModeFull
	}
}

// Async runs Capture after delay on a new goroutine.
func Async(mode Mode, delay time.Duration) <-chan Result {
	ch := make(chan Result, 1)
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		img, err := Capture(mode)
		ch <- resultFrom(img, err)
	}()
	return ch
}

func resultFrom(img image.Image, err error) Result {
	if err == nil {
		return Result{Image: img}
	}
	if errors.Is(err, ErrCancelled) {
		return Result{Cancelled: true, Err: err}
	}
	return Result{Err: err}
}

// Capture takes a screenshot with the platform helper and returns a decoded image.
func Capture(mode Mode) (image.Image, error) {
	mode = Normalize(mode)
	f, err := os.CreateTemp("", "dogubako-capture-*.png")
	if err != nil {
		return nil, err
	}
	dest := f.Name()
	_ = f.Close()
	defer os.Remove(dest)

	if err := runOSCapture(mode, dest); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrCancelled
	}
	img, _, err := imageproc.DecodeBytes(data)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	return img, nil
}

func runOSCapture(mode Mode, dest string) error {
	spec, err := commandFor(currentOS, mode, dest)
	if err != nil {
		return err
	}
	out, err := runCmd(spec.Name, spec.Args...)
	if err != nil {
		return classifyCmdError(spec.Name, out, err)
	}
	if _, statErr := os.Stat(dest); statErr != nil {
		return ErrCancelled
	}
	return nil
}

type cmdSpec struct {
	Name string
	Args []string
}

func commandFor(goos string, mode Mode, dest string) (cmdSpec, error) {
	switch goos {
	case "darwin":
		return darwinCommand(mode, dest), nil
	case "linux":
		return linuxCommand(mode, dest)
	default:
		return cmdSpec{}, fmt.Errorf("%w (%s)", ErrUnsupported, goos)
	}
}

func darwinCommand(mode Mode, dest string) cmdSpec {
	args := []string{"-x", "-t", "png"}
	switch mode {
	case ModeRegion:
		args = append(args, "-i")
	case ModeWindow:
		args = append(args, "-W")
	}
	args = append(args, dest)
	return cmdSpec{Name: "screencapture", Args: args}
}

func linuxCommand(mode Mode, dest string) (cmdSpec, error) {
	tool := firstOnPATH(linuxTools)
	if tool == "" {
		return cmdSpec{}, fmt.Errorf("%w: install gnome-screenshot (Ubuntu)", ErrNoTool)
	}
	return linuxArgs(tool, mode, dest)
}

var linuxTools = []string{
	"gnome-screenshot",
	"spectacle",
	"maim",
	"scrot",
	"grim",
	"import",
}

func firstOnPATH(names []string) string {
	for _, name := range names {
		if _, err := lookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func linuxArgs(tool string, mode Mode, dest string) (cmdSpec, error) {
	switch tool {
	case "gnome-screenshot":
		args := []string{"-f", dest}
		switch mode {
		case ModeWindow:
			args = append([]string{"-w"}, args...)
		case ModeRegion:
			args = append([]string{"-a"}, args...)
		}
		return cmdSpec{Name: tool, Args: args}, nil
	case "spectacle":
		args := []string{"-b", "-n", "-o", dest}
		switch mode {
		case ModeWindow:
			args = append(args, "-a")
		case ModeRegion:
			args = append(args, "-r")
		default:
			args = append(args, "-f")
		}
		return cmdSpec{Name: tool, Args: args}, nil
	case "maim":
		args := []string{dest}
		if mode == ModeRegion || mode == ModeWindow {
			args = []string{"-s", dest}
		}
		return cmdSpec{Name: tool, Args: args}, nil
	case "scrot":
		args := []string{dest}
		switch mode {
		case ModeRegion:
			args = []string{"-s", dest}
		case ModeWindow:
			args = []string{"-s", dest}
		}
		return cmdSpec{Name: tool, Args: args}, nil
	case "grim":
		if mode == ModeFull {
			return cmdSpec{Name: tool, Args: []string{dest}}, nil
		}
		if _, err := lookPath("slurp"); err != nil {
			return cmdSpec{}, fmt.Errorf("%w: grim needs slurp for region/window capture", ErrNoTool)
		}
		geom, err := runCmd("slurp")
		if err != nil {
			return cmdSpec{}, classifyCmdError("slurp", geom, err)
		}
		g := strings.TrimSpace(string(geom))
		if g == "" {
			return cmdSpec{}, ErrCancelled
		}
		return cmdSpec{Name: tool, Args: []string{"-g", g, dest}}, nil
	case "import":
		if mode == ModeFull {
			return cmdSpec{Name: tool, Args: []string{"-window", "root", dest}}, nil
		}
		return cmdSpec{Name: tool, Args: []string{dest}}, nil
	default:
		return cmdSpec{}, fmt.Errorf("%w: %s", ErrNoTool, tool)
	}
}

func classifyCmdError(name string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	lower := strings.ToLower(msg + " " + errText)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 && !looksLikeHardFailure(lower) {
		return ErrCancelled
	}
	if strings.Contains(lower, "not authorized") ||
		strings.Contains(lower, "permission") && strings.Contains(lower, "screen") {
		return fmt.Errorf("macOS の「画面収録」許可が必要です / Screen Recording permission required: %w", err)
	}
	if msg != "" {
		return fmt.Errorf("%s: %s", name, msg)
	}
	return fmt.Errorf("%s: %w", name, err)
}

func looksLikeHardFailure(lower string) bool {
	for _, needle := range []string{
		"cannot", "unable", "error", "failed", "not found", "no display", "permission",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
