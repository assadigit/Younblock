package engine_test

// The M8-af engine shapes (ADR-0043/0044/0045, spec W1-W9), pinned by
// test:
//
//  1. The report's new sections render in a release-shaped report
//     with no hostnames and no IPs (R21): the experiment line
//     (method/substitution/breaker), the tcp verdicts split, the
//     fragPackets counter on the discipline line, and the network
//     test's honest not-run line.
//
//  2. The network test lane's gating: NOT_ON while the engine is off,
//     the status JSON shape (running/done), and the not-run report
//     line flipping to the running line while one is in flight.
//
// Both-ways proof: with the report sections stashed, test 1 FAILS on
// every missing line; with the phase gate stashed, test 2 FAILS
// (NOT_ON never returned — the lane would dial with no protect door).

import (
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
)

// TestReportRendersM8afSections (shape 1): every debug.24 section is
// present in a rendered sample — the definition-of-done report test
// for the round.
func TestReportRendersM8afSections(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	report := eng.GetDiagnosticReport()
	wants := []string{
		// W4: the experiment line — the three switches the sitting
		// rode on, printed where the report starts.
		"experiment: method=auto substitution=on breaker=on",
		// W1: the kernel-TCP verdict split.
		"tcp verdicts: upstream-lost=0 downstream-lost=0 both-ok=0 unknown=0",
		// W9: the fragmenter's packet total on the discipline line
		// (M8-ag: the lifts=/scouts= escapes ride it too).
		"gvideo discipline: hellos=0 cooldowns=0 lifts=0 scouts=0 fragPackets=",
		// W7: the network test's honest not-run line.
		"network test: not run this session (press Run network test in Diagnostics)",
	}
	for _, want := range wants {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	for _, leak := range []string{"youtube.com", "googlevideo", "127.0.0.", "192.0.2."} {
		if strings.Contains(report, leak) {
			t.Errorf("release-shaped report leaks %q (R21)", leak)
		}
	}
	eng.Stop()
}

// TestNetworkTestGatingAndStatus (shape 2): the lane refuses to run
// while the engine is off (it dials through the engine's own door —
// protect, DoH, fragment — so it needs the run), and the status JSON
// carries the honest shape.
func TestNetworkTestGatingAndStatus(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if got := eng.RunNetworkTestJSON(); got != `{"error":"NOT_ON"}` {
		t.Fatalf("RunNetworkTest while off = %s, want NOT_ON", got)
	}
	status := eng.NetworkTestStatusJSON()
	if !strings.Contains(status, `"running":false`) || !strings.Contains(status, `"done":false`) {
		t.Fatalf("status while never run = %s, want running:false done:false", status)
	}
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	// The report's line while a run is in flight is honest too (the
	// lane is async; the engine test cannot dial real addresses, so
	// the run itself completes with the no-addresses line — the gating
	// and the status shape are what this test pins).
	if got := eng.RunNetworkTestJSON(); got != `{"error":null}` {
		t.Fatalf("RunNetworkTest while on = %s, want started", got)
	}
	if got := eng.RunNetworkTestJSON(); got != `{"error":"RUNNING"}` {
		t.Fatalf("RunNetworkTest while running = %s, want RUNNING", got)
	}
	// The async run finishes (the stub resolver has no googlevideo
	// codes; the redirector lookup fails; the honest no-addresses
	// line lands) — poll for the done flip.
	deadline := waitForNetworkTest(t, eng)
	if !deadline {
		t.Fatal("the network test never finished")
	}
	status = eng.NetworkTestStatusJSON()
	if !strings.Contains(status, `"done":true`) || !strings.Contains(status, `"lines":[`) {
		t.Fatalf("status after the run = %s, want done:true with lines", status)
	}
	report := eng.GetDiagnosticReport()
	if !strings.Contains(report, "network test:") {
		t.Fatalf("the report must carry the network test section:\n%s", report)
	}
	eng.Stop()
}

func waitForNetworkTest(t *testing.T, eng *engine.Engine) bool {
	t.Helper()
	for i := 0; i < 100; i++ {
		if strings.Contains(eng.NetworkTestStatusJSON(), `"done":true`) {
			return true
		}
		sleepMs(50)
	}
	return false
}

func sleepMs(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
