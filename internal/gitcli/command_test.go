package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatCommand(t *testing.T) {
	if got := FormatCommand([]string{"add", "-A"}); got != "git add -A" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCommand([]string{"commit", "-m", "hello world"}); got != "git commit -m 'hello world'" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCommand([]string{"add", "--", "my file.txt"}); got != "git add -- 'my file.txt'" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCommand([]string{"commit", "-m", "it's ready"}); got != `git commit -m 'it'\''s ready'` {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(FormatCommand([]string{"fetch", "--prune", "origin"}), "--no-pager") {
		t.Fatal("pager flag leaked into the display")
	}
}

func TestCommandTraceUserArgs(t *testing.T) {
	dir := initRepo(t)
	writeCommit(t, dir, "a.txt", "first")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	var events []string
	repo.SetCommandTrace(func(begin bool, args []string, stdout, stderr string) {
		line := FormatCommand(args)
		if strings.Contains(line, "--no-pager") || strings.Contains(line, " -C ") || strings.Contains(line, "ssh") {
			t.Errorf("trace includes an implementation wrapper: %q", line)
		}
		if begin {
			events = append(events, "start "+line)
			return
		}
		events = append(events, "end "+line+" out="+stdout+" err="+stderr)
	})

	if _, err := repo.Snapshot(ctx, SnapshotOpts{}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("snapshot reads were traced: %#v", events)
	}
	if err := repo.Pull(ctx); err == nil {
		t.Fatal("expected missing upstream")
	}
	if len(events) != 0 {
		t.Fatalf("pull lookup was traced: %#v", events)
	}
	if err := repo.CreateTag(ctx, "bad name", "HEAD"); err == nil {
		t.Fatal("expected invalid tag")
	}
	if len(events) != 0 {
		t.Fatalf("check-ref-format was traced: %#v", events)
	}

	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(ctx, "hello world", true, false); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(events, "\n")
	if !strings.Contains(joined, "start git add -A\n") {
		t.Fatalf("missing add start:\n%s", joined)
	}
	if !strings.Contains(joined, "start git commit -m 'hello world'\n") {
		t.Fatalf("missing commit start:\n%s", joined)
	}
	if !strings.Contains(joined, "end git commit -m 'hello world'") {
		t.Fatalf("missing commit end:\n%s", joined)
	}
	addAt := strings.Index(joined, "start git add -A")
	commitAt := strings.Index(joined, "start git commit -m 'hello world'")
	if addAt < 0 || commitAt < 0 || addAt > commitAt {
		t.Fatalf("order:\n%s", joined)
	}

	before := len(events)
	err = repo.Push(ctx)
	if err == nil {
		t.Fatal("expected push to fail without origin")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("push error should keep git's message")
	}
	var sawEnd bool
	for _, ev := range events[before:] {
		if !strings.Contains(ev, "git push -u origin main") {
			t.Fatalf("unexpected push trace %q", ev)
		}
		if strings.HasPrefix(ev, "end ") {
			sawEnd = true
			if !strings.Contains(ev, "out=") || strings.HasSuffix(ev, "out= err=") {
				t.Fatalf("failed push dropped both streams: %q", ev)
			}
		}
	}
	if !sawEnd {
		t.Fatalf("push end missing: %#v", events[before:])
	}
}
