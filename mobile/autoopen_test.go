package mobile

// autoopen_test.go — the facade surface for the connect-time hand-off
// switch (M8-am, ADR-0051, D105): AutoOpenYouTube reports the engine's
// normalized `autoOpenYouTube` settings — the same truth GetSettings
// reports and the WebView's Settings row edits — so the Kotlin service,
// the settings file and the UI can never disagree about whether the
// YouTube app opens when the engine turns on. The fresh Android engine
// carries the checked-by-default ruling; the shell reads the switch
// fresh at the moment "on" arrives, and a change saved while connected
// applies on the next connect (the allow-list's rule, verbatim).

import (
	"encoding/json"
	"testing"
)

// TestAutoOpenYouTubeDefaultOn: the fresh engine (the Android platform's
// LoadFor defaults) reports the ruling's true, as the JSON shape the
// Kotlin reader parses.
func TestAutoOpenYouTubeDefaultOn(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	got := m.AutoOpenYouTube()
	var parsed map[string]bool
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("AutoOpenYouTube() = %s (not a JSON object): %v", got, err)
	}
	if !parsed["enabled"] {
		t.Fatalf("AutoOpenYouTube() = %s, want enabled:true (M8-am, D105: checked by default)", got)
	}
}

// TestAutoOpenYouTubeTracksSavedSettings: the WebView's save path moves
// the facade's answer both ways — the switch is a real switch.
func TestAutoOpenYouTubeTracksSavedSettings(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	// Off through the settings door (the partial-payload save the UI
	// emits for the one row).
	saved := m.SaveSettings(`{"autoOpenYouTube":false}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings = %s, want error:null", saved)
	}
	if got := m.AutoOpenYouTube(); got != `{"enabled":false}` {
		t.Fatalf("AutoOpenYouTube() = %s, want enabled:false after the save", got)
	}

	// And back on — the facade follows whatever the normalized settings
	// hold, never a cached copy.
	saved = m.SaveSettings(`{"autoOpenYouTube":true}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings = %s, want error:null", saved)
	}
	if got := m.AutoOpenYouTube(); got != `{"enabled":true}` {
		t.Fatalf("AutoOpenYouTube() = %s, want enabled:true after the save", got)
	}
}
