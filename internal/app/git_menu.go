package app

import (
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/nus/dogubako/internal/gitcli"
)

// gitBranchMenu is the context menu opened by right-clicking a branch.
type gitBranchMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos         image.Point
	ref         gitcli.Ref
	pushLabel   string
	renameLabel string
	deleteLabel string
	onPush      func(gitcli.Ref)
	onRename    func(gitcli.Ref)
	onDelete    func(gitcli.Ref)
}

func (m *gitBranchMenu) Open(ref gitcli.Ref, at image.Point, pushLabel, renameLabel, deleteLabel string, onPush, onRename, onDelete func(gitcli.Ref)) {
	m.ref = ref
	m.pos = at
	m.pushLabel = pushLabel
	m.renameLabel = renameLabel
	m.deleteLabel = deleteLabel
	m.onPush = onPush
	m.onRename = onRename
	m.onDelete = onDelete
	m.menu.SetOpen(true)
}

func (m *gitBranchMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

// branchMenuItems is Push, then rename, then delete.
// Push and rename are disabled for a remote-tracking branch.
// Delete is disabled for the checked-out branch.
func branchMenuItems(ref gitcli.Ref, push, rename, delete string) []basicwidget.PopupMenuItem[string] {
	return []basicwidget.PopupMenuItem[string]{
		{
			Text:     push,
			Value:    "push",
			Disabled: ref.Remote != "",
		},
		{
			Text:     rename,
			Value:    "rename",
			Disabled: ref.Remote != "",
		},
		{
			Text:     delete,
			Value:    "delete",
			Disabled: ref.Current,
		},
	}
}

func (m *gitBranchMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems(branchMenuItems(m.ref, m.pushLabel, m.renameLabel, m.deleteLabel))
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok {
			return
		}
		switch item.Value {
		case "push":
			if m.ref.Remote != "" || m.onPush == nil {
				return
			}
			m.onPush(m.ref)
		case "rename":
			if m.ref.Remote != "" || m.onRename == nil {
				return
			}
			m.onRename(m.ref)
		case "delete":
			if m.ref.Current || m.onDelete == nil {
				return
			}
			m.onDelete(m.ref)
		}
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitBranchMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitBranchMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitBranchMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// gitCommitMenu is the context menu opened by right-clicking a commit.
type gitCommitMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos            image.Point
	hash           string
	branchLabel    string
	tagLabel       string
	rebaseLabel    string
	rebaseDisabled bool
	onBranch       func(hash string)
	onTag          func(hash string)
	onRebase       func(hash string)
}

func (m *gitCommitMenu) Open(hash string, at image.Point, branchLabel, tagLabel, rebaseLabel string, rebaseDisabled bool, onBranch, onTag, onRebase func(hash string)) {
	m.hash = hash
	m.pos = at
	m.branchLabel = branchLabel
	m.tagLabel = tagLabel
	m.rebaseLabel = rebaseLabel
	m.rebaseDisabled = rebaseDisabled
	m.onBranch = onBranch
	m.onTag = onTag
	m.onRebase = onRebase
	m.menu.SetOpen(true)
}

func (m *gitCommitMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

// commitMenuItems is create branch, create tag, then rebase onto this commit.
// Rebase stays visible when it cannot run. Branch and tag stay enabled.
func commitMenuItems(branch, tag, rebase string, rebaseDisabled bool) []basicwidget.PopupMenuItem[string] {
	return []basicwidget.PopupMenuItem[string]{
		{Text: branch, Value: "branch"},
		{Text: tag, Value: "tag"},
		{Text: rebase, Value: "rebase", Disabled: rebaseDisabled},
	}
}

// commitRebaseDisabled reports that there is no branch to replay, or the
// clicked commit is already HEAD.
func commitRebaseDisabled(detached bool, branch, head, hash string) bool {
	if detached || branch == "" {
		return true
	}
	return head != "" && hash == head
}

func (m *gitCommitMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems(commitMenuItems(m.branchLabel, m.tagLabel, m.rebaseLabel, m.rebaseDisabled))
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok || m.hash == "" {
			return
		}
		switch item.Value {
		case "branch":
			if m.onBranch != nil {
				m.onBranch(m.hash)
			}
		case "tag":
			if m.onTag != nil {
				m.onTag(m.hash)
			}
		case "rebase":
			if m.rebaseDisabled || m.onRebase == nil {
				return
			}
			m.onRebase(m.hash)
		}
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitCommitMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitCommitMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitCommitMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// gitSelectionMenu stages or unstages the lines selected in the diff.
type gitSelectionMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos     image.Point
	patch   string
	unstage bool
	label   string
	onAct   func(patch string, unstage bool)
}

func (m *gitSelectionMenu) Open(patch string, unstage bool, at image.Point, label string, onAct func(string, bool)) {
	m.patch = patch
	m.unstage = unstage
	m.pos = at
	m.label = label
	m.onAct = onAct
	m.menu.SetOpen(true)
}

func (m *gitSelectionMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitSelectionMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems([]basicwidget.PopupMenuItem[string]{
		{Text: m.label, Value: "act"},
	})
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok || item.Value != "act" || m.onAct == nil || m.patch == "" {
			return
		}
		m.onAct(m.patch, m.unstage)
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitSelectionMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitSelectionMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitSelectionMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// gitChangeMenu is the context menu opened by right-clicking a working-copy change.
type gitChangeMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos          image.Point
	stageLabel   string
	discardLabel string
	staged       bool
	onStage      func(unstage bool)
	onDiscard    func()
}

func (m *gitChangeMenu) Open(at image.Point, stageLabel, discardLabel string, staged bool, onStage func(bool), onDiscard func()) {
	m.pos = at
	m.stageLabel = stageLabel
	m.discardLabel = discardLabel
	m.staged = staged
	m.onStage = onStage
	m.onDiscard = onDiscard
	m.menu.SetOpen(true)
}

func (m *gitChangeMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitChangeMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems([]basicwidget.PopupMenuItem[string]{
		{Text: m.stageLabel, Value: "stage"},
		{Border: true},
		{Text: m.discardLabel, Value: "discard"},
	})
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok {
			return
		}
		switch item.Value {
		case "stage":
			if m.onStage != nil {
				m.onStage(m.staged)
			}
		case "discard":
			if m.onDiscard != nil {
				m.onDiscard()
			}
		}
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitChangeMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitChangeMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitChangeMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// gitTagMenu is the context menu opened by right-clicking a tag.
type gitTagMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos               image.Point
	name              string
	deleteLabel       string
	pushLabel         string
	remoteDeleteLabel string
	onDelete          func(name string)
	onPush            func(name string)
	onRemoteDelete    func(name string)
}

func (m *gitTagMenu) Open(name string, at image.Point, deleteLabel, pushLabel, remoteDeleteLabel string, onDelete, onPush, onRemoteDelete func(name string)) {
	m.name = name
	m.pos = at
	m.deleteLabel = deleteLabel
	m.pushLabel = pushLabel
	m.remoteDeleteLabel = remoteDeleteLabel
	m.onDelete = onDelete
	m.onPush = onPush
	m.onRemoteDelete = onRemoteDelete
	m.menu.SetOpen(true)
}

func (m *gitTagMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitTagMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems([]basicwidget.PopupMenuItem[string]{
		{Text: m.deleteLabel, Value: "delete"},
		{Text: m.pushLabel, Value: "push"},
		{Text: m.remoteDeleteLabel, Value: "remote-delete"},
	})
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok || m.name == "" {
			return
		}
		switch item.Value {
		case "delete":
			if m.onDelete != nil {
				m.onDelete(m.name)
			}
		case "push":
			if m.onPush != nil {
				m.onPush(m.name)
			}
		case "remote-delete":
			if m.onRemoteDelete != nil {
				m.onRemoteDelete(m.name)
			}
		}
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitTagMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitTagMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitTagMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// gitPushDialog asks which push to send after the Push button is pressed.
type gitPushDialog struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content gitPushDialogContent
}

func (d *gitPushDialog) Ask(message, normal, lease, force, cancel string, onChoose func(gitcli.PushMode)) {
	d.content.messageText = message
	d.content.normalText = normal
	d.content.leaseText = lease
	d.content.forceText = force
	d.content.cancelText = cancel
	d.content.onChoose = onChoose
	d.popup.SetOpen(true)
}

func (d *gitPushDialog) IsOpen() bool {
	return d.popup.IsOpen()
}

func (d *gitPushDialog) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	d.content.popup = &d.popup
	d.popup.SetContent(&d.content)
	d.popup.SetModal(true)
	d.popup.SetBackgroundDark(true)
	d.popup.SetCloseByClickingOutside(true)
	d.popup.SetAnimated(true)
	if d.popup.IsOpen() {
		adder.AddWidget(&d.popup)
	}
	return nil
}

func (d *gitPushDialog) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&d.popup, widgetBounds.Bounds())
}

