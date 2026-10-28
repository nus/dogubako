package app

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestHoverCursor(t *testing.T) {
	tests := []struct {
		name       string
		overWindow bool
		buttonDown bool
		x, y       float64
		wantX      float64
		wantY      float64
	}{
		{name: "over window", overWindow: true, x: 10, y: 20, wantX: 10, wantY: 20},
		{name: "covered", x: 10, y: 20, wantX: cursorOutside, wantY: cursorOutside},
		{name: "drag over another window", buttonDown: true, x: 10, y: 20, wantX: 10, wantY: 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := hoverCursor(tt.overWindow, tt.buttonDown, tt.x, tt.y)
			if gotX != tt.wantX || gotY != tt.wantY {
				t.Fatalf("hoverCursor() = %v,%v, want %v,%v", gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestCursorPositionRoundTrip(t *testing.T) {
	setCursorPosition(0, 0)
	t.Cleanup(func() { setCursorPosition(0, 0) })

	setCursorPosition(123, 456)
	x, y := ebiten.CursorPosition()
	if x != 123 || y != 456 {
		t.Fatalf("CursorPosition() = %d,%d, want 123,456; ebiten InputState layout may have changed", x, y)
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.Fatal("writing the cursor position changed the left button state")
	}
}
