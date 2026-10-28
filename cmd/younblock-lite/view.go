// view.go — the lite desktop edition's state-to-screen mapping.
//
// The lite edition (younblock-lite-desktop) is the dependency-free Windows
// shell: the same engine, the same settings file and the same Windows
// system-proxy mechanism as the main desktop app, behind a hand-drawn
// native window instead of the WebView2 screen — one .exe, nothing to
// install (DECISIONS.md D114). This round (D115) lifts the lite screen to
// the v3 main screen itself: the same four data-states, the same T-table
// copy, the same live figures (timer, session bytes) and the same sub-line
// rules the Svelte screen implements (frontend/src/lib/v3.ts +
// App.svelte) — ported verbatim so both desktops tell the same story about
// the same engine state. This file stays OS-neutral so the copy and the
// phase mapping are unit-tested on any host, exactly like the engine's
// own state machine.
package main

import (
	"fmt"
	"time"

	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/usage"
)

// ScreenState is the screen's four visual states — v3.ts's V3State,
// driven only by the engine's real status events.
type ScreenState int

const (
	ScreenIdle ScreenState = iota
	ScreenConnecting
	ScreenConnected
	ScreenFailed
)

// String names the state for tests and diagnostics.
func (s ScreenState) String() string {
	switch s {
	case ScreenIdle:
		return "idle"
	case ScreenConnecting:
		return "connecting"
	case ScreenConnected:
		return "connected"
	case ScreenFailed:
		return "failed"
	}
	return "unknown"
}

// LeverAction is what a committed lever trigger asks the engine for
// (UI-SPEC §4). While connecting the lever is inert — the reference
// ignores gestures there — so no cancel action exists.
type LeverAction int

const (
	LeverNone LeverAction = iota
	LeverConnect
	LeverRetry
	LeverDisconnect
)

// The per-state copy — the reference T table, ported verbatim from
// v3.ts's V3_COPY. Nothing here invents a word.
var v3Copy = map[ScreenState]struct{ Headline, Cap, Hint, Sub string }{
	ScreenIdle: {
		Headline: "Not connected",
		Cap:      "Start",
		Hint:     "Tap or swipe up to connect",
		Sub:      "", // the idle sub-line is the real last-session line
	},
	ScreenConnecting: {
		Headline: "Connecting...",
		Cap:      "Wait",
		Hint:     "Almost there",
		Sub:      "Setting things up",
	},
	ScreenFailed: {
		Headline: "Couldn't connect",
		Cap:      "Retry",
		Hint:     "Tap or swipe up to retry",
		Sub:      "Could not start. Try again.",
	},
	ScreenConnected: {
		Headline: "Connected",
		Cap:      "Stop",
		Hint:     "Tap or swipe down to disconnect",
		Sub:      "", // D121: the owner ordered "YouTube unblocked." gone
	},
}

// WarmingDetail is the M8-an line (ADR-0052): the detail line while the
// warm gate holds the connect-time hand-off — kept from the screen this
// one replaces (App.svelte's WARMING_DETAIL).
const WarmingDetail = "Getting the first video ready…"

// Screen is everything the window paints for one engine state — the v3
// main screen's whole surface, mirrored.
type Screen struct {
	// State is the visual state (the data-state attribute).
	State ScreenState
	// Slow is the connected screen's slow variant (degraded health:
	// the same readable amber as failed).
	Slow bool

	// Headline is the hero word (the h1).
	Headline string
	// Sub is the line under it (may be empty; SubAmber colors it).
	Sub      string
	SubAmber bool
	// Hint is the quiet line near the bottom (also the lever's
	// aria-label — the action cue).
	Hint string
	// CapLabel is the word on the key cap ("Start"/"Wait"/...).
	CapLabel string
	// Action is what a committed lever trigger performs.
	Action LeverAction

	// ConnectedAt is when the engine reached on (the timer's zero).
	ConnectedAt time.Time
	// ShowTimer: the connected screen's session clock.
	ShowTimer bool
	// The session's live figures (bytes; -1 = the host cannot count).
	DownBytes, UpBytes int64
	// ShowStats: the arrowed byte totals under the timer.
	ShowStats bool
	// ShowCard: the Open YouTube card slot.
	ShowCard bool

	// LastSessionSummary is the idle screen's line ("Last session:
	// 22 min, 15.7 MB"); empty when there is nothing honest to say.
	LastSessionSummary string
}

