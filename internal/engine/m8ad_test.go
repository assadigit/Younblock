package engine_test

// The M8-ad engine shapes (ADR-0037/0038/0039), pinned by test:
//
//   1. Auto STARTS at legacy — the old Python app's 299-cut rule is
//      the shape the tester's own network proved for years (and the
//      one he pointed back at: "read the backend script again"). The
//      regime the debug.20/21 reports measured — resets on
//      recognizably-shaped hellos — punishes weak splitting, and
//      balanced (the previous auto start, ~4-6 pieces of 80-200 bytes)
//      was never in the fight. Both-ways: with presetFor reverted to
//      balanced, this test FAILS.
//
//   2. The heal sweep walks DOWN the strength ladder — legacy first
//      (config.HealSweepOrder), so a failed start check tries the
//      strongest shape before conceding to weaker ones. Both-ways:
//      with the old order restored (legacy last), this test FAILS.
//
//   3. The M8-ad report sections — the rich tier's cause split on the
//      stall line (pending = the mid-video wedge's recovery bound,
//      trickle = the per-connection throttle the both-silent clock
//      could never see) renders in a release-shaped report with no
//      hostnames and no IPs (R21).

import (
	"strings"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/fragment"
)

// TestAutoStartsAtLegacy (M8-ad shape 1): auto's preset is the legacy
// rule — 299 random cuts at 1 ms, the shape the old Python backend
// proved on this network for years.
func TestAutoStartsAtLegacy(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if got := eng.PresetForTest(config.MethodAuto); got != fragment.Legacy {
		t.Fatalf("auto's start preset = %q, want %q (the Python rule — the strongest shape, first)", got, fragment.Legacy)
	}
	// The explicit methods keep their direct mapping.
	if got := eng.PresetForTest(config.MethodStrong); got != fragment.Strong {
		t.Fatalf("strong's preset = %q, want strong", got)
	}
	if got := eng.PresetForTest(config.MethodLegacy); got != fragment.Legacy {
		t.Fatalf("legacy's preset = %q, want legacy", got)
	}
}

// TestHealSweepLeadsWithLegacy (M8-ad shape 2): the sweep's first
// attempt is the strongest shape.
func TestHealSweepLeadsWithLegacy(t *testing.T) {
	order := config.HealSweepOrder
	if len(order) != 4 {
		t.Fatalf("the sweep must keep four methods, got %d", len(order))
	}
	if order[0] != config.MethodLegacy {
		t.Fatalf("the sweep must LEAD with legacy (walk DOWN the strength ladder), got %q", order[0])
	}
	seen := map[config.Method]bool{}
	for _, m := range order {
		seen[m] = true
	}
	for _, m := range []config.Method{config.MethodLegacy, config.MethodStrong, config.MethodBalanced, config.MethodLight} {
		if !seen[m] {
			t.Fatalf("the sweep must keep %q reachable", m)
		}
	}
}

// TestReportRendersM8adSections (M8-ad shape 3): the stall line
// carries the rich tier's cause split, in a release-shaped report.
func TestReportRendersM8adSections(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	report := eng.GetDiagnosticReport()
	if !strings.Contains(report, "rich=0 (pending=0 trickle=0)") {
		t.Fatalf("report missing the rich tier's cause split: %q", report)
	}
	for _, leak := range []string{"youtube.com", "googlevideo", "127.0.0.", "192.0.2."} {
		if strings.Contains(report, leak) {
			t.Fatalf("release-shaped report leaks %q (R21)", leak)
		}
	}
}
