<script lang="ts">
  // State label, subtitle, notice line, timer and the quiet line
  // (file 02, section 4.1). The timer and quiet line keep their height in
  // every state so nothing jumps.
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
  const subtitle = $derived(subtitleFor(state.phase, state.detail));
  const used = $derived(
    stats ? formatBytes((stats.bytesDown ?? 0) + (stats.bytesUp ?? 0)) : null,
  );
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
  <p class="state-label">{stateLabel(state.phase)}</p>
  <p class="subtitle">{subtitle}</p>
  <p class="notice" class:visible={!!noticeText} aria-live="polite">{noticeText ?? ""}</p>
  <div class="metrics">
    <p class="timer" class:visible={showTimer} aria-hidden={!showTimer}>
      {stats ? formatTimer(stats.uptimeSec) : "00:00:00"}
    </p>
    <p class="quiet" class:visible={showTimer} aria-hidden={!showTimer}>
      {stats ? fill(copy.stats.used, { n: used ?? "0 B" }) : ""}
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
    font-size: 28px;
    line-height: 34px;
    font-weight: 600;
    letter-spacing: -0.01em;
    color: var(--color-text);
  }
  .subtitle {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 20px;
    color: var(--color-text-muted);
    min-height: 20px;
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
    font-variant-numeric: tabular-nums;
    font-feature-settings: "tnum";
    color: var(--color-text);
    min-height: 32px;
    opacity: 0;
    transition: opacity 160ms ease-out;
  }
  .quiet {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 20px;
    color: var(--color-text-muted);
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
  .fix {
    margin-top: var(--space-3);
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-full);
    background: var(--green-500);
    color: var(--color-on-primary);
    font-weight: 500;
  }
  .fix:hover {
    background: var(--green-400);
  }
</style>
