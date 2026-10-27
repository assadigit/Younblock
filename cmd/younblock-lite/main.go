// Command younblock-lite is the lite desktop edition
// (younblock-lite-desktop, DECISIONS.md D114): the dependency-free
// Windows shell over the same engine — one .exe, a plain Win32 window,
// no WebView2, no browser runtime, no installer, no DLLs beside it.
// Copy it to any Windows 10+ box and double-click: that is the whole
// install.
//
// What it shares with every other edition (one source of truth):
//   - the engine (internal/engine) with its phase machine, watchdog,
//     self-heal and honest JSON API;
//   - the Windows platform adapter (internal/platform/windows): the
//     same registry proxy with backup, crash recovery and the
//     --restore-proxy hatch;
//   - the settings file (the main app's settings apply here too, and
//     connect-on-launch / open-browser-on-connect are honored);
//   - the version line (internal/config AppVersion).
//
// What it deliberately leaves out (the lite trade, D114): the web
// screen (WebView2), the tray, the settings panel, the diagnostics
// view, the dedicated browser session. The screen is one word, one
// line, one button — plus an Open YouTube link and the version footer.
//
// On non-Windows hosts this command refuses honestly (the lite edition
// is a Windows app; Linux has the web edition and the native window
// edition), which also keeps `go test` runnable everywhere for the
// OS-neutral halves (view.go, app.go).
package main

import (
        "flag"
        "fmt"
        "os"

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

        // The platform adapter runs crash recovery in New() on Windows (a
        // leftover proxy setting from a crashed run is restored right now,
        // before anything starts) — the lite edition inherits that for
        // free, exactly like the main desktop.
        plat := platform.New()

        if *restore {
                if err := plat.RestoreProxy(); err != nil {
                        fmt.Println("Restore failed:", err)
                        os.Exit(1)
                }
                fmt.Println("Internet settings restored.")
                return
        }

        app, err := NewApp(engine.Options{Platform: plat})
        if err != nil {
                fmt.Println("Startup failed:", err)
                os.Exit(1)
        }

        runUI(app)
}
