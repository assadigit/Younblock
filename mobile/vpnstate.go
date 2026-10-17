// vpnstate.go — the M5 honest routing report (docs/android-port 05 §M5,
// doc 04 §5/§8; DECISIONS.md D78). The engine's phase alone is an
// ENGINE truth ("the proxy path works"); the Android product's
// "Connected" claim is a ROUTING truth: the engine serving AND a stack
// bound to its door. VpnState is the one call that answers that
// question honestly, for the shell's notification and its resume-time
// reconciliation — nothing in the product may claim routing without it.
//
// The three states the shell reconciles from:
//
//   - routing=true            — carry traffic; the notification may say On.
//   - tun=true, routing=false — a STALE stack: the engine left serving
//     (off/error/stopping, or a restart cycle is connecting). This is
//     D76's "missed event" surface made observable: the shell unwinds it
//     (StopTun) and never reports Connected.
//   - tun=false, routing=false — nothing bound; the phase field says why
//     (connecting = consent/gate or checks pending; error = typed start
//     failure; off = idle).
//
// gomobile constraint (ADR-0004): string-out JSON, as everywhere.
package mobile

import (
	"encoding/json"

	"github.com/assadigit/Younblock/internal/engine"
)

// vpnStateReport is the VpnState reply shape.
type vpnStateReport struct {
	// Phase is the engine's phase verbatim (off/connecting/on/degraded/
	// error/stopping) — the engine stays the single source of truth
	// (ADR-0006); this report adds the routing truth on top.
	Phase string `json:"phase"`
	// Tun is true while a stack is bound to the facade.
	Tun bool `json:"tun"`
	// Routing is the product's "Connected": the engine serving (on or
	// degraded) AND a stack bound. Degraded still routes — partially,
	// honestly labeled by phase; the notification says so.
	Routing bool `json:"routing"`
	// Door is the loopback SOCKS5 address of the bound stack
	// (diagnostics only; empty when no stack is bound).
	Door string `json:"door,omitempty"`
}

// VpnState returns the honest combined routing state as JSON:
// {"phase":"on","tun":true,"routing":true,"door":"127.0.0.1:port"}.
// The shell calls this on every state event and at resume; it never
// blocks and is safe from any thread.
//
// routing requires THREE live things at once: the engine serving (on or
// degraded), a stack bound to the facade, and that stack's door being
// the LIVE server's door — an engine restart builds a new server, so a
// stack still holding the old one's door is stale (routing=false) even
// while the engine is "on" again. The M5 lifecycle suite caught exactly
// that case: the report must never say routing while the door is dead.
func (m *Engine) VpnState() string {
	m.mu.Lock()
	t := m.tun
	m.mu.Unlock()

	phase := m.eng.GetState().Phase
	rpt := vpnStateReport{Phase: string(phase)}
	if t != nil {
		rpt.Tun = true
		rpt.Door = t.door
		rpt.Routing = (phase == engine.PhaseOn || phase == engine.PhaseDegraded) &&
			m.eng.RunningServer() == t.server
	}
	out, err := json.Marshal(rpt)
	if err != nil {
		// Unreachable for this shape; the honest fallback is the bare phase.
		return `{"phase":` + quoteJSON(string(phase)) + `,"tun":false,"routing":false}`
	}
	return string(out)
}

// quoteJSON renders s as a JSON string (the marshal-failure fallback).
func quoteJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
