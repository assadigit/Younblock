package config

// The M8-aj settings-v7 shapes (ADR-0049), pinned by test:
//
//   1. The substitution experiment RETIRES: DefaultSettings, a fresh
//      install, and a pre-v6 file all land on
//      experimentSubstitution=false / experimentBreaker=true — the
//      debug.27 field verdict (37 substituted wins, big=0, the player
//      pinned to the blackholed cluster's URL set, no video opened,
//      the cluster never marked dead because substituted wins never
//      terminal) concluded the four-round experiment.
//   2. The v7 migration retires existing files: a v6 file's
//      experimentSubstitution=true is the shipped default verbatim —
//      never a choice its owner made — and flips OFF. A v7 file's
//      explicit true IS the owner's word and rides verbatim (the
//      toggle stays a real switch).
//
// Both-ways proof: with the v7 migration stashed, shape 2's v6-file
// leg FAILS (loads true); with the default flip stashed, shape 1
// FAILS on the on/on default; with the version bump stashed, the
// migrated version reads 6 and shape 2's owner's-word leg FAILS (the
// v7 file gets migrated to false like everyone else).

import (
	"testing"
)

// TestExperimentSwitchesRetiredOff (shape 1, fresh + migrated + default):
func TestExperimentSwitchesRetiredOff(t *testing.T) {
	d := DefaultSettings()
	if d.Advanced.ExperimentSubstitution || !d.Advanced.ExperimentBreaker {
		t.Fatalf("DefaultSettings switches = %v/%v, want off/on (M8-aj, ADR-0049: the experiment concluded)", d.Advanced.ExperimentSubstitution, d.Advanced.ExperimentBreaker)
	}

	// A fresh install (no file).
	fresh, _, err := LoadFor(t.TempDir(), "android")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Advanced.ExperimentSubstitution || !fresh.Advanced.ExperimentBreaker {
		t.Fatalf("fresh install switches = %v/%v, want off/on", fresh.Advanced.ExperimentSubstitution, fresh.Advanced.ExperimentBreaker)
	}

	// A pre-v6 file: the fields were absent, never chosen — the v6
	// migration flips both on, then the v7 migration retires
	// substitution (the field verdict), the breaker stays.
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
	if migrated.Advanced.ExperimentSubstitution || !migrated.Advanced.ExperimentBreaker {
		t.Fatalf("pre-v6 file switches = %v/%v, want off/on (the v7 retirement)", migrated.Advanced.ExperimentSubstitution, migrated.Advanced.ExperimentBreaker)
	}
	if migrated.Version != 7 {
		t.Fatalf("migrated version = %d, want 7", migrated.Version)
	}
}

// TestExperimentSwitchesV7MigrationAndOwnersWord (shape 2): the v6
// default retires; a v7 owner's explicit word rides.
func TestExperimentSwitchesV7MigrationAndOwnersWord(t *testing.T) {
	// A v6 file carrying the shipped-on default (every v6 writer
	// recorded the whole struct verbatim) — flipped OFF by the
	// migration: the field's true was never a choice.
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{
  "version": 6,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "advanced": {"experimentSubstitution": true, "experimentBreaker": true}
}`)
	s, _, err := LoadFor(dir, "android")
	if err != nil {
		t.Fatal(err)
	}
	if s.Advanced.ExperimentSubstitution || !s.Advanced.ExperimentBreaker {
		t.Fatalf("v6 shipped-on file switches = %v/%v, want off/on (the v7 retirement: the default, never a choice)", s.Advanced.ExperimentSubstitution, s.Advanced.ExperimentBreaker)
	}

	// A v7 file whose owner re-enabled the lane at v7 — the owner's
	// word, verbatim.
	dir2 := t.TempDir()
	writeSettingsFile(t, dir2, `{
  "version": 7,
  "mode": "youtube",
  "method": "auto",
  "port": 4525,
  "advanced": {"experimentSubstitution": true, "experimentBreaker": true}
}`)
	owner, _, err := LoadFor(dir2, "android")
	if err != nil {
		t.Fatal(err)
	}
	if !owner.Advanced.ExperimentSubstitution || !owner.Advanced.ExperimentBreaker {
		t.Fatalf("v7 owner's-word switches = %v/%v, want on/on (a v7 file's explicit true rides)", owner.Advanced.ExperimentSubstitution, owner.Advanced.ExperimentBreaker)
	}
}

// TestExperimentNodesFileShipsEmpty (the W8 shape, unchanged): the
// embedded foreign-cluster file carries only its comment header — the
// loader returns nothing and the network test's foreign lane skips
// silently.
func TestExperimentNodesFileShipsEmpty(t *testing.T) {
	nodes := ExperimentNodes()
	if len(nodes) != 0 {
		t.Fatalf("ExperimentNodes = %v, want empty (the file ships with only its comment header)", nodes)
	}
}
