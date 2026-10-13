// Package fragment splits the first TLS ClientHello into small pieces
// (file 01, section 5). Plan is a pure function with an injectable random
// source so tests are deterministic.
package fragment

import (
	"math/rand"
	"sync"
	"time"
)

// Preset selects how the first message is split.
type Preset string

const (
	// Light sends few, large pieces.
	Light Preset = "light"
	// Balanced is the default first try.
	Balanced Preset = "balanced"
	// Strong sends smaller pieces.
	Strong Preset = "strong"
	// Legacy reproduces the old Python rule exactly.
	Legacy Preset = "legacy"
)

// Valid reports whether p is a known preset.
func (p Preset) Valid() bool {
	return p == Light || p == Balanced || p == Strong || p == Legacy
}

// Config controls one fragmentation run.
type Config struct {
	// Min and Max are the random piece-size bounds for the chunk presets.
	Min int
	// Max is also the largest non-final piece for the chunk presets.
	Max int
	// Pause is the sleep between pieces.
	Pause time.Duration
}

// PresetConfig returns the built-in Config for a preset.
func PresetConfig(p Preset) Config {
	switch p {
	case Light:
		return Config{Min: 150, Max: 300, Pause: 1 * time.Millisecond}
	case Strong:
		return Config{Min: 20, Max: 80, Pause: 10 * time.Millisecond}
	case Legacy:
		// The Python rule: up to 299 random cut points, 1 ms pause, any sizes.
		return Config{Min: 0, Max: 0, Pause: 1 * time.Millisecond}
	default: // balanced
		return Config{Min: 80, Max: 200, Pause: 5 * time.Millisecond}
	}
}

// RNG is the randomness source; *rand.Rand satisfies it.
type RNG interface {
	Intn(n int) int
}

// SafeRNG makes any RNG safe for concurrent use.
type SafeRNG struct {
	mu sync.Mutex
	r  RNG
}

// NewSafeRNG wraps an RNG (for example *rand.Rand) for concurrent use.
func NewSafeRNG(r RNG) *SafeRNG { return &SafeRNG{r: r} }

// Intn satisfies RNG.
func (s *SafeRNG) Intn(n int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.r.Intn(n)
}

// NewDefaultRNG returns a concurrency-safe, time-seeded source for production.
func NewDefaultRNG() *SafeRNG {
	return NewSafeRNG(rand.New(rand.NewSource(time.Now().UnixNano())))
}

// legacyThreshold is the length below which the legacy rule sends whole.
const legacyThreshold = 300

// legacyMaxCuts is the maximum number of random cut points (the Python used 299).
const legacyMaxCuts = 299

// Plan returns the piece lengths for a message of exactly length bytes.
//
// Rules (file 01, section 5):
//   - Pieces are in order, lengths sum exactly to length, none empty.
//   - For length >= 2 there are at least 2 pieces.
//   - No piece exceeds the preset maximum except possibly the last.
//   - legacy reproduces the Python rule: length < 300 sends whole, otherwise
//     min(299, length-2) sorted random cut points in 1..length-2.
func Plan(length int, p Preset, rng RNG, cfg Config) []int {
	if length <= 0 {
		return nil
	}
	if rng == nil {
		rng = NewDefaultRNG()
	}
	if p == Legacy {
		return planLegacy(length, rng)
	}
	if cfg.Min < 1 {
		cfg.Min = 1
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	return planChunks(length, cfg, rng)
}

func planChunks(length int, cfg Config, rng RNG) []int {
	var pieces []int
	rem := length
	for rem > cfg.Max {
		size := cfg.Min + rng.Intn(cfg.Max-cfg.Min+1)
		pieces = append(pieces, size)
		rem -= size
	}
	if len(pieces) == 0 {
		if length < 2 {
			return []int{length}
		}
		// Small message: split in two non-empty halves.
		first := length / 2
		return []int{first, length - first}
	}
	if rem > 0 {
		pieces = append(pieces, rem)
	}
	return pieces
}

func planLegacy(length int, rng RNG) []int {
	if length < legacyThreshold {
		return []int{length}
	}
	// Cut points are distinct values in [1, length-2].
	available := length - 2
	k := legacyMaxCuts
	if available < k {
		k = available
	}
	// Partial Fisher-Yates over 1..length-2 using only Intn.
	vals := make([]int, available)
	for i := range vals {
		vals[i] = i + 1
	}
	cuts := make([]int, k)
	for i := 0; i < k; i++ {
		j := i + rng.Intn(available-i)
		vals[i], vals[j] = vals[j], vals[i]
		cuts[i] = vals[i]
	}
	sortInts(cuts)
	pieces := make([]int, 0, k+1)
	prev := 0
	for _, c := range cuts {
		pieces = append(pieces, c-prev)
		prev = c
	}
	pieces = append(pieces, length-prev)
	return pieces
}

func sortInts(a []int) {
	// Insertion sort: k is at most 299, and this keeps the package free of
	// pulling in sort for a tiny, deterministic job.
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// MeasureSleep returns how long a 1 ms sleep really takes on this machine
// (file 01, section 5: Windows can round short sleeps up to ~15 ms).
func MeasureSleep() time.Duration {
	start := time.Now()
	time.Sleep(time.Millisecond)
	return time.Since(start)
}
