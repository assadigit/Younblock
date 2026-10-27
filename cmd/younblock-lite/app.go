// app.go — the lite desktop edition's engine wiring: everything the
// window does to the engine and everything the engine tells the window,
// with no UI toolkit in the middle. The same contract the Wails shell
// implements (app.go at the repo root) and the web edition implements
// (internal/webui), spoken here by a plain channel — so the whole file
// is OS-neutral and testable beside the engine's own tests.
package main

import (
        "encoding/json"
        "sync"

        "github.com/assadigit/Younblock/internal/engine"
)

// App is the lite shell over one engine.
type App struct {
        eng *engine.Engine
        // plat is kept for the few shell-level platform calls (the
        // --restore-proxy hatch); the engine holds its own reference.
        plat engine.Platform

        // events carries parsed state events to the UI poller (the
        // Windows timer drains it; a buffer keeps the engine's emitters
        // never blocked by a busy paint).
        events chan engine.State

        mu       sync.Mutex
        quitting bool
}

// NewApp builds the engine exactly the way every edition does — the
// same platform adapter (real Windows system proxy with backup and
// crash recovery on Windows; the call-recording stub elsewhere, which
// is what makes this file testable on any host), the same settings
// file, the same defaults.
func NewApp(opts engine.Options) (*App, error) {
        eng, err := engine.New(opts)
        if err != nil {
                return nil, err
        }
        a := &App{
                eng:    eng,
                plat:   opts.Platform,
                events: make(chan engine.State, 8),
        }
        eng.SetEventSink(a)
        return a, nil
}

// OnEvent is the engine's sink: state events are parsed and handed to
// the UI; stats events are not a lite-screen concern (no speed lines —
// the lite screen is one word, one line, one button).
func (a *App) OnEvent(name, payload string) {
        if name != "state" {
                return
        }
        var st engine.State
        if err := json.Unmarshal([]byte(payload), &st); err != nil {
                return
        }
        select {
        case a.events <- st:
        default: // the freshest state wins; a stalled older one is dropped
        }
}

// State reads the engine's current state (thread-safe; the UI timer
// polls it beside the event channel so a missed event cannot freeze
// the screen — the belt-and-braces pair the tray icon also uses).
func (a *App) State() engine.State { return a.eng.GetState() }

// Poll returns the freshest state: whatever events queued, else the
// engine's own snapshot. Called from the UI thread only.
func (a *App) Poll() engine.State {
        for {
                select {
                case st := <-a.events:
                        // Drain to the last queued one.
                        last := st
                drain:
                        for {
                                select {
                                case st2 := <-a.events:
                                        last = st2
                                default:
                                        break drain
                                }
                        }
                        return last
                default:
                        return a.eng.GetState()
                }
        }
}

// Connect starts the tunnel (async, like every edition — the phase
// machine reports progress through events).
func (a *App) Connect() { go a.engage() }

// Disconnect stops it: the engine's synchronous honest stop, so it runs
// off the UI thread and the phase events drive the screen meanwhile.
func (a *App) Disconnect() {
        go func() {
                if err := a.eng.Stop(); err != nil {
                        a.eng.Logger().Warn("lite: stop failed", "reason", err.Error())
                }
        }()
}

// engage runs Start on a worker; the engine's own guards make a raced
// second call honest (connecting→cancel, on→no-op).
func (a *App) engage() {
        if err := a.eng.Start(); err != nil {
                a.eng.Logger().Warn("lite: start failed", "reason", err.Error())
        }
}

// Quit is the one deliberate end-of-app path (the D112 semantics, one
// edition over): the engine stops and RESTORES THE WINDOWS PROXY FIRST
// (Engine.Shutdown's contract), then the registered on-quit callback
// fires — the UI turns that into window teardown, so the process never
// exits with the system proxy still pointing at a dead port.
func (a *App) Quit() {
        a.mu.Lock()
        already := a.quitting
        a.quitting = true
        a.mu.Unlock()
        if already {
                return
        }
        go a.eng.Quit()
}

// IsQuitting reports whether the end-of-app path already began (the
// close button asks, so a slow stop never double-fires).
func (a *App) IsQuitting() bool {
        a.mu.Lock()
        defer a.mu.Unlock()
        return a.quitting
}

// OnEngineQuit registers the callback the engine invokes after its
// shutdown has stopped and restored everything (the UI exits then).
func (a *App) OnEngineQuit(fn func()) { a.eng.SetOnQuit(fn) }

// ConnectOnLaunch reads the same settings file the main desktop reads:
// a user who turned "connect on launch" on gets it from the lite
// edition too — one settings file, every edition, no second knob.
func (a *App) ConnectOnLaunch() bool {
        var s struct {
                ConnectOnLaunch bool `json:"connectOnLaunch"`
        }
        if err := json.Unmarshal([]byte(a.eng.GetSettingsJSON()), &s); err != nil {
                return false
        }
        return s.ConnectOnLaunch
}

// OpenYouTubeOnConnect reads the desktop's openBrowserOnConnect
// setting: the main shell opens a dedicated browser window; the lite
// shell's honest equivalent is opening the default browser (through
// the live tunnel, because the system proxy is set).
func (a *App) OpenYouTubeOnConnect() bool {
        var s struct {
                OpenBrowserOnConnect bool `json:"openBrowserOnConnect"`
        }
        if err := json.Unmarshal([]byte(a.eng.GetSettingsJSON()), &s); err != nil {
                return false
        }
        return s.OpenBrowserOnConnect
}

// StopNow stops the engine synchronously — the OS session-end path
// (WM_ENDSESSION), where no goroutine gets to run again after the
// handler returns: the proxy must be back before the process dies.
func (a *App) StopNow() { _ = a.eng.Stop() }

// RestoreProxy is the recovery hatch (the --restore-proxy flag): put
// back whatever the backup recorded, and exit — the same door
// cmd/core's --restore-proxy opens, because the backup file is shared.
func (a *App) RestoreProxy() error { return a.plat.RestoreProxy() }
