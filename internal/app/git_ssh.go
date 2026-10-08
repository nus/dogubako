package app

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

func (t *GitTool) buildSSH(context *guigui.Context, adder *guigui.ChildAdder, lang i18n.Lang, model *GitModel) error {
	if model.SSHActive() && !t.sshWas {
		t.sshDestInput.SetValue("")
		t.sshPortInput.SetValue("")
		t.sshDirShown = ""
	}
	t.sshWas = model.SSHActive()
	if gen := model.SSHListGen(); gen != t.sshSeenGen {
		t.sshSeenGen = gen
		t.sshHostList.SelectItemByIndex(-1)
		t.sshDirList.SelectItemByIndex(-1)
	}

	t.sshConnected = model.SSHConnected()
	busy := model.SSHBusy()
	status := model.SSHStatus(lang)
	if status == "" && t.sshConnected && model.SSHIsRepo() {
		status = i18n.T(lang, i18n.GitSSHRepoMark)
	} else if status == "" && t.sshConnected && !busy && len(model.SSHEntries()) == 0 {
		status = i18n.T(lang, i18n.GitSSHEmptyDir)
	}
	t.sshStatusOn = status != ""
	if t.sshStatusOn {
		t.sshStatus.SetValue(status)
		t.sshStatus.SetMultiline(true)
		t.sshStatus.SetWrapMode(basicwidget.WrapModeNormal)
		t.sshStatus.SetSelectable(true)
		adder.AddWidget(&t.sshStatus)
	}

	if !t.sshConnected {
		t.sshDirShown = ""
		return t.buildSSHHosts(context, adder, lang, model, busy)
	}
	return t.buildSSHDirs(context, adder, lang, model, busy)
}

func (t *GitTool) buildSSHHosts(context *guigui.Context, adder *guigui.ChildAdder, lang i18n.Lang, model *GitModel, busy bool) error {
	setBoldText(&t.sshHostsTitle, true)
	t.sshHostsTitle.SetValue(i18n.T(lang, i18n.GitSSHHosts))
	t.sshHostsTitle.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	adder.AddWidget(&t.sshHostsTitle)

	hosts := model.SSHHosts()
	t.sshShowHosts = len(hosts) > 0
	if t.sshShowHosts {
		t.sshHostItems = t.sshHostItems[:0]
		for _, host := range hosts {
			t.sshHostItems = append(t.sshHostItems, basicwidget.ListItem[string]{Text: host, Value: host})
		}
		t.sshHostList.SetStyle(basicwidget.ListStyleNormal)
		t.sshHostList.SetHighlightVisibleWhenUnfocused(true)
		t.sshHostList.SetHoverBackgroundVisible(true)
		t.sshHostList.SetItems(t.sshHostItems)
		t.sshHostList.OnItemSelected(func(context *guigui.Context, index int) {
			item, ok := t.sshHostList.ItemByIndex(index)
			if !ok || item.Value == "" || model.SSHBusy() {
				return
			}
			t.sshDestInput.SetValue(item.Value)
			t.sshPortInput.SetValue("")
			model.SSHConnect(item.Value, "")
		})
		context.SetEnabled(&t.sshHostList, !busy)
		adder.AddWidget(&t.sshHostList)
	} else {
		t.sshNoHosts.SetValue(i18n.T(lang, i18n.GitSSHNoHosts))
		t.sshNoHosts.SetMultiline(true)
		t.sshNoHosts.SetWrapMode(basicwidget.WrapModeNormal)
		adder.AddWidget(&t.sshNoHosts)
	}

	t.sshDestLabel.SetValue(i18n.T(lang, i18n.GitSSHDest))
	t.sshDestLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	adder.AddWidget(&t.sshDestLabel)
	t.sshDestInput.SetPlaceholder("user@host")
	t.sshDestInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			model.SSHConnect(text, t.sshPortInput.Value())
		}
	})
	context.SetEnabled(&t.sshDestInput, !busy)
	adder.AddWidget(&t.sshDestInput)

	t.sshPortLabel.SetValue(i18n.T(lang, i18n.GitSSHPort))
	t.sshPortLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	adder.AddWidget(&t.sshPortLabel)
	t.sshPortInput.SetPlaceholder("22")
	t.sshPortInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			model.SSHConnect(t.sshDestInput.Value(), text)
		}
	})
	context.SetEnabled(&t.sshPortInput, !busy)
	adder.AddWidget(&t.sshPortInput)

	t.sshConnectBtn.SetText(i18n.T(lang, i18n.GitSSHConnect))
	t.sshConnectBtn.SetType(basicwidget.ButtonTypePrimary)
	t.sshConnectBtn.OnDown(func(context *guigui.Context) {
		model.SSHConnect(t.sshDestInput.Value(), t.sshPortInput.Value())
	})
	context.SetEnabled(&t.sshConnectBtn, !busy)
	adder.AddWidget(&t.sshConnectBtn)

	t.sshCancelBtn.SetText(i18n.T(lang, i18n.GitCancel))
	t.sshCancelBtn.OnDown(func(context *guigui.Context) {
		model.CancelSSH()
	})
	adder.AddWidget(&t.sshCancelBtn)
	return nil
}

