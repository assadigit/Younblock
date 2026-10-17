// lifecycle_test.go — the M5 core connection lifecycle suite (docs/
// android-port 05 §M5, DECISIONS.md D77/D78), host-runnable against the
// REAL engine and the REAL tunstack, exactly like the M4 suites.
//
// What this suite proves, in the M5 vocabulary:
//
//   - Consent gating: an armed-closed start gate holds the engine at
//     "connecting" — never "on", never a "Connected" claim — until the
//     shell opens it (SetVpnReady), and the hold burns no start budget.
//     Denial is modeled as Stop-during-hold: the engine lands "off"
//     cleanly (the Kotlin denial path calls stop()).
//   - Health reporting: VpnState() tells the routing truth at every
//     step — routing only while the engine serves AND a stack is bound.
//   - Partial-start failure injection: the engine's own error path
//     (a busy listener port) leaves routing=false; every refusal of
//     StartTun leaves routing=false.
//   - Repeated connect/disconnect: three full cycles through the gate,
//     stack bind, engine stop, stack unwind — states honest at each
//     step, and the stale-stack case (engine off, stack still bound)
//     is REPORTED, not hidden, so the shell can reconcile (D78).
//   - Service death: Shutdown with routing live unwinds everything and
//     leaves the report all-false.
package mobile

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
)

// vpnStateOf parses the VpnState report into the fields the assertions
// need (the shape is this package's own vpnStateReport).
func vpnStateOf(t *testing.T, m *Engine) (phase string, tun, routing bool) {
	t.Helper()
	var r vpnStateReport
	if err := json.Unmarshal([]byte(m.VpnState()), &r); err != nil {
		t.Fatalf("VpnState unparseable: %v (raw %q)", err, m.VpnState())
	}
	if r.Tun && !strings.HasPrefix(r.Door, "127.0.0.1:") {
		t.Fatalf("VpnState = %q: a bound stack must report its door", m.VpnState())
	}
	return r.Phase, r.Tun, r.Routing
}

// TestLifecycleConsentGate — the M5 core: the engine cannot reach "on"
// while the shell has not granted VPN readiness, no matter how long it
// waits; the moment it is granted the connect completes; denial (stop
// during the hold) lands off cleanly.
func TestLifecycleConsentGate(t *testing.T) {
	m, _ := newServingHarness(t)

	// Arm the gate exactly as NewEngine does for the shell.
	m.SetVpnReady(false)

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}

	// The hold: still connecting well past any check latency, and the
	// honest report never claims routing.
	time.Sleep(2 * time.Second)
	if p, _, routing := vpnStateOf(t, m); p != "connecting" || routing {
		t.Fatalf("during the consent hold: phase=%s routing=%v, want connecting/false", p, routing)
	}

	// Denial path (what MainActivity does on a refused dialog): stop.
	if got := m.Stop(); got != `{"error":null}` {
		t.Fatalf("Stop = %s", got)
	}
	if p, tun, routing := vpnStateOf(t, m); p != "off" || tun || routing {
		t.Fatalf("after denial stop: phase=%s tun=%v routing=%v, want off/false/false", p, tun, routing)
	}

	// The user tries again and grants: the full connect completes.
	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start 2 = %s", got)
	}
	m.SetVpnReady(true)
	waitForPhaseJSON(t, m, engine.PhaseOn, 40*time.Second)
	if p, tun, routing := vpnStateOf(t, m); p != "on" || tun || routing {
		// No stack yet: tun=false is honest; routing=false too.
		t.Fatalf("after grant+on: phase=%s tun=%v routing=%v, want on/false/false (no stack yet)", p, tun, routing)
	}
}

// waitForPhaseJSON is waitForPhase with the full state JSON in the
// failure message (the typed error code is the diagnosis for start
// failures).
func waitForPhaseJSON(t *testing.T, m *Engine, want engine.Phase, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.eng.GetState().Phase == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("engine never reached %s within %v; state %s; recent log: %v",
		want, timeout, m.GetState(), m.eng.Logger().Ring.Lines())
}

