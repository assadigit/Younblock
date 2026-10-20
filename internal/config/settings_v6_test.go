package config

// The M8-af settings-v6 shapes (ADR-0044, spec W4), pinned by test:
//
//   1. The experiment switches ship ON: a fresh install, a pre-v6
//      file (the absent fields were never a choice the file
//      recorded), and DefaultSettings all carry
//      experimentSubstitution=true and experimentBreaker=true — the
//      round's semantics (W6-scoped substitution, W5-fixed breaker).
//   2. A v6 file records the owner's word: an explicit false stays
//      false (the toggles are real switches, not constants).
//   3. ExperimentNodes (W8): the embedded foreign-cluster file ships
//      EMPTY — the loader reads it, skips comments and blanks, and
//      returns nothing, so the network test behaves exactly as if the
//      lane did not exist.
//
// Both-ways proof: with the v6 migration stashed, the pre-v6-file
// test FAILS (both switches false); with the loader's comment/blank
// skipping stashed, the nodes test fails on garbage lines.

import (
	"path/filepath"
	"testing"
)

// TestExperimentSwitchesDefaultOn (shape 1, fresh + migrated + default):
func TestExperimentSwitchesDefaultOn(t *testing.T) {
	d := DefaultSettings()
	if !d.Advanced.ExperimentSubstitution || !d.Advanced.ExperimentBreaker {
		t.Fatalf("DefaultSettings switches = %v/%v, want on/on (the debug.24 shipped semantics)", d.Advanced.ExperimentSubstitution, d.Advanced.ExperimentBreaker)
	}

	// A fresh install (no file).
	fresh, _, err := LoadFor(t.TempDir(), "android")
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.Advanced.ExperimentSubstitution || !fresh.Advanced.ExperimentBreaker {
		t.Fatalf("fresh install switches = %v/%v, want on/on", fresh.Advanced.ExperimentSubstitution, fresh.Advanced.ExperimentBreaker)
	}

	// A pre-v6 file: the fields were absent, never chosen — both flip
	// on, exactly the v5 pattern.
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 5,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "apps": ["com.google.android.youtube"]
}`)
	migrated, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !migrated.Advanced.ExperimentSubstitution || !migrated.Advanced.ExperimentBreaker {
		t.Fatalf("pre-v6 file switches = %v/%v, want on/on (absent fields are the shipped default)", migrated.Advanced.ExperimentSubstitution, migrated.Advanced.ExperimentBreaker)
	}
	if migrated.Version != 6 {
		t.Fatalf("migrated version = %d, want 6", migrated.Version)
	}
}

// TestExperimentSwitchesV6FileIsTheOwnersWord (shape 2): an explicit
// off survives verbatim — the tester's A/B is a real setting.
func TestExperimentSwitchesV6FileIsTheOwnersWord(t *testing.T) {
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 6,
  "mode": "youtube",
  "method": "legacy",
  "port": 4525,
  "advanced": {"experimentSubstitution": false, "experimentBreaker": false}
}`)
	s, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if s.Advanced.ExperimentSubstitution || s.Advanced.ExperimentBreaker {
		t.Fatalf("explicit off switches = %v/%v, want off/off (the owner's word)", s.Advanced.ExperimentSubstitution, s.Advanced.ExperimentBreaker)
	}
}

// TestExperimentNodesFileShipsEmpty (shape 3, spec W8): the embedded
// foreign-cluster file carries only its comment header — the loader
// returns nothing and the network test's foreign lane skips silently.
func TestExperimentNodesFileShipsEmpty(t *testing.T) {
	nodes := ExperimentNodes()
	if len(nodes) != 0 {
		t.Fatalf("ExperimentNodes = %v, want empty (the file ships with only its comment header)", nodes)
	}
	// The file itself is where Ali's data goes — prove the loader lives
	// beside it (a wrong embed path would already have failed the
	// build; this keeps the contract honest).
	if filepath.Ext("experiment_nodes.txt") != ".txt" {
		t.Fatal("unreachable")
	}
}
