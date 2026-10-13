// app.go is the thin UI adapter over the engine's JSON API (file 01,
// section 3, rule 4): every method takes and returns the JSON strings the
// engine already speaks, so the frontend, the Android bridge and the
// headless CLI all share one contract.
package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App carries the engine plus the wails context.
type App struct {
	ctx      context.Context
	eng      *engine.Engine
	platform engine.Platform
	browser  browserSession
	quitting bool
}

func newApp() *App {
	plat := platform.New()
	eng, err := engine.New(engine.Options{Platform: plat})
	if err != nil {
		// The window will show the error state; the engine still needs an
		// object. This path is practically unreachable (settings problems
		// fall back to defaults inside engine.New).
		panic(err)
	}
	a := &App{eng: eng, platform: plat}
	eng.SetEventSink(a)
	eng.SetOnQuit(func() {
		a.quitting = true
		runtime.Quit(a.ctx)
	})
	return a
}

// startup receives the wails context and starts the tray.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startTray()
	// Turn on automatically when the app opens (if the user asked for it).
	var settings struct {
		ConnectOnLaunch bool `json:"connectOnLaunch"`
	}
	if err := jsonUnmarshalStrict(a.eng.GetSettingsJSON(), &settings); err == nil && settings.ConnectOnLaunch {
		go a.eng.Start()
	}
}

// domReady runs once the frontend is live.
func (a *App) domReady(ctx context.Context) {
	a.ctx = ctx
}

// shutdown stops the engine (Quit always stops and restores first).
func (a *App) shutdown(ctx context.Context) {
	a.stopBrowserSession() // end the dedicated Chrome session before the engine
	a.stopTray()
	a.eng.Shutdown()
}

// beforeClose hides to the tray instead of quitting; the frontend runs the
// one-time close notice (file 02, section 5) and hides the window itself.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitting {
		return false // real quit: allow the close
	}
	runtime.EventsEmit(ctx, "close-requested")
	return true // prevent: hide to tray instead
}

// showWindow brings the window up (tray click, second instance).
func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
}

// hideWindow hides the window to the tray.
func (a *App) hideWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowHide(a.ctx)
}

// OnEvent forwards engine events to the frontend (and mirrors state to the
// tray icon).
func (a *App) OnEvent(name string, json string) {
	a.trayOnEvent(name, json)
	if name == "state" {
		a.onStateEvent(json)
	}
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, json)
}

// onStateEvent reacts to phase changes for shell-level behavior that must
// stay out of the neutral engine: the optional dedicated Chrome session
// (user-requested; DECISIONS.md D40). "on" launches it when the setting is
// set; "off" and "error" end it (that Chrome is hard-wired to our proxy,
// which just stopped).
func (a *App) onStateEvent(payload string) {
	var st struct {
		Phase string `json:"phase"`
		Port  int    `json:"port"`
	}
	if err := jsonUnmarshalStrict(payload, &st); err != nil {
		return
	}
	switch st.Phase {
	case "on":
		var s struct {
			OpenBrowserOnConnect bool `json:"openBrowserOnConnect"`
		}
		if err := jsonUnmarshalStrict(a.eng.GetSettingsJSON(), &s); err == nil && s.OpenBrowserOnConnect {
			port := st.Port
			go a.launchBrowserSession(port)
		}
	case "off", "error":
		go a.stopBrowserSession()
	}
}

// emitBrowserEvent tells the UI what happened to the dedicated Chrome
// session (R13: nothing changes silently).
func (a *App) emitBrowserEvent(name, arg string) {
	if a.ctx == nil {
		return
	}
	payload := `""`
	if arg != "" {
		payload = marshalString(arg)
	}
	runtime.EventsEmit(a.ctx, name, payload)
}

// marshalString renders s as a JSON string.
func marshalString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// ---- The JSON API surface (mirrors engine/jsonapi.go exactly) ----

// GetState returns the engine state.
func (a *App) GetState() string { return a.eng.GetStateJSON() }

// Start begins connecting.
func (a *App) Start() string { return a.eng.StartJSON() }

// Stop turns off.
func (a *App) Stop() string { return a.eng.StopJSON() }

// Toggle switches on/off.
func (a *App) Toggle() string { return a.eng.ToggleJSON() }

// GetSettings returns the normalized settings.
func (a *App) GetSettings() string { return a.eng.GetSettingsJSON() }

// SaveSettings applies and persists a settings JSON payload.
func (a *App) SaveSettings(payload string) string { return a.eng.SaveSettingsJSON(payload) }

// RunConnectionTest runs the four-step test.
func (a *App) RunConnectionTest() string { return a.eng.RunConnectionTestJSON() }

// AutoFix runs the heal sweep on demand.
func (a *App) AutoFix() string { return a.eng.AutoFixJSON() }

// RestoreWindowsProxy restores the previous internet settings.
func (a *App) RestoreWindowsProxy() string { return a.eng.RestoreWindowsProxyJSON() }

// GetDiagnosticReport returns the redacted diagnostic report.
func (a *App) GetDiagnosticReport() string { return a.eng.GetDiagnosticReportJSON() }

// OpenLogsFolder opens the logs folder in Explorer (shell behavior).
func (a *App) OpenLogsFolder() string {
	out := a.eng.OpenLogsFolderJSON()
	var payload struct {
		Path string `json:"path"`
	}
	if err := jsonUnmarshalStrict(out, &payload); err == nil && payload.Path != "" {
		cmd := exec.Command("explorer", strings.ReplaceAll(payload.Path, "/", "\\"))
		if err := cmd.Start(); err != nil {
			a.eng.Logger().Warn("opening the logs folder failed", "reason", err.Error())
		}
	}
	return out
}

// Quit stops everything and exits.
func (a *App) Quit() string {
	go func() {
		a.eng.Quit() // stops, restores, then calls our onQuit -> runtime.Quit
	}()
	return `{"error":null}`
}

// GetAppInfo returns app metadata.
func (a *App) GetAppInfo() string { return a.eng.GetAppInfoJSON() }

// AppName and Version keep the shell in sync with the configuration block.
const (
	appName    = config.AppName
	appVersion = config.AppVersion
)

// jsonUnmarshalStrict decodes JSON into v (plain unmarshal; named for intent).
func jsonUnmarshalStrict(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