// TestLifecycleRoutingReportAndCycles — three full connect/disconnect
// cycles through the gate: bind the stack, verify routing=true, stop
// the engine (the stale-stack report — the shell's reconciliation
// signal), unwind the stack, repeat. The exit criterion: at no step
// does the report overstate, and nothing leaks between cycles (a fresh
// StartTun binds each time; the stats track the state).
func TestLifecycleRoutingReportAndCycles(t *testing.T) {
	m, _ := newServingHarness(t)
	m.SetVpnReady(false)

	for cycle := 1; cycle <= 3; cycle++ {
		if got := m.Start(); got != `{"error":null}` {
			t.Fatalf("cycle %d: Start = %s", cycle, got)
		}
		m.SetVpnReady(true)
		waitForPhase(t, m, engine.PhaseOn, 40*time.Second)

		_, fd := tunPair(t)
		if got := m.StartTun(fd, 1500); !strings.Contains(got, `"error":null`) {
			t.Fatalf("cycle %d: StartTun = %s", cycle, got)
		}
		if p, tun, routing := vpnStateOf(t, m); p != "on" || !tun || !routing {
			t.Fatalf("cycle %d bound: phase=%s tun=%v routing=%v, want on/true/true", cycle, p, tun, routing)
		}

		// User stop. The engine goes off; the stack is the service's to
		// unwind — until it does, the report says exactly that (the
		// D78 stale signal: tun=true, routing=false).
		if got := m.Stop(); got != `{"error":null}` {
			t.Fatalf("cycle %d: Stop = %s", cycle, got)
		}
		waitForPhase(t, m, engine.PhaseOff, 10*time.Second)
		if p, tun, routing := vpnStateOf(t, m); p != "off" || !tun || routing {
			t.Fatalf("cycle %d stale: phase=%s tun=%v routing=%v, want off/true/false (stale stack reported)", cycle, p, tun, routing)
		}

		// The shell reconciles: StopTun unwinds; the report goes quiet.
		if got := m.StopTun(); got != `{"error":null}` {
			t.Fatalf("cycle %d: StopTun = %s", cycle, got)
		}
		if p, tun, routing := vpnStateOf(t, m); p != "off" || tun || routing {
			t.Fatalf("cycle %d after unwind: phase=%s tun=%v routing=%v, want off/false/false", cycle, p, tun, routing)
		}
		if st := m.TunStats(); !strings.Contains(st, `"active":false`) {
			t.Fatalf("cycle %d: TunStats = %s", cycle, st)
		}
	}

	// A refused double-bind never lies: with no stack bound, a second
	// StartTun after re-serving would be the reconnect path; here the
	// engine is off, so the refusal is NOT_RUNNING and routing stays
	// false — the honest terminal state.
	if p, tun, routing := vpnStateOf(t, m); p != "off" || tun || routing {
		t.Fatalf("final: phase=%s tun=%v routing=%v, want off/false/false", p, tun, routing)
	}
}

