<script lang="ts">
  // DiagnosticsView.svelte — the pro screen's diagnostics surface (M8-q,
  // the ADR-0015 port). The M8-p round built the first-class evidence
  // path (a Diagnostics screen, honest copy, the system share sheet)
  // into the CLASSIC tree only — retired with it: the classic editions
  // and their variant switch are gone (one UI everywhere), and this is
  // the one surface every edition serves:
  //
  //   * the report the engine already builds (version, connection
  //     state, counters, the tunnel timing lines, the last 200 log
  //     lines — and, inside the APK, the TUN counters and the shell
  //     build identity), auto-loaded when the view opens;
  //   * Copy report — through the clipboard ladder (lib/clipboard.ts:
  //     the Android shell bridge, the async API, the execCommand
  //     fallback), with honest feedback:
  //     "Copied" only when a path really succeeded;
  //   * Share — the system share sheet, rendered only where the shell
  //     bridge offers it (the APK);
  //   * the report itself in a readonly, selectable <textarea> — the
  //     last-resort path that works even when every programmatic copy
  //     fails (press-and-hold, Select all, Copy).
  //
  // The view is a full-screen dialog over the stage: Escape and the back
  // button close it, Tab wraps inside, and focus returns to the menu
  // button (the entry point) on close.
  import { createEventDispatcher, onDestroy } from "svelte";
  import type { Backend, NetworkTestStatus, Settings } from "./backend";
  import { canShare, copyText, shareText } from "./clipboard";
  import { ChevronLeft, Copy, Share2, RefreshCw, Check } from "./icons";

  export let backend: Backend | null = null;

  const dispatch = createEventDispatcher<{ close: void }>();

  let report: string | null = null;
  let loading = false;
  let failed = false;
  let copied = false;
  let copiedTimer: number | undefined;
  let copyFailed = false;
  let copyFailedTimer: number | undefined;

  // ---- The debug.24 experiments section (spec W4/W7) ----
  // The network test lane's availability gates the whole section: the
  // Android WebView and the Linux web edition have it, the Windows
  // desktop and the offline preview do not (null hides it).
  let experimentsAvailable = false;
  let settings: Settings | null = null;
  let methodSaving = false;
  let toggleSaving = false;
  let experimentError: string | null = null;
  let experimentErrorTimer: number | undefined;
  let testRunning = false;
  let testLines: string[] | null = null;
  let testPoll: number | undefined;

  /** The share sheet exists only behind the APK shell bridge. */
  const shareable = canShare();

  const METHODS: Array<{ id: string; label: string }> = [
    { id: "legacy", label: "Legacy" },
    { id: "strong", label: "Strong" },
    { id: "balanced", label: "Balanced" },
    { id: "light", label: "Light" },
  ];

  function noteExperimentError(text: string): void {
    experimentError = text;
    window.clearTimeout(experimentErrorTimer);
    experimentErrorTimer = window.setTimeout(() => (experimentError = null), 6000);
  }

  async function loadExperiments(): Promise<void> {
    if (!backend) return;
    const status = await backend.getNetworkTestStatus();
    if (status === null) {
      experimentsAvailable = false;
      return;
    }
    experimentsAvailable = true;
    settings = await backend.getSettings();
    if (status.running) {
      testRunning = true;
      pollTest();
    } else if (status.done && status.lines) {
      testLines = status.lines;
    }
  }

  /** Saves the full settings document with one field changed (the
   *  spread keeps every field this screen never touches — a partial
   *  save would zero them server-side). */
  async function saveSettingsPatched(patch: (s: Settings) => Settings): Promise<void> {
    if (!backend || !settings) return;
    const next = patch({ ...settings, advanced: { ...(settings.advanced ?? {}) } });
    try {
      settings = await backend.saveSettings(next);
    } catch (e) {
      noteExperimentError(`Could not save the setting (${e instanceof Error ? e.message : "error"}).`);
    }
  }

  async function onMethod(id: string): Promise<void> {
    if (methodSaving || !settings || settings.method === id) return;
    methodSaving = true;
    await saveSettingsPatched((s) => ({ ...s, method: id }));
    methodSaving = false;
    // The report line the tester pastes says the world it came from.
    void load();
  }

  async function onToggle(key: "experimentSubstitution" | "experimentBreaker"): Promise<void> {
    if (toggleSaving || !settings) return;
    toggleSaving = true;
    const now = !((settings.advanced?.[key] as boolean | undefined) ?? true);
    await saveSettingsPatched((s) => ({ ...s, advanced: { ...s.advanced, [key]: now } }));
    toggleSaving = false;
  }

  function toggleState(key: "experimentSubstitution" | "experimentBreaker"): boolean {
    return (settings?.advanced?.[key] as boolean | undefined) ?? true;
  }

  async function onRunTest(): Promise<void> {
    if (!backend || testRunning) return;
    try {
      const err = await backend.runNetworkTest();
      if (err === "NOT_ON") {
        noteExperimentError("Turn Younblock on first — the test dials through the engine.");
        return;
      }
      if (err === "RUNNING") {
        testRunning = true;
        return;
      }
      if (err) {
        noteExperimentError(`The test could not start (${err}).`);
        return;
      }
      testRunning = true;
      testLines = null;
      pollTest();
    } catch (e) {
      noteExperimentError(`The test could not start (${e instanceof Error ? e.message : "error"}).`);
    }
  }

  function pollTest(): void {
    window.clearInterval(testPoll);
    testPoll = window.setInterval(async () => {
      if (!backend) return;
      const status: NetworkTestStatus | null = await backend.getNetworkTestStatus();
      if (!status) return;
      if (!status.running) {
        testRunning = false;
        window.clearInterval(testPoll);
        if (status.done && status.lines) testLines = status.lines;
        void load(); // the report now carries the table too
      }
    }, 2000);
  }

  let backBtn: HTMLElement | undefined;
  let viewEl: HTMLElement | undefined;

  async function load(): Promise<void> {
    if (!backend || loading) return;
    loading = true;
    failed = false;
    try {
      report = await backend.getDiagnosticReport();
      if (report === null) {
        failed = true;
        report = null;
      }
    } catch {
      failed = true;
      report = null;
    } finally {
      loading = false;
    }
  }

  // Mount = fetch once. The component only exists while open (App.svelte
  // renders it conditionally), so every open re-reads a fresh snapshot.
  void load();
  void loadExperiments();

  async function onCopy(): Promise<void> {
    if (report === null) return;
    const ok = await copyText(report);
    copyFailed = !ok;
    copied = ok;
    if (ok) {
      window.clearTimeout(copyFailedTimer);
      copiedTimer = window.setTimeout(() => (copied = false), 2000);
    } else {
      window.clearTimeout(copiedTimer);
      copyFailedTimer = window.setTimeout(() => (copyFailed = false), 6000);
    }
  }

  function onShare(): void {
    if (report === null) return;
    // The bridge reports dispatch, not delivery; its own failure is
    // logged by the shell. There is no honest page-side feedback beyond
    // the share sheet visibly appearing (or not).
    shareText(report);
  }

  function close(): void {
    dispatch("close");
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    // Tab (and Shift+Tab) wrap inside the dialog — focus must not leak
    // to the dimmed stage behind it.
    if (e.key !== "Tab" || !viewEl) return;
    const focusables = Array.from(
      viewEl.querySelectorAll<HTMLElement>(
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

  // The dialog takes focus when it opens (the back button — the way out
  // is the first thing a keyboard user needs).
  window.setTimeout(() => backBtn?.focus({ preventScroll: true }), 20);

  onDestroy(() => {
    window.clearTimeout(copiedTimer);
    window.clearTimeout(copyFailedTimer);
    window.clearTimeout(experimentErrorTimer);
    window.clearInterval(testPoll);
  });
</script>

<svelte:window on:keydown={onKeydown} />

<div
  class="view"
  bind:this={viewEl}
  role="dialog"
  aria-modal="true"
  aria-label="Diagnostics"
>
  <header class="view-head">
    <button type="button" class="back" bind:this={backBtn} on:click={close} aria-label="Back">
      <ChevronLeft class="back-icon" />
    </button>
    <span class="view-title">Diagnostics</span>
    <span class="head-spacer" aria-hidden="true"></span>
  </header>

  <div class="content">
    <p class="intro">
      A support report: version, connection state, counters and the last log lines — no website
      names unless you turned them on in the advanced settings. Copy it and paste it into the chat
      when you report a problem.
    </p>

    <div class="actions" role="group" aria-label="Diagnostics actions">
      <button
        type="button"
        class="primary"
        on:click={onCopy}
        disabled={report === null || loading}
      >
        {#if copied}
          <Check class="btn-icon" />
        {:else}
          <Copy class="btn-icon" />
        {/if}
        <span>{copied ? "Copied" : "Copy report"}</span>
      </button>
      {#if shareable}
        <button type="button" class="secondary" on:click={onShare} disabled={report === null || loading}>
          <Share2 class="btn-icon" />
          <span>Share</span>
        </button>
      {/if}
      <button type="button" class="secondary" on:click={load} disabled={loading}>
        <RefreshCw class={'btn-icon' + (loading ? ' spin' : '')} />
        <span>{loading ? "Creating…" : "Refresh"}</span>
      </button>
    </div>

    {#if copied}
      <p class="status ok" role="status">Copied — paste it into the chat.</p>
    {:else if copyFailed}
      <p class="status err" role="alert">
        Copying failed — press and hold the report below, choose Select all, then Copy.
      </p>
    {/if}

    {#if experimentsAvailable}
      <section class="experiments" aria-label="Experiments">
        <h2 class="exp-title">Experiments (debug.24)</h2>
        <p class="exp-note">
          The same-minute A/B switches. Change one thing, press Run network test, play a
          video — the report says which world its numbers came from.
        </p>

        <div class="exp-row" role="group" aria-label="Fragmentation method">
          <span class="exp-label">Method</span>
          <div class="segmented">
            {#each METHODS as m (m.id)}
              <button
                type="button"
                class="seg"
                aria-pressed={settings?.method === m.id}
                disabled={methodSaving}
                on:click={() => onMethod(m.id)}
              >
                {m.label}
              </button>
            {/each}
          </div>
          {#if settings}
            {@const currentMethod = settings.method}
            {#if !METHODS.some((m) => m.id === currentMethod)}
              <span class="exp-hint">current: {currentMethod}</span>
            {/if}
          {/if}
        </div>

        <div class="exp-row" role="group" aria-label="Experiment switches">
          <button
            type="button"
            class="toggle"
            aria-pressed={toggleState("experimentSubstitution")}
            disabled={toggleSaving}
            on:click={() => onToggle("experimentSubstitution")}
          >
            <span class="toggle-label">Substitution</span>
            <span class="toggle-state" data-on={toggleState("experimentSubstitution")}>
              {toggleState("experimentSubstitution") ? "on" : "off"}
            </span>
          </button>
          <button
            type="button"
            class="toggle"
            aria-pressed={toggleState("experimentBreaker")}
            disabled={toggleSaving}
            on:click={() => onToggle("experimentBreaker")}
          >
            <span class="toggle-label">Emission breaker</span>
            <span class="toggle-state" data-on={toggleState("experimentBreaker")}>
              {toggleState("experimentBreaker") ? "on" : "off"}
            </span>
          </button>
        </div>

        <button type="button" class="primary run" on:click={onRunTest} disabled={testRunning}>
          {testRunning ? "Running the test…" : "Run network test"}
        </button>
        <p class="exp-note">
          Probes the current video addresses under all four methods in the same minute —
          takes up to a few minutes on a fully silent network. The table lands in the
          report below.
        </p>
        {#if experimentError}
          <p class="status err" role="alert">{experimentError}</p>
        {/if}
        {#if testRunning}
          <p class="state">Running — keep this screen open until it finishes.</p>
        {/if}
        {#if testLines}
          <pre class="test-table">{testLines.join("\n")}</pre>
        {/if}
      </section>
    {/if}

    {#if loading && report === null}
      <p class="state">Creating the report…</p>
    {:else if failed}
      <p class="state">The report could not be created.</p>
      <button type="button" class="secondary" on:click={load}>Retry</button>
    {:else if report !== null}
      <textarea
        class="report"
        readonly
        spellcheck="false"
        aria-label="Diagnostic report"
        rows="16"
        value={report}
      ></textarea>
      <p class="note">
        The whole report is selectable — press and hold inside it, choose Select all, then Copy,
        if the button above ever fails.
      </p>
    {/if}
  </div>
</div>

<style>
  .view {
    position: fixed;
    inset: 0;
    z-index: 40; /* above the sheet (30) and its backdrop (20) */
    display: flex;
    flex-direction: column;
    background: var(--sheet-bg);
    animation: view-in 200ms var(--ease-out);
  }
  @keyframes view-in {
    from {
      opacity: 0;
      transform: translateY(0.75rem);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  .view-head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: calc(0.875rem + env(safe-area-inset-top, 0px)) 1rem 0.875rem;
    border-bottom: 1px solid var(--sheet-border);
  }
  .back {
    flex: none;
    width: var(--menu-btn);
    height: var(--menu-btn);
    display: grid;
    place-items: center;
    border-radius: 50%;
    border: 1px solid var(--chip-border);
    background: var(--chip-bg);
    color: var(--text-secondary);
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition:
      background 150ms ease,
      transform 150ms ease;
  }
  .back:hover {
    background: rgba(255, 255, 255, 0.16);
    color: var(--text-primary);
  }
  .back:active {
    transform: scale(0.94);
  }
  .back:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .back :global(svg) {
    width: 1.25rem;
    height: 1.25rem;
    color: currentColor;
  }
  .view-title {
    flex: 1;
    font-size: 0.9375rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.01em;
    color: var(--text-primary);
  }
  .head-spacer {
    flex: none;
    width: var(--menu-btn);
    height: var(--menu-btn);
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 1.125rem 1.375rem calc(1.5rem + env(safe-area-inset-bottom, 0px));
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }

  .intro {
    margin: 0;
    font-size: 0.8125rem;
    line-height: 1.55;
    color: var(--text-secondary);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .primary,
  .secondary {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    min-height: 2.75rem; /* the sheet's 44px control height */
    padding: 0 1.25rem;
    border-radius: 0.75rem;
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .primary {
    border: 1px solid transparent;
    background: var(--red-500);
    color: #fff;
  }
  .primary:hover:not(:disabled) {
    background: var(--red-400);
  }
  .primary:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .secondary {
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.12);
    color: var(--text-secondary);
  }
  .secondary:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
  }
  .secondary:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .primary:focus-visible,
  .secondary:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .primary :global(.btn-icon),
  .secondary :global(.btn-icon) {
    flex: none;
    width: 1rem;
    height: 1rem;
  }
  .secondary :global(.btn-icon.spin) {
    animation: spin var(--dur-spin) linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .status {
    margin: 0.75rem 0 0;
    font-size: 0.8125rem;
    line-height: 1.5;
  }
  .status.ok {
    color: var(--green-led);
  }
  .status.err {
    color: var(--red-400);
  }

  .state {
    margin: 1rem 0 0;
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }

  .report {
    display: block;
    width: 100%;
    margin-top: 1rem;
    min-height: 12rem;
    max-height: 22rem;
    padding: 0.75rem;
    box-sizing: border-box;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(0, 0, 0, 0.25);
    color: var(--text-secondary);
    font-size: 0.6875rem;
    line-height: 1.5;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    resize: vertical;
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }
  .report:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }

  .note {
    margin: 0.625rem 0 0;
    font-size: 0.75rem;
    line-height: 1.5;
    color: var(--text-tertiary);
  }

  /* ---- the debug.24 experiments section ---- */
  .experiments {
    margin-top: 1.75rem;
    padding-top: 1.25rem;
    border-top: 1px solid var(--sheet-border);
  }
  .exp-title {
    margin: 0;
    font-size: 0.875rem;
    font-weight: var(--fw-semibold);
    color: var(--text-primary);
  }
  .exp-note {
    margin: 0.5rem 0 0;
    font-size: 0.75rem;
    line-height: 1.55;
    color: var(--text-tertiary);
  }
  .exp-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.625rem;
    margin-top: 1rem;
  }
  .exp-label {
    flex: none;
    min-width: 5.5rem;
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .segmented {
    display: inline-flex;
    border: 1px solid var(--chip-border);
    border-radius: 0.75rem;
    overflow: hidden;
  }
  .seg {
    min-height: 2.5rem;
    padding: 0 0.875rem;
    border: none;
    border-right: 1px solid var(--chip-border);
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .seg:last-child {
    border-right: none;
  }
  .seg:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.12);
  }
  .seg[aria-pressed="true"] {
    background: var(--red-500);
    color: #fff;
  }
  .seg:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .seg:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: -0.125rem;
  }
  .exp-hint {
    font-size: 0.75rem;
    color: var(--text-tertiary);
  }
  .toggle {
    flex: 1 1 12rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    min-height: 2.75rem;
    padding: 0 0.875rem;
    border-radius: 0.75rem;
    border: 1px solid var(--chip-border);
    background: var(--chip-bg);
    color: var(--text-secondary);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .toggle:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.16);
  }
  .toggle:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .toggle:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .toggle-label {
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
  }
  .toggle-state {
    flex: none;
    min-width: 2.25rem;
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
    text-align: center;
    font-size: 0.6875rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.04em;
    background: rgba(255, 255, 255, 0.14);
    color: var(--text-tertiary);
  }
  .toggle-state[data-on="true"] {
    background: var(--green-led);
    color: #06240f;
  }
  .run {
    margin-top: 1rem;
  }
  .test-table {
    margin: 0.75rem 0 0;
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
    max-height: 22rem;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }
</style>
