//go:build !windows

// browser_other.go — development counterpart of browser_windows.go: the
// dedicated Chrome session never launches on a non-Windows build (D12:
// no real Windows behavior runs during development). The setting is still
// accepted and stored so it carries over to a Windows machine.
package main

func (a *App) launchBrowserSession(port int) {
	a.eng.Logger().Info("dedicated Chrome session requested; not supported on this platform",
		"port", port)
}

func (a *App) stopBrowserSession() {}
