// format.ts — the screen's display helpers.

import type { SessionRecord, UsageSummary } from "./types";

/** Format milliseconds as HH:MM:SS (the timer under the status). */
export function formatDuration(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
}

/** Format bits per second: "0 Kb/s", "334.3 Kb/s", "136.6 Mbps". */
export function formatBitrate(bps: number): string {
  if (!Number.isFinite(bps) || bps <= 0) return "0 Kb/s";
  const kbps = bps / 1000;
  if (kbps < 1000) return `${kbps.toFixed(1)} Kb/s`;
  return `${(kbps / 1000).toFixed(1)} Mbps`;
}

/** Format a byte count the round-3 way: decimal units (1 GB = 1,000,000,000
 *  bytes, the way internet providers bill) and 3 significant digits, so the
 *  live number visibly ticks: "999 B", "12.3 MB", "123 MB", "2.10 GB". */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "0 B";
  if (n < 1000) return `${Math.floor(n)} B`;
  const units = ["KB", "MB", "GB", "TB", "PB"];
  let v = n;
  let u = -1;
  // Roll over just below 1000 so a rounded "1000 KB" never appears.
  do {
    v /= 1000;
    u++;
  } while (v >= 999.5 && u < units.length - 1);
  const s = v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2);
  return `${s} ${units[u]}`;
}

/** Format a session length in words: "45 s", "22 min", "1 h 1 min",
 *  "2 h 10 min" (the last-session line and the menu's session rows). */
export function formatDurationWords(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  return `${h} h ${m % 60} min`;
}

/** Format a session's end time for the menu rows: "Oct 17, 03:57 AM"
 *  (locale-aware; the record carries no text of its own). */
export function formatDateTime(ms: number): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(ms));
}

/** Fold a session list into the menu's Data-usage summary. The real host
 *  computes this on the Go side (GetUsage); the preview backend reuses this
 *  so both speak exactly the same rules: Today and This month are counted
 *  by session END time, in the viewer's calendar. */
export function summarizeUsage(sessions: SessionRecord[], now = Date.now()): UsageSummary {
  const d = new Date(now);
  const startOfDay = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const startOfMonth = new Date(d.getFullYear(), d.getMonth(), 1).getTime();
  let todayBytes = 0;
  let monthBytes = 0;
  let allTimeBytes = 0;
  for (const s of sessions) {
    const b = s.downBytes + s.upBytes;
    allTimeBytes += b;
    if (s.endMs >= startOfDay) todayBytes += b;
    if (s.endMs >= startOfMonth) monthBytes += b;
  }
  return {
    sessions: sessions.slice(0, 10),
    sessionCount: sessions.length,
    todayBytes,
    monthBytes,
    allTimeBytes,
  };
}
