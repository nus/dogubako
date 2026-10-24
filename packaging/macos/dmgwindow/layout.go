// Package dmgwindow describes the Finder window of the macOS UDZO disk image.
//
// The background is drawn here, at package time, and is not stored in the
// repository. Icon positions are the centers Finder stores in the disk
// image, in points from the top-left of the window content.
package dmgwindow

import (
	"fmt"
	"image/color"
)

// Point is an icon center in the Finder window, in points.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Layout is the single description of the window, the icon positions, and
// the arrow drawn between them.
type Layout struct {
	Width        int   `json:"width"`
	Height       int   `json:"height"`
	IconSize     int   `json:"icon_size"`
	TextSize     int   `json:"text_size"`
	App          Point `json:"app"`
	Applications Point `json:"applications"`

	// Gap is the space between an icon's edge and the arrow, in points.
	Gap float64
	// Shaft is the arrow's line thickness, in points.
	Shaft float64
	// Head is the length of the arrow head, in points.
	Head float64
	// HeadHalf is half the arrow head's height, in points.
	HeadHalf float64
}

// Default is the window used for Dogubako.app and the Applications alias.
func Default() Layout {
	return Layout{
		Width:        720,
		Height:       380,
		IconSize:     128,
		TextSize:     13,
		App:          Point{X: 180, Y: 168},
		Applications: Point{X: 540, Y: 168},
		Gap:          32,
		Shaft:        10,
		Head:         32,
		HeadHalf:     20,
	}
}

// Arrow is the horizontal arrow from the app icon toward Applications.
// X0 is the tail, X1 is where the head begins, and Tip is the point.
// Coordinates are in points, origin at the top-left.
type Arrow struct {
	X0, X1, Tip, Y float64
	Shaft          float64
	HeadHalf       float64
}

// Arrow returns the arrow placed between the two icons.
func (l Layout) Arrow() (Arrow, error) {
	if err := l.Validate(); err != nil {
		return Arrow{}, err
	}
	half := float64(l.IconSize) / 2
	tip := float64(l.Applications.X) - half - l.Gap
	return Arrow{
		X0:       float64(l.App.X) + half + l.Gap,
		X1:       tip - l.Head,
		Tip:      tip,
		Y:        float64(l.App.Y),
		Shaft:    l.Shaft,
		HeadHalf: l.HeadHalf,
	}, nil
}

// Validate reports a window that would scroll or whose arrow would miss
// the gap between the icons.
func (l Layout) Validate() error {
	if l.Width < 1 || l.Height < 1 {
		return fmt.Errorf("window size must be positive")
	}
	if l.IconSize < 16 || l.TextSize < 1 {
		return fmt.Errorf("icon size and text size must be positive")
	}
	if l.Gap < 1 || l.Shaft < 1 || l.Head < 1 || l.HeadHalf < 1 {
		return fmt.Errorf("arrow size must be positive")
	}
	if l.App.Y != l.Applications.Y {
		return fmt.Errorf("icons must share a vertical center so the arrow is horizontal")
	}
	if l.App.X >= l.Applications.X {
		return fmt.Errorf("Dogubako.app must sit to the left of Applications")
	}
	const margin = 8
	label := l.TextSize + 8
	for _, p := range []Point{l.App, l.Applications} {
		if p.X-l.IconSize/2 < margin || p.X+l.IconSize/2 > l.Width-margin {
			return fmt.Errorf("icon at %d,%d does not fit horizontally", p.X, p.Y)
		}
		if p.Y-l.IconSize/2 < margin || p.Y+l.IconSize/2+label > l.Height-margin {
			return fmt.Errorf("icon at %d,%d does not fit vertically", p.X, p.Y)
		}
	}
	half := float64(l.IconSize) / 2
	x0 := float64(l.App.X) + half + l.Gap
	tip := float64(l.Applications.X) - half - l.Gap
	if tip-l.Head <= x0 {
		return fmt.Errorf("arrow does not fit between the icons")
	}
	return nil
}

// Background and ArrowColor are the only colors in the window image.
var (
	Background = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	ArrowColor = color.RGBA{R: 72, G: 72, B: 78, A: 255}
)
