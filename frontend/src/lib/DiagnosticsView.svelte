<script lang="ts">
  // DiagnosticsView.svelte — the pro screen's diagnostics surface (M8-q,
  // the ADR-0015 port). The M8-p round built the first-class evidence
  // path (a Diagnostics screen, honest copy, the system share sheet)
  // into the CLASSIC tree only — but the Android APK bakes the pro
  // screen (D66: VITE_UI_VARIANT is set only by the classic desktop
  // editions; build-android.sh leaves it unset, so main.ts defaults to
  // pro). Ali's device never rendered the classic tree, so debug.9
  // shipped the promised button into a variant their phone cannot show.
  // This is the same surface in the variant every webui edition serves:
  //
  //   * the report the engine already builds (version, connection
  //     state, counters, the tunnel timing lines, the last 200 log
  //     lines — and, inside the APK, the TUN counters and the shell
  //     build identity), auto-loaded when the view opens;
  //   * Copy report — through the SAME clipboard ladder the classic
  //     screen uses (lib/clipboard.ts: the Android shell bridge, the
  //     async API, the execCommand fallback), with honest feedback:
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
  import type { Backend } from "./backend";
  import { canShare, copyText, shareText } from "../classic/lib/clipboard";
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

  /** The share sheet exists only behind the APK shell bridge. */
  const shareable = canShare();

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
</style>