// screen folds the engine state into the v3 screen (v3.ts's v3State plus
// App.svelte's copy selection, one port). The engine's "disconnecting"
// winding-down maps to idle — the resting look; by the time the cap lands
// the engine is off. Warm (M8-an) keeps the connecting look until the
// hand-off — the honest state for a tunnel that is up but not yet usable.
func screen(st engine.State) Screen {
	state := ScreenIdle
	slow := false
	switch st.Phase {
	case engine.PhaseConnecting:
		state = ScreenConnecting
	case engine.PhaseOn:
		if !st.Warm {
			state = ScreenConnecting // the warming look
		} else {
			state = ScreenConnected
		}
	case engine.PhaseDegraded:
		if !st.Warm {
			state = ScreenConnecting
		} else {
			state, slow = ScreenConnected, true
		}
	case engine.PhaseError:
		state = ScreenFailed
	default: // off, stopping, and anything a future engine adds
		state = ScreenIdle
	}

	sc := Screen{
		State:       state,
		Slow:        slow,
		Headline:    v3Copy[state].Headline,
		Hint:        v3Copy[state].Hint,
		CapLabel:    v3Copy[state].Cap,
		DownBytes:   st.DownBytes,
		UpBytes:     st.UpBytes,
		ConnectedAt: st.Since,
	}

	// The sub-line rules (App.svelte's stateSub/subLine, one port):
	// local errors never exist here (the Go engine call cannot be
	// refused from inside the window's own code path), so the states
	// speak their own lines.
	switch state {
	case ScreenConnecting:
		if st.Phase == engine.PhaseOn || st.Phase == engine.PhaseDegraded {
			sc.Sub = WarmingDetail // the warm gate holds the hand-off
		} else {
			sc.Sub = v3Copy[ScreenConnecting].Sub
		}
	case ScreenFailed:
		sc.Sub = failedSubLine(st.Error)
		sc.SubAmber = true // the amber words are never alone on the full screen
	case ScreenConnected:
		if slow {
			sc.Sub, sc.SubAmber = "Slow connection, tuning...", true
		} else {
			sc.Sub = v3Copy[ScreenConnected].Sub
		}
	case ScreenIdle:
		sc.Sub = lastSessionLine(st.LastSession)
	}

	// The lever's action (LeverButton.svelte's actionFor).
	switch state {
	case ScreenIdle:
		sc.Action = LeverConnect
	case ScreenFailed:
		sc.Action = LeverRetry
	case ScreenConnected:
		sc.Action = LeverDisconnect
	}

	// The connected figures: timer, bytes, card.
	if state == ScreenConnected {
		sc.ShowTimer = true
		sc.ShowStats = st.DownBytes >= 0 && st.UpBytes >= 0
		sc.ShowCard = true
	}
	// The idle screen keeps the last-session line for itself.
	if state == ScreenIdle {
		sc.LastSessionSummary = sc.Sub
	}
	return sc
}

// failedSubLine is the failed state's sub line: the engine's own error
// words win when they are more specific than the generic line — the
// engine's typed code is shown honestly even unworded (the lite's
// established form, D114); a plain connect failure maps to the spec copy.
func failedSubLine(err *engine.StateError) string {
	if err != nil && err.Code != "" {
		return fmt.Sprintf("Could not start. Try again. (%s)", err.Code)
	}
	return v3Copy[ScreenFailed].Sub
}

// lastSessionLine is the idle screen's honest line: only a session that
// really ran (longer than a minute) gets the line (App.svelte's
// lastSessionShown).
func lastSessionLine(last *usage.SessionRecord) string {
	if last == nil || last.EndMs-last.StartMs <= 60_000 {
		return ""
	}
	return fmt.Sprintf("Last session: %s, %s",
		formatDurationWords(last.EndMs-last.StartMs),
		formatBytes(last.DownBytes+last.UpBytes))
}

// ---- the display helpers (format.ts, ported verbatim) ----

// formatDuration formats milliseconds as HH:MM:SS (the timer under the
// status).
func formatDuration(ms int64) string {
	total := ms
	if total < 0 {
		total = 0
	}
	total /= 1000
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// formatBytes formats a byte count the round-3 way: decimal units and 3
// significant digits, so the live number visibly ticks: "999 B",
// "12.3 MB", "123 MB", "2.10 GB".
func formatBytes(n int64) string {
	if n < 0 {
		return "0 B"
	}
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	u := -1
	// Roll over just below 1000 so a rounded "1000 KB" never appears.
	for {
		v /= 1000
		u++
		if !(v >= 999.5 && u < len(units)-1) {
			break
		}
	}
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f %s", v, units[u])
	case v >= 10:
		return fmt.Sprintf("%.1f %s", v, units[u])
	default:
		return fmt.Sprintf("%.2f %s", v, units[u])
	}
}

// formatDurationWords formats a session length in words: "45 s",
// "22 min", "1 h 1 min" (the last-session line).
func formatDurationWords(ms int64) string {
	s := ms / 1000
	if s < 0 {
		s = 0
	}
	if s < 60 {
		return fmt.Sprintf("%d s", s)
	}
	m := s / 60
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	h := m / 60
	return fmt.Sprintf("%d h %d min", h, m%60)
}

// screenChanged compares what the screen paints (the fields the painter
// renders; byte totals included -- they tick the stats row).
func screenChanged(a, b Screen) bool {
	if a.State != b.State || a.Slow != b.Slow {
		return true
	}
	if a.Headline != b.Headline || a.Sub != b.Sub || a.SubAmber != b.SubAmber || a.Hint != b.Hint {
		return true
	}
	if a.CapLabel != b.CapLabel || a.Action != b.Action {
		return true
	}
	if a.ShowTimer != b.ShowTimer || a.ShowStats != b.ShowStats || a.ShowCard != b.ShowCard {
		return true
	}
	if a.DownBytes != b.DownBytes || a.UpBytes != b.UpBytes {
		return true
	}
	if !a.ConnectedAt.Equal(b.ConnectedAt) {
		return true
	}
	return a.LastSessionSummary != b.LastSessionSummary
}
