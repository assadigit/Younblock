// Package engine is the one brain (file 01, section 3): the state machine,
// start/stop, self-healing and the JSON API the UI talks to. It talks to the
// operating system only through the Platform interface so the same engine
// can later run on Android (file 04).
package engine

import (
        "syscall"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/netx"
)

// Capabilities says what the current platform can do.
type Capabilities struct {
        Tray          bool `json:"tray"`
        StartWithOS   bool `json:"startWithOS"`
        SystemProxy   bool `json:"systemProxy"`
        PerAppRouting bool `json:"perAppRouting"`
        // BrowserSession reports whether the optional dedicated-browser
        // session exists here (the Windows shell launches Chrome with proxy
        // flags; the web edition has no shell to launch anything — D50).
        BrowserSession bool `json:"browserSession"`
}

// NoticeKind separates failure notifications from recovery ones.
type NoticeKind string

const (
        // NoticeFailure tells the user something stopped working.
        NoticeFailure NoticeKind = "failure"
        // NoticeRecovery tells the user it is fixed.
        NoticeRecovery NoticeKind = "recovery"
)

// Platform is the only door to operating-system behavior (file 01, section
// 3, rule 3). The neutral engine packages never import Windows, Wails or
// tray code; only implementations of this interface may.
//
// RestoreProxy is an addition to the spec's list, needed by the
// RestoreWindowsProxy API entry (file 01, section 10) and the
// --restore-proxy flag: it puts back whatever system proxy state the
// platform changed (recorded in DECISIONS.md).
type Platform interface {
        // Name reports "windows", "android" or "stub".
        Name() string
        // Capabilities reports what this platform supports.
        Capabilities() Capabilities
        // DialControl returns the socket hook (nil on desktop).
        DialControl() netx.ControlFunc
        // Attach applies the system proxy for the given mode (no-op on stub).
        Attach(port int, s config.Settings) error
        // EnsureAttached re-asserts the system proxy if another program
        // overwrote it while we are on (the watchdog's anti-clobber check);
        // it reports whether it had to rewrite something (D46).
        EnsureAttached(port int, s config.Settings) (bool, error)
        // Detach restores the system proxy to what it was before Attach.
        Detach() error
        // SetAutostart adds or removes the launch-with-OS entry.
        SetAutostart(enabled bool) error
        // Notify shows a user notification (toast on Windows).
        Notify(kind NoticeKind, text string)
        // ConfigDir returns the per-user configuration directory.
        ConfigDir() string
        // RestoreProxy restores saved proxy state (crash recovery path).
        RestoreProxy() error
}

// ControlFunc matches netx.ControlFunc (kept here so the interface above
// reads without extra imports in call sites).
type ControlFunc = func(network, address string, c syscall.RawConn) error
