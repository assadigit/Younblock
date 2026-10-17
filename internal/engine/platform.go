// Package engine is the one brain (file 01, section 3): the state machine,
// start/stop, self-healing and the JSON API the UI talks to. It talks to the
// operating system only through the Platform interface so the same engine
// can later run on Android (file 04).
package engine

import (
	"context"
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

// StartGater is an optional Platform extension (roadmap M5, DECISIONS.md
// D77): a platform whose product semantics require an external
// permission or readiness before traffic may route implements it, and
// the engine's connect run then waits for the door to open BEFORE the
// listener and the checks, holding the phase at "connecting" — outside
// the start budget, because a permission dialog is not a network
// failure. The engine invents no new phase for this (ADR-0006): the
// page honestly reads "connecting" while the platform decides, and the
// shell's notification carries the precise reason (consent pending).
//
// Only the Android adapter implements it (the VPN consent plus the
// established TUN). Desktop platforms do not: the type assertion in the
// connect run fails and their Start path is unchanged, byte for byte.
//
// M7 (D81) extends the same door with a REFUSAL: WaitStartReady may
// return a *PlatformRefusalError instead of nil — the platform
// discovered the connect cannot honestly proceed (on Android today:
// none of the allow-list's apps is installed). The connect run then
// routes it through its ordinary failure path (phase "error" with the
// typed code), so the page shows the honest failure from the engine's
// phase machine — the single truth, ADR-0006 — instead of a parallel
// shell-side error UI or a silent unwind. A context cancellation still
// returns ctx.Err() and exits quietly: the canceller owns the phase
// transition (D77, unchanged).
type StartGater interface {
	// WaitStartReady blocks until the platform permits the connect
	// run to proceed, until ctx is done, or until the platform
	// refuses the run (a *PlatformRefusalError). On ctx.Done the run
	// exits quietly — the canceller (Stop, a second Start, Shutdown)
	// already owns the phase transition, and this goroutine must
	// never overwrite it.
	WaitStartReady(ctx context.Context) error
}

// PlatformRefusalError is the D81 refusal the platform's start gate
// can hand a waiting connect run: the connect cannot proceed for a
// platform reason the engine cannot know (on Android: PackageManager
// found none of the allow-list's packages installed — NO_TARGET_APPS).
// The code vocabulary is the engine's UI error codes; the mobile facade
// whitelists what may cross the bridge.
type PlatformRefusalError struct {
	Code string
}

func (e *PlatformRefusalError) Error() string {
	return "platform refused the connect: " + e.Code
}

// ControlFunc matches netx.ControlFunc (kept here so the interface above
// reads without extra imports in call sites).
type ControlFunc = func(network, address string, c syscall.RawConn) error
