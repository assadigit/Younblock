// younblock is the desktop app shell (file 01, section 3; file 02,
// section 3): a fixed dark window over the engine, a tray icon (Windows),
// single-instance behavior, and a --minimized launch mode.
//
// One screen (the one-UI round): every edition — this shell, the Linux
// window and web editions, the Android WebView — ships the same UI v3
// main screen, so a UI change lands on every version at once. The
// window's client size is the reference design size (360 x 780, the
// refinement round's younblock-ui-reference (1).html .app); the classic
// editions and their VITE_UI_VARIANT/windowSize override are retired.
package main

import (
	"embed"
	"flag"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed packaging/icons/younblock.png
var windowIcon []byte

// windowSize is the design client size of the app window — the
// refinement round's reference screen (UI-SPEC (2).md v7: the .app in
// younblock-ui-reference (1).html is 360 x 780, tall enough that the
// hint and the card slot clear the lever's track outright; the ≤700px
// short-screen CSS keeps smaller windows and phones honest). Kept a
// string (a -X override remains possible for one-off builds), but no
// released lane overrides it anymore.
var windowSize = "360x780"

// designW/designH are the parsed values every sizing decision reads.
var designW, designH = parseWindowSize()

// window minimums (round-2 fix #5): the redesigned screen scales by width
// and stacks vertically, so any client area from 320×560 up renders —
// the window is resizable down to those floors (below ~640 tall, the
// card slot rides over the track's bottom edge by design; the layout
// never clips the lever itself).
const (
	windowMinW = 320
	windowMinH = 560
)

// parseWindowSize reads windowSize once, falling back to the pro size on
// anything malformed (a bad -X flag should degrade, never crash).
func parseWindowSize() (int, int) {
	parts := strings.SplitN(windowSize, "x", 2)
	if len(parts) != 2 {
		return 360, 780
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w < 200 || h < 200 {
		return 360, 780
	}
	return w, h
}

func main() {
	minimized := flag.Bool("minimized", false, "start hidden in the tray")
	flag.Parse()

	app := newApp()

	err := wails.Run(&options.App{
		Title:             "Younblock",
		Width:             designW,
		Height:            designH,
		MinWidth:          windowMinW,
		MinHeight:         windowMinH,
		DisableResize:     false, // round-2 fix #5: resizable, floored at 320×560
		WindowStartState:  options.Normal,
		StartHidden:       *minimized,
		HideWindowOnClose: false,                                            // D93: OnBeforeClose decides — a real quit by default, tray-hide only if opted in
		BackgroundColour:  &options.RGBA{R: 0x3b, G: 0x3a, B: 0x4a, A: 255}, // idle gradient top (#3b3a4a, docs/ui-v3/younblock-tokens.css): no white flash
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []any{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "6f1f4a2a-1c66-4a7d-9db1-78ee27a04b21-younblock",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				app.showWindow()
			},
		},
		Windows: &windows.Options{
			Theme:                windows.Dark,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		// Linux (the native window edition, D67): the window icon makes
		// the taskbar and the alt-tab switcher show the Younblock mark
		// instead of a generic gear.
		Linux: &linux.Options{
			Icon: windowIcon,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