func (d *gitPushDialog) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (d *gitPushDialog) contentSize(context *guigui.Context) image.Point {
	return d.content.Measure(context, guigui.Constraints{})
}

type gitPushDialogContent struct {
	guigui.DefaultWidget

	popup *basicwidget.Popup

	message basicwidget.Text
	normal  basicwidget.Button
	lease   basicwidget.Button
	force   basicwidget.Button
	cancel  basicwidget.Button

	messageText string
	normalText  string
	leaseText   string
	forceText   string
	cancelText  string
	onChoose    func(gitcli.PushMode)

	layoutItems []guigui.LinearLayoutItem
}

func (c *gitPushDialogContent) choose(mode gitcli.PushMode) {
	on := c.onChoose
	c.onChoose = nil
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
	if on != nil {
		on(mode)
	}
}

func (c *gitPushDialogContent) close() {
	c.onChoose = nil
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
}

func (c *gitPushDialogContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&c.message)
	adder.AddWidget(&c.normal)
	adder.AddWidget(&c.lease)
	adder.AddWidget(&c.force)
	adder.AddWidget(&c.cancel)

	c.message.SetValue(c.messageText)
	c.message.SetMultiline(true)
	c.message.SetWrapMode(basicwidget.WrapModeNormal)
	c.message.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.message.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	// Escape must reach this dialog even when focus is still on the history.
	context.SetButtonInputReceptive(c, true)

	c.normal.SetText(c.normalText)
	c.normal.SetType(basicwidget.ButtonTypePrimary)
	c.normal.OnDown(func(context *guigui.Context) {
		c.choose(gitcli.PushNormal)
	})
	c.lease.SetText(c.leaseText)
	c.lease.OnDown(func(context *guigui.Context) {
		c.choose(gitcli.PushForceWithLease)
	})
	c.force.SetText(c.forceText)
	c.force.OnDown(func(context *guigui.Context) {
		c.choose(gitcli.PushForce)
	})
	c.cancel.SetText(c.cancelText)
	c.cancel.OnDown(func(context *guigui.Context) {
		c.close()
	})
	return nil
}

