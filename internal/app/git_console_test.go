package app

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestGitConsoleLog(t *testing.T) {
	var log gitConsoleLog
	log.begin([]string{"add", "-A"})
	if got := log.text(); got != "$ git add -A\n" {
		t.Fatalf("start = %q", got)
	}
	if !log.consumeDirty() {
		t.Fatal("start should ask for a redraw")
	}
	log.finish("", "")
	if got := log.text(); got != "$ git add -A\n" {
		t.Fatalf("empty success = %q", got)
	}
	log.begin([]string{"commit", "-m", "hello world"})
	log.finish("saved\n", "hint\n")
	got := log.text()
	want := "$ git add -A\n\n$ git commit -m 'hello world'\nsaved\nhint\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	log.begin([]string{"push", "origin"})
	log.finish("remote\n", "")
	if !strings.Contains(log.text(), "saved\nhint\n") || !strings.Contains(log.text(), "remote\n") {
		t.Fatalf("failure and success output = %q", log.text())
	}
}

func TestGitConsoleSeparatesOutput(t *testing.T) {
	var log gitConsoleLog
	log.begin([]string{"commit", "-m", "hello world"})
	text, spans := log.render()
	if text != "$ git commit -m 'hello world'\n" || len(spans) != 1 || spans[0].kind != consoleCommand {
		t.Fatalf("running = %q %#v", text, spans)
	}
	log.finish("saved\n", "fatal: x\n")
	text, spans = log.render()
	want := "$ git commit -m 'hello world'\nsaved\nfatal: x\n"
	if text != want {
		t.Fatalf("got %q", text)
	}
	if len(spans) != 3 {
		t.Fatalf("spans = %#v", spans)
	}
	if spans[0].kind != consoleCommand || text[spans[0].start:spans[0].end] != "$ git commit -m 'hello world'\n" {
		t.Fatalf("command span = %#v %q", spans[0], text[spans[0].start:spans[0].end])
	}
	if spans[1].kind != consoleStdout || text[spans[1].start:spans[1].end] != "saved\n" {
		t.Fatalf("stdout span = %#v %q", spans[1], text[spans[1].start:spans[1].end])
	}
	if spans[2].kind != consoleStderr || text[spans[2].start:spans[2].end] != "fatal: x\n" {
		t.Fatalf("stderr span = %#v %q", spans[2], text[spans[2].start:spans[2].end])
	}
	styles := consoleTextStyles(spans, true)
	fallback := color.NRGBA{R: 1, G: 2, B: 3, A: 4}
	out, uniform := styles.ColorInRange(spans[1].start, spans[1].end, fallback)
	if !uniform || out != fallback {
		t.Fatal("stdout should keep the normal text color")
	}
	cmd, uniform := styles.ColorInRange(spans[0].start, spans[0].end, fallback)
	if !uniform || cmd == fallback {
		t.Fatal("command should be colored")
	}
	errClr, uniform := styles.ColorInRange(spans[2].start, spans[2].end, fallback)
	if !uniform || errClr == fallback || errClr == cmd {
		t.Fatal("stderr should be a different color from the command")
	}
}

func TestGitConsoleLogDropsOldest(t *testing.T) {
	var log gitConsoleLog
	log.begin([]string{"add", "first"})
	log.finish("", "")
	for i := 0; i < gitConsoleLimit; i++ {
		log.begin([]string{"fetch", "origin"})
		log.finish("", "")
	}
	if log.count() != gitConsoleLimit {
		t.Fatalf("count = %d", log.count())
	}
	if strings.Contains(log.text(), "git add first") {
		t.Fatal("oldest command was kept")
	}
	if strings.Count(log.text(), "git fetch origin\n") != gitConsoleLimit {
		t.Fatalf("text = %q", log.text())
	}
}

func TestGitConsolePerTab(t *testing.T) {
	m := &GitModel{}
	m.ensure()
	m.console.begin([]string{"add", "-A"})
	m.console.finish("", "")
	other := &gitSession{owner: m}
	m.tabs = append(m.tabs, other)
	m.active = 1
	m.gitSession = other
	if m.ConsoleText() != "" {
		t.Fatalf("new tab = %q", m.ConsoleText())
	}
	m.active = 0
	m.gitSession = m.tabs[0]
	if !strings.Contains(m.ConsoleText(), "git add -A") {
		t.Fatalf("first tab = %q", m.ConsoleText())
	}
	m.tabs[0] = &gitSession{owner: m}
	m.gitSession = m.tabs[0]
	if m.ConsoleText() != "" {
		t.Fatal("closed tab kept its transcript")
	}
}

func TestConsoleScrollFollow(t *testing.T) {
	if consoleBottomOffset(100, 40) != 0 {
		t.Fatal("short content stays at the top")
	}
	if consoleBottomOffset(100, 250) != -150 {
		t.Fatal("bottom offset")
	}
	if !consoleAtBottom(0, 100, 40) {
		t.Fatal("short content is at the bottom")
	}
	if !consoleAtBottom(-150, 100, 250) {
		t.Fatal("scrolled to the end")
	}
	if consoleAtBottom(0, 100, 250) {
		t.Fatal("the top of a long transcript is not the bottom")
	}
	if !consoleAtBottom(-148, 100, 250) {
		t.Fatal("a couple of pixels still counts as the bottom")
	}
	if consoleAtBottom(-100, 100, 250) {
		t.Fatal("the middle should hold still")
	}
}

func TestGitViewBottoms(t *testing.T) {
	rest := image.Rect(0, 10, 400, 510)
	list, right, console := gitViewBottoms(rest, 24, false)
	if list != 510 || right != 510-12 || !console.Empty() {
		t.Fatalf("closed = %d %d %v", list, right, console)
	}
	list, right, console = gitViewBottoms(rest, 24, true)
	if console.Min.X != 0 || console.Max.X != 400 || console.Max.Y != 510 {
		t.Fatalf("console = %v", console)
	}
	if console.Dy() != gitConsoleHeight(24) {
		t.Fatalf("height = %d", console.Dy())
	}
	if list != console.Min.Y || right != console.Min.Y {
		t.Fatalf("columns = %d %d, console top %d", list, right, console.Min.Y)
	}
}

func TestConsoleFollowingDefault(t *testing.T) {
	var s gitSession
	if !s.ConsoleFollowing() {
		t.Fatal("a new tab follows the bottom")
	}
	s.NoteConsoleScroll(-100, 100, 250)
	if s.ConsoleFollowing() {
		t.Fatal("scrolling away should stop following")
	}
	if s.ConsoleOffset() != -100 {
		t.Fatalf("offset = %v", s.ConsoleOffset())
	}
	s.NoteConsoleScroll(-150, 100, 250)
	if !s.ConsoleFollowing() {
		t.Fatal("returning to the end should follow again")
	}
}
