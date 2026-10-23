package appimage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
)

const sampleDesktop = `# comment
[Desktop Entry]
Type=Application
Name=道具箱
Name[en]=Dogubako
Comment=Everyday workspace tools
Comment[ja]=日常作業向けツール群
Exec=dogubako
Icon=dogubako
Terminal=false
Categories=Utility;Graphics;
StartupNotify=true
X-AppImage-Version=0.1.0
`

func withLang(t *testing.T, lang i18n.Lang) {
	t.Helper()
	dir := t.TempDir()
	restore := i18n.OverrideUserConfigDir(func() (string, error) { return dir, nil })
	t.Cleanup(restore)
	if err := i18n.Save(lang); err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	t      *testing.T
	root   string
	data   string
	appdir string
	app    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		t:      t,
		root:   root,
		data:   filepath.Join(root, "data"),
		appdir: filepath.Join(root, "AppDir"),
		app:    filepath.Join(root, "Dogubako.AppImage"),
	}
	if err := os.WriteFile(f.app, []byte("appimage-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.writeSources(sampleDesktop, []byte("png-256"), []byte("png-512"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", f.data)
	t.Setenv("APPIMAGE", f.app)
	t.Setenv("APPDIR", f.appdir)
	return f
}

func (f *fixture) writeSources(desktop string, icon256, icon512 []byte) {
	f.t.Helper()
	mustWrite(f.t, filepath.Join(f.appdir, desktopFile), []byte(desktop))
	mustWrite(f.t, iconSource(f.appdir, "256x256"), icon256)
	mustWrite(f.t, iconSource(f.appdir, "512x512"), icon512)
	// Decoys the installer must not read.
	mustWrite(f.t, filepath.Join(f.appdir, iconFile), []byte("root-icon"))
	mustWrite(f.t, filepath.Join(f.appdir, "usr", "share", "applications", desktopFile), []byte("Exec=decoy\n"))
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) dispatch(args ...string) (code int, handled bool, stdout, stderr string) {
	f.t.Helper()
	var out, errb bytes.Buffer
	code, handled = Dispatch(args, &out, &errb)
	return code, handled, out.String(), errb.String()
}

func (f *fixture) installed() (desktop, icon256, icon512 string) {
	return installedPaths(f.data)
}

func TestInstallDesktop(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	code, handled, stdout, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("install: code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}
	desktop, icon256, icon512 := f.installed()
	if stdout != desktop+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	for _, path := range []string{desktop, icon256, icon512} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %o", path, fi.Mode().Perm())
		}
	}
	got256, err := os.ReadFile(icon256)
	if err != nil {
		t.Fatal(err)
	}
	got512, err := os.ReadFile(icon512)
	if err != nil {
		t.Fatal(err)
	}
	if string(got256) != "png-256" || string(got512) != "png-512" {
		t.Fatalf("icons = %q %q", got256, got512)
	}
	assertEntry(t, desktop, f.app)
	if _, err := os.Stat(f.app); err != nil {
		t.Fatal(err)
	}
}

