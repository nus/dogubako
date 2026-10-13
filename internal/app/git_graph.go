package app

import (
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitLaneColors is the commit-graph palette. Colors repeat after the last one.
var gitLaneColors = []color.NRGBA{
	{R: 0x4a, G: 0x68, B: 0xee, A: 0xff},
	{R: 0x4a, G: 0xed, B: 0x67, A: 0xff},
	{R: 0xed, G: 0xc4, B: 0x4a, A: 0xff},
	{R: 0xed, G: 0x4a, B: 0x67, A: 0xff},
	{R: 0x76, G: 0xed, B: 0xd1, A: 0xff},
	{R: 0xed, G: 0xe8, B: 0x77, A: 0xff},
	{R: 0xed, G: 0x94, B: 0x77, A: 0xff},
	{R: 0xa3, G: 0x77, B: 0xed, A: 0xff},
}

func gitLaneColor(index int) color.NRGBA {
	if index < 0 {
		index = 0
	}
	return gitLaneColors[index%len(gitLaneColors)]
}

func gitLaneWidth(u int) int {
	w := u / 2
	if w < 10 {
		return 10
	}
	return w
}

func gitLaneMinWidth(u int) int {
	w := u / 4
	if w < 6 {
		w = 6
	}
	natural := gitLaneWidth(u)
	if w > natural {
		w = natural
	}
	if w < 1 {
		return 1
	}
	return w
}

func gitGraphLaneCount(lanes int) int {
	if lanes < 1 {
		return 1
	}
	return lanes
}

func gitGraphNaturalWidth(u, lanes int) int {
	if u < 1 {
		u = 1
	}
	return gitGraphLaneCount(lanes)*gitLaneWidth(u) + u/4
}

func gitGraphMinWidth(u, lanes int) int {
	if u < 1 {
		u = 1
	}
	return gitGraphLaneCount(lanes)*gitLaneMinWidth(u) + u/4
}

// gitGraphReserve is the width kept for the subject and the meta columns.
func gitGraphReserve(u int) int {
	if u < 1 {
		u = 1
	}
	author, hash, date := gitMetaCols(u)
	return author + hash + date + 6*u
}

func clampGitGraphCol(w, rowW, u, lanes int) int {
	if u < 1 {
		u = 1
	}
	minW := gitGraphMinWidth(u, lanes)
	if rowW <= 0 {
		if w < minW {
			return minW
		}
		return w
	}
	maxW := rowW - gitGraphReserve(u)
	if maxW < minW {
		maxW = rowW / 2
		if maxW < 1 {
			maxW = 1
		}
		if minW > maxW {
			minW = maxW
		}
	}
	if w < minW {
		w = minW
	}
	if w > maxW {
		w = maxW
	}
	if w < 1 {
		return 1
	}
	return w
}

// gitGraphColWidth is the commit-graph column width.
// units is the dragged width in unit sizes. Zero keeps the width that fits the lanes.
func gitGraphColWidth(rowW, u, lanes int, units float64) int {
	if u < 1 {
		u = 1
	}
	w := gitGraphNaturalWidth(u, lanes)
	if units > 0 {
		w = int(units*float64(u) + 0.5)
	}
	return clampGitGraphCol(w, rowW, u, lanes)
}

// gitLaneStep is the space between lane centers in a column of colW pixels.
// A wider column keeps the natural spacing. A narrower column packs the lanes in.
func gitLaneStep(colW, lanes, u int) int {
	natural := gitLaneWidth(u)
	n := gitGraphLaneCount(lanes)
	pad := 0
	if u > 0 {
		pad = u / 4
	}
	if pad > colW {
		pad = 0
	}
	inner := colW - pad
	if inner < n {
		inner = n
	}
	step := inner / n
	if step > natural {
		step = natural
	}
	if step < 1 {
		return 1
	}
	return step
}

type gitGraphGlyph struct {
	guigui.DefaultWidget

	row         gitcli.GraphRow
	lanes       int
	head        bool
	expandAbove bool
	expandBelow bool
}

func (g *gitGraphGlyph) Set(row gitcli.GraphRow, lanes int, head, expandAbove, expandBelow bool) {
	g.row = row
	g.lanes = lanes
	g.head = head
	g.expandAbove = expandAbove
	g.expandBelow = expandBelow
}

func (g *gitGraphGlyph) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(g.row.Commit.Hash)
	w.WriteInt(g.row.Lane)
	w.WriteInt(g.row.Color)
	w.WriteInt(g.lanes)
	w.WriteBool(g.head)
	w.WriteBool(g.expandAbove)
	w.WriteBool(g.expandBelow)
	writeGitEdges(w, g.row.Incoming)
	writeGitEdges(w, g.row.Outgoing)
}

