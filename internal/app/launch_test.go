package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
)

func useLang(t *testing.T, lang i18n.Lang) {
	t.Helper()
	dir := t.TempDir()
	restore := i18n.OverrideUserConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	if err := i18n.Save(lang); err != nil {
		t.Fatal(err)
	}
}

func isolateConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	restore := overrideSidebarConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
}

func parseLaunch(args ...string) (Launch, int, bool, string, string) {
	var stdout, stderr bytes.Buffer
	launch, code, open := ParseLaunch(args, &stdout, &stderr)
	return launch, code, open, stdout.String(), stderr.String()
}

func TestParseLaunchNoArgsKeepsWindow(t *testing.T) {
	useLang(t, i18n.EN)
	launch, code, open, stdout, stderr := parseLaunch()
	if !open || code != 0 || launch.Tool != "" || launch.Path != "" || stdout != "" || stderr != "" {
		t.Fatalf("open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
	}
}

func TestParseLaunchHelp(t *testing.T) {
	useLang(t, i18n.EN)
	usage := cliUsage(i18n.EN) + "\n"
	if !strings.Contains(usage, "dogubako git [directory]") || !strings.Contains(usage, "dogubako image [file]") {
		t.Fatalf("usage = %q", usage)
	}
	if !strings.Contains(usage, "--install-desktop") || !strings.Contains(usage, "--uninstall-desktop") || !strings.Contains(usage, "--help") {
		t.Fatalf("usage = %q", usage)
	}
	if knownTool(ToolMTP) {
		if !strings.Contains(usage, "\n  dogubako mtp\n") {
			t.Fatalf("usage = %q", usage)
		}
	} else if strings.Contains(usage, "dogubako mtp") {
		t.Fatalf("mtp listed where the tool is absent: %q", usage)
	}

	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		launch, code, open, stdout, stderr := parseLaunch(args...)
		if open || code != 0 || launch.Tool != "" || stdout != usage || stderr != "" {
			t.Fatalf("%q: open=%v code=%d stdout=%q stderr=%q", args, open, code, stdout, stderr)
		}
	}

	_, code, open, stdout, stderr := parseLaunch("--help", "git")
	if open || code != 1 || stdout != "" || stderr != usage {
		t.Fatalf("extra help: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
	}
}

func TestParseLaunchHelpFollowsLanguage(t *testing.T) {
	useLang(t, i18n.JA)
	_, code, open, stdout, stderr := parseLaunch("help")
	usage := cliUsage(i18n.JA) + "\n"
	if open || code != 0 || stdout != usage || stderr != "" {
		t.Fatalf("ja help: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
	}
	if !strings.Contains(stdout, "使い方:") || !strings.Contains(stdout, "dogubako git [ディレクトリ]") {
		t.Fatalf("ja usage = %q", stdout)
	}

	useLang(t, i18n.EN)
	_, _, _, stdout, _ = parseLaunch("-h")
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "dogubako git [directory]") {
		t.Fatalf("en usage = %q", stdout)
	}
}

func TestParseLaunchCommands(t *testing.T) {
	useLang(t, i18n.EN)
	usage := cliUsage(i18n.EN) + "\n"

	cases := []struct {
		args []string
		tool ToolID
	}{
		{[]string{"git"}, ToolGit},
		{[]string{"image"}, ToolImage},
		{[]string{"screenshot"}, ToolScreenshot},
		{[]string{"android"}, ToolAndroid},
		{[]string{"android-shot"}, ToolAndroidShot},
		{[]string{"stopwatch"}, ToolStopwatch},
	}
	for _, tc := range cases {
		launch, code, open, stdout, stderr := parseLaunch(tc.args...)
		if !open || code != 0 || launch.Tool != tc.tool || launch.Path != "" || stdout != "" || stderr != "" {
			t.Fatalf("%q: open=%v code=%d launch=%+v stdout=%q stderr=%q", tc.args, open, code, launch, stdout, stderr)
		}
	}

	if knownTool(ToolMTP) {
		launch, code, open, stdout, stderr := parseLaunch("mtp")
		if !open || code != 0 || launch.Tool != ToolMTP || stdout != "" || stderr != "" {
			t.Fatalf("mtp: open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
		}
	} else {
		_, code, open, stdout, stderr := parseLaunch("mtp")
		if open || code != 1 || stdout != "" || stderr != i18n.T(i18n.EN, i18n.CLINoMTP)+"\n" {
			t.Fatalf("missing mtp: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
		}
	}

	for _, args := range [][]string{{"nope"}, {"Git"}, {"git", "a", "b"}, {"screenshot", "extra"}, {"image", "a", "b"}, {"mtp", "extra"}} {
		_, code, open, stdout, stderr := parseLaunch(args...)
		if open || code != 1 || stdout != "" || stderr != usage {
			t.Fatalf("%q: open=%v code=%d stdout=%q stderr=%q", args, open, code, stdout, stderr)
		}
	}
}

func TestParseLaunchResolvesRelativePath(t *testing.T) {
	useLang(t, i18n.EN)
	dir := t.TempDir()
	t.Chdir(dir)
	launch, code, open, stdout, stderr := parseLaunch("git", filepath.Join("nested", "repo"))
	want, err := filepath.Abs(filepath.Join("nested", "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if !open || code != 0 || launch.Tool != ToolGit || launch.Path != want || stdout != "" || stderr != "" {
		t.Fatalf("open=%v code=%d launch=%+v want %s stdout=%q stderr=%q", open, code, launch, want, stdout, stderr)
	}

	launch, code, open, _, _ = parseLaunch("image", "photo.png")
	want, err = filepath.Abs("photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !open || code != 0 || launch.Tool != ToolImage || launch.Path != want {
		t.Fatalf("image: open=%v code=%d launch=%+v want %s", open, code, launch, want)
	}
}

func TestParseLaunchValidatesPathKind(t *testing.T) {
	useLang(t, i18n.EN)
	dir := t.TempDir()
	photo := filepath.Join(dir, "photo.png")
	writeLaunchPNG(t, photo)
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notes, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	absPhoto, err := filepath.Abs("photo.png")
	if err != nil {
		t.Fatal(err)
	}
	_, code, open, stdout, stderr := parseLaunch("git", "photo.png")
	want := i18n.T(i18n.EN, i18n.CLINotDirectory, absPhoto) + "\n"
	if open || code != 1 || stdout != "" || stderr != want {
		t.Fatalf("git file: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, code, open, stdout, stderr = parseLaunch("image", dir)
	want = i18n.T(i18n.EN, i18n.CLINotFile, absDir) + "\n"
	if open || code != 1 || stdout != "" || stderr != want {
		t.Fatalf("image dir: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
	}

	_, code, open, stdout, stderr = parseLaunch("git", " ")
	if open || code != 1 || stdout != "" || stderr != i18n.T(i18n.EN, i18n.CLINeedPath)+"\n" {
		t.Fatalf("blank: open=%v code=%d stdout=%q stderr=%q", open, code, stdout, stderr)
	}

	launch, code, open, stdout, stderr := parseLaunch("git", ".")
	absDot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if !open || code != 0 || launch.Tool != ToolGit || launch.Path != absDot || stdout != "" || stderr != "" {
		t.Fatalf("git dir: open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
	}

	launch, code, open, stdout, stderr = parseLaunch("image", "notes.txt")
	absNotes, err := filepath.Abs("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !open || code != 0 || launch.Tool != ToolImage || launch.Path != absNotes || stdout != "" || stderr != "" {
		t.Fatalf("image file: open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
	}

	launch, code, open, stdout, stderr = parseLaunch("git", "missing")
	if !open || code != 0 || launch.Tool != ToolGit || stdout != "" || stderr != "" {
		t.Fatalf("missing dir: open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
	}
	launch, code, open, stdout, stderr = parseLaunch("image", "missing.png")
	if !open || code != 0 || launch.Tool != ToolImage || stdout != "" || stderr != "" {
		t.Fatalf("missing file: open=%v code=%d launch=%+v stdout=%q stderr=%q", open, code, launch, stdout, stderr)
	}

	useLang(t, i18n.JA)
	_, _, _, stdout, stderr = parseLaunch("git", "photo.png")
	if stdout != "" || stderr != i18n.T(i18n.JA, i18n.CLINotDirectory, absPhoto)+"\n" {
		t.Fatalf("ja stderr = %q", stderr)
	}
}

func TestApplyLaunchSelectsTool(t *testing.T) {
	isolateConfig(t)

	var image Model
	image.SetMode(ToolGit)
	image.ApplyLaunch(Launch{})
	if image.Mode() != ToolGit {
		t.Fatalf("empty launch changed mode to %q", image.Mode())
	}

	var shot Model
	shot.SetAndroidFace(AndroidFaceFiles)
	shot.ApplyLaunch(Launch{Tool: ToolAndroidShot})
	if shot.Mode() != ToolAndroid || shot.AndroidFace() != AndroidFaceScreen {
		t.Fatalf("android-shot mode=%q face=%q", shot.Mode(), shot.AndroidFace())
	}
	var shotAgain Model
	if shotAgain.Mode() != ToolAndroid || shotAgain.AndroidFace() != AndroidFaceScreen {
		t.Fatalf("reloaded android-shot mode=%q face=%q", shotAgain.Mode(), shotAgain.AndroidFace())
	}

	var files Model
	files.SetAndroidFace(AndroidFaceScreen)
	files.SetMode(ToolStopwatch)
	files.ApplyLaunch(Launch{Tool: ToolAndroid})
	if files.Mode() != ToolAndroid || files.AndroidFace() != AndroidFaceFiles {
		t.Fatalf("android mode=%q face=%q", files.Mode(), files.AndroidFace())
	}

	var watch Model
	watch.ApplyLaunch(Launch{Tool: ToolStopwatch})
	var watchAgain Model
	if watchAgain.Mode() != ToolStopwatch {
		t.Fatalf("reloaded mode = %q", watchAgain.Mode())
	}
}

func TestApplyLaunchGit(t *testing.T) {
	isolateConfig(t)
	repo := initRepo(t, "app")
	sub := filepath.Join(repo, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	var opened Model
	opened.ApplyLaunch(Launch{Tool: ToolGit, Path: sub})
	waitGit(t, opened.Git())
	if opened.Mode() != ToolGit {
		t.Fatalf("mode = %q", opened.Mode())
	}
	if !opened.Git().HasRepo() || canonicalPath(opened.Git().Path()) != canonicalPath(repo) {
		t.Fatalf("repo = %q has=%v want %s", opened.Git().Path(), opened.Git().HasRepo(), canonicalPath(repo))
	}
	if !strings.Contains(opened.Git().StatusText(i18n.EN), "Opened") {
		t.Fatalf("status = %q", opened.Git().StatusText(i18n.EN))
	}

	var remembered Model
	remembered.ApplyLaunch(Launch{Tool: ToolGit})
	remembered.Git().EnsureLoaded()
	waitGit(t, remembered.Git())
	if canonicalPath(remembered.Git().Path()) != canonicalPath(repo) {
		t.Fatalf("remembered = %q", remembered.Git().Path())
	}

	var missing Model
	missing.ApplyLaunch(Launch{Tool: ToolGit, Path: filepath.Join(t.TempDir(), "missing")})
	waitGit(t, missing.Git())
	if missing.Mode() != ToolGit {
		t.Fatalf("missing mode = %q", missing.Mode())
	}
	if !strings.Contains(missing.Git().StatusText(i18n.EN), "Could not open the repository") {
		t.Fatalf("missing status = %q", missing.Git().StatusText(i18n.EN))
	}

	plain := t.TempDir()
	var notRepo Model
	notRepo.ApplyLaunch(Launch{Tool: ToolGit, Path: plain})
	waitGit(t, notRepo.Git())
	if notRepo.Mode() != ToolGit || notRepo.Git().HasRepo() {
		t.Fatalf("not repo mode=%q has=%v", notRepo.Mode(), notRepo.Git().HasRepo())
	}
	if !strings.Contains(notRepo.Git().StatusText(i18n.EN), "Could not open the repository") {
		t.Fatalf("not repo status = %q", notRepo.Git().StatusText(i18n.EN))
	}
}

func TestApplyLaunchImage(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeLaunchPNG(t, path)

	parent := filepath.Dir(dir)
	t.Chdir(parent)
	launch, code, open, _, _ := parseLaunch("image", filepath.Join(filepath.Base(dir), "photo.png"))
	if !open || code != 0 {
		t.Fatalf("open=%v code=%d", open, code)
	}

	var m Model
	m.ApplyLaunch(launch)
	if m.Mode() != ToolImage || !m.Image().HasSource() || m.Image().SourceName() != "photo.png" {
		t.Fatalf("mode=%q name=%q has=%v", m.Mode(), m.Image().SourceName(), m.Image().HasSource())
	}
	if !strings.Contains(m.Image().StatusText(i18n.EN), "Loaded photo.png") {
		t.Fatalf("status = %q", m.Image().StatusText(i18n.EN))
	}
	var again Model
	if again.Mode() != ToolImage {
		t.Fatalf("reloaded mode = %q", again.Mode())
	}

	var missing Model
	missing.ApplyLaunch(Launch{Tool: ToolImage, Path: filepath.Join(dir, "missing.png")})
	if missing.Mode() != ToolImage || missing.Image().HasSource() {
		t.Fatalf("missing mode=%q has=%v", missing.Mode(), missing.Image().HasSource())
	}
	if !strings.Contains(missing.Image().StatusText(i18n.EN), "Failed to load") {
		t.Fatalf("missing status = %q", missing.Image().StatusText(i18n.EN))
	}

	var folder Model
	folder.ApplyLaunch(Launch{Tool: ToolImage, Path: dir})
	if folder.Mode() != ToolImage || folder.Image().HasSource() {
		t.Fatalf("dir mode=%q has=%v", folder.Mode(), folder.Image().HasSource())
	}
	if !strings.Contains(folder.Image().StatusText(i18n.EN), "Failed to load") {
		t.Fatalf("dir status = %q", folder.Image().StatusText(i18n.EN))
	}
}

func writeLaunchPNG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
}
