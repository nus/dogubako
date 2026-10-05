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
	renameLabel string
	deleteLabel string
	onRename    func(gitcli.Ref)
	onDelete    func(gitcli.Ref)
}

func (m *gitBranchMenu) Open(ref gitcli.Ref, at image.Point, renameLabel, deleteLabel string, onRename, onDelete func(gitcli.Ref)) {
	m.ref = ref
	m.pos = at
	m.renameLabel = renameLabel
	m.deleteLabel = deleteLabel
	m.onRename = onRename
	m.onDelete = onDelete
	m.menu.SetOpen(true)
}

func (m *gitBranchMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitBranchMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems([]basicwidget.PopupMenuItem[string]{
		{
			Text:     m.renameLabel,
			Value:    "rename",
			Disabled: m.ref.Remote != "",
		},
		{
			Text:     m.deleteLabel,
			Value:    "delete",
			Disabled: m.ref.Current,
		},
	})
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok {
			return
		}
		switch item.Value {
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

	pos      image.Point
	hash     string
	label    string
	onCreate func(hash string)
}

func (m *gitCommitMenu) Open(hash string, at image.Point, label string, onCreate func(hash string)) {
	m.hash = hash
	m.pos = at
	m.label = label
	m.onCreate = onCreate
	m.menu.SetOpen(true)
}

func (m *gitCommitMenu) IsOpen() bool {
	return m.menu.IsOpen()
}

func (m *gitCommitMenu) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m.menu.SetItems([]basicwidget.PopupMenuItem[string]{
		{Text: m.label, Value: "tag"},
	})
	m.menu.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := m.menu.ItemByIndex(index)
		if !ok || item.Value != "tag" || m.onCreate == nil || m.hash == "" {
			return
		}
		m.onCreate(m.hash)
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

	rowItems    []guigui.LinearLayoutItem
	pushItems   []guigui.LinearLayoutItem
	layoutItems []guigui.LinearLayoutItem
}

func (c *gitRenameContent) submit() {
	name := strings.TrimSpace(c.input.Value())
	if name == "" || name == c.initial {
		return
	}
	if c.showPush {
		push := c.push.Value()
		ok := c.onOKPush
		c.popup.SetOpen(false)
		if ok != nil {
			ok(name, push)
		}
		return
	}
	ok := c.onOK
	c.popup.SetOpen(false)
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
	// Enter confirms. A focus loss, including a click outside the dialog, does not.
	c.input.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			c.submit()
		}
		guigui.RequestRebuild()
	})

	c.cancel.SetText(c.cancelText)
	c.cancel.OnDown(func(context *guigui.Context) {
		c.popup.SetOpen(false)
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

func (c *gitRenameContent) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	cursor := image.Pt(ebiten.CursorPosition())
	outside := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if outside && c.popup != nil && c.popup.IsOpen() && !cursor.In(widgetBounds.Bounds()) {
		c.popup.SetOpen(false)
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
