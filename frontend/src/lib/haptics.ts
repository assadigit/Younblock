// haptics.ts — the key cap's haptics (the refinement round, UI-SPEC (2) §3
// items 8 and 12): 8 ms when the finger presses the cap down, 12 ms when a
// connect or disconnect completes (engine-confirmed), 30/60/30 on entering
// the failed state. The Android WebView implements navigator.vibrate (the
// VIBRATE permission is declared for exactly this); where it is missing —
// desktops, the Linux web edition, test harnesses — every call is a silent
// no-op. No new bridge was added for it.

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

/** 8 ms — the finger pressed the key cap down (UI-SPEC §3 item 12). */
export function press(): void {
  fire(8);
}

/** 12 ms — a connect or disconnect completed (UI-SPEC §3 item 8). */
export function confirm(): void {
  fire(12);
}

/** 30/60/30 — entering the failed state. */
export function error(): void {
  fire([30, 60, 30]);
}
