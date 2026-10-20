package engine_test

// The M8-ae engine shapes (ADR-0040/0041/0042), pinned by test:
//
//  1. The wire-discipline report section — the gvideo discipline line
//     (hellos, the volume the censor actually sees, and cooldowns, the
//     breaker's arms) renders in a release-shaped report with no
//     hostnames and no IPs (R21).
//
//  2. The round's knob contract — the probe pass's trickle budget
//     (OriginProbePerPass = 10: the pass is never a burst again), the
//     probe cadence decoupled from the prewarm (GoogleVideoProbeEvery
//     = 15 min), and the emission breaker's threshold/window/cooldown
//     (3 terminals in 30 s -> 45 s of wire-quiet). The M8-ab churn
//     kick's debounce is GONE from the config surface — the lane it
//     paced is deleted.
//
// Both-ways proof: with the M8-ae report line stashed, test 1 FAILS on
// the missing section while the no-leak guards pass on both sides;
// with any knob rebased, test 2 FAILS on the pin.

import (
        "strings"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
)

// TestReportRendersM8aeSections (M8-ae shape 1): the gvideo
// discipline line renders in a release-shaped report and leaks
// nothing (R21).
func TestReportRendersM8aeSections(t *testing.T) {
        eng, _, _, _ := newTestEngine(t)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForPhase(t, eng, engine.PhaseOn)
        report := eng.GetDiagnosticReport()
        wants := []string{
                // M8-ae (ADR-0042): the wire-discipline evidence — the
                // googlevideo-SNI hellos the engine put on the wire this
                // session (read against routedFlows; the reference
                // implementations run ~1 per flow, debug.22 ran ~4 plus
                // bursts) and the emission breaker's arms.
                "gvideo discipline: hellos=0 cooldowns=0",
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

// TestWireDisciplineKnobs (M8-ae shape 2): the round's knob contract,
// pinned — the trickle budget, the decoupled probe cadence, and the
// emission breaker's shape.
func TestWireDisciplineKnobs(t *testing.T) {
        if config.OriginProbePerPass != 10 {
                t.Fatalf("OriginProbePerPass = %d, want 10 (the trickle, never the burst)", config.OriginProbePerPass)
        }
        if config.GoogleVideoProbeEvery != 15*time.Minute {
                t.Fatalf("GoogleVideoProbeEvery = %v, want 15m (decoupled from the 5m DoH prewarm)", config.GoogleVideoProbeEvery)
        }
        if config.GvideoBreakerTerminals != 3 {
                t.Fatalf("GvideoBreakerTerminals = %d, want 3", config.GvideoBreakerTerminals)
        }
        if config.GvideoBreakerWindow != 30*time.Second {
                t.Fatalf("GvideoBreakerWindow = %v, want 30s", config.GvideoBreakerWindow)
        }
        if config.GvideoBreakerCooldown != 45*time.Second {
                t.Fatalf("GvideoBreakerCooldown = %v, want 45s", config.GvideoBreakerCooldown)
        }
}
