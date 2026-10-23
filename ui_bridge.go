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

	"github.com/assadigit/Younblock/internal/usage"
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
	// DownBytes and UpBytes are this session's relayed byte totals
	// (round-3); -1 means the engine cannot count, and the screen
	// then hides its usage lines instead of inventing numbers.
	DownBytes int64 `json:"downBytes"`
	UpBytes   int64 `json:"upBytes"`
	// LastSession is the most recently finished session; nil when
	// there has never been one. No omitempty: the explicit null is
	// how a cleared history clears the screen's Last-session line.
	LastSession *usage.SessionRecord `json:"lastSession"`
	// Degraded (the v3 screen's slow mode): the serving run flapped to
	// the engine's degraded phase — the run still serves, the health
	// monitor is re-checking. The same signal the HTTP/SSE host folds
	// into its snapshots for the Android WebView (one UI, one set of
	// signals — every edition now); omitempty keeps old hosts' JSON
	// unchanged when the capability is absent.
	Degraded bool `json:"degraded,omitempty"`
	// Warming (the M8-an warm gate): the engine serves but its video
	// lane is still settling. Meaningful only while connected; the
	// screen additionally gates its warming copy on the host's
	// autoOpenTarget capability, so this surfaces only where something
	// waits on the gate.
	Warming bool `json:"warming,omitempty"`
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

// UsageSource is the optional Engine capability behind the round-3 usage
// lines: the live session's relayed bytes (or the last finished
// session's, once it ended) and the last finished record. An engine
// without it gets -1/-1 and no lastSession — the screen hides what it
// does not know instead of showing a made-up zero.
type UsageSource interface {
	Usage() (downBytes, upBytes int64, lastSession *usage.SessionRecord)
}

// HealthSource is the optional Engine capability behind the v3 screen's
// slow-mode and warming signals: whether the serving run currently sits
// in the engine's degraded phase (slow mode) and whether its warm gate
// is still holding. Polled on the same once-a-second stats cadence; a
// bridge without it simply never sets Degraded/Warming (the screen then
// shows neither, exactly like before this capability existed).
type HealthSource interface {
	Health() (degraded, warming bool)
}

// healthFrom reads the health pair off an engine that can report it.
func healthFrom(engine Engine) (degraded, warming bool) {
	if src, ok := engine.(HealthSource); ok {
		return src.Health()
	}
	return false, false
}

// usageFrom reads the usage triple off an engine that can count.
func usageFrom(engine Engine) (down, up int64, last *usage.SessionRecord) {
	if src, ok := engine.(UsageSource); ok {
		return src.Usage()
	}
	return -1, -1, nil
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
	b := &Bridge{
		engine: engine,
		emit:   emit,
		now:    time.Now,
		snap:   Snapshot{Phase: PhaseDisconnected, PingMs: -1, DownBytes: -1, UpBytes: -1},
	}
	// Round-3: the first snapshot already carries the last finished
	// session (a restarted app shows its "Last session:" line at once).
	b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(engine)
	return b
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
	b.snap = Snapshot{Phase: PhaseConnecting, PingMs: -1, DownBytes: -1, UpBytes: -1}
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
		b.snap = Snapshot{Phase: PhaseError, PingMs: -1, DownBytes: -1, UpBytes: -1, Message: "Could not connect. Try again."}
	} else {
		down, up, ping := b.engine.Stats()
		b.snap = Snapshot{
			Phase:         PhaseConnected,
			ConnectedAtMs: b.now().UnixMilli(),
			DownBps:       down,
			UpBps:         up,
			PingMs:        ping,
		}
		// Round-3: the session just began — carry its counters (and
		// the last finished session, for the line under "Not
		// Connected" after the next disconnect).
		b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(b.engine)
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
		b.snap = Snapshot{Phase: PhaseDisconnected, PingMs: -1, DownBytes: -1, UpBytes: -1}
		b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(b.engine)
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
	b.snap = Snapshot{Phase: PhaseDisconnected, PingMs: -1, DownBytes: -1, UpBytes: -1}
	// Round-3: the engine just closed the session — the snapshot
	// carries the record so the screen can show "Last session: …".
	b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(b.engine)
	s = b.snap
	b.mu.Unlock()
	b.emit(s)
	return err
}

// RefreshUsage re-reads the engine's usage triple into the snapshot and
// emits it — used after the history is cleared so the screen's
// Last-session line disappears at once (the engine's own state event
// already says the same thing on the webui path).
func (b *Bridge) RefreshUsage() {
	b.mu.Lock()
	b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(b.engine)
	s := b.snap
	b.mu.Unlock()
	b.emit(s)
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
	degraded, warming := healthFrom(b.engine)

	b.mu.Lock()
	if b.snap.Phase != PhaseConnected {
		b.mu.Unlock()
		return
	}
	b.snap.DownBps, b.snap.UpBps, b.snap.PingMs = down, up, ping
	b.snap.Degraded, b.snap.Warming = degraded, warming
	// Round-3: the usage counters tick on the same once-a-second
	// cadence as the speeds.
	b.snap.DownBytes, b.snap.UpBytes, b.snap.LastSession = usageFrom(b.engine)
	s := b.snap
	b.mu.Unlock()
	b.emit(s)
}
