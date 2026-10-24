package dmgwindow

import (
	"encoding/json"
	"image/color"
	"testing"
)

func TestDefaultFitsWithoutScroll(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestArrowSitsBetweenIcons(t *testing.T) {
	layout := Default()
	arrow, err := layout.Arrow()
	if err != nil {
		t.Fatal(err)
	}
	if arrow.Y != float64(layout.App.Y) {
		t.Fatalf("arrow y = %v, icon y = %d", arrow.Y, layout.App.Y)
	}
	appRight := float64(layout.App.X + layout.IconSize/2)
	appsLeft := float64(layout.Applications.X - layout.IconSize/2)
	if arrow.X0 <= appRight || arrow.Tip >= appsLeft {
		t.Fatalf("arrow [%v, %v] overlaps icons [%v, %v]", arrow.X0, arrow.Tip, appRight, appsLeft)
	}
	if arrow.Tip <= arrow.X1 || arrow.X1 <= arrow.X0 {
		t.Fatalf("arrow does not point right: tail %v base %v tip %v", arrow.X0, arrow.X1, arrow.Tip)
	}
}

func TestRenderArrowPointsRight(t *testing.T) {
	layout := Default()
	img, err := Render(layout, 1)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != layout.Width || img.Bounds().Dy() != layout.Height {
		t.Fatalf("size %v", img.Bounds())
	}
	arrow, err := layout.Arrow()
	if err != nil {
		t.Fatal(err)
	}

	shaftX := int((arrow.X0 + arrow.X1) / 2)
	if !isArrow(img.RGBAAt(shaftX, int(arrow.Y))) {
		t.Fatal("shaft center is not the arrow color")
	}
	// Above the shaft, still beside the head, the background stays empty.
	if !isBackground(img.RGBAAt(shaftX, int(arrow.Y)-20)) {
		t.Fatal("background above the shaft is painted")
	}
	// Inside the head, off the shaft.
	headX := int(arrow.X1) + 6
	if !isArrow(img.RGBAAt(headX, int(arrow.Y)-10)) {
		t.Fatal("arrow head is missing")
	}
	// Past the tip there is no paint, so the arrow ends pointing right.
	if !isBackground(img.RGBAAt(int(arrow.Tip)+4, int(arrow.Y))) {
		t.Fatal("paint continues past the arrow tip")
	}
	// The icon centers stay clear so the arrow does not show through them.
	if !isBackground(img.RGBAAt(layout.App.X, layout.App.Y)) {
		t.Fatal("arrow paints the app icon center")
	}
	if !isBackground(img.RGBAAt(layout.Applications.X, layout.Applications.Y)) {
		t.Fatal("arrow paints the Applications icon center")
	}
}

func TestRenderRetinaScale(t *testing.T) {
	layout := Default()
	img, err := Render(layout, 2)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != layout.Width*2 || img.Bounds().Dy() != layout.Height*2 {
		t.Fatalf("size %v", img.Bounds())
	}
	arrow, err := layout.Arrow()
	if err != nil {
		t.Fatal(err)
	}
	if !isArrow(img.RGBAAt(int(arrow.X0+arrow.X1), int(arrow.Y*2))) {
		t.Fatal("retina shaft center is not the arrow color")
	}
}

func TestLayoutJSON(t *testing.T) {
	layout := Default()
	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"width", "height", "icon_size", "text_size", "app", "applications"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing %s in %s", key, data)
		}
	}
	if _, ok := got["gap"]; ok {
		t.Fatal("arrow metrics leaked into layout json")
	}
}

func TestRejectsOverlappingArrow(t *testing.T) {
	layout := Default()
	layout.Applications.X = layout.App.X + layout.IconSize
	if err := layout.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func isArrow(c color.RGBA) bool {
	return c.R < 180 && c.G < 180 && c.B < 190 && c.A == 255
}

func isBackground(c color.RGBA) bool {
	return c == Background
}