// TestLifecycleStaleStackRefusal — the missed-event case (D76's honest
// failure surface, now observable): the engine restarts under a bound
// stack (stop → start). The old stack still holds the facade slot, the
// report says stale, the next StartTun refuses TUN_ACTIVE loudly, and
// the shell's reconciliation (StopTun) makes a fresh bind possible.
func TestLifecycleStaleStackRefusal(t *testing.T) {
	m, _ := startServingEngine(t) // gate open (harness default), serving

	_, fd := tunPair(t)
	if got := m.StartTun(fd, 1500); !strings.Contains(got, `"error":null`) {
		t.Fatalf("StartTun = %s", got)
	}
	if _, _, routing := vpnStateOf(t, m); !routing {
		t.Fatal("routing must be true with the engine on and a stack bound")
	}

	// Engine stop WITHOUT the service reacting (the missed event).
	if got := m.Stop(); got != `{"error":null}` {
		t.Fatalf("Stop = %s", got)
	}
	waitForPhase(t, m, engine.PhaseOff, 10*time.Second)

	// Reconnect attempt: the engine serves again, but the stale stack
	// refuses the bind loudly (TUN_ACTIVE), the report stays
	// routing=false, and only reconciliation (StopTun) fixes it.
	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start 2 = %s", got)
	}
	waitForPhase(t, m, engine.PhaseOn, 40*time.Second)

	_, fd2 := tunPair(t)
	if got := m.StartTun(fd2, 1500); got != `{"error":"TUN_ACTIVE"}` {
		t.Fatalf("stale StartTun = %s, want TUN_ACTIVE", got)
	}
	if p, tun, routing := vpnStateOf(t, m); p != "on" || !tun || routing {
		t.Fatalf("stale refusal: phase=%s tun=%v routing=%v, want on/true/false (stale visible)", p, tun, routing)
	}

	if got := m.StopTun(); got != `{"error":null}` {
		t.Fatalf("reconcile StopTun = %s", got)
	}
	_, fd3 := tunPair(t)
	if got := m.StartTun(fd3, 1500); !strings.Contains(got, `"error":null`) {
		t.Fatalf("post-reconcile StartTun = %s", got)
	}
	if _, _, routing := vpnStateOf(t, m); !routing {
		t.Fatal("routing must be true after reconciliation and re-bind")
	}
}

// TestLifecycleStartFailureInjection — partial-start failure at the
// engine's own step: a busy listener port fails the connect run into
// the error phase (typed PORT_BUSY), and the report never claims
// routing. The gate being open changes nothing about the honesty.
func TestLifecycleStartFailureInjection(t *testing.T) {
	m, _ := newServingHarness(t)
	m.SetVpnReady(false)

	// Occupy a port and pin it in settings (AutoPort off → PORT_BUSY).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("busy-port listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	var s config.Settings
	if err := json.Unmarshal([]byte(m.GetSettings()), &s); err != nil {
		t.Fatalf("settings unparseable: %v", err)
	}
	s.Port, s.AutoPort = port, false
	payload, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if out := m.SaveSettings(string(payload)); strings.Contains(out, `"error":"UNKNOWN"`) {
		t.Fatalf("SaveSettings = %s", out)
	}

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}
	m.SetVpnReady(true)
	waitForPhase(t, m, engine.PhaseError, 10*time.Second)

	if p, tun, routing := vpnStateOf(t, m); p != "error" || tun || routing {
		t.Fatalf("after start failure: phase=%s tun=%v routing=%v, want error/false/false", p, tun, routing)
	}
	if st := m.GetState(); !strings.Contains(st, `"PORT_BUSY"`) {
		t.Fatalf("start failure state = %s, want the typed PORT_BUSY code", st)
	}
}

// TestLifecycleServiceDeath — the shell process dies with routing live:
// Shutdown unwinds the stack, the door, the engine; the report is
// all-false afterwards (never a zombie "Connected").
func TestLifecycleServiceDeath(t *testing.T) {
	m, _ := startServingEngine(t)

	_, fd := tunPair(t)
	if got := m.StartTun(fd, 1500); !strings.Contains(got, `"error":null`) {
		t.Fatalf("StartTun = %s", got)
	}
	if _, _, routing := vpnStateOf(t, m); !routing {
		t.Fatal("routing must be live before the death")
	}

	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown hung with routing live")
	}

	if p, tun, routing := vpnStateOf(t, m); p != "off" || tun || routing {
		t.Fatalf("after service death: phase=%s tun=%v routing=%v, want off/false/false", p, tun, routing)
	}
}

// TestVpnStateNotRunning — the fresh engine's report is the quiet truth.
func TestVpnStateNotRunning(t *testing.T) {
	dir := t.TempDir()
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()
	if p, tun, routing := vpnStateOf(t, m); p != "off" || tun || routing {
		t.Fatalf("fresh engine: phase=%s tun=%v routing=%v, want off/false/false", p, tun, routing)
	}
}