func writeGitEdges(w *guigui.StateKeyWriter, edges []gitcli.Edge) {
	w.WriteInt(len(edges))
	for _, e := range edges {
		w.WriteInt(e.From)
		w.WriteInt(e.To)
		w.WriteInt(e.Color)
		w.WriteBool(e.Gray)
		w.WriteBool(e.LockTop)
	}
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
	n := g.lanes
	if n < 1 {
		n = 1
	}
	lw := gitLaneStep(b.Dx(), n, u)
	midY := float32(b.Min.Y + b.Dy()/2)
	top := float32(b.Min.Y)
	bot := float32(b.Max.Y)
	laneX := func(lane int) float32 {
		return float32(b.Min.X) + float32(lw)/2 + float32(lane*lw)
	}
	stroke := gitGraphStroke(u)
	bg := basicwidgetdraw.BackgroundColor(context.ColorMode())
	// A lane change is one cubic from this commit's center to the next,
	// split on the row boundary.
	H := float32(b.Dy())
	for _, e := range g.row.Incoming {
		if e.From == e.To || (g.expandAbove && e.LockTop) {
			strokeGitSegment(dst, laneX(e.To), top, laneX(e.To), midY, stroke, gitEdgeColor(e), bg)
			continue
		}
		strokeGitCurve(dst, laneX(e.From), midY-H, laneX(e.To), midY, stroke, gitEdgeColor(e), bg, false)
	}
	for _, e := range g.row.Outgoing {
		if e.From == e.To || (g.expandBelow && !e.LockTop) {
			strokeGitSegment(dst, laneX(e.From), midY, laneX(e.From), bot, stroke, gitEdgeColor(e), bg)
			continue
		}
		strokeGitCurve(dst, laneX(e.From), midY, laneX(e.To), midY+H, stroke, gitEdgeColor(e), bg, true)
	}
	x := laneX(g.row.Lane)
	r := gitNodeRadius(lw)
	node := gitLaneColor(g.row.Color)
	open := g.row.Commit.Hash == gitcli.Uncommitted
	if open {
		node = gitUncommittedColor
		vector.DrawFilledCircle(dst, x, midY, r, bg, true)
		vector.StrokeCircle(dst, x, midY, r, stroke, node, true)
		return
	}
	vector.DrawFilledCircle(dst, x, midY, r, node, true)
	vector.StrokeCircle(dst, x, midY, r, 1, gitFade(bg, 0xbf), true)
}

var gitUncommittedColor = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}

func gitGraphStroke(u int) float32 {
	if u >= 28 {
		return 2.5
	}
	return 2
}

func gitEdgeColor(e gitcli.Edge) color.NRGBA {
	if e.Gray {
		return gitUncommittedColor
	}
	return gitLaneColor(e.Color)
}

func gitFade(c color.Color, a uint8) color.NRGBA {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = uint8(int(n.A) * int(a) / 255)
	return n
}

// gitGraphBridge continues a commit's outgoing lanes through the detail
// panel that opens under that commit.
type gitGraphBridge struct {
	guigui.DefaultWidget

	lanes int
	edges []gitcli.Edge
}

func (g *gitGraphBridge) Set(lanes int, edges []gitcli.Edge) {
	g.lanes = lanes
	g.edges = append(g.edges[:0], edges...)
}

func (g *gitGraphBridge) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteInt(g.lanes)
	writeGitEdges(w, g.edges)
}

func (g *gitGraphBridge) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
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

