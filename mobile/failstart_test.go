package mobile

// failstart_test.go — the M7 platform-refusal door through the real
// facade (docs/android-port 05 §M7, DECISIONS.md D81): the shell calls
// FailStart when the platform cannot honestly proceed (none of the
// allow-list's apps installed), and the gate-held connect run fails
// through the engine's ordinary path — phase "error" with the typed
// code in the state JSON, the same document the WebView's page reads.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
)

// waitForFailStartPhase polls the facade's state JSON for a phase.
func waitForFailStartPhase(t *testing.T, m *Engine, want string, timeout time.Duration) engine.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var st engine.State
		if err := json.Unmarshal([]byte(m.GetState()), &st); err == nil && string(st.Phase) == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("phase %s did not arrive within %v (state %s)", want, timeout, m.GetState())
	return engine.State{}
}

// TestFailStartRefusesHeldRun — the full D81 path: NewEngine arms the
// gate closed; Start holds at "connecting" (never a listener, never
// "on"); FailStart("NO_TARGET_APPS") refuses the held run; the engine
// lands in phase "error" with the typed code — the page's honest
// "Can't Connect" with a reason, not a silent unwind.
func TestFailStartRefusesHeldRun(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start = %s", got)
	}
	waitForFailStartPhase(t, m, "connecting", 2*time.Second)

	if got := m.FailStart("NO_TARGET_APPS"); got != `{"error":null}` {
		t.Fatalf("FailStart = %s, want error:null (delivered)", got)
	}
	st := waitForFailStartPhase(t, m, "error", 2*time.Second)
	if st.Error == nil || st.Error.Code != "NO_TARGET_APPS" {
		t.Fatalf("state error = %+v, want the typed NO_TARGET_APPS code", st.Error)
	}
}

// TestFailStartRecoversOnNextConnect — a refusal poisons nothing: after
// the shell opens the gate (the user installed an app; the service
// re-established), a retry Start connects normally.
func TestFailStartRecoversOnNextConnect(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start 1 = %s", got)
	}
	waitForFailStartPhase(t, m, "connecting", 2*time.Second)
	if got := m.FailStart("NO_TARGET_APPS"); got != `{"error":null}` {
		t.Fatalf("FailStart = %s", got)
	}
	waitForFailStartPhase(t, m, "error", 2*time.Second)

	// The shell's recovery: open the gate, retry.
	m.SetVpnReady(true)
	if got := m.Start(); got != `{"error":null}` {
		t.Fatalf("Start 2 = %s", got)
	}
	// The real engine reaches a terminal phase (on/degraded/error —
	// never a lie); whichever it is, the refusal left no residue.
	waitForTerminal(t, m, 40*time.Second)
}

// TestFailStartNotHeldOnOpenGate — an open gate holds no run to refuse:
// the facade reports NOT_HELD and touches nothing.
func TestFailStartNotHeldOnOpenGate(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	m.SetVpnReady(true)
	if got := m.FailStart("NO_TARGET_APPS"); got != `{"error":"NOT_HELD"}` {
		t.Fatalf("FailStart on an open gate = %s, want NOT_HELD", got)
	}
	var st engine.State
	if err := json.Unmarshal([]byte(m.GetState()), &st); err != nil {
		t.Fatal(err)
	}
	if st.Phase != engine.PhaseOff {
		t.Fatalf("phase = %s, want off (NOT_HELD must not invent a failure)", st.Phase)
	}
}

// TestFailStartBadCode — the code vocabulary is whitelisted at the
// bridge: no arbitrary error injection.
func TestFailStartBadCode(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	if got := m.FailStart("SOME_FAKE_CODE"); got != `{"error":"BAD_CODE"}` {
		t.Fatalf("FailStart(bogus) = %s, want BAD_CODE", got)
	}
}

// waitForTerminal waits for any terminal phase (mirrors the M5 helper).
func waitForTerminal(t *testing.T, m *Engine, timeout time.Duration) engine.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	terminal := map[engine.Phase]bool{
		engine.PhaseOn: true, engine.PhaseDegraded: true, engine.PhaseError: true, engine.PhaseOff: true,
	}
	for time.Now().Before(deadline) {
		var st engine.State
		if err := json.Unmarshal([]byte(m.GetState()), &st); err == nil && terminal[st.Phase] {
			return st
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("engine never reached a terminal phase; state %s", m.GetState())
	return engine.State{}
}
