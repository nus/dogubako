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

func TestConsoleControlAt(t *testing.T) {
	button := image.Rect(360, 480, 384, 498)
	tip := image.Rect(300, 450, 420, 472)
	if !consoleControlAt(image.Pt(370, 488), button, tip, true) {
		t.Fatal("the console icon is not a commit click")
	}
	if !consoleControlAt(image.Pt(340, 460), button, tip, true) {
		t.Fatal("the close-console balloon is not a commit click")
	}
	if consoleControlAt(image.Pt(340, 460), button, tip, false) {
		t.Fatal("a closed balloon must not keep covering the commit")
	}
	if consoleControlAt(image.Pt(20, 200), button, tip, true) {
		t.Fatal("a commit away from the console control stays clickable")
	}
}

func TestCommitOpenFromClick(t *testing.T) {
	if open, load := commitOpenFromClick("keep", "behind", true, false); open != "keep" || load {
		t.Fatalf("console control open=%q load=%v", open, load)
	}
	if open, load := commitOpenFromClick("", "behind", false, false); open != "behind" || !load {
		t.Fatalf("row click open=%q load=%v", open, load)
	}
	if open, load := commitOpenFromClick("behind", "behind", false, false); open != "" || load {
		t.Fatalf("second click open=%q load=%v", open, load)
	}
	if open, load := commitOpenFromClick("keep", "behind", false, true); open != "keep" || load {
		t.Fatalf("list already opened open=%q load=%v", open, load)
	}
	if open, load := commitOpenFromClick("keep", "", false, false); open != "keep" || load {
		t.Fatalf("balloon only open=%q load=%v", open, load)
	}
}

func TestConsoleClickKeepsCommit(t *testing.T) {
	open, console := consoleClickKeepsCommit("abc", false, true)
	if open != "abc" || console {
		t.Fatalf("icon click open=%q console=%v", open, console)
	}
	open, console = consoleClickKeepsCommit("abc", true, false)
	if open != "abc" || console {
		t.Fatalf("balloon click open=%q console=%v", open, console)
	}
	open, console = consoleClickKeepsCommit("", false, false)
	if open != "" || !console {
		t.Fatalf("show balloon open=%q console=%v", open, console)
	}
}

func TestGitViewBottoms(t *testing.T) {
	rest := image.Rect(0, 10, 400, 510)
	list, right, split, console := gitViewBottoms(rest, 24, false, 0.8)
	if list != 510 || right != 510-12 || !split.Empty() || !console.Empty() {
		t.Fatalf("closed = %d %d %v %v", list, right, split, console)
	}
	list, right, split, console = gitViewBottoms(rest, 24, true, 0)
	if console.Min.X != 0 || console.Max.X != 400 || console.Max.Y != 510 {
		t.Fatalf("console = %v", console)
	}
	if console.Dy() != gitConsoleHeight(24) {
		t.Fatalf("height = %d", console.Dy())
	}
	if split.Dy() != gitWorkSplitWidth(24) || split.Max.Y != console.Min.Y || split.Min.X != console.Min.X || split.Max.X != console.Max.X {
		t.Fatalf("boundary = %v, console top %d", split, console.Min.Y)
	}
	if list != split.Min.Y || right != split.Min.Y {
		t.Fatalf("columns = %d %d, boundary top %d", list, right, split.Min.Y)
	}
	if list != console.Min.Y-split.Dy() {
		t.Fatalf("columns should be shorter by the boundary, list %d console %d", list, console.Min.Y)
	}
}

