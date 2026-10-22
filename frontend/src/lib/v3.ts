// v3.ts — the UI v3 main screen's state model and copy (docs/ui-v3/UI-SPEC.md §5).
// The four data-states are driven ONLY by the engine's real status events
// (spec §8): idle→disconnected, connecting→connecting, connect error→failed,
// on→connected. Values (colors, sizes, motion) all come from
// docs/ui-v3/younblock-tokens.css, imported by pro.ts; nothing here invents
// a value.

import type { Snapshot } from "./types";
import { GENERIC_CONNECT_ERROR } from "./backend";

/** The screen's four states (data-state on the root). */
export type V3State = "disconnected" | "connecting" | "failed" | "connected";

/** What a committed trigger asks the host for, by state (spec §4). */
export type LeverAction = "connect" | "cancel" | "retry" | "disconnect";

/** Per-state copy (spec §5, exact): headline, cap label, hint, aria label. */
export const V3_COPY: Record<V3State, { headline: string; cap: string; hint: string; aria: string }> = {
  disconnected: {
    headline: "Not connected",
    cap: "START",
    hint: "Tap or swipe down to connect",
    aria: "Connect to YouTube",
  },
  connecting: {
    headline: "Connecting...",
    cap: "CANCEL",
    hint: "Tap to cancel",
    aria: "Cancel connecting",
  },
  failed: {
    headline: "Couldn't connect",
    cap: "RETRY",
    hint: "Tap or swipe down to retry",
    aria: "Retry connecting",
  },
  connected: {
    headline: "Connected",
    cap: "STOP",
    hint: "Tap or swipe up to disconnect",
    aria: "Disconnect",
  },
};

/**
 * Fold a backend Snapshot into the screen's state (spec §8: engine events
 * only, never UI timers):
 *   disconnected | connecting | connected | disconnecting → their v3 selves;
 *   error → failed.
 * `warming` (the engine serving but its video lane still settling, ADR-0052)
 * keeps the connecting look until the hand-off — the honest state for a
 * tunnel that is up but not yet usable. `localError` (a refused Go call,
 * the one pre-existing error signal this screen has) shows the failed look
 * for its 6-second life, exactly like the screen it replaces.
 */
export function v3State(snap: Snapshot, localError: string, warming: boolean): V3State {
  if (localError !== "" && snap.phase === "disconnected") return "failed";
  switch (snap.phase) {
    case "connecting":
      return "connecting";
    case "connected":
      return warming ? "connecting" : "connected";
    // The engine is winding down: the screen already shows its resting
    // look; by the time the cap lands (280 ms) the engine is off. A Start
    // tapped in this window is refused by the bridge and ignored.
    case "disconnecting":
      return "disconnected";
    case "error":
      return "failed";
    default:
      return "disconnected";
  }
}

/**
 * The failed state's sub line (spec §5: warning icon + "Check your
 * internet, then retry", amber). The engine's own error words win when they
 * are MORE specific than the generic line (the NO_TARGET_APPS explanation);
 * the generic connect failure maps to the spec copy so the four states read
 * exactly as specified. A local refusal keeps its own honest words.
 */
export function failedSubLine(message: string | null | undefined): string {
  if (message && message !== GENERIC_CONNECT_ERROR) return message;
  return "Check your internet, then retry";
}
