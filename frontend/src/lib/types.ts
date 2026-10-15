// types.ts — the screen's connection contract. These values mirror the Go
// side (ui_bridge.go) exactly: Phase strings and Snapshot json names.

/** The connection state. Must match ui_bridge.go's Phase constants. */
export type Phase =
  | "disconnected"
  | "connecting"
  | "connected"
  | "disconnecting"
  | "error";

/** The state snapshot the screen renders. Must match ui_bridge.go's Snapshot. */
export interface Snapshot {
  phase: Phase;
  /** Unix time in milliseconds when the connection became active; null when not connected. */
  connectedAtMs: number | null;
  /** Speeds in bits per second. */
  downBps: number;
  upBps: number;
  /** Round-trip in milliseconds; null when unknown. */
  pingMs: number | null;
  /** Optional short line shown instead of the ping. */
  message?: string | null;
}
