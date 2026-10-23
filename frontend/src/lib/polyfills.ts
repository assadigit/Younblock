// polyfills.ts — the any-version WebView floor (the 1.6.7-debug.2 round).
//
// The Android edition's minSdk is 26 (Android 8.0, 2017), and an 8.0/8.1
// phone whose WebView was never updated ships Chrome ~58-60 — common
// wherever Play cannot deliver WebView updates. The bundle is built at
// es2017 (vite.config.ts), so every newer SYNTAX parses there; what
// esbuild cannot transpile are the RUNTIME pieces the svelte 5 runtime
// references:
//
//   queueMicrotask — svelte's reactivity flush scheduler (Chrome 71+;
//     referenced on the first flush, i.e. at boot)
//   globalThis     — svelte's environment probe at module top level
//     (Chrome 71+; evaluated before any component renders)
//   String.prototype.replaceAll — svelte's template and CSS processing
//     paths (Chrome 85+)
//
// Each is feature-detected and filled ONLY when missing: modern WebViews
// and every desktop edition execute the native ones untouched, and the
// fills are the spec-equivalent shapes (the replaceAll fill preserves
// the $-substitution semantics of the replacement string, which the
// naive split/join form would silently drop).
//
// This module must be the FIRST import of main.ts: ES module evaluation
// order is import order, so these fills run before the svelte runtime's
// own top-level probes.

if (typeof window.queueMicrotask === "undefined") {
  // Microtask ordering is preserved: Promise.then callbacks ARE
  // microtasks on every engine that lacks queueMicrotask.
  window.queueMicrotask = (cb: () => void): void => {
    void Promise.resolve().then(cb);
  };
}

if (typeof globalThis === "undefined") {
  // In a browser there is exactly one global object; where the
  // `globalThis` binding is missing, window IS it. The bare identifier
  // resolves through the global scope, so defining the property on
  // window makes every later `globalThis.x` reference resolve.
  (window as unknown as { globalThis: unknown }).globalThis = window;
}

if (typeof String.prototype.replaceAll === "undefined") {
  const escapeRegExp = (s: string): string =>
    s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  Object.defineProperty(String.prototype, "replaceAll", {
    enumerable: false,
    configurable: true,
    writable: true,
    value(this: string, search: string | RegExp, replacement: string): string {
      if (search instanceof RegExp) {
        if (!search.global) {
          throw new TypeError(
            "String.prototype.replaceAll called with a non-global RegExp",
          );
        }
        return this.replace(search, replacement);
      }
      // Route through replace with an escaped global pattern so the
      // replacement's $-substitutions behave exactly like the native
      // method's.
      return this.replace(
        new RegExp(escapeRegExp(String(search)), "g"),
        replacement,
      );
    },
  });
}
