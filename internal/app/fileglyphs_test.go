package app

import (
	"testing"

	"github.com/gogpu/ui/geometry"
)

func TestAndroidNameSlotsLayout(t *testing.T) {
	bounds := geometry.NewRect(10, 20, 400, 28)
	d0 := androidNameSlotsOf(bounds, 0)
	if d0.Expand.Min.X != bounds.Min.X+androidNamePadX {
		t.Fatalf("depth0 expand x = %v", d0.Expand.Min.X)
	}
	if d0.Icon.Min.X <= d0.Expand.Max.X {
		t.Fatalf("icon overlaps expand: %#v %#v", d0.Expand, d0.Icon)
	}
	if d0.Text.Min.X <= d0.Icon.Max.X {
		t.Fatalf("text overlaps icon: %#v %#v", d0.Icon, d0.Text)
	}
	if d0.Text.Max.X > bounds.Max.X {
		t.Fatalf("text overflows cell")
	}

	d1 := androidNameSlotsOf(bounds, 1)
	shift := d1.Expand.Min.X - d0.Expand.Min.X
	if shift != androidNameIndent {
		t.Fatalf("depth indent = %v", shift)
	}
}

func TestAndroidNameSlotsDepthFloor(t *testing.T) {
	bounds := geometry.NewRect(0, 0, 200, 28)
	neg := androidNameSlotsOf(bounds, -3)
	zero := androidNameSlotsOf(bounds, 0)
	if neg.Expand.Min.X != zero.Expand.Min.X {
		t.Fatalf("negative depth should clamp")
	}
}
