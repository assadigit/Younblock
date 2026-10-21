package engine_test

// m8an_test.go — the wait-until-ready round's pins (M8-an,
// ADR-0052/0053). The debug.30 field report: "The app reverted to our
// old problem, waiting a lot for initial video to load, which took
// like 1-2 minutes… I think the app connection time is too short, so
// when it automatically switches to youtube app it's not ready yet."
// Two behavior changes answer it, each pinned here both ways:
//
//   - ADR-0052 — the warm gate: the serving run's readiness is HELD
//     (warm=false) through a settle window and one light re-check
//     before it flips, the flip is EMITTED as a state event (the
//     hand-off's trigger), a failing network never traps the gate
//     (the cap), and the run's end re-arms it.
//   - ADR-0053 — the persisted dead-cluster marks: a file in the
//     config directory seeds the next session's cluster memory from
//     second zero, and the report's cluster line says so (seeded=).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// newWarmEngine is newTestEngine with the warm gate's timing shrunk
// (the opts overrides ADR-0052 adds) and a caller-owned config
// directory, so a dead-cluster file can be planted before the engine
// reads it at start.
func newWarmEngine(t *testing.T, dir string, settle, retry, cap time.Duration) (*engine.Engine, *stub.Platform, *recordingSink, *fakeNetwork) {
	t.Helper()
	plat := stub.NewWithDir(dir)
	net := &fakeNetwork{}
	net.acceptAll(true)
	eng, err := engine.New(engine.Options{
		Platform:       plat,
		LogDir:         dir + "/logs",
		Checker:        net,
		WarmGateSettle: settle,
		WarmGateRetry:  retry,
		WarmGateCap:    cap,
	})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	sink := &recordingSink{}
	eng.SetEventSink(sink)
	t.Cleanup(eng.Shutdown)
	return eng, plat, sink, net
}

// TestWarmGateHoldsThenOpens: the gate holds warm closed through the
// settle window after on (the tester's "make the user wait"), opens
// only after a light re-check ran (the "only you're sure"), and the
// flip rides a state event (the Kotlin hand-off's trigger).
//
// Both ways (recorded in ADR-0052's Evidence): stashing the
// startWarmGate hook makes the hold vanish — warm reads true the
// moment on does — and this test fails at the hold assertion; without
// the state emission the sink assertion fails.
func TestWarmGateHoldsThenOpens(t *testing.T) {
	eng, _, sink, net := newWarmEngine(t, t.TempDir(), 800*time.Millisecond, 100*time.Millisecond, 6*time.Second)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, eng, 5*time.Second, phaseOn)

	// The hold: through the settle window (half of it sampled with a
	// wide margin) warm stays closed.
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if st := eng.GetState(); st.Warm {
			t.Fatalf("warm=true inside the settle window: the gate must hold the hand-off closed (state %+v)", st)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The re-check: at least one QuickCheck beyond the connect's own
	// before the gate may open.
	callsBefore := net.callCount()
	st := waitForState(t, eng, 6*time.Second, func(s engine.State) bool { return s.Warm })
	if !st.Warm {
		t.Fatalf("warm never flipped: %+v", st)
	}
	if got := net.callCount(); got <= callsBefore {
		t.Errorf("light re-checks = %d (before %d): the gate opened without re-proving the chain", got, callsBefore)
	}

	// The emission: the flip rode a state event — the Kotlin shell's
	// hand-off and the connect screen's warming copy both key on it.
	found := false
	for _, payload := range sink.states() {
		var s engine.State
		if json.Unmarshal([]byte(payload), &s) == nil && s.Warm {
			found = true
		}
	}
	if !found {
		t.Error("no state event carried warm=true — the hand-off would never fire")
	}
}

// TestWarmGateCapNeverTraps: a network whose checks keep failing still
// gets the gate OPENED — at the cap, never trapped. A late hand-off is
// recoverable (the user taps YouTube); a trapped one is not.
//
// Both ways: stashing the cap case makes warm never flip on a failing
// network and this test times out at the wait.
func TestWarmGateCapNeverTraps(t *testing.T) {
	eng, _, _, net := newWarmEngine(t, t.TempDir(), 100*time.Millisecond, 100*time.Millisecond, 500*time.Millisecond)
	net.acceptAll(false) // every check fails: the engine lands degraded, the re-checks fail

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, eng, 10*time.Second, phaseDegraded)

	st := waitForState(t, eng, 5*time.Second, func(s engine.State) bool { return s.Warm })
	if !st.Warm {
		t.Fatalf("the cap never opened the gate: %+v", st)
	}
	if calls := net.callCount(); calls < 2 {
		t.Errorf("re-checks = %d, want the connect check plus at least one retry", calls)
	}
}

// TestWarmGateResetsAcrossRuns: the run's end (off) re-arms the gate
// closed — the next connect holds, re-checks, and opens again. A
// stale warm from the previous run must never hand the shell an
// already-open gate at second zero (the debug.30 mistake, restated).
//
// Both ways: stashing the setPhaseLocked reset makes the second run
// start warm=true and the hold assertion fails.
func TestWarmGateResetsAcrossRuns(t *testing.T) {
	eng, _, _, _ := newWarmEngine(t, t.TempDir(), 400*time.Millisecond, 100*time.Millisecond, 6*time.Second)

	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, eng, 5*time.Second, phaseOn)
	waitForState(t, eng, 6*time.Second, func(s engine.State) bool { return s.Warm })

	if err := eng.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if st := waitForPhase(t, eng, engine.PhaseOff); st.Warm {
		t.Fatalf("warm=true after off: the gate must re-arm closed between runs")
	}

	// The second connect re-runs the whole gate.
	if err := eng.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	waitForState(t, eng, 5*time.Second, phaseOn)
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if st := eng.GetState(); st.Warm {
			t.Fatalf("warm=true at the second run's second zero: the gate must re-arm (setPhaseLocked reset)")
		}
		time.Sleep(20 * time.Millisecond)
	}
	st := waitForState(t, eng, 6*time.Second, func(s engine.State) bool { return s.Warm })
	if !st.Warm {
		t.Fatalf("the re-armed gate never opened: %+v", st)
	}
}

// TestDeadClusterFileSeedsNextSession: a dead-clusters file in the
// config directory seeds the engine's cluster memory at start — the
// report's cluster line carries the marks live from second zero
// (dead=1 (seeded=1)), the state doc 11 §5.2 assumed and debug.30
// disproved.
//
// Both ways (recorded in ADR-0053's Evidence): stashing the seeding
// hook makes the line read seeded=0 and this test fails.
func TestDeadClusterFileSeedsNextSession(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, config.GoogleVideoDeadClustersFile)
	if err := os.WriteFile(file, []byte("sn-qxau5\n"), 0o600); err != nil {
		t.Fatalf("planting the dead-cluster file: %v", err)
	}

	eng, _, _, _ := newWarmEngine(t, dir, 50*time.Millisecond, 50*time.Millisecond, 2*time.Second)
	if err := eng.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, eng, 5*time.Second, phaseOn)

	// dead= counts marks THIS session measured (none yet); seeded=
	// counts the file's marks live from second zero — the fresh
	// session's honest shape, and exactly what the tester's debug.31
	// report reads right after connect.
	report := eng.GetDiagnosticReportJSON()
	if !strings.Contains(report, "dead=0 (seeded=1)") {
		t.Errorf("the report's cluster line does not show the seeded mark — want \"dead=0 (seeded=1)\":\n%s", report)
	}
	if !strings.Contains(report, "gvideo dead clusters seeded from the persisted file") {
		t.Errorf("the seeding log line is missing from the report:\n%s", report)
	}
}
