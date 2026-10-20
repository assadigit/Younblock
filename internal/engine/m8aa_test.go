package engine_test

// The M8-aa engine shapes (ADR-0029/0030), pinned by test:
//
//   1. The Android lean stall budget — an android-named platform's
//      listener builds the proxy with the 8 s lean budget and the
//      report says so (leanBudget=8s); the stub (desktop-shaped)
//      platform keeps the 15 s config default byte-for-byte. The
//      debug.18 report's three mid-response corpses each held the
//      app's retry for the whole 15 s; the next paste must show three
//      8 s ones.
//
//   2. The auto-timeline heuristic — the debug.18 report came back
//      with "timeline: no marks" (the tester never pressed the
//      buttons). The poller pairs a routed burst after quiet with the
//      down-burst that answers it, stamps both into the log ring
//      (AUTO-MARK lines), renders the pair in the report, and resets
//      on Stop like every other run-scoped diagnostic (R21).
//
//   3. The M8-aa report sections — the tier split with the budget that
//      fired, the origin-probe counters with the box's and the good
//      table's counts, and the new counters line all render in a
//      release-shaped report (no hostnames, no IPs).
//
// Both-ways proof: with the engine's M8-aa additions stashed, tests 1
// and 2 FAIL — the leanBudget=/auto-timeline= sections are absent from
// the rendered report entirely (the section-render both-ways of
// ADR-0027's own report test) — while test 3's no-leak guards pass on
// both sides.

import (
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// namedPlatform wraps the stub with a controllable Name (the lean
// budget's platform gate reads exactly this).
type namedPlatform struct {
	*stub.Platform
	name string
}

func (p *namedPlatform) Name() string { return p.name }

// newNamedEngine builds a started engine on a platform that reports
// the given name.
func newNamedEngine(t *testing.T, name string) *engine.Engine {
	t.Helper()
	dir := t.TempDir()
	plat := &namedPlatform{Platform: stub.NewWithDir(dir), name: name}
	net := &fakeNetwork{}
	net.acceptAll(true)
	eng, err := engine.New(engine.Options{
		Platform: plat,
		LogDir:   dir + "/logs",
		Checker:  net,
	})
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	t.Cleanup(eng.Shutdown)
	return eng
}

// TestStallLeanBudgetAndroid (M8-aa shape 1): android builds the proxy
// with the 8 s lean budget and the report carries it; a desktop-shaped
// platform keeps the 15 s default.
func TestStallLeanBudgetAndroid(t *testing.T) {
	android := newNamedEngine(t, "android")
	if got := android.GetDiagnosticReport(); !strings.Contains(got, "leanBudget=8s") {
		t.Fatalf("android report must carry leanBudget=8s")
	}

	desktop := newNamedEngine(t, "stub")
	if got := desktop.GetDiagnosticReport(); !strings.Contains(got, "leanBudget=15s") {
		t.Fatalf("desktop-shaped report must keep leanBudget=15s (byte-for-byte old behavior)")
	}
}

// TestAutoTimelinePairsPlayWithData (M8-aa shape 2): a routed burst
// after quiet stamps play-suspected, the down-burst that follows pairs
// it with first-data, both stamps ride the ring, the report renders the
// pair — and Stop resets the heuristic like every run-scoped surface.
func TestAutoTimelinePairsPlayWithData(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)

	// Let one poll tick establish the baseline, then the burst.
	time.Sleep(600 * time.Millisecond)
	eng.Stats().RoutedFlowStart()
	time.Sleep(600 * time.Millisecond) // the tick that stamps play-suspected
	eng.Stats().AddBytesDown(150 * 1024)
	time.Sleep(600 * time.Millisecond) // the tick that pairs first-data

	report := eng.GetDiagnosticReport()
	if !strings.Contains(report, "auto-timeline: play-suspected -> first-data elapsed=") {
		t.Fatalf("auto-timeline pair missing from the report")
	}
	if !strings.Contains(report, "pairs=1") {
		t.Fatalf("auto-timeline pair count missing")
	}
	if !strings.Contains(report, "AUTO-MARK play-suspected") || !strings.Contains(report, "AUTO-MARK first-data") {
		t.Fatalf("the AUTO-MARK stamps must ride the log ring")
	}

	// R21 hygiene: the heuristic is run-scoped — Stop clears it.
	if err := eng.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := eng.GetDiagnosticReport(); !strings.Contains(got, "auto-timeline: no pair yet") {
		t.Fatalf("auto-timeline must reset on Stop, got: %q", firstLine(got, "auto-timeline"))
	}
}

// TestReportRendersM8aaSections (M8-aa shape 3): every new section
// renders in a release-shaped report and none of them leaks a hostname
// or an IP (R21).
func TestReportRendersM8aaSections(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	report := eng.GetDiagnosticReport()
	wants := []string{
		"counters: stallKillsLean",
		"racedSaves",
		"salvagedSaves",
		"routedFlows",
		"stall kills: lean=",
		"origin probe: passes=",
		"auto-timeline:",
	}
	for _, want := range wants {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	for _, leak := range []string{"youtube.com", "googlevideo", "127.0.0.", "192.0.2."} {
		if strings.Contains(report, leak) {
			t.Fatalf("release-shaped report leaks %q (R21)", leak)
		}
	}
}

// firstLine returns the first report line containing substr (failure
// messages only).
func firstLine(report, substr string) string {
	for _, l := range strings.Split(report, "\n") {
		if strings.Contains(l, substr) {
			return l
		}
	}
	return ""
}
