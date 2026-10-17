// engine_adapter.go — the small Engine wrapper agent-instructions.md asks
// for: it adapts the existing engine (internal/engine) to the Bridge's
// Start/Stop/Stats interface without touching any proxy logic.
//
// The existing Engine.Start() is asynchronous (it returns while the connect
// sequence is still running and reports progress through state events), so
// this wrapper polls GetState() until the run settles: on/degraded means
// ready, error means failed, off means someone else canceled the run.
// Canceling the ctx stops the engine, which is also how the engine cleans
// up a connect sequence that is still running.
package main

import (
	"context"
	"errors"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/usage"
)

// younblockEngine implements the Bridge's Engine interface over the real engine.
type younblockEngine struct {
	eng *engine.Engine
}

// newYounblockEngine wraps an existing engine.Engine.
func newYounblockEngine(eng *engine.Engine) *younblockEngine {
	return &younblockEngine{eng: eng}
}

// Start begins the connect sequence and returns only when the proxy is
// really ready (on or degraded), it failed, or ctx was canceled.
func (y *younblockEngine) Start(ctx context.Context) error {
	if err := y.eng.Start(); err != nil {
		return err // "busy" while the engine is still stopping
	}
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			// The person pressed Cancel: the engine's own Stop cancels a
			// connect sequence that is still running and restores Windows.
			_ = y.eng.Stop()
			return ctx.Err()
		case <-tick.C:
			switch y.eng.GetState().Phase {
			case engine.PhaseOn, engine.PhaseDegraded:
				return nil
			case engine.PhaseError:
				return errors.New("start failed")
			case engine.PhaseOff:
				// The run was abandoned from elsewhere (tray, quit).
				return errors.New("canceled")
			}
		}
	}
}

// Stop turns everything off. The engine makes this safe to call twice
// and when nothing is running.
func (y *younblockEngine) Stop() error {
	return y.eng.Stop()
}

// Stats returns the current speeds in bits per second and the YouTube
// round-trip in milliseconds (-1 when unknown).
func (y *younblockEngine) Stats() (downBps, upBps float64, pingMs int) {
	sample := y.eng.Stats().Sample() // rates are bytes per second
	ping := -1
	if st := y.eng.GetState(); st.LatencyMs != nil && *st.LatencyMs > 0 {
		ping = int(*st.LatencyMs)
	}
	return sample.RateDown * 8, sample.RateUp * 8, ping
}

// Usage implements the bridge's UsageSource (round-3): the live (or
// just-ended) session's relayed bytes and the last finished session —
// the same numbers the webui edition serves at /api/state.
func (y *younblockEngine) Usage() (downBytes, upBytes int64, lastSession *usage.SessionRecord) {
	return y.eng.Usage()
}
