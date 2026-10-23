// v3.ts — the main screen's state model and copy (the refinement round,
// UI-SPEC (2).md). The four data-states are driven ONLY by the engine's
// real status events: idle→idle, connecting→connecting, connect error→
// failed, on→connected. Values (colors, sizes, motion) all come from
// docs/ui-v3/younblock-tokens.css, imported by pro.ts; the copy is the
// reference file's T table, ported verbatim; nothing here invents a value.

import type { Snapshot } from "./types";
import { GENERIC_CONNECT_ERROR } from "./backend";

/** The screen's four states (data-state on the root). */
export type V3State = "idle" | "connecting" | "failed" | "connected";

/** What a committed trigger asks the host for, by state (UI-SPEC §4).
 *  While connecting the lever is inert (the reference ignores gestures
 *  there), so no "cancel" action exists anymore. */
export type LeverAction = "connect" | "retry" | "disconnect";

/** Per-state copy (the reference T table, exact): headline, cap label,
 *  hint (also the track's aria-label, UI-SPEC §5), default sub-line. */
export const V3_COPY: Record<V3State, { headline: string; cap: string; hint: string; sub: string }> = {
  idle: {
    headline: "Not connected",
    cap: "Start",
    hint: "Tap or swipe up to connect",
    sub: "", // the idle sub-line is the real last-session line, not fixed copy
  },
  connecting: {
    headline: "Connecting...",
    cap: "Wait",
    hint: "Almost there",
    sub: "Setting things up",
  },
  failed: {
    headline: "Couldn't connect",
    cap: "Retry",
    hint: "Tap or swipe up to retry",
    sub: "Could not start. Try again.",
  },
  connected: {
    headline: "Connected",
    cap: "Stop",
    hint: "Tap or swipe down to disconnect",
    sub: "YouTube unblocked.",
  },
};

/**
 * Fold a backend Snapshot into the screen's state (engine events only,
 * never UI timers):
 *   disconnected → idle, connecting → connecting, connected → connected,
 *   error → failed. The engine's "disconnecting" winding-down maps to
 *   idle (the resting look; by the time the cap lands the engine is off).
 * `warming` (the engine serving but its video lane still settling,
 *  ADR-0052) keeps the connecting look until the hand-off — the honest
 * state for a tunnel that is up but not yet usable. `localError` (a
 * refused Go call, the one pre-existing error signal this screen has)
 * shows the failed look for its 6-second life.
 */
export function v3State(snap: Snapshot, localError: string, warming: boolean): V3State {
  if (localError !== "" && snap.phase === "disconnected") return "failed";
  switch (snap.phase) {
    case "connecting":
      return "connecting";
    case "connected":
      return warming ? "connecting" : "connected";
    case "disconnecting":
      return "idle";
    case "error":
      return "failed";
    default:
      return "idle";
  }
}

/**
 * The failed state's sub line (UI-SPEC §2: "Could not start. Try again.").
 * The engine's own error words win when they are MORE specific than the
 * generic line (the NO_TARGET_APPS explanation); the generic connect
 * failure maps to the spec copy so the states read exactly as specified.
 * A local refusal keeps its own honest words.
 */
export function failedSubLine(message: string | null | undefined): string {
  if (message && message !== GENERIC_CONNECT_ERROR) return message;
  return "Could not start. Try again.";
}
