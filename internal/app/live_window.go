package app

import "github.com/gogpu/gogpu"

// liveWindowProvider reports window size from the live physical framebuffer.
//
// On macOS, AppKit's live-resize modal loop defers EventResize and leaves
// App.Size() (cached LogicalSize) stale until the drag ends. PhysicalSize is
// updated every tick, and gogpu.Context.Width/Height are derived from it.
// Using that same source for the UI WindowProvider keeps layout in sync so
// FrameworkManaged can full-repaint the newly allocated pixmap instead of
// leaving black margins.
type liveWindowProvider struct {
	app *gogpu.App
}

func newLiveWindowProvider(app *gogpu.App) *liveWindowProvider {
	return &liveWindowProvider{app: app}
}

func (p *liveWindowProvider) Size() (width, height int) {
	pw, ph := p.app.PhysicalSize()
	if pw <= 0 || ph <= 0 {
		return p.app.Size()
	}
	return logicalSizeFromPhysical(pw, ph, p.app.ScaleFactor())
}

func (p *liveWindowProvider) ScaleFactor() float64 {
	return p.app.ScaleFactor()
}

func (p *liveWindowProvider) RequestRedraw() {
	p.app.RequestRedraw()
}

// logicalSizeFromPhysical matches gogpu.Context.Size: physical / scale, truncating toward zero.
func logicalSizeFromPhysical(physW, physH int, scale float64) (int, int) {
	if scale <= 0 {
		scale = 1
	}
	return int(float64(physW) / scale), int(float64(physH) / scale)
}
