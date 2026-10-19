// tun.go — the facade's TUN lifecycle (docs/android-port 04 §2.5/§6,
// roadmap M4): the Kotlin service establishes the VpnService TUN, starts
// the engine, and once the engine is serving calls StartTun(fd, mtu);
// on every path to "off" (user stop, error, onRevoke) it calls StopTun.
//
// The facade is deliberately a thin, honest adapter:
//
//   - StartTun requires the engine to be serving (a live proxy server),
//     opens the SOCKS5 door on that server when it is not already open
//     (loopback, free port), and builds one tunstack around the fd.
//     One stack per engine on-phase: a fresh StartTun needs a StopTun
//     first (TUN_ACTIVE is the honest refusal — it makes a service bug
//     visible instead of silently stacking two readers on one fd).
//   - StopTun is idempotent and closes the door it opened, but only if
//     the same server still runs it (an engine restart builds a new
//     server; its door belongs to its own cycle).
//   - The fd crosses the boundary for good: os.NewFile takes ownership,
//     and StopTun (or Shutdown) releases it. A failed StartTun after
//     the fd was accepted has released it too (the stack's error paths
//     close the wire) — Kotlin never double-closes.
//   - The engine stopping on its own does NOT auto-stop the stack here:
//     the service owns the TUN lifecycle per doc 04 §6 and reacts to
//     the "state" events. If it misses one, the next StartTun refuses
//     loudly, and Shutdown always unwinds the stack.
//
// gomobile constraint (ADR-0004): string-in/string-out, primitives only.
package mobile

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/proxy"
	"github.com/assadigit/Younblock/internal/tunstack"
)

// tunState is one live stack plus the server whose door it uses (the
// identity makes StopTun's door-close safe across engine restarts), and
// the door address for VpnState's diagnostics field (M5).
type tunState struct {
	stack  *tunstack.Stack
	server *proxy.Server
	door   string
}

// tunResult is the StartTun/StopTun reply shape (the jsonapi style:
// {"error":null} plus what the caller may need).
type tunResult struct {
	Error *string `json:"error"`
	Door  string  `json:"door,omitempty"`
}

// tunError renders {"error":"CODE"}.
func tunError(code string) string {
	return fmt.Sprintf(`{"error":%q}`, code)
}

// StartTun binds an established TUN fd (the VpnService's, IFF_NO_PI
// semantics: one raw IP packet per read/write) to the engine's SOCKS5
// door. Requires the engine to be serving; the engine must have reached
// "on" (or degraded) before this call — the door lives on the running
// proxy server. mtu must match the VpnService builder's setting (0
// falls back to the stack's 1500 default).
//
// Returns {"error":null,"door":"127.0.0.1:port"} on success, or
// {"error":"NOT_RUNNING"} (engine not serving), {"error":"TUN_ACTIVE"}
// (StopTun first), {"error":"BAD_FD"} (unusable descriptor),
// {"error":"DOOR"} (the loopback door could not be opened) or
// {"error":"STACK"} (the stack could not be built). On EVERY return the
// fd has been consumed by this call: either the stack owns it (success,
// and the stack's error paths release it) or it is closed here (every
// refusal) — the caller never closes, never double-closes.
func (m *Engine) StartTun(fd int32, mtu int32) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// From the first line on, this call owns the fd (single-owner rule
	// across the bridge; see the doc comment).
	if fd < 0 {
		return tunError("BAD_FD")
	}
	if m.tun != nil {
		closeFd(int(fd))
		return tunError("TUN_ACTIVE")
	}
	// Non-blocking FIRST: os.NewFile registers a foreign fd with the
	// netpoller only when O_NONBLOCK is already set — otherwise the wire
	// pump's reads block in read(2) forever and no Close can unwind them
	// (the facade test caught exactly that). fcntl failing here is the
	// honest BAD_FD: the number is not a usable descriptor.
	if err := setNonblock(int(fd)); err != nil {
		closeFd(int(fd))
		return tunError("BAD_FD")
	}
	srv := m.eng.RunningServer()
	if srv == nil {
		closeFd(int(fd))
		return tunError("NOT_RUNNING")
	}
	if srv.Socks5Port() == 0 {
		if err := srv.StartSocks5("127.0.0.1", 0); err != nil {
			closeFd(int(fd))
			return tunError("DOOR")
		}
	}
	door := srv.Socks5Addr()
	if door == "" {
		closeFd(int(fd))
		return tunError("DOOR")
	}

	// From here the fd is either live in the stack or released by the
	// stack's own error paths.
	st, err := tunstack.New(tunstack.Config{
		Wire:      os.NewFile(uintptr(fd), "tun"),
		MTU:       uint32(mtu),
		ProxyAddr: door,
		// The door dial waits out the door's own honest setup (DoH
		// resolve + upstream dial) before replying; this budget must
		// cover it or flows the desktop doors deliver get aborted
		// mid-setup (the M8-n first-video defect — the old omitted
		// value meant the 10 s stack default, shorter than the door's
		// own ≤21 s budget).
		HandshakeTimeout: config.TunHandshakeTimeout,
		Logger:           m.eng.Logger(),
		// M8-z (ADR-0027 §7): the udp443 fallback timeline names the
		// host only under the logDomains setting (R21).
		LogHostnames: m.eng.Settings().Advanced.LogDomains,
	})
	if err != nil {
		return tunError("STACK")
	}
	m.tun = &tunState{stack: st, server: srv, door: door}

	// nil Error marshals as "error":null (the jsonapi shape).
	return marshalTunResult(tunResult{Door: door})
}

// StopTun unwinds the stack (the fd is released) and closes the door
// this facade opened, unless the engine has moved to a newer server
// (restart cycle) whose door is not ours to close. Idempotent; safe to
// call from any state.
func (m *Engine) StopTun() string {
	m.mu.Lock()
	t := m.tun
	m.tun = nil
	m.mu.Unlock()

	if t == nil {
		return `{"error":null}`
	}
	t.stack.Stop()
	if srv := m.eng.RunningServer(); srv != nil && srv == t.server {
		srv.StopSocks5()
	}
	return `{"error":null}`
}

// TunStats reports the stack counters as JSON (diagnostics only — no
// user-facing claim rides on these, ADR-0007). active is false when no
// stack is bound.
func (m *Engine) TunStats() string {
	m.mu.Lock()
	t := m.tun
	m.mu.Unlock()

	if t == nil {
		return `{"active":false}`
	}
	st := t.stack.Stats()
	out, err := json.Marshal(map[string]any{
		"active":           true,
		"flowsOpened":      st.FlowsOpened,
		"flowsRstUnmapped": st.FlowsRSTUnmapped,
		"flowsOverCap":     st.FlowsOverCap,
		"doorFailures":     st.DoorFailures,
		"dnsQueries":       st.DNSQueries,
		"udpDropped":       st.UDPDropped,
	})
	if err != nil {
		return `{"active":true}`
	}
	return string(out)
}

// marshalTunResult renders the StartTun reply (nil Error → null).
func marshalTunResult(r tunResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"error":"INTERNAL"}`
	}
	return string(b)
}
