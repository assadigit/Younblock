package config

// apps_test.go — the M6 schema-v2 tests (doc 05 §M6, doc 04 §9): the
// `apps` field, its Normalize rules, and the v1 → v2 migration.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultApps(t *testing.T) {
	got := DefaultApps()
	want := []string{"com.android.chrome", "com.google.android.youtube"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultApps() = %v, want %v", got, want)
	}
	if s := DefaultSettings(); !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("DefaultSettings().Apps = %v, want %v", s.Apps, want)
	}
	// A copy, not the package-level slice: mutating the result must not
	// corrupt the built-in default.
	got[0] = "mutated"
	if DefaultApps()[0] != "com.android.chrome" {
		t.Fatal("DefaultApps() returned the package-level slice (not a copy)")
	}
}

func TestNormalizeAppsRules(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil falls back to defaults", nil, DefaultApps()},
		{"empty falls back to defaults", []string{}, DefaultApps()},
		{"trim and keep order", []string{"  com.android.chrome ", "com.google.android.youtube"}, DefaultApps()},
		{"dedupe", []string{"com.android.chrome", "com.android.chrome"}, []string{"com.android.chrome"}},
		{"drop invalid shapes", []string{"chrome", "com", "1com.example", "com.1example", "com.exa mple", "com.exa:mple", "com..example", ".com.example", "com.example."}, DefaultApps()},
		{"all-invalid falls back to defaults", []string{"no dot", "!!!", ""}, DefaultApps()},
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

func TestNormalizeFillsAppsAndBumpsVersion(t *testing.T) {
	s := DefaultSettings()
	s.Version = 1
	s.Apps = nil
	n := Normalize(s)
	if n.Version != settingsVersion {
		t.Errorf("version = %d, want %d", n.Version, settingsVersion)
	}
	if !reflect.DeepEqual(n.Apps, DefaultApps()) {
		t.Errorf("apps = %v, want defaults restored", n.Apps)
	}
}

func TestSettingsV2MigrationFromV1File(t *testing.T) {
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
	if s.Version != 2 {
		t.Errorf("version = %d, want 2 (migrated)", s.Version)
	}
	if !reflect.DeepEqual(s.Apps, DefaultApps()) {
		t.Errorf("apps = %v, want the built-in defaults filled in", s.Apps)
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
	// Persisting the migrated settings writes a v2 file carrying apps.
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
	if raw["version"].(float64) != 2 {
		t.Errorf("saved version = %v, want 2", raw["version"])
	}
	apps, ok := raw["apps"].([]any)
	if !ok || len(apps) != 2 {
		t.Fatalf("saved apps = %v, want the two defaults", raw["apps"])
	}
	if apps[0] != "com.android.chrome" || apps[1] != "com.google.android.youtube" {
		t.Errorf("saved apps = %v, want chrome+youtube", apps)
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