func TestInstallOverwrites(t *testing.T) {
	withLang(t, i18n.JA)
	f := newFixture(t)
	desktop, icon256, icon512 := f.installed()
	mustWrite(t, desktop, []byte("old desktop\n"))
	if err := os.Chmod(desktop, 0o600); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, icon256, []byte("old-256"))
	mustWrite(t, icon512, []byte("old-512"))

	if code, handled, stdout, stderr := f.dispatch(installFlag); code != 0 || !handled || stderr != "" {
		t.Fatalf("first: code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}

	other := filepath.Join(f.root, "other.AppImage")
	if err := os.WriteFile(other, []byte("other"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.writeSources(sampleDesktop, []byte("new-256"), []byte("new-512"))
	t.Setenv("APPIMAGE", other)
	code, handled, stdout, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("second: code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}
	if stdout != desktop+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	assertEntry(t, desktop, other)
	got, err := os.ReadFile(icon256)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-256" {
		t.Fatalf("icon = %q", got)
	}
	fi, err := os.Stat(desktop)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o", fi.Mode().Perm())
	}
}

func TestInstallQuotesPath(t *testing.T) {
	withLang(t, i18n.EN)
	root := t.TempDir()
	dir := filepath.Join(root, "my apps")
	app := filepath.Join(dir, "100%.AppImage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	t.Setenv("APPIMAGE", app)
	code, handled, _, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("code=%d handled=%v stderr=%q", code, handled, stderr)
	}
	desktop, _, _ := f.installed()
	assertEntry(t, desktop, app)
}

func TestInstallRelativeAppImage(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	t.Chdir(f.root)
	t.Setenv("APPIMAGE", filepath.Base(f.app))
	code, handled, _, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("code=%d handled=%v stderr=%q", code, handled, stderr)
	}
	desktop, _, _ := f.installed()
	assertEntry(t, desktop, f.app)
}

func TestInstallFollowsSymlinkPath(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	link := filepath.Join(f.root, "linked.AppImage")
	if err := os.Symlink(f.app, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", link)
	code, handled, _, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("code=%d handled=%v stderr=%q", code, handled, stderr)
	}
	desktop, _, _ := f.installed()
	assertEntry(t, desktop, link)
}

func TestInstallDataHomeFallback(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	home := filepath.Join(f.root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	work := filepath.Join(f.root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)

	cases := []string{"", "unset", "relative/share"}
	for _, xdg := range cases {
		t.Run(xdg, func(t *testing.T) {
			switch xdg {
			case "":
				t.Setenv("XDG_DATA_HOME", "")
			case "unset":
				t.Setenv("XDG_DATA_HOME", "placeholder")
				os.Unsetenv("XDG_DATA_HOME")
			default:
				t.Setenv("XDG_DATA_HOME", xdg)
			}
			code, handled, stdout, stderr := f.dispatch(installFlag)
			if code != 0 || !handled || stderr != "" {
				t.Fatalf("code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
			}
			want := filepath.Join(home, ".local", "share", "applications", desktopFile)
			if stdout != want+"\n" {
				t.Fatalf("stdout = %q", stdout)
			}
			assertEntry(t, want, f.app)
		})
	}
	if _, err := os.Stat(filepath.Join(work, "relative")); !os.IsNotExist(err) {
		t.Fatalf("relative XDG_DATA_HOME was used: %v", err)
	}
}

func TestInstallFailuresWriteNothing(t *testing.T) {
	withLang(t, i18n.JA)
	f := newFixture(t)
	plant := filepath.Join(f.data, "applications", desktopFile)
	mustWrite(t, plant, []byte("keep\n"))

	t.Run("no appimage", func(t *testing.T) {
		t.Setenv("APPIMAGE", "")
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if stderr != i18n.T(i18n.JA, i18n.DesktopNeedAppImage)+"\n" {
			t.Fatalf("stderr = %q", stderr)
		}
		got, err := os.ReadFile(plant)
		if err != nil || string(got) != "keep\n" {
			t.Fatalf("plant changed: %q %v", got, err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		missing := filepath.Join(f.root, "missing.AppImage")
		t.Setenv("APPIMAGE", missing)
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if stderr != i18n.T(i18n.JA, i18n.DesktopAppImageMissing, missing)+"\n" {
			t.Fatalf("stderr = %q", stderr)
		}
		got, err := os.ReadFile(plant)
		if err != nil || string(got) != "keep\n" {
			t.Fatalf("plant changed: %q %v", got, err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		t.Setenv("APPIMAGE", f.root)
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if !strings.Contains(stderr, f.root) {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("newline path", func(t *testing.T) {
		path := filepath.Join(f.root, "bad\nname.AppImage")
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("APPIMAGE", path)
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if stderr != i18n.T(i18n.JA, i18n.DesktopBadPath)+"\n" {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("missing icon", func(t *testing.T) {
		t.Setenv("APPIMAGE", f.app)
		if err := os.Remove(iconSource(f.appdir, "512x512")); err != nil {
			t.Fatal(err)
		}
		fresh := filepath.Join(f.root, "empty-data")
		t.Setenv("XDG_DATA_HOME", fresh)
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" || stderr == "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if !strings.Contains(stderr, "アイコンを読めません") {
			t.Fatalf("stderr = %q", stderr)
		}
		if _, err := os.Stat(fresh); !os.IsNotExist(err) {
			t.Fatalf("data dir created: %v", err)
		}
	})

	t.Run("write fails", func(t *testing.T) {
		f.writeSources(sampleDesktop, []byte("png-256"), []byte("png-512"))
		blocker := filepath.Join(f.root, "not-a-dir")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_DATA_HOME", filepath.Join(blocker, "share"))
		code, handled, stdout, stderr := f.dispatch(installFlag)
		if code != 1 || !handled || stdout != "" || stderr == "" {
			t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
		}
		if !strings.Contains(stderr, "ファイルを書けません") {
			t.Fatalf("stderr = %q", stderr)
		}
	})
}

func TestUninstallDesktop(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	if code, handled, _, stderr := f.dispatch(installFlag); code != 0 || !handled || stderr != "" {
		t.Fatalf("install code=%d handled=%v stderr=%q", code, handled, stderr)
	}
	icon1024 := filepath.Join(f.data, "icons", "hicolor", "1024x1024", "apps", iconFile)
	mustWrite(t, icon1024, []byte("keep-1024"))
	other := filepath.Join(f.data, "applications", "other.desktop")
	mustWrite(t, other, []byte("other\n"))
	appBefore, err := os.ReadFile(f.app)
	if err != nil {
		t.Fatal(err)
	}

	code, handled, stdout, stderr := f.dispatch(uninstallFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("uninstall code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}
	if stdout != i18n.T(i18n.EN, i18n.DesktopRemoved)+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	desktop, icon256, icon512 := f.installed()
	for _, path := range []string{desktop, icon256, icon512} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still present: %v", path, err)
		}
	}
	for _, path := range []string{
		icon1024,
		other,
		filepath.Join(f.data, "applications"),
		filepath.Dir(icon256),
		filepath.Dir(icon512),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	appAfter, err := os.ReadFile(f.app)
	if err != nil {
		t.Fatal(err)
	}
	if string(appAfter) != string(appBefore) {
		t.Fatal("AppImage bytes changed")
	}

	code, handled, stdout, stderr = f.dispatch(uninstallFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("second code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}
	if stdout != i18n.T(i18n.EN, i18n.DesktopNothing)+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestUninstallPartialAndManualEntry(t *testing.T) {
	withLang(t, i18n.JA)
	f := newFixture(t)
	_, icon256, _ := f.installed()
	mustWrite(t, icon256, []byte("only-256"))
	manual := filepath.Join(f.data, "applications", desktopFile)
	mustWrite(t, manual, []byte("Exec=dogubako\nIcon=dogubako\n"))

	code, handled, stdout, stderr := f.dispatch(uninstallFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("code=%d handled=%v stderr=%q stdout=%q", code, handled, stderr, stdout)
	}
	if stdout != i18n.T(i18n.JA, i18n.DesktopRemoved)+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if _, err := os.Stat(icon256); !os.IsNotExist(err) {
		t.Fatalf("256 still present: %v", err)
	}
	if _, err := os.Stat(manual); !os.IsNotExist(err) {
		t.Fatalf("manual desktop still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.data, "applications")); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallRejectsMissingAppImage(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	desktop, _, _ := f.installed()
	mustWrite(t, desktop, []byte("keep\n"))

	t.Setenv("APPIMAGE", "")
	t.Setenv("APPDIR", "")
	code, handled, stdout, stderr := f.dispatch(uninstallFlag)
	if code != 1 || !handled || stdout != "" {
		t.Fatalf("unset: code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
	}
	if stderr != i18n.T(i18n.EN, i18n.DesktopNeedAppImage)+"\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	got, err := os.ReadFile(desktop)
	if err != nil || string(got) != "keep\n" {
		t.Fatalf("removed despite missing APPIMAGE: %q %v", got, err)
	}

	missing := filepath.Join(f.root, "gone.AppImage")
	t.Setenv("APPIMAGE", missing)
	code, handled, stdout, stderr = f.dispatch(uninstallFlag)
	if code != 1 || !handled || stdout != "" {
		t.Fatalf("missing: code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
	}
	got, err = os.ReadFile(desktop)
	if err != nil || string(got) != "keep\n" {
		t.Fatalf("removed despite missing file: %q %v", got, err)
	}
}

func TestUninstallRemoveError(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	desktop, icon256, _ := f.installed()
	if err := os.MkdirAll(filepath.Join(desktop, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, icon256, []byte("stay"))
	code, handled, stdout, stderr := f.dispatch(uninstallFlag)
	if code != 1 || !handled || stdout != "" || !strings.Contains(stderr, "Could not remove") {
		t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, stdout, stderr)
	}
	if _, err := os.Stat(icon256); err != nil {
		t.Fatalf("icon removed after desktop delete failed: %v", err)
	}
}

func TestDispatchUsageAndIgnoredArgs(t *testing.T) {
	withLang(t, i18n.EN)
	f := newFixture(t)
	desktop, _, _ := f.installed()
	mustWrite(t, desktop, []byte("keep\n"))
	usage := Usage(i18n.EN) + "\n"
	if !strings.Contains(usage, installFlag) || !strings.Contains(usage, uninstallFlag) {
		t.Fatalf("usage = %q", usage)
	}

	cases := [][]string{
		{installFlag, uninstallFlag},
		{uninstallFlag, installFlag},
		{installFlag, "extra"},
		{"git", installFlag},
		{installFlag, installFlag},
		{uninstallFlag, "--help"},
	}
	for _, args := range cases {
		code, handled, stdout, stderr := f.dispatch(args...)
		if code != 1 || !handled || stdout != "" || stderr != usage {
			t.Fatalf("%q: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, stdout, stderr)
		}
		got, err := os.ReadFile(desktop)
		if err != nil || string(got) != "keep\n" {
			t.Fatalf("%q changed the file: %q %v", args, got, err)
		}
	}

	for _, args := range [][]string{nil, {"git"}, {"git", "/tmp/repo"}, {"--help"}} {
		code, handled, stdout, stderr := f.dispatch(args...)
		if code != 0 || handled || stdout != "" || stderr != "" {
			t.Fatalf("%q: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, stdout, stderr)
		}
	}
}

func TestMessagesFollowLanguage(t *testing.T) {
	f := newFixture(t)
	t.Setenv("APPIMAGE", "")

	withLang(t, i18n.EN)
	_, _, _, stderr := f.dispatch(installFlag)
	if stderr != i18n.T(i18n.EN, i18n.DesktopNeedAppImage)+"\n" {
		t.Fatalf("en stderr = %q", stderr)
	}
	withLang(t, i18n.JA)
	_, _, _, stderr = f.dispatch(uninstallFlag)
	if stderr != i18n.T(i18n.JA, i18n.DesktopNeedAppImage)+"\n" {
		t.Fatalf("ja stderr = %q", stderr)
	}
	withLang(t, i18n.JA)
	_, _, _, stderr = f.dispatch(installFlag, uninstallFlag)
	if stderr != Usage(i18n.JA)+"\n" {
		t.Fatalf("ja usage = %q", stderr)
	}
}

func TestRewriteDesktop(t *testing.T) {
	in := "# Exec=dogubako\n" + sampleDesktop + "TryExec=/usr/bin/dogubako\n"
	got := rewriteDesktop(in, `"/tmp/my app%%.AppImage"`, "/tmp/my app%.AppImage")
	if strings.Count(got, "TryExec=") != 1 {
		t.Fatalf("TryExec count:\n%s", got)
	}
	if strings.Contains(got, "/usr/bin/dogubako") {
		t.Fatalf("old TryExec kept:\n%s", got)
	}
	if !strings.Contains(got, "# Exec=dogubako\n") {
		t.Fatalf("comment changed:\n%s", got)
	}
	if kept := stripExec(in); strings.Join(kept, "\n") != strings.Join(stripExec(got), "\n") {
		t.Fatalf("other lines changed\n got:\n%s", got)
	}
	entry := rewriteDesktop("Exec = dogubako\nIcon=dogubako\n", `"/bin/app"`, "/bin/app")
	if !strings.Contains(entry, "Exec=\"/bin/app\"\nTryExec=/bin/app\nIcon=dogubako\n") {
		t.Fatalf("entry = %q", entry)
	}
}

func TestQuoteExec(t *testing.T) {
	cases := map[string]string{
		`/usr/bin/app`:             `"/usr/bin/app"`,
		`/home/me/My App.AppImage`: `"/home/me/My App.AppImage"`,
		`/tmp/100%.AppImage`:       `"/tmp/100%%.AppImage"`,
		`/tmp/a"b`:                 `"/tmp/a\"b"`,
		`/tmp/a\b`:                 `"/tmp/a\\b"`,
		"/tmp/a\tb":                `"/tmp/a\tb"`,
	}
	for in, want := range cases {
		if got := quoteExec(in); got != want {
			t.Errorf("quoteExec(%q) = %q, want %q", in, got, want)
		}
	}
}

func assertEntry(t *testing.T, path, appPath string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("desktop entry does not end with a newline: %q", text)
	}
	fields := desktopFields(text)
	if fields["Exec"] != quoteExec(appPath) {
		t.Fatalf("Exec = %q", fields["Exec"])
	}
	if fields["TryExec"] != appPath {
		t.Fatalf("TryExec = %q", fields["TryExec"])
	}
	if fields["Icon"] != "dogubako" {
		t.Fatalf("Icon = %q", fields["Icon"])
	}
	if fields["Name"] != "道具箱" || fields["Name[en]"] != "Dogubako" {
		t.Fatalf("names = %q %q", fields["Name"], fields["Name[en]"])
	}
	if fields["Comment"] != "Everyday workspace tools" || fields["Comment[ja]"] != "日常作業向けツール群" {
		t.Fatalf("comments changed: %#v", fields)
	}
	if fields["Categories"] != "Utility;Graphics;" {
		t.Fatalf("Categories = %q", fields["Categories"])
	}
	if fields["X-AppImage-Version"] != "0.1.0" {
		t.Fatalf("version = %q", fields["X-AppImage-Version"])
	}
	if strings.Count(text, "\nExec=") != 1 || strings.Count(text, "\nTryExec=") != 1 {
		t.Fatalf("unexpected exec lines:\n%s", text)
	}
	keptIn := stripExec(sampleDesktop)
	keptOut := stripExec(text)
	if strings.Join(keptIn, "\n") != strings.Join(keptOut, "\n") {
		t.Fatalf("fields other than Exec/TryExec changed\n got:\n%s", text)
	}
}

func desktopFields(text string) map[string]string {
	fields := map[string]string{}
	for _, line := range splitLines(text) {
		key := desktopKey(line)
		if key == "" {
			continue
		}
		_, value, _ := strings.Cut(strings.TrimRight(line, "\r"), "=")
		fields[key] = value
	}
	return fields
}

func stripExec(content string) []string {
	var kept []string
	for _, line := range splitLines(content) {
		switch desktopKey(line) {
		case "Exec", "TryExec":
			continue
		default:
			kept = append(kept, line)
		}
	}
	return kept
}
