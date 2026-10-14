<script lang="ts">
  // State label, subtitle, notice line, timer and the quiet line
  // (file 02, section 4.1). The timer and quiet line keep their height in
  // every state so nothing jumps. v1.2.0 polish (D45): a state-colored
  // status dot beside the label, tighter typographic rhythm, and an
  // amber ghost action for the degraded state (it reads as a warning
  // repair action, not a second "turn on" button).
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
  const subtitle = $derived(subtitleFor(state.phase, state.detail, state.fail));
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
  const errorCopy = $derived(
    state.phase === "error" && state.error ? errorText ?? "" : null,
  );

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
  <p class="subtitle" class:warn={state.phase === "degraded"}>{subtitle}</p>
  <p class="notice" class:visible={!!noticeText} aria-live="polite">{noticeText ?? ""}</p>
  <div class="metrics">
    <p class="timer" class:visible={showTimer} aria-hidden={!showTimer}>
      {stats ? formatTimer(stats.uptimeSec) : "00:00:00"}
    </p>
    <p class="quiet" class:visible={showTimer} aria-hidden={!showTimer}>
      {quietText}
    </p>
  </div>
  {#if state.phase === "error" && errorCopy}
    <div class="error-row">
      <p class="error-text">{errorCopy}</p>
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
    background: var(--green-400);
    box-shadow: 0 0 0 3px rgba(52, 199, 123, 0.18);
  }
  .dot.connecting {
    background: var(--color-warn);
    box-shadow: 0 0 0 3px rgba(242, 184, 75, 0.18);
    animation: dot-wait 1.6s ease-in-out infinite alternate;
  }
  .dot.degraded {
    background: var(--color-warn);
    box-shadow: 0 0 0 3px rgba(242, 184, 75, 0.18);
  }
  .dot.error {
    background: var(--color-danger);
    box-shadow: 0 0 0 3px rgba(255, 106, 95, 0.18);
  }
  .dot.stopping {
    background: var(--neutral-600);
    box-shadow: 0 0 0 3px rgba(101, 105, 113, 0.14);
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
    color: var(--amber-300);
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
  .metrics {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0;
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
  .timer.visible,
  .quiet.visible {
    opacity: 1;
  }
  .error-row {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    margin-top: var(--space-2);
  }
  .error-text {
    margin: 0;
    color: var(--color-danger);
    font-size: var(--text-sm);
    line-height: 20px;
    max-width: 280px;
  }
  .link {
    color: var(--color-text-muted);
    text-decoration: underline;
    min-height: 44px;
    padding: var(--space-1) var(--space-2);
  }
  /* Degraded repair action: an amber ghost button — clearly a warning
     fix, not a second "turn on". */
  .fix {
    margin-top: var(--space-3);
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-full);
    border: 1px solid rgba(242, 184, 75, 0.45);
    background: rgba(242, 184, 75, 0.08);
    color: var(--amber-300);
    font-weight: 500;
    transition:
      background 160ms ease-out,
      border-color 160ms ease-out;
  }
  .fix:hover {
    background: rgba(242, 184, 75, 0.16);
    border-color: rgba(242, 184, 75, 0.7);
  }

  @media (prefers-reduced-motion: reduce) {
    .dot.connecting {
      animation: none;
    }
  }
</style>
