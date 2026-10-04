package app

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

var gitLaneColors = []color.NRGBA{
	{R: 0x4e, G: 0xc9, B: 0xb0, A: 0xff},
	{R: 0x56, G: 0x9c, B: 0xd6, A: 0xff},
	{R: 0xc5, G: 0x86, B: 0xc0, A: 0xff},
	{R: 0xce, G: 0x91, B: 0x78, A: 0xff},
	{R: 0xd7, G: 0xba, B: 0x7d, A: 0xff},
	{R: 0x6a, G: 0x99, B: 0x55, A: 0xff},
	{R: 0xf4, G: 0x47, B: 0x47, A: 0xff},
	{R: 0x9c, G: 0xdc, B: 0xfe, A: 0xff},
}

func gitLaneColor(lane int) color.NRGBA {
	if lane < 0 {
		lane = 0
	}
	return gitLaneColors[lane%len(gitLaneColors)]
}

func gitLaneWidth(u int) int {
	w := u / 2
	if w < 10 {
		return 10
	}
	return w
}

type gitGraphGlyph struct {
	guigui.DefaultWidget

	row   gitcli.GraphRow
	lanes int
	head  bool
}

func (g *gitGraphGlyph) Set(row gitcli.GraphRow, lanes int, head bool) {
	g.row = row
	g.lanes = lanes
	g.head = head
}

func (g *gitGraphGlyph) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(g.row.Commit.Hash)
	w.WriteInt(g.row.Lane)
	w.WriteInt(g.lanes)
	w.WriteBool(g.head)
}

func (g *gitGraphGlyph) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	n := g.lanes
	if n < 1 {
		n = 1
	}
	s := image.Pt(n*gitLaneWidth(u)+u/4, u)
	if w, ok := constraints.FixedWidth(); ok {
		s.X = w
	}
	if h, ok := constraints.FixedHeight(); ok {
		s.Y = h
	}
	return s
}

func (g *gitGraphGlyph) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	u := basicwidget.UnitSize(context)
	lw := gitLaneWidth(u)
	n := g.lanes
	if n < 1 {
		n = 1
	}
	midY := float32(b.Min.Y + b.Dy()/2)
	top := float32(b.Min.Y)
	bot := float32(b.Max.Y)
	laneX := func(lane int) float32 {
		return float32(b.Min.X) + float32(lw)/2 + float32(lane*lw)
	}
	stroke := float32(2)
	if u >= 28 {
		stroke = 2.5
	}
	// A lane change is one straight segment split across two rows. Both halves
	// meet at the row edge halfway between the lanes, so merge lines connect.
	for _, e := range g.row.Incoming {
		vector.StrokeLine(dst, gitEdgeBoundaryX(laneX, e), top, laneX(e.To), midY, stroke, gitEdgeColor(e), true)
	}
	for _, e := range g.row.Outgoing {
		vector.StrokeLine(dst, laneX(e.From), midY, gitEdgeBoundaryX(laneX, e), bot, stroke, gitEdgeColor(e), true)
	}
	x := laneX(g.row.Lane)
	r := float32(lw) * 0.28
	if r < 3 {
		r = 3
	}
	node := gitLaneColor(g.row.Lane)
	if g.row.Commit.Hash == gitcli.Uncommitted {
		node = gitUncommittedColor
	}
	vector.DrawFilledCircle(dst, x, midY, r, node, true)
	if g.head {
		vector.StrokeCircle(dst, x, midY, r+2.5, stroke, node, true)
	}
}

var gitUncommittedColor = color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff}

func gitEdgeColor(e gitcli.Edge) color.NRGBA {
	if e.Gray {
		return gitUncommittedColor
	}
	return gitLaneColor(e.To)
}

func gitEdgeBoundaryX(laneX func(int) float32, e gitcli.Edge) float32 {
	return (laneX(e.From) + laneX(e.To)) / 2
}

func gitMetaCols(u int) (author, hash, date int) {
	return 6 * u, 4 * u, 7 * u
}

