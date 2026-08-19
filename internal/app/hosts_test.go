package app

import (
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type stubPanel struct {
	widget.WidgetBase
}

func newStubPanel() *stubPanel {
	p := &stubPanel{}
	p.SetVisible(true)
	p.SetEnabled(true)
	return p
}

func (p *stubPanel) Layout(_ widget.Context, cons geometry.Constraints) geometry.Size {
	size := cons.Constrain(geometry.Sz(100, 100))
	p.SetBounds(geometry.FromPointSize(p.Position(), size))
	return size
}

func (p *stubPanel) Draw(widget.Context, widget.Canvas) {}

func (p *stubPanel) Event(widget.Context, event.Event) bool { return false }

func TestModeHostChildrenOnlyActive(t *testing.T) {
	s := &Shell{}
	h := &modeHost{
		shell:      s,
		image:      newStubPanel(),
		screenshot: newStubPanel(),
		android:    newStubPanel(),
	}
	h.SetVisible(true)

	if got := h.Children(); len(got) != 1 || got[0] != h.image {
		t.Fatalf("default children = %v", got)
	}

	s.model.SetMode(ToolScreenshot)
	if got := h.Children(); len(got) != 1 || got[0] != h.screenshot {
		t.Fatalf("screenshot children = %v", got)
	}

	s.model.SetMode(ToolAndroid)
	if got := h.Children(); len(got) != 1 || got[0] != h.android {
		t.Fatalf("android children = %v", got)
	}
}

func TestModeHostHidesInactive(t *testing.T) {
	s := &Shell{}
	h := &modeHost{
		shell:      s,
		image:      newStubPanel(),
		screenshot: newStubPanel(),
		android:    newStubPanel(),
	}
	h.Layout(nil, geometry.Tight(geometry.Sz(400, 300)))
	if !h.image.(*stubPanel).IsVisible() {
		t.Fatal("image should be visible")
	}
	if h.screenshot.(*stubPanel).IsVisible() || h.android.(*stubPanel).IsVisible() {
		t.Fatal("inactive tools should be hidden")
	}

	s.model.SetMode(ToolAndroid)
	h.Layout(nil, geometry.Tight(geometry.Sz(400, 300)))
	if !h.android.(*stubPanel).IsVisible() {
		t.Fatal("android should be visible")
	}
	if h.image.(*stubPanel).IsVisible() || h.screenshot.(*stubPanel).IsVisible() {
		t.Fatal("previous tools should be hidden")
	}
}
