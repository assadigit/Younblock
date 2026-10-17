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
