package app

import (
	"image"
	"strings"
	"time"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitDetail is the bottom panel: commit details and the change list, on separate tabs.
type gitDetail struct {
	guigui.DefaultWidget

	detailsBtn basicwidget.Button
	changesBtn basicwidget.Button
	panel      basicwidget.Panel
	body       gitDetailBody
	changes    gitChanges
	seen       string
	tab        int

	tabItems    []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
	tabRow      guigui.LinearLayout
}

func (d *gitDetail) Set(lang i18n.Lang, model *GitModel, onMenu func(gitcli.Ref), split bool, onSplit func(bool)) {
	hash := ""
	if model.HasRepo() {
		hash = model.Detail().Hash
	}
	if hash != d.seen {
		d.seen = hash
		d.panel.ForceSetScrollOffset(0, 0)
	}
	d.detailsBtn.SetText(i18n.T(lang, i18n.GitDetails))
	d.changesBtn.SetText(i18n.T(lang, i18n.GitChangeContent))
	d.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	if d.tab == 0 {
		d.detailsBtn.SetType(basicwidget.ButtonTypePrimary)
		d.changesBtn.SetType(basicwidget.ButtonTypeNormal)
	} else {
		d.detailsBtn.SetType(basicwidget.ButtonTypeNormal)
		d.changesBtn.SetType(basicwidget.ButtonTypePrimary)
	}
	d.body.Set(lang, model, onMenu)
	d.changes.Set(lang, model, split, onSplit)
}

func (d *gitDetail) selectTab(tab int) {
	if d.tab == tab {
		return
	}
	d.tab = tab
	d.panel.ForceSetScrollOffset(0, 0)
	guigui.RequestRebuild()
}

func (d *gitDetail) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&d.detailsBtn)
	adder.AddWidget(&d.changesBtn)
	if d.tab == 1 {
		adder.AddWidget(&d.changes)
	} else {
		adder.AddWidget(&d.panel)
	}
	d.detailsBtn.OnDown(func(context *guigui.Context) { d.selectTab(0) })
	d.changesBtn.OnDown(func(context *guigui.Context) { d.selectTab(1) })
	d.panel.SetContent(&d.body)
	d.panel.SetAutoBorder(false)
	d.panel.SetBorders(basicwidget.PanelBorders{})
	d.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	return nil
}

func (d *gitDetail) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	d.tabItems = d.tabItems[:0]
	d.tabItems = append(d.tabItems,
		guigui.LinearLayoutItem{Widget: &d.detailsBtn},
		guigui.LinearLayoutItem{Widget: &d.changesBtn},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
	)
	d.tabRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: d.tabItems, Gap: u / 4}
	body := guigui.Widget(&d.panel)
	if d.tab == 1 {
		body = &d.changes
	}
	d.layoutItems = d.layoutItems[:0]
	d.layoutItems = append(d.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &d.tabRow},
		guigui.LinearLayoutItem{Widget: body, Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     d.layoutItems,
		Gap:       u / 8,
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// gitDetailBody is the commit panel content: author, refs, hash, parents, and message.
type gitDetailBody struct {
	guigui.DefaultWidget

	extra        basicwidget.Text
	authorLabel  basicwidget.Text
	authorText   basicwidget.Text
	whenText     basicwidget.Text
	refsLabel    basicwidget.Text
	badges       gitRefBadges
	shaLabel     basicwidget.Text
	hashText     basicwidget.Text
	parentsLabel basicwidget.Text
	parentsText  basicwidget.Text
	body         basicwidget.Text

	showExtra   bool
	showCommit  bool
	showRefs    bool
	showParents bool

	parentHashes []string
	parentRanges []basicwidget.TextRange
	onParent     func(string)

	authorItems []guigui.LinearLayoutItem
	refsItems   []guigui.LinearLayoutItem
	shaItems    []guigui.LinearLayoutItem
	parentItems []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
	authorRow   guigui.LinearLayout
	refsRow     guigui.LinearLayout
	shaRow      guigui.LinearLayout
	parentRow   guigui.LinearLayout
}

func (d *gitDetailBody) Set(lang i18n.Lang, model *GitModel, onMenu func(gitcli.Ref)) {
	d.authorLabel.SetValue(i18n.T(lang, i18n.GitAuthor))
	d.refsLabel.SetValue(i18n.T(lang, i18n.GitRefs))
	d.shaLabel.SetValue(i18n.T(lang, i18n.GitSHA))
	d.parentsLabel.SetValue(i18n.T(lang, i18n.GitParents))

	d.showExtra = false
	d.showCommit = false
	d.showRefs = false
	d.showParents = false
	if !model.HasRepo() {
		d.extra.SetValue("")
		return
	}

	if model.Snapshot().Status.Detached {
		d.extra.SetValue(i18n.T(lang, i18n.GitDetached))
		d.showExtra = true
	}

	detail := model.Detail()
	if detail.Hash == "" {
		return
	}
	d.showCommit = true
	author := detail.Author
	if detail.Email != "" {
		if author != "" {
			author += " "
		}
		author += "<" + detail.Email + ">"
	}
	d.authorText.SetValue(author)
	if detail.When.IsZero() {
		d.whenText.SetValue("")
	} else {
		d.whenText.SetValue(detail.When.Local().Format(time.RFC1123))
	}
	d.badges.Set(detail.Decorations, detail.Hash, nil, onMenu)
	d.showRefs = d.badges.Len() > 0
	d.hashText.SetValue(detail.Hash)
	d.onParent = func(hash string) { model.RevealCommit(hash) }
	d.setParents(detail.Parents)
	body := strings.TrimSpace(detail.Body)
	if body == "" {
		body = detail.Subject
	}
	d.body.SetValue(body)
}

func (d *gitDetailBody) setParents(parents []string) {
	d.parentHashes = d.parentHashes[:0]
	d.parentRanges = d.parentRanges[:0]
	var b strings.Builder
	for _, p := range parents {
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("  ")
		}
		short := shortHash(p)
		start := b.Len()
		b.WriteString(short)
		d.parentRanges = append(d.parentRanges, basicwidget.TextRange{StartInBytes: start, EndInBytes: b.Len()})
		d.parentHashes = append(d.parentHashes, p)
	}
	d.parentsText.SetValue(b.String())
	d.showParents = len(d.parentHashes) > 0
}