func (c *gitPushDialogContent) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if c.popup == nil || !c.popup.IsOpen() {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.close()
		return guigui.HandleInputByWidget(c)
	}
	return guigui.HandleInputResult{}
}

func (c *gitPushDialogContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 16 * u
	for _, b := range []*basicwidget.Button{&c.normal, &c.lease, &c.force, &c.cancel} {
		if need := b.Measure(context, guigui.Constraints{}).X + u; need > w {
			w = need
		}
	}
	msgH := c.message.Measure(context, guigui.FixedWidthConstraints(w-u)).Y
	btnH := c.normal.Measure(context, guigui.Constraints{}).Y
	h := msgH + 4*btnH + 3*(u/4) + u/2 + u
	return image.Pt(w, h)
}

func (c *gitPushDialogContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	btnH := c.normal.Measure(context, guigui.Constraints{}).Y
	c.layoutItems = slices.Delete(c.layoutItems, 0, len(c.layoutItems))
	c.layoutItems = append(c.layoutItems,
		guigui.LinearLayoutItem{Widget: &c.message, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.normal, Size: guigui.FixedSize(btnH)},
		guigui.LinearLayoutItem{Widget: &c.lease, Size: guigui.FixedSize(btnH)},
		guigui.LinearLayoutItem{Widget: &c.force, Size: guigui.FixedSize(btnH)},
		guigui.LinearLayoutItem{Widget: &c.cancel, Size: guigui.FixedSize(btnH)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.layoutItems,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// worktreeCloseDisabled reports that a working tree cannot be removed.
// The main tree is one of these, and a bare parent is the main tree.
func worktreeCloseDisabled(wt gitcli.Worktree) bool {
	return wt.Main || wt.Bare || wt.Path == ""
}

// worktreeMenuItems is only Close. Force delete is not a menu item.
// Close is disabled for the main working tree.
func worktreeMenuItems(label string, disabled bool) []basicwidget.PopupMenuItem[string] {
	return []basicwidget.PopupMenuItem[string]{
		{Text: label, Value: "close", Disabled: disabled},
	}
}

// gitWorktreeMenu is the context menu opened by right-clicking a worktree row.
type gitWorktreeMenu struct {
	guigui.DefaultWidget

	menu basicwidget.PopupMenu[string]

	pos      image.Point
	label    string
	disabled bool
	onClose  func()
}

func (m *gitWorktreeMenu) Open(disabled bool, at image.Point, label string, onClose func()) {
	m.disabled = disabled
	m.pos = at
	m.label = label
	m.onClose = onClose
	m.menu.SetOpen(true)
}

func (m *gitWorktreeMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitWorktreeMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems(worktreeMenuItems(m.label, m.disabled))
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok || item.Value != "close" || m.disabled || m.onClose == nil {
			return
		}
		m.onClose()
	})
	if m.menu.IsOpen() {
		adder.AddWidget(&m.menu)
	}
	return nil
}

func (m *gitWorktreeMenu) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.menu, widgetBounds.Bounds())
}

func (m *gitWorktreeMenu) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (m *gitWorktreeMenu) contentSize(context *guigui.Context) image.Point {
	return m.menu.Measure(context, guigui.Constraints{})
}

// worktreeRemoveChoice is a dialog option: the action, then the remove
// command's option in parentheses.
func worktreeRemoveChoice(action, option string) string {
	return action + " (" + option + ")"
}

