package fragment

import (
	"math/rand"
	"testing"
	"time"
)

func seededRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

func checkPieces(t *testing.T, length int, pieces []int, maxPiece int) {
	t.Helper()
	sum := 0
	for i, p := range pieces {
		if p <= 0 {
			t.Fatalf("piece %d is empty (length %d, pieces %v)", i, length, pieces)
		}
		if i < len(pieces)-1 && maxPiece > 0 && p > maxPiece {
			t.Fatalf("piece %d = %d exceeds max %d (pieces %v)", i, p, maxPiece, pieces)
		}
		sum += p
	}
	if sum != length {
		t.Fatalf("pieces sum to %d, want %d (%v)", sum, length, pieces)
	}
	if length >= 2 && len(pieces) < 2 {
		t.Fatalf("length %d produced %d pieces, want >= 2 (%v)", length, len(pieces), pieces)
	}
}

func TestPlanChunksAllPresetsAndLengths(t *testing.T) {
	presets := []Preset{Light, Balanced, Strong}
	lengths := []int{1, 2, 3, 5, 79, 80, 81, 199, 200, 201, 300, 519, 520, 521, 4096, 16389}
	for _, p := range presets {
		cfg := PresetConfig(p)
		for _, l := range lengths {
			pieces := Plan(l, p, seededRand(int64(l)+42), cfg)
			checkPieces(t, l, pieces, cfg.Max)
		}
	}
}

func TestPlanPieceCountsMatchSpecTable(t *testing.T) {
	// File 01 section 5, for a ~520-byte hello: light 2-4, balanced 3-7, strong 7-26.
	cases := []struct {
		preset Preset
		lo, hi int
	}{
		{Light, 2, 4},
		{Balanced, 3, 7},
		{Strong, 7, 26},
	}
	for _, tc := range cases {
		minN, maxN := 100, 0
		for seed := int64(0); seed < 200; seed++ {
			pieces := Plan(520, tc.preset, seededRand(seed), PresetConfig(tc.preset))
			checkPieces(t, 520, pieces, PresetConfig(tc.preset).Max)
			if n := len(pieces); n < minN {
				minN = n
			} else if n > maxN {
				maxN = n
			}
		}
		if minN < tc.lo || maxN > tc.hi {
			t.Errorf("preset %s piece count range = %d-%d, want %d-%d", tc.preset, minN, maxN, tc.lo, tc.hi)
		}
	}
}

func TestPlanLegacyReproducesPythonRule(t *testing.T) {
	// Below 300: one piece.
	if got := Plan(299, Legacy, seededRand(1), PresetConfig(Legacy)); len(got) != 1 || got[0] != 299 {
		t.Errorf("legacy(299) = %v, want [299]", got)
	}
	// 520 bytes: up to 299 cut points, up to 300 segments (see DECISIONS.md D8).
	for seed := int64(0); seed < 100; seed++ {
		pieces := Plan(520, Legacy, seededRand(seed), PresetConfig(Legacy))
		checkPieces(t, 520, pieces, 0)
		if len(pieces) > 300 {
			t.Fatalf("legacy(520) produced %d pieces, want <= 300", len(pieces))
		}
	}
	// Very small available range: length 301 -> available cut positions 1..299,
	// so at most 299 cuts.
	pieces := Plan(301, Legacy, seededRand(7), PresetConfig(Legacy))
	if len(pieces) > 300 {
		t.Errorf("legacy(301) produced %d pieces, want <= 300", len(pieces))
	}
	checkPieces(t, 301, pieces, 0)
	// Exactly 300 bytes: the Python crashes here; ours clamps (DECISIONS.md D8).
	pieces = Plan(300, Legacy, seededRand(7), PresetConfig(Legacy))
	checkPieces(t, 300, pieces, 0)
	if len(pieces) > 299 { // 298 available positions + final segment
		t.Errorf("legacy(300) produced %d pieces, want <= 299", len(pieces))
	}
}

func TestPlanDeterministicWithSeededSource(t *testing.T) {
	a := Plan(2000, Balanced, seededRand(99), PresetConfig(Balanced))
	b := Plan(2000, Balanced, seededRand(99), PresetConfig(Balanced))
	if len(a) != len(b) {
		t.Fatalf("different piece counts %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("piece %d differs: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestPlanZeroAndOne(t *testing.T) {
	if got := Plan(0, Balanced, seededRand(1), PresetConfig(Balanced)); got != nil {
		t.Errorf("Plan(0) = %v, want nil", got)
	}
	if got := Plan(1, Balanced, seededRand(1), PresetConfig(Balanced)); len(got) != 1 || got[0] != 1 {
		t.Errorf("Plan(1) = %v, want [1]", got)
	}
	if got := Plan(2, Strong, seededRand(1), PresetConfig(Strong)); len(got) != 2 {
		t.Errorf("Plan(2) = %v, want 2 pieces", got)
	}
}

func TestPlanOverrideBounds(t *testing.T) {
	cfg := Config{Min: 50, Max: 60, Pause: time.Millisecond}
	for seed := int64(0); seed < 50; seed++ {
		pieces := Plan(1000, Balanced, seededRand(seed), cfg)
		checkPieces(t, 1000, pieces, 60)
	}
}

func TestSafeRNGConcurrency(t *testing.T) {
	r := NewSafeRNG(seededRand(5))
	done := make(chan int)
	for g := 0; g < 4; g++ {
		go func() {
			for i := 0; i < 100; i++ {
				_ = r.Intn(100)
			}
			done <- 1
		}()
	}
	for g := 0; g < 4; g++ {
		<-done
	}
}
