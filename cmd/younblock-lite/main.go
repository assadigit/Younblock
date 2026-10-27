// Command younblock-lite is the lite desktop edition
// (younblock-lite-desktop, DECISIONS.md D114/D115): the
// dependency-free Windows shell over the same engine — one .exe, a
// native window, no WebView2, no browser runtime, no installer, no
// DLLs beside it. Copy it to any Windows 10+ box and double-click:
// that is the whole install.
//
// What it shares with every other edition (one source of truth):
//   - the engine (internal/engine) with its phase machine, watchdog,
//     self-heal and honest JSON API;
//   - the Windows platform adapter (internal/platform/windows): the
//     same registry proxy with backup, crash recovery and the
//     --restore-proxy hatch;
//   - the settings file (the main app's settings apply here too, and
//     connect-on-launch / open-browser-on-connect are honored — and
//     editable from the lite menu sheet's toggles);
//   - the version line (internal/config AppVersion).
//
// The screen (D115) is the v3 main screen itself — the same design the
// Wails desktop and the Android WebView render, redrawn natively: the
// state gradient backgrounds with their halftone texture, the glass
// header, the status column (ghost tile, hero word, sub line, tabular
// timer, byte totals), the Open YouTube card, the physical key-cap
// lever with its drag gestures, and the sliding menu sheet. GDI+
// (a Windows system component since XP) paints the shapes; plain GDI
// paints the words; the exe stays one small file with nothing beside
// it. What it deliberately leaves out (the lite trade, D114): the web
// bundle (WebView2), the tray, the diagnostics view, the dedicated
// browser session.
//
// On non-Windows hosts this command refuses honestly (the lite edition
// is a Windows app; Linux has the web edition and the native window
// edition), which also keeps `go test` runnable everywhere for the
// OS-neutral halves (view.go, app.go, geom.go).
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform"
)

func main() {
	restore := flag.Bool("restore-proxy", false, "restore the Windows internet settings and exit")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("%s lite %s\n", config.AppName, config.AppVersion)
		return
	}

	// The run log (D117): every launch leaves its trail — the Windows
	// it landed on, which DPI door opened, what fell back, what broke.
	// A -H windowsgui process has no console; before this existed, a
	// startup panic was literally invisible (that is how v1.6.12's
	// missing-API crash went undiagnosed).
	diagInit()
	defer func() {
		if rec := recover(); rec != nil {
			diagLog("PANIC (fatal): %v\n%s", rec, debug.Stack())
			diagFatal(rec)
			os.Exit(1)
		}
	}()

	// The platform adapter runs crash recovery in New() on Windows (a
	// leftover proxy setting from a crashed run is restored right now,
	// before anything starts) — the lite edition inherits that for
	// free, exactly like the main desktop.
	plat := platform.New()
	diagLog("platform ready (proxy crash recovery ran in New)")

	if *restore {
		if err := plat.RestoreProxy(); err != nil {
			diagLog("--restore-proxy failed: %v", err)
			fmt.Println("Restore failed:", err)
			os.Exit(1)
		}
		diagLog("--restore-proxy: internet settings restored")
		fmt.Println("Internet settings restored.")
		return
	}

	app, err := NewApp(engine.Options{Platform: plat})
	if err != nil {
		diagLog("startup failed: %v", err)
		// fmt alone is invisible under windowsgui — SAY it in a dialog.
		diagFatal(err)
		os.Exit(1)
	}
	diagLog("engine wired; entering the window life")

	runUI(app)
	diagLog("clean exit")
}
