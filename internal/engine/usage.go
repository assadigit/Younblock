// usage.go — the round-3 session tracking. One usage session spans an
// engine "on" run (on and degraded are the same session; a heal that
// flaps between them must not split it), its bytes are the deltas of the
// relay counters the tunnels have always fed (AddBytesDown/AddBytesUp in
// internal/proxy/tunnel.go — including the first fragmented record), and
// the finished record goes to the usage store beside settings.json.
//
// The ENGINE owns the tracking because both UI transports sit on top of
// it — the Wails bridge (through younblockEngine's Usage) and the webui
// JSON API (the Linux single-file edition and the Android WebView read
// /api/state and /api/usage). Exactly one tracker, one file, on every
// platform; the numbers cannot disagree between editions.
//
// Locking: every store write happens while e.mu is held. That is
// deliberate — a few-KB atomic rename once per disconnect, the same
// weight as a settings save — because it makes the write order strict:
// an in-flight autosave either completes before the final record lands
// or is skipped entirely; a late autosave can never resurrect a running
// record the disconnect already cleared.
package engine

import (
	"time"

	"github.com/assadigit/Younblock/internal/usage"
)

// usageAutosaveEvery is the crash-recovery cadence (the round-3 decision
// table: "The running session is saved every 15 s and recovered on the
// next start").
const usageAutosaveEvery = 15 * time.Second

// usageOpen prepares the history store under the config directory. A
// failure is logged once and disables the history — the live session
// counters still work; GetUsage answers its typed error and the menu
// shows its honest "Could not read the usage history" line.
func (e *Engine) usageOpen() {
	u, err := usage.Open(usage.DefaultPath(e.platform.ConfigDir()))
	if err != nil {
		e.logger.Warn("usage history unavailable", "reason", errKind(err))
		return
	}
	e.usage = u
}

// startUsageSessionLocked begins a session at the current relay totals;
// caller holds e.mu. Called from goOn — and only when no session is
// live, so an on→degraded→on flap keeps the ORIGINAL start (the timer
// on screen does not reset either).
func (e *Engine) startUsageSessionLocked() {
	if e.sessLive {
		return
	}
	e.sessLive = true
	e.sessStart = e.opts.Now()
	e.sessBaseDown, e.sessBaseUp = e.stats.Totals()
}

// endUsageSessionLocked closes the live session and records it; caller
// holds e.mu. Anything that leaves on/degraded ends it: a user Stop, a
// quit, an engine error. A cancelled or failed CONNECT never started
// one, so it records nothing — the round-3 rule.
func (e *Engine) endUsageSessionLocked() {
	if !e.sessLive {
		return
	}
	e.sessLive = false
	rec := e.usageRecordLocked()
	e.lastSess = &rec
	if e.usage != nil {
		e.usage.End(rec)
	}
}

// usageRecordLocked snapshots the live session as a record at now;
// caller holds e.mu.
func (e *Engine) usageRecordLocked() usage.SessionRecord {
	down, up := e.stats.Totals()
	rec := usage.SessionRecord{
		StartMs:   e.sessStart.UnixMilli(),
		EndMs:     e.opts.Now().UnixMilli(),
		DownBytes: down - e.sessBaseDown,
		UpBytes:   up - e.sessBaseUp,
	}
	if rec.DownBytes < 0 {
		rec.DownBytes = 0 // counters cannot go backwards; clamp anyway
	}
	if rec.UpBytes < 0 {
		rec.UpBytes = 0
	}
	if rec.EndMs < rec.StartMs {
		rec.EndMs = rec.StartMs
	}
	return rec
}

// autosaveUsage snapshots the live session into the store for crash
// recovery. Called from the stats loop's ticker; the store write happens
// under e.mu (see the package comment for why the ordering demands it).
func (e *Engine) autosaveUsage() {
	e.mu.Lock()
	if !e.sessLive || e.usage == nil {
		e.mu.Unlock()
		return
	}
	rec := e.usageRecordLocked()
	e.usage.SaveRunning(rec)
	e.mu.Unlock()
}

// Usage exposes the session data for the bridge's UsageSource
// (ui_bridge.go): the live session's byte totals — or, once it has
// ended, the last finished session's, so the row does not blink to zero
// while "stopping" settles — plus the last finished record itself.
func (e *Engine) Usage() (downBytes, upBytes int64, last *usage.SessionRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sessLive {
		down, up := e.stats.Totals()
		downBytes, upBytes = down-e.sessBaseDown, up-e.sessBaseUp
		if downBytes < 0 {
			downBytes = 0
		}
		if upBytes < 0 {
			upBytes = 0
		}
	} else if e.lastSess != nil {
		downBytes, upBytes = e.lastSess.DownBytes, e.lastSess.UpBytes
	}
	if e.lastSess != nil {
		rec := *e.lastSess
		last = &rec
	}
	return downBytes, upBytes, last
}