func shortHash(h string) string {
	if len(h) >= 7 {
		return h[:7]
	}
	return h
}

func (d *gitDetailBody) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if d.showExtra {
		d.extra.SetWrapMode(basicwidget.WrapModeNone)
		d.extra.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		adder.AddWidget(&d.extra)
	}
	if !d.showCommit {
		return nil
	}
	adder.AddWidget(&d.authorLabel)
	adder.AddWidget(&d.authorText)
	adder.AddWidget(&d.whenText)
	if d.showRefs {
		adder.AddWidget(&d.refsLabel)
		adder.AddWidget(&d.badges)
	}
	adder.AddWidget(&d.shaLabel)
	adder.AddWidget(&d.hashText)
	if d.showParents {
		adder.AddWidget(&d.parentsLabel)
		adder.AddWidget(&d.parentsText)
	}
	adder.AddWidget(&d.body)

	setBoldText(&d.authorLabel, true)
	setBoldText(&d.refsLabel, true)
	setBoldText(&d.shaLabel, true)
	setBoldText(&d.parentsLabel, true)
	setBoldText(&d.authorText, true)

	for _, text := range []*basicwidget.Text{&d.authorLabel, &d.authorText, &d.whenText, &d.refsLabel, &d.shaLabel, &d.hashText, &d.parentsLabel, &d.parentsText} {
		text.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		text.SetWrapMode(basicwidget.WrapModeNone)
	}
	d.whenText.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	d.whenText.SetEllipsisString("…")
	d.hashText.SetSelectable(true)
	if d.showParents {
		var styles basicwidget.TextStyles
		link, _ := basicwidgetdraw.BorderAccentColors(context.ColorMode(), basicwidgetdraw.RoundedRectBorderTypeRegular)
		for _, r := range d.parentRanges {
			styles.SetColorInRange(r.StartInBytes, r.EndInBytes, link)
			styles.SetUnderlineInRange(r.StartInBytes, r.EndInBytes, true)
		}
		d.parentsText.SetOverrideStyles(&styles, false)
		d.parentsText.SetHotspotRanges(d.parentRanges)
		d.parentsText.OnHotspotUp(func(context *guigui.Context, r basicwidget.TextRange) {
			for i, hr := range d.parentRanges {
				if hr.StartInBytes != r.StartInBytes || hr.EndInBytes != r.EndInBytes {
					continue
				}
				if d.onParent != nil && i < len(d.parentHashes) {
					d.onParent(d.parentHashes[i])
				}
				return
			}
		})
	}
	d.parentsText.SetSelectable(true)
	d.body.SetMultiline(true)
	d.body.SetWrapMode(basicwidget.WrapModeNormal)
	d.body.SetSelectable(true)
	return nil
}

func (d *gitDetailBody) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	d.contentLayout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (d *gitDetailBody) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return d.contentLayout(context).Measure(context, constraints)
}

func (d *gitDetailBody) contentLayout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	labelW := 4 * u
	d.layoutItems = d.layoutItems[:0]
	if d.showExtra {
		d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Widget: &d.extra, Size: guigui.FixedSize(u)})
	}
	if d.showCommit {
		d.authorItems = d.authorItems[:0]
		d.authorItems = append(d.authorItems,
			guigui.LinearLayoutItem{Widget: &d.authorLabel, Size: guigui.FixedSize(labelW)},
			guigui.LinearLayoutItem{Widget: &d.authorText, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &d.whenText, Size: guigui.FixedSize(12 * u)},
		)
		d.authorRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: d.authorItems, Gap: u / 4}
		d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &d.authorRow})

		if d.showRefs {
			d.refsItems = d.refsItems[:0]
			d.refsItems = append(d.refsItems,
				guigui.LinearLayoutItem{Widget: &d.refsLabel, Size: guigui.FixedSize(labelW)},
				guigui.LinearLayoutItem{Widget: &d.badges, Size: guigui.FlexibleSize(1)},
			)
			d.refsRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: d.refsItems, Gap: u / 4}
			d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &d.refsRow})
		}

		d.shaItems = d.shaItems[:0]
		d.shaItems = append(d.shaItems,
			guigui.LinearLayoutItem{Widget: &d.shaLabel, Size: guigui.FixedSize(labelW)},
			guigui.LinearLayoutItem{Widget: &d.hashText, Size: guigui.FlexibleSize(1)},
		)
		d.shaRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: d.shaItems, Gap: u / 4}
		d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &d.shaRow})

		if d.showParents {
			d.parentItems = d.parentItems[:0]
			d.parentItems = append(d.parentItems,
				guigui.LinearLayoutItem{Widget: &d.parentsLabel, Size: guigui.FixedSize(labelW)},
				guigui.LinearLayoutItem{Widget: &d.parentsText, Size: guigui.FlexibleSize(1)},
			)
			d.parentRow = guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: d.parentItems, Gap: u / 4}
			d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &d.parentRow})
		}

		d.layoutItems = append(d.layoutItems, guigui.LinearLayoutItem{Widget: &d.body})
	}
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     d.layoutItems,
		Gap:       u / 8,
	}
}
