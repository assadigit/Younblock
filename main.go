// younblock is the Windows app shell (file 01, section 3; file 02,
// section 3): a fixed 360 x 560 dark window over the engine, a tray icon,
// single-instance behavior, and a --minimized launch mode.
package main

import (
	"embed"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	minimized := flag.Bool("minimized", false, "start hidden in the tray")
	flag.Parse()

	app := newApp()

	err := wails.Run(&options.App{
		Title:             "Younblock",
		Width:             360,
		Height:            560,
		DisableResize:     true,
		WindowStartState:  options.Normal,
		StartHidden:       *minimized,
		HideWindowOnClose: false,                      // OnBeforeClose hides to the tray instead
		BackgroundColour:  options.NewRGB(37, 33, 32), // --neutral-950 #252121-family dark: no white flash (D48 exact match)
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
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
