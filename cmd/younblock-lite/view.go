// view.go — the lite desktop edition's honest state-to-screen mapping.
//
// The lite edition (younblock-lite-desktop) is the dependency-free Windows
// shell: the same engine, the same settings file and the same Windows
// system-proxy mechanism as the main desktop app, behind a plain Win32
// window instead of the WebView2 screen — one .exe, nothing to install
// (DECISIONS.md D114). This file stays OS-neutral so the copy and the
// phase mapping are unit-tested on any host, exactly like the engine's
// own state machine.
package main

import (
        "fmt"

        "github.com/assadigit/Younblock/internal/engine"
)

// DotColor is the semantic status-LED color; the Windows painter turns
// the name into a GDI RGB (one place, no magic numbers there).
type DotColor int

const (
        DotGray DotColor = iota // off / stopped
        DotAmber               // connecting, warming, slow mode
        DotRed                 // connected (the brand's "on")
        DotMuted               // error — quiet, the words carry it
)

// ViewModel is everything the window paints for one engine state.
type ViewModel struct {
        Phase engine.Phase
        // Headline is the state word ("Off", "Connected", ...).
        Headline string
        // Detail is the honest secondary line (empty lines are skipped).
        Detail string
        // ButtonLabel / ButtonEnabled drive the one primary action.
        ButtonLabel   string
        ButtonEnabled bool
        // Dot is the status-LED color name.
        Dot DotColor
        // Footer is the small bottom line (version rides beside it in the
        // painter; here only the dynamic half).
        Footer string
}

// view maps the engine state to the lite screen. The copy is short on
// purpose — the lite screen is one word, one line, one button — and it
// reuses the established lines where they exist (the M8-an warm-gate
// copy "Getting the first video ready…") so both desktops tell the same
// story about the same engine state.
func view(st engine.State) ViewModel {
        v := ViewModel{Phase: st.Phase}
        switch st.Phase {
        case engine.PhaseOff:
                v.Headline = "Off"
                v.Detail = "Your internet is untouched."
                v.ButtonLabel, v.ButtonEnabled = "Connect", true
                v.Dot = DotGray
        case engine.PhaseConnecting:
                v.Headline = "Connecting"
                v.Detail = "Getting the tunnel ready…"
                v.ButtonLabel, v.ButtonEnabled = "Cancel", true
                v.Dot = DotAmber
        case engine.PhaseStopping:
                v.Headline = "Stopping"
                v.Detail = "Restoring your internet settings…"
                v.ButtonLabel, v.ButtonEnabled = "", false
                v.Dot = DotAmber
        case engine.PhaseOn, engine.PhaseDegraded:
                if st.Phase == engine.PhaseDegraded {
                        v.Headline = "Slow mode"
                        v.Detail = "Connected — the line is being re-checked."
                        v.Dot = DotAmber
                } else if !st.Warm {
                        // M8-an: the serving run is up but the video lane is
                        // still settling — the same copy the v3 screen shows.
                        v.Headline = "Connected"
                        v.Detail = "Getting the first video ready…"
                        v.Dot = DotAmber
                } else {
                        v.Headline = "Connected"
                        v.Detail = "Videos should start fast."
                        v.Dot = DotRed
                }
                if st.LatencyMs != nil && st.Phase == engine.PhaseOn && st.Warm {
                        v.Detail += fmt.Sprintf("  ·  %d ms ping", *st.LatencyMs)
                }
                v.ButtonLabel, v.ButtonEnabled = "Disconnect", true
        case engine.PhaseError:
                v.Headline = "Couldn't connect"
                v.Detail = "The tunnel could not start."
                if st.Error != nil && st.Error.Code != "" {
                        // Typed code, honest even unworded — the diagnostic
                        // vocabulary is the engine's, shown small and plain.
                        v.Detail += " (" + st.Error.Code + ")"
                }
                v.ButtonLabel, v.ButtonEnabled = "Retry", true
                v.Dot = DotMuted
        default:
                // Unknown phase (a future engine): everything honest, nothing
                // invented — the word is the phase, the action is retry-safe.
                v.Headline = string(st.Phase)
                v.ButtonLabel, v.ButtonEnabled = "Retry", false
                v.Dot = DotGray
        }
        // The footer names the live local proxy while it serves.
        if st.Phase == engine.PhaseOn || st.Phase == engine.PhaseDegraded {
                if st.Port > 0 {
                        v.Footer = fmt.Sprintf("127.0.0.1:%d", st.Port)
                }
        }
        return v
}

// primaryAction picks the one action the primary button performs for a
// phase — Start for the cold phases, Stop for the live ones (Stop also
// cancels a connecting run; the engine owns that distinction).
func primaryAction(st engine.State) string {
        switch st.Phase {
        case engine.PhaseOn, engine.PhaseDegraded, engine.PhaseConnecting:
                return "stop"
        default:
                return "start"
        }
}
