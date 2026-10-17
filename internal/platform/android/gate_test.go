// gate_test.go — the M5 start-gate semantics of the Android adapter
// (DECISIONS.md D77): the zero value is open (the adapter's no-op form
// must never hold a connect run), SetStartReady arms/opens/re-arms, a
// cancellation releases the waiter with the ctx error, and one open
// wakes every waiter.
package android

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
)

// TestStartGateZeroValueOpen — the adapter straight out of New never
// holds: WaitStartReady returns immediately (cmd/core, host tests).
func TestStartGateZeroValueOpen(t *testing.T) {
	p := New(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.WaitStartReady(ctx); err != nil {
		t.Fatalf("zero-value gate held the run: %v", err)
	}
}

// TestStartGateHoldOpenRelease — armed closed blocks; opening wakes the
// waiter; opening again is a no-op.
func TestStartGateHoldOpenRelease(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- p.WaitStartReady(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("gate open while armed closed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	p.SetStartReady(true)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiter after open: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("open did not release the waiter")
	}

	// Idempotent open.
	p.SetStartReady(true)
	if err := p.WaitStartReady(context.Background()); err != nil {
		t.Fatalf("second open broke the gate: %v", err)
	}
}

// TestStartGateCancelReleases — a cancelled run is released with the
// context error (the canceller owns the engine phase; the adapter must
// not invent one).
func TestStartGateCancelReleases(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.WaitStartReady(ctx) }()

	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled waiter got %v, want context.Canceled", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("cancel did not release the waiter")
	}

	// The gate itself is still armed closed — the cancellation did not
	// open it (the next run waits too, as the shell re-arms on unwind).
	p.SetStartReady(false)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := p.WaitStartReady(ctx2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("post-cancel gate state: %v, want a held gate (DeadlineExceeded)", err)
	}
}

// TestStartGateRearmAfterOpen — after an open, a re-arm (ready=false)
// holds the NEXT run: this is the revoke/service-death semantics.
func TestStartGateRearmAfterOpen(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(true)
	p.SetStartReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := p.WaitStartReady(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("re-armed gate did not hold: %v", err)
	}
}

// TestStartGateWakesAllWaiters — one open releases every concurrent
// waiter (the engine may run several aborted+retried sequences).
func TestStartGateWakesAllWaiters(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	const n = 4
	done := make(chan error, n)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for i := 0; i < n; i++ {
		go func() { done <- p.WaitStartReady(ctx) }()
	}

	time.Sleep(100 * time.Millisecond)
	p.SetStartReady(true)
	for i := 0; i < n; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("waiter %d: %v", i, err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("waiter %d not released by the open", i)
		}
	}
}

// ---- M7 refusal semantics (D81) -----------------------------------------

// TestStartGateRefuseDeliversToWaiter — a refusal wakes the registered
// waiter with the typed error, and the gate stays ARMED afterwards (the
// shell re-runs the whole sequence on the next connect; a refusal is a
// resolution, not an open). Delivery goes to the registered waiter —
// the engine's one-live-run rule (D82) guarantees at most one.
func TestStartGateRefuseDeliversToWaiter(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.WaitStartReady(ctx) }()
	time.Sleep(100 * time.Millisecond) // let the waiter register

	if !p.RefuseStart("NO_TARGET_APPS") {
		t.Fatal("RefuseStart on an armed gate must deliver (true)")
	}
	select {
	case err := <-done:
		var pr *engine.PlatformRefusalError
		if !errors.As(err, &pr) || pr.Code != "NO_TARGET_APPS" {
			t.Fatalf("waiter got %v, want the typed NO_TARGET_APPS refusal", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("the refusal did not reach the waiter")
	}

	// Still armed: the next run waits (the shell re-discovers before
	// opening; a refusal never opens the door).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel2()
	if err := p.WaitStartReady(ctx2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("post-refusal gate state: %v, want still armed (DeadlineExceeded)", err)
	}
}

// TestStartGateRefuseSurvivesRearm — THE race the registration design
// exists for: refuse() hands the error to the waiter atomically, and a
// re-arm (SetStartReady(false), exactly what the shell's unwind does
// right after a refusal) can never claw it back — a woken-but-not-yet-
// scheduled waiter still sees the refusal. A cleared-in-hindsight
// delivery would let a refused run proceed as if permitted, undoing
// D77's whole guarantee.
func TestStartGateRefuseSurvivesRearm(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.WaitStartReady(ctx) }()
	time.Sleep(100 * time.Millisecond) // let the waiter register

	if !p.RefuseStart("NO_TARGET_APPS") {
		t.Fatal("RefuseStart must deliver")
	}
	p.SetStartReady(false) // the shell's immediate re-arm — the race

	select {
	case err := <-done:
		var pr *engine.PlatformRefusalError
		if !errors.As(err, &pr) || pr.Code != "NO_TARGET_APPS" {
			t.Fatalf("waiter got %v after the re-arm — the delivery was lost (the exact D81 race)", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("the delivered refusal vanished under the re-arm")
	}
}

// TestStartGateRefuseLatchesForLateRun — refuse() before the run reaches
// the gate (the start race: the goroutine is spawned but not scheduled):
// the refusal latches and the next wait() consumes it immediately.
func TestStartGateRefuseLatchesForLateRun(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)

	if !p.RefuseStart("NO_TARGET_APPS") {
		t.Fatal("RefuseStart with no waiter must latch (true)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.WaitStartReady(ctx)
	var pr *engine.PlatformRefusalError
	if !errors.As(err, &pr) || pr.Code != "NO_TARGET_APPS" {
		t.Fatalf("late run got %v, want the latched refusal", err)
	}
}

// TestStartGateLatchClearedByRearm — a re-arm is a clean slate: an
// unconsumed latch cannot poison the next connect (the shell re-discovers
// the platform state on every attempt).
func TestStartGateLatchClearedByRearm(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)
	p.RefuseStart("NO_TARGET_APPS") // latched, nobody waiting
	p.SetStartReady(false)          // the shell's unwind re-arm

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := p.WaitStartReady(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("re-arm must clear the latch; got %v, want a held gate", err)
	}
}

// TestStartGateLatchClearedByOpen — an open supersedes any latch.
func TestStartGateLatchClearedByOpen(t *testing.T) {
	p := New(Options{})
	p.SetStartReady(false)
	p.RefuseStart("NO_TARGET_APPS")
	p.SetStartReady(true)

	if err := p.WaitStartReady(context.Background()); err != nil {
		t.Fatalf("open must clear the latch; got %v", err)
	}
}

// TestStartGateRefuseOpenGateNotHeld — an open gate refuses nothing
// (false): no run is held at an open door, and the shell's honest-stop
// path owns a serving engine.
func TestStartGateRefuseOpenGateNotHeld(t *testing.T) {
	p := New(Options{}) // zero value: open
	if p.RefuseStart("NO_TARGET_APPS") {
		t.Fatal("RefuseStart on an open gate must report not-held (false)")
	}
}
