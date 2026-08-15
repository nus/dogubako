package app

import "github.com/guigui-gui/guigui/basicwidget"

func setBoldText(t *basicwidget.Text, bold bool) {
	var style basicwidget.TextStyle
	style.SetBold(bold)
	t.SetBaseStyle(&style)
}
