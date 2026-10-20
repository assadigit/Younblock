package engine_test

// The M8-ab engine shapes (ADR-0031/0032/0033), pinned by test:
//
//   1. The probe pass's name-list rotation — the debug.19 pass probed
//      rr1..rr5 of the 2 FRESHEST learned codes (early in a session
//      that is the local cluster twice, so the verified pool could not
//      escape the blackhole even in principle). The rotation walks the
//      code store round-robin, one rr name per code with the rr index
//      rotating too: every remembered code — foreign clusters included
//      — is covered within a few passes, and the cursor wraps.
//
//   2. The M8-ab report sections — the substitution line
//      (attempts/wins/pool, the one line that says whether the
//      cross-cache bet worked in the field), the gvideoTerminals
//      counter the churn kick reads, and the origin-probe line's
//      familiar shape all render in a release-shaped report with no
//      hostnames and no IPs (R21).
//
//   3. The persisted good pool — a pre-written
//      googlevideo-good-ips.txt seeds the verified-live table at start
//      (the report's pool counts say so), and the Stop-time flush
//      rewrites the file. The first video of session N+1 rides the
//      origins session N verified.
//
// Both-ways proof: with the engine's M8-ab additions stashed, tests 2
// and 3 FAIL — the substitution line and the pool counts are absent
// from the rendered report entirely — while test 1's pure function
// fails by construction (it does not exist on the stashed side); the
// no-leak guards pass on both sides.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// TestProbePassNamesRotatesTheCodeWindow (M8-ab shape 1): the stable
// names always lead, the code window advances per pass, the rr index
// rotates with the cursor, and the cursor wraps at the store's end.
func TestProbePassNamesRotatesTheCodeWindow(t *testing.T) {
	stable := []string{"a.example", "b.example"}
	codes := []string{"sn-c0", "sn-c1", "sn-c2", "sn-c3", "sn-c4"}

	// Pass 0: window [c0, c1], rr indexes 1 and 2 (the cursor rotates
	// the rr variant too).
	names, next := probePassNamesExported(stable, codes, 0, 2)
	if len(names) != 4 || names[0] != "a.example" || names[1] != "b.example" {
		t.Fatalf("stable names must lead, got %v", names)
	}
	if names[2] != "rr1---sn-c0.googlevideo.com" || names[3] != "rr2---sn-c1.googlevideo.com" {
		t.Fatalf("window 0 names = %v", names)
	}
	if next != 2 {
		t.Fatalf("next cursor = %d, want 2", next)
	}

	// Pass 1: the window advances.
	_, next = probePassNamesExported(stable, codes, next, 2)
	if next != 4 {
		t.Fatalf("next cursor after pass 1 = %d, want 4", next)
	}

	// Pass 2: the window reaches the end and the cursor wraps.
	names, next = probePassNamesExported(stable, codes, next, 2)
	if names[2] != "rr5---sn-c4.googlevideo.com" {
		t.Fatalf("the last code must be covered before the wrap, got %v", names[2:])
	}
	if len(names) != 4 {
		t.Fatalf("wrap window must not duplicate codes, got %v", names)
	}
	if next != 0 {
		t.Fatalf("next cursor after wrap = %d, want 0", next)
	}

	// A window larger than the store covers every code exactly once.
	names, _ = probePassNamesExported(stable, codes, 0, 8)
	if len(names) != len(stable)+len(codes) {
		t.Fatalf("oversized window = %v", names)
	}

	// An empty store keeps the stable names alone, cursor unchanged.
	names, next = probePassNamesExported(stable, nil, 3, 2)
	if len(names) != 2 || next != 3 {
		t.Fatalf("empty store: names=%v next=%d", names, next)
	}
}

// probePassNamesExported reaches the engine package's unexported pure
// function through the one export the tests need (the engine package
// keeps its API surface untouched).
func probePassNamesExported(stable, codes []string, cursor, perPass int) ([]string, int) {
	return engine.ProbePassNamesForTest(stable, codes, cursor, perPass)
}

// TestReportRendersM8abSections (M8-ab shape 2): every new section
// renders in a release-shaped report and none of them leaks a hostname
// or an IP (R21).
func TestReportRendersM8abSections(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	report := eng.GetDiagnosticReport()
	wants := []string{
		"substitution: attempts=",
		"gvideoTerminals",
		"origin probe: passes=",
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

// TestGoodPoolPersistenceRoundTrip (M8-ab shape 3): a pre-written pool
// file seeds the verified-live table at start (the report's pool
// counts say so), and the Stop-time flush keeps the file on disk.
func TestGoodPoolPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	plat := stub.NewWithDir(dir)
	net := &fakeNetwork{}
	net.acceptAll(true)
	poolPath := filepath.Join(dir, config.GoogleVideoGoodIPsFile)
	seed := "192.0.2.10\n192.0.2.11\n"
	if err := os.WriteFile(poolPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
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

	// The seeded pool shows in the report: two gvideo-flagged origins.
	report := eng.GetDiagnosticReport()
	if !strings.Contains(report, "pool=2 (gvideo 2)") {
		t.Fatalf("the seeded pool must render its counts, got: %q", firstLine(report, "substitution:"))
	}

	// Stop flushes the pool back; the seeds survive the round trip.
	if err := eng.Stop(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(poolPath)
	if err != nil {
		t.Fatalf("the pool file must survive Stop: %v", err)
	}
	if !strings.Contains(string(data), "192.0.2.10") {
		t.Fatalf("the seed must round-trip through Stop, got %q", string(data))
	}
}
