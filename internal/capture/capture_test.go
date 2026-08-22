package capture

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestDarwinCommand(t *testing.T) {
	got := darwinCommand(ModeFull, "/tmp/a.png")
	if got.Name != "screencapture" {
		t.Fatalf("name = %s", got.Name)
	}
	if !reflect.DeepEqual(got.Args, []string{"-x", "-t", "png", "/tmp/a.png"}) {
		t.Fatalf("full args = %v", got.Args)
	}
	got = darwinCommand(ModeRegion, "/tmp/a.png")
	if !contains(got.Args, "-i") {
		t.Fatalf("region args = %v", got.Args)
	}
	got = darwinCommand(ModeWindow, "/tmp/a.png")
	if !contains(got.Args, "-W") {
		t.Fatalf("window args = %v", got.Args)
	}
}

func TestLinuxGnomeScreenshotArgs(t *testing.T) {
	got, err := linuxArgs("gnome-screenshot", ModeFull, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-f", "/tmp/a.png"}) {
		t.Fatalf("full = %v", got.Args)
	}
	got, err = linuxArgs("gnome-screenshot", ModeRegion, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-a", "-f", "/tmp/a.png"}) {
		t.Fatalf("region = %v", got.Args)
	}
	got, err = linuxArgs("gnome-screenshot", ModeWindow, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-w", "-f", "/tmp/a.png"}) {
		t.Fatalf("window = %v", got.Args)
	}
}

