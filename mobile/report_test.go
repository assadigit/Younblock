package mobile

// report_test.go — the M8-p diagnostics pins (ADR-0015): the facade
// installs its section on the engine's diagnostic report, so the report
// the WebView's Diagnostics screen copies carries the evidence the
// engine alone cannot see. Two layers:
//
//   - the light path (no TUN, no serving engine): "tun: inactive" is
//     always present (the report must say WHICH state the TUN is in,
//     never silently omit it), and the shell version line appears only
//     after Kotlin's SetShellVersion call — the exact build identity a
//     pasted report needs, since versionName stays 1.6.6 across debug
//     rounds.
//   - the live path (the serving harness + a real socketpair TUN):
//     "tun: active" with the counter names — the line the per-video
//     stall diagnosis reads (flows stuck at zero = the flow never
//     reached the door; udpDropped climbing = the QUIC suspect).

import (
	"strings"
	"testing"
)

func TestReportSectionWithoutTun(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	// Before Kotlin's call: no shell line, but the TUN line is present
	// and honest.
	report := m.GetDiagnosticReport()
	if !strings.Contains(report, "\"report\":") {
		t.Fatalf("facade report is not the JSON wrapper:\n%s", report)
	}
	body := m.eng.GetDiagnosticReport()
	if !strings.Contains(body, "tun: inactive") {
		t.Errorf("report missing 'tun: inactive' before SetShellVersion:\n%s", body)
	}
	if strings.Contains(body, "android shell:") {
		t.Errorf("report carries a shell line before SetShellVersion:\n%s", body)
	}

	// After Kotlin's call: the identity line rides in every report.
	m.SetShellVersion("1.6.6 (versionCode 8)")
	body = m.eng.GetDiagnosticReport()
	if !strings.Contains(body, "android shell: 1.6.6 (versionCode 8)\n") {
		t.Errorf("report missing the shell line after SetShellVersion:\n%s", body)
	}
	if !strings.Contains(body, "tun: inactive") {
		t.Errorf("report lost 'tun: inactive' after SetShellVersion:\n%s", body)
	}

	// The section carries no addresses (the R21 redaction extends to it).
	if strings.Contains(body, "127.0.0.1") {
		t.Errorf("report section leaks an address:\n%s", body)
	}
}

func TestReportSectionWithLiveTun(t *testing.T) {
	m, protector := startServingEngine(t)
	_, fd := tunPair(t)

	if got := m.StartTun(fd, 1500); !strings.Contains(got, `"error":null`) {
		t.Fatalf("StartTun = %s", got)
	}
	t.Cleanup(func() { m.StopTun() })

	m.SetShellVersion("1.6.6 (versionCode 8)")
	body := m.eng.GetDiagnosticReport()
	for _, want := range []string{
		"android shell: 1.6.6 (versionCode 8)",
		"tun: active ",
		"flows=", "rstUnmapped=", "overCap=",
		"doorFailures=", "dnsQueries=", "udpDropped=",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "tun: inactive") {
		t.Errorf("report says inactive with a live TUN:\n%s", body)
	}
	// The protector was wired through the whole harness; sanity only.
	if protector.count() < 2 {
		t.Errorf("protect called %d times, want >= 2", protector.count())
	}
}
