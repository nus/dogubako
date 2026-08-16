// Command makeappimage writes a type-2 AppImage from an AppDir and runtime ELF.
//
// The image is the runtime concatenated with a SquashFS of the AppDir.
// mksquashfs (squashfs-tools) is required.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	appdir := flag.String("appdir", "", "AppDir to pack")
	runtime := flag.String("runtime", "", "type-2 AppImage runtime ELF")
	out := flag.String("out", "", "output .AppImage path")
	flag.Parse()
	if *appdir == "" || *runtime == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: makeappimage -appdir DIR -runtime FILE -out FILE.AppImage")
		os.Exit(2)
	}
	if err := writeAppImage(*out, *appdir, *runtime); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeAppImage(outPath, appDir, runtimePath string) error {
	if err := validateAppDir(appDir); err != nil {
		return err
	}
	runtime, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	if err := validateRuntime(runtime); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	tmp, err := os.CreateTemp("", "dogubako-*.squashfs")
	if err != nil {
		return err
	}
	squashPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Remove(squashPath); err != nil {
		return err
	}
	defer os.Remove(squashPath)

	if err := makeSquashfs(appDir, squashPath); err != nil {
		return err
	}
	squash, err := os.ReadFile(squashPath)
	if err != nil {
		return err
	}
	if !isSquashfs(squash) {
		return fmt.Errorf("mksquashfs did not produce a squashfs image")
	}

	image := make([]byte, 0, len(runtime)+len(squash))
	image = append(image, runtime...)
	image = append(image, squash...)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, image, 0o755)
}

func validateAppDir(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "AppRun")); err != nil {
		return fmt.Errorf("missing AppRun in %s", dir)
	}
	desktops, err := filepath.Glob(filepath.Join(dir, "*.desktop"))
	if err != nil {
		return err
	}
	if len(desktops) == 0 {
		return fmt.Errorf("missing .desktop file in %s", dir)
	}
	return nil
}

func validateRuntime(b []byte) error {
	if !isELF(b) {
		return fmt.Errorf("not an ELF file")
	}
	return nil
}

func isELF(b []byte) bool {
	return len(b) >= 4 && b[0] == 0x7f && b[1] == 'E' && b[2] == 'L' && b[3] == 'F'
}

func isSquashfs(b []byte) bool {
	return len(b) >= 4 && b[0] == 'h' && b[1] == 's' && b[2] == 'q' && b[3] == 's'
}

func makeSquashfs(appDir, dest string) error {
	mksquashfs, err := exec.LookPath("mksquashfs")
	if err != nil {
		return fmt.Errorf("mksquashfs not found (install squashfs-tools)")
	}
	cmd := exec.Command(mksquashfs, appDir, dest,
		"-root-owned", "-noappend", "-comp", "gzip", "-all-root", "-no-xattrs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mksquashfs: %w\n%s", err, out)
	}
	return nil
}
