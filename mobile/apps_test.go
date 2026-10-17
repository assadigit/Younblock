package mobile

// apps_test.go — the facade surface for the per-app allow-list (doc 04
// §9; D86: the YouTube default): RoutedApps reports the engine's
// normalized `apps` settings — the same truth GetSettings reports and
// the WebView's Apps-to-unblock row edits — so the Kotlin service, the
// settings file and the UI can never disagree about what routes. The
// list starts with the YouTube app; a list the user EMPTIED stays empty
// (the honest refusal state, never route-everything), and the v3 Load
// migration (config) is what carries debug.1's born-empty files to the
// default.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
)

func TestRoutedAppsDefaultIsYouTube(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	got := m.RoutedApps()
	// D86: the fresh engine reports the YouTube default, as an explicit
	// array (never null: the Kotlin reader must see one).
	if !containsApp(got, "com.google.android.youtube") {
		t.Fatalf("RoutedApps() = %s, want the YouTube default on the list", got)
	}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s (not a JSON object with an apps list): %v", got, err)
	}
	want := []string{"com.google.android.youtube"}
	if !reflect.DeepEqual(parsed["apps"], want) {
		t.Errorf("RoutedApps() apps = %v, want %v (D86)", parsed["apps"], want)
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

	// An emptied/invalid list persists as the honest EMPTY list: the
	// user's own state under D86, and the Kotlin establish refuses to
	// route it (the M7 NO_TARGET_APPS door) — a VpnService with zero
	// allowed apps would route EVERYTHING, so the empty list rides with
	// the typed refusal, never a fallback to the default.
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

// TestRoutedAppsDebug1FileMigratesToDefault is the update path on Ali's
// device (D86): debug.1 shipped apps:[] (D83) and saved it; the debug.2
// engine's first Load over that file migrates it to the YouTube default
// — the exact ruling "mAKE the youtube default added list".
func TestRoutedAppsDebug1FileMigratesToDefault(t *testing.T) {
	dir := t.TempDir()
	v2Empty := `{"version":2,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":[],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v2Empty), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	var parsed map[string][]string
	if err := json.Unmarshal([]byte(m.RoutedApps()), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", m.RoutedApps(), err)
	}
	want := []string{"com.google.android.youtube"}
	if !reflect.DeepEqual(parsed["apps"], want) {
		t.Errorf("apps = %v, want the D86 default after the v2-empty migration", parsed["apps"])
	}
	// The two doors agree after the migration.
	var s config.Settings
	if err := json.Unmarshal([]byte(m.GetSettings()), &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("GetSettings().Apps = %v, want %v — the doors disagree after migration", s.Apps, want)
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

// containsApp reports whether the RoutedApps JSON lists pkg.
func containsApp(routedAppsJSON string, pkg string) bool {
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(routedAppsJSON), &parsed); err != nil {
		return false
	}
	for _, p := range parsed["apps"] {
		if p == pkg {
			return true
		}
	}
	return false
}
