// shellback.ts — the Android back button's page half (the 1.6.7-debug.3
// polish round). The screen's modal surfaces — the menu sheet, the
// diagnostics view — must close on BACK before the system's default
// (leaving the app) ever runs; that is what a modal means on Android.
//
// The mechanism, both directions CSP-clean and fail-safe:
//   page→shell  nothing: the shell PROBES the page — MainActivity's
//               onBackPressed evaluates `window.YounblockUI.back()`
//               through evaluateJavascript (browser-initiated script,
//               not the page's own eval, so the page's CSP never applies;
//               and if some WebView build still refused it, the probe
//               returns null and the shell falls back to the default
//               back exactly as before this round — the behavior can
//               only ever be additive, never broken).
//   shell→page  the module exposes YounblockUI.back() on window the
//               moment it loads, before any modal can open: the probe
//               never sees a half-installed surface.
//
// Where no shell exists (the desktops, the Linux web edition, plain
// browsers) the module is inert: nobody probes it, and Escape already
// closes both modals there.
//
// es2017 floor (the 1.6.7-debug.2 round): plain checks only, no
// runtime-API surface beyond Array — the old-WebView boot floor holds.

type BackHandler = () => void

/** The modal closers, innermost last (diagnostics sits over the sheet). */
const stack: BackHandler[] = []

/**
 * Register the closer of the topmost modal; returns the function that
 * unregisters it — call it when the modal closes, whatever closed it
 * (the X, Escape, a completed action), so the stack never holds a dead
 * closer.
 */
export function pushBackHandler(h: BackHandler): () => void {
  stack.push(h)
  return () => {
    const i = stack.indexOf(h)
    if (i >= 0) {
      stack.splice(i, 1)
    }
  }
}

/**
 * The shell's entry point (evaluated on every back press): true = a modal
 * was open and has been closed, the press is consumed; false = nothing
 * was open, the shell runs its default back.
 */
;(window as unknown as { YounblockUI?: { back: () => boolean } }).YounblockUI = {
  back: (): boolean => {
    if (stack.length === 0) return false
    stack[stack.length - 1]()
    return true
  },
}
