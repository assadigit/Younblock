// view_test.go — the v3 screen mapping, tested on any host: the four
// data-states, the T-table copy, the sub-line rules (warming, slow,
// the typed error, the last-session line), the live figures' visibility
// and the lever's action per state.
package main

import (
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/usage"
)

func int64p(v int64) *int64 { return &v }

func TestScreenIdle(t *testing.T) {
	sc := screen(engine.State{Phase: engine.PhaseOff})
	if sc.State != ScreenIdle {
		t.Fatalf("off state = %s, want idle", sc.State)
	}
	if sc.Headline != "Not connected" || sc.CapLabel != "Start" {
		t.Errorf("idle copy = %+v", sc)
	}
	if sc.Hint != "Tap or swipe up to connect" {
		t.Errorf("idle hint = %q", sc.Hint)
	}
	if sc.Action != LeverConnect {
		t.Errorf("idle action = %d, want connect", sc.Action)
	}
	if sc.ShowTimer || sc.ShowStats || sc.ShowCard {
		t.Errorf("idle shows live figures: %+v", sc)
	}
	if sc.Sub != "" {
		t.Errorf("idle sub = %q, want empty (no session)", sc.Sub)
	}
}

func TestScreenIdleLastSessionLine(t *testing.T) {
	last := &usage.SessionRecord{
		StartMs:   time.Now().Add(-22 * time.Minute).UnixMilli(),
		EndMs:     time.Now().UnixMilli(),
		DownBytes: 15_000_000,
		UpBytes:   800_000,
	}
	sc := screen(engine.State{Phase: engine.PhaseOff, LastSession: last})
	want := "Last session: 22 min, 15.8 MB"
	if sc.Sub != want {
		t.Fatalf("idle sub = %q, want %q", sc.Sub, want)
	}
	// A session under a minute never gets the line.
	tiny := &usage.SessionRecord{StartMs: 1000, EndMs: 30_000, DownBytes: 5, UpBytes: 5}
	if sc := screen(engine.State{Phase: engine.PhaseOff, LastSession: tiny}); sc.Sub != "" {
		t.Errorf("short-session sub = %q, want empty", sc.Sub)
	}
}

func TestScreenConnectingAndWarming(t *testing.T) {
	sc := screen(engine.State{Phase: engine.PhaseConnecting})
	if sc.State != ScreenConnecting || sc.CapLabel != "Wait" {
		t.Fatalf("connecting = %+v", sc)
	}
	if sc.Sub != "Setting things up" {
		t.Errorf("connecting sub = %q", sc.Sub)
	}
	if sc.Action != LeverNone {
		t.Errorf("connecting action = %d, want none (the lever is inert)", sc.Action)
	}

	// M8-an: the run is up but the warm gate holds — the screen
	// keeps the connecting look with the warming words.
	warm := screen(engine.State{Phase: engine.PhaseOn, Warm: false})
	if warm.State != ScreenConnecting {
		t.Fatalf("warming state = %s, want connecting", warm.State)
	}
	if warm.Sub != WarmingDetail {
		t.Errorf("warming sub = %q, want %q", warm.Sub, WarmingDetail)
	}
	if warm.ShowCard || warm.ShowTimer {
		t.Errorf("warming shows the connected figures: %+v", warm)
	}
}

func TestScreenConnected(t *testing.T) {
	since := time.Now().Add(-time.Minute)
	sc := screen(engine.State{
		Phase:       engine.PhaseOn,
		Warm:        true,
		Since:       since,
		DownBytes:   15_700_000,
		UpBytes:     828_000,
		LastSession: nil,
	})
	if sc.State != ScreenConnected {
		t.Fatalf("on state = %s, want connected", sc.State)
	}
	if sc.Headline != "Connected" || sc.CapLabel != "Stop" {
		t.Errorf("connected copy = %+v", sc)
	}
	if sc.Sub != "YouTube unblocked." {
		t.Errorf("connected sub = %q", sc.Sub)
	}
	if sc.Action != LeverDisconnect {
		t.Errorf("connected action = %d, want disconnect", sc.Action)
	}
	if !sc.ShowTimer || !sc.ShowStats || !sc.ShowCard {
		t.Errorf("connected hides its figures: %+v", sc)
	}
	if !sc.ConnectedAt.Equal(since) {
		t.Errorf("connectedAt = %v, want the engine's since", sc.ConnectedAt)
	}
	// The byte totals ride along for the stats row.
	if sc.DownBytes != 15_700_000 || sc.UpBytes != 828_000 {
		t.Errorf("connected bytes = %d/%d", sc.DownBytes, sc.UpBytes)
	}
}