func refDecorations(ds []gitcli.Decoration) []gitcli.Decoration {
	if len(ds) == 0 {
		return nil
	}
	var locals, remotes, tags []gitcli.Decoration
	for _, d := range ds {
		if d.Name == "" || d.Name == "HEAD" {
			continue
		}
		switch d.Kind {
		case "head":
			locals = append(locals, d)
		case "remote":
			remotes = append(remotes, d)
		case "tag":
			tags = append(tags, d)
		}
	}
	used := make([]bool, len(remotes))
	out := make([]gitcli.Decoration, 0, len(locals)+len(remotes)+len(tags))
	for _, local := range locals {
		var remotesHere []string
		for i, remote := range remotes {
			remoteName, branch, ok := remoteBranchName(remote.Name)
			if !ok || branch != local.Name {
				continue
			}
			used[i] = true
			remotesHere = append(remotesHere, remoteName)
		}
		if len(remotesHere) > 0 {
			slices.Sort(remotesHere)
			local.Name = local.Name + " " + strings.Join(remotesHere, " ")
		}
		out = append(out, local)
	}
	for i, remote := range remotes {
		if !used[i] {
			out = append(out, remote)
		}
	}
	out = append(out, tags...)
	return out
}

// remoteBranchName splits origin/main into origin and main.
func remoteBranchName(short string) (remote, branch string, ok bool) {
	remote, branch, ok = strings.Cut(short, "/")
	if !ok || remote == "" || branch == "" || branch == "HEAD" {
		return "", "", false
	}
	return remote, branch, true
}