func (g *gitGraphBridge) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() || len(g.edges) == 0 {
		return
	}
	u := basicwidget.UnitSize(context)
	lw := gitLaneStep(b.Dx(), g.lanes, u)
	stroke := gitGraphStroke(u)
	bg := basicwidgetdraw.BackgroundColor(context.ColorMode())
	top := float32(b.Min.Y)
	bot := float32(b.Max.Y)
	rh := float32(u)
	laneX := func(lane int) float32 {
		return float32(b.Min.X) + float32(lw)/2 + float32(lane*lw)
	}
	// The open commit stretches the graph. A lane change keeps its normal
	// row of curve, either just under the commit or just above the next one.
	for pass := 0; pass < 2; pass++ {
		for _, e := range g.edges {
			if (pass == 0) != e.Gray {
				continue
			}
			x0 := laneX(e.From)
			x1 := laneX(e.To)
			if e.From == e.To {
				strokeGitSegment(dst, x1, top, x1, bot, stroke, gitEdgeColor(e), bg)
				continue
			}
			if e.LockTop {
				strokeGitCurve(dst, x0, top-rh/2, x1, top+rh/2, stroke, gitEdgeColor(e), bg, false)
				if top+rh/2 < bot {
					strokeGitSegment(dst, x1, top+rh/2, x1, bot, stroke, gitEdgeColor(e), bg)
				}
				continue
			}
			if top < bot-rh/2 {
				strokeGitSegment(dst, x0, top, x0, bot-rh/2, stroke, gitEdgeColor(e), bg)
			}
			strokeGitCurve(dst, x0, bot-rh/2, x1, bot+rh/2, stroke, gitEdgeColor(e), bg, true)
		}
	}
}

// bridgeEdges keeps one edge per destination lane. A colored edge wins over a gray one.
func bridgeEdges(edges []gitcli.Edge) []gitcli.Edge {
	idx := map[int]int{}
	out := make([]gitcli.Edge, 0, len(edges))
	for _, e := range edges {
		if i, ok := idx[e.To]; ok {
			if out[i].Gray && !e.Gray {
				out[i] = e
			}
			continue
		}
		idx[e.To] = len(out)
		out = append(out, e)
	}
	return out
}

func gitNodeRadius(laneWidth int) float32 {
	r := float32(laneWidth) * 0.28
	if r < 3 {
		return 3
	}
	return r
}

func strokeGitPath(dst *ebiten.Image, p *vector.Path, stroke float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, p, &vector.StrokeOptions{
		Width:    stroke,
		LineCap:  vector.LineCapButt,
		LineJoin: vector.LineJoinRound,
	}, op)
}

func strokeGitSegment(dst *ebiten.Image, x0, y0, x1, y1, stroke float32, clr, bg color.Color) {
	vector.StrokeLine(dst, x0, y0, x1, y1, stroke+2, gitFade(bg, 0xbf), true)
	vector.StrokeLine(dst, x0, y0, x1, y1, stroke, clr, true)
}

// strokeGitCurve draws one half of a rounded lane change.
// The full curve runs from (x0, y0) to (x1, y1). first selects the upper half.
func strokeGitCurve(dst *ebiten.Image, x0, y0, x1, y1, stroke float32, clr, bg color.Color, first bool) {
	if x0 == x1 {
		mid := (y0 + y1) / 2
		if first {
			strokeGitSegment(dst, x0, y0, x0, mid, stroke, clr, bg)
		} else {
			strokeGitSegment(dst, x0, mid, x0, y1, stroke, clr, bg)
		}
		return
	}
	ax, ay, bx, by, cx, cy, dx, dy := gitCurveHalf(x0, y0, x1, y1, first)
	var p vector.Path
	p.MoveTo(ax, ay)
	p.CubicTo(bx, by, cx, cy, dx, dy)
	strokeGitPath(dst, &p, stroke+2, gitFade(bg, 0xbf))
	strokeGitPath(dst, &p, stroke, clr)
}

// gitCurveHalf splits the cubic C x0,y0+0.8H x1,y1-0.8H x1,y1 at its midpoint.
func gitCurveHalf(x0, y0, x1, y1 float32, first bool) (ax, ay, bx, by, cx, cy, dx, dy float32) {
	H := y1 - y0
	p1y := y0 + 0.8*H
	p2y := y1 - 0.8*H
	aX := x0
	aY := (y0 + p1y) / 2
	bX := (x0 + x1) / 2
	bY := (p1y + p2y) / 2
	cX := x1
	cY := (p2y + y1) / 2
	dX := (aX + bX) / 2
	dY := (aY + bY) / 2
	eX := (bX + cX) / 2
	eY := (bY + cY) / 2
	fX := (dX + eX) / 2
	fY := (dY + eY) / 2
	if first {
		return x0, y0, aX, aY, dX, dY, fX, fY
	}
	return fX, fY, eX, eY, cX, cY, x1, y1
}

