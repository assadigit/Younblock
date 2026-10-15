// ui_bridge_test.go — exercises the Bridge with a fake engine: the full
// connect/cancel/disconnect cycle, the error path, stats refresh, and the
// no-op guards. Run with the race detector.
package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeEngine is a scriptable Engine.
type fakeEngine struct {
	mu         sync.Mutex
	started    int
	stopped    int
	startErr   error
	startDelay time.Duration
	blockStart chan struct{} // when set, Start waits for it to be closed
	down       float64
	up         float64
	ping       int
}

func (f *fakeEngine) Start(ctx context.Context) error {
	f.mu.Lock()
	if f.blockStart != nil {
		block := f.blockStart
		f.mu.Unlock()
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		f.mu.Unlock()
		if f.startDelay > 0 {
			t := time.NewTimer(f.startDelay)
			defer t.Stop()
			select {
			case <-t.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	f.mu.Lock()
	f.started++
	err := f.startErr
	f.mu.Unlock()
	return err
}

func (f *fakeEngine) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped++
	return nil
}

func (f *fakeEngine) Stats() (float64, float64, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.down, f.up, f.ping
}

// startedCount / stopCount are race-safe readers for assertions and waits.
func (f *fakeEngine) startedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *fakeEngine) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

// recorder collects every emitted snapshot in order.
type recorder struct {
	mu    sync.Mutex
	snaps []Snapshot
}

func (r *recorder) emit(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snaps = append(r.snaps, s)
}

func (r *recorder) last() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.snaps) == 0 {
		return Snapshot{}
	}
	return r.snaps[len(r.snaps)-1]
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.snaps)
}

// waitFor polls until cond passes or the deadline does.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestBridgeInitialSnapshot(t *testing.T) {
	b := NewBridge(&fakeEngine{}, nil)
	s := b.GetSnapshot()
	if s.Phase != PhaseDisconnected {
		t.Fatalf("initial phase = %q, want disconnected", s.Phase)
	}
	if s.PingMs != -1 {
		t.Fatalf("initial ping = %d, want -1", s.PingMs)
	}
	if s.ConnectedAtMs != 0 {
		t.Fatalf("initial connectedAtMs = %d, want 0", s.ConnectedAtMs)
	}
}

func TestBridgeConnects(t *testing.T) {
	eng := &fakeEngine{down: 125e6, up: 3e6, ping: 42}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	if err := b.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got := rec.last().Phase; got != PhaseConnecting {
		t.Fatalf("phase right after Connect = %q, want connecting", got)
	}

	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnected })
	s := b.GetSnapshot()
	if s.ConnectedAtMs == 0 {
		t.Fatal("connected snapshot has no timestamp")
	}
	if s.DownBps != 125e6 || s.UpBps != 3e6 || s.PingMs != 42 {
		t.Fatalf("stats not copied: %+v", s)
	}
	if eng.started != 1 {
		t.Fatalf("engine started %d times, want 1", eng.started)
	}
	if eng.stopped != 0 {
		t.Fatalf("engine stopped %d times, want 0", eng.stopped)
	}
}

func TestBridgeConnectFails(t *testing.T) {
	eng := &fakeEngine{startErr: errors.New("no route")}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseError })
	s := b.GetSnapshot()
	if s.Message != "Could not connect. Try again." {
		t.Fatalf("error message = %q", s.Message)
	}
	if s.ConnectedAtMs != 0 {
		t.Fatalf("failed connect kept a timestamp: %+v", s)
	}
}

func TestBridgeCancelWhileConnecting(t *testing.T) {
	eng := &fakeEngine{blockStart: make(chan struct{})}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnecting })

	// Cancel before Start finishes.
	if err := b.Disconnect(); err != nil {
		t.Fatalf("Disconnect during connect: %v", err)
	}
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseDisconnected })

	// The late Start result must be ignored (no connected snapshot after
	// the disconnect), but the engine must be stopped to undo it.
	close(eng.blockStart)
	waitFor(t, time.Second, func() bool { return eng.stopCount() >= 1 })

	phases := make([]Phase, 0, rec.count())
	rec.mu.Lock()
	for _, s := range rec.snaps {
		phases = append(phases, s.Phase)
	}
	rec.mu.Unlock()
	for i, p := range phases {
		if i > 0 && p == PhaseConnected {
			t.Fatalf("connected snapshot after cancel: %v", phases)
		}
	}
}

func TestBridgeDisconnectFromError(t *testing.T) {
	eng := &fakeEngine{startErr: errors.New("port busy")}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseError })

	if err := b.Disconnect(); err != nil {
		t.Fatalf("Disconnect from error: %v", err)
	}
	if got := rec.last().Phase; got != PhaseDisconnected {
		t.Fatalf("phase after clearing the error = %q, want disconnected", got)
	}
	if eng.stopped != 0 {
		t.Fatalf("engine stopped %d times from the error state, want 0", eng.stopped)
	}
}

func TestBridgeConnectWhileBusyOrConnectedIsNoop(t *testing.T) {
	eng := &fakeEngine{blockStart: make(chan struct{})}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnecting })
	before := rec.count()
	if err := b.Connect(); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	if rec.count() != before {
		t.Fatal("a second Connect during connecting changed the state")
	}

	close(eng.blockStart)
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnected })
	before = rec.count()
	if err := b.Connect(); err != nil {
		t.Fatalf("Connect while connected: %v", err)
	}
	if rec.count() != before {
		t.Fatal("Connect while connected changed the state")
	}
	if eng.started != 1 {
		t.Fatalf("engine started %d times, want 1", eng.started)
	}
}

func TestBridgeDisconnectIsIdempotent(t *testing.T) {
	eng := &fakeEngine{}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnected })

	if err := b.Disconnect(); err != nil {
		t.Fatalf("first Disconnect: %v", err)
	}
	if err := b.Disconnect(); err != nil {
		t.Fatalf("second Disconnect: %v", err)
	}
	if eng.stopped != 1 {
		t.Fatalf("engine stopped %d times, want 1", eng.stopped)
	}
	if got := rec.last().Phase; got != PhaseDisconnected {
		t.Fatalf("final phase = %q, want disconnected", got)
	}
}

func TestBridgeRefreshStats(t *testing.T) {
	eng := &fakeEngine{down: 1, up: 1, ping: 10}
	rec := &recorder{}
	b := NewBridge(eng, rec.emit)

	_ = b.Connect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseConnected })

	eng.mu.Lock()
	eng.down, eng.up, eng.ping = 500e6, 20e6, 55
	eng.mu.Unlock()

	b.refreshStats()
	s := b.GetSnapshot()
	if s.DownBps != 500e6 || s.UpBps != 20e6 || s.PingMs != 55 {
		t.Fatalf("refreshed stats wrong: %+v", s)
	}
	if got := rec.last().DownBps; got != 500e6 {
		t.Fatalf("refreshed stats not emitted: %v", got)
	}

	// While disconnected the refresh must do nothing.
	_ = b.Disconnect()
	waitFor(t, time.Second, func() bool { return rec.last().Phase == PhaseDisconnected })
	eng.mu.Lock()
	eng.down = 999
	eng.mu.Unlock()
	b.refreshStats()
	if got := b.GetSnapshot().DownBps; got != 0 {
		t.Fatalf("stats refreshed while disconnected: %v", got)
	}
}

func TestBridgeRunStatsStopsWithContext(t *testing.T) {
	eng := &fakeEngine{}
	b := NewBridge(eng, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		b.RunStats(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunStats did not stop after ctx cancel")
	}
}
