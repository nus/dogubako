package capture

import (
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"reflect"
	"testing"
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
	origLook := lookPath
	t.Cleanup(func() { lookPath = origLook })
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

func TestCommandForUnsupported(t *testing.T) {
	_, err := commandFor("windows", ModeFull, "/tmp/a.png")
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
	origOS, origLook, origRun := currentOS, lookPath, runCmd
	t.Cleanup(func() {
		currentOS, lookPath, runCmd = origOS, origLook, origRun
	})
	currentOS = "linux"
	lookPath = func(name string) (string, error) {
		if name == "gnome-screenshot" {
			return "/usr/bin/gnome-screenshot", nil
		}
		return "", exec.ErrNotFound
	}
	runCmd = func(name string, args ...string) ([]byte, error) {
		dest := args[len(args)-1]
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

	got, err := Capture(ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("size = %v", got.Bounds())
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
	if got := resultFrom(nil, ErrCancelled); !got.Cancelled {
		t.Fatalf("%+v", got)
	}
	if got := resultFrom(nil, ErrNoTool); got.Cancelled || got.Err == nil {
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