func (t *GitTool) buildSSHDirs(context *guigui.Context, adder *guigui.ChildAdder, lang i18n.Lang, model *GitModel, busy bool) error {
	setBoldText(&t.sshTarget, true)
	t.sshTarget.SetValue(model.SSHTarget())
	t.sshTarget.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	adder.AddWidget(&t.sshTarget)

	t.sshDestLabel.SetValue(i18n.T(lang, i18n.GitSSHPickDir))
	t.sshDestLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	adder.AddWidget(&t.sshDestLabel)

	dir := model.SSHDir()
	if dir != t.sshDirShown {
		t.sshDirShown = dir
		t.sshPathInput.SetValue(dir)
	}
	t.sshPathInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			model.SSHGo(text)
		}
	})
	context.SetEnabled(&t.sshPathInput, !busy)
	adder.AddWidget(&t.sshPathInput)

	t.sshUpBtn.SetText(i18n.T(lang, i18n.GitSSHUp))
	t.sshUpBtn.OnDown(func(context *guigui.Context) {
		model.SSHUp()
	})
	context.SetEnabled(&t.sshUpBtn, !busy && model.SSHCanUp())
	adder.AddWidget(&t.sshUpBtn)

	t.sshOpenDirBtn.SetText(i18n.T(lang, i18n.GitSSHOpenHere))
	t.sshOpenDirBtn.SetType(basicwidget.ButtonTypePrimary)
	t.sshOpenDirBtn.OnDown(func(context *guigui.Context) {
		model.SSHChoose()
	})
	context.SetEnabled(&t.sshOpenDirBtn, !busy && model.SSHDir() != "")
	adder.AddWidget(&t.sshOpenDirBtn)

	t.sshBackBtn.SetText(i18n.T(lang, i18n.GitSSHBack))
	t.sshBackBtn.OnDown(func(context *guigui.Context) {
		model.SSHBack()
	})
	context.SetEnabled(&t.sshBackBtn, !busy)
	adder.AddWidget(&t.sshBackBtn)

	entries := model.SSHEntries()
	t.sshDirItems = t.sshDirItems[:0]
	for _, e := range entries {
		t.sshDirItems = append(t.sshDirItems, basicwidget.ListItem[string]{Text: e.Name, Value: e.Name})
	}
	t.sshDirList.SetStyle(basicwidget.ListStyleNormal)
	t.sshDirList.SetHighlightVisibleWhenUnfocused(true)
	t.sshDirList.SetHoverBackgroundVisible(true)
	t.sshDirList.SetItems(t.sshDirItems)
	t.sshDirList.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := t.sshDirList.ItemByIndex(index)
		if !ok || item.Value == "" || model.SSHBusy() {
			return
		}
		model.SSHEnter(item.Value)
	})
	context.SetEnabled(&t.sshDirList, !busy)
	adder.AddWidget(&t.sshDirList)
	return nil
}

func (t *GitTool) layoutSSH(context *guigui.Context, bounds image.Rectangle, layouter *guigui.ChildLayouter, u int) {
	pad := u / 2
	gap := u / 3
	x0 := bounds.Min.X + pad
	x1 := bounds.Max.X - pad
	if x1 < x0+8*u {
		x1 = x0 + 8*u
	}
	w := x1 - x0
	y := bounds.Min.Y + pad
	bottom := bounds.Max.Y - pad

	statusH := 0
	if t.sshStatusOn {
		statusH = t.sshStatus.Measure(context, guigui.FixedWidthConstraints(w)).Y
	}
	if !t.sshConnected {
		t.layoutSSHHosts(context, layouter, u, gap, x0, x1, w, y, bottom, statusH)
		return
	}
	t.layoutSSHDirs(context, layouter, u, gap, x0, x1, w, y, bottom, statusH)
}

