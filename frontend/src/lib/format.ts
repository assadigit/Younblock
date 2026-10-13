// format.ts — display helpers (file 02, section 4.1: 1024-based byte
// sizes with one decimal above KB; a tabular timer).

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

/** The subtitle line for a state (file 02, section 4.1 table). */
export function subtitleFor(phase: string, detail: string): string {
  if (phase === "on") return "Unblocking YouTube";
  if (phase === "degraded") return "On, but YouTube isn't responding";
  if (phase === "off") return "Turn on to unblock YouTube";
  if (phase === "connecting") {
    if (detail === "starting") return "Getting ready…";
    if (detail === "checking") return "Checking YouTube…";
    if (detail.startsWith("trying:")) return "Trying another method…";
    return "Getting ready…";
  }
  if (phase === "stopping") return "";
  return "";
}
