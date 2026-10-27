// app_test.go — the lite shell's engine wiring, tested beside the
// engine's own tests with the same tools (the stub platform records
// proxy calls; a fake network says yes to every method), so the whole
// connect → serve → disconnect → quit cycle runs network-free on any
// host.
package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// fakeNet says yes to every method (the engine's own fakeNetwork with
// the knobs the lite tests need).
type fakeNet struct{}

func (fakeNet) QuickCheck(context.Context, config.Method) bool { return true }
func (fakeNet) HealCheck(context.Context, config.Method) bool  { return true }

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	plat := stub.NewWithDir(dir)
	app, err := NewApp(engine.Options{
		Platform: plat,
		LogDir:   dir + "/logs",
		Checker:  fakeNet{},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(app.eng.Shutdown)
	return app
}

func waitForPhase(t *testing.T, app *App, want ...engine.Phase) engine.State {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st := app.State()
		for _, w := range want {
			if st.Phase == w {
				return st
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("engine never reached %v (now %s)", want, app.State().Phase)
	return engine.State{}
}

// The one cycle the window performs: connect, serve, disconnect — with
// the sink's events reaching Poll (the UI's door) along the way.
func TestAppConnectServeDisconnect(t *testing.T) {
	app := newTestApp(t)
	if got := app.Poll().Phase; got != engine.PhaseOff {
		t.Fatalf("fresh app phase = %s, want off", got)
	}
	if app.ConnectOnLaunch() {
		t.Errorf("connectOnLaunch default should be off in a fresh dir")
	}

	app.Connect()
	st := waitForPhase(t, app, engine.PhaseOn, engine.PhaseError)
	if st.Phase != engine.PhaseOn {
		t.Fatalf("connect landed on %s (%+v) — the fake network accepts everything", st.Phase, st.Error)
	}
	if st.Port <= 0 {
		t.Fatalf("on without a port: %+v", st)
	}

	// The view over the live state paints the connected screen.
	v := view(app.Poll())
	if v.ButtonLabel != "Disconnect" || v.Footer == "" {
		t.Errorf("connected view = %+v", v)
	}

	app.Disconnect()
	st = waitForPhase(t, app, engine.PhaseOff)
	_ = st
	if v := view(app.Poll()); v.Headline != "Off" {
		t.Errorf("post-stop view = %+v", v)
	}
}

// The sink's channel must not block the engine when the UI is slow:
// many rapid state changes collapse to the freshest one.
func TestAppEventChannelDrainsToFreshest(t *testing.T) {
	app := newTestApp(t)
	for i := 0; i < 32; i++ {
		app.OnEvent("state", `{"phase":"connecting"}`)
	}
	st := app.Poll()
	if st.Phase != engine.PhaseConnecting {
		t.Fatalf("Poll after 32 queued events = %s, want connecting (freshest)", st.Phase)
	}
	// Non-state events are not the lite screen's concern.
	app.OnEvent("stats", `{"downBps":1}`)
	if st := app.Poll(); st.Phase != engine.PhaseOff {
		t.Fatalf("a stats event changed the polled state: %+v", st)
	}
	// Garbage payloads are dropped, not fatal.
	app.OnEvent("state", "{not json")
	if st := app.Poll(); st.Phase != engine.PhaseOff {
		t.Fatalf("a garbage event changed the polled state: %+v", st)
	}
}

// The quit path: Quit asks the engine to stop-and-restore first, then
// fires the registered callback exactly once — and a second Quit is a
// no-op (the raced close button and engine event both arrive).
func TestAppQuitFiresOnceAfterStop(t *testing.T) {
	app := newTestApp(t)
	app.Connect()
	waitForPhase(t, app, engine.PhaseOn, engine.PhaseError)

	var wg sync.WaitGroup
	wg.Add(1)
	fired := 0
	var mu sync.Mutex
	app.OnEngineQuit(func() {
		mu.Lock()
		fired++
		mu.Unlock()
		wg.Done()
	})

	if app.IsQuitting() {
		t.Fatalf("IsQuitting before Quit = true")
	}
	app.Quit()
	app.Quit() // the raced second call must be a no-op
	app.Quit()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("the engine quit callback never fired")
	}
	mu.Lock()
	n := fired
	mu.Unlock()
	if n != 1 {
		t.Errorf("quit callback fired %d times, want 1", n)
	}
	if !app.IsQuitting() {
		t.Errorf("IsQuitting after Quit = false")
	}
	// The engine really stopped (proxy restored on Windows; the stub
	// records the detach the lite close path must have triggered).
	st := app.State()
	if st.Phase != engine.PhaseOff && st.Phase != engine.PhaseError {
		t.Errorf("engine phase after quit = %s, want off/error", st.Phase)
	}
}
