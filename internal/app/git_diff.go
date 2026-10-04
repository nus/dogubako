package app

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/i18n"
)

// gitFileDiff is one file section of a unified diff.
type gitFileDiff struct {
	Name   string
	Status string
	Text   string
}

// gitChanges is the changes tab: a file list beside the colored diff.
type gitChanges struct {
	guigui.DefaultWidget

	list  basicwidget.List[int]
	panel basicwidget.Panel
	diff  basicwidget.Text
	note  basicwidget.Text

	files    []gitFileDiff
	items    []basicwidget.ListItem[int]
	selected int
	seen     string
	message  string
}

func (c *gitChanges) Set(lang i18n.Lang, model *GitModel) {
	c.message = ""
	c.files = nil
	if !model.HasRepo() {
		c.message = i18n.T(lang, i18n.GitEmpty)
		c.note.SetValue(c.message)
		return
	}
	detail := model.Detail()
	if detail.Hash == "" {
		c.message = i18n.T(lang, i18n.GitNoCommits)
		c.note.SetValue(c.message)
		return
	}
	if detail.Hash != c.seen {
		c.seen = detail.Hash
		c.selected = 0
		c.panel.ForceSetScrollOffset(0, 0)
	}
	c.files = splitUnifiedDiff(detail.Diff)
	if len(c.files) == 0 {
		c.message = i18n.T(lang, i18n.GitNoChanges)
		c.note.SetValue(c.message)
		return
	}
	if c.selected < 0 || c.selected >= len(c.files) {
		c.selected = 0
	}
	c.items = c.items[:0]
	for i, f := range c.files {
		c.items = append(c.items, basicwidget.ListItem[int]{
			Text:  f.Status + "  " + f.Name,
			Value: i,
		})
	}
	c.list.SetItems(c.items)
	c.list.SelectItemByIndex(c.selected)
}

func (c *gitChanges) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if c.message != "" {
		c.note.SetMultiline(true)
		c.note.SetWrapMode(basicwidget.WrapModeNormal)
		c.note.SetSelectable(true)
		adder.AddWidget(&c.note)
		return nil
	}
	adder.AddWidget(&c.list)
	adder.AddWidget(&c.panel)
	c.list.SetStyle(basicwidget.ListStyleNormal)
	c.list.SetHighlightVisibleWhenUnfocused(true)
	c.list.SetSelectedItemBold(false)
	c.list.OnItemSelected(func(context *guigui.Context, index int) {
		if index < 0 || index >= len(c.files) || index == c.selected {
			return
		}
		c.selected = index
		c.panel.ForceSetScrollOffset(0, 0)
		guigui.RequestRebuild()
	})
	c.panel.SetContent(&c.diff)
	c.panel.SetAutoBorder(false)
	c.panel.SetBorders(basicwidget.PanelBorders{})
	c.panel.SetBackgroundStyle(basicwidget.PanelBackgroundStyleNone)
	c.panel.SetContentConstraints(basicwidget.PanelContentConstraintsNone)
	c.diff.SetMultiline(true)
	c.diff.SetWrapMode(basicwidget.WrapModeNone)
	c.diff.SetSelectable(true)
	setGitDiffFont(&c.diff)
	text := ""
	if c.selected >= 0 && c.selected < len(c.files) {
		text = c.files[c.selected].Text
	}
	c.diff.SetValue(text)
	styles := styleDiff(text, context.ColorMode() == ebiten.ColorModeDark)
	c.diff.SetOverrideStyles(&styles, false)
	return nil
}

func (c *gitChanges) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	if c.message != "" {
		layouter.LayoutWidget(&c.note, bounds)
		return
	}
	u := basicwidget.UnitSize(context)
	listW := bounds.Dx() / 3
	if maxW := 18 * u; listW > maxW {
		listW = maxW
	}
	if minW := 10 * u; listW < minW {
		listW = minW
	}
	if listW > bounds.Dx()/2 {
		listW = bounds.Dx() / 2
	}
	if listW < 0 {
		listW = 0
	}
	gap := u / 4
	left := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+listW, bounds.Max.Y)
	right := image.Rect(bounds.Min.X+listW+gap, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	if right.Min.X > right.Max.X {
		right.Min.X = right.Max.X
	}
	layouter.LayoutWidget(&c.list, left)
	layouter.LayoutWidget(&c.panel, right)
}

func setGitDiffFont(t *basicwidget.Text) {
	family := gitDiffFont()
	if family == nil {
		return
	}
	var style basicwidget.TextStyle
	style.SetFontFamily(family)
	t.SetBaseStyle(&style)
}