var (
	gitBadgeHead   = color.NRGBA{R: 0x2f, G: 0x81, B: 0xf7, A: 0xff}
	gitBadgeLocal  = color.NRGBA{R: 0x2a, G: 0x9d, B: 0x8f, A: 0xff}
	gitBadgeRemote = color.NRGBA{R: 0x5c, G: 0x6b, B: 0xc0, A: 0xff}
	gitBadgeTag    = color.NRGBA{R: 0xc4, G: 0x84, B: 0x1d, A: 0xff}
	gitBadgeText   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

type gitRefBadge struct {
	guigui.DefaultWidget

	text     basicwidget.Text
	local    basicwidget.Text
	remote   basicwidget.Text
	measure  basicwidget.Text
	split    bool
	sep      image.Rectangle
	kind     string
	head     bool
	ref      gitcli.Ref
	parts    []gitcli.Ref
	partEnds []int
	onSwitch func(gitcli.Ref)
	onMenu   func(gitcli.Ref)
	clicks   gitClickCount
	clickRef gitcli.Ref
}

// splitMergedBranch separates a merged label such as "main origin upstream"
// into the local name and the remote names.
func splitMergedBranch(name string) (local, remote string, ok bool) {
	local, remote, ok = strings.Cut(name, " ")
	return local, remote, ok && local != "" && remote != ""
}

// mergedBranchRefs splits a merged badge into the local branch and each
// remote-tracking branch. "main origin upstream" is main, origin/main, and
// upstream/main.
func mergedBranchRefs(d gitcli.Decoration, hash string) ([]gitcli.Ref, bool) {
	if d.Kind != "head" {
		return nil, false
	}
	localName, remoteLabel, ok := splitMergedBranch(d.Name)
	if !ok {
		return nil, false
	}
	names := strings.Fields(remoteLabel)
	if len(names) == 0 {
		return nil, false
	}
	refs := make([]gitcli.Ref, 0, 1+len(names))
	refs = append(refs, gitcli.Ref{Name: localName, Hash: hash, Current: d.HEAD})
	for _, name := range names {
		refs = append(refs, gitcli.Ref{
			Name:   name + "/" + localName,
			Hash:   hash,
			Remote: name,
		})
	}
	return refs, true
}

// mergedBadgeEnds returns the exclusive max X of each hit region in a merged
// badge. The first region is the local name and ends at the separator.
// Each following region is one remote name.
func mergedBadgeEnds(right, splitX, remoteX int, remoteWidths []int, spaceW int) []int {
	ends := make([]int, 1+len(remoteWidths))
	ends[0] = splitX
	x := remoteX
	for i, w := range remoteWidths {
		if i == len(remoteWidths)-1 {
			ends[i+1] = right
			break
		}
		x += w
		ends[i+1] = x + spaceW/2
		x += spaceW
	}
	return ends
}

func refAtX(x int, refs []gitcli.Ref, ends []int) (gitcli.Ref, bool) {
	if len(refs) == 0 || len(refs) != len(ends) {
		return gitcli.Ref{}, false
	}
	for i, end := range ends {
		if x < end {
			return refs[i], true
		}
	}
	return refs[len(refs)-1], true
}

func setGitBadgeText(t *basicwidget.Text, value string) {
	t.SetValue(value)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	t.SetWrapMode(basicwidget.WrapModeNone)
	var style basicwidget.TextStyle
	style.SetBold(true)
	style.SetScale(0.85)
	style.SetColor(gitBadgeText)
	useGitMono(&style)
	t.SetBaseStyle(&style)
}

func (b *gitRefBadge) Set(d gitcli.Decoration, hash string, onSwitch func(gitcli.Ref), onMenu func(gitcli.Ref)) {
	ref, _ := branchSwitchRef(d, hash)
	if b.ref != ref {
		b.clicks = gitClickCount{}
		b.clickRef = gitcli.Ref{}
	}
	b.ref = ref
	b.onSwitch = onSwitch
	b.onMenu = onMenu
	b.kind = d.Kind
	b.head = d.HEAD
	b.parts = b.parts[:0]
	local, remote, _ := splitMergedBranch(d.Name)
	if refs, ok := mergedBranchRefs(d, hash); ok {
		b.parts = append(b.parts, refs...)
		b.split = true
	} else {
		b.split = false
	}
	if b.split {
		if d.HEAD {
			local = "✓ " + local
		}
		setGitBadgeText(&b.local, local)
		setGitBadgeText(&b.remote, remote)
		return
	}
	name := d.Name
	if d.HEAD {
		name = "✓ " + name
	}
	setGitBadgeText(&b.text, name)
}

func (b *gitRefBadge) fill() color.NRGBA {
	switch b.kind {
	case "remote":
		return gitBadgeRemote
	case "tag":
		return gitBadgeTag
	default:
		if b.head {
			return gitBadgeHead
		}
		return gitBadgeLocal
	}
}

func (b *gitRefBadge) refAt(x int) (gitcli.Ref, bool) {
	if len(b.parts) > 1 && len(b.partEnds) == len(b.parts) {
		return refAtX(x, b.parts, b.partEnds)
	}
	if b.ref.Name == "" {
		return gitcli.Ref{}, false
	}
	return b.ref, true
}

func (b *gitRefBadge) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !context.IsEnabled(b) || !widgetBounds.IsHitAtCursor() {
		return guigui.HandleInputResult{}
	}
	ref, ok := b.refAt(image.Pt(ebiten.CursorPosition()).X)
	if !ok {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && b.onMenu != nil && !ref.IsTag() {
		b.onMenu(ref)
		return guigui.HandleInputByWidget(b)
	}
	if b.onSwitch == nil || ref.Current {
		return guigui.HandleInputResult{}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	if b.clickRef != ref {
		b.clicks = gitClickCount{}
		b.clickRef = ref
	}
	if b.clicks.click(ebiten.Tick()) < 2 {
		return guigui.HandleInputResult{}
	}
	b.onSwitch(ref)
	return guigui.HandleInputByWidget(b)
}

func (b *gitRefBadge) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if b.onSwitch == nil || !context.IsEnabled(b) || !widgetBounds.IsHitAtCursor() {
		return 0, false
	}
	ref, ok := b.refAt(image.Pt(ebiten.CursorPosition()).X)
	if !ok || ref.Current {
		return 0, false
	}
	return ebiten.CursorShapePointer, true
}

func (b *gitRefBadge) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if b.split {
		adder.AddWidget(&b.local)
		adder.AddWidget(&b.remote)
		return nil
	}
	adder.AddWidget(&b.text)
	return nil
}

