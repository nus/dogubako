package app

import (
	"log"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/dnd"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// runDesktop draws the widget tree into a CPU pixmap and presents it.
// gogpu/ui's GPU compositor leaves software adapters as a black window.
func runDesktop(gpuApp *gogpu.App, uiApp *uiapp.App) error {
	uiApp.Window().SetRenderMode(uiapp.RenderModeFrameworkManaged)
	widget.RegisterClipboardProvider(gpuApp)

	var canvas *ggcanvas.Canvas
	var lastW, lastH int
	gpuApp.OnDraw(func(dc *gogpu.Context) {
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}
		// EventResize is deferred during macOS live resize. HandleResize marks
		// needsFullRepaint so the blank pixmap from canvas.Resize is fully
		// redrawn instead of leaving black strips in the expanded area.
		if w != lastW || h != lastH {
			uiApp.Window().HandleResize(w, h)
			lastW, lastH = w, h
		}
		if canvas == nil {
			var err error
			canvas, err = ggcanvas.New(gpuApp.GPUContextProvider(), w, h)
			if err != nil {
				log.Printf("ggcanvas: %v", err)
				return
			}
		} else if err := canvas.Resize(w, h); err != nil {
			log.Printf("ggcanvas: %v", err)
			return
		}

		uiApp.Frame()
		if !uiApp.Window().DrawTo(render.NewCanvas(canvas.Context(), w, h)) {
			return
		}
		canvas.MarkDirty()
		if err := canvas.Render(dc.RenderTarget()); err != nil {
			log.Printf("present: %v", err)
		}
	})

	gpuApp.OnDragDrop(func(paths []string, x, y float64) {
		uiApp.Window().DndManager().DropExternal(dnd.DragData{
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
