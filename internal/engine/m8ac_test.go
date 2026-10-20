package engine_test

// The M8-ac engine shapes (ADR-0034/0035/0036), pinned by test:
//
//   1. The DoH brownout tracker — the dial-churn detector that pauses
//      the engine's background lanes while the connection-killer
//      works the resolver: three dials inside the window enter the
//      brownout, continued dials hold it, a full quiet period lifts
//      it. The debug.20 report's doh line read dials 59 (6 thirteen
//      hours earlier) — the fingerprint this detector reads.
//
//   2. The M8-ac report sections — the stall kill's cause split
//      (pending vs idle — the number that says whether the engine is
//      killing dead origins or the app's pooled connections), the
//      origin-probe line's rehabilitated count (the box was wrong,
//      directly), the good-drop churn on the counters line, and the
//      DoH brownouts on the doh line — all rendering in a
//      release-shaped report with no hostnames and no IPs (R21).
//
// Both-ways proof: with the brownout gate neutered, test 1 FAILS (the
// tracker never enters); with the M8-ac report additions stashed,
// test 2 FAILS on the missing sections while the no-leak guards pass
// on both sides.

import (
        "strings"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
)

// TestDoHBrownoutTracker (M8-ac shape 1): three dials inside the
// window enter the brownout, continued dials hold it, a full quiet
// period lifts it — the background lanes' pause condition.
func TestDoHBrownoutTracker(t *testing.T) {
        tr := engine.NewDoHBrownoutForTest()
        fake := time.Unix(1792500000, 0)
        tr.SetClockForTest(func() time.Time { return fake })

        // Quiet lane: no dials, no brownout.
        if tr.StepForTest(0) || tr.ActiveForTest() {
                t.Fatalf("a quiet lane must never brown out")
        }
        // One dial: below the threshold.
        if tr.StepForTest(1) || tr.ActiveForTest() {
                t.Fatalf("one dial is not a brownout")
        }
        // Two more dials inside the window: the threshold trips.
        fake = fake.Add(2 * time.Second)
        if !tr.StepForTest(3) {
                t.Fatalf("three dials inside the window must ENTER the brownout")
        }
        if !tr.ActiveForTest() {
                t.Fatalf("the brownout must be active")
        }
        // The quiet clock runs while the dials keep coming.
        fake = fake.Add(10 * time.Second)
        if tr.StepForTest(4) {
                t.Fatalf("continued churn re-entering is not a new brownout")
        }
        if !tr.ActiveForTest() {
                t.Fatalf("a fresh dial holds the brownout")
        }
        // A full quiet period without a dial lifts it.
        fake = fake.Add(config.BrownoutQuiet + time.Second)
        tr.StepForTest(4)
        if tr.ActiveForTest() {
                t.Fatalf("a full quiet period must lift the brownout")
        }
        // And it re-arms on the next churn burst.
        fake = fake.Add(time.Second)
        if !tr.StepForTest(7) {
                t.Fatalf("a fresh burst after the lift must ENTER again")
        }
}

// TestReportRendersM8acSections (M8-ac shape 2): every new section
// renders in a release-shaped report and none of them leaks a hostname
// or an IP (R21).
func TestReportRendersM8acSections(t *testing.T) {
        eng, _, _, _ := newTestEngine(t)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForPhase(t, eng, engine.PhaseOn)
        report := eng.GetDiagnosticReport()
        wants := []string{
                // M8-ad (ADR-0037) extended the line with the rich tier's own
                // cause split; the lean half is the M8-ac shape unchanged.
                "stall kills: lean=0 (pending=0 idle=0) rich=0 (pending=0 trickle=0) leanBudget=",
                "rehabilitated=0",
                "goodDrops 0",
                "brownouts=0",
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
