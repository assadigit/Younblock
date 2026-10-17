package config

// apps_test.go — the per-app allow-list tests (doc 04 §9; M6 schema v2,
// D83's empty default, D86's YouTube default, now D87's five-app default
// — YouTube + Chrome, Firefox, Opera, Brave — and the v4 one-time
// migration): the `apps` field, its Normalize rules, the v1/v2/v3 → v4
// migrations, and the user-agency rule (a saved empty list stays empty;
// a curated list is never re-touched).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// d87Default is the D87 ruling's list, spelled out once so every
// assertion below reads against the decision, not against a helper.
var d87Default = []string{
	"com.google.android.youtube",
	"com.android.chrome",
	"org.mozilla.firefox",
	"com.opera.browser",
	"com.brave.browser",
}

func TestDefaultAppsIsYouTubeAndTheBrowsers(t *testing.T) {
	// D87: the default added list is the YouTube app plus the four
	// mainstream browsers — Ali's ruling ("Add chrome to the default
	// addlist as well, also firefox, opera, brave browser"), extending
	// D86's YouTube default. Fresh copies, never a shared slice.
	if got := DefaultApps(); !reflect.DeepEqual(got, d87Default) {
		t.Errorf("DefaultApps() = %v, want %v (D87)", got, d87Default)
	}
	if s := DefaultSettings(); !reflect.DeepEqual(s.Apps, d87Default) {
		t.Errorf("DefaultSettings().Apps = %v, want %v", s.Apps, d87Default)
	}
	// Each call returns an independent copy (mutating one must not
	// leak into the next caller's list).
	a, b := DefaultApps(), DefaultApps()
	a[0] = "mutated.example.app"
	if !reflect.DeepEqual(b, d87Default) {
		t.Errorf("DefaultApps() slices share backing storage: %v", b)
	}
	// The marshaled shape the facade and the WebView depend on.
	bs, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	want := `"apps":["com.google.android.youtube","com.android.chrome","org.mozilla.firefox","com.opera.browser","com.brave.browser"]`
	if !strings.Contains(string(bs), want) {
		t.Errorf("marshaled settings = %s, want the D87 default in apps", bs)
	}
}

func TestNormalizeAppsRules(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		// An empty or all-invalid list STAYS empty — the user's own
		// state under D87 (they removed the last entry, or a partial
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
		// The D87 default's five entries are all valid package shapes
		// (the whole default must survive its own validator).
		{"the D87 default round-trips", d87Default, d87Default},
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
	// D87 default is filled by DefaultSettings and the Load migration.
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

func TestSettingsV4MigrationFromV1File(t *testing.T) {
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
	// it migrates straight to the D87 default (the five apps).
	if !reflect.DeepEqual(s.Apps, d87Default) {
		t.Errorf("apps = %v, want %v (D87 default)", s.Apps, d87Default)
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
	// Persisting the migrated settings writes a v4 file carrying apps.
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
	if !ok || len(apps) != len(d87Default) || apps[0] != "com.google.android.youtube" {
		t.Fatalf("saved apps = %v, want the D87 default", raw["apps"])
	}
}

// TestSettingsV4MigrationFromDebug1EmptyList is THE debug.1 device case
// (D86, now under the v4 migration): debug.1 shipped with apps:[] (D83)
// and an entry path that refused friendly names, so its v2 files carry
// the born-empty shipped default — never a user choice. An update to a
// D87 build migrates that file straight to the five-app default.
func TestSettingsV4MigrationFromDebug1EmptyList(t *testing.T) {
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
	if !reflect.DeepEqual(s.Apps, d87Default) {
		t.Fatalf("apps = %v, want the D87 default (the debug.1 born-empty list migrates)", s.Apps)
	}
}

// TestSettingsV4MigrationFromV3OldDefault is THE debug.2 device case
// (D87): debug.2 shipped with exactly [com.google.android.youtube] —
// every fresh v3 file carried it, and the v3 migration wrote it over
// debug.1's born-empty files. A v3 file still holding EXACTLY that list
// was written by the build itself, never curated, so the v4 migration
// upgrades it to the five-app default — which is exactly Ali's
// requested outcome on the one device carrying debug.2.
func TestSettingsV4MigrationFromV3OldDefault(t *testing.T) {
	dir := t.TempDir()
	v3OldDefault := `{"version":3,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.google.android.youtube"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v3OldDefault), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Version != settingsVersion {
		t.Errorf("version = %d, want %d (migrated)", s.Version, settingsVersion)
	}
	if !reflect.DeepEqual(s.Apps, d87Default) {
		t.Errorf("apps = %v, want the D87 default (the v3-era shipped default upgrades)", s.Apps)
	}
}

// TestSettingsV3EmptyListIsUserAgency: a v3 file's empty list is the
// user's own (they removed the last entry under the debug.2 build) and
// must never be refilled by the v4 migration — the agency rule D86
// set and D87 keeps. (A v4 empty is the same rule one version later.)
func TestSettingsV3EmptyListIsUserAgency(t *testing.T) {
	dir := t.TempDir()
	v3Empty := `{"version":3,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":[],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v3Empty), 0o600); err != nil {
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
	// And once saved, the file is v4-empty — still empty forever after.
	if err := Save(dir, got); err != nil {
		t.Fatal(err)
	}
	again, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(2): %v", err)
	}
	if len(again.Apps) != 0 {
		t.Errorf("apps after save+load = %v, want still empty", again.Apps)
	}
}

// TestSettingsV3CustomListSurvives: a debug.2 user who ADDED an app
// (e.g. tapped + Chrome — row 4 of the test circle) keeps their list
// verbatim — the v4 migration upgrades only the never-customized
// era-default state, never a curated list.
func TestSettingsV3CustomListSurvives(t *testing.T) {
	dir := t.TempDir()
	v3Custom := `{"version":3,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.google.android.youtube","com.android.chrome"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v3Custom), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"com.google.android.youtube", "com.android.chrome"}
	if !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("apps = %v, want the user's list kept verbatim: %v", s.Apps, want)
	}
}

// TestSettingsV2CustomListSurvives: a debug.1 user who DID configure a
// list keeps it verbatim — including a list that happens to equal the
// later D86 default, because under debug.1's refusing entry path every
// entry was hand-typed (an exact package name), i.e. a deliberate
// choice. The v4 migration upgrades only v3-era files; v1/v2 lists
// with entries are custom, always.
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

// TestSettingsV2HandTypedYouTubeSurvives: the v2 twin of the
// exactly-the-old-default case — a debug.1 user who typed the full
// "com.google.android.youtube" package name by hand ends up with a
// file identical to what debug.2 would later ship as its default, but
// it was a deliberate act (the only way a v2 file got entries), so it
// is NOT upgraded to the five-app default. D86 drew this exact line
// ("any v1/v2 list with entries is kept verbatim"); D87 keeps it.
func TestSettingsV2HandTypedYouTubeSurvives(t *testing.T) {
	dir := t.TempDir()
	v2Typed := `{"version":2,"mode":"youtube","method":"auto","port":4525,"autoPort":true,` +
		`"startWithWindows":false,"connectOnLaunch":false,"openBrowserOnConnect":false,` +
		`"firstRunDone":true,"closeNoticeShown":false,"apps":["com.google.android.youtube"],` +
		`"advanced":{"dohServers":[],"domains":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(v2Typed), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"com.google.android.youtube"}
	if !reflect.DeepEqual(s.Apps, want) {
		t.Errorf("apps = %v, want the hand-typed list kept verbatim: %v", s.Apps, want)
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
