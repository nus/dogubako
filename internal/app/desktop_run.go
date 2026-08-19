package app

import (
	"log"
	"os"
	"strings"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/dnd"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// preferPixmapPresent reports whether the GPU compositor would leave the
// window black. gogpu/ui's desktop.Run draws into per-boundary GPU textures
// then presents the unused CPU pixmap on software adapters (Empty/CPU).
func preferPixmapPresent() bool {
	if strings.EqualFold(os.Getenv("GOGPU_RENDER_MODE"), "cpu") {
		return true
	}
	switch strings.ToLower(os.Getenv("GOGPU_GRAPHICS_API")) {
	case "software", "sw", "cpu":
		return true
	default:
		return false
	}
}

// runDesktop starts the window loop. Software adapters present via a CPU
// pixmap; real GPUs keep gogpu/ui's per-boundary compositor.
func runDesktop(gpuApp *gogpu.App, uiApp *uiapp.App) error {
	if preferPixmapPresent() {
		return runPixmapDesktop(gpuApp, uiApp)
	}
	return desktop.Run(gpuApp, uiApp)
}

func runPixmapDesktop(gpuApp *gogpu.App, uiApp *uiapp.App) error {
	// canvas.Render takes GPU-direct when AcceleratorCanRenderDirect is true
	// and would present an empty surface. Force the pixmap present path.
	if os.Getenv("GOGPU_RENDER_MODE") == "" {
		_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	}

	uiApp.Window().SetRenderMode(uiapp.RenderModeHostManaged)
	widget.RegisterClipboardProvider(gpuApp)

	var canvas *ggcanvas.Canvas
	fullRedraw := true

	gpuApp.OnDraw(func(dc *gogpu.Context) {
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}
		if canvas == nil {
			provider := gpuApp.GPUContextProvider()
			if provider == nil {
				return
			}
			var err error
			canvas, err = ggcanvas.New(provider, w, h)
			if err != nil {
				log.Printf("dogubako: ggcanvas.New: %v", err)
				return
			}
		}

		uiApp.Frame()

		cw, ch := canvas.Size()
		if cw != w || ch != h {
			if err := canvas.Resize(w, h); err != nil {
				log.Printf("dogubako: canvas.Resize: %v", err)
				return
			}
			fullRedraw = true
		}

		win := uiApp.Window()
		if !fullRedraw && !win.NeedsRedraw() && !win.HasDirtyBoundaries() && !win.NeedsAnimationFrame() {
			return
		}
		win.ClearAnimationFrame()

		cc := canvas.Context()
		bg := win.ThemeBackground()
		cc.ClearWithColor(gg.RGBA{
			R: float64(bg.R),
			G: float64(bg.G),
			B: float64(bg.B),
			A: float64(bg.A),
		})

		widgetCanvas := render.NewCanvas(cc, w, h)
		win.DrawTo(widgetCanvas)

		canvas.MarkDirty()
		if err := canvas.Render(dc.RenderTarget()); err != nil {
			log.Printf("dogubako: canvas.Render: %v", err)
		}
		fullRedraw = false
	})

	gpuApp.OnDragDrop(func(paths []string, x, y float64) {
		mgr := uiApp.Window().DndManager()
		mgr.DropExternal(dnd.DragData{
			Kind:    dnd.KindFile,
			Payload: dnd.FilePayload{Paths: paths},
		}, x, y)
	})

	gpuApp.OnClose(func() {
		uiApp.Window().Close()
		gg.CloseAccelerator()
		if canvas != nil {
			_ = canvas.Close()
		}
	})

	return gpuApp.Run()
}
