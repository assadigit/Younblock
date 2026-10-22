// haptics.ts — the v3 button's haptics (spec §4): tick when a swipe crosses
// the threshold or springs back, confirm when connect/disconnect completes,
// error on entering the failed state. The Android WebView implements
// navigator.vibrate (the VIBRATE permission is declared for exactly this);
// where it is missing — desktops, the Linux web edition, test harnesses —
// every call is a silent no-op. No new bridge was added for it.

type Pattern = number | number[];

/** True when the host can actually vibrate (checked once, lazily). */
function supported(): boolean {
  return typeof navigator !== "undefined" && typeof navigator.vibrate === "function";
}

/** Fire-and-forget: a refusal or missing API never breaks the flow. */
function fire(pattern: Pattern): void {
  if (!supported()) return;
  try {
    navigator.vibrate(pattern);
  } catch {
    // never fatal: a broken vibrate must not break a connect tap
  }
}

/** 10 ms — the swipe crossed the threshold, or the cap sprang back. */
export function tick(): void {
  fire(10);
}

/** 25 ms — a connect or disconnect completed (engine-confirmed). */
export function confirm(): void {
  fire(25);
}

/** 30/60/30 — entering the failed state. */
export function error(): void {
  fire([30, 60, 30]);
}
