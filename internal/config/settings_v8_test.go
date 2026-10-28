package config

// The M8-am settings-v8 shapes (ADR-0051, D105), pinned by test:
//
//   1. autoOpenYouTube defaults TRUE on Android (the owner's
//      checked-by-default ruling): a fresh install lands on it.
//   2. The v8 migration carries every pre-v8 Android file to true —
//      the field was absent from every file the previous rounds ever
//      wrote, so its false there is the shipped default, never a
//      decision the file's owner made (the v5 fragmentAllRouted
//      pattern, verbatim). The desktops keep the field off: nothing
//      there reads it (no YouTube app to open).
//   3. A v8 file records the owner's explicit choice either way —
//      false is the switch doing its job, never re-flipped.
//   4. Save persists the field (the round-trip).
//
// Both-ways proof: with the v8 migration stashed, shape 2's pre-v8
// leg FAILS (loads false); with applyPlatformDefaults' line stashed,
// shape 1's fresh-install leg FAILS; with the version bump stashed,
// the migrated file reads 7 and shape 2's version assertion FAILS.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAutoOpenYouTubeAndroidDefaultOn (shape 1, the ruling): the fresh
// install on Android opens the hand-off; the fresh desktop install and
// DefaultSettings keep it off (the desktop-inert value).
func TestAutoOpenYouTubeAndroidDefaultOn(t *testing.T) {
	sa, _, err := LoadFor(t.TempDir(), "android")
	if err != nil {
		t.Fatal(err)
	}
	if !sa.AutoOpenYouTube {
		t.Fatal("fresh install on Android must default autoOpenYouTube to true (M8-am, D105: checked by default)")
	}
	sd, _, err := LoadFor(t.TempDir(), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if sd.AutoOpenYouTube {
		t.Fatal("fresh install on a desktop must keep autoOpenYouTube off (nothing there reads it)")
	}
	d := DefaultSettings()
	if d.AutoOpenYouTube {
		t.Fatal("DefaultSettings must carry the desktop-inert false; LoadFor applies Android's true")
	}
}

// TestAutoOpenYouTubeV8MigrationAndOwnersWord (shapes 2 and 3): the
// pre-v8 Android file migrates to true, the desktop file does not, and
// a v8 file's explicit choice rides verbatim.
func TestAutoOpenYouTubeV8MigrationAndOwnersWord(t *testing.T) {
	// A v7 file (every debug.29-era file): the field was never written,
	// so its absent false is the shipped default — Android migrates to
	// the ruling's true.
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 7,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "apps": ["com.google.android.youtube"]
}`)
	migrated, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !migrated.AutoOpenYouTube {
		t.Fatal("a pre-v8 Android file must migrate autoOpenYouTube to true (the field was never a choice)")
	}
	// A pre-v8 file migrates through every later schema too (D123's v9
	// routingTarget migration runs for every file below v9); the version
	// lands on the current schema version, not 8.
	if migrated.Version != settingsVersion {
		t.Fatalf("migrated version = %d, want %d", migrated.Version, settingsVersion)
	}

	// The same v7 file on a desktop stays off (inert there) — and still
	// normalizes its version to the current schema like every file.
	dirD := t.TempDir()
	writeSettingsFile(t, dirD, `{
  "version": 7,
  "mode": "youtube",
  "method": "auto",
  "port": 4525
}`)
	desktop, _, err := LoadFor(dirD, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if desktop.AutoOpenYouTube {
		t.Fatal("a desktop file must not gain the hand-off (nothing there reads it)")
	}

	// A v8 file's explicit false IS the owner's word and rides — the
	// switch stays a real switch on both sides.
	dir2 := t.TempDir()
	writeSettingsFile(t, dir2, `{
  "version": 8,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "autoOpenYouTube": false
}`)
	ownersWord, _, err := LoadFor(dir2, "android")
	if err != nil {
		t.Fatal(err)
	}
	if ownersWord.AutoOpenYouTube {
		t.Fatal("a v8 file's explicit autoOpenYouTube=false must ride verbatim (the owner's word)")
	}

	// And the explicit true at v8 rides as true (not re-derived).
	dir3 := t.TempDir()
	writeSettingsFile(t, dir3, `{
  "version": 8,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "autoOpenYouTube": true
}`)
	ownersOn, _, err := LoadFor(dir3, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !ownersOn.AutoOpenYouTube {
		t.Fatal("a v8 file's explicit autoOpenYouTube=true must ride verbatim")
	}
}

// TestAutoOpenYouTubeRoundTrip (shape 4): the field survives LoadFor →
// Save → re-read with its value intact, and the written file carries
// the key (the UI's full-document saves round-trip it untouched).
func TestAutoOpenYouTubeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !s.AutoOpenYouTube {
		t.Fatal("the round-trip's start must be the Android default true")
	}
	s.AutoOpenYouTube = false
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if v, ok := raw["autoOpenYouTube"].(bool); !ok || v {
		t.Fatalf("settings.json must record the owner's false autoOpenYouTube: %s", data)
	}
	back, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if back.AutoOpenYouTube {
		t.Fatal("the saved false did not survive the round-trip")
	}
	if back.Version != settingsVersion {
		t.Fatalf("round-tripped version = %d, want %d", back.Version, settingsVersion)
	}
}
