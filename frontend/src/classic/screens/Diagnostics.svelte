<script lang="ts">
  // Diagnostics (M8-p, ADR-0015 — Ali's ask: "add an option to copy paste
  // logs and diagnostics so I can paste it here"): the report the engine
  // already builds (version, state, counters, the tunnel timing lines,
  // the last 200 log lines — and, inside the APK, the TUN counters and
  // the shell build identity) on one screen with three ways out:
  //
  //   - Copy report — through the clipboard ladder (lib/clipboard.ts):
  //     the Android shell bridge first, the async API, the execCommand
  //     fallback. Honest toasts: "Copied" only when a path succeeded.
  //   - Share — the system share sheet (APK only; hidden elsewhere),
  //     the most reliable way to hand the report to a chat app.
  //   - The text itself — a readonly, selectable <textarea>: even if
  //     every programmatic path fails, press-and-hold + select-all +
  //     copy always works, and the failure toast says exactly that.
  //
  // The screen never claims more than the engine enforces (D77): the
  // intro says WHAT the report carries, not what fixed it.
  import Icon from "../components/Icon.svelte";
  import { copy } from "../i18n";
  import { canShare, copyText, shareText } from "../lib/clipboard";
  import type { Backend } from "../lib/backend";

  interface Props {
    backend: Backend;
    onBack: () => void;
    onToast: (text: string) => void;
  }
  let { backend, onBack, onToast }: Props = $props();

  let report = $state<string | null>(null);
  let failed = $state(false);
  let loading = $state(false);
  let copied = $state(false);

  const shareable = $derived(canShare());

  async function load() {
    loading = true;
    failed = false;
    try {
      const res = await backend.getDiagnosticReport();
      report = res.report;
    } catch {
      failed = true;
      report = null;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    // Mount = fetch once. Refresh re-runs it on demand — the report is a
    // snapshot, not a stream (the state SSE already covers "live").
    void load();
  });

  async function onCopy() {
    if (!report) return;
    const ok = await copyText(report);
    onToast(ok ? copy.diagnostics.copied : copy.diagnostics.copyFailed);
    copied = ok;
    if (ok) {
      window.setTimeout(() => (copied = false), 2000);
    }
  }

  function onShare() {
    if (!report) return;
    const ok = shareText(report);
    onToast(ok ? copy.diagnostics.shared : copy.diagnostics.shareFailed);
  }

  // debug.18 (M8-z, ADR-0027 §8.3.6): the timeline stamps. "I pressed
  // play" goes right before starting a new video; "First frame" when
  // the picture appears — the report then carries the elapsed time
  // between the pair and the failed/slow flows inside that window (the
  // single number that says whether the round's work succeeded).
  let marked = $state(false);
  async function onMark(label: string) {
    try {
      await backend.mark(label);
      marked = true;
      onToast(copy.diagnostics.marked);
      window.setTimeout(() => (marked = false), 2000);
    } catch {
      // A failed stamp must not block the tester's flow; the report is
      // still copyable.
    }
  }
</script>

<section class="view" aria-label={copy.diagnostics.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1 class="view-title">{copy.diagnostics.title}</h1>
  </header>

  <div class="view-scroll">
    <div class="card">
      <p class="overline">{copy.diagnostics.title}</p>
      <p class="intro">{copy.diagnostics.intro}</p>
      <div class="actions" role="group" aria-label={copy.diagnostics.title}>
        <button type="button" class="primary" onclick={onCopy} disabled={!report} data-copied={copied}>
          {#if copied}
            <Icon name="check" size={16} />
          {:else}
            <Icon name="copy" size={16} />
          {/if}
          <span>{copied ? copy.toast.copied : copy.diagnostics.copy}</span>
        </button>
        {#if shareable}
          <button type="button" class="secondary" onclick={onShare} disabled={!report}>
            <Icon name="share" size={16} />
            <span>{copy.diagnostics.share}</span>
          </button>
        {/if}
        <button type="button" class="secondary" onclick={load} disabled={loading}>
          {#if loading}
            <span class="spinner" aria-hidden="true"></span>
          {:else}
            <Icon name="refresh" size={16} />
          {/if}
          <span>{copy.diagnostics.refresh}</span>
        </button>
      </div>
      <p class="hint">{copy.diagnostics.logDomainsHint}</p>
      <div class="actions" role="group" aria-label={copy.diagnostics.markPlay}>
        <button type="button" class="secondary" onclick={() => onMark(copy.diagnostics.markPlay)} data-marked={marked}>
          <Icon name="play" size={16} />
          <span>{copy.diagnostics.markPlay}</span>
        </button>
        <button type="button" class="secondary" onclick={() => onMark(copy.diagnostics.markFirstFrame)} data-marked={marked}>
          <Icon name="check" size={16} />
          <span>{copy.diagnostics.markFirstFrame}</span>
        </button>
      </div>
      <p class="hint">{copy.diagnostics.markHint}</p>
    </div>

    <div class="card report-card">
      <p class="overline">{copy.diagnostics.reportLabel}</p>
      {#if loading && !report}
        <p class="state">{copy.diagnostics.loading}</p>
      {:else if failed}
        <p class="state">{copy.diagnostics.loadFailed}</p>
        <button type="button" class="secondary" onclick={load}>{copy.diagnostics.retry}</button>
      {:else if report}
        <textarea
          class="report"
          readonly
          spellcheck="false"
          aria-label={copy.a11y.reportBox}
          rows="16"
          value={report}
        ></textarea>
      {/if}
    </div>
  </div>
</section>

<style>
  .view-scroll {
    gap: var(--space-4);
  }
  .intro {
    margin: 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .hint {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 17px;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .actions .primary,
  .actions .secondary {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 44px;
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
    transition:
      background var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .actions .primary {
    background: var(--red-500);
    color: var(--color-on-primary);
    font-weight: 500;
  }
  .actions .primary:hover {
    background: var(--red-400);
  }
  .actions .primary:active,
  .actions .secondary:active {
    transform: scale(0.97);
  }
  .actions .primary:disabled,
  .actions .secondary:disabled {
    opacity: 0.55;
  }
  .actions .secondary {
    border: 1px solid var(--color-border);
    color: var(--color-text);
  }
  .actions .secondary:hover {
    background: var(--color-surface-2);
  }
  .actions button:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  .actions .primary[data-copied="true"] {
    background: var(--neutral-700);
  }
  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: var(--radius-full);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .state {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  /* The report itself: monospace, one line per row, scrollable but
     bounded (long reports must not push the screen into the footer),
     and SELECTABLE — it is the copy path of last resort. */
  .report {
    width: 100%;
    min-height: 220px;
    max-height: 340px;
    resize: vertical;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 16px;
    padding: var(--space-3);
    overflow-y: auto;
    white-space: pre;
    -webkit-overflow-scrolling: touch;
  }
  .report:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 1px;
  }
  @media (prefers-reduced-motion: reduce) {
    .actions .primary,
    .actions .secondary {
      transition: none;
    }
    .actions .primary:active,
    .actions .secondary:active {
      transform: none;
    }
  }
</style>
