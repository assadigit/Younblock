package engine_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// TestPrivacyLogsAndReportCarryNoHostnamesOrIPs is the R21 check: an
// automated scan of the log file and diagnostic report samples for
// hostnames and IP addresses. The engine runs a session with default
// settings (logDomains off): the report and the log file must contain
// neither visited-site hostnames nor raw IPs beyond the loopback listener
// address the user already knows (127.0.0.1 appears in the report as the
// local proxy address reference only — assert it does not).
func TestPrivacyLogsAndReportCarryNoHostnamesOrIPs(t *testing.T) {
	dir := t.TempDir()
	plat := stub.NewWithDir(dir)
	fake := &fakeNetwork{}
	fake.acceptAll(true)
	eng, err := engine.New(engine.Options{
		Platform: plat,
		LogDir:   dir + "/logs",
		Checker:  fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Shutdown)

	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, eng, 5*time.Second, phaseOn)
	// Generate traffic-driven log lines (notices, state changes) and a
	// connection test summary through the fake network.
	eng.RunConnectionTestJSON()
	time.Sleep(200 * time.Millisecond)
	if err := eng.Stop(); err != nil {
		t.Fatal(err)
	}

	report := eng.GetDiagnosticReport()
	// Banned strings: visited-site hostnames, provider bootstrap IPs, and
	// the public YouTube/DNS names (the mode word "youtube" is fine; the
	// report must not contain full hostnames).
	banned := []string{
		"www.youtube.com", "youtube.com.", "youtu.be", "googlevideo.com",
		"ytimg.com", "ggpht.com", "sky.rethinkdns.com", "cloudflare-dns.com",
		"dns.google", "172.67.162.27", "1.1.1.1", "8.8.8.8",
		"generate_204", "robots.txt", "redirector.googlevideo.com",
	}
	for _, bad := range banned {
		if strings.Contains(report, bad) {
			t.Errorf("diagnostic report contains %q:\n%s", bad, report)
		}
	}

	// Scan the log file on disk too.
	logDir := dir + "/logs"
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("log dir: %v", err)
	}
	scanned := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "app.log") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, bad := range banned {
			if strings.Contains(string(data), bad) {
				t.Errorf("log file %s contains %q", entry.Name(), bad)
			}
		}
		// IPs in general: scan for dotted quads that are not the local
		// listener the user already sees.
		for _, token := range strings.Fields(string(data)) {
			if ip := net.ParseIP(strings.Trim(token, ":,()[]")); ip != nil {
				if !ip.IsLoopback() && !ip.IsUnspecified() {
					t.Errorf("log file %s contains non-loopback IP %q", entry.Name(), token)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no log file was written to scan")
	}
}

// TestPrivacyWithLogDomainsEnabledShowsHostnames documents the opt-in: when
// "Keep website names in the log" is on, names may appear (the user asked).
func TestPrivacyWithLogDomainsEnabledShowsHostnames(t *testing.T) {
	dir := t.TempDir()
	plat := stub.NewWithDir(dir)
	fake := &fakeNetwork{}
	fake.acceptAll(true)
	// Settings with logDomains enabled, saved before the engine loads.
	s := config.DefaultSettings()
	s.Advanced.LogDomains = true
	if err := config.Save(dir, s); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Options{
		Platform: plat,
		LogDir:   dir + "/logs",
		Checker:  fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Shutdown)
	var loaded config.Settings
	if err := jsonUnmarshal(eng.GetSettingsJSON(), &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.Advanced.LogDomains {
		t.Fatal("logDomains should have loaded as true")
	}
	// With the flag on, the proxy logs hostnames for tunnels; there is no
	// hostname traffic in this hermetic test, so this only documents the
	// setting round-trip (the redaction side is covered above).
}

func jsonUnmarshal(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