// gitWorktreeDialog asks whether to remove a linked worktree, or force-remove it.
// The choices are radio buttons. Close runs the selected one. Cancel runs nothing.
type gitWorktreeDialog struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content gitWorktreeDialogContent
}

func (d *gitWorktreeDialog) Ask(message, closeText, forceText, okText, cancelText string, onChoose func(force bool)) {
	d.content.messageText = message
	d.content.closeText = closeText
	d.content.forceText = forceText
	d.content.okText = okText
	d.content.cancelText = cancelText
	d.content.onChoose = onChoose
	d.content.labelPress = 0
	d.content.message.SetValue(message)
	d.content.closeLbl.SetValue(closeText)
	d.content.forceLbl.SetValue(forceText)
	d.content.ok.SetText(okText)
	d.content.cancel.SetText(cancelText)
	d.content.group.SetValues([]bool{false, true})
	d.content.group.SelectItemByValue(false)
	d.popup.SetOpen(true)
}

func (d *gitWorktreeDialog) IsOpen() bool {
	return d.popup.IsOpen()
}

func (d *gitWorktreeDialog) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	d.content.popup = &d.popup
	d.popup.SetContent(&d.content)
	d.popup.SetModal(true)
	d.popup.SetBackgroundDark(true)
	d.popup.SetCloseByClickingOutside(true)
	d.popup.SetAnimated(true)
	if d.popup.IsOpen() {
		adder.AddWidget(&d.popup)
	}
	return nil
}

func (d *gitWorktreeDialog) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&d.popup, widgetBounds.Bounds())
}

func (d *gitWorktreeDialog) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (d *gitWorktreeDialog) contentSize(context *guigui.Context) image.Point {
	return d.content.Measure(context, guigui.Constraints{})
}

type gitWorktreeDialogContent struct {
	guigui.DefaultWidget

	popup *basicwidget.Popup

	message  basicwidget.Text
	group    basicwidget.RadioButtonGroup[bool]
	closeLbl basicwidget.Text
	forceLbl basicwidget.Text
	ok       basicwidget.Button
	cancel   basicwidget.Button

	messageText string
	closeText   string
	forceText   string
	okText      string
	cancelText  string
	onChoose    func(force bool)
	// labelPress is 1 or 2 while the pointer is down on a choice label.
	labelPress int
	closeHit   image.Rectangle
	forceHit   image.Rectangle

	closeItems  []guigui.LinearLayoutItem
	forceItems  []guigui.LinearLayoutItem
	rowItems    []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
}

func (c *gitWorktreeDialogContent) submit() {
	force, ok := c.group.SelectedValue()
	if !ok {
		force = false
	}
	on := c.onChoose
	c.onChoose = nil
	c.labelPress = 0
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
	if on != nil {
		on(force)
	}
}

func (c *gitWorktreeDialogContent) close() {
	c.onChoose = nil
	c.labelPress = 0
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
}

func (c *gitWorktreeDialogContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	c.group.SetValues([]bool{false, true})
	if _, ok := c.group.SelectedValue(); !ok {
		c.group.SelectItemByValue(false)
	}
	closeBtn := c.group.RadioButton(0)
	forceBtn := c.group.RadioButton(1)

	adder.AddWidget(&c.message)
	adder.AddWidget(&c.group)
	adder.AddWidget(closeBtn)
	adder.AddWidget(&c.closeLbl)
	adder.AddWidget(forceBtn)
	adder.AddWidget(&c.forceLbl)
	adder.AddWidget(&c.cancel)
	adder.AddWidget(&c.ok)

	c.message.SetValue(c.messageText)
	c.message.SetMultiline(true)
	c.message.SetWrapMode(basicwidget.WrapModeNormal)
	c.message.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.message.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	c.closeLbl.SetValue(c.closeText)
	c.closeLbl.SetSelectable(false)
	c.closeLbl.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	c.forceLbl.SetValue(c.forceText)
	c.forceLbl.SetSelectable(false)
	c.forceLbl.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	// Escape must reach this dialog even when focus is still on the history.
	context.SetButtonInputReceptive(c, true)

	c.ok.SetText(c.okText)
	c.ok.SetType(basicwidget.ButtonTypePrimary)
	c.ok.OnDown(func(context *guigui.Context) {
		c.submit()
	})
	c.cancel.SetText(c.cancelText)
	c.cancel.OnDown(func(context *guigui.Context) {
		c.close()
	})
	return nil
}

func (c *gitWorktreeDialogContent) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if c.popup == nil || !c.popup.IsOpen() {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.close()
		return guigui.HandleInputByWidget(c)
	}
	return guigui.HandleInputResult{}
}