func gitMetaCols(u int) (author, hash, date int) {
	return 6 * u, 4 * u, 7 * u
}

func gitCommitColGap(u int) int {
	if u < 1 {
		u = 1
	}
	return u / 6
}

func gitMinSubjectCol(u int) int { return 4 * u }

func gitMinAuthorCol(u int) int { return 3 * u }

func gitMinHashCol(u int) int { return 3 * u }

func gitMinDateCol(u int) int { return 5 * u }

func gitMaxMetaCol(u int) int { return 48 * u }

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// gitMetaColWidths is the author, commit-id, and date column widths.
// A unit value of zero keeps that column's default width.
func gitMetaColWidths(rowW, u, graphW int, authorU, hashU, dateU float64) (author, hash, date int) {
	if u < 1 {
		u = 1
	}
	author, hash, date = gitMetaCols(u)
	if authorU > 0 {
		author = int(authorU*float64(u) + 0.5)
	}
	if hashU > 0 {
		hash = int(hashU*float64(u) + 0.5)
	}
	if dateU > 0 {
		date = int(dateU*float64(u) + 0.5)
	}
	return gitFitMetaCols(author, hash, date, rowW, u, graphW)
}

// gitFitMetaCols keeps the title column usable. Extra width stays on the title,
// so a narrow row shrinks the meta columns down toward their minimums.
func gitFitMetaCols(author, hash, date, rowW, u, graphW int) (int, int, int) {
	if u < 1 {
		u = 1
	}
	maxW := gitMaxMetaCol(u)
	author = clampInt(author, gitMinAuthorCol(u), maxW)
	hash = clampInt(hash, gitMinHashCol(u), maxW)
	date = clampInt(date, gitMinDateCol(u), maxW)
	if rowW <= 0 {
		return author, hash, date
	}
	overflow := graphW + gitMinSubjectCol(u) + author + hash + date + 4*gitCommitColGap(u) - rowW
	if overflow <= 0 {
		return author, hash, date
	}
	shrink := func(w, minW int) int {
		extra := w - minW
		if extra <= 0 || overflow <= 0 {
			return w
		}
		d := extra
		if d > overflow {
			d = overflow
		}
		overflow -= d
		return w - d
	}
	date = shrink(date, gitMinDateCol(u))
	hash = shrink(hash, gitMinHashCol(u))
	author = shrink(author, gitMinAuthorCol(u))
	return author, hash, date
}

// gitMetaRoom is the widest a meta column can be beside the graph, a minimum
// title, and the other two meta columns.
func gitMetaRoom(rowW, u, graphW, keep1, keep2, minW int) int {
	maxW := gitMaxMetaCol(u)
	if rowW <= 0 {
		return maxW
	}
	room := rowW - graphW - gitMinSubjectCol(u) - keep1 - keep2 - 4*gitCommitColGap(u)
	return clampInt(room, minW, maxW)
}

// gitMetaEdges is the horizontal extent of the title, author, commit id, and
// date. The three meta columns are packed against the right edge of the row.
func gitMetaEdges(rowRight, gap, authorW, hashW, dateW int) (subjectR, authorL, authorR, hashL, hashR, dateL, dateR int) {
	dateR = rowRight
	dateL = dateR - dateW
	hashR = dateL - gap
	hashL = hashR - hashW
	authorR = hashL - gap
	authorL = authorR - authorW
	subjectR = authorL - gap
	return subjectR, authorL, authorR, hashL, hashR, dateL, dateR
}

// gitMetaSplitEdges is where the title, author, and commit-id columns end.
func gitMetaSplitEdges(rowRight, gap, authorW, hashW, dateW int) (subjectR, authorR, hashR int) {
	subjectR, _, authorR, _, hashR, _, _ = gitMetaEdges(rowRight, gap, authorW, hashW, dateW)
	return subjectR, authorR, hashR
}

// gitColSplitRect is the hit target centered on a column edge.
func gitColSplitRect(edge, splitW, y0, y1, minX, maxX int) image.Rectangle {
	x := edge - splitW/2
	if x < minX {
		x = minX
	}
	if x > maxX-splitW {
		x = maxX - splitW
	}
	if x < minX {
		x = minX
	}
	if y1 < y0 {
		y1 = y0
	}
	return image.Rect(x, y0, x+splitW, y1)
}

