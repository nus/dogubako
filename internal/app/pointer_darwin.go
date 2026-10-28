//go:build darwin

package app

import (
	"structs"

	"github.com/ebitengine/purego/objc"
	"github.com/hajimehoshi/ebiten/v2"
)

type nsPoint struct {
	_    structs.HostLayout
	X, Y float64
}

type nsSize struct {
	_             structs.HostLayout
	Width, Height float64
}

type nsRect struct {
	_      structs.HostLayout
	Origin nsPoint
	Size   nsSize
}

var (
	selMouseLocation       = objc.RegisterName("mouseLocation")
	selWindowNumberAtPoint = objc.RegisterName("windowNumberAtPoint:belowWindowWithWindowNumber:")
	selSharedApplication   = objc.RegisterName("sharedApplication")
	selWindows             = objc.RegisterName("windows")
	selCount               = objc.RegisterName("count")
	selObjectAtIndex       = objc.RegisterName("objectAtIndex:")
	selWindowNumber        = objc.RegisterName("windowNumber")
	selContentView         = objc.RegisterName("contentView")
	selFrame               = objc.RegisterName("frame")
	selConvertRectToScreen = objc.RegisterName("convertRectToScreen:")
	selNew                 = objc.RegisterName("new")
	selRelease             = objc.RegisterName("release")
)

// mouseInContentRect reports whether p lies in r using the same edges as
// NSMouseInRect for a non-flipped view: the minimum x and maximum y are
// inside, and the maximum x and minimum y are outside.
func mouseInContentRect(p nsPoint, r nsRect) bool {
	maxX := r.Origin.X + r.Size.Width
	maxY := r.Origin.Y + r.Size.Height
	return p.X >= r.Origin.X && p.X < maxX && p.Y > r.Origin.Y && p.Y <= maxY
}

// pointerOverWindow reports whether this app's content view is the
// frontmost window at the pointer, including when that window is not key.
func pointerOverWindow() bool {
	over := true
	ebiten.RunOnMainThread(func() {
		over = pointerOverAppWindow()
	})
	return over
}

func pointerOverAppWindow() bool {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selNew)
	defer pool.Send(selRelease)

	pos := objc.Send[nsPoint](objc.ID(objc.GetClass("NSEvent")), selMouseLocation)
	top := objc.Send[uintptr](objc.ID(objc.GetClass("NSWindow")), selWindowNumberAtPoint, pos, uintptr(0))
	if top == 0 {
		return false
	}

	app := objc.ID(objc.GetClass("NSApplication")).Send(selSharedApplication)
	windows := app.Send(selWindows)
	n := int(windows.Send(selCount))
	for i := range n {
		window := windows.Send(selObjectAtIndex, i)
		if window == 0 || uintptr(window.Send(selWindowNumber)) != top {
			continue
		}
		view := window.Send(selContentView)
		if view == 0 {
			return false
		}
		frame := objc.Send[nsRect](view, selFrame)
		screen := objc.Send[nsRect](window, selConvertRectToScreen, frame)
		return mouseInContentRect(pos, screen)
	}
	return false
}
