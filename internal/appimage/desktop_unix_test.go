//go:build unix

package appimage

import (
	"os"
	"syscall"
	"testing"

	"github.com/nus/dogubako/internal/i18n"
)

func TestInstallModeIgnoresUmask(t *testing.T) {
	withLang(t, i18n.EN)
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	f := newFixture(t)
	code, handled, _, stderr := f.dispatch(installFlag)
	if code != 0 || !handled || stderr != "" {
		t.Fatalf("code=%d handled=%v stderr=%q", code, handled, stderr)
	}
	desktop, icon256, icon512 := f.installed()
	for _, path := range []string{desktop, icon256, icon512} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %o", path, fi.Mode().Perm())
		}
	}
}
