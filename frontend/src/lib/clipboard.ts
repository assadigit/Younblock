// clipboard.ts — getting text OUT of the page, honestly (M8-p, ADR-0015).
//
// The Diagnostics screen (and Help's copy-report button) must deliver the
// report to the user's clipboard, and the ONE environment where that is
// hard is the Android WebView this project ships in: navigator.clipboard
// is missing or rejects there depending on the WebView build, and a
// silent failure that still says "Copied" is worse than no button — the
// user would paste nothing and the device round would be wasted.
//
// So the paths are ordered by reliability:
//
//   1. The Android shell bridge — MainActivity injects window.
//      YounblockShell (addJavascriptInterface) whose copyToClipboard uses
//      the native ClipboardManager. Present only inside the APK; the
//      interface methods are synchronous from JS's point of view.
//   2. navigator.clipboard.writeText — the desktop browsers and the
//      Linux web edition (127.0.0.1 is a trustworthy origin there).
//   3. The execCommand fallback — a temporary off-screen textarea +
//      document.execCommand("copy"), the old WebView-safe path, inside
//      the user gesture the button press provides.
//
// Every path reports an honest boolean; callers toast accordingly.
// shareText is Android-shell-only (the system share sheet — the most
// robust way to hand the report to a chat app), so the Share button
// renders only where canShare() is true.

/** The Android shell's injected surface (MainActivity.PageBridge). */
export interface ShellBridge {
  copyToClipboard(text: string): boolean;
  shareText(text: string): boolean;
  /** The "Quit Younblock" door (D112): optional so older shells —
   *  and every non-APK host — keep typing cleanly without it. Present
   *  in the APK from 1.6.9. */
  quitApp?(): boolean;
}

declare global {
  interface Window {
    YounblockShell?: ShellBridge;
  }
}

/** The shell bridge, or null outside the APK. */
export function shellBridge(): ShellBridge | null {
  if (typeof window === "undefined") return null;
  const bridge = window.YounblockShell;
  if (!bridge || typeof bridge.copyToClipboard !== "function") return null;
  return bridge;
}

/** True when the system share sheet is available (the APK shell). */
export function canShare(): boolean {
  const bridge = shellBridge();
  return bridge !== null && typeof bridge.shareText === "function";
}

/** Copy text through the first path that works; resolves honestly. */
export async function copyText(text: string): Promise<boolean> {
  // 1. The Android shell bridge.
  const bridge = shellBridge();
  if (bridge) {
    try {
      if (bridge.copyToClipboard(text)) return true;
    } catch {
      // Fall through to the browser paths — the bridge exists but
      // refused (e.g. the clipboard service is unhappy).
    }
  }
  // 2. The async clipboard API.
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Not granted / not available — try the legacy path.
    }
  }
  // 3. The execCommand fallback (must stay inside the user gesture).
  try {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.top = "-1000px";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.focus();
    area.select();
    area.setSelectionRange(0, text.length);
    const ok = document.execCommand("copy");
    area.remove();
    return ok;
  } catch {
    return false;
  }
}

/** Open the system share sheet with the text (APK only). */
export function shareText(text: string): boolean {
  const bridge = shellBridge();
  if (!bridge || typeof bridge.shareText !== "function") return false;
  try {
    return bridge.shareText(text);
  } catch {
    return false;
  }
}

/**
 * Hand the confirmed "Quit Younblock" to the native shell (D112, the
 * APK from 1.6.9): the shell runs the ordered end-of-app sequence —
 * engine and proxy stopped, TUN closed, service stopped, foreground
 * out, notification cancelled by ID, activity finished and removed
 * from Recents, process ended — so there is nothing left for the page
 * to do afterwards except die with the app. Resolves true when the
 * sequence was dispatched; false when no quit-capable shell exists
 * (the desktops, the Linux web edition, a plain browser, an older
 * APK) — the caller then keeps the app alive and says so honestly,
 * mirroring copyText's "report what actually happened" contract.
 */
export function quitApp(): boolean {
  const bridge = shellBridge();
  if (!bridge || typeof bridge.quitApp !== "function") return false;
  try {
    return bridge.quitApp();
  } catch {
    return false;
  }
}
