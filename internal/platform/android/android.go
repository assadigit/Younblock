// Package android is the Android platform adapter (docs/android-port
// 04 §2.8, ADR-0003/0011). Routing on Android is owned by the Kotlin
// VpnService (the TUN, the per-app allow-list, the teardown), so the
// OS-proxy surface the desktop adapters implement — Attach, Detach,
// EnsureAttached, SetAutostart, RestoreProxy — is a deliberate no-op here:
// the engine's proxy stays on loopback and no system setting is touched.
//
// What Android adds over the stub is exactly two doors:
//
//   - the protect hook: every engine-upstream socket (tunnel dials and DoH
//     bootstrap connections) must call VpnService.protect(fd) BEFORE it
//     connects, or the socket re-enters the TUN interface and the app
//     routes to itself forever. DialControl turns the Kotlin-side
//     SocketProtector into the netx.ControlFunc the engine already wires
//     into its one outbound dialer (netx/dialer.go wrote the hook for
//     exactly this; ADR-0011).
//   - the Notify callback: the Kotlin shell owns the notification channel,
//     so user-visible notices cross the boundary as a callback instead of
//     the desktop toast/send mechanisms.
//
// The adapter is pure Go on purpose: no Android APIs, no build tag. It
// compiles and its tests run on any host (doc 05 M3 task 2 requires
// host-runnable tests); only the `select` wiring (select_android.go)
// carries the android build tag, and the Kotlin shell provides the two
// doors as plain functions across the gomobile bridge (doc 04 §4).
package android

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/netx"
)

// ProtectFunc protects one socket file descriptor before it connects.
// The Kotlin implementation is VpnService.protect(fd); returning false
// means the socket could not be protected and the dial must be aborted
// (a failed protect is a routing loop waiting to happen — fail safe).
type ProtectFunc func(fd int32) bool

// NotifyFunc delivers a user-visible notice to the shell (the Kotlin side
// owns the notification channel and any rate limiting).
type NotifyFunc func(kind engine.NoticeKind, text string)

// Options configures the adapter. All fields are optional; the zero
// adapter is a well-behaved no-op like the stub (tests and cmd/core use
// it that way), while the mobile/ facade fills in the real doors.
type Options struct {
	// ConfigDir is the app-private configuration directory (Kotlin passes
	// getFilesDir(); settings.json and logs live under it).
	ConfigDir string
	// Protect is the VpnService.protect hook (nil on non-Android runs and
	// tests that do not exercise the dial path).
	Protect ProtectFunc
	// Notify is the shell notice callback (nil drops notices).
	Notify NotifyFunc
}

// Platform implements engine.Platform for Android.
type Platform struct {
	opts Options

	// dialMu guards the lazily built ControlFunc so the engine can call
	// DialControl from any goroutine (the DoH resolver and the tunnel
	// dialer each hold a copy).
	dialMu sync.Mutex
	dial   netx.ControlFunc
}

// New returns an Android platform adapter.
func New(opts Options) *Platform {
	return &Platform{opts: opts}
}

// Name reports "android".
func (p *Platform) Name() string { return "android" }

// Capabilities reports what this platform supports. Per-app routing is
// the VpnService's (doc 04 §2.4); there is no tray, no start-with-OS
// (AND-028: nothing auto-starts after reboot), no system proxy and no
// dedicated browser session on Android.
func (p *Platform) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		PerAppRouting: true,
	}
}

// DialControl returns the protect hook (nil when no protector is set —
// the engine treats nil like the desktops do).
func (p *Platform) DialControl() netx.ControlFunc {
	p.dialMu.Lock()
	defer p.dialMu.Unlock()
	if p.dial != nil || p.opts.Protect == nil {
		return p.dial
	}
	p.dial = func(network, address string, c syscall.RawConn) error {
		var opErr error
		if err := c.Control(func(fd uintptr) {
			if !p.opts.Protect(int32(fd)) {
				// No hostnames or addresses in the message (privacy rules,
				// file 03 R21); an fd number is not sensitive, but keep the
				// error short and classifiable anyway.
				opErr = fmt.Errorf("protect failed for fd %d", fd)
			}
		}); err != nil {
			return err
		}
		return opErr
	}
	return p.dial
}

// Attach is a no-op: the VpnService owns routing; the engine's proxy stays
// on loopback and no system setting is changed (doc 04 §2.8).
func (p *Platform) Attach(port int, s config.Settings) error { return nil }

// EnsureAttached is a no-op that never had to rewrite anything.
func (p *Platform) EnsureAttached(port int, s config.Settings) (bool, error) {
	return false, nil
}

// Detach is a no-op: nothing was attached.
func (p *Platform) Detach() error { return nil }

// SetAutostart is a no-op: nothing auto-starts on Android (AND-028).
func (p *Platform) SetAutostart(enabled bool) error { return nil }

// Notify forwards the notice to the shell callback (no-op without one).
func (p *Platform) Notify(kind engine.NoticeKind, text string) {
	if p.opts.Notify != nil {
		p.opts.Notify(kind, text)
	}
}

// ConfigDir returns the app-private configuration directory.
func (p *Platform) ConfigDir() string { return p.opts.ConfigDir }

// RestoreProxy is a no-op: nothing was ever changed.
func (p *Platform) RestoreProxy() error { return nil }