// gitResizeAuthor sets the author width from the title's right edge.
// x is that edge. The author's right edge stays put, so the title takes the difference.
func gitResizeAuthor(authorR, gap, x, minW, maxW int) int {
	return clampInt(authorR-x-gap, minW, maxW)
}

// gitTradeCols splits a fixed span at x, the right edge of the left column.
// leftL and rightR stay put, so only those two columns change width.
func gitTradeCols(leftL, rightR, gap, x, minL, minR int) (leftW, rightW int) {
	total := rightR - gap - leftL
	if total < minL+minR {
		return minL, minR
	}
	leftW = clampInt(x-leftL, minL, total-minR)
	return leftW, total - leftW
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

	tool        *GitTool
	graph       gitGraphGlyph
	badges      gitRefBadges
	subject     basicwidget.Text
	author      basicwidget.Text
	hash        basicwidget.Text
	date        basicwidget.Text
	graphUnits  float64
	authorUnits float64
	hashUnits   float64
	dateUnits   float64
	uncommitted bool

	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (r *gitCommitRow) Set(row gitcli.GraphRow, lanes int, head, expandAbove, expandBelow bool, lang i18n.Lang, onSwitch func(gitcli.Ref), onMenu func(gitcli.Ref)) {
	r.uncommitted = row.Commit.Hash == gitcli.Uncommitted
	r.graph.Set(row, lanes, head, expandAbove, expandBelow)
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
		text.SetSelectable(false)
	}
	r.date.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	return nil
}

func (r *gitCommitRow) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if r.tool == nil || graphArrowBlocked(context) {
		return guigui.HandleInputResult{}
	}
	down, ok := graphArrow()
	if !ok || !r.tool.moveOpenCommit(down) {
		return guigui.HandleInputResult{}
	}
	return guigui.HandleInputByWidget(r)
}

func (r *gitCommitRow) layoutRow(context *guigui.Context, rowW int) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	n := r.graph.lanes
	if n < 1 {
		n = 1
	}
	graphW := gitGraphColWidth(rowW, u, n, preferGraphUnits(r.graphUnits))
	authorU, hashU, dateU := preferMetaUnits(r.authorUnits, r.hashUnits, r.dateUnits)
	authorW, hashW, dateW := gitMetaColWidths(rowW, u, graphW, authorU, hashU, dateU)
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
		Gap:       gitCommitColGap(u),
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
	r.layoutRow(context, widgetBounds.Bounds().Dx()).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func gitCommitHeaderHeight(u int) int {
	if u < 1 {
		u = 1
	}
	return u
}

// gitCommitHeader is the column-title row above the commit list.
// Width drags start here and nowhere else in the list.
type gitCommitHeader struct {
	guigui.DefaultWidget

	tool    *GitTool
	graph   basicwidget.Text
	subject basicwidget.Text
	author  basicwidget.Text
	hash    basicwidget.Text
	date    basicwidget.Text

	layoutItems []guigui.LinearLayoutItem
	layout      guigui.LinearLayout
}

func (h *gitCommitHeader) SetLang(lang i18n.Lang) {
	h.graph.SetValue(i18n.T(lang, i18n.GitGraph))
	h.subject.SetValue(i18n.T(lang, i18n.GitTitle))
	h.author.SetValue(i18n.T(lang, i18n.GitAuthor))
	h.hash.SetValue(i18n.T(lang, i18n.GitColID))
	h.date.SetValue(i18n.T(lang, i18n.GitDate))
}

func (h *gitCommitHeader) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&h.graph)
	adder.AddWidget(&h.subject)
	adder.AddWidget(&h.author)
	adder.AddWidget(&h.hash)
	adder.AddWidget(&h.date)
	for _, text := range []*basicwidget.Text{&h.graph, &h.subject, &h.author, &h.hash, &h.date} {
		text.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		text.SetWrapMode(basicwidget.WrapModeNone)
		text.SetSelectable(false)
	}
	h.date.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	return nil
}

