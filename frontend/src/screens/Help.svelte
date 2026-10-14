<script lang="ts">
  // Help (file 02, section 4.4): one scroll, the connection test with live
  // progress, the common-problems accordion, and the three rescue buttons.
  // v1.5.0 (D56/D58): the blocks sit on cards with one caption voice, the
  // Run-test button carries a spinner while it works (functional motion,
  // D45), and the accordion shares the app's reveal + chevron language.
  import Accordion from "../components/Accordion.svelte";
  import Icon from "../components/Icon.svelte";
  import TestSteps from "../components/TestSteps.svelte";
  import { copy } from "../i18n";
  import type { TestStep } from "../lib/backend";

  interface Props {
    steps: TestStep[];
    testing: boolean;
    hasTray?: boolean;
    onBack: () => void;
    onRunTest: () => void;
    onFixConnection: () => void;
    onCopyReport: () => void;
    onOpenLogs: () => void;
    onRestore: () => void;
  }
  let {
    steps,
    testing,
    hasTray = true,
    onBack,
    onRunTest,
    onFixConnection,
    onCopyReport,
    onOpenLogs,
    onRestore,
  }: Props = $props();

  let openIndex = $state<number | null>(null);
  // q2 ("Where did the window go?") is tray copy: the Linux web edition
  // has no window or tray, so it is hidden there (D50).
  const problems = $derived.by(() => {
    const all = [
      { q: copy.help.problems.q1, a: copy.help.problems.a1 },
      { q: copy.help.problems.q2, a: copy.help.problems.a2, tray: true },
      { q: copy.help.problems.q3, a: copy.help.problems.a3 },
      { q: copy.help.problems.q4, a: copy.help.problems.a4 },
      { q: copy.help.problems.q5, a: copy.help.problems.a5 },
      { q: copy.help.problems.q6, a: copy.help.problems.a6 },
      { q: copy.help.problems.q7, a: copy.help.problems.a7 },
    ];
    return hasTray ? all : all.filter((p) => !p.tray);
  });

  const failed = $derived(steps.some((s) => !s.ok));
  const failedCritical = $derived(
    steps.some((s) => !s.ok && (s.id === "youtube" || s.id === "video")),
  );
  const nothingWorked = $derived(steps.length === 4 && steps.every((s) => !s.ok));
</script>

<section class="help view" aria-label={copy.help.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1 class="view-title">{copy.help.title}</h1>
  </header>

  <div class="view-scroll">
    <div class="card">
      <p class="overline">{copy.help.testTitle}</p>
      <button type="button" class="primary" onclick={onRunTest} disabled={testing} aria-busy={testing}>
        {#if testing}
          <span class="spinner" aria-hidden="true"></span>
        {/if}
        <span>{copy.buttons.runTest}</span>
      </button>
      {#if steps.length > 0}
        <div class="steps-wrap">
          <TestSteps {steps} running={testing} />
          {#if failedCritical && !nothingWorked}
            <button type="button" class="secondary" onclick={onFixConnection}>
              {copy.buttons.fixConnection}
            </button>
          {/if}
          {#if nothingWorked}
            <p class="note">{copy.help.testNothingWorked}</p>
          {/if}
        </div>
      {/if}
    </div>

    <div class="block">
      <p class="overline">{copy.help.common}</p>
      {#each problems as problem, index (problem.q)}
        <Accordion
          title={problem.q}
          open={openIndex === index}
          onToggle={() => (openIndex = openIndex === index ? null : index)}
        >
          {problem.a}
        </Accordion>
      {/each}
    </div>

    <div class="card">
      <p class="overline">{copy.help.stillStuck}</p>
      <div class="rescue">
        <button type="button" onclick={onCopyReport}>
          <Icon name="copy" size={16} />
          <span>{copy.buttons.copyReport}</span>
        </button>
        <button type="button" onclick={onOpenLogs}>
          <Icon name="folder-open" size={16} />
          <span>{copy.buttons.openLogs}</span>
        </button>
        <button type="button" onclick={onRestore}>
          <Icon name="life-buoy" size={16} />
          <span>{copy.buttons.restoreSettings}</span>
        </button>
      </div>
    </div>
  </div>
</section>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .steps-wrap {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: 0 calc(-1 * var(--space-1));
  }
  .primary {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-md);
    background: var(--red-500);
    color: var(--color-on-primary);
    font-weight: 500;
    transition:
      background var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .primary:hover {
    background: var(--red-400);
  }
  .primary:active {
    transform: scale(0.98);
  }
  .primary:disabled {
    opacity: 0.6;
    cursor: default;
  }
  /* The working spinner (D58): functional progress, so it keeps turning
     even under reduced motion (D45) — same policy as the connecting arc. */
  .spinner {
    width: 16px;
    height: 16px;
    border-radius: var(--radius-full);
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: var(--color-on-primary);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .secondary {
    align-self: flex-start;
    margin: 0 var(--space-1);
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-md);
    border: 1px solid rgba(255, 197, 61, 0.45);
    background: rgba(255, 197, 61, 0.08);
    color: var(--gold-300);
    font-weight: 500;
    transition:
      background var(--dur-fast) ease-out,
      border-color var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .secondary:hover {
    background: rgba(255, 197, 61, 0.16);
    border-color: rgba(255, 197, 61, 0.7);
  }
  .secondary:active {
    transform: scale(0.98);
  }
  .note {
    margin: 0 var(--space-1);
    color: var(--gold-300);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .rescue {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .rescue button {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 44px;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    font-size: var(--text-sm);
    text-align: start;
    transition:
      background var(--dur-fast) ease-out,
      border-color var(--dur-fast) ease-out;
  }
  .rescue button :global(svg) {
    color: var(--color-text-muted);
    transition: color var(--dur-fast) ease-out;
  }
  .rescue button:hover {
    background: var(--color-surface-2);
    border-color: var(--color-border-strong);
  }
  .rescue button:hover :global(svg) {
    color: var(--gold-300);
  }

  @media (prefers-reduced-motion: reduce) {
    .primary,
    .secondary,
    .rescue button,
    .rescue button :global(svg) {
      transition: none;
    }
    .primary:active,
    .secondary:active {
      transform: none;
    }
    /* .spinner intentionally still spins — functional. */
  }
</style>
