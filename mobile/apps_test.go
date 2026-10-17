package mobile

// apps_test.go — the facade surface for the per-app allow-list (doc 04
// §9; D86 put the YouTube app on the default; D87 — Ali's second
// ruling — extends it to the Chrome, Firefox, Opera and Brave
// browsers): RoutedApps reports the engine's normalized `apps`
// settings — the same truth GetSettings reports and the WebView's
// Apps-to-unblock row edits — so the Kotlin service, the settings file
// and the UI can never disagree about what routes. The list starts
// with the five default apps; a list the user EMPTIED stays empty (the
// honest refusal state, never route-everything), and the v4 Load
// migration (config) is what carries debug.1's born-empty files and
// debug.2's [youtube]-only files to the default.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/assadigit/Younblock/internal/config"
)

// d87Default mirrors internal/config's D87 default here so the facade
// tests read against the ruling, not against a private helper.
var d87Default = []string{
	"com.google.android.youtube",
	"com.android.chrome",
	"org.mozilla.firefox",
	"com.opera.browser",
	"com.brave.browser",
}

func TestRoutedAppsDefaultIsTheD87List(t *testing.T) {
	m, err := NewEngine(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	got := m.RoutedApps()
	// D87: the fresh engine reports the five-app default, as an explicit
	// array (never null: the Kotlin reader must see one).
	if !containsApp(got, "com.google.android.youtube") {
		t.Fatalf("RoutedApps() = %s, want the YouTube app on the default list", got)
	}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s (not a JSON object with an apps list): %v", got, err)
	}
	if !reflect.DeepEqual(parsed["apps"], d87Default) {
		t.Errorf("RoutedApps() apps = %v, want %v (D87)", parsed["apps"], d87Default)
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
	// user's own state under D87, and the Kotlin establish refuses to
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

// TestRoutedAppsDebug1FileMigratesToDefault is the debug.1 update path
// (D86, now under the v4 migration): debug.1 shipped apps:[] (D83) and
// saved it; a D87 engine's first Load over that file migrates it to
// the five-app default.
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
	if !reflect.DeepEqual(parsed["apps"], d87Default) {
		t.Errorf("apps = %v, want the D87 default after the v2-empty migration", parsed["apps"])
	}
	// The two doors agree after the migration.
	var s config.Settings
	if err := json.Unmarshal([]byte(m.GetSettings()), &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Apps, d87Default) {
		t.Errorf("GetSettings().Apps = %v, want %v — the doors disagree after migration", s.Apps, d87Default)
	}
}

// TestRoutedAppsDebug2FileMigratesToDefault is the debug.2 update path
// on Ali's device (D87): debug.2 shipped exactly [youtube] as its
// default and saved it (fresh default or the v3 migration); a D87
// engine's first Load over that v3 file upgrades it to the five-app
// default — "Add chrome to the default addlist as well, also firefox,
// opera, brave browser."
func TestRoutedAppsDebug2FileMigratesToDefault(t *testing.T) {
	dir := t.TempDir()
	v3OldDefault := `{"version":3,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.google.android.youtube"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v3OldDefault), 0o600); err != nil {
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
	if !reflect.DeepEqual(parsed["apps"], d87Default) {
		t.Errorf("apps = %v, want the D87 default after the v3-old-default upgrade", parsed["apps"])
	}
	// The two doors agree after the migration.
	var s config.Settings
	if err := json.Unmarshal([]byte(m.GetSettings()), &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Apps, d87Default) {
		t.Errorf("GetSettings().Apps = %v, want %v — the doors disagree after migration", s.Apps, d87Default)
	}
}

// TestRoutedAppsV3CustomListSurvives: a debug.2 file whose list the
// user curated (e.g. tapped + Chrome — a superset of the old default)
// rides the v4 migration untouched — the engine reports exactly what
// was saved, and the chips (D87 widens them to all five defaults) are
// the one-tap way to regain any default app.
func TestRoutedAppsV3CustomListSurvives(t *testing.T) {
	dir := t.TempDir()
	v3Custom := `{"version":3,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.google.android.youtube","com.android.chrome"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v3Custom), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEngine(dir, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer m.Shutdown()

	want := []string{"com.google.android.youtube", "com.android.chrome"}
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(m.RoutedApps()), &parsed); err != nil {
		t.Fatalf("RoutedApps() = %s: %v", m.RoutedApps(), err)
	}
	if !reflect.DeepEqual(parsed["apps"], want) {
		t.Errorf("apps = %v, want the curated list kept verbatim: %v", parsed["apps"], want)
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
