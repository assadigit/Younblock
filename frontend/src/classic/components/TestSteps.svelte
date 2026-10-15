<script lang="ts">
  // The four connection-test steps with a check or cross, a short result
  // and a hint when one fails (file 02, section 4.4).
  // v1.5.0 (D58): while the test runs, a quiet "Checking…" row with a
  // spinning arc shows where the test is (functional motion, D45); when
  // a step passes, its check pops in once (decorative — off under
  // reduced motion). The ms readout is styled as a tabular chip.
  import Icon from "./Icon.svelte";
  import { copy } from "../i18n";
  import type { TestStep } from "../lib/backend";

  interface Props {
    steps: TestStep[];
    running: boolean;
  }
  let { steps, running }: Props = $props();

  const hints: Record<string, string> = {
    listener: copy.help.stepHints.listener,
    dns: copy.help.stepHints.dns,
    youtube: copy.help.stepHints.youtube,
    video: copy.help.stepHints.video,
  };
  const names: Record<string, string> = {
    listener: copy.help.steps.listener,
    dns: copy.help.steps.dns,
    youtube: copy.help.steps.youtube,
    video: copy.help.steps.video,
  };
</script>

<ul class="steps" aria-live="polite">
  {#each steps as step (step.id)}
    <li class="step" class:ok={step.ok} class:fail={!step.ok}>
      <span class="mark" aria-hidden="true">
        {#if step.ok}
          <Icon name="check" size={18} strokeWidth={2.25} />
        {:else}
          <Icon name="x" size={18} strokeWidth={2.25} />
        {/if}
      </span>
      <div class="text">
        <p class="name">{names[step.id] ?? step.id}</p>
        {#if step.ok}
          <p class="result">{step.detail}</p>
        {:else}
          <p class="result">{step.detail || hints[step.id]}</p>
          <p class="hint">{hints[step.id]}</p>
        {/if}
      </div>
      <span class="ms" aria-hidden="true">{step.ms} ms</span>
    </li>
  {/each}
  {#if running}
    <li class="step pending">
      <span class="mark" aria-hidden="true">
        <span class="wait"></span>
      </span>
      <div class="text">
        <p class="name muted">{copy.help.testChecking}</p>
      </div>
    </li>
  {/if}
</ul>

<style>
  .steps {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
  .step {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
    min-height: 44px;
  }
  .step:last-child {
    border-bottom: none;
  }
  .mark {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    border-radius: var(--radius-full);
    margin-top: 1px;
  }
  .step.ok .mark {
    /* The logo's lightning on the padlock red: a gold check. */
    color: var(--gold-400);
    background: var(--red-900);
  }
  /* The one-shot result pop (decoration — removed under reduced
     motion, D45/D58). */
  .step.ok .mark :global(svg) {
    animation: mark-pop 240ms var(--ease-spring-out) backwards;
  }
  .step.fail .mark {
    color: var(--color-danger);
    background: var(--red-700);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  .name {
    margin: 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .name.muted {
    color: var(--color-text-muted);
  }
  .result {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
    margin-top: 2px;
  }
  .hint {
    margin: 4px 0 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .step.fail .hint {
    color: var(--gold-300);
  }
  .ms {
    margin-top: 3px;
    padding: 2px 7px;
    border-radius: var(--radius-full);
    background: var(--color-surface-2);
    color: var(--neutral-500);
    font-size: var(--text-2xs);
    line-height: 14px;
    font-variant-numeric: tabular-nums;
    font-weight: 500;
  }
  /* The pending row: a small gold arc, turning while the test works —
     functional state, so it keeps spinning under reduced motion (D45). */
  .wait {
    width: 14px;
    height: 14px;
    border-radius: var(--radius-full);
    border: 2px solid rgba(255, 197, 61, 0.25);
    border-top-color: var(--color-warn);
    animation: wait-spin 0.9s linear infinite;
  }
  @keyframes wait-spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes mark-pop {
    from {
      transform: scale(0.4);
      opacity: 0;
    }
    to {
      transform: scale(1);
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .step.ok .mark :global(svg) {
      animation: none;
    }
    /* .wait intentionally still spins — functional. */
  }
</style>
