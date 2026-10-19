package stats

// hoststats.go — the per-host flow table behind the report's "Top 10
// slowest or failed hosts" section (M8-z, ADR-0027 §8.3.1). The proxy
// notes every tunnel flow, first byte, and terminal failure per host
// and class; the report renders the slowest-or-failed cohort.
//
// PRIVACY: the table lives in MEMORY ONLY (like the log ring, R21's
// territory), is bounded (maxHosts, least-recent evictions), and the
// report prints the host NAMES only when the LogHostnames setting is
// on — the release build's report renders the same rows with
// host=<redacted> so the numbers stay comparable without leaking any
// name.
//
// There are deliberately no per-IP maps here (the package's own rule):
// a host row is keyed by the CONNECT/SOCKS hostname the doors already
// decided routing on.

import (
	"sort"
	"sync"
	"time"
)

// maxHosts caps the per-host table: least-recently-touched entries are
// evicted first (the report only ever shows the worst 10 anyway).
const maxHosts = 256

// HostStat is one host's row.
type HostStat struct {
	Host           string
	Class          string // "routed" (budgeted) or "plain"
	Flows          int64
	Failures       int64
	PlainKills     int64 // the 8 s safety net's closes (subset of Failures)
	WorstFirstByte int64 // ms; 0 = never delivered a first byte
	LastSeen       time.Time
}

// HostTable is the bounded, mutex-guarded per-host flow table. A nil
// *HostTable is valid and silently drops every note (the proxy's
// Config.Hosts is nil in the unit tests that do not assert on it).
type HostTable struct {
	mu    sync.Mutex
	hosts map[string]*HostStat
}

// NewHostTable builds an empty table.
func NewHostTable() *HostTable {
	return &HostTable{hosts: make(map[string]*HostStat)}
}

// row returns (creating when needed) the host's row; caller holds mu.
func (t *HostTable) rowLocked(host, class string, now time.Time) *HostStat {
	if r, ok := t.hosts[host]; ok {
		// The class is per-flow; the row keeps the latest seen.
		if class != "" {
			r.Class = class
		}
		r.LastSeen = now
		return r
	}
	if len(t.hosts) >= maxHosts {
		// Evict the least recently touched entry.
		oldest := ""
		var oldestAt time.Time
		for h, r := range t.hosts {
			if oldest == "" || r.LastSeen.Before(oldestAt) {
				oldest, oldestAt = h, r.LastSeen
			}
		}
		if oldest != "" {
			delete(t.hosts, oldest)
		}
	}
	r := &HostStat{Host: host, Class: class, LastSeen: now}
	if r.Class == "" {
		r.Class = "plain"
	}
	t.hosts[host] = r
	return r
}

func (t *HostTable) note(host, class string, mut func(*HostStat)) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	r := t.rowLocked(host, class, time.Now())
	if mut != nil {
		mut(r)
	}
}

// NoteFlow records one established tunnel to host (class routed/plain).
func (t *HostTable) NoteFlow(host, class string) {
	t.note(host, class, func(r *HostStat) { r.Flows++ })
}

// NoteFirstByte records a delivered first byte and its latency in ms.
func (t *HostTable) NoteFirstByte(host, class string, ms int64) {
	t.note(host, class, func(r *HostStat) {
		if ms > r.WorstFirstByte {
			r.WorstFirstByte = ms
		}
	})
}

// NoteFailure records a terminal failure (first-byte budget death, the
// plain safety net's close, a stall kill).
func (t *HostTable) NoteFailure(host, class string) {
	t.note(host, class, func(r *HostStat) { r.Failures++ })
}

// NotePlainKill records a plain-relay safety-net close (ADR-0024 item B).
func (t *HostTable) NotePlainKill(host, class string) {
	t.note(host, class, func(r *HostStat) {
		r.Failures++
		r.PlainKills++
	})
}

// PlainSummary is the plain-relay section's aggregate (ADR-0027 §8.3.2).
type PlainSummary struct {
	Flows        int64
	Kills        int64
	WorstFirstMS int64
}

// Plain aggregates the class=plain cohort.
func (t *HostTable) Plain() PlainSummary {
	if t == nil {
		return PlainSummary{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var s PlainSummary
	for _, r := range t.hosts {
		if r.Class != "plain" {
			continue
		}
		s.Flows += r.Flows
		s.Kills += r.PlainKills
		if r.WorstFirstByte > s.WorstFirstMS {
			s.WorstFirstMS = r.WorstFirstByte
		}
	}
	return s
}

// TopSlowest returns the n hosts with the worst "slowest or failed"
// profile: failures first (desc), then worst first-byte (desc), then
// flow count (desc). Hosts with no failure and no first byte rank last.
func (t *HostTable) TopSlowest(n int) []HostStat {
	if t == nil || n <= 0 {
		return nil
	}
	t.mu.Lock()
	rows := make([]HostStat, 0, len(t.hosts))
	for _, r := range t.hosts {
		rows = append(rows, *r)
	}
	t.mu.Unlock()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Failures != b.Failures {
			return a.Failures > b.Failures
		}
		if a.WorstFirstByte != b.WorstFirstByte {
			return a.WorstFirstByte > b.WorstFirstByte
		}
		return a.Flows > b.Flows
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// Len reports the table size (tests).
func (t *HostTable) Len() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.hosts)
}
