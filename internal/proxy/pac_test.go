package proxy

import (
	"strings"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
)

func TestMatchDomainSemantics(t *testing.T) {
	cases := []struct {
		host   string
		domain string
		want   bool
	}{
		{"youtube.com", "youtube.com", true},
		{"www.youtube.com", "youtube.com", true},
		{"a.b.youtube.com", "youtube.com", true},
		{"notyoutube.com", "youtube.com", false},
		{"youtube.com.evil.example", "youtube.com", false},
		{"youtu.be", "youtube.com", false},
		{"googlevideo.com", "googlevideo.com", true},
		{"r1---sn-x.googlevideo.com", "googlevideo.com", true},
		{"YTIMG.com", "ytimg.com", true},          // case-insensitive
		{"www.youtube.com.", "youtube.com", true}, // trailing dot tolerated
		{"xyoutu.be", "youtu.be", false},
	}
	for _, tc := range cases {
		if got := MatchDomain(tc.host, tc.domain); got != tc.want {
			t.Errorf("MatchDomain(%q, %q) = %v, want %v", tc.host, tc.domain, got, tc.want)
		}
	}
}

func TestPACYoutubeMode(t *testing.T) {
	pac := PAC(4525, []string{"youtube.com", "googlevideo.com"}, config.ModeYouTube)
	for _, want := range []string{
		`host === "youtube.com"`,
		`shExpMatch(host, "*.youtube.com")`,
		`host === "googlevideo.com"`,
		`shExpMatch(host, "*.googlevideo.com")`,
		`return "PROXY 127.0.0.1:4525; DIRECT";`,
		`return "DIRECT";`,
		`function FindProxyForURL(url, host)`,
	} {
		if !strings.Contains(pac, want) {
			t.Errorf("PAC missing %q in:\n%s", want, pac)
		}
	}
	// The DIRECT fallback must come after the PROXY entry so the browser
	// falls back to direct when the app is not running.
	if strings.Index(pac, "PROXY") > strings.LastIndex(pac, `return "DIRECT"`) {
		t.Error("PAC order wrong: DIRECT must be reachable after PROXY failures")
	}
}

func TestPACAllMode(t *testing.T) {
	pac := PAC(4530, nil, config.ModeAll)
	for _, want := range []string{
		"isPlainHostName(host)",
		`isInNet(resolved, "10.0.0.0", "255.0.0.0")`,
		`isInNet(resolved, "172.16.0.0", "255.240.0.0")`,
		`isInNet(resolved, "192.168.0.0", "255.255.0.0")`,
		`isInNet(resolved, "127.0.0.0", "255.0.0.0")`,
		`return "PROXY 127.0.0.1:4530; DIRECT";`,
	} {
		if !strings.Contains(pac, want) {
			t.Errorf("all-mode PAC missing %q in:\n%s", want, pac)
		}
	}
}

func TestPACManualModeHasNoProxy(t *testing.T) {
	// Manual mode never installs a PAC, but if one is ever generated it must
	// not point at us; here we only assert the generator stays consistent.
	pac := PAC(4525, config.DefaultDomains(), config.ModeManual)
	if strings.Contains(pac, "PROXY 127.0.0.1") {
		t.Errorf("manual mode should not generate a proxying PAC:\n%s", pac)
	}
}

func TestPACGeneratorIsDeterministic(t *testing.T) {
	a := PAC(4525, config.DefaultDomains(), config.ModeYouTube)
	b := PAC(4525, config.DefaultDomains(), config.ModeYouTube)
	if a != b {
		t.Error("PAC generation is not deterministic")
	}
}
