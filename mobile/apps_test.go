package mobile

// apps_test.go — the M6 facade surface for the per-app allow-list
// (docs/android-port 05 §M6, 04 §9; D83: no allow-list default):
// RoutedApps reports the engine's normalized `apps` settings — the
// same truth GetSettings reports and the WebView's Apps-to-unblock
// row edits — so the Kotlin service, the settings file and the UI can
// never disagree about what routes. The list starts EMPTY; an empty
// list is the honest refusal state, never route-everything.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
)

func TestRoutedAppsDefaultIsEmpty(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	got := m.RoutedApps()
	// D83: no default — the fresh engine reports the honest empty list,
	// as an explicit [] (never null: the Kotlin reader must see an
	// array, and stopSelfHonest's refusal path depends on it).
	if !strings.Contains(got, `"apps":[]`) {
		t.Fatalf("RoutedApps() = %s, want the explicit empty list {\"apps\":[\"…]}", got)
	}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s (not a JSON object with an apps list): %v", got, err)
	}
	if len(parsed["apps"]) != 0 {
		t.Errorf("RoutedApps() apps = %v, want empty (D83: no default)", parsed["apps"])
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

	// An emptied/invalid list persists as the honest EMPTY list (D83):
	// Normalize keeps it empty, RoutedApps reports it, and the Kotlin
	// establish refuses to route (the M7 NO_TARGET_APPS door) — a
	// VpnService with zero allowed apps would route EVERYTHING, so the
	// empty list rides with the typed refusal, never a fallback.
	saved = m.SaveSettings(`{"apps":["not a package",""]}`)
	if !jsonErrNull(saved) {
		t.Fatalf("SaveSettings(all-invalid) = %s, want error:null", saved)
	}
	got = m.RoutedApps()
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", got, err)
	}
	if len(parsed["apps"]) != 0 {
		t.Errorf("all-invalid apps = %v, want the honest empty list", parsed["apps"])
	}
	// The settings door agrees (one truth, two views).
	settings = m.GetSettings()
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		t.Fatalf("GetSettings = %s: %v", settings, err)
	}
	if len(s.Apps) != 0 {
		t.Errorf("GetSettings().Apps = %v, want empty — the two doors disagree", s.Apps)
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
