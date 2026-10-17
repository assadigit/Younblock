package config

// apps_test.go — the per-app allow-list tests (doc 04 §9; M6 schema v2,
// D83's empty default, now D86's YouTube default + the v3 one-time
// migration): the `apps` field, its Normalize rules, the v1/v2 → v3
// migrations, and the user-agency rule (a saved empty list stays empty).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultAppsIsYouTube(t *testing.T) {
	// D86: the default added list is the YouTube app — Ali's ruling
	// after his first device observation ("mAKE the youtube default
	// added list"). Fresh copies, never a shared slice.
	want := []string{"com.google.android.youtube"}
	if got := DefaultApps(); !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultApps() = %v, want %v (D86)", got, want)
	}
	if s := DefaultSettings(); !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("DefaultSettings().Apps = %v, want %v", s.Apps, want)
	}
	// Each call returns an independent copy (mutating one must not
	// leak into the next caller's list).
	a, b := DefaultApps(), DefaultApps()
	a[0] = "mutated.example.app"
	if !reflect.DeepEqual(b, want) {
		t.Errorf("DefaultApps() slices share backing storage: %v", b)
	}
	// The marshaled shape the facade and the WebView depend on.
	bs, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bs), `"apps":["com.google.android.youtube"]`) {
		t.Errorf("marshaled settings = %s, want the YouTube default in apps", bs)
	}
}

