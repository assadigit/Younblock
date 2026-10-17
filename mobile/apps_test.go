package mobile

// apps_test.go — the M6 facade surface for the per-app allow-list
// (docs/android-port 05 §M6, 04 §9): RoutedApps reports the engine's
// normalized `apps` settings — the same truth GetSettings reports and
// the WebView's Apps-to-unblock row edits — so the Kotlin service, the
// settings file and the UI can never disagree about what routes.

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
)

func TestRoutedAppsDefault(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	got := m.RoutedApps()
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s (not a JSON object with an apps list): %v", got, err)
	}
	if !reflect.DeepEqual(parsed["apps"], config.DefaultApps()) {
		t.Errorf("RoutedApps() apps = %v, want the built-in defaults %v", parsed["apps"], config.DefaultApps())
	}
}

func TestRoutedAppsTracksSavedSettings(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	// The WebView's save path: a full settings payload with a new list.
	saved := m.SaveSettings(`{"apps":["org.mozilla.firefox","com.android.chrome"]}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings = %s, want error:null", saved)
	}
	got := m.RoutedApps()
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", got, err)
	}
	want := []string{"org.mozilla.firefox", "com.android.chrome"}
	if !reflect.DeepEqual(parsed["apps"], want) {
		t.Errorf("RoutedApps() apps = %v, want %v (order preserved, as saved)", parsed["apps"], want)
	}

	// The same truth through the settings door (one list, two views).
	settings := m.GetSettings()
	var s config.Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		t.Fatalf("GetSettings = %s: %v", settings, err)
	}
	if !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("GetSettings().Apps = %v, want %v — the two doors disagree", s.Apps, want)
	}

	// An emptied/invalid list cannot persist: Normalize restores the
	// defaults, and RoutedApps reports them (a VpnService with zero
	// allowed apps would route EVERYTHING — fail toward the default
	// scope, never the route-everything scope).
	saved = m.SaveSettings(`{"apps":["not a package",""]}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings(all-invalid) = %s, want error:null", saved)
	}
	got = m.RoutedApps()
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", got, err)
	}
	if !reflect.DeepEqual(parsed["apps"], config.DefaultApps()) {
		t.Errorf("all-invalid apps = %v, want the defaults restored", parsed["apps"])
	}
}

func TestRoutedAppsSurvivesEngineRestartCycle(t *testing.T) {
	dir := t.TempDir()
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	m.Shutdown()

	want := []string{"com.android.chrome", "org.mozilla.firefox"}
	saved := m.SaveSettings(`{"apps":["com.android.chrome","org.mozilla.firefox"]}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings = %s", saved)
	}

	// A fresh engine over the same config dir (the process-death path):
	// the list persisted and the new facade reports it.
	m2, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine(2): %v", err)
	}
	defer m2.Shutdown()
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(m2.RoutedApps()), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", m2.RoutedApps(), err)
	}
	if !reflect.DeepEqual(parsed["apps"], want) {
		t.Errorf("apps after restart = %v, want %v (settings must survive)", parsed["apps"], want)
	}
}

// jsonErrNull reports whether a jsonapi-style reply carries "error":null.
func jsonErrNull(reply string) bool {
	var r struct {
		Error *string `json:"error"`
	}
	return json.Unmarshal([]byte(reply), &r) == nil && r.Error == nil
}
