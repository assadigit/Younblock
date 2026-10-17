//go:build !windows

// tray_other.go provides no-op tray hooks so the app package compiles on
// non-Windows development machines; the real tray lives in
// tray_windows.go (file 01, section 9).
package main

import (
	"github.com/assadigit/Younblock/internal/engine"
)

func (a *App) startTray() {
	a.eng.Logger().Info("tray is not available on this platform")
}

func (a *App) stopTray() {}

func (a *App) trayOnEvent(name, payload string) {}

// closeHidesToTray: without a tray icon there is nowhere to hide — the
// window's close button is the quit (the shutdown path stops the engine
// and restores the proxy first), whatever the user's close setting says.
func closeHidesToTray(keepInTray bool) bool { return false && keepInTray }

var _ = engine.PhaseOff