func (c *gitWorktreeDialogContent) choiceRowHeight(context *guigui.Context) int {
	h := c.group.RadioButton(0).Measure(context, guigui.Constraints{}).Y
	if textH := c.closeLbl.Measure(context, guigui.Constraints{}).Y; textH > h {
		h = textH
	}
	if textH := c.forceLbl.Measure(context, guigui.Constraints{}).Y; textH > h {
		h = textH
	}
	return h
}

func (c *gitWorktreeDialogContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	c.group.SetValues([]bool{false, true})
	u := basicwidget.UnitSize(context)
	w := 18 * u
	radio := c.group.RadioButton(0).Measure(context, guigui.Constraints{}).X
	gap := u / 4
	for _, lbl := range []*basicwidget.Text{&c.closeLbl, &c.forceLbl} {
		if need := radio + gap + lbl.Measure(context, guigui.Constraints{}).X + u; need > w {
			w = need
		}
	}
	cancelW := c.cancel.Measure(context, guigui.Constraints{}).X
	okW := c.ok.Measure(context, guigui.Constraints{}).X
	if need := cancelW + okW + gap + u; need > w {
		w = need
	}
	msgH := c.message.Measure(context, guigui.FixedWidthConstraints(w-u)).Y
	rowH := c.choiceRowHeight(context)
	btnH := c.ok.Measure(context, guigui.Constraints{}).Y
	if cancelH := c.cancel.Measure(context, guigui.Constraints{}).Y; cancelH > btnH {
		btnH = cancelH
	}
	h := msgH + rowH + rowH + btnH + 3*(u/2) + u
	return image.Pt(w, h)
}

func (c *gitWorktreeDialogContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.group.SetValues([]bool{false, true})
	u := basicwidget.UnitSize(context)
	bounds := widgetBounds.Bounds()
	rowH := c.choiceRowHeight(context)
	btnH := c.ok.Measure(context, guigui.Constraints{}).Y
	if cancelH := c.cancel.Measure(context, guigui.Constraints{}).Y; cancelH > btnH {
		btnH = cancelH
	}
	radio := c.group.RadioButton(0).Measure(context, guigui.Constraints{}).X
	gap := u / 4
	cancelW := c.cancel.Measure(context, guigui.Constraints{}).X
	okW := c.ok.Measure(context, guigui.Constraints{}).X
	c.closeHit = image.Rectangle{}
	c.forceHit = image.Rectangle{}

	choiceRow := func(items *[]guigui.LinearLayoutItem, radioBtn guigui.Widget, label *basicwidget.Text) guigui.LinearLayout {
		*items = slices.Delete(*items, 0, len(*items))
		*items = append(*items,
			guigui.LinearLayoutItem{Widget: radioBtn, Size: guigui.FixedSize(radio)},
			guigui.LinearLayoutItem{Widget: label, Size: guigui.FlexibleSize(1)},
		)
		return guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     *items,
			Gap:       gap,
		}
	}
	closeRow := choiceRow(&c.closeItems, c.group.RadioButton(0), &c.closeLbl)
	forceRow := choiceRow(&c.forceItems, c.group.RadioButton(1), &c.forceLbl)

	c.rowItems = slices.Delete(c.rowItems, 0, len(c.rowItems))
	c.rowItems = append(c.rowItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.cancel, Size: guigui.FixedSize(cancelW)},
		guigui.LinearLayoutItem{Widget: &c.ok, Size: guigui.FixedSize(okW)},
	)
	buttons := guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.rowItems,
		Gap:       gap,
	}

	c.layoutItems = slices.Delete(c.layoutItems, 0, len(c.layoutItems))
	c.layoutItems = append(c.layoutItems,
		guigui.LinearLayoutItem{Widget: &c.message, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(rowH), Layout: &closeRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(rowH), Layout: &forceRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(btnH), Layout: &buttons},
	)
	pad := u / 2
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.layoutItems,
		Gap:       u / 2,
		Padding:   guigui.Padding{Start: pad, Top: pad, End: pad, Bottom: pad},
	}).LayoutWidgets(context, bounds, layouter)

	// The label sits beside the radio. A click there selects that choice.
	innerX := bounds.Min.X + pad + radio + gap
	innerR := bounds.Max.X - pad
	msgH := bounds.Dy() - pad*2 - rowH - rowH - btnH - 3*(u/2)
	if msgH < 0 {
		msgH = 0
	}
	y := bounds.Min.Y + pad + msgH + u/2
	c.closeHit = image.Rect(innerX, y, innerR, y+rowH)
	y += rowH + u/2
	c.forceHit = image.Rect(innerX, y, innerR, y+rowH)
}