func TestNormalizeAppsRules(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		// An empty or all-invalid list STAYS empty — the user's own
		// state under D86 (they removed the last entry, or a partial
		// save emptied it). The NO_TARGET_APPS door owns what happens
		// at connect; empty must persist honestly, never restore.
		{"nil stays empty", nil, []string{}},
		{"empty stays empty", []string{}, []string{}},
		{"trim and keep order", []string{"  com.android.chrome ", "org.mozilla.firefox"}, []string{"com.android.chrome", "org.mozilla.firefox"}},
		{"dedupe", []string{"com.android.chrome", "com.android.chrome"}, []string{"com.android.chrome"}},
		{"drop invalid shapes", []string{"chrome", "com", "1com.example", "com.1example", "com.exa mple", "com.exa:mple", "com..example", ".com.example", "com.example."}, []string{}},
		{"all-invalid stays empty", []string{"no dot", "!!!", ""}, []string{}},
		{"mixed keeps the valid ones", []string{"com.android.chrome", "not a package", "org.mozilla.firefox"}, []string{"com.android.chrome", "org.mozilla.firefox"}},
		{"case preserved", []string{"Com.Example.App"}, []string{"Com.Example.App"}},
		{"underscores and digits accepted", []string{"com.example_2.app3"}, []string{"com.example_2.app3"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeApps(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normalizeApps(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeAppsCap(t *testing.T) {
	list := make([]string, maxApps+10)
	for i := range list {
		list[i] = "com.example.app" + string(rune('a'+i%26)) + strings.Repeat("x", i/26) // distinct, valid
	}
	got := normalizeApps(list)
	if len(got) != maxApps {
		t.Errorf("len = %d, want the cap %d", len(got), maxApps)
	}
	if got[0] != list[0] {
		t.Errorf("first-seen order broken: %q", got[0])
	}
}

func TestNormalizeKeepsNilAppsEmptyAndBumpsVersion(t *testing.T) {
	// Normalize is the validator, not the migration: nil arrives here
	// only from partial payloads (a full-document save always carries
	// apps), and the fail-closed answer is the honest empty list. The
	// D86 default is filled by DefaultSettings and the Load migration.
	s := DefaultSettings()
	s.Version = 1
	s.Apps = nil
	n := Normalize(s)
	if n.Version != settingsVersion {
		t.Errorf("version = %d, want %d", n.Version, settingsVersion)
	}
	if len(n.Apps) != 0 {
		t.Errorf("apps = %v, want empty (Normalize never fills a default)", n.Apps)
	}
	if n.Apps == nil {
		t.Error("apps is nil, want a non-nil empty slice (marshals as [])")
	}
}

func TestSettingsV3MigrationFromV1File(t *testing.T) {
	dir := t.TempDir()
	// A real v1-era file: version 1, no apps field, custom values the
	// migration must preserve.
	v1 := `{"version":1,"mode":"all","method":"legacy","port":4530,"autoPort":false,` +
		`"startWithWindows":true,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,` +
		`"advanced":{"pieceSizeMin":100,"pieceSizeMax":500,"pauseMs":7,"dohServers":[],"domains":["youtube.com","youtu.be"],"logDomains":true,"idleTimeoutSec":60}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	s, notice, err := Load(dir)
	if err != nil || notice != "" {
		t.Fatalf("Load: %v notice=%q", err, notice)
	}
	if s.Version != settingsVersion {
		t.Errorf("version = %d, want %d (migrated)", s.Version, settingsVersion)
	}
	// The v1 file had no apps field — the never-configured state — so
	// it migrates to the D86 default: the YouTube app (D86).
	if want := []string{"com.google.android.youtube"}; !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("apps = %v, want %v (D86 default)", s.Apps, want)
	}
	// The v1 values survive the migration verbatim.
	if s.Mode != ModeAll || s.Method != MethodLegacy || s.Port != 4530 || s.AutoPort {
		t.Errorf("v1 fields lost: mode=%q method=%q port=%d autoPort=%v", s.Mode, s.Method, s.Port, s.AutoPort)
	}
	if !s.StartWithWindows || !s.FirstRunDone || !s.Advanced.LogDomains || s.Advanced.PauseMs != 7 {
		t.Errorf("v1 flags lost: %+v", s)
	}
	if strings.Join(s.Advanced.Domains, ",") != "youtube.com,youtu.be" {
		t.Errorf("v1 domains lost: %v", s.Advanced.Domains)
	}
	// Persisting the migrated settings writes a v3 file carrying apps.
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["version"].(float64) != float64(settingsVersion) {
		t.Errorf("saved version = %v, want %d", raw["version"], settingsVersion)
	}
	apps, ok := raw["apps"].([]any)
	if !ok || len(apps) != 1 || apps[0] != "com.google.android.youtube" {
		t.Fatalf("saved apps = %v, want the D86 default", raw["apps"])
	}
}

// TestSettingsV3MigrationFromDebug1EmptyList is THE device case (D86):
// debug.1 shipped with apps:[] (D83) and an entry path that refused
// friendly names, so its v2 files carry the born-empty shipped default —
// never a user choice. The update to the D86 build migrates that file to
// the YouTube default, which is exactly Ali's requested outcome.
func TestSettingsV3MigrationFromDebug1EmptyList(t *testing.T) {
	dir := t.TempDir()
	v2Empty := `{"version":2,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":[],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v2Empty), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Version != settingsVersion {
		t.Errorf("version = %d, want %d", s.Version, settingsVersion)
	}
	if want := []string{"com.google.android.youtube"}; !reflect.DeepEqual(s.Apps, want) {
		t.Fatalf("apps = %v, want the D86 default (the debug.1 born-empty list migrates)", s.Apps)
	}
}

// TestSettingsV3EmptyListIsUserAgency: once the file IS v3, an empty list
// is the user's own (they removed the last entry) and must never be
// refilled by a later load — the agency rule the M6→M8-a line kept.
func TestSettingsV3EmptyListIsUserAgency(t *testing.T) {
	dir := t.TempDir()
	s := DefaultSettings()
	s.Apps = []string{}
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Apps) != 0 {
		t.Errorf("apps = %v, want the user's empty list kept (agency, never restore)", got.Apps)
	}
	if got.Apps == nil {
		t.Error("apps is nil, want non-nil []")
	}
}

// TestSettingsV2CustomListSurvives: a debug.1 user who DID configure a
// list keeps it verbatim — the migration only fills the born-empty state.
func TestSettingsV2CustomListSurvives(t *testing.T) {
	dir := t.TempDir()
	v2Custom := `{"version":2,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.android.chrome","com.google.android.youtube"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v2Custom), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"com.android.chrome", "com.google.android.youtube"}
	if !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("apps = %v, want the user's list kept verbatim: %v", s.Apps, want)
	}
}

func TestSettingsAppsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := DefaultSettings()
	s.Apps = []string{"com.android.chrome", "org.mozilla.firefox", "com.google.android.youtube"}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"com.android.chrome", "org.mozilla.firefox", "com.google.android.youtube"}
	if !reflect.DeepEqual(got.Apps, want) {
		t.Errorf("apps round trip = %v, want %v", got.Apps, want)
	}
}

func TestSettingsAppsJSONShape(t *testing.T) {
	// The exact JSON shape the mobile facade (RoutedApps) and the Svelte
	// apps row depend on.
	b, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var apps []string
	if err := json.Unmarshal(raw["apps"], &apps); err != nil {
		t.Fatalf("apps field not a string list: %v", err)
	}
	if !reflect.DeepEqual(apps, DefaultApps()) {
		t.Errorf("apps = %v, want %v", apps, DefaultApps())
	}
}
