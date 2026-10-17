<script lang="ts">
  // MenuSheet.svelte — the sheet behind the menu button: About / How it
  // works / Help. Escape and the red Close button dismiss it. While open
  // it behaves like a modal: focus moves in, Tab wraps inside, and focus
  // returns to the menu button on close.
  //
  // M6 (doc 05 §M6, doc 07 §5): on Android the sheet gains the honest
  // mobile surfaces — the Apps-to-unblock row (the `apps` settings field,
  // read and saved through the backend), the VPN-slot/key-icon About
  // lines, the privacy disclosure, and the diagnostics view. Everything
  // is gated on what the HOST actually reports (AppInfo.platform and
  // capabilities.perAppRouting): the desktop sheet renders exactly the
  // sections it always had — the same copy, byte for byte — and a
  // preview build with no host shows no Android-only sections at all.
  import { createEventDispatcher } from "svelte";
  import type { Backend, Settings } from "./backend";

  export let open = false;
  /** Host app version from the backend (backend.getAppInfo); empty hides
   *  the line — a dev preview has no host, and a hard-coded copy of the
   *  number here would go stale on the next bump (it already did once). */
  export let version = "";
  /** Host platform from AppInfo ("android" on the phone; "" when absent). */
  export let platform = "";
  /** True only when the host reports the per-app routing capability. */
  export let perAppRouting = false;
  /** The backend for settings/report reads and saves (null in preview). */
  export let backend: Backend | null = null;

  const dispatch = createEventDispatcher<{ close: void }>();

  const android = platform === "android";

  // =====================================================================
  // CONFIG — edit the three texts here
  // =====================================================================
  const ABOUT = [
    "Younblock helps YouTube load in your browser when the network gets in the way. It runs only on your PC, with no account and no analytics.",
    "It is not a VPN: it does not hide your IP address and adds no encryption — YouTube's own HTTPS still protects the content.",
  ];

  // The Android About lines: the first keeps the product promise; the
  // second replaces the desktop "not a VPN" line with the honest
  // VPN-slot statement (ADR-0007 + doc 07 §1: no overstated VPN/privacy/
  // coverage language — the app occupies Android's VPN slot, says so,
  // and says what that does NOT mean); the third is the .ir scope rule
  // (D83 — never processed, always direct); the fourth is the
  // open-source attribution doc 07 §7 requires for the Android lane's
  // permissive dependencies.
  const ABOUT_ANDROID = [
    "Younblock helps YouTube load when the network gets in the way. It runs only on your device, with no account and no analytics.",
    "It uses Android's VPN slot — that is the key icon you'll see while it runs — as a local traffic router for the apps you choose. It is still not a VPN: it does not hide your IP address and adds no encryption. YouTube's own HTTPS still protects the content.",
    "Websites ending in .ir are never processed: no unblocking, no fragmentation — they always connect directly, exactly as if Younblock were not running.",
    "Open-source components: the gVisor network stack (Apache-2.0) and gomobile (BSD-3).",
  ];

  const HOW_IT_WORKS = [
    "Your browser sends its request to Younblock, which runs only on your PC.",
    "Younblock looks up YouTube's address with a secure DNS service, so the lookup can't be tampered with on the way.",
    "The first “hello” message to YouTube is sent in small pieces. Filters that scan it for the site's name usually don't recognise the pieces.",
    "After that, everything flows normally, protected by YouTube's own HTTPS.",
  ];

  // Android differs only in the first line: nothing "sends its request
  // to Younblock" here — the listed apps' traffic is routed in through
  // the VPN slot (the whole flow stays on-device). D86 put the YouTube
  // app on the default list; D87 (Ali's second ruling) extended it to
  // the Chrome, Firefox, Opera and Brave browsers.
  const HOW_IT_WORKS_ANDROID = [
    "The apps on your list — the YouTube app and the Chrome, Firefox, Opera and Brave browsers are added by default — have their traffic routed through Younblock, on your device. Nothing else is routed.",
    HOW_IT_WORKS[1],
    HOW_IT_WORKS[2],
    HOW_IT_WORKS[3],
  ];

  const HELP = [
    "YouTube still doesn't load — turn Younblock off and on again, then fully close your browser and reopen it.",
    "The window disappeared — Younblock keeps running in the tray, near the clock. Click its icon to reopen it; choose Quit to stop it completely.",
    "My internet stopped after the app closed — the setting restores itself the next time the app starts.",
  ];

  const HELP_ANDROID = [
    "YouTube still doesn't load — turn Younblock off and on again, then restart the YouTube app or your browser.",
    "The key icon at the top of the screen — that is Android showing a VPN is active. It is Younblock routing only the apps on your list; tapping the icon opens Android's VPN settings, where you can revoke it any time.",
    "An app you added doesn't seem routed — changes to the list apply the next time you connect. Turn Younblock off and on, and check the app is actually installed.",
  ];

  // The privacy disclosure (doc 07 §5, ADR-0007): every claim below is
  // verified against the implementation — no analytics code exists, no
  // Younblock-operated endpoint exists, DNS goes to the user-configured
  // DoH providers, logs are local and rotating. The last line states the
  // .ir rule and its one honest limit (D83: never routed; lookups still
  // use the DoH providers — on Android the app has no other resolver).
  const PRIVACY = [
    "Younblock collects nothing: no analytics, no crash reports, no account.",
    "Nothing is sent to any Younblock-run server — none exists. The routed apps' traffic goes to its own destinations, and their DNS lookups go to the secure-DNS services you configure.",
    "Your IP address is not hidden and no extra encryption is added.",
    "Websites ending in .ir are never routed or unblocked — they connect directly. Their name lookups still use your secure-DNS services: on Android the app has no other resolver.",
    "Logs stay on this device and you can clear them from Android's app settings.",
  ];
  // =====================================================================

  // ---- the apps list (Android only) ------------------------------------

  /** Friendly labels for the known packages (the D87 default list, in
   *  its order — the quick-add chips render in this order too);
   *  anything else shows its package name — never a guess at what the
   *  app might be. */
  const APP_LABELS: Record<string, string> = {
    "com.google.android.youtube": "YouTube",
    "com.android.chrome": "Chrome",
    "org.mozilla.firefox": "Firefox",
    "com.opera.browser": "Opera",
    "com.brave.browser": "Brave",
  };

  /** Friendly-name aliases the add field accepts (D86 opened the list;
   *  D87 — "Add chrome to the default addlist as well, also firefox,
   *  opera, brave browser" — extends both the default and the aliases):
   *  typing "youtube", "chrome", "firefox", "opera" or "brave"
   *  resolves to the package instead of being refused as an invalid
   *  package shape — the debug.1 entry path demanded exact package
   *  strings, which is how the product's own namesake ended up
   *  un-addable and nothing ever routed (Ali's first device
   *  observation). */
  const APP_ALIASES: Record<string, string> = {
    youtube: "com.google.android.youtube",
    chrome: "com.android.chrome",
    firefox: "org.mozilla.firefox",
    opera: "com.opera.browser",
    brave: "com.brave.browser",
    "brave browser": "com.brave.browser",
  };

  /** Resolve a user-typed entry: a known friendly name becomes its
   *  package; anything else crosses as typed (the engine's save
   *  response stays the validation authority). */
  function resolveAppInput(raw: string): string {
    return APP_ALIASES[raw.trim().toLowerCase()] ?? raw.trim();
  }

  let settings: Settings | null = null;
  let appsBusy = false;
  let appsError = "";
  let newApp = "";

  $: apps = settings?.apps ?? [];

  /** The known apps not yet on the list — the quick-add chips (D86
   *  made adding the product's own namesake one tap instead of a
   *  package-name spelling test; D87 widens the chip row to the whole
   *  default list, so a curated list can regain any default app with
   *  one tap too). */
  $: quickAdds = Object.keys(APP_LABELS).filter((p) => !apps.includes(p));

  // Load the settings once the sheet is first opened on a host that
  // actually has them (per-app routing capability). Re-reading on every
  // open keeps the list fresh if the classic screen saved meanwhile.
  let loadedForOpen = false;
  $: if (open && perAppRouting && backend && !loadedForOpen) {
    loadedForOpen = true;
    void refreshSettings();
  }
  $: if (!open) loadedForOpen = false;

  async function refreshSettings(): Promise<void> {
    if (!backend) return;
    try {
      settings = await backend.getSettings();
    } catch {
      settings = null;
    }
  }

  function appLabel(pkg: string): string {
    return APP_LABELS[pkg] ?? pkg;
  }

  async function saveApps(next: string[], added?: string): Promise<void> {
    if (!backend || !settings) return;
    appsBusy = true;
    appsError = "";
    try {
      // Full-document save: every other field round-trips untouched.
      const saved = await backend.saveSettings({ ...settings, apps: next });
      settings = saved;
      if (added && !(saved.apps ?? []).includes(added)) {
        // Normalize refused the entry — honest feedback, not a silent drop.
        appsError = `“${added}” is not a valid Android package name.`;
      }
    } catch (e) {
      appsError = e instanceof Error ? `Could not save: ${e.message}` : "Could not save. Try again.";
    } finally {
      appsBusy = false;
    }
  }

  async function removeApp(pkg: string): Promise<void> {
    await saveApps(apps.filter((p) => p !== pkg));
  }

  async function addApp(): Promise<void> {
    const pkg = resolveAppInput(newApp);
    if (!pkg || appsBusy) return;
    newApp = "";
    if (apps.includes(pkg)) {
      appsError = `${appLabel(pkg)} is already on the list.`;
      return;
    }
    await saveApps([...apps, pkg], pkg);
  }

  /** One-tap add for the known apps (the quick-add chips): the same
   *  save path as typing, with the same saved-response authority. */
  async function quickAdd(pkg: string): Promise<void> {
    if (appsBusy || apps.includes(pkg)) return;
    await saveApps([...apps, pkg], pkg);
  }

  // ---- diagnostics (Android only) --------------------------------------

  let report: string | null = null;
  let reportBusy = false;
  let reportError = "";
  let copied = false;
  let copiedTimer: number | undefined;

  async function runDiagnostics(): Promise<void> {
    if (!backend || reportBusy) return;
    reportBusy = true;
    reportError = "";
    try {
      report = await backend.getDiagnosticReport();
      if (report === null) reportError = "The report could not be created.";
    } catch {
      reportError = "The report could not be created. Try again.";
    } finally {
      reportBusy = false;
    }
  }

  async function copyReport(): Promise<void> {
    if (report === null) return;
    try {
      await navigator.clipboard.writeText(report);
      copied = true;
      window.clearTimeout(copiedTimer);
      copiedTimer = window.setTimeout(() => (copied = false), 2000);
    } catch {
      reportError = "Copying failed — select the text and copy it manually.";
    }
  }

  // ---- modal focus handling (unchanged from the desktop sheet) ---------

  let closeBtn: HTMLElement | undefined;
  let sheetEl: HTMLElement | undefined;
  let restoreFocus: HTMLElement | null = null;

  // A modal sheet takes focus when it opens and hands it back when it
  // closes. The tiny delay lets the visibility transition release the
  // element first (a hidden element cannot be focused). preventScroll
  // matters: the closed sheet still sits below the stage (translateY),
  // and a plain focus() would scroll .app to reveal it, shoving the
  // whole screen up for a moment.
  $: if (open) {
    if (!restoreFocus) {
      restoreFocus = document.activeElement as HTMLElement | null;
    }
    window.setTimeout(() => closeBtn?.focus({ preventScroll: true }), 20);
  } else {
    restoreFocus?.focus?.();
    restoreFocus = null;
  }

  function close() {
    dispatch("close");
  }

  // Escape closes; Tab (and Shift+Tab) wrap inside the sheet — a modal
  // dialog must not leak focus to the dimmed screen behind it. The
  // focusables query is generic, so every control the M6 sections add
  // (remove buttons, the add field, diagnostics buttons) is trapped too.
  function onKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    if (e.key !== "Tab" || !sheetEl) return;
    const focusables = Array.from(
      sheetEl.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      ),
    ).filter((el) => el.offsetParent !== null);
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (e.shiftKey ? document.activeElement === first : document.activeElement === last) {
      e.preventDefault();
      (e.shiftKey ? last : first).focus({ preventScroll: true });
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="backdrop" class:open aria-hidden="true" on:click={close}></div>

<div
  class="sheet"
  class:open
  bind:this={sheetEl}
  role="dialog"
  aria-modal="true"
  aria-label="About Younblock"
  aria-hidden={!open}
>
  <span class="grabber" aria-hidden="true"></span>

  <button type="button" class="close" bind:this={closeBtn} on:click={close} aria-label="Close">
    <svg viewBox="0 0 14 14" aria-hidden="true">
      <path d="M2 2l10 10M12 2L2 12" />
    </svg>
  </button>

  <div class="content">
    {#if perAppRouting}
      <section aria-labelledby="apps-heading">
        <h2 id="apps-heading">Apps to unblock</h2>
        <p>Younblock routes only the apps on this list through the VPN slot — the YouTube app and the Chrome, Firefox, Opera and Brave browsers come already added; add or remove apps to change what routes. Other apps are untouched, and websites ending in .ir are never routed: they always connect directly.</p>
        {#if settings === null}
          <p class="dim">Loading the list…</p>
        {:else}
          {#if apps.length === 0}
            <p class="dim">No apps on the list — nothing is routed. The default apps (YouTube, Chrome, Firefox, Opera, Brave) were all removed; add one back, then connect.</p>
          {/if}
          <ul class="apps" aria-label="Routed apps">
            {#each apps as pkg (pkg)}
              <li>
                <span class="app-label">{appLabel(pkg)}</span>
                {#if APP_LABELS[pkg]}
                  <span class="app-pkg">{pkg}</span>
                {/if}
                <button
                  type="button"
                  class="remove"
                  aria-label={`Remove ${appLabel(pkg)} (${pkg})`}
                  title={`Remove ${appLabel(pkg)}`}
                  disabled={appsBusy}
                  on:click={() => removeApp(pkg)}
                >
                  <svg viewBox="0 0 14 14" aria-hidden="true">
                    <path d="M3 3l8 8M11 3l-8 8" />
                  </svg>
                </button>
              </li>
            {/each}
          </ul>
          {#if quickAdds.length > 0}
            <div class="quickadd" role="group" aria-label="Quick-add an app">
              {#each quickAdds as pkg (pkg)}
                <button
                  type="button"
                  class="chip"
                  disabled={appsBusy}
                  on:click={() => quickAdd(pkg)}
                >
                  + {appLabel(pkg)}
                </button>
              {/each}
            </div>
          {/if}
          <form class="add" on:submit|preventDefault={addApp}>
            <input
              type="text"
              aria-label="Add an app — a friendly name or an Android package name"
              placeholder="youtube, chrome, brave, or com.example.app"
              autocomplete="off"
              spellcheck="false"
              bind:value={newApp}
              disabled={appsBusy}
            />
            <button type="submit" disabled={appsBusy || newApp.trim() === ""}>Add</button>
          </form>
          <p class="note">Tap a button above to add an app, or type a name — “youtube”, “chrome”, “firefox”, “opera” and “brave” are recognized; other apps need their Android package name. Changes apply the next time you connect.</p>
          {#if appsError}<p class="inline-error" role="alert">{appsError}</p>{/if}
        {/if}
      </section>
    {/if}

    <section>
      <h2>About</h2>
      {#each android ? ABOUT_ANDROID : ABOUT as line}
        <p>{line}</p>
      {/each}
      {#if version}<p class="version">Younblock {version}</p>{/if}
    </section>

    <section>
      <h2>How it works</h2>
      {#each android ? HOW_IT_WORKS_ANDROID : HOW_IT_WORKS as line}
        <p>{line}</p>
      {/each}
    </section>

    {#if android}
      <section aria-labelledby="privacy-heading">
        <h2 id="privacy-heading">Privacy</h2>
        {#each PRIVACY as line}
          <p>{line}</p>
        {/each}
      </section>
    {/if}

    <section>
      <h2>Help</h2>
      {#each android ? HELP_ANDROID : HELP as line}
        <p>{line}</p>
      {/each}
    </section>

    {#if android}
      <section aria-labelledby="diag-heading">
        <h2 id="diag-heading">Diagnostics</h2>
        <p>A support report: version, connection state, counters and the last log lines — no website names unless you turned them on in the advanced settings.</p>
        {#if report === null}
          <button type="button" class="action" disabled={reportBusy} on:click={runDiagnostics}>
            {reportBusy ? "Creating…" : "Create report"}
          </button>
        {:else}
          <pre class="report" aria-label="Diagnostic report">{report}</pre>
          <div class="diag-row">
            <button type="button" class="action" on:click={copyReport}>
              {copied ? "Copied" : "Copy report"}
            </button>
            <button type="button" class="action" on:click={() => (report = null)}>Clear</button>
          </div>
        {/if}
        {#if reportError}<p class="inline-error" role="alert">{reportError}</p>{/if}
      </section>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    opacity: 0;
    pointer-events: none;
    transition: opacity 300ms ease;
  }
  .backdrop.open {
    opacity: 1;
    pointer-events: auto;
  }

  .sheet {
    position: fixed;
    left: 50%;
    bottom: 0;
    width: min(100%, 22.5rem);
    max-height: min(78%, 30.5rem);
    display: flex;
    flex-direction: column;
    margin-left: calc(min(50%, 11.25rem) * -1);
    background: var(--sheet-bg);
    border: 1px solid var(--sheet-border);
    border-bottom: none;
    border-radius: 1.5rem 1.5rem 0 0;
    padding: 0.875rem 1.375rem calc(1.5rem + env(safe-area-inset-bottom, 0px));
    transform: translateY(105%);
    transition: transform var(--dur-slide) var(--ease-out);
    visibility: hidden;
  }
  .sheet.open {
    transform: none;
    visibility: visible;
  }

  .grabber {
    flex: none;
    width: 2.5rem;
    height: 0.25rem;
    margin: 0 auto 1rem;
    border-radius: 0.25rem;
    background: rgba(255, 255, 255, 0.18);
  }

  .close {
    position: absolute;
    top: 1.1rem;
    right: 1.1rem;
    width: 2.25rem;
    height: 2.25rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--red-500);
    box-shadow: 0 0.25rem 0.625rem rgba(0, 0, 0, 0.35);
    transition: transform var(--dur-fast) ease;
  }
  .close:hover {
    transform: scale(1.06);
  }
  .close:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .close svg {
    width: 0.875rem;
    height: 0.875rem;
    fill: none;
    stroke: #fff;
    stroke-width: 2;
    stroke-linecap: round;
  }

  .content {
    overflow-y: auto;
    padding-right: 0.375rem;
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }

  section {
    padding: 1.1rem 0.25rem 1.25rem;
  }
  section + section {
    border-top: 1px solid var(--sheet-border);
  }

  h2 {
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--red-400);
    margin-bottom: 0.6rem;
  }

  p {
    font-size: 0.8125rem;
    font-weight: var(--fw-medium);
    line-height: 1.55;
    color: var(--text-secondary);
  }
  p + p {
    margin-top: 0.5rem;
  }

  .version {
    margin-top: 0.75rem;
    color: var(--text-tertiary);
  }

  /* ---- the M6 apps section ---- */

  .dim,
  .note {
    color: var(--text-tertiary);
    font-size: 0.75rem;
  }

  .apps {
    list-style: none;
    margin: 0.75rem 0 0;
    padding: 0;
  }
  .apps li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem 0;
  }
  .app-label {
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    color: var(--text-secondary);
  }
  .app-pkg {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.6875rem;
    color: var(--text-tertiary);
  }
  .apps li:not(:has(.app-pkg)) .app-label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .remove {
    flex: none;
    width: 2.75rem;
    height: 2.75rem;
    margin: -0.5rem -0.5rem -0.5rem 0;
    display: grid;
    place-items: center;
    border-radius: 50%;
    border: 1px solid var(--sheet-border);
    background: transparent;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .remove:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.1);
  }
  .remove:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .remove:disabled {
    opacity: 0.35;
    cursor: default;
  }
  .remove svg {
    width: 0.625rem;
    height: 0.625rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    color: var(--text-secondary);
  }

  .add {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .quickadd {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }
  .chip {
    height: 2.25rem;
    padding: 0 0.9375rem;
    border-radius: 999px;
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.08);
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .chip:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.16);
  }
  .chip:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .chip:disabled {
    opacity: 0.35;
    cursor: default;
  }
  .add input {
    flex: 1;
    min-width: 0;
    height: 2.75rem;
    padding: 0 0.875rem;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.08);
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-family: inherit;
  }
  .add input::placeholder {
    color: var(--text-tertiary);
  }
  .add input:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .add button,
  .action {
    height: 2.75rem;
    min-width: 5.5rem;
    padding: 0 1.25rem;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.12);
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .add button:hover:not(:disabled),
  .action:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
  }
  .add button:focus-visible,
  .action:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .add button:disabled,
  .action:disabled {
    opacity: 0.45;
    cursor: default;
  }

  .inline-error {
    margin-top: 0.5rem;
    color: var(--red-400);
  }

  /* ---- the diagnostics view ---- */

  .report {
    margin: 0.75rem 0 0;
    max-height: 14rem;
    overflow-y: auto;
    padding: 0.75rem;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(0, 0, 0, 0.25);
    color: var(--text-secondary);
    font-size: 0.6875rem;
    line-height: 1.5;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    white-space: pre-wrap;
    word-break: break-word;
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }
  .diag-row {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }
</style>
