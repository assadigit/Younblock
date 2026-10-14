<script lang="ts">
  // Help (file 02, section 4.4): one scroll, the connection test with live
  // progress, the common-problems accordion, and the three rescue buttons.
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

<section class="help" aria-label={copy.help.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1>{copy.help.title}</h1>
  </header>

  <div class="scroll">
    <div class="block">
      <p class="block-title">{copy.help.testTitle}</p>
      <button type="button" class="primary" onclick={onRunTest} disabled={testing}>
        {copy.buttons.runTest}
      </button>
      {#if steps.length > 0}
        <TestSteps {steps} running={testing} />
        {#if failedCritical && !nothingWorked}
          <button type="button" class="secondary" onclick={onFixConnection}>
            {copy.buttons.fixConnection}
          </button>
        {/if}
        {#if nothingWorked}
          <p class="note">{copy.help.testNothingWorked}</p>
        {/if}
      {/if}
    </div>

    <div class="block">
      <p class="block-title">{copy.help.common}</p>
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

    <div class="block">
      <p class="block-title">{copy.help.stillStuck}</p>
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
  .help {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .view-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    padding-top: max(var(--space-2), env(safe-area-inset-top));
  }
  .view-header h1 {
    margin: 0;
    font-size: var(--text-md);
    line-height: 24px;
    font-weight: 500;
  }
  .back {
    width: 44px;
    height: 44px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    margin-inline-start: calc(-1 * var(--space-2));
  }
  .back:hover {
    color: var(--color-text);
    background: var(--color-surface);
  }
  .scroll {
    flex: 1;
    overflow-y: auto;
    max-height: 100%;
    padding: 0 var(--space-4);
    padding-bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .block-title {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .primary {
    align-self: flex-start;
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-full);
    background: var(--red-500);
    color: var(--color-on-primary);
    font-weight: 500;
  }
  .primary:hover {
    background: var(--red-400);
  }
  .primary:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .secondary {
    align-self: flex-start;
    margin-top: var(--space-2);
    min-height: 44px;
    padding: var(--space-2) var(--space-6);
    border-radius: var(--radius-full);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    font-weight: 500;
  }
  .secondary:hover {
    background: var(--color-surface);
  }
  .note {
    margin: var(--space-2) 0 0;
    color: var(--color-text-muted);
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
  }
  .rescue button:hover {
    background: var(--color-surface);
  }
</style>