func (h *gitCommitHeader) layoutRow(context *guigui.Context, rowW int) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	lanes := 1
	var graphU, authorU, hashU, dateU float64
	if h.tool != nil {
		if h.tool.graphLanes > 0 {
			lanes = h.tool.graphLanes
		}
		h.tool.ensureGraphWidth()
		graphU = h.tool.graphUnits
		authorU = h.tool.authorUnits
		hashU = h.tool.hashUnits
		dateU = h.tool.dateUnits
	}
	graphU = preferGraphUnits(graphU)
	authorU, hashU, dateU = preferMetaUnits(authorU, hashU, dateU)
	graphW := gitGraphColWidth(rowW, u, lanes, graphU)
	authorW, hashW, dateW := gitMetaColWidths(rowW, u, graphW, authorU, hashU, dateU)
	h.layoutItems = slices.Delete(h.layoutItems, 0, len(h.layoutItems))
	h.layoutItems = append(h.layoutItems,
		guigui.LinearLayoutItem{Widget: &h.graph, Size: guigui.FixedSize(graphW)},
		guigui.LinearLayoutItem{Widget: &h.subject, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &h.author, Size: guigui.FixedSize(authorW)},
		guigui.LinearLayoutItem{Widget: &h.hash, Size: guigui.FixedSize(hashW)},
		guigui.LinearLayoutItem{Widget: &h.date, Size: guigui.FixedSize(dateW)},
	)
	h.layout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     h.layoutItems,
		Gap:       gitCommitColGap(u),
	}
	return h.layout
}

func (h *gitCommitHeader) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	var style basicwidget.TextStyle
	useGitMono(&style)
	style.SetBold(true)
	for _, text := range []*basicwidget.Text{&h.graph, &h.subject, &h.author, &h.hash, &h.date} {
		text.SetBaseStyle(&style)
	}
	b := widgetBounds.Bounds()
	inset := basicwidget.RoundedCornerRadius(context)
	inner := b
	inner.Min.X += inset
	inner.Max.X -= inset
	if inner.Max.X < inner.Min.X {
		inner.Max.X = inner.Min.X
	}
	h.layoutRow(context, inner.Dx()).LayoutWidgets(context, inner, layouter)
}

func (h *gitCommitHeader) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 16 * u
	if fixed, ok := constraints.FixedWidth(); ok && fixed > 0 {
		w = fixed
	}
	return image.Pt(w, gitCommitHeaderHeight(u))
}

func (r *gitCommitRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	rowW := 0
	if w, ok := constraints.FixedWidth(); ok {
		rowW = w
	}
	s := r.layoutRow(context, rowW).Measure(context, constraints)
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

// gitGraphSplit is the draggable right edge of the commit-graph column.
type gitGraphSplit struct {
	guigui.DefaultWidget

	tool *GitTool
}

func (s *gitGraphSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil {
		return guigui.HandleInputResult{}
	}
	if s.tool.graphDragging || widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.tool.dragGraphSplit(context)
	}
	return guigui.HandleInputResult{}
}

func (s *gitGraphSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool != nil && s.tool.graphDragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (s *gitGraphSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	x := b.Min.X + b.Dx()/2
	fillRect(dst, image.Rect(x, b.Min.Y, x+1, b.Max.Y), gitHistSplitColor(context.ColorMode()))
}

func (t *GitTool) setGraphUnits(units float64) {
	changed := units != t.graphUnits
	t.graphUnits = units
	for i := 0; i < t.commitRows.Len(); i++ {
		t.commitRows.At(i).graphUnits = units
	}
	t.expand.graphUnits = units
	if changed {
		rememberGitGraphUnits(units)
	}
}

func (t *GitTool) setGraphUnitsFromCursor(x, inset, u int) {
	if t.histGraph.Empty() {
		return
	}
	if u < 1 {
		u = 1
	}
	rowW := t.histGraph.Dx() - 2*inset
	if rowW < 1 {
		rowW = 1
	}
	graphW := x - (t.histGraph.Min.X + inset)
	w := clampGitGraphCol(graphW, rowW, u, t.graphLanes)
	t.setGraphUnits(float64(w) / float64(u))
}

func (t *GitTool) dragGraphSplit(context *guigui.Context) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if !t.graphDragging {
		if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.graphDragging = true
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Keep the last width from while the button was held. Re-sampling after
		// release can snap the column back to the lane width.
		t.stopGraphDrag(context)
		guigui.RequestRebuild()
		return guigui.HandleInputByWidget(t)
	}
	t.setGraphUnitsFromCursor(image.Pt(ebiten.CursorPosition()).X, basicwidget.RoundedCornerRadius(context), u)
	t.setGraphDragPassthrough(context, true)
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(t)
}

