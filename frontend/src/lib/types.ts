// types.ts — the screen's connection contract. These values mirror the Go
// side (ui_bridge.go) exactly: Phase strings and Snapshot json names.

/** The connection state. Must match ui_bridge.go's Phase constants. */
export type Phase =
  | "disconnected"
  | "connecting"
  | "connected"
  | "disconnecting"
  | "error";

/** One finished usage session. Must match internal/usage's SessionRecord
 *  (round-3 data contract: names must match exactly). */
export interface SessionRecord {
  /** Unix ms when the session began (the moment the engine reached on). */
  startMs: number;
  /** Unix ms when the session ended (disconnect or quit). */
  endMs: number;
  /** Bytes received from websites during the session. */
  downBytes: number;
  /** Bytes sent to websites during the session. */
  upBytes: number;
}

/** The menu's Data-usage summary (Go's GetUsage). */
export interface UsageSummary {
  /** Newest first, at most 10 entries. */
  sessions: SessionRecord[];
  /** How many sessions the history keeps (up to 500). */
  sessionCount: number;
  todayBytes: number;
  monthBytes: number;
  allTimeBytes: number;
}

/** The state snapshot the screen renders. Must match ui_bridge.go's Snapshot. */
export interface Snapshot {
  phase: Phase;
  /** Unix time in milliseconds when the connection became active; null when not connected. */
  connectedAtMs: number | null;
  /** Speeds in bits per second; -1 when the engine cannot measure them. */
  downBps: number;
  upBps: number;
  /** Round-trip in milliseconds; null when unknown. */
  pingMs: number | null;
  /** Optional short line shown instead of the ping. */
  message?: string | null;
  /** Bytes used by THIS session (round-3); -1 when the engine cannot count. */
  downBytes: number;
  upBytes: number;
  /** The last finished session; null when there has never been one. */
  lastSession?: SessionRecord | null;
  /** M8-an (ADR-0052): true while the engine is serving but its video
   * lane is still warming — the connect-time hand-off's hold. Set by the
   * HTTP/SSE host (the engine's own state carries `warm`) and by the
   * Wails bridge's HealthSource; absent on the preview host, where
   * nothing waits on it. */
  warming?: boolean;
  /** UI v3 (docs/ui-v3/UI-SPEC.md §8): true while the engine reports its
   * serving run degraded (its health signal — a real engine event, never a
   * UI guess). The v3 screen shows it as slow mode (data-health="slow").
   * False while serving healthy; absent/false everywhere else. */
  degraded?: boolean;
}
