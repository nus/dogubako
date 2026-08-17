// memstat runs the Guigui UI for a few frames and prints heap / RSS.
//
//	go run ./cmd/memstat
//	go run -tags nocjk ./cmd/memstat -widget=empty
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nus/dogubako/internal/app"
	"github.com/nus/dogubako/internal/memstat"
)

func main() {
	widget := flag.String("widget", "app", "root widget: app (dogubako) or empty (background only)")
	frames := flag.Int("frames", 30, "GUI frames to run before measuring; 0 skips the window")
	heapPath := flag.String("heap", "", "write a pprof heap profile after the last snapshot")
	asJSON := flag.Bool("json", false, "print snapshots as JSON")
	noGC := flag.Bool("no-gc", false, "do not GC before capturing snapshots")
	flag.Parse()

	gc := !*noGC
	var snaps []memstat.Snapshot

	snaps = append(snaps, memstat.Capture("after init", gc))

	var runErr error
	if *frames > 0 {
		root, err := newRoot(*widget, *frames, func() {
			snaps = append(snaps, memstat.Capture(fmt.Sprintf("after %d frames", *frames), gc))
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		op := &guigui.RunOptions{
			Title:         "dogubako memstat",
			WindowSize:    image.Pt(1100, 760),
			WindowMinSize: image.Pt(800, 560),
		}
		runErr = guigui.Run(root, op)
	}

	meta := report{
		Widget:   *widget,
		CJKFont:  cjkEnabled,
		Frames:   *frames,
		GOOS:     runtime.GOOS,
		GOARCH:   runtime.GOARCH,
		Snapshot: snaps,
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(meta); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("dogubako memstat  widget=%s  cjkfont=%v  frames=%d\n", meta.Widget, meta.CJKFont, meta.Frames)
		for _, s := range snaps {
			fmt.Print(memstat.Format(s))
		}
	}

	if *heapPath != "" {
		if err := writeHeap(*heapPath); err != nil {
			fmt.Fprintln(os.Stderr, "heap profile:", err)
			os.Exit(1)
		}
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

type report struct {
	Widget   string             `json:"widget"`
	CJKFont  bool               `json:"cjkfont"`
	Frames   int                `json:"frames"`
	GOOS     string             `json:"goos"`
	GOARCH   string             `json:"goarch"`
	Snapshot []memstat.Snapshot `json:"snapshots"`
}

func newRoot(widget string, frames int, done func()) (guigui.Widget, error) {
	switch widget {
	case "app":
		return &measureRoot{frames: frames, done: done}, nil
	case "empty":
		return &emptyRoot{frames: frames, done: done}, nil
	default:
		return nil, fmt.Errorf("unknown -widget %q (want app or empty)", widget)
	}
}

func writeHeap(path string) error {
	runtime.GC()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.WriteHeapProfile(f)
}

type measureRoot struct {
	app.Root
	frames int
	ticks  int
	done   func()
}

func (r *measureRoot) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if err := r.Root.Tick(context, widgetBounds); err != nil {
		return err
	}
	return tickAndStop(&r.ticks, r.frames, r.done)
}

type emptyRoot struct {
	guigui.DefaultWidget
	background basicwidget.Background
	frames     int
	ticks      int
	done       func()
}

func (e *emptyRoot) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&e.background)
	return nil
}

func (e *emptyRoot) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&e.background, widgetBounds.Bounds())
}

func (e *emptyRoot) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	return tickAndStop(&e.ticks, e.frames, e.done)
}

func tickAndStop(ticks *int, frames int, done func()) error {
	*ticks++
	if *ticks < frames {
		return nil
	}
	if done != nil {
		done()
	}
	return ebiten.Termination
}
