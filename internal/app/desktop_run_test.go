package app

import (
	"testing"
)

func TestPreferPixmapPresent(t *testing.T) {
	t.Setenv("GOGPU_GRAPHICS_API", "")
	t.Setenv("GOGPU_RENDER_MODE", "")
	if preferPixmapPresent() {
		t.Fatal("empty env should use GPU compositor")
	}

	t.Setenv("GOGPU_GRAPHICS_API", "software")
	if !preferPixmapPresent() {
		t.Fatal("software API should present the CPU pixmap")
	}

	t.Setenv("GOGPU_GRAPHICS_API", "")
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	if !preferPixmapPresent() {
		t.Fatal("cpu render mode should present the CPU pixmap")
	}

	t.Setenv("GOGPU_RENDER_MODE", "")
	t.Setenv("GOGPU_GRAPHICS_API", "vulkan")
	if preferPixmapPresent() {
		t.Fatal("vulkan should keep the GPU compositor")
	}
}
