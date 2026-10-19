package engine_test

// The M8-z report shapes (ADR-0027 §8.3/§8.4), pinned by test: every
// new section renders — the counters line, the DoH counters, the Top-10
// slowest-or-failed hosts (redacted unless hostnames are logged), the
// plain-relay summary, the DNS detail, the dead-origin probe's honest
// skip line, and the timeline marks with the last pair's elapsed time
// and window counts. The debug-build notice prints ONLY in a
// DebugTesterBuild with hostnames on; a release-shaped report never
// carries it and never carries a host name.
//
// Both-ways proof: with the section renders stashed, the assertions on
// the corresponding sections FAIL; the redaction guards pass on both
// sides.

import (
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
)

// TestReportRendersEveryNewSection (the definition-of-done report
// test): every §8.3/§8.4 section is present in a rendered sample.
func TestReportRendersEveryNewSection(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)

	// The tester flow: two marks a moment apart.
	eng.Mark("I pressed play")
	time.Sleep(40 * time.Millisecond)
	eng.Mark("First frame")

	report := eng.GetDiagnosticReport()
	wants := []string{
		// §8.4 counters.
		"counters: plainFirstByteTimeouts",
		"dialNear",
		"slowFirstBytes",
		// §8.3.3 DoH counters.
		"doh: dials",
		"hedgeWins",
		// §8.3.1 the Top-10 hosts (redacted in this run: logDomains off).
		"top 10 slowest or failed hosts:",
		// §8.3.2 the plain-relay summary.
		"plain-relay: flows=",
		// §8.3.3 the DNS detail.
		"dns detail:",
		// §8.3.5 the dead-origin probe's honest skip (no penalized IPs
		// in this run, and it must say so, not stay silent).
		"dead-origin probe:",
		// §8.3.6 the timeline pair with its window.
		"timeline: MARK I pressed play -> MARK First frame elapsed=",
		"failedFlows=",
		// §8.1 a release-shaped report NEVER carries the debug notice.
		"fragmentAllRouted=false",
	}
	for _, want := range wants {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "DEBUG BUILD") {
		t.Errorf("a non-debug run must not carry the DEBUG BUILD notice:\n%s", report)
	}
	if strings.Contains(report, "host=youtube.com") {
		t.Errorf("a logDomains-off report must not carry host names:\n%s", report)
	}
	// The marks also land in the log ring (the MARK lines the tester
	// sees in the report's tail).
	ringLines := strings.Join(eng.Logger().Ring.Lines(), "\n")
	if !strings.Contains(ringLines, "MARK I pressed play") || !strings.Contains(ringLines, "MARK First frame") {
		t.Errorf("the MARK lines must land in the ring:\n%s", ringLines)
	}
	eng.Stop()
}

// TestReportDebugBuildNoticeAndHostnames (§8.1): ONLY a DebugTesterBuild
// with hostnames on starts the report with the visible notice — the
// release build keeps hostnames off even when the section data exists.
func TestReportDebugBuildNoticeAndHostnames(t *testing.T) {
	old := config.DebugTesterBuild
	defer func() { config.DebugTesterBuild = old }()

	// Hostnames on through the settings surface (the debug build's own
	// default path is LoadFor's business, pinned in the config tests).
	// The notice is SETTINGS-driven, not phase-driven — the report
	// renders it without an engine start (TestReportRendersEveryNewSection
	// already covers the On-path rendering).
	config.DebugTesterBuild = "true"
	eng, _, _, _ := newTestEngine(t)
	if got := eng.SaveSettingsJSON(`{"advanced":{"logDomains":true}}`); !strings.Contains(got, `"logDomains":true`) {
		t.Fatalf("SaveSettings = %s", got)
	}
	report := eng.GetDiagnosticReport()
	if !strings.HasPrefix(report, "DEBUG BUILD: hostnames included for diagnosis\n") {
		t.Errorf("the debug report must START with the notice:\n%.200s", report)
	}
	if !strings.Contains(report, "fragmentAllRouted=false") {
		t.Errorf("the mode line must carry fragmentAllRouted: %.200s", report)
	}

	// A release-shaped build with the same settings: hostnames still on
	// (the owner's own choice), but NO debug notice — the notice is the
	// debug lane's, not the setting's.
	config.DebugTesterBuild = ""
	eng2, _, _, _ := newTestEngine(t)
	if got := eng2.SaveSettingsJSON(`{"advanced":{"logDomains":true}}`); !strings.Contains(got, `"logDomains":true`) {
		t.Fatalf("SaveSettings = %s", got)
	}
	report2 := eng2.GetDiagnosticReport()
	if strings.Contains(report2, "DEBUG BUILD") {
		t.Errorf("a release build must never print the debug notice:\n%.200s", report2)
	}
}

// TestMarkWindowCountsFlows (§8.3.6): the marks' window counts the
// failed/slow flows BETWEEN the pair — counters after the second mark
// land in the NEXT window, not this one.
func TestMarkWindowCountsFlows(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)

	eng.Mark("I pressed play")
	// A failure and a slow first-byte inside the window.
	eng.Stats().ConnError()
	eng.Stats().FirstByteSlow()
	eng.Stats().PlainFirstByteTimeout()
	eng.Mark("First frame")
	// After the pair: the next window's business.
	eng.Stats().ConnError()

	report := eng.GetDiagnosticReport()
	line := ""
	for _, l := range strings.Split(report, "\n") {
		if strings.HasPrefix(l, "timeline: MARK I pressed play -> MARK First frame") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("no timeline line in the report:\n%s", report)
	}
	if !strings.Contains(line, "failedFlows=1") || !strings.Contains(line, "plainKills=1") || !strings.Contains(line, "slowFirstBytes=1") {
		t.Fatalf("the window must count exactly the between-marks flows: %q", line)
	}
	eng.Stop()
}

// TestMarksResetOnStop: Stop clears the timeline with the ring (R21
// hygiene — the next run's report starts clean).
func TestMarksResetOnStop(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, eng, engine.PhaseOn)
	eng.Mark("one")
	eng.Stop()
	waitForPhase(t, eng, engine.PhaseOff)
	report := eng.GetDiagnosticReport()
	if !strings.Contains(report, "timeline: no marks") {
		t.Errorf("a fresh run's report must say there are no marks:\n%s", report)
	}
}