func TestGitConsoleFixedHeight(t *testing.T) {
	if gitConsoleFixedHeight(500, 24) != gitConsoleHeight(24) {
		t.Fatal("a tall window keeps about five lines")
	}
	rest := image.Rect(0, 0, 200, 120)
	_, _, split, console := gitViewBottoms(rest, 24, true, 0)
	if console.Dy() != 120*2/3 {
		t.Fatalf("short window = %d", console.Dy())
	}
	if split.Dy() != gitWorkSplitWidth(24) || split.Max.Y != console.Min.Y {
		t.Fatalf("short boundary = %v", split)
	}
	_, _, _, tight := gitViewBottoms(image.Rect(0, 0, 100, 40), 24, true, 0)
	if tight.Dy() != 40*2/3 || tight.Dy() >= gitConsoleMinHeight(24) {
		t.Fatalf("fixed short height = %d", tight.Dy())
	}
}

func TestGitConsoleDragHeight(t *testing.T) {
	const u = 24
	split := gitWorkSplitWidth(u)
	minConsole := gitConsoleMinHeight(u)
	minUpper := gitHistMinPane(u)
	if minConsole != u*3/4+2*(u/4) {
		t.Fatalf("one line = %d", minConsole)
	}
	avail := 600 - split
	if got := gitConsoleDragHeight(600, split, 0.5, minConsole, minUpper); got != int(float64(avail)*0.5) {
		t.Fatalf("half = %d", got)
	}
	grown := 900 - split
	if got := gitConsoleDragHeight(900, split, 0.5, minConsole, minUpper); got != int(float64(grown)*0.5) {
		t.Fatalf("resized half = %d", got)
	}
	if got := gitConsoleDragHeight(600, split, 0.01, minConsole, minUpper); got != minConsole {
		t.Fatalf("min = %d", got)
	}
	if got := gitConsoleDragHeight(600, split, 0.99, minConsole, minUpper); got != 600-split-minUpper {
		t.Fatalf("max = %d", got)
	}
	if got := gitConsoleDragHeight(600, split, 0.8, minConsole, minUpper); got <= 600*2/3 {
		t.Fatalf("dragged %d should pass two thirds", got)
	}
}

func TestGitConsoleDragFollowsPointer(t *testing.T) {
	rest := image.Rect(0, 100, 500, 700)
	_, _, split, console := gitViewBottoms(rest, 24, true, 0)
	line := split.Min.Y + split.Dy()/2
	up := gitConsoleFracFromY(rest, line-80, 24)
	_, _, _, grown := gitViewBottoms(rest, 24, true, up)
	if grown.Dy() <= console.Dy() {
		t.Fatalf("up = %d, was %d", grown.Dy(), console.Dy())
	}
	down := gitConsoleFracFromY(rest, line+40, 24)
	_, _, _, shrunk := gitViewBottoms(rest, 24, true, down)
	if shrunk.Dy() >= console.Dy() {
		t.Fatalf("down = %d, was %d", shrunk.Dy(), console.Dy())
	}
	if shrunk.Dy() < gitConsoleMinHeight(24) {
		t.Fatalf("shrunk below one line: %d", shrunk.Dy())
	}
	top := gitConsoleFracFromY(rest, rest.Min.Y, 24)
	_, _, _, wide := gitViewBottoms(rest, 24, true, top)
	if wide.Dy() <= rest.Dy()*2/3 {
		t.Fatalf("tall drag = %d", wide.Dy())
	}
	// The same share is what history and the commit screen both lay out.
	small := image.Rect(0, 0, 400, 360)
	_, _, _, kept := gitViewBottoms(small, 24, true, top)
	if kept.Dy() <= small.Dy()*2/3 {
		t.Fatalf("shrunk window = %d, two thirds %d", kept.Dy(), small.Dy()*2/3)
	}
	upper := small.Dy() - gitWorkSplitWidth(24) - kept.Dy()
	if upper < gitHistMinPane(24) {
		t.Fatalf("upper pane = %d", upper)
	}
	var tool GitTool
	_, _, freshSplit, fresh := gitViewBottoms(rest, 24, true, tool.consoleFrac)
	if fresh.Dy() != gitConsoleHeight(24) || freshSplit.Empty() {
		t.Fatalf("new launch = %v %v", fresh, freshSplit)
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
