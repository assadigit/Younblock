<script lang="ts">
  // State label, subtitle, notice line, timer and the quiet line
  // (file 02, section 4.1). The timer and quiet line keep their height in
  // every state so nothing jumps. v1.2.0 polish (D45): a state-colored
  // status dot beside the label, tighter typographic rhythm, and an
  // amber ghost action for the degraded state (it reads as a warning
  // repair action, not a second "turn on" button).
  // v1.4.0 (D49): in the error state the reason IS the subtitle — right
  // under the state label, where the eye lands first. The old layout
  // reserved an empty subtitle line and then repeated the reason in a
  // separate low paragraph below the collapsed metrics, splitting one
  // message over two slots with a dead gap between them.
  import { copy, fill } from "../i18n";
  import { formatBytes, formatTimer, subtitleFor } from "../lib/format";
  import type { BackendState, Stats } from "../lib/backend";

  interface Props {
    state: BackendState;
    stats: Stats | null;
    errorText: string | null;
    noticeText: string | null;
    onOpenHelp: () => void;
    onFixConnection: () => void;
  }
  let { state, stats, errorText, noticeText, onOpenHelp, onFixConnection }: Props = $props();

  const showTimer = $derived(state.phase === "on" || state.phase === "degraded");
  const subtitle = $derived(
    state.phase === "error" && errorText
      ? errorText
      : subtitleFor(state.phase, state.detail, state.fail),
  );
  const used = $derived(
    stats ? formatBytes((stats.bytesDown ?? 0) + (stats.bytesUp ?? 0)) : null,
  );
  // The quiet line: response time first (when known), then usage.
  const quietText = $derived.by(() => {
    if (!stats) return "";
    const usage = fill(copy.stats.used, { n: used ?? "0 B" });
    if (state.latencyMs != null && state.latencyMs > 0) {
      return `${fill(copy.stats.ping, { n: String(state.latencyMs) })} · ${usage}`;
    }
    return usage;
  });
  const labels: Record<string, string> = {
    off: copy.state.off,
    connecting: copy.state.connecting,
    on: copy.state.on,
    degraded: copy.state.degraded,
    error: copy.state.error,
    stopping: copy.state.stopping,
  };
  function stateLabel(phase: string): string {
    return labels[phase] ?? phase;
  }
</script>