func (t *GitTool) ensureGraphWidth() {
	if t.graphWidthLoaded {
		return
	}
	t.graphWidthLoaded = true
	if t.graphUnits > 0 {
		t.ensureMetaWidths()
		return
	}
	if u := preferredGitGraphUnits(); u > 0 {
		t.graphUnits = u
		for i := 0; i < t.commitRows.Len(); i++ {
			t.commitRows.At(i).graphUnits = u
		}
		t.expand.graphUnits = u
	}
	t.ensureMetaWidths()
}

func (t *GitTool) ensureMetaWidths() {
	if t.metaWidthLoaded {
		return
	}
	t.metaWidthLoaded = true
	author, hash, date := preferredGitMetaUnits()
	if t.authorUnits == 0 {
		t.authorUnits = author
	}
	if t.hashUnits == 0 {
		t.hashUnits = hash
	}
	if t.dateUnits == 0 {
		t.dateUnits = date
	}
	for i := 0; i < t.commitRows.Len(); i++ {
		r := t.commitRows.At(i)
		if r.authorUnits == 0 {
			r.authorUnits = t.authorUnits
		}
		if r.hashUnits == 0 {
			r.hashUnits = t.hashUnits
		}
		if r.dateUnits == 0 {
			r.dateUnits = t.dateUnits
		}
	}
}

func (t *GitTool) stopGraphDrag(context *guigui.Context) {
	if !t.graphDragging {
		return
	}
	t.graphDragging = false
	if context != nil {
		t.setGraphDragPassthrough(context, false)
	}
	rememberGitGraphUnits(t.graphUnits)
}

func (t *GitTool) setGraphDragPassthrough(context *guigui.Context, pass bool) {
	for _, w := range []guigui.Widget{
		&t.tabBar,
		&t.sidePane,
		&t.sideSplit,
		&t.fetchBtn,
		&t.pullBtn,
		&t.pushBtn,
		&t.status,
		&t.hint,
		&t.expand,
		&t.commitList,
		&t.graphEmpty,
		&t.histStack.header,
		&t.histStack.diff,
		&t.histStack.split,
	} {
		context.SetPassthrough(w, pass)
	}
}

func (t *GitTool) cursorOnCommitSplit() bool {
	c := image.Pt(ebiten.CursorPosition())
	for _, b := range []image.Rectangle{
		t.graphSplitBounds,
		t.authorSplitBounds,
		t.hashSplitBounds,
		t.dateSplitBounds,
	} {
		if !b.Empty() && c.In(b) {
			return true
		}
	}
	return false
}

const (
	gitMetaDragAuthor = 1
	gitMetaDragHash   = 2
	gitMetaDragDate   = 3
)

// gitMetaSplit is the draggable boundary of the title, author, commit id, or date.
// The title boundary resizes the author and lets the title take the difference.
// The author boundary trades width with the commit id. The commit-id boundary
// trades width with the date.
type gitMetaSplit struct {
	guigui.DefaultWidget

	tool *GitTool
	kind int
}

func (s *gitMetaSplit) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if s.tool == nil || s.tool.graphDragging {
		return guigui.HandleInputResult{}
	}
	if s.tool.metaDrag == s.kind || s.tool.metaDrag == 0 && widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.tool.dragMetaSplit(context, s.kind)
	}
	return guigui.HandleInputResult{}
}

func (s *gitMetaSplit) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.tool != nil && s.tool.metaDrag == s.kind || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (s *gitMetaSplit) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	x := b.Min.X + b.Dx()/2
	fillRect(dst, image.Rect(x, b.Min.Y, x+1, b.Max.Y), gitHistSplitColor(context.ColorMode()))
}

func (t *GitTool) applyMetaUnits(author, hash, date float64) {
	changed := author != t.authorUnits || hash != t.hashUnits || date != t.dateUnits
	t.authorUnits = author
	t.hashUnits = hash
	t.dateUnits = date
	for i := 0; i < t.commitRows.Len(); i++ {
		r := t.commitRows.At(i)
		r.authorUnits = author
		r.hashUnits = hash
		r.dateUnits = date
	}
	if changed {
		rememberGitMetaUnits(author, hash, date)
	}
}

