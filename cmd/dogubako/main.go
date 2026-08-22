package main

import (
	"fmt"
	"image"
	"os"

	"github.com/guigui-gui/guigui"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nus/dogubako/internal/app"
	"github.com/nus/dogubako/internal/appicon"
	_ "github.com/nus/dogubako/internal/cjkembed"
	"github.com/nus/dogubako/internal/i18n"
)

func main() {
	if img, err := appicon.RGBA(); err == nil {
		ebiten.SetWindowIcon([]image.Image{img})
	}
	op := &guigui.RunOptions{
		Title:         i18n.T(i18n.Load(), i18n.AppTitle),
		WindowSize:    image.Pt(1100, 760),
		WindowMinSize: image.Pt(800, 560),
	}
	if err := guigui.Run(&app.Root{}, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
