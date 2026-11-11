package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"

	"github.com/egoist/mygo/ui"
)

func svg(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconLogo     = svg(`<path d="M2.4 13.4V2.8l5.6 6.4 5.6-6.4v10.6"/><circle cx="8" cy="9.2" r="1.25" fill="currentColor" stroke="none"/>`)
	iconGrid     = svg(`<rect x="2" y="2" width="5" height="5" rx="1.2"/><rect x="9" y="2" width="5" height="5" rx="1.2"/><rect x="2" y="9" width="5" height="5" rx="1.2"/><rect x="9" y="9" width="5" height="5" rx="1.2"/>`)
	iconSearch   = svg(`<circle cx="7.2" cy="7.2" r="4.6"/><path d="M10.6 10.6L14 14"/>`)
	iconSettings = svg(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.6v1.7M8 12.7v1.7M1.6 8h1.7M12.7 8h1.7M3.5 3.5l1.2 1.2M11.3 11.3l1.2 1.2M12.5 3.5l-1.2 1.2M4.7 11.3l-1.2 1.2"/>`)
	iconBack     = svg(`<path d="M9.8 3.2L5 8l4.8 4.8"/>`)
	iconPlay     = svg(`<path d="M5 3.2l7 4.8-7 4.8z"/>`)
	iconStop     = svg(`<rect x="3.8" y="3.8" width="8.4" height="8.4" rx="1.4"/>`)
	iconTrash    = svg(`<path d="M2.6 4.4h10.8M6.2 4.4V2.8h3.6v1.6M4 4.4l.7 9h6.6l.7-9"/>`)
	iconLink     = svg(`<path d="M6.2 9.8a2.6 2.6 0 0 0 3.7 0l2.2-2.2a2.6 2.6 0 0 0-3.7-3.7l-1 1"/><path d="M9.8 6.2a2.6 2.6 0 0 0-3.7 0L3.9 8.4a2.6 2.6 0 0 0 3.7 3.7l1-1"/>`)
	iconSync     = svg(`<path d="M13.2 6.6A5.4 5.4 0 0 0 4 4.1L2.8 5.3M2.8 9.4a5.4 5.4 0 0 0 9.2 2.5l1.2-1.2"/><path d="M2.8 2.4v2.9h2.9M13.2 13.6v-2.9h-2.9"/>`)
	iconCopy     = svg(`<rect x="5.4" y="5.4" width="7.8" height="7.8" rx="1.4"/><path d="M10.6 5.4V3.8A1.4 1.4 0 0 0 9.2 2.4H3.8A1.4 1.4 0 0 0 2.4 3.8v5.4a1.4 1.4 0 0 0 1.4 1.4h1.6"/>`)
	iconServer   = svg(`<rect x="2" y="2.4" width="12" height="4.4" rx="1.4"/><rect x="2" y="9.2" width="12" height="4.4" rx="1.4"/><path d="M4.6 4.6h.01M4.6 11.4h.01"/>`)
	iconChart    = svg(`<path d="M2.4 13.6V8.8M6.4 13.6V4.4M10.4 13.6V6.8M14.4 13.6V2.8"/>`)
	iconEdit     = svg(`<path d="M10.2 2.6l3.2 3.2-7.8 7.8-4 0.8 0.8-4z"/><path d="M8.8 4l3.2 3.2"/>`)
	iconPuzzle   = svg(`<path d="M6.2 2.2h3.6v1.6a1.6 1.6 0 1 0 3.2 0V2.2h1.6v3.6h-1.6a1.6 1.6 0 1 0 0 3.2h1.6v3.6h-3.6v-1.6a1.6 1.6 0 1 0-3.2 0v1.6H2.2V9h1.6a1.6 1.6 0 1 0 0-3.2H2.2V2.2h4z"/>`)
	iconDebug    = svg(`<path d="M3.2 4.2l4 3.8-4 3.8"/><path d="M8.8 12.2h4.2"/>`)
	iconKey      = svg(`<rect x="3" y="7" width="10" height="7" rx="1.6"/><path d="M5.2 7V5.2a2.8 2.8 0 0 1 5.6 0V7"/>`)
)

// trayIcon draws the menu bar icon: three server units, black on macOS for
// the system to tint, accent blue elsewhere to show on light and dark taskbars.
func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{R: 58, G: 110, B: 230, A: 255}
	if runtime.GOOS == "darwin" {
		ink = color.NRGBA{A: 255}
	}
	for _, y := range []int{5, 13, 21} {
		for row := 0; row < 6; row++ {
			for x := 4; x < 28; x++ {
				if (x == 4 || x == 27) && (row == 0 || row == 5) {
					continue // round the corners
				}
				img.SetNRGBA(x, y+row, ink)
			}
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}
