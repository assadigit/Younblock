// format.ts — display helpers (file 02, section 4.1: 1024-based byte
// sizes with one decimal above KB; a tabular timer).
// v1.5.0 (D59): the subtitle lines now come from the copy deck like
// every other word on screen — the English strings used to be baked in
// here while en.json carried an unused twin of the same table.

import { copy } from "../i18n";

/** Format bytes as B / KB / MB / GB (1024-based, one decimal above KB). */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "0 B";
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}

/** Format seconds as HH:MM:SS with tabular digits. */
export function formatTimer(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(s / 3600);
  const minutes = Math.floor((s % 3600) / 60);
  const seconds = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
}

/** The subtitle line for a state (file 02, section 4.1 table, copy in
 * en.json). The degraded line says which part failed when the engine
 * reported it (D44): a secure-DNS problem reads differently from a
 * YouTube-tunnel one. */
export function subtitleFor(
  phase: string,
  detail: string,
  fail?: { step: string; code: string } | null,
): string {
  if (phase === "on") return copy.subtitle.on;
  if (phase === "degraded") {
    if (fail && fail.step === "dns") return copy.subtitle.degradedDns;
    return copy.subtitle.degraded;
  }
  if (phase === "off") return copy.subtitle.off;
  if (phase === "connecting") {
    if (detail === "checking") return copy.subtitle.checking;
    if (detail.startsWith("trying:")) return copy.subtitle.trying;
    return copy.subtitle.starting;
  }
  if (phase === "stopping") return copy.subtitle.stopping;
  return "";
}
