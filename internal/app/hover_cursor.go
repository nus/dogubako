package app

import (
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

// cursorOutside is a window position no widget contains. Hover and cursor
// shapes are decided from ebiten.CursorPosition, so placing the sampled
// point here makes those checks miss while a drag can still read the real
// position.
const cursorOutside = -1_000_000

// ebitenKeyCount and ebitenMouseButtonCount match ui.InputState in
// github.com/hajimehoshi/ebiten/v2 v2.10.4 (KeyMax+1 and MouseButtonMax+1).
// TestCursorPositionRoundTrip fails when that layout changes.
const (
	ebitenKeyCount         = 122
	ebitenMouseButtonCount = 5
)

// ebitenInputState is the prefix of ui.InputState through the cursor.
// InputTime is int64.
type ebitenInputState struct {
	keyPressed       [ebitenKeyCount]int64
	keyReleased      [ebitenKeyCount]int64
	mousePressed     [ebitenMouseButtonCount]int64
	mouseReleased    [ebitenMouseButtonCount]int64
	cursorX, cursorY float64
}

//go:linkname inputStateGet github.com/hajimehoshi/ebiten/v2/internal/inputstate.Get
func inputStateGet() unsafe.Pointer

//go:linkname inputStateUpdate github.com/hajimehoshi/ebiten/v2/internal/inputstate.(*inputState).Update
func inputStateUpdate(state unsafe.Pointer, fn func(*ebitenInputState))

func setCursorPosition(x, y float64) {
	inputStateUpdate(inputStateGet(), func(state *ebitenInputState) {
		state.cursorX = x
		state.cursorY = y
	})
}

func mouseButtonDown() bool {
	return ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
}

// hoverCursor is the position hover and cursor-shape checks should see.
// A pressed button keeps the real position so a drag that has left the
// window still tracks the pointer. Otherwise a pointer over another window
// is reported outside this window.
func hoverCursor(overWindow, buttonDown bool, x, y float64) (float64, float64) {
	if buttonDown || overWindow {
		return x, y
	}
	return cursorOutside, cursorOutside
}

// placeCursorForHover moves the sampled cursor off this window when the
// pointer is over a different window. macOS updates the window-relative
// position from mouseLocationOutsideOfEventStream even then, and hover
// follows that position.
func placeCursorForHover() {
	x, y := ebiten.CursorPosition()
	nx, ny := hoverCursor(pointerOverWindow(), mouseButtonDown(), float64(x), float64(y))
	if nx == float64(x) && ny == float64(y) {
		return
	}
	setCursorPosition(nx, ny)
}

// windowHoverGame runs before each guigui update so hit testing, hover
// drawing, and cursor shapes all see the adjusted position.
type windowHoverGame struct {
	ebiten.Game
}

func (g *windowHoverGame) Update() error {
	placeCursorForHover()
	return g.Game.Update()
}

func (g *windowHoverGame) Draw(screen *ebiten.Image) {
	placeCursorForHover()
	g.Game.Draw(screen)
}

func (g *windowHoverGame) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	if f, ok := g.Game.(ebiten.LayoutFer); ok {
		return f.LayoutF(outsideWidth, outsideHeight)
	}
	w, h := g.Layout(int(outsideWidth), int(outsideHeight))
	return float64(w), float64(h)
}

// RunGame runs game, limiting hover to the window actually under the pointer.
func RunGame(game ebiten.Game, options *ebiten.RunGameOptions) error {
	return ebiten.RunGameWithOptions(&windowHoverGame{Game: game}, options)
}
