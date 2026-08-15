package dialog

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

func TestTrimPath(t *testing.T) {
	if got := trimPath("  \n"); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := trimPath("file:///tmp/a.png\n"); got != "/tmp/a.png" {
		t.Fatalf("file uri = %q", got)
	}
}

func TestZenityAndKdialogArgs(t *testing.T) {
	filter := &FileFilter{Description: "Images", Extensions: []string{"png", "jpg", "jpeg"}}

	name, args := zenityOpenArgs("画像を開く", filter)
	if name != "zenity" {
		t.Fatalf("name = %s", name)
	}
	if !contains(args, "--file-selection") || !contains(args, "--title=画像を開く") {
		t.Fatalf("open args = %v", args)
	}
	if !contains(args, "--file-filter=Images | *.png *.jpg *.jpeg") {
		t.Fatalf("filter arg = %v", args)
	}

	name, args = zenitySaveArgs("画像を保存", "out.png", filter)
	if name != "zenity" || !contains(args, "--save") || !contains(args, "--filename=out.png") {
		t.Fatalf("save args = %v", args)
	}

	name, args = kdialogOpenArgs("画像を開く", filter)
	if name != "kdialog" {
		t.Fatalf("name = %s", name)
	}
	if !reflect.DeepEqual(args, []string{"--getopenfilename", ".", "Images (*.png *.jpg *.jpeg)", "--title", "画像を開く"}) {
		t.Fatalf("kdialog open = %v", args)
	}
}

func TestResultFromCmd(t *testing.T) {
	if got := resultFromCmd([]byte("/tmp/a.png\n"), nil); got.Path != "/tmp/a.png" || got.Cancelled {
		t.Fatalf("ok = %+v", got)
	}
	if got := resultFromCmd(nil, nil); !got.Cancelled {
		t.Fatalf("empty output should cancel: %+v", got)
	}
	if got := resultFromCmd(nil, errors.New("boom")); got.Err == nil || got.Cancelled {
		t.Fatalf("other errors must not look like cancel: %+v", got)
	}
}

func TestTryExternalMissingCommand(t *testing.T) {
	origLook, origCmd := execLookPath, execCommand
	t.Cleanup(func() {
		execLookPath = origLook
		execCommand = origCmd
	})
	execLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, ok := tryExternal("zenity", []string{"--file-selection"}); ok {
		t.Fatal("missing command should skip")
	}
}

func TestTryExternalCancelAndPath(t *testing.T) {
	origLook, origCmd := execLookPath, execCommand
	t.Cleanup(func() {
		execLookPath = origLook
		execCommand = origCmd
	})
	execLookPath = func(string) (string, error) { return "/usr/bin/zenity", nil }

	execCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("/home/user/pic.png\n"), nil
	}
	res, ok := tryExternal("zenity", []string{"--file-selection"})
	if !ok || res.Path != "/home/user/pic.png" {
		t.Fatalf("path = %+v ok=%v", res, ok)
	}

	execCommand = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("display missing")
	}
	res, ok = tryExternal("zenity", nil)
	if !ok || res.Err == nil {
		t.Fatalf("error result = %+v ok=%v", res, ok)
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