func splitUnifiedDiff(diff string) []gitFileDiff {
	diff = strings.ReplaceAll(diff, "\r\n", "\n")
	if !strings.Contains(diff, "diff --git ") {
		return nil
	}
	var out []gitFileDiff
	var b strings.Builder
	var cur *gitFileDiff
	flush := func() {
		if cur == nil {
			return
		}
		cur.Text = b.String()
		out = append(out, *cur)
		b.Reset()
		cur = nil
	}
	for _, line := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			cur = &gitFileDiff{
				Name:   gitDiffName(strings.TrimRight(line, "\n")),
				Status: "M",
			}
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(trimmed, "new file mode "):
			cur.Status = "A"
		case strings.HasPrefix(trimmed, "deleted file mode "):
			cur.Status = "D"
		case strings.HasPrefix(trimmed, "rename from "), strings.HasPrefix(trimmed, "rename to "):
			cur.Status = "R"
			if name, ok := strings.CutPrefix(trimmed, "rename to "); ok {
				cur.Name = name
			}
		case strings.HasPrefix(trimmed, "copy from "), strings.HasPrefix(trimmed, "copy to "):
			cur.Status = "C"
			if name, ok := strings.CutPrefix(trimmed, "copy to "); ok {
				cur.Name = name
			}
		case strings.HasPrefix(trimmed, "+++ "):
			if p, ok := diffPath(trimmed[len("+++ "):]); ok && cur.Status != "R" && cur.Status != "C" {
				cur.Name = p
			}
		case strings.HasPrefix(trimmed, "--- "):
			if cur.Name == "" {
				if p, ok := diffPath(trimmed[len("--- "):]); ok {
					cur.Name = p
				}
			}
		}
		b.WriteString(line)
	}
	flush()
	return out
}

func gitDiffName(header string) string {
	rest := strings.TrimPrefix(header, "diff --git ")
	i := strings.LastIndex(rest, " b/")
	if i >= 0 && strings.HasPrefix(rest, "a/") {
		to := rest[i+len(" b/"):]
		if to != "" {
			return to
		}
		return rest[len("a/"):i]
	}
	return strings.TrimSpace(rest)
}

func diffPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "/dev/null" {
		return "", false
	}
	if strings.HasPrefix(s, "a/") || strings.HasPrefix(s, "b/") {
		return s[len("a/"):], true
	}
	return s, true
}

func styleDiff(text string, dark bool) basicwidget.TextStyles {
	var styles basicwidget.TextStyles
	pal := diffPalette(dark)
	for start := 0; start < len(text); {
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end = start + end + 1
		}
		line := text[start:end]
		var clr color.NRGBA
		var bg color.NRGBA
		switch {
		case strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "new file ") ||
			strings.HasPrefix(line, "deleted file ") ||
			strings.HasPrefix(line, "old mode ") ||
			strings.HasPrefix(line, "new mode ") ||
			strings.HasPrefix(line, "similarity ") ||
			strings.HasPrefix(line, "dissimilarity ") ||
			strings.HasPrefix(line, "rename ") ||
			strings.HasPrefix(line, "copy ") ||
			strings.HasPrefix(line, "Binary files ") ||
			strings.HasPrefix(line, "\\") ||
			strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "+++"):
			clr = pal.meta
		case strings.HasPrefix(line, "@@"):
			clr = pal.hunk
		case strings.HasPrefix(line, "+"):
			bg = pal.addBg
		case strings.HasPrefix(line, "-"):
			bg = pal.delBg
		}
		if clr.A != 0 {
			styles.SetColorInRange(start, end, clr)
		}
		if bg.A != 0 {
			styles.SetBackgroundColorInRange(start, end, bg)
		}
		if end == len(text) {
			break
		}
		start = end
	}
	return styles
}

type diffColors struct {
	addBg, delBg, hunk, meta color.NRGBA
}

func diffPalette(dark bool) diffColors {
	if dark {
		return diffColors{
			addBg: color.NRGBA{R: 0x1b, G: 0x3d, B: 0x26, A: 0xff},
			delBg: color.NRGBA{R: 0x3d, G: 0x1b, B: 0x1e, A: 0xff},
			hunk:  color.NRGBA{R: 0x79, G: 0xc0, B: 0xff, A: 0xff},
			meta:  color.NRGBA{R: 0x8b, G: 0x94, B: 0x9e, A: 0xff},
		}
	}
	return diffColors{
		addBg: color.NRGBA{R: 0xda, G: 0xfb, B: 0xe1, A: 0xff},
		delBg: color.NRGBA{R: 0xff, G: 0xeb, B: 0xe9, A: 0xff},
		hunk:  color.NRGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
		meta:  color.NRGBA{R: 0x57, G: 0x60, B: 0x6a, A: 0xff},
	}
}