<div class="status" aria-live="polite">
  <p class="state-label">
    <span class="dot {state.phase}" aria-hidden="true"></span>
    <span>{stateLabel(state.phase)}</span>
  </p>
  <p
    class="subtitle"
    class:warn={state.phase === "degraded"}
    class:error={state.phase === "error"}
  >{subtitle}</p>
  <p class="notice" class:visible={!!noticeText} aria-live="polite">{noticeText ?? ""}</p>
  <!-- D48: the timer/quiet block collapses to zero height when its
       state can never show it (off/connecting/error/stopping) and grows
       back with a smooth grid-rows animation — in the error state that
       reserved 52 px used to push the reason text and "Open help" link
       past the window bottom. -->
  <div class="metrics" class:visible={showTimer} aria-hidden={!showTimer}>
    <div class="metrics-body">
      <p class="timer">{stats ? formatTimer(stats.uptimeSec) : "00:00:00"}</p>
      <p class="quiet">{quietText}</p>
    </div>
  </div>
  {#if state.phase === "error"}
    <div class="error-row">
      <button type="button" class="link" onclick={onOpenHelp}>Open help</button>
    </div>
  {/if}
  {#if state.phase === "degraded"}
    <button type="button" class="fix" onclick={onFixConnection}>Fix connection</button>
  {/if}
</div>


<style>
  .status {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--space-2);
  }
  .state-label {
    margin: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 28px;
    line-height: 34px;
    font-weight: 600;
    letter-spacing: -0.015em;
    color: var(--color-text);
  }
  /* The status dot: color alone never carries meaning, but it gives an
     instant at-a-glance read next to the word. */
  .dot {
    width: 7px;
    height: 7px;
    border-radius: var(--radius-full);
    background: var(--neutral-500);
    box-shadow: 0 0 0 3px rgba(163, 166, 173, 0.14);
    transition:
      background 320ms ease-out,
      box-shadow 320ms ease-out;
  }
  .dot.on {
    background: var(--red-400);
    box-shadow: 0 0 0 3px rgba(221, 50, 51, 0.18);
  }
  .dot.connecting {
    background: var(--color-warn);
    box-shadow: 0 0 0 3px rgba(255, 197, 61, 0.18);
    animation: dot-wait 1.6s ease-in-out infinite alternate;
  }
  .dot.degraded {
    background: var(--color-warn);
    box-shadow: 0 0 0 3px rgba(255, 197, 61, 0.18);
  }
  .dot.error {
    background: var(--color-danger);
    box-shadow: 0 0 0 3px rgba(255, 106, 95, 0.18);
  }
  .dot.stopping {
    background: var(--neutral-600);
    box-shadow: 0 0 0 3px rgba(128, 118, 113, 0.14);
  }
  @keyframes dot-wait {
    from {
      opacity: 0.55;
    }
    to {
      opacity: 1;
    }
  }
  .subtitle {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 20px;
    color: var(--color-text-muted);
    min-height: 20px;
    transition: color 320ms ease-out;
  }
  /* Degraded: the reason line is warmed up so the failure is not
     lost in gray (D45). */
  .subtitle.warn {
    color: var(--gold-300);
  }
  /* Error: the reason line carries the danger color and the D48 budget —
     a compact cap with its own scroll so a long UNKNOWN paragraph can
     never push the footer out of the window (the full text stays
     available); the max-width keeps the line length readable. */
  .subtitle.error {
    color: var(--color-danger);
    max-width: 280px;
    max-height: 100px;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: none;
  }
  .subtitle.error::-webkit-scrollbar {
    display: none;
  }
  .notice {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 20px;
    color: var(--color-text-muted);
    min-height: 20px;
    opacity: 0;
    transition: opacity 160ms ease-out;
  }
  .notice.visible {
    opacity: 1;
  }
  /* D48: 0fr <-> 1fr grid rows give a smooth height animation without
     JS; the children keep their own min-heights inside. */
  .metrics {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows 240ms var(--ease-spring-out, ease-out);
  }
  .metrics.visible {
    grid-template-rows: 1fr;
  }
  .metrics-body {
    overflow: hidden;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .timer {
    margin: 0;
    font-size: var(--text-timer);
    line-height: 32px;
    font-weight: 500;
    letter-spacing: 0.01em;
    font-variant-numeric: tabular-nums;
    font-feature-settings: "tnum";
    color: var(--color-text);
    min-height: 32px;
    opacity: 0;
    transition: opacity 160ms ease-out;
  }
  .quiet {
    margin: 0;
    font-size: var(--text-xs);
    line-height: 20px;
    letter-spacing: 0.01em;
    color: var(--neutral-500);
    min-height: 20px;
    opacity: 0;
    transition: opacity 160ms ease-out;
  }
  .metrics.visible .timer,
  .metrics.visible .quiet {
    opacity: 1;
  }
  /* D49: the reason itself moved up into the subtitle slot, so the
     error row now carries only the "Open help" link. */
  .error-row {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    margin-top: var(--space-1);
    max-width: 280px;
  }
  .link {
    color: var(--color-text-muted);
    text-decoration: underline;
    min-height: 44px;
    padding: var(--space-1) var(--space-2);
    flex: none;
  }
  /* Degraded repair action: a gold ghost button — clearly a warning
     fix, not a second "turn on". v1.3.1: flex:none so it always keeps
     its full height in the scroll column (never squashed onto the
     window edge, Ali's D48 report). */
  .fix {
    margin-top: var(--space-2);
    flex: none;
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-full);
    border: 1px solid rgba(255, 197, 61, 0.45);
    background: rgba(255, 197, 61, 0.08);
    color: var(--gold-300);
    font-weight: 500;
    transition:
      background 160ms ease-out,
      border-color 160ms ease-out;
  }
  .fix:hover {
    background: rgba(255, 197, 61, 0.16);
    border-color: rgba(255, 197, 61, 0.7);
  }

  @media (prefers-reduced-motion: reduce) {
    .dot.connecting {
      animation: none;
    }
  }
</style>
