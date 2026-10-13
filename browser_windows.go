//go:build windows

// browser_windows.go — spawns and ends the optional dedicated Chrome
// session (user-requested; DECISIONS.md D40). Windows-only: it looks Chrome
// up in the registry and the standard folders, launches it with the legacy
// flags from internal/browser, and ends it when the engine turns off. Like
// every Windows adapter it is never run during development on Linux; the
// logic is unit-tested in the neutral internal/browser package (D12).
package main

import (
        "os"
        "os/exec"
        "path/filepath"

        "github.com/assadigit/Younblock/internal/browser"
        "github.com/assadigit/Younblock/internal/config"
        "golang.org/x/sys/windows/registry"
)

// chromePath finds chrome.exe: the App Paths registry key first (covers
// custom install locations), then the standard folders.
func chromePath() string {
        if p, err := appPathFromRegistry(); err == nil && p != "" {
                if _, statErr := os.Stat(p); statErr == nil {
                        return p
                }
        }
        if p := browser.Find(browser.Candidates(os.Getenv), func(path string) bool {
                _, err := os.Stat(path)
                return err == nil
        }); p != "" {
                return p
        }
        return ""
}

// appPathFromRegistry reads the chrome.exe entry Windows keeps for
// "App Paths".
func appPathFromRegistry() (string, error) {
        k, err := registry.OpenKey(registry.CURRENT_USER,
                `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
        if err != nil {
                k, err = registry.OpenKey(registry.LOCAL_MACHINE,
                        `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
                if err != nil {
                        return "", err
                }
        }
        defer k.Close()
        v, _, err := k.GetStringValue("")
        return v, err
}

// launchBrowserSession starts the dedicated Chrome window for the given
// proxy port. R13 (nothing changes silently): the outcome is logged and
// mirrored to the UI as a browser:opened / browser:failed event.
func (a *App) launchBrowserSession(port int) {
        exe := chromePath()
        if exe == "" {
                a.eng.Logger().Warn("dedicated Chrome session requested but chrome.exe was not found")
                a.emitBrowserEvent("browser:failed", "not-found")
                return
        }
        profile := filepath.Join(os.Getenv("LocalAppData"), config.AppName, browser.ProfileDirName)
        cmd := exec.Command(exe, browser.Args(profile, port)...)

        a.browser.mu.Lock()
        if a.browser.running {
                a.browser.mu.Unlock()
                return // one session at a time
        }
        a.browser.mu.Unlock()

        if err := cmd.Start(); err != nil {
                a.eng.Logger().Warn("starting the dedicated Chrome session failed", "reason", err.Error())
                a.emitBrowserEvent("browser:failed", "start-failed")
                return
        }
        a.browser.mu.Lock()
        a.browser.cmd = cmd
        a.browser.running = true
        a.browser.mu.Unlock()
        a.eng.Logger().Info("dedicated Chrome session opened",
                "port", port, "profileDir", profileDirName)
        a.emitBrowserEvent("browser:opened", "")

        // Detect the user closing Chrome themselves so stopBrowserSession
        // knows there is nothing left to end.
        go func() {
                _ = cmd.Wait()
                a.browser.mu.Lock()
                a.browser.running = false
                a.browser.cmd = nil
                a.browser.mu.Unlock()
        }()
}

// stopBrowserSession ends the dedicated Chrome session this app started.
// That Chrome is hard-wired to this app's proxy; once the proxy stops it
// has no internet, so closing it with the session is the old app's
// behavior (D2) and the sane one. Quiet when nothing is running.
func (a *App) stopBrowserSession() {
        a.browser.mu.Lock()
        cmd := a.browser.cmd
        running := a.browser.running
        a.browser.cmd = nil
        a.browser.running = false
        a.browser.mu.Unlock()
        if !running || cmd == nil || cmd.Process == nil {
                return
        }
        if err := cmd.Process.Kill(); err != nil {
                a.eng.Logger().Warn("closing the dedicated Chrome session failed", "reason", err.Error())
        }
        a.eng.Logger().Info("dedicated Chrome session closed")
        a.emitBrowserEvent("browser:closed", "")
}

// profileDirName keeps the log line free of a user-specific path.
const profileDirName = "%LOCALAPPDATA%/" + config.AppName + "/" + browser.ProfileDirName