func (c *gitWorktreeDialogContent) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	cursor := image.Pt(ebiten.CursorPosition())
	hit := 0
	switch {
	case cursor.In(c.closeHit):
		hit = 1
	case cursor.In(c.forceHit):
		hit = 2
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && hit != 0 {
		c.labelPress = hit
		return guigui.HandleInputByWidget(c)
	}
	if c.labelPress != 0 && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		idx := c.labelPress - 1
		pressed := c.labelPress
		c.labelPress = 0
		if hit == pressed {
			c.group.SelectItemByIndex(idx)
			guigui.RequestRedraw(c.group.RadioButton(idx))
		}
		return guigui.HandleInputByWidget(c)
	}
	return guigui.HandleInputResult{}
}

func (c *gitWorktreeDialogContent) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	cursor := image.Pt(ebiten.CursorPosition())
	if cursor.In(c.closeHit) || cursor.In(c.forceHit) {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

// gitConfirm asks before deleting a branch.
type gitConfirm struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content gitConfirmContent
}

func (c *gitConfirm) Ask(message, deleteText, cancelText string, onYes func()) {
	c.content.messageText = message
	c.content.deleteText = deleteText
	c.content.cancelText = cancelText
	c.content.onYes = onYes
	c.popup.SetOpen(true)
}

func (c *gitConfirm) IsOpen() bool {
	return c.popup.IsOpen()
}

func (c *gitConfirm) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	c.content.popup = &c.popup
	c.popup.SetContent(&c.content)
	c.popup.SetModal(true)
	c.popup.SetBackgroundDark(true)
	c.popup.SetCloseByClickingOutside(true)
	c.popup.SetAnimated(true)
	if c.popup.IsOpen() {
		adder.AddWidget(&c.popup)
	}
	return nil
}

func (c *gitConfirm) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&c.popup, widgetBounds.Bounds())
}

func (c *gitConfirm) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (c *gitConfirm) contentSize(context *guigui.Context) image.Point {
	return c.content.Measure(context, guigui.Constraints{})
}

type gitConfirmContent struct {
	guigui.DefaultWidget

	popup *basicwidget.Popup

	message basicwidget.Text
	delete  basicwidget.Button
	cancel  basicwidget.Button

	messageText string
	deleteText  string
	cancelText  string
	onYes       func()

	rowItems    []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
}

func (c *gitConfirmContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&c.message)
	adder.AddWidget(&c.cancel)
	adder.AddWidget(&c.delete)

	c.message.SetValue(c.messageText)
	c.message.SetMultiline(true)
	c.message.SetWrapMode(basicwidget.WrapModeNormal)
	c.message.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.message.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	c.cancel.SetText(c.cancelText)
	c.cancel.OnDown(func(context *guigui.Context) {
		c.popup.SetOpen(false)
	})
	c.delete.SetText(c.deleteText)
	c.delete.SetType(basicwidget.ButtonTypePrimary)
	c.delete.OnDown(func(context *guigui.Context) {
		yes := c.onYes
		c.popup.SetOpen(false)
		if yes != nil {
			yes()
		}
	})
	return nil
}

func (c *gitConfirmContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 16 * u
	cancelW := c.cancel.Measure(context, guigui.Constraints{}).X
	deleteW := c.delete.Measure(context, guigui.Constraints{}).X
	if need := cancelW + deleteW + u/4 + u; need > w {
		w = need
	}
	msgH := c.message.Measure(context, guigui.FixedWidthConstraints(w-u)).Y
	btnH := c.delete.Measure(context, guigui.Constraints{}).Y
	h := msgH + btnH + 2*u
	if h < 5*u {
		h = 5 * u
	}
	return image.Pt(w, h)
}

func (c *gitConfirmContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	btnH := c.delete.Measure(context, guigui.Constraints{}).Y
	cancelW := c.cancel.Measure(context, guigui.Constraints{}).X
	deleteW := c.delete.Measure(context, guigui.Constraints{}).X

	c.rowItems = slices.Delete(c.rowItems, 0, len(c.rowItems))
	c.rowItems = append(c.rowItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.cancel, Size: guigui.FixedSize(cancelW)},
		guigui.LinearLayoutItem{Widget: &c.delete, Size: guigui.FixedSize(deleteW)},
	)
	row := guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.rowItems,
		Gap:       u / 4,
	}

	c.layoutItems = slices.Delete(c.layoutItems, 0, len(c.layoutItems))
	c.layoutItems = append(c.layoutItems,
		guigui.LinearLayoutItem{Widget: &c.message, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(btnH), Layout: &row},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.layoutItems,
		Gap:       u / 2,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// gitRename asks for a new local branch name.
type gitRename struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content gitRenameContent
}

func (r *gitRename) Ask(prompt, initial, okText, cancelText string, onOK func(name string)) {
	r.content.prompt = prompt
	r.content.initial = initial
	r.content.okText = okText
	r.content.cancelText = cancelText
	r.content.onOK = onOK
	r.content.showPush = false
	r.content.onOKPush = nil
	r.content.focusInput = true
	r.content.input.SetValue(initial)
	r.popup.SetOpen(true)
}

