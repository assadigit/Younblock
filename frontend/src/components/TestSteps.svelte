<script lang="ts">
  // The four connection-test steps with a check or cross, a short result
  // and a hint when one fails (file 02, section 4.4).
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
          <Icon name="check" size={18} />
        {:else}
          <Icon name="x" size={18} />
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
  {:else}
    {#if !running}
      <li class="placeholder">{""}</li>
    {/if}
  {/each}
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
    color: var(--green-500);
    background: var(--green-900);
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
    color: var(--amber-300);
  }
  .ms {
    color: var(--neutral-500);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    margin-top: 3px;
  }
  .placeholder {
    height: 4px;
  }
</style>
