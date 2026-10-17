// usage_bridge_test.go — the round-3 usage fold-in on the Bridge: an
// engine that implements UsageSource gets its counters carried in every
// connected snapshot (and the last finished session after a disconnect),
// and a plain engine gets the honest -1/-1 that hides the lines.
package main

import (
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/usage"
)

// countingEngine is a fakeEngine that also implements UsageSource.
type countingEngine struct {
	fakeEngine
	mu        sync.Mutex
	downBytes int64
	upBytes   int64
	last      *usage.SessionRecord
}

func (c *countingEngine) Usage() (int64, int64, *usage.SessionRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rec := c.last
	if rec != nil {
		copy := *rec
		rec = &copy
	}
	return c.downBytes, c.upBytes, rec
}

func (c *countingEngine) set(down, up int64, last *usage.SessionRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.downBytes, c.upBytes, c.last = down, up, last
}

func TestBridgeCarriesUsageWhenEngineCounts(t *testing.T) {
	eng := &countingEngine{fakeEngine: fakeEngine{down: 100, up: 5, ping: 40}}
	b := NewBridge(eng, nil)

	// Fresh: the counting engine reports 0/0 and no last session — real
	// zeros, not the unknown -1 (the screen hides the lines anyway until
	// a session is live).
	if s := b.GetSnapshot(); s.DownBytes != 0 || s.UpBytes != 0 || s.LastSession != nil {
		t.Fatalf("fresh snapshot = %+v, want 0/0/nil", s)
	}

	if err := b.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitFor(t, time.Second, func() bool { return b.GetSnapshot().Phase == PhaseConnected })

	eng.set(2_100_000_000, 300_000_000, nil)
	b.refreshStats()
	s := b.GetSnapshot()
	if s.DownBytes != 2_100_000_000 || s.UpBytes != 300_000_000 {
		t.Fatalf("connected snapshot usage = %d/%d, want 2100000000/300000000", s.DownBytes, s.UpBytes)
	}

	// The engine ends the session: Usage() now reports the finished record.
	last := &usage.SessionRecord{StartMs: 1, EndMs: 2, DownBytes: 2_100_000_000, UpBytes: 300_000_000}
	eng.set(2_100_000_000, 300_000_000, last)
	if err := b.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	s = b.GetSnapshot()
	if s.LastSession == nil || s.LastSession.DownBytes != 2_100_000_000 {
		t.Fatalf("disconnected snapshot lastSession = %+v, want the finished record", s.LastSession)
	}

	// A cleared history (the engine drops its last record) reaches the
	// screen through RefreshUsage — the App.ClearUsage path.
	eng.set(-1, -1, nil)
	b.RefreshUsage()
	if s := b.GetSnapshot(); s.LastSession != nil {
		t.Fatalf("lastSession after a clear = %+v, want nil", s.LastSession)
	}
}

func TestBridgeSeedsLastSessionAtConstruction(t *testing.T) {
	// A restarted app: the engine already knows the last finished session
	// (seeded from the history file), and the bridge's FIRST snapshot —
	// the one the screen reads on load — must carry it.
	eng := &countingEngine{fakeEngine: fakeEngine{}}
	eng.set(0, 0, &usage.SessionRecord{StartMs: 1, EndMs: 2, DownBytes: 700, UpBytes: 30})
	b := NewBridge(eng, nil)
	s := b.GetSnapshot()
	if s.LastSession == nil || s.LastSession.DownBytes != 700 {
		t.Fatalf("fresh snapshot lastSession = %+v, want the seeded record", s.LastSession)
	}
}

func TestBridgeHidesUsageWhenEngineCannotCount(t *testing.T) {
	eng := &fakeEngine{down: 100, up: 5, ping: 40} // no UsageSource
	b := NewBridge(eng, nil)

	if err := b.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitFor(t, time.Second, func() bool { return b.GetSnapshot().Phase == PhaseConnected })
	b.refreshStats()
	s := b.GetSnapshot()
	if s.DownBytes != -1 || s.UpBytes != -1 || s.LastSession != nil {
		t.Fatalf("snapshot usage = %d/%d/%+v, want the honest -1/-1/nil", s.DownBytes, s.UpBytes, s.LastSession)
	}
}

func TestBridgeUsageZeroValuesMarshalExplicit(t *testing.T) {
	// A connected snapshot must carry downBytes/upBytes as real values
	// (0 is honest at session start), never as the unknown -1.
	eng := &countingEngine{fakeEngine: fakeEngine{}}
	b := NewBridge(eng, nil)
	if err := b.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitFor(t, time.Second, func() bool { return b.GetSnapshot().Phase == PhaseConnected })
	eng.set(0, 0, nil)
	b.refreshStats()
	s := b.GetSnapshot()
	if s.DownBytes != 0 || s.UpBytes != 0 {
		t.Fatalf("zero-byte session = %d/%d, want 0/0 (not -1)", s.DownBytes, s.UpBytes)
	}
}