func (b *gitRefBadge) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	pad := u / 4
	bounds := widgetBounds.Bounds()
	if !b.split {
		b.sep = image.Rectangle{}
		b.partEnds = b.partEnds[:0]
		text := bounds
		text.Min.X += pad
		text.Max.X -= pad
		if text.Dx() < 0 {
			text.Max.X = text.Min.X
		}
		layouter.LayoutWidget(&b.text, text)
		return
	}
	gap := gitBadgeSepGap(u)
	localW := b.local.Measure(context, guigui.Constraints{}).X
	remoteW := b.remote.Measure(context, guigui.Constraints{}).X
	x := bounds.Min.X + pad
	layouter.LayoutWidget(&b.local, image.Rect(x, bounds.Min.Y, x+localW, bounds.Max.Y))
	sepW := gitBadgeSepWidth(u)
	sepX := x + localW + (gap-sepW)/2
	b.sep = image.Rect(sepX, bounds.Min.Y, sepX+sepW, bounds.Max.Y)
	rx := x + localW + gap
	layouter.LayoutWidget(&b.remote, image.Rect(rx, bounds.Min.Y, rx+remoteW, bounds.Max.Y))
	b.partEnds = b.badgePartEnds(context, bounds.Max.X, x+localW+gap/2, rx)
}

func (b *gitRefBadge) badgePartEnds(context *guigui.Context, right, splitX, remoteX int) []int {
	n := len(b.parts) - 1
	if n <= 0 {
		return b.partEnds[:0]
	}
	widths := make([]int, n)
	for i, ref := range b.parts[1:] {
		widths[i] = b.textWidth(context, ref.Remote)
	}
	spaceW := 0
	if n > 1 {
		spaceW = b.textWidth(context, " ")
	}
	ends := mergedBadgeEnds(right, splitX, remoteX, widths, spaceW)
	return append(b.partEnds[:0], ends...)
}

func (b *gitRefBadge) textWidth(context *guigui.Context, value string) int {
	setGitBadgeText(&b.measure, value)
	return b.measure.Measure(context, guigui.Constraints{}).X
}

func gitBadgeSepGap(u int) int {
	gap := u / 3
	if gap < 6 {
		gap = 6
	}
	return gap
}

func gitBadgeSepWidth(u int) int {
	w := u / 16
	if w < 1 {
		w = 1
	}
	return w
}

func (b *gitRefBadge) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	pad := u / 4
	tw := b.text.Measure(context, guigui.Constraints{}).X
	if b.split {
		tw = b.local.Measure(context, guigui.Constraints{}).X + gitBadgeSepGap(u) + b.remote.Measure(context, guigui.Constraints{}).X
	}
	h := u
	if fixed, ok := constraints.FixedHeight(); ok && fixed > 0 {
		h = fixed
	}
	return image.Pt(tw+pad*2, h)
}

func (b *gitRefBadge) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	bounds := widgetBounds.Bounds()
	if bounds.Empty() {
		return
	}
	inset := bounds.Dy() / 6
	if inset < 1 {
		inset = 1
	}
	pill := bounds
	pill.Min.Y += inset
	pill.Max.Y -= inset
	fillRoundRect(dst, pill, b.fill())
	if !b.split || b.sep.Empty() {
		return
	}
	fillRoundRectClipped(dst, pill, image.Rect(b.sep.Min.X, pill.Min.Y, pill.Max.X, pill.Max.Y), gitBadgeRemote)
	line := b.sep
	trim := pill.Dy() / 5
	if trim < 1 {
		trim = 1
	}
	line.Min.Y = pill.Min.Y + trim
	line.Max.Y = pill.Max.Y - trim
	if line.Dy() > 0 && line.Dx() > 0 {
		fillRect(dst, line, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xcc})
	}
}

