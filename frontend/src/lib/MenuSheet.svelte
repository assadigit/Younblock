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
  import { formatBytes, formatDateTime, formatDurationWords } from "./format";
  import { X, Activity, ChevronRight } from "./icons";
  import type { UsageSummary } from "./types";

  export let open = false;
  /** Host app version from the backend (backend.getAppInfo); empty hides
   *  the line — a dev preview has no host, and a hard-coded copy of the
   *  number here would go stale on the next bump (it already did once). */
  export let version = "";
  /** Host platform from AppInfo ("android" on the phone; "" when absent). */
  export let platform = "";
  /** True only when the host reports the per-app routing capability. */
  export let perAppRouting = false;
  /** True only when the host reports the connect-time hand-off
   *  capability (M8-am, D105, the launch-only shape M8-ap: Android
   *  opens the YouTube app when the engine turns on). Gates the
   *  sheet's auto-open row — a host with no target app never shows
   *  it. */
  export let autoOpenTarget = false;
  /** True only when the host reports the tray capability (the Windows
   *  desktop) — D93 gates the close setting's row on it: a platform with
   *  no tray has nothing to keep running, so the row never shows there. */
  export let tray = false;
  /** The backend for settings/report reads and saves (null in preview). */
  export let backend: Backend | null = null;

  const dispatch = createEventDispatcher<{ close: void; diagnostics: void }>();

  const android = platform === "android";

  // =====================================================================
  // CONFIG — edit the texts here
  // =====================================================================
  /** Round-2 fix #8: the credit line, shown with the version in About. */
  const CREDIT = "Created by ChitaCodes";
  /** Websites confirmed to open through Younblock (site-test-plan.md): a
   *  site lands here only after it was really blocked without Younblock and
   *  really opened with it, on two different days or networks. Until then
   *  the honest line below is all the menu claims. */
  const VERIFIED_SITES: string[] = [];
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
    "The window disappeared — you turned on “Keep running in the tray when closed”, so Younblock is still running near the clock. Click its icon to reopen it; choose Quit to stop it completely.",
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
  // The D93 close setting (tray platforms only): off is the shipped
  // default — the close button is a real shutdown — so the copy says
  // exactly what each position does before the user flips anything.
  const KEEP_IN_TRAY_TITLE = "Keep running in the tray when closed";
  const KEEP_IN_TRAY_HINT =
    "Off by default: closing the window stops Younblock completely. On: closing it keeps the app running in the tray near the clock — right-click the tray icon and choose Quit to stop it.";
  // The M8-am connect-time hand-off (D105: on is the shipped default on
  // Android). The copy says exactly what happens at connect time, so
  // flipping it is an informed choice, not a surprise launch.
  const AUTO_OPEN_TITLE = "Open YouTube when Younblock connects";
  const AUTO_OPEN_HINT =
    "On by default: the moment the connection is ready, Younblock opens the YouTube app for you, so the first video starts through the ready tunnel. Off: nothing is opened for you.";

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
  // actually has them (per-app routing, the connect-time hand-off, or
  // the tray capability — the surfaces that read settings.json).
  // Re-reading on every open keeps the state fresh if another
  // edition's screen saved meanwhile.
  let loadedForOpen = false;
  $: if (open && (perAppRouting || autoOpenTarget || tray) && backend && !loadedForOpen) {
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

  // ---- the close setting (D93, tray platforms only) --------------------

  let trayBusy = false;
  let trayError = "";

  /** Full-document save, the apps row's path: every other field
   *  round-trips untouched. On failure the checkbox snaps back to the
   *  last saved state — the row never shows a state the engine did not
   *  accept. */
  async function saveKeepInTray(next: boolean): Promise<void> {
    if (!backend || !settings || trayBusy) return;
    trayBusy = true;
    trayError = "";
    try {
      settings = await backend.saveSettings({ ...settings, keepInTrayOnClose: next });
    } catch (e) {
      trayError = e instanceof Error ? `Could not save: ${e.message}` : "Could not save. Try again.";
    } finally {
      trayBusy = false;
    }
  }

  // ---- the connect-time hand-off (M8-am, autoOpenTarget platforms) ------

  let autoOpenBusy = false;
  let autoOpenError = "";

  /** Full-document save, the keep-in-tray row's path verbatim: the
   *  whole settings document round-trips, every other field untouched,
   *  and on failure the checkbox snaps back to the last saved state. */
  async function saveAutoOpen(next: boolean): Promise<void> {
    if (!backend || !settings || autoOpenBusy) return;
    autoOpenBusy = true;
    autoOpenError = "";
    try {
      settings = await backend.saveSettings({ ...settings, autoOpenYouTube: next });
    } catch (e) {
      autoOpenError = e instanceof Error ? `Could not save: ${e.message}` : "Could not save. Try again.";
    } finally {
      autoOpenBusy = false;
    }
  }

  // ---- data usage (round-3, every platform) ------------------------------

  // The menu's top section: totals for Today / This month / All time, the
  // last sessions, and a confirmed Clear. The host owns the history (one
  // JSON file, times and byte totals only); the sheet asks for a fresh
  // summary EVERY time it opens — the round-3 screen rule, so another
  // window's session or a previous sheet visit can never show stale rows.
  let usage: UsageSummary | null = null;
  let usageError = false;
  let usageBusy = false;
  let confirmClear = false;

  $: if (open && backend) void loadUsage();

  async function loadUsage(): Promise<void> {
    if (!backend) return;
    usageBusy = true;
    usageError = false;
    confirmClear = false;
    try {
      usage = await backend.getUsage();
      if (usage === null) usageError = true;
    } catch {
      usage = null;
      usageError = true;
    } finally {
      usageBusy = false;
    }
  }

  /** Delete (red, after the confirmation) vs Keep — only Delete clears. */
  async function clearHistory(): Promise<void> {
    if (!backend || usageBusy) return;
    usageBusy = true;
    try {
      await backend.clearUsage();
      await loadUsage();
    } catch {
      usageError = true;
    } finally {
      usageBusy = false;
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
  aria-label="Younblock menu"
  aria-hidden={!open}
>
  <span class="grabber" aria-hidden="true"></span>

  <!-- Round-2 fix #5: the sheet's header lives INSIDE the sheet — title
       left, neutral 44px X right — instead of the old red circle that
       floated half outside; the body scrolls underneath it. -->
  <header class="sheet-head">
    <span class="sheet-title">Younblock</span>
    <button type="button" class="close" bind:this={closeBtn} on:click={close} aria-label="Close">
      <X class="close-icon" />
    </button>
  </header>

  <div class="content">
    <!-- M8-q (the ADR-0015 port): the diagnostics entry leads the sheet.
         The old surface lived at the very BOTTOM of this sheet (below
         Help) behind an android gate, its Copy used the raw
         navigator.clipboard call that silently fails inside the Android
         WebView — and the M8-p rewrite shipped only in the since-retired
         classic tree. One first-class entry on every platform
         now: it opens the full DiagnosticsView (report + the honest
         clipboard ladder + Share + selectable text). -->
    <button type="button" class="diag-entry" on:click={() => dispatch("diagnostics")}>
      <span class="diag-icon" aria-hidden="true">
        <Activity />
      </span>
      <span class="diag-text">
        <span class="diag-title">Diagnostics</span>
        <span class="diag-hint">Copy a support report to paste into a chat</span>
      </span>
      <ChevronRight class="diag-chev" aria-hidden="true" />
    </button>

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

    <!-- Round-3: the Data usage section sits directly under the apps row on
         Android and at the very top on the desktops — one markup, both
         platforms, the same numbers the main screen counts live. -->
    <section aria-labelledby="usage-heading">
      <h2 id="usage-heading">Data usage</h2>
      {#if usageError}
        <p class="dim">Could not read the usage history.</p>
      {:else if usage === null}
        <p class="dim">Loading…</p>
      {:else}
        <div class="usage-cards" role="group" aria-label="Data usage totals">
          <div class="ucard">
            <span class="unum">{formatBytes(usage.todayBytes)}</span>
            <span class="ucap">Today</span>
          </div>
          <div class="ucard">
            <span class="unum">{formatBytes(usage.monthBytes)}</span>
            <span class="ucap">This month</span>
          </div>
          <div class="ucard">
            <span class="unum">{formatBytes(usage.allTimeBytes)}</span>
            <span class="ucap">All time</span>
          </div>
        </div>
        {#if usage.sessions.length === 0}
          <p class="dim">No sessions yet. Every session you run will be listed here.</p>
        {:else}
          <ul class="sessions" aria-label="Recent sessions">
            {#each usage.sessions as s (s.startMs)}
              <li>
                <span class="when">{formatDateTime(s.endMs)}</span>
                <span class="howlong">{formatDurationWords(s.endMs - s.startMs)}</span>
                <span class="size">{formatBytes(s.downBytes + s.upBytes)}</span>
              </li>
            {/each}
          </ul>
        {/if}
        <p class="note">Counts the data that passes through Younblock, saved only on {android ? "this device" : "this PC"}. Your internet provider may count a little more.</p>
        {#if confirmClear}
          <div class="confirm" role="group" aria-label="Delete the whole history?">
            <span>Delete the whole history?</span>
            <button type="button" class="action danger" disabled={usageBusy} on:click={clearHistory}>Delete</button>
            <button type="button" class="action" disabled={usageBusy} on:click={() => (confirmClear = false)}>Keep</button>
          </div>
        {:else}
          <button
            type="button"
            class="action"
            disabled={usageBusy || usage.sessionCount === 0}
            on:click={() => (confirmClear = true)}
          >
            Clear history
          </button>
        {/if}
      {/if}
    </section>

    <!-- The settings section renders for whichever host rows it holds:
         D93's close setting on tray hosts (the Windows desktop — every
         other platform has nothing to keep running), and M8-am's
         connect-time hand-off on autoOpenTarget hosts (Android). One
         sheet, honest per host — a platform with neither never shows
         the section. -->
    {#if tray || autoOpenTarget}
      <section aria-labelledby="settings-heading">
        <h2 id="settings-heading">Settings</h2>
        {#if settings === null}
          <p class="dim">Loading…</p>
        {:else}
          {#if tray}
            <div class="setting-row">
              <div class="setting-text">
                <span class="setting-title">{KEEP_IN_TRAY_TITLE}</span>
                <span class="setting-hint">{KEEP_IN_TRAY_HINT}</span>
              </div>
              <label class="toggle">
                <input
                  type="checkbox"
                  role="switch"
                  aria-label={KEEP_IN_TRAY_TITLE}
                  checked={settings.keepInTrayOnClose ?? false}
                  disabled={trayBusy}
                  on:change={(e) => saveKeepInTray(e.currentTarget.checked)}
                />
                <span class="toggle-track" aria-hidden="true"><span class="toggle-thumb"></span></span>
              </label>
            </div>
            {#if trayError}<p class="inline-error" role="alert">{trayError}</p>{/if}
          {/if}
          {#if autoOpenTarget}
            <div class="setting-row">
              <div class="setting-text">
                <span class="setting-title">{AUTO_OPEN_TITLE}</span>
                <span class="setting-hint">{AUTO_OPEN_HINT}</span>
              </div>
              <label class="toggle">
                <input
                  type="checkbox"
                  role="switch"
                  aria-label={AUTO_OPEN_TITLE}
                  checked={settings.autoOpenYouTube ?? true}
                  disabled={autoOpenBusy}
                  on:change={(e) => saveAutoOpen(e.currentTarget.checked)}
                />
                <span class="toggle-track" aria-hidden="true"><span class="toggle-thumb"></span></span>
              </label>
            </div>
            {#if autoOpenError}<p class="inline-error" role="alert">{autoOpenError}</p>{/if}
          {/if}
        {/if}
      </section>
    {/if}

    <section>
      <h2>About</h2>
      {#each android ? ABOUT_ANDROID : ABOUT as line}
        <p>{line}</p>
      {/each}
      <p class="version">{version ? `Younblock ${version} · ` : ""}{CREDIT}</p>
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

    <!-- Round-2 fix #8: the honest "other websites" line — until a site
         passes the two-day test plan it is not named as working. -->
    <section aria-labelledby="other-heading">
      <h2 id="other-heading">Other websites</h2>
      {#if VERIFIED_SITES.length > 0}
        <p>Confirmed so far: {VERIFIED_SITES.join(", ")}.</p>
      {:else}
        <p>Some other blocked websites open too, but that is still being tested.</p>
      {/if}
    </section>

    <section>
      <h2>Help</h2>
      {#each android ? HELP_ANDROID : HELP as line}
        <p>{line}</p>
      {/each}
    </section>
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
    margin: 0 auto 0.75rem;
    border-radius: 0.25rem;
    background: rgba(255, 255, 255, 0.18);
  }

  /* Round-2 fix #5: the header is part of the sheet — title left, neutral
     44px X right — and it stays put while the body scrolls underneath. */
  .sheet-head {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.25rem 0 0.875rem;
    border-bottom: 1px solid var(--sheet-border);
  }
  .sheet-title {
    font-size: 0.9375rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.01em;
    color: var(--text-primary);
  }

  .close {
    flex: none;
    width: var(--menu-btn);
    height: var(--menu-btn);
    display: grid;
    place-items: center;
    border-radius: 50%;
    border: 1px solid var(--chip-border);
    background: var(--chip-bg);
    color: var(--text-secondary);
    transition:
      background 150ms ease,
      transform 150ms ease;
  }
  .close:hover {
    background: rgba(255, 255, 255, 0.16);
    color: var(--text-primary);
  }
  .close:active {
    transform: scale(0.94);
  }
  .close:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .close :global(svg) {
    width: 1.25rem;
    height: 1.25rem;
    stroke-width: 2.2;
  }

  .content {
    flex: 1;
    min-height: 0;
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
  /* the diagnostics entry (a button, not a section) sits above them all;
     * it gets the same bottom rhythm the first section would, and a
     * separator from the section list below it */
  .diag-entry + section {
    border-top: 1px solid var(--sheet-border);
    margin-top: 0.25rem;
  }

  /* ---- the diagnostics entry (M8-q, ADR-0015 — the sheet's lead row) ---- */

  .diag-entry {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    min-height: 3.5rem; /* a 56px target — the sheet's most important row */
    margin: 0.25rem 0.375rem 0.25rem 0;
    padding: 0.625rem 0.875rem;
    border-radius: 0.75rem;
    border: 1px solid var(--chip-border);
    background: var(--sheet-raised);
    color: var(--text-primary);
    font-family: inherit;
    text-align: left;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition:
      background 150ms ease,
      transform 150ms ease;
  }
  .diag-entry:hover {
    background: rgba(255, 255, 255, 0.14);
  }
  .diag-entry:active {
    transform: scale(0.99);
  }
  .diag-entry:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .diag-icon {
    flex: none;
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: 0.625rem;
    background: rgba(255, 0, 0, 0.14);
    border: 1px solid rgba(255, 0, 0, 0.25);
  }
  .diag-icon :global(svg) {
    width: 1.125rem;
    height: 1.125rem;
    color: var(--red-400);
  }
  .diag-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .diag-title {
    font-size: 0.875rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.01em;
  }
  .diag-hint {
    font-size: 0.75rem;
    font-weight: var(--fw-medium);
    line-height: 1.4;
    color: var(--text-tertiary);
  }
  .diag-entry :global(.diag-chev) {
    flex: none;
    width: 1rem;
    height: 1rem;
    color: var(--text-tertiary);
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

  /* ---- the D93 close setting ---- */

  .setting-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 2.75rem;
  }
  .setting-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .setting-title {
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    color: var(--text-secondary);
  }
  .setting-hint {
    font-size: 0.75rem;
    font-weight: var(--fw-medium);
    line-height: 1.45;
    color: var(--text-tertiary);
  }

  /* The switch: the sheet's own drawing (the old screen's toggle,
    weight-matched to the 44px controls around it). The hidden input
    covers the whole track, so the touch target is the switch itself. */
  .toggle {
    position: relative;
    display: inline-grid;
    flex: none;
  }
  .toggle input {
    position: absolute;
    inset: 0;
    width: 2.75rem;
    height: 1.625rem;
    opacity: 0;
    margin: 0;
    cursor: pointer;
  }
  .toggle-track {
    width: 2.75rem;
    height: 1.625rem;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.18);
    display: grid;
    align-items: center;
    padding: 0.1875rem;
    transition: background 200ms ease;
  }
  .toggle:hover .toggle-track {
    background: rgba(255, 255, 255, 0.28);
  }
  .toggle-thumb {
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 999px;
    background: #f4f1ee;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
    transition: transform 200ms var(--ease-out);
  }
  .toggle input:checked + .toggle-track {
    background: var(--red-500);
  }
  .toggle input:checked + .toggle-track .toggle-thumb {
    transform: translateX(1.125rem);
  }
  .toggle input:focus-visible + .toggle-track {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .toggle input:disabled + .toggle-track {
    opacity: 0.45;
  }

  /* ---- the round-3 Data usage section ---- */

  .usage-cards {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.1rem;
  }
  .ucard {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.15rem;
    padding: 0.55rem 0.25rem 0.5rem;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.05);
  }
  .unum {
    font-size: 0.875rem;
    font-weight: var(--fw-semibold);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .ucap {
    font-size: 0.6875rem;
    color: var(--text-tertiary);
  }

  .sessions {
    list-style: none;
    margin: 0.75rem 0 0;
    padding: 0;
  }
  .sessions li {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    padding: 0.3rem 0;
    font-size: 0.75rem;
    font-weight: var(--fw-medium);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .when {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .howlong {
    flex: none;
    color: var(--text-tertiary);
  }
  .size {
    flex: none;
    font-weight: var(--fw-semibold);
  }

  .confirm {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }
  .confirm span {
    flex: 1;
    min-width: 0;
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .confirm .action {
    min-width: 4.5rem;
  }
  .danger {
    color: var(--red-400);
    border-color: rgba(224, 107, 107, 0.4);
  }
  .danger:hover:not(:disabled) {
    background: rgba(224, 107, 107, 0.12);
  }

  /* The old in-sheet diagnostics view (.report/.diag-row) is gone —
     M8-q moved the whole surface into DiagnosticsView.svelte (the
     full-screen dialog the sheet's lead row opens). */
</style>