func TestLinuxCommandPicksFirstTool(t *testing.T) {
	origLook, origEnv := lookPath, environ
	t.Cleanup(func() {
		lookPath = origLook
		environ = origEnv
	})
	environ = func(string) string { return "" }
	lookPath = func(name string) (string, error) {
		if name == "maim" {
			return "/usr/bin/maim", nil
		}
		return "", exec.ErrNotFound
	}
	got, err := linuxCommand(ModeFull, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "maim" {
		t.Fatalf("name = %s", got.Name)
	}
}

func TestLinuxCommandNoTool(t *testing.T) {
	origLook := lookPath
	t.Cleanup(func() { lookPath = origLook })
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	_, err := linuxCommand(ModeFull, "/tmp/a.png")
	if !errors.Is(err, ErrNoTool) {
		t.Fatalf("err = %v", err)
	}
}

func TestPreferredLinuxToolsDefersGnomeOnXFCE(t *testing.T) {
	origLook, origEnv := lookPath, environ
	t.Cleanup(func() {
		lookPath = origLook
		environ = origEnv
	})
	environ = func(key string) string {
		if key == "XDG_CURRENT_DESKTOP" {
			return "XFCE"
		}
		return ""
	}
	lookPath = func(name string) (string, error) {
		switch name {
		case "gnome-screenshot", "scrot":
			return "/usr/bin/" + name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	got, err := linuxCommands(ModeFull, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("tools = %v", got)
	}
	if got[0].Name != "scrot" {
		t.Fatalf("xfce first = %s, want scrot", got[0].Name)
	}
	if got[len(got)-1].Name != "gnome-screenshot" {
		t.Fatalf("xfce last = %s, want gnome-screenshot", got[len(got)-1].Name)
	}
}

func TestPreferredLinuxToolsKeepsGnomeOnGNOME(t *testing.T) {
	origLook, origEnv := lookPath, environ
	t.Cleanup(func() {
		lookPath = origLook
		environ = origEnv
	})
	environ = func(key string) string {
		if key == "XDG_CURRENT_DESKTOP" {
			return "ubuntu:GNOME"
		}
		return ""
	}
	lookPath = func(name string) (string, error) {
		switch name {
		case "gnome-screenshot", "scrot":
			return "/usr/bin/" + name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	got, err := linuxCommands(ModeFull, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "gnome-screenshot" {
		t.Fatalf("gnome first = %s", got[0].Name)
	}
}

func TestCommandForUnsupported(t *testing.T) {
	_, err := commandsFor("windows", ModeFull, "/tmp/a.png")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

func TestClassifyCancelVsFailure(t *testing.T) {
	cancel := processExit(1)
	if err := classifyCmdError("screencapture", nil, cancel); !errors.Is(err, ErrCancelled) {
		t.Fatalf("exit 1 should cancel: %v", err)
	}
	if err := classifyCmdError("gnome-screenshot", []byte("Unable to capture\n"), cancel); errors.Is(err, ErrCancelled) {
		t.Fatal("hard failure must not look like cancel")
	}
}

func TestCaptureWithStubCommand(t *testing.T) {
	origOS, origLook, origRun, origEnv := currentOS, lookPath, runCmd, environ
	t.Cleanup(func() {
		currentOS, lookPath, runCmd, environ = origOS, origLook, origRun, origEnv
	})
	currentOS = "linux"
	environ = func(string) string { return "" }
	lookPath = func(name string) (string, error) {
		if name == "gnome-screenshot" {
			return "/usr/bin/gnome-screenshot", nil
		}
		return "", exec.ErrNotFound
	}
	runCmd = func(_ context.Context, name string, args ...string) ([]byte, error) {
		dest := args[len(args)-1]
		return writeStubPNG(dest)
	}

	got, err := Capture(context.Background(), ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("size = %v", got.Bounds())
	}
}

func TestCaptureFallsBackWhenFirstToolWritesNothing(t *testing.T) {
	origOS, origLook, origRun, origEnv := currentOS, lookPath, runCmd, environ
	t.Cleanup(func() {
		currentOS, lookPath, runCmd, environ = origOS, origLook, origRun, origEnv
	})
	currentOS = "linux"
	environ = func(string) string { return "" }
	lookPath = func(name string) (string, error) {
		switch name {
		case "gnome-screenshot", "scrot":
			return "/usr/bin/" + name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	var tried []string
	runCmd = func(_ context.Context, name string, args ...string) ([]byte, error) {
		tried = append(tried, name)
		if name == "gnome-screenshot" {
			return nil, nil
		}
		return writeStubPNG(args[len(args)-1])
	}

	got, err := Capture(context.Background(), ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 3 {
		t.Fatalf("size = %v", got.Bounds())
	}
	if !reflect.DeepEqual(tried, []string{"gnome-screenshot", "scrot"}) {
		t.Fatalf("tried = %v", tried)
	}
}

func TestCaptureEmptyFileIsNotSuccess(t *testing.T) {
	origOS, origLook, origRun, origEnv := currentOS, lookPath, runCmd, environ
	t.Cleanup(func() {
		currentOS, lookPath, runCmd, environ = origOS, origLook, origRun, origEnv
	})
	currentOS = "linux"
	environ = func(string) string { return "" }
	lookPath = func(name string) (string, error) {
		if name == "gnome-screenshot" {
			return "/usr/bin/gnome-screenshot", nil
		}
		return "", exec.ErrNotFound
	}
	runCmd = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		f, err := os.Create(args[len(args)-1])
		if err != nil {
			return nil, err
		}
		return nil, f.Close()
	}

	_, err := Capture(context.Background(), ModeFull)
	if err == nil {
		t.Fatal("expected error for empty screenshot")
	}
	if errors.Is(err, ErrCancelled) {
		t.Fatal("empty file must not look like user cancel")
	}
}

func TestCaptureTimeout(t *testing.T) {
	origOS, origLook, origRun, origEnv := currentOS, lookPath, runCmd, environ
	t.Cleanup(func() {
		currentOS, lookPath, runCmd, environ = origOS, origLook, origRun, origEnv
	})
	currentOS = "linux"
	environ = func(string) string { return "" }
	lookPath = func(name string) (string, error) {
		if name == "gnome-screenshot" {
			return "/usr/bin/gnome-screenshot", nil
		}
		return "", exec.ErrNotFound
	}
	runCmd = func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Capture(ctx, ModeFull)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestResultFromTimeout(t *testing.T) {
	got := resultFrom("", ErrTimeout)
	if got.Cancelled || !errors.Is(got.Err, ErrTimeout) {
		t.Fatalf("%+v", got)
	}
}

func TestTimeoutByMode(t *testing.T) {
	if Timeout(ModeFull) != 15*time.Second {
		t.Fatalf("full = %s", Timeout(ModeFull))
	}
	if Timeout(ModeRegion) != 90*time.Second {
		t.Fatalf("region = %s", Timeout(ModeRegion))
	}
}

func TestNormalize(t *testing.T) {
	if Normalize("") != ModeFull {
		t.Fatal("empty")
	}
	if Normalize(ModeRegion) != ModeRegion {
		t.Fatal("region")
	}
}

func TestResultFrom(t *testing.T) {
	if got := resultFrom("", ErrCancelled); !got.Cancelled {
		t.Fatalf("%+v", got)
	}
	if got := resultFrom("", ErrNoTool); got.Cancelled || got.Err == nil {
		t.Fatalf("%+v", got)
	}
}

func TestLinuxArgsMaimAndImport(t *testing.T) {
	got, err := linuxArgs("maim", ModeRegion, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-s", "/tmp/a.png"}) {
		t.Fatalf("maim = %v", got.Args)
	}
	got, err = linuxArgs("import", ModeFull, "/tmp/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-window", "root", "/tmp/a.png"}) {
		t.Fatalf("import full = %v", got.Args)
	}
}

func TestLiveFallbackWhenGnomeIsBusy(t *testing.T) {
	if os.Getenv("DOGUBAKO_LIVE_CAPTURE") != "1" {
		t.Skip("set DOGUBAKO_LIVE_CAPTURE=1")
	}
	if _, err := exec.LookPath("gnome-screenshot"); err != nil {
		t.Skip(err)
	}
	if _, err := exec.LookPath("scrot"); err != nil {
		t.Skip(err)
	}
	stuckCtx, stuckCancel := context.WithCancel(context.Background())
	defer stuckCancel()
	stuck := exec.CommandContext(stuckCtx, "gnome-screenshot", "-a", "-f", t.TempDir()+"/stuck.png")
	stuck.Env = os.Environ()
	if err := stuck.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stuckCancel()
		_ = stuck.Wait()
	}()
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	img, err := Capture(ctx, ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() < 10 || img.Bounds().Dy() < 10 {
		t.Fatalf("size = %v", img.Bounds())
	}
}

func TestResolveSpecSlurp(t *testing.T) {
	origRun := runCmd
	t.Cleanup(func() { runCmd = origRun })
	runCmd = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name != "slurp" {
			t.Fatalf("name = %s", name)
		}
		return []byte("10,20 30x40\n"), nil
	}
	got, err := resolveSpec(context.Background(), cmdSpec{
		Name:       "grim",
		Args:       []string{"-g", "", "/tmp/a.png"},
		needsSlurp: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"-g", "10,20 30x40", "/tmp/a.png"}) {
		t.Fatalf("args = %v", got.Args)
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func processExit(code int) error {
	return exec.Command("sh", "-c", "exit 1").Run()
}

func writeStubPNG(dest string) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return nil, err
	}
	return nil, nil
}
