// format.ts — the screen's two display helpers.

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
