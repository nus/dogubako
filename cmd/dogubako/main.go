package main

import (
	"fmt"
	"image"
	"os"

	"github.com/guigui-gui/guigui"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nus/dogubako/internal/app"
	"github.com/nus/dogubako/internal/appicon"
	"github.com/nus/dogubako/internal/appimage"
	_ "github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/i18n"
)

func main() {
	args := os.Args[1:]
	if code, handled := appimage.Dispatch(args, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	launch, code, openWindow := app.ParseLaunch(args, os.Stdout, os.Stderr)
	if !openWindow {
		os.Exit(code)
	}
	if img, err := appicon.RGBA(); err == nil {
		ebiten.SetWindowIcon([]image.Image{img})
	}
	op := &guigui.RunOptions{
		Title:         i18n.T(i18n.Load(), i18n.AppTitle),
		WindowSize:    image.Pt(1100, 760),
		WindowMinSize: image.Pt(800, 560),
	}
	root := &app.Root{}
	root.ApplyLaunch(launch)
	if err := guigui.RunWithCustomFunc(root, op, app.RunGame); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
