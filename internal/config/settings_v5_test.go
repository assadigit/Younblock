package config

// The M8-z settings-v5 shapes (ADR-0024 work item A + ADR-0027 §8.1),
// pinned by test:
//
//   1. fragmentAllRouted defaults TRUE on Android (fresh install and
//      pre-v5 file alike — the absent field was never a choice the
//      file recorded) and FALSE on every desktop (hard constraint 5:
//      desktop behavior unchanged).
//   2. A v5 file records the owner's word: an explicit false on
//      Android stays false (the setting is a real setting, not a
//      constant).
//   3. The debug-tester LogHostnames default: ONLY a
//      DebugTesterBuild on Android turns logDomains on for the shipped
//      state; a release build (the flag's default) never does — R21
//      keeps hostnames off in every release shape.
//
// Both-ways proof: with applyPlatformDefaults stashed, the Android
// fresh-install test FAILS (fragmentAllRouted false) and the debug
// hostnames test FAILS; the desktop and v5-file guards pass on both
// sides.

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSettingsFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestFragmentAllRoutedAndroidDefault (shape 1, the fix's core):
// fresh install on Android starts fragmented-all; the desktop does
// not. Pre-fix (no platform-aware load) both load false.
func TestFragmentAllRoutedAndroidDefault(t *testing.T) {
	androidDir := t.TempDir()
	desktopDir := t.TempDir()

	sa, _, err := LoadFor(androidDir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !sa.FragmentAllRouted {
		t.Fatal("fresh install on Android must default fragmentAllRouted to true (ADR-0024)")
	}
	sd, _, err := LoadFor(desktopDir, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if sd.FragmentAllRouted {
		t.Fatal("fresh install on a desktop must keep fragmentAllRouted off (hard constraint 5)")
	}
	// The plain Load (the desktop path) is platform-blind: off.
	s2, _, err := Load(desktopDir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.FragmentAllRouted {
		t.Fatal("config.Load must stay platform-blind (desktop editions)")
	}
}

// TestFragmentAllRoutedV4FileMigratesOnAndroid (shape 1, the
// upgrade path): a v4 file (no fragmentAllRouted field ever written)
// loads on Android with the flag on, on desktop off. The absent field
// was the shipped default of its era, never a choice.
func TestFragmentAllRoutedV4FileMigratesOnAndroid(t *testing.T) {
	const v4 = `{
  "version": 4,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "apps": ["com.google.android.youtube"]
}`
	androidDir := t.TempDir()
	writeSettingsFile(t, androidDir, v4)
	sa, _, err := LoadFor(androidDir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !sa.FragmentAllRouted {
		t.Fatal("a pre-v5 file on Android must migrate fragmentAllRouted to true")
	}
	if sa.Version != settingsVersion {
		t.Fatalf("migrated version = %d, want %d", sa.Version, settingsVersion)
	}

	desktopDir := t.TempDir()
	writeSettingsFile(t, desktopDir, v4)
	sd, _, err := LoadFor(desktopDir, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if sd.FragmentAllRouted {
		t.Fatal("a pre-v5 file on desktop must stay fragmentAllRouted off")
	}
}

// TestFragmentAllRoutedV5FileRespectsExplicitChoice (shape 2,
// guard): a v5 file with fragmentAllRouted=false on Android is the
// owner's word — it survives verbatim. Passes on both sides.
func TestFragmentAllRoutedV5FileRespectsExplicitChoice(t *testing.T) {
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 5,
  "mode": "youtube",
  "method": "auto",
  "fragmentAllRouted": false
}`)
	s, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if s.FragmentAllRouted {
		t.Fatal("a v5 file's explicit false is the owner's choice; the migration must not override it")
	}
}

// TestDebugTesterBuildLogHostnamesDefault (shape 3, ADR-0027 §8.1):
// ONLY the debug-tester lane turns logDomains on for Android's
// shipped state; the release build (DebugTesterBuild=false, the
// default) keeps hostnames off — R21.
func TestDebugTesterBuildLogHostnamesDefault(t *testing.T) {
	old := DebugTesterBuild
	defer func() { DebugTesterBuild = old }()

	// Release build (the default, the empty ldflags value): hostnames
	// stay off.
	DebugTesterBuild = ""
	dir := t.TempDir()
	s, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if s.Advanced.LogDomains {
		t.Fatal("a release build must keep LogHostnames off by default (R21)")
	}

	// Debug tester build (the -ldflags -X lane): hostnames on for the
	// fresh shipped state.
	DebugTesterBuild = "true"
	dir = t.TempDir()
	s, _, err = LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Advanced.LogDomains {
		t.Fatal("the debug tester build must turn LogHostnames on by default on Android")
	}
	// ... and only on Android: the debug flag never touches desktops.
	dir = t.TempDir()
	s, _, err = LoadFor(dir, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if s.Advanced.LogDomains {
		t.Fatal("the debug tester flag must not turn hostnames on for desktop editions")
	}

	// An owner's saved logDomains survives verbatim in every build:
	// the v5 migration never rewrites a recorded choice.
	DebugTesterBuild = ""
	dir = t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 5,
  "mode": "youtube",
  "advanced": {"logDomains": true}
}`)
	s, _, err = LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Advanced.LogDomains {
		t.Fatal("a saved logDomains=true must survive a release build's load")
	}
}

// TestDebugBuildConstantsPinTheSpec (guards): the shipped knobs carry
// the spec's numbers.
func TestDebugBuildConstantsPinTheSpec(t *testing.T) {
	if PlainFirstByteTimeout != 8*1e9 {
		t.Errorf("PlainFirstByteTimeout = %v, want 8s", PlainFirstByteTimeout)
	}
	if DoHHedgeDelay != 300*1e6 {
		t.Errorf("DoHHedgeDelay = %v, want 300ms", DoHHedgeDelay)
	}
	if DoHReadIdleTimeout != 15*1e9 || DoHPingTimeout != 5*1e9 {
		t.Errorf("DoH keepalive = %v/%v, want 15s/5s", DoHReadIdleTimeout, DoHPingTimeout)
	}
	if GoogleVideoCodeCap != 64 || GoogleVideoPrewarmEvery != 5*60*1e9 || GoogleVideoPrewarmInflight != 3 {
		t.Errorf("googlevideo prewarm knobs = %d/%v/%d, want 64/5m/3", GoogleVideoCodeCap, GoogleVideoPrewarmEvery, GoogleVideoPrewarmInflight)
	}
	if DebugTesterBuild != "" {
		t.Error(`DebugTesterBuild must default to "" — the release value (the debug lane sets it via ldflags)`)
	}
	if IsDebugTesterBuild() {
		t.Error("IsDebugTesterBuild must be false for the empty default")
	}
	if !func() bool {
		DebugTesterBuild = "true"
		defer func() { DebugTesterBuild = "" }()
		return IsDebugTesterBuild()
	}() {
		t.Error(`IsDebugTesterBuild must be true for "true"`)
	}
}