func (t *GitTool) layoutSSHHosts(context *guigui.Context, layouter *guigui.ChildLayouter, u, gap, x0, x1, w, y, bottom, statusH int) {
	titleH := t.sshHostsTitle.Measure(context, guigui.FixedWidthConstraints(w)).Y
	layouter.LayoutWidget(&t.sshHostsTitle, image.Rect(x0, y, x1, y+titleH))
	y += titleH + gap

	inputH := t.sshDestInput.Measure(context, guigui.FixedWidthConstraints(w)).Y
	if inputH < u {
		inputH = u
	}
	labelH := t.sshDestLabel.Measure(context, guigui.Constraints{}).Y
	btnH := t.sshConnectBtn.Measure(context, guigui.Constraints{}).Y
	if t.sshCancelBtn.Measure(context, guigui.Constraints{}).Y > btnH {
		btnH = t.sshCancelBtn.Measure(context, guigui.Constraints{}).Y
	}
	rowH := btnH
	if inputH > rowH {
		rowH = inputH
	}
	form := labelH + gap + inputH + gap + rowH
	if statusH > 0 {
		form += gap + statusH
	}
	listBottom := bottom - form - gap
	if t.sshShowHosts {
		if listBottom < y+2*u {
			listBottom = y + 2*u
		}
		layouter.LayoutWidget(&t.sshHostList, image.Rect(x0, y, x1, listBottom))
		y = listBottom + gap
	} else {
		noH := t.sshNoHosts.Measure(context, guigui.FixedWidthConstraints(w)).Y
		layouter.LayoutWidget(&t.sshNoHosts, image.Rect(x0, y, x1, y+noH))
		y += noH + gap
	}

	layouter.LayoutWidget(&t.sshDestLabel, image.Rect(x0, y, x1, y+labelH))
	y += labelH + gap
	layouter.LayoutWidget(&t.sshDestInput, image.Rect(x0, y, x1, y+inputH))
	y += inputH + gap

	portLabelW := t.sshPortLabel.Measure(context, guigui.Constraints{}).X
	portW := 5 * u
	connW := t.sshConnectBtn.Measure(context, guigui.Constraints{}).X
	cancelW := t.sshCancelBtn.Measure(context, guigui.Constraints{}).X
	x := x0
	layouter.LayoutWidget(&t.sshPortLabel, image.Rect(x, y, x+portLabelW, y+rowH))
	x += portLabelW + gap
	connX := x1 - cancelW - gap - connW
	portEnd := x + portW
	if portEnd > connX-gap {
		portEnd = connX - gap
	}
	if portEnd < x {
		portEnd = x
	}
	layouter.LayoutWidget(&t.sshPortInput, image.Rect(x, y, portEnd, y+rowH))
	layouter.LayoutWidget(&t.sshCancelBtn, image.Rect(x1-cancelW, y, x1, y+rowH))
	layouter.LayoutWidget(&t.sshConnectBtn, image.Rect(connX, y, connX+connW, y+rowH))
	y += rowH + gap
	if t.sshStatusOn {
		layouter.LayoutWidget(&t.sshStatus, image.Rect(x0, y, x1, y+statusH))
	}
}

func (t *GitTool) layoutSSHDirs(context *guigui.Context, layouter *guigui.ChildLayouter, u, gap, x0, x1, w, y, bottom, statusH int) {
	titleH := t.sshTarget.Measure(context, guigui.FixedWidthConstraints(w)).Y
	layouter.LayoutWidget(&t.sshTarget, image.Rect(x0, y, x1, y+titleH))
	y += titleH + gap

	labelH := t.sshDestLabel.Measure(context, guigui.Constraints{}).Y
	layouter.LayoutWidget(&t.sshDestLabel, image.Rect(x0, y, x1, y+labelH))
	y += labelH + gap

	inputH := t.sshPathInput.Measure(context, guigui.FixedWidthConstraints(w)).Y
	if inputH < u {
		inputH = u
	}
	layouter.LayoutWidget(&t.sshPathInput, image.Rect(x0, y, x1, y+inputH))
	y += inputH + gap

	btnH := t.sshOpenDirBtn.Measure(context, guigui.Constraints{}).Y
	upW := t.sshUpBtn.Measure(context, guigui.Constraints{}).X
	openW := t.sshOpenDirBtn.Measure(context, guigui.Constraints{}).X
	backW := t.sshBackBtn.Measure(context, guigui.Constraints{}).X
	layouter.LayoutWidget(&t.sshUpBtn, image.Rect(x0, y, x0+upW, y+btnH))
	layouter.LayoutWidget(&t.sshOpenDirBtn, image.Rect(x0+upW+gap, y, x0+upW+gap+openW, y+btnH))
	layouter.LayoutWidget(&t.sshBackBtn, image.Rect(x1-backW, y, x1, y+btnH))
	y += btnH + gap

	listBottom := bottom
	if statusH > 0 {
		listBottom = bottom - statusH - gap
	}
	if listBottom < y+2*u {
		listBottom = y + 2*u
	}
	layouter.LayoutWidget(&t.sshDirList, image.Rect(x0, y, x1, listBottom))
	if t.sshStatusOn {
		sy := listBottom + gap
		layouter.LayoutWidget(&t.sshStatus, image.Rect(x0, sy, x1, sy+statusH))
	}
}