// AskTag asks for a tag name. The push option starts unchecked.
func (r *gitRename) AskTag(prompt, pushLabel, okText, cancelText string, onOK func(name string, push bool)) {
	r.content.prompt = prompt
	r.content.initial = ""
	r.content.okText = okText
	r.content.cancelText = cancelText
	r.content.onOK = nil
	r.content.showPush = true
	r.content.pushLabel = pushLabel
	r.content.onOKPush = onOK
	r.content.focusInput = true
	r.content.push.SetValue(false)
	r.content.input.SetValue("")
	r.popup.SetOpen(true)
}

func (r *gitRename) IsOpen() bool {
	return r.popup.IsOpen()
}

func (r *gitRename) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	r.content.popup = &r.popup
	r.popup.SetContent(&r.content)
	r.popup.SetModal(true)
	r.popup.SetBackgroundDark(true)
	r.popup.SetCloseByClickingOutside(true)
	r.popup.SetAnimated(true)
	if r.popup.IsOpen() {
		adder.AddWidget(&r.popup)
	}
	return nil
}

func (r *gitRename) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.popup, widgetBounds.Bounds())
}

func (r *gitRename) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return image.Point{}
}

func (r *gitRename) contentSize(context *guigui.Context) image.Point {
	return r.content.Measure(context, guigui.Constraints{})
}

type gitRenameContent struct {
	guigui.DefaultWidget

	popup *basicwidget.Popup

	message  basicwidget.Text
	input    basicwidget.TextInput
	push     basicwidget.Checkbox
	pushText basicwidget.Text
	ok       basicwidget.Button
	cancel   basicwidget.Button

	prompt      string
	initial     string
	okText      string
	cancelText  string
	onOK        func(name string)
	showPush    bool
	pushLabel   string
	pushHit     image.Rectangle
	pushPressed bool
	onOKPush    func(name string, push bool)
	// focusInput is set when the dialog opens and consumed by Tick, once the
	// text field is in the widget tree and can take focus.
	focusInput bool

	rowItems    []guigui.LinearLayoutItem
	pushItems   []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
}

func (c *gitRenameContent) close() {
	c.focusInput = false
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
}

func (c *gitRenameContent) submit() {
	name := strings.TrimSpace(c.input.Value())
	if name == "" || name == c.initial {
		return
	}
	c.focusInput = false
	if c.showPush {
		push := c.push.Value()
		ok := c.onOKPush
		c.onOK = nil
		c.onOKPush = nil
		if c.popup != nil {
			c.popup.SetOpen(false)
		}
		if ok != nil {
			ok(name, push)
		}
		return
	}
	ok := c.onOK
	c.onOK = nil
	c.onOKPush = nil
	if c.popup != nil {
		c.popup.SetOpen(false)
	}
	if ok != nil {
		ok(name)
	}
}

func (c *gitRenameContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&c.message)
	adder.AddWidget(&c.input)
	if c.showPush {
		adder.AddWidget(&c.push)
		adder.AddWidget(&c.pushText)
	}
	adder.AddWidget(&c.cancel)
	adder.AddWidget(&c.ok)

	c.message.SetValue(c.prompt)
	c.message.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	c.input.SetPlaceholder(c.prompt)
	if c.showPush {
		c.pushText.SetValue(c.pushLabel)
		c.pushText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		c.pushText.SetSelectable(false)
	}
	// Enter in the field confirms. A focus loss, including a click outside, does not.
	c.input.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			c.submit()
		}
		guigui.RequestRebuild()
	})

	// Keys reach this dialog before the text field is focused, and while focus
	// sits on another control inside it.
	context.SetButtonInputReceptive(c, true)

	c.cancel.SetText(c.cancelText)
	c.cancel.OnDown(func(context *guigui.Context) {
		c.close()
	})
	c.ok.SetText(c.okText)
	c.ok.SetType(basicwidget.ButtonTypePrimary)
	name := strings.TrimSpace(c.input.Value())
	context.SetEnabled(&c.ok, name != "" && name != c.initial)
	c.ok.OnDown(func(context *guigui.Context) {
		c.submit()
	})
	return nil
}

func (c *gitRenameContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 16 * u
	return image.Pt(w, c.stackHeight(context, w))
}

// stackHeight is the height Layout actually uses. The popup clips to the
// measured size, so a short value hides the bottom of the buttons.
func (c *gitRenameContent) stackHeight(context *guigui.Context, width int) int {
	u := basicwidget.UnitSize(context)
	padTop, padBottom, padSide := c.dialogPadding(u)
	inner := width - 2*padSide
	if inner < u {
		inner = u
	}
	inH := c.input.Measure(context, guigui.FixedWidthConstraints(inner)).Y
	btnH := c.buttonRowHeight(context)
	gap := u / 2
	// message, input, flexible spacer, buttons. The spacer is its own item.
	n := 4
	h := u + inH + btnH + u/2
	if c.showPush {
		n++
		h += u
	}
	h += gap*(n-1) + padTop + padBottom
	if !c.showPush && h < 6*u {
		h = 6 * u
	}
	return h
}

