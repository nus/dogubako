package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
	"github.com/nus/dogubako/internal/i18n"
)

// gitHeadButton is the icon at the right end of the History Fetch / Pull / Push
// row. It has no label. Hovering it shows its name.
type gitHeadButton struct {
	guigui.DefaultWidget

	button  basicwidget.Button
	tip     basicwidget.TooltipArea
	enabled bool
	onDown  func(*guigui.Context)
}

func (b *gitHeadButton) set(context *guigui.Context, lang i18n.Lang, enabled bool, onDown func(*guigui.Context)) {
	b.enabled = enabled
	b.onDown = onDown
	b.button.SetIcon(gitHeadIcon(context.ColorMode(), basicwidget.LineHeight(context)))
	b.button.SetText("")
	b.button.OnDown(func(ctx *guigui.Context) {
		if b.onDown != nil && b.enabled {
			b.onDown(ctx)
		}
	})
	b.tip.SetText(i18n.T(lang, i18n.GitJumpHead))
}

func (b *gitHeadButton) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetEnabled(&b.button, b.enabled)
	adder.AddWidget(&b.button)
	adder.AddWidget(&b.tip)
	return nil
}

func (b *gitHeadButton) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&b.button, bounds)
	layouter.LayoutWidget(&b.tip, bounds)
}

type gitHeadIconKey struct {
	mode ebiten.ColorMode
	size int
}

var gitHeadIcons = map[gitHeadIconKey]*ebiten.Image{}

// gitHeadIcon draws a circle with a short check, in the text color.
// Each color mode and size is drawn once.
func gitHeadIcon(mode ebiten.ColorMode, size int) *ebiten.Image {
	if size < 16 {
		size = 16
	}
	key := gitHeadIconKey{mode: mode, size: size}
	if img, ok := gitHeadIcons[key]; ok {
		return img
	}
	raster := size * 2
	img := ebiten.NewImage(raster, raster)
	sw := headJumpStroke(float32(raster))
	cx, cy, r := headJumpCircle(float32(raster))
	col := textColor(mode)
	vector.StrokeCircle(img, cx, cy, r, sw, col, true)
	for _, s := range headJumpCheck(float32(raster)) {
		vector.StrokeLine(img, s[0], s[1], s[2], s[3], sw, col, true)
	}
	gitHeadIcons[key] = img
	return img
}

func headJumpStroke(size float32) float32 {
	sw := size / 12
	if sw < 2 {
		sw = 2
	}
	return sw
}

// headJumpCircle is the ring around the check, in a size×size box.
func headJumpCircle(size float32) (cx, cy, r float32) {
	return size * 0.5, size * 0.5, size * 0.34
}

// headJumpCheck is a short check inside the ring. It does not reach the stroke.
func headJumpCheck(size float32) [][4]float32 {
	return [][4]float32{
		{size * 0.40, size * 0.52, size * 0.48, size * 0.62},
		{size * 0.48, size * 0.62, size * 0.64, size * 0.42},
	}
}

// headJumpView is the history state one frame of a jump looks at.
type headJumpView struct {
	tab      int
	path     string
	jumpTab  int
	jumpPath string
	head     string
	inGraph  bool
	hasMore  bool
	loading  bool
	busy     bool
	limit    int
	asked    int
}

// stepHeadJump follows the checked-out commit for one frame.
// next is the hash still being followed. sel is the row to select and
// scroll, once. load asks for the next history page.
// A different tab or repository, an empty HEAD, or a history that never
// contains the commit selects nothing.
func stepHeadJump(pending string, v headJumpView) (next, sel string, load bool) {
	if pending == "" {
		return "", "", false
	}
	if v.tab != v.jumpTab || v.path != v.jumpPath {
		return "", "", false
	}
	head := strings.TrimSpace(v.head)
	if head == "" || head == gitcli.Uncommitted {
		return "", "", false
	}
	if v.inGraph {
		return "", head, false
	}
	if !v.hasMore {
		return "", "", false
	}
	if v.loading || v.busy {
		return head, "", false
	}
	if v.asked != 0 && v.asked == v.limit {
		return "", "", false
	}
	return head, "", true
}

