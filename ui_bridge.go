// ui_bridge.go — connects the Younblock screen to the proxy engine.
// Place in your Go project root (next to main.go, package main).
// It uses only the Go standard library, so it compiles without Wails.
//
// What it does:
//   * keeps the one true connection state (disconnected / connecting / connected / disconnecting / error)
//   * tells the screen about every change (through the "emit" function you give it)
//   * refreshes speed and ping once per second while connected
//
// What you must provide: an Engine, a thin wrapper around your existing proxy code.

package main

import (
	"context"
	"sync"
	"time"
)

// Phase is the connection state. The text values must match src/lib/types.ts exactly.
type Phase string

const (
	PhaseDisconnected  Phase = "disconnected"
	PhaseConnecting    Phase = "connecting"
	PhaseConnected     Phase = "connected"
	PhaseDisconnecting Phase = "disconnecting"
	PhaseError         Phase = "error"
)

// Snapshot is sent to the screen as JSON. The json names must match src/lib/types.ts exactly.
type Snapshot struct {
	Phase Phase `json:"phase"`
	// ConnectedAtMs is Unix time in milliseconds when the connection became active; 0 when not connected.
	ConnectedAtMs int64 `json:"connectedAtMs"`
	// DownBps and UpBps are speeds in bits per second.
	DownBps float64 `json:"downBps"`
	UpBps   float64 `json:"upBps"`
	// PingMs is the round-trip time in milliseconds; -1 when unknown.
	PingMs int `json:"pingMs"`
	// Message is an optional short line shown instead of the ping, for example "Could not connect. Try again."
	Message string `json:"message,omitempty"`
}

// Engine is the thin wrapper around your existing proxy code.
//
//	Start  begins unblocking (open the local proxy, set it up for Chrome, and so on).
//	       It must return nil when everything is ready, or an error when it failed.
//	       It must return quickly after ctx is cancelled (the person pressed Cancel).
//	Stop   ends unblocking. It must be safe to call more than once and when nothing is running.
//	Stats  returns the current speeds in bits per second and the ping in ms (-1 when unknown).
type Engine interface {
	Start(ctx context.Context) error
	Stop() error
	Stats() (downBps, upBps float64, pingMs int)
}

// Bridge owns the connection state. Create it once with NewBridge.
type Bridge struct {
	mu      sync.Mutex
	engine  Engine
	emit    func(Snapshot)
	now     func() time.Time
	snap    Snapshot
	cancel  context.CancelFunc // cancels a Start that is still running
	attempt uint64             // grows on every Connect/Disconnect so late answers can be ignored
}

// NewBridge builds a Bridge. emit is called after every state change, never while a lock is held.
func NewBridge(engine Engine, emit func(Snapshot)) *Bridge {
	if emit == nil {
		emit = func(Snapshot) {}
	}
	return &Bridge{
		engine: engine,
		emit:   emit,
		now:    time.Now,
		snap:   Snapshot{Phase: PhaseDisconnected, PingMs: -1},
	}
}

// GetSnapshot returns the current state. The screen calls this when it opens.
func (b *Bridge) GetSnapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snap
}

// Connect starts connecting and returns at once. Progress arrives through emit.
func (b *Bridge) Connect() error {
	b.mu.Lock()
	switch b.snap.Phase {
	case PhaseConnecting, PhaseConnected, PhaseDisconnecting:
		b.mu.Unlock()
		return nil // already busy or already done: nothing to do
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.attempt++
	id := b.attempt
	b.snap = Snapshot{Phase: PhaseConnecting, PingMs: -1}
	s := b.snap
	b.mu.Unlock()

	b.emit(s)
	go b.runStart(ctx, id)
	return nil
}

func (b *Bridge) runStart(ctx context.Context, id uint64) {
	err := b.engine.Start(ctx)

	b.mu.Lock()
	if id != b.attempt {
		// The person pressed Cancel (or Stop) while we were connecting.
		b.mu.Unlock()
		if err == nil {
			_ = b.engine.Stop()
		}
		return
	}
	b.cancel = nil
	if err != nil {
		b.snap = Snapshot{Phase: PhaseError, PingMs: -1, Message: "Could not connect. Try again."}
	} else {
		down, up, ping := b.engine.Stats()
		b.snap = Snapshot{
			Phase:         PhaseConnected,
			ConnectedAtMs: b.now().UnixMilli(),
			DownBps:       down,
			UpBps:         up,
			PingMs:        ping,
		}
	}
	s := b.snap
	b.mu.Unlock()
	b.emit(s)
}

// Disconnect stops unblocking. It also cancels a connection that is still starting.
// It returns after the engine has stopped.
func (b *Bridge) Disconnect() error {
	b.mu.Lock()
	switch b.snap.Phase {
	case PhaseDisconnected, PhaseDisconnecting:
		b.mu.Unlock()
		return nil
	case PhaseError:
		b.snap = Snapshot{Phase: PhaseDisconnected, PingMs: -1}
		s := b.snap
		b.mu.Unlock()
		b.emit(s)
		return nil
	}
	b.attempt++ // any Start still running is now out of date
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.snap.Phase = PhaseDisconnecting
	b.snap.Message = ""
	s := b.snap
	b.mu.Unlock()
	b.emit(s)

	err := b.engine.Stop()

	b.mu.Lock()
	b.snap = Snapshot{Phase: PhaseDisconnected, PingMs: -1}
	s = b.snap
	b.mu.Unlock()
	b.emit(s)
	return err
}

// RunStats refreshes speed and ping once per second while connected.
// Start it once, in its own goroutine, and cancel ctx when the app closes:
//
//	go bridge.RunStats(ctx)
func (b *Bridge) RunStats(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.refreshStats()
		}
	}
}

func (b *Bridge) refreshStats() {
	b.mu.Lock()
	if b.snap.Phase != PhaseConnected {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	down, up, ping := b.engine.Stats() // called without the lock: it may take a moment

	b.mu.Lock()
	if b.snap.Phase != PhaseConnected {
		b.mu.Unlock()
		return
	}
	b.snap.DownBps, b.snap.UpBps, b.snap.PingMs = down, up, ping
	s := b.snap
	b.mu.Unlock()
	b.emit(s)
}
