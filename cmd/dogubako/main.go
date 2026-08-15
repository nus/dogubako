package main

import (
	"fmt"
	"image"
	"os"

	"github.com/guigui-gui/guigui"
	_ "github.com/guigui-gui/guigui/basicwidget/cjkfont"

	"github.com/nus/dogubako/internal/app"
)

func main() {
	op := &guigui.RunOptions{
		Title:         "道具箱",
		WindowSize:    image.Pt(1100, 760),
		WindowMinSize: image.Pt(800, 560),
	}
	if err := guigui.Run(&app.Root{}, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