type gitRefBadges struct {
	guigui.DefaultWidget

	badges      guigui.WidgetSlice[*gitRefBadge]
	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (b *gitRefBadges) Set(ds []gitcli.Decoration, hash string, onSwitch func(gitcli.Ref), onMenu func(gitcli.Ref)) {
	labels := refDecorations(ds)
	b.badges.SetLen(len(labels))
	for i, d := range labels {
		b.badges.At(i).Set(d, hash, onSwitch, onMenu)
	}
}

func (b *gitRefBadges) Len() int { return b.badges.Len() }

func (b *gitRefBadges) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for i := 0; i < b.badges.Len(); i++ {
		adder.AddWidget(b.badges.At(i))
	}
	return nil
}

func (b *gitRefBadges) layoutRow(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	b.layoutItems = slices.Delete(b.layoutItems, 0, len(b.layoutItems))
	for i := 0; i < b.badges.Len(); i++ {
		b.layoutItems = append(b.layoutItems, guigui.LinearLayoutItem{Widget: b.badges.At(i)})
	}
	b.layout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     b.layoutItems,
		Gap:       u / 6,
	}
	return b.layout
}

func (b *gitRefBadges) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b.layoutRow(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (b *gitRefBadges) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	if b.Len() == 0 {
		return image.Point{}
	}
	return b.layoutRow(context).Measure(context, constraints)
}

type gitCommitRow struct {
	guigui.DefaultWidget

	graph       gitGraphGlyph
	badges      gitRefBadges
	subject     basicwidget.Text
	author      basicwidget.Text
	hash        basicwidget.Text
	date        basicwidget.Text
	uncommitted bool

	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (r *gitCommitRow) Set(row gitcli.GraphRow, lanes int, head bool, lang i18n.Lang, onSwitch func(gitcli.Ref), onMenu func(gitcli.Ref)) {
	r.uncommitted = row.Commit.Hash == gitcli.Uncommitted
	r.graph.Set(row, lanes, head)
	r.badges.Set(row.Commit.Decorations, row.Commit.Hash, onSwitch, onMenu)
	if r.uncommitted {
		r.subject.SetValue(i18n.T(lang, i18n.GitUncommitted))
		r.author.SetValue("")
		r.hash.SetValue("")
		r.date.SetValue("")
		return
	}
	r.subject.SetValue(row.Commit.Subject)
	r.author.SetValue(row.Commit.Author)
	r.hash.SetValue(row.Commit.Short())
	if row.Commit.When.IsZero() {
		r.date.SetValue("")
	} else {
		r.date.SetValue(row.Commit.When.Local().Format("2006-01-02 15:04"))
	}
}

func (r *gitCommitRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.graph)
	if r.badges.Len() > 0 {
		adder.AddWidget(&r.badges)
	}
	adder.AddWidget(&r.subject)
	adder.AddWidget(&r.author)
	adder.AddWidget(&r.hash)
	adder.AddWidget(&r.date)
	for _, text := range []*basicwidget.Text{&r.subject, &r.author, &r.hash, &r.date} {
		text.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		text.SetWrapMode(basicwidget.WrapModeNone)
	}
	r.date.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	return nil
}

func (r *gitCommitRow) layoutRow(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	n := r.graph.lanes
	if n < 1 {
		n = 1
	}
	graphW := n*gitLaneWidth(u) + u/4
	authorW, hashW, dateW := gitMetaCols(u)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	r.layoutItems = append(r.layoutItems,
		guigui.LinearLayoutItem{Widget: &r.graph, Size: guigui.FixedSize(graphW)},
	)
	if r.badges.Len() > 0 {
		r.layoutItems = append(r.layoutItems, guigui.LinearLayoutItem{Widget: &r.badges})
	}
	r.layoutItems = append(r.layoutItems,
		guigui.LinearLayoutItem{Widget: &r.subject, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.author, Size: guigui.FixedSize(authorW)},
		guigui.LinearLayoutItem{Widget: &r.hash, Size: guigui.FixedSize(hashW)},
		guigui.LinearLayoutItem{Widget: &r.date, Size: guigui.FixedSize(dateW)},
	)
	r.layout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.layoutItems,
		Gap:       u / 6,
	}
	return r.layout
}

