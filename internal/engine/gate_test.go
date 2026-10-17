// gate_test.go — host-runnable tests for the M5 platform start gate
// (DECISIONS.md D77): the engine's connect run waits for an optional
// platform door BEFORE the listener, holding the phase at "connecting"
// outside the start budget; cancellations (Stop, a second Start) exit
// quietly and never overwrite the canceller's phase transition.
//
// The gated platform here is the test stub plus a channel gate — the
// same contract internal/platform/android implements for the real
// adapter (its own suite covers the channel semantics directly).
package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// gatedStub is a stub Platform that also implements engine.StartGater,
// including the M7 refusal (D81): refuse() delivers a typed refusal to
// the waiting run — the same contract internal/platform/android
// implements for the real adapter (its own suite covers the channel
// semantics directly).
type gatedStub struct {
	*stub.Platform
	gate    chan struct{} // closed = open
	refusal chan error    // buffered(1): a typed refusal
}

func (g *gatedStub) WaitStartReady(ctx context.Context) error {
	select {
	case <-g.gate:
		return nil
	case err := <-g.refusal:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// refuse hands the gate a typed refusal (the adapter's RefuseStart).
func (g *gatedStub) refuse(code string) {
	g.refusal <- &engine.PlatformRefusalError{Code: code}
}

// acceptAllChecker passes every check (the connect run completes the
// moment the gate opens).
type acceptAllChecker struct{}

func (acceptAllChecker) QuickCheck(ctx context.Context, m config.Method) bool { return true }
func (acceptAllChecker) HealCheck(ctx context.Context, m config.Method) bool  { return true }

func newGatedEngine(t *testing.T, startTimeout time.Duration) (*engine.Engine, *gatedStub) {
	t.Helper()
	dir := t.TempDir()
	plat := &gatedStub{Platform: stub.NewWithDir(dir), gate: make(chan struct{}), refusal: make(chan error, 1)}
	eng, err := engine.New(engine.Options{
		Platform:     plat,
		LogDir:       dir + "/logs",
		Checker:      acceptAllChecker{},
		StartTimeout: startTimeout,
	})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(eng.Shutdown)
	return eng, plat
}

// waitForGatePhase polls the phase with an explicit deadline.
func waitForGatePhase(t *testing.T, eng *engine.Engine, want engine.Phase, timeout time.Duration) engine.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if st := eng.GetState(); st.Phase == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("phase %s did not arrive within %v (now %s)", want, timeout, eng.GetState().Phase)
	return engine.State{}
}

// TestStartGateHoldsConnect — the run holds in "connecting" while the
// gate is closed, and the hold does NOT burn the start budget: the wait
// here outlives a 200 ms StartTimeout, yet the engine still reaches "on"
// once the gate opens (a timeout-burning hold would have gone degraded).
func TestStartGateHoldsConnect(t *testing.T) {
	eng, plat := newGatedEngine(t, 200*time.Millisecond)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Held: connecting for longer than the whole start budget.
	time.Sleep(600 * time.Millisecond)
	if st := eng.GetState(); st.Phase != engine.PhaseConnecting {
		t.Fatalf("held phase = %s, want connecting (gate closed must hold, outside the budget)", st.Phase)
	}

	close(plat.gate) // the shell granted readiness
	waitForGatePhase(t, eng, engine.PhaseOn, 5*time.Second)
}

// TestStartGateStopDuringHold — Stop while the run is held cancels it
// cleanly: the phase lands off promptly, and the abandoned goroutine
// never flips the phase afterwards (the canceller owns the transition).
func TestStartGateStopDuringHold(t *testing.T) {
	eng, _ := newGatedEngine(t, time.Hour)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseConnecting, 2*time.Second)

	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if st := eng.GetState(); st.Phase != engine.PhaseOff {
		t.Fatalf("phase after Stop = %s, want off", st.Phase)
	}
	// The gate stays closed: nothing may wake the abandoned run. Give it
	// a window to misbehave, then assert the phase is still the
	// canceller's.
	time.Sleep(300 * time.Millisecond)
	if st := eng.GetState(); st.Phase != engine.PhaseOff {
		t.Fatalf("phase drifted to %s after Stop (abandoned run touched it)", st.Phase)
	}
}

