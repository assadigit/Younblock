// view_test.go — the lite screen's phase mapping, table-driven like the
// engine's own tests (run on every host; the Windows painter consumes
// exactly what these tests pin down).
package main

import (
	"testing"

	"github.com/assadigit/Younblock/internal/engine"
)

func TestViewOff(t *testing.T) {
	v := view(engine.State{Phase: engine.PhaseOff})
	if v.Headline != "Off" || v.ButtonLabel != "Connect" || !v.ButtonEnabled {
		t.Fatalf("off view = %+v", v)
	}
	if v.Dot != DotGray {
		t.Errorf("off dot = %v, want gray", v.Dot)
	}
	if v.Footer != "" {
		t.Errorf("off footer = %q, want empty", v.Footer)
	}
}

func TestViewConnecting(t *testing.T) {
	v := view(engine.State{Phase: engine.PhaseConnecting})
	if v.Headline != "Connecting" || v.ButtonLabel != "Cancel" || !v.ButtonEnabled {
		t.Fatalf("connecting view = %+v", v)
	}
	if v.Dot != DotAmber {
		t.Errorf("connecting dot = %v, want amber", v.Dot)
	}
}

func TestViewOnWarming(t *testing.T) {
	// M8-an: on but not warm — the established copy, amber dot.
	v := view(engine.State{Phase: engine.PhaseOn, Warm: false, Port: 4525})
	if v.Headline != "Connected" || v.Detail != "Getting the first video ready…" {
		t.Fatalf("warming view = %+v", v)
	}
	if v.Dot != DotAmber {
		t.Errorf("warming dot = %v, want amber", v.Dot)
	}
	if v.ButtonLabel != "Disconnect" {
		t.Errorf("warming button = %q", v.ButtonLabel)
	}
	if v.Footer != "127.0.0.1:4525" {
		t.Errorf("on footer = %q", v.Footer)
	}
}

func TestViewOnWarm(t *testing.T) {
	lat := int64(230)
	v := view(engine.State{Phase: engine.PhaseOn, Warm: true, Port: 4525, LatencyMs: &lat})
	if v.Dot != DotRed {
		t.Errorf("warm dot = %v, want red (the brand's on)", v.Dot)
	}
	want := "Videos should start fast.  ·  230 ms ping"
	if v.Detail != want {
		t.Errorf("warm detail = %q, want %q", v.Detail, want)
	}
}

func TestViewDegraded(t *testing.T) {
	v := view(engine.State{Phase: engine.PhaseDegraded, Port: 4525})
	if v.Headline != "Slow mode" || v.Dot != DotAmber {
		t.Fatalf("degraded view = %+v", v)
	}
	if v.Footer != "127.0.0.1:4525" {
		t.Errorf("degraded footer = %q", v.Footer)
	}
	// Latency does not ride the degraded line (it is being re-checked).
	lat := int64(99)
	v = view(engine.State{Phase: engine.PhaseDegraded, Warm: true, LatencyMs: &lat, Port: 4525})
	if v.Detail == "Videos should start fast.  ·  99 ms ping" {
		t.Errorf("degraded detail borrowed the warm copy: %q", v.Detail)
	}
}

func TestViewError(t *testing.T) {
	v := view(engine.State{Phase: engine.PhaseError, Error: &engine.StateError{Code: "start-timeout"}})
	if v.Headline != "Couldn't connect" || v.ButtonLabel != "Retry" || !v.ButtonEnabled {
		t.Fatalf("error view = %+v", v)
	}
	if v.Detail != "The tunnel could not start. (start-timeout)" {
		t.Errorf("error detail = %q", v.Detail)
	}
	if v.Footer != "" {
		t.Errorf("error footer = %q, want empty", v.Footer)
	}
}

func TestViewStopping(t *testing.T) {
	v := view(engine.State{Phase: engine.PhaseStopping})
	if v.ButtonEnabled {
		t.Fatalf("stopping view = %+v, button must be disabled", v)
	}
}

func TestViewUnknownPhase(t *testing.T) {
	// A future engine phase: honest word, no invented copy, no action.
	v := view(engine.State{Phase: engine.Phase("hibernating")})
	if v.Headline != "hibernating" || v.ButtonEnabled {
		t.Fatalf("unknown view = %+v", v)
	}
}

func TestPrimaryAction(t *testing.T) {
	cases := []struct {
		phase engine.Phase
		want  string
	}{
		{engine.PhaseOff, "start"},
		{engine.PhaseError, "start"},
		{engine.PhaseConnecting, "stop"},
		{engine.PhaseOn, "stop"},
		{engine.PhaseDegraded, "stop"},
		{engine.PhaseStopping, "start"}, // disabled button anyway
	}
	for _, c := range cases {
		if got := primaryAction(engine.State{Phase: c.phase}); got != c.want {
			t.Errorf("primaryAction(%s) = %q, want %q", c.phase, got, c.want)
		}
	}
}
