package mobile

// warm_test.go — the facade pin for the warm gate's wire shape (M8-an,
// ADR-0052): the state JSON the Kotlin shell parses on every "state"
// event carries the warm flag — the hand-off's readiness half. The
// Kotlin side reads optBoolean("warm", false) and fires kill+launch
// only on (on|degraded) AND warm AND tunBound, so a missing field must
// never happen on this build's own state events.

import (
	"encoding/json"
	"testing"
)

// TestStateJSONCarriesWarmField: GetState's JSON has the warm key —
// both ways: stash the State.Warm field and this fails (the key
// vanishes and the shell's optBoolean default would hold the gate
// closed forever on a warm-less host).
func TestStateJSONCarriesWarmField(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(m.GetState()), &parsed); err != nil {
		t.Fatalf("GetState() = %s (not JSON): %v", m.GetState(), err)
	}
	v, ok := parsed["warm"]
	if !ok {
		t.Fatalf("GetState() = %s: the warm field is missing — the Kotlin hand-off gate keys on it", m.GetState())
	}
	if _, isBool := v.(bool); !isBool {
		t.Fatalf("warm = %T (%v), want a bool", v, v)
	}
	// At rest (phase off) warm must read false — the gate is closed.
	if parsed["warm"] != false {
		t.Errorf("warm = %v at rest, want false (the gate re-arms closed between runs)", parsed["warm"])
	}
}