func (r *gitCommitRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	var style basicwidget.TextStyle
	if v, ok := context.Env(r, basicwidget.EnvKeyListItemColorType); ok {
		if ct, ok := v.(basicwidget.ListItemColorType); ok {
			if c := ct.TextColor(context); c != nil {
				style.SetColor(c)
			}
		}
	}
	if r.uncommitted {
		style.SetColor(gitUncommittedColor)
	}
	useGitMono(&style)
	r.subject.SetBaseStyle(&style)
	r.author.SetBaseStyle(&style)
	r.hash.SetBaseStyle(&style)
	r.date.SetBaseStyle(&style)
	r.layoutRow(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *gitCommitRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	s := r.layoutRow(context).Measure(context, constraints)
	if s.Y < u {
		s.Y = u
	}
	return s
}

// branchSwitchRef is the branch a graph badge switches to. A merged badge
// such as "main origin" switches to the local branch.
func branchSwitchRef(d gitcli.Decoration, hash string) (gitcli.Ref, bool) {
	switch d.Kind {
	case "head":
		name, _, _ := strings.Cut(d.Name, " ")
		if name == "" || name == "HEAD" {
			return gitcli.Ref{}, false
		}
		return gitcli.Ref{Name: name, Hash: hash, Current: d.HEAD}, true
	case "remote":
		remote, _, ok := remoteBranchName(d.Name)
		if !ok {
			return gitcli.Ref{}, false
		}
		return gitcli.Ref{Name: d.Name, Hash: hash, Remote: remote}, true
	case "tag":
		if d.Name == "" {
			return gitcli.Ref{}, false
		}
		return gitcli.Ref{Name: d.Name, Full: "refs/tags/" + d.Name, Hash: hash}, true
	default:
		return gitcli.Ref{}, false
	}
}

type gitClickCount struct {
	n         int
	lastPlus1 int64
}

func (c *gitClickCount) click(tick int64) int {
	if c.lastPlus1 != 0 && tick-(c.lastPlus1-1) < int64(ebiten.TPS())/2 {
		c.n++
	} else {
		c.n = 1
	}
	c.lastPlus1 = tick + 1
	return c.n
}

// fillRoundRectClipped fills the rounded rect, clipped to clip.
// Coordinates stay in dst's space; the sub-image only limits the painted region.
func fillRoundRectClipped(dst *ebiten.Image, bounds, clip image.Rectangle, clr color.NRGBA) {
	clip = clip.Intersect(bounds).Intersect(dst.Bounds())
	if clip.Empty() {
		return
	}
	sub := dst.SubImage(clip)
	if sub == nil {
		return
	}
	fillRoundRect(sub.(*ebiten.Image), bounds, clr)
}

func fillRoundRect(dst *ebiten.Image, bounds image.Rectangle, clr color.NRGBA) {
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	x := float32(bounds.Min.X)
	y := float32(bounds.Min.Y)
	w := float32(bounds.Dx())
	h := float32(bounds.Dy())
	rad := h / 2
	if rad*2 > w {
		rad = w / 2
	}
	var p vector.Path
	p.MoveTo(x+rad, y)
	p.LineTo(x+w-rad, y)
	p.ArcTo(x+w, y, x+w, y+rad, rad)
	p.LineTo(x+w, y+h-rad)
	p.ArcTo(x+w, y+h, x+w-rad, y+h, rad)
	p.LineTo(x+rad, y+h)
	p.ArcTo(x, y+h, x, y+h-rad, rad)
	p.LineTo(x, y+rad)
	p.ArcTo(x, y, x+rad, y, rad)
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, nil, op)
}
