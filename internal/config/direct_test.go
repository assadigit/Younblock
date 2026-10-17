package config

// direct_test.go — the never-processed TLD policy (D83): every host
// under .ir stays out of the app's machinery, and nothing else is
// swept up by accident.

import "testing"

func TestUnderDirectTLD(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"ir", true},
		{"IR", true},
		{"example.ir", true},
		{"a.b.example.ir", true},
		{"example.IR", true},
		{"example.ir.", true},     // trailing root dot tolerated
		{" example.ir ", true},    // surrounding spaces tolerated
		{"example.ir.com", false}, // .ir must be the last label
		{"notir", false},
		{"notir.com", false},
		{"ircom", false},
		{"", false},
		{"192.168.1.1", false},
	}
	for _, tc := range cases {
		if got := UnderDirectTLD(tc.host); got != tc.want {
			t.Errorf("UnderDirectTLD(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestDirectTLDsIsACopy(t *testing.T) {
	got := DirectTLDs()
	if len(got) == 0 {
		t.Fatal("DirectTLDs() is empty — the .ir rule has lost its list")
	}
	got[0] = "mutated"
	if DirectTLDs()[0] == "mutated" {
		t.Fatal("DirectTLDs() returned the package-level slice (not a copy)")
	}
}