// headJumpReady reports whether the History icon can be pressed.
// A row already on screen stays pressable during fetch or commit.
// A row that still needs another page waits until that operation finishes.
// An empty graph that is still loading its first page stays disabled.
func headJumpReady(hasRepo, busy, historyLoading, graphEmpty, inGraph, hasMore bool, head string) bool {
	head = strings.TrimSpace(head)
	if !hasRepo || head == "" || head == gitcli.Uncommitted {
		return false
	}
	if graphEmpty && historyLoading {
		return false
	}
	if inGraph {
		return true
	}
	return hasMore && !busy
}

func graphHasCommit(rows []gitcli.GraphRow, hash string) bool {
	hash = strings.TrimSpace(hash)
	if hash == "" || hash == gitcli.Uncommitted {
		return false
	}
	for _, row := range rows {
		if row.Commit.Hash == hash {
			return true
		}
	}
	return false
}

func (t *GitTool) canJumpHead(model *GitModel) bool {
	if model == nil {
		return false
	}
	snap := model.Snapshot()
	return headJumpReady(
		model.HasRepo(),
		model.Busy(),
		model.HistoryLoading(),
		len(snap.Graph) == 0,
		graphHasCommit(snap.Graph, snap.HEAD),
		model.HasMoreHistory(),
		snap.HEAD,
	)
}

func (t *GitTool) clearHeadJump() {
	t.headJump = ""
	t.headJumpAsked = 0
}

// noteHeadJump drops a jump that belongs to another tab or repository.
func (t *GitTool) noteHeadJump(model *GitModel) {
	if t.headJump == "" || model == nil {
		return
	}
	if model.ActiveTab() != t.headJumpTab || model.Path() != t.headJumpPath {
		t.clearHeadJump()
	}
}

// startHeadJump asks History to select the checked-out commit and bring
// its row into view. It does not check out a branch or open the detail panel.
func (t *GitTool) startHeadJump(model *GitModel) {
	if model == nil || !t.canJumpHead(model) {
		return
	}
	head := strings.TrimSpace(model.Snapshot().HEAD)
	if head == "" || head == gitcli.Uncommitted {
		return
	}
	t.headJump = head
	t.headJumpTab = model.ActiveTab()
	t.headJumpPath = model.Path()
	t.headJumpAsked = 0
	guigui.RequestRebuild()
}

// syncHeadJump advances a jump started by the History icon.
// The returned hash should be scrolled into view once. An empty return
// means the row is not in the list yet. The status line is left alone.
// list may be nil when the graph is not on screen; the request is kept.
func (t *GitTool) syncHeadJump(model *GitModel, list *basicwidget.List[string]) string {
	if t.headJump == "" || model == nil {
		return ""
	}
	snap := model.Snapshot()
	next, sel, load := stepHeadJump(t.headJump, headJumpView{
		tab:      model.ActiveTab(),
		path:     model.Path(),
		jumpTab:  t.headJumpTab,
		jumpPath: t.headJumpPath,
		head:     snap.HEAD,
		inGraph:  graphHasCommit(snap.Graph, snap.HEAD),
		hasMore:  model.HasMoreHistory(),
		loading:  model.HistoryLoading(),
		busy:     model.Busy(),
		limit:    model.historyLimit(),
		asked:    t.headJumpAsked,
	})
	if load {
		t.headJump = next
		t.headJumpAsked = model.historyLimit()
		model.LoadMoreHistory()
		return ""
	}
	if sel != "" {
		if list != nil && list.IndexByValue(sel) >= 0 {
			t.clearHeadJump()
			model.HighlightCommit(sel)
			return sel
		}
		// The commit is in the snapshot, but the list does not have the row
		// yet. Keep the request so the scroll is not dropped.
		t.headJump = sel
		return ""
	}
	t.headJump = next
	if next == "" {
		t.headJumpAsked = 0
	}
	return ""
}
