// younblock is the desktop app shell (file 01, section 3; file 02,
// section 3): a fixed dark window over the engine, a tray icon (Windows),
// single-instance behavior, and a --minimized launch mode.
//
// The same source builds both editions (D66): the redesigned connect
// screen (360 x 628) and the classic simple UI (360 x 760, the v0.6.0
// screen). The frontend picks its screen from VITE_UI_VARIANT at build
// time; this shell picks the window size from the same fact via
//
//	wails build -ldflags "-X main.windowSize=360x760"
//
// so a classic frontend never lands in a short pro window or vice versa.
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

// windowSize is the design client size of the edition being built. The
// pro default lives here; the classic build overrides it through the
// linker (see the package comment). Kept a string because -X can only
// set strings.
var windowSize = "360x640"

// designW/designH are the parsed values every sizing decision reads.
var designW, designH = parseWindowSize()

// window minimums (round-2 fix #5): the redesigned screen scales by width
// and stacks vertically, so any client area from 320×560 up renders fully
// — the window is resizable down to those floors.
const (
	windowMinW = 320
	windowMinH = 560
)

// parseWindowSize reads windowSize once, falling back to the pro size on
// anything malformed (a bad -X flag should degrade, never crash).
func parseWindowSize() (int, int) {
	parts := strings.SplitN(windowSize, "x", 2)
	if len(parts) != 2 {
		return 360, 640
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w < 200 || h < 200 {
		return 360, 640
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
		BackgroundColour:  &options.RGBA{R: 0x3c, G: 0x3b, B: 0x4b, A: 255}, // not-connected surface top: no white flash
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