// TestStartGateSecondStartCancels — a second Start while connecting
// cancels the held run (the existing button semantics); the engine then
// behaves normally (off, startable again).
func TestStartGateSecondStartCancels(t *testing.T) {
	eng, plat := newGatedEngine(t, time.Hour)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start 1: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseConnecting, 2*time.Second)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start 2: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseOff, 2*time.Second)

	// A third Start with the gate open connects to on.
	close(plat.gate)
	if err := eng.Start(); err != nil {
		t.Fatalf("Start 3: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseOn, 5*time.Second)
}

// TestStartGateOpenIsTransparent — an open gate costs nothing: Start
// with the gate already open connects normally (the desktop-shaped
// path, plus the android case where the TUN is already established).
func TestStartGateOpenIsTransparent(t *testing.T) {
	eng, plat := newGatedEngine(t, 10*time.Second)
	close(plat.gate)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseOn, 5*time.Second)
}

// TestStartGateRefusalFailsStart (D81) — a platform refusal at the gate
// lands the run in the ordinary failure path: phase "error" with the
// typed code in the state (the page's honest "Can't Connect" with a
// reason), the start budget unburned, no listener ever started.
func TestStartGateRefusalFailsStart(t *testing.T) {
	eng, plat := newGatedEngine(t, time.Hour)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseConnecting, 2*time.Second)

	plat.refuse("NO_TARGET_APPS")
	st := waitForGatePhase(t, eng, engine.PhaseError, 2*time.Second)
	if st.Error == nil || st.Error.Code != "NO_TARGET_APPS" {
		t.Fatalf("state error = %+v, want the typed NO_TARGET_APPS code", st.Error)
	}
	// No listener was ever started (the gate precedes it), so the run's
	// failure path had nothing to unwind — proved by the clean Stop below.
	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop after refusal: %v", err)
	}
}

// TestStartGateRefusalThenRecovery (D81) — a refusal poisons nothing:
// the next Start with the gate OPEN connects normally (the shell
// re-runs consent/TUN before opening; the engine just sees an open
// door on the retry).
func TestStartGateRefusalThenRecovery(t *testing.T) {
	eng, plat := newGatedEngine(t, 10*time.Second)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start 1: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseConnecting, 2*time.Second)
	plat.refuse("NO_TARGET_APPS")
	waitForGatePhase(t, eng, engine.PhaseError, 2*time.Second)

	// The error phase is terminal until the next Start (Stop from error
	// is the engine's documented no-op) — the retry IS the recovery: a
	// Start with the gate open reaches on, the refusal left no residue.
	close(plat.gate)
	if err := eng.Start(); err != nil {
		t.Fatalf("Start 2: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseOn, 5*time.Second)
}

// TestStartGateCancellationBeatsRefusal — D77's quiet-cancellation rule
// survives D81: when the run is cancelled and a refusal lands after it,
// the canceller owns the phase (off), and the refusal never flips it.
// The buffered refusal channel models the adapter's atomic delivery to
// a run that already left — the send succeeds, nobody reads it.
func TestStartGateCancellationBeatsRefusal(t *testing.T) {
	eng, plat := newGatedEngine(t, time.Hour)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForGatePhase(t, eng, engine.PhaseConnecting, 2*time.Second)

	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if st := eng.GetState(); st.Phase != engine.PhaseOff {
		t.Fatalf("phase after Stop = %s, want off", st.Phase)
	}
	// A late refusal (delivered after the run was cancelled) must not
	// resurrect an error phase — the canceller owns the transition.
	plat.refuse("NO_TARGET_APPS")
	time.Sleep(200 * time.Millisecond)
	if st := eng.GetState(); st.Phase != engine.PhaseOff {
		t.Fatalf("phase drifted to %s after a late refusal (canceller must own it)", st.Phase)
	}
}