func TestScreenSlowMode(t *testing.T) {
	sc := screen(engine.State{Phase: engine.PhaseDegraded, Warm: true})
	if sc.State != ScreenConnected || !sc.Slow {
		t.Fatalf("degraded = %+v, want the connected slow variant", sc)
	}
	if sc.Sub != "Slow connection, tuning..." || !sc.SubAmber {
		t.Errorf("slow sub = %q amber=%v", sc.Sub, sc.SubAmber)
	}
}

func TestScreenFailed(t *testing.T) {
	sc := screen(engine.State{Phase: engine.PhaseError})
	if sc.State != ScreenFailed || sc.CapLabel != "Retry" {
		t.Fatalf("error = %+v", sc)
	}
	if sc.Sub != "Could not start. Try again." {
		t.Errorf("failed sub = %q", sc.Sub)
	}
	if !sc.SubAmber {
		t.Errorf("failed sub not amber")
	}
	if sc.Action != LeverRetry {
		t.Errorf("failed action = %d, want retry", sc.Action)
	}
	// A typed code rides along, honestly.
	sc = screen(engine.State{Phase: engine.PhaseError, Error: &engine.StateError{Code: "NO_TARGET_APPS"}})
	if sc.Sub != "Could not start. Try again. (NO_TARGET_APPS)" {
		t.Errorf("typed failed sub = %q", sc.Sub)
	}
}

func TestScreenStoppingAndUnknown(t *testing.T) {
	// The winding-down phase maps to the resting idle look.
	if sc := screen(engine.State{Phase: engine.PhaseStopping}); sc.State != ScreenIdle {
		t.Errorf("stopping state = %s, want idle", sc.State)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.00 KB"},
		{12_300_000, "12.3 MB"},
		{123_000_000, "123 MB"},
		{2_100_000_000, "2.10 GB"},
		{-1, "0 B"},
	}
	for _, c := range cases {
		if got := formatBytes(c.in); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(0); got != "00:00:00" {
		t.Errorf("duration(0) = %q", got)
	}
	if got := formatDuration(1000 * (3*3600 + 21*60 + 13)); got != "03:21:13" {
		t.Errorf("duration = %q, want 03:21:13", got)
	}
	if got := formatDuration(-5); got != "00:00:00" {
		t.Errorf("negative duration = %q", got)
	}
}

func TestFormatDurationWords(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{45_000, "45 s"},
		{22 * 60_000, "22 min"},
		{61 * 60_000, "1 h 1 min"},
		{130 * 60_000, "2 h 10 min"},
	}
	for _, c := range cases {
		if got := formatDurationWords(c.ms); got != c.want {
			t.Errorf("words(%d) = %q, want %q", c.ms, got, c.want)
		}
	}
}

func TestScreenStateString(t *testing.T) {
	if ScreenIdle.String() != "idle" || ScreenFailed.String() != "failed" {
		t.Errorf("state names broken")
	}
}

// The screen's diff driver: only the fields the painter renders.
func TestScreenChanged(t *testing.T) {
	a := screen(engine.State{Phase: engine.PhaseOff})
	b := screen(engine.State{Phase: engine.PhaseOff})
	if screenChanged(a, b) {
		t.Fatalf("identical screens report changed")
	}
	b.DownBytes = 5
	if !screenChanged(a, b) {
		t.Fatalf("a byte tick must repaint the stats row")
	}
	b = screen(engine.State{Phase: engine.PhaseOn, Warm: true})
	if !screenChanged(a, b) {
		t.Fatalf("a state change must repaint")
	}
}