func (t *GitTool) resizeMetaFromCursor(x, inset, u int) {
	if t.histGraph.Empty() || u < 1 {
		return
	}
	rowLeft := t.histGraph.Min.X + inset
	rowRight := t.histGraph.Max.X - inset
	rowW := rowRight - rowLeft
	if rowW < 1 {
		return
	}
	graphW := gitGraphColWidth(rowW, u, t.graphLanes, preferGraphUnits(t.graphUnits))
	authorU, hashU, dateU := preferMetaUnits(t.authorUnits, t.hashUnits, t.dateUnits)
	authorW, hashW, dateW := gitMetaColWidths(rowW, u, graphW, authorU, hashU, dateU)
	gap := gitCommitColGap(u)
	_, authorL, authorR, hashL, hashR, _, dateR := gitMetaEdges(rowRight, gap, authorW, hashW, dateW)
	switch t.metaDrag {
	case gitMetaDragAuthor:
		authorW = gitResizeAuthor(authorR, gap, x, gitMinAuthorCol(u), gitMetaRoom(rowW, u, graphW, hashW, dateW, gitMinAuthorCol(u)))
	case gitMetaDragHash:
		authorW, hashW = gitTradeCols(authorL, hashR, gap, x, gitMinAuthorCol(u), gitMinHashCol(u))
	case gitMetaDragDate:
		hashW, dateW = gitTradeCols(hashL, dateR, gap, x, gitMinHashCol(u), gitMinDateCol(u))
	default:
		return
	}
	t.applyMetaUnits(float64(authorW)/float64(u), float64(hashW)/float64(u), float64(dateW)/float64(u))
}

func (t *GitTool) dragMetaSplit(context *guigui.Context, kind int) guigui.HandleInputResult {
	u := basicwidget.UnitSize(context)
	if t.metaDrag == 0 {
		if kind == 0 || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.metaDrag = kind
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Keep the last width from while the button was held.
		t.metaDrag = 0
		t.setGraphDragPassthrough(context, false)
		guigui.RequestRebuild()
		return guigui.HandleInputByWidget(t)
	}
	t.resizeMetaFromCursor(image.Pt(ebiten.CursorPosition()).X, basicwidget.RoundedCornerRadius(context), u)
	t.setGraphDragPassthrough(context, true)
	guigui.RequestRebuild()
	return guigui.HandleInputByWidget(t)
}

// gitSpin is the history loading indicator: a ring of dots that chase a head.
type gitSpin struct {
	guigui.DefaultWidget
}

func (s *gitSpin) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteInt64(time.Now().UnixMilli() / 32)
}

func (s *gitSpin) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	size := image.Pt(u, u)
	if w, ok := constraints.FixedWidth(); ok {
		size.X = w
	}
	if h, ok := constraints.FixedHeight(); ok {
		size.Y = h
	}
	return size
}

func (s *gitSpin) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	guigui.RequestRebuild()
	return nil
}

func (s *gitSpin) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Dx() < 2 || b.Dy() < 2 {
		return
	}
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	radius := float32(side) * 0.28
	if radius > 11 {
		radius = 11
	}
	if radius < 5 {
		radius = 5
	}
	dot := radius * 0.28
	if dot < 1.4 {
		dot = 1.4
	}
	cx := float32(b.Min.X + b.Dx()/2)
	cy := float32(b.Min.Y + b.Dy()/2)
	bg := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xe6}
	ink := color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}
	if context.ColorMode() == ebiten.ColorModeDark {
		bg = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xdd}
		ink = color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	}
	vector.DrawFilledCircle(dst, cx, cy, radius+dot*1.8, bg, true)

	const n = 8
	turn := float64(time.Now().UnixMilli()%1000) / 1000
	head := turn * n
	for i := 0; i < n; i++ {
		ang := (float64(i)/float64(n))*2*math.Pi - math.Pi/2
		x := cx + radius*float32(math.Cos(ang))
		y := cy + radius*float32(math.Sin(ang))
		dist := head - float64(i)
		if dist < 0 {
			dist += n
		}
		fade := 1 - dist/n
		c := ink
		c.A = uint8(36 + 219*fade*fade)
		vector.DrawFilledCircle(dst, x, y, dot, c, true)
	}
}