func (c *gitRenameContent) dialogPadding(u int) (top, bottom, side int) {
	side = u / 2
	top = u / 2
	bottom = u / 2
	if c.showPush {
		// Clear the popup's rounded corner so the buttons stay fully visible.
		bottom = u
	}
	return top, bottom, side
}

func (c *gitRenameContent) buttonRowHeight(context *guigui.Context) int {
	u := basicwidget.UnitSize(context)
	h := c.ok.Measure(context, guigui.Constraints{}).Y
	if cancelH := c.cancel.Measure(context, guigui.Constraints{}).Y; cancelH > h {
		h = cancelH
	}
	if h < u {
		h = u
	}
	return h
}

func (c *gitRenameContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	padTop, padBottom, padSide := c.dialogPadding(u)
	inner := bounds.Dx() - 2*padSide
	if inner < u {
		inner = u
	}
	inH := c.input.Measure(context, guigui.FixedWidthConstraints(inner)).Y
	btnH := c.buttonRowHeight(context)
	cancelW := c.cancel.Measure(context, guigui.Constraints{}).X
	okW := c.ok.Measure(context, guigui.Constraints{}).X
	c.pushHit = image.Rectangle{}

	c.rowItems = slices.Delete(c.rowItems, 0, len(c.rowItems))
	c.rowItems = append(c.rowItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.cancel, Size: guigui.FixedSize(cancelW)},
		guigui.LinearLayoutItem{Widget: &c.ok, Size: guigui.FixedSize(okW)},
	)
	row := guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.rowItems,
		Gap:       u / 4,
	}

	c.layoutItems = slices.Delete(c.layoutItems, 0, len(c.layoutItems))
	c.layoutItems = append(c.layoutItems,
		guigui.LinearLayoutItem{Widget: &c.message, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.input, Size: guigui.FixedSize(inH)},
	)
	if c.showPush {
		c.pushItems = slices.Delete(c.pushItems, 0, len(c.pushItems))
		c.pushItems = append(c.pushItems,
			guigui.LinearLayoutItem{Widget: &c.push, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.pushText, Size: guigui.FlexibleSize(1)},
		)
		pushRow := guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     c.pushItems,
			Gap:       u / 4,
		}
		c.layoutItems = append(c.layoutItems, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &pushRow})
	}
	c.layoutItems = append(c.layoutItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(btnH), Layout: &row},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.layoutItems,
		Gap:       u / 2,
		Padding:   guigui.Padding{Start: padSide, Top: padTop, End: padSide, Bottom: padBottom},
	}).LayoutWidgets(context, bounds, layouter)
	if c.showPush {
		gap := u / 2
		rowY := bounds.Min.Y + padTop + u + gap + inH + gap
		labelX := bounds.Min.X + padSide + u
		c.pushHit = image.Rect(labelX, rowY, bounds.Max.X-padSide, rowY+u)
	}
}

func (c *gitRenameContent) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if !c.focusInput {
		return nil
	}
	c.focusInput = false
	if c.popup == nil || !c.popup.IsOpen() {
		return nil
	}
	c.input.SelectAll()
	context.SetFocused(&c.input, true)
	return nil
}

func (c *gitRenameContent) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if c.popup == nil || !c.popup.IsOpen() {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		c.close()
		return guigui.HandleInputByWidget(c)
	}
	// Enter in the text field commits there and submits via OnValueChanged.
	// This covers Enter before that field is focused, and the numpad key.
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		c.submit()
		return guigui.HandleInputByWidget(c)
	}
	return guigui.HandleInputResult{}
}

func (c *gitRenameContent) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	cursor := image.Pt(ebiten.CursorPosition())
	outside := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if outside && c.popup != nil && c.popup.IsOpen() && !cursor.In(widgetBounds.Bounds()) {
		c.close()
		return guigui.HandleInputByWidget(c)
	}
	if !c.showPush || c.pushHit.Empty() {
		return guigui.HandleInputResult{}
	}
	hit := cursor.In(c.pushHit)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && hit {
		c.pushPressed = true
		return guigui.HandleInputByWidget(c)
	}
	if c.pushPressed && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		c.pushPressed = false
		if hit {
			c.push.SetValue(!c.push.Value())
			guigui.RequestRedraw(&c.push)
		}
		return guigui.HandleInputByWidget(c)
	}
	return guigui.HandleInputResult{}
}

func (c *gitRenameContent) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if c.showPush && image.Pt(ebiten.CursorPosition()).In(c.pushHit) {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}
