package capture

import (
	"context"
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

const (
	timeoutFull        = 15 * time.Second
	timeoutInteractive = 90 * time.Second
	minPNGSize         = 8
)

// Result is the outcome of an asynchronous capture.
type Result struct {
	Image     image.Image
	Cancelled bool
	Err       error
}

var (
	// ErrCancelled means the user dismissed an interactive picker, or a
	// capture was aborted to start another one.
	ErrCancelled = errors.New("capture cancelled")
	// ErrTimeout means the helper did not finish before the deadline.
	ErrTimeout = errors.New("capture timed out")
	// ErrUnsupported is returned on operating systems this app does not target.
	ErrUnsupported = errors.New("screen capture is not supported on this OS")
	// ErrNoTool means no known screenshot helper was found on PATH.
	ErrNoTool = errors.New("no screenshot command found")
)

var (
	currentOS = runtime.GOOS
	lookPath  = exec.LookPath
	runCmd    = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.CombinedOutput()
	}
	environ = os.Getenv
)

// Timeout is how long a helper may run after any delay, by capture mode.
func Timeout(mode Mode) time.Duration {
	switch Normalize(mode) {
	case ModeWindow, ModeRegion:
		return timeoutInteractive
	default:
		return timeoutFull
	}
}

// Normalize maps an unknown value onto ModeFull.
func Normalize(mode Mode) Mode {
	switch mode {
	case ModeWindow, ModeRegion:
		return mode
	default:
		return ModeFull
	}
}

// Async runs Capture after delay on a new goroutine. ctx cancels the delay
// and kills the helper process.
func Async(ctx context.Context, mode Mode, delay time.Duration) <-chan Result {
	if ctx == nil {
		ctx = context.Background()
	}
	ch := make(chan Result, 1)
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				ch <- resultFrom(nil, classifyContextError(ctx.Err()))
				return
			case <-timer.C:
			}
		}
		img, err := Capture(ctx, mode)
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
func Capture(ctx context.Context, mode Mode) (image.Image, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	mode = Normalize(mode)
	f, err := os.CreateTemp("", "dogubako-capture-*.png")
	if err != nil {
		return nil, err
	}
	dest := f.Name()
	_ = f.Close()
	// Do not leave an empty file: gnome-screenshot can exit 0 without writing
	// (GApplication single-instance), and an empty dest used to look like a cancel.
	_ = os.Remove(dest)
	defer os.Remove(dest)

	if err := runOSCapture(ctx, mode, dest); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	if len(data) < minPNGSize {
		return nil, fmt.Errorf("screenshot file is empty")
	}
	img, _, err := imageproc.DecodeBytes(data)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	return img, nil
}

func runOSCapture(ctx context.Context, mode Mode, dest string) error {
	specs, err := commandsFor(currentOS, mode, dest)
	if err != nil {
		return err
	}
	var lastErr error
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return classifyContextError(err)
		}
		_ = os.Remove(dest)
		spec, err := resolveSpec(ctx, spec)
		if err != nil {
			if errors.Is(err, ErrCancelled) {
				return err
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return classifyContextError(ctxErr)
			}
			lastErr = err
			continue
		}
		out, err := runCmd(ctx, spec.Name, spec.Args...)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return classifyContextError(ctxErr)
		}
		if err != nil {
			classified := classifyCmdError(spec.Name, out, err)
			if errors.Is(classified, ErrCancelled) {
				return classified
			}
			lastErr = classified
			continue
		}
		if hasImageFile(dest) {
			return nil
		}
		lastErr = fmt.Errorf("%s produced no image", spec.Name)
	}
	if lastErr != nil {
		return lastErr
	}
	return ErrCancelled
}

func hasImageFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < minPNGSize {
		return false
	}
	return true
}

func classifyContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ErrCancelled
}

type cmdSpec struct {
	Name       string
	Args       []string
	needsSlurp bool
}

func resolveSpec(ctx context.Context, spec cmdSpec) (cmdSpec, error) {
	if !spec.needsSlurp {
		return spec, nil
	}
	geom, err := runCmd(ctx, "slurp")
	if ctxErr := ctx.Err(); ctxErr != nil {
		return cmdSpec{}, classifyContextError(ctxErr)
	}
	if err != nil {
		return cmdSpec{}, classifyCmdError("slurp", geom, err)
	}
	g := strings.TrimSpace(string(geom))
	if g == "" {
		return cmdSpec{}, ErrCancelled
	}
	args := append([]string{}, spec.Args...)
	for i, a := range args {
		if a == "" {
			args[i] = g
			spec.Args = args
			return spec, nil
		}
	}
	spec.Args = append([]string{"-g", g}, args...)
	return spec, nil
}

func commandsFor(goos string, mode Mode, dest string) ([]cmdSpec, error) {
	switch goos {
	case "darwin":
		return []cmdSpec{darwinCommand(mode, dest)}, nil
	case "linux":
		return linuxCommands(mode, dest)
	default:
		return nil, fmt.Errorf("%w (%s)", ErrUnsupported, goos)
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
	specs, err := linuxCommands(mode, dest)
	if err != nil {
		return cmdSpec{}, err
	}
	return specs[0], nil
}

func linuxCommands(mode Mode, dest string) ([]cmdSpec, error) {
	var specs []cmdSpec
	var lastErr error
	for _, tool := range preferredLinuxTools() {
		if _, err := lookPath(tool); err != nil {
			continue
		}
		spec, err := linuxArgs(tool, mode, dest)
		if err != nil {
			lastErr = err
			continue
		}
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("%w: install gnome-screenshot (Ubuntu)", ErrNoTool)
	}
	return specs, nil
}

var linuxTools = []string{
	"gnome-screenshot",
	"spectacle",
	"maim",
	"scrot",
	"grim",
	"import",
}

func preferredLinuxTools() []string {
	hint := linuxSessionHint()
	preferGnome := desktopPrefers(hint, "gnome", "unity", "cinnamon")
	preferKDE := desktopPrefers(hint, "kde", "plasma")
	known := hint != "" && (preferGnome || preferKDE ||
		desktopPrefers(hint, "xfce", "lxqt", "lxde", "mate", "i3", "sway", "hyprland", "budgie"))

	out := make([]string, 0, len(linuxTools))
	var deferred []string
	for _, tool := range linuxTools {
		switch {
		case !known:
			out = append(out, tool)
		case tool == "gnome-screenshot" && !preferGnome:
			deferred = append(deferred, tool)
		case tool == "spectacle" && !preferKDE:
			deferred = append(deferred, tool)
		default:
			out = append(out, tool)
		}
	}
	return append(out, deferred...)
}

func linuxSessionHint() string {
	return strings.ToLower(strings.Join([]string{
		environ("XDG_CURRENT_DESKTOP"),
		environ("DESKTOP_SESSION"),
		environ("GDMSESSION"),
	}, ":"))
}

func desktopPrefers(hint string, names ...string) bool {
	for _, n := range names {
		if strings.Contains(hint, n) {
			return true
		}
	}
	return false
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
		case ModeRegion, ModeWindow:
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
		return cmdSpec{Name: tool, Args: []string{"-g", "", dest}, needsSlurp: true}, nil
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
