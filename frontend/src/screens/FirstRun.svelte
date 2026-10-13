<script lang="ts">
  // First run (file 02, section 4.2): shown once when firstRunDone is false.
  import Wordmark from "../components/Wordmark.svelte";
  import { copy } from "../i18n";
  import type { Settings } from "../lib/backend";

  interface Props {
    startWithWindowsSupported: boolean;
    onGetStarted: (autostart: boolean) => void;
  }
  let { startWithWindowsSupported, onGetStarted }: Props = $props();

  let autostart = $state(true); // checked by default per the spec
</script>

<section class="first-run" aria-label={copy.firstRun.title}>
  <div class="logo"><Wordmark /></div>
  <h1 class="title">{copy.firstRun.title}</h1>
  <p class="text">{copy.firstRun.text}</p>
  {#if startWithWindowsSupported}
    <label class="check">
      <input type="checkbox" bind:checked={autostart} />
      <span>{copy.firstRun.autostart}</span>
    </label>
  {/if}
  <button type="button" class="primary" onclick={() => onGetStarted(autostart && startWithWindowsSupported)}>
    {copy.buttons.getStarted}
  </button>
</section>

<style>
  .first-run {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: center;
    gap: var(--space-4);
    padding: var(--space-8) var(--space-8);
    max-width: 480px;
    margin-inline: auto;
  }
  .logo {
    margin-bottom: var(--space-4);
  }
  .title {
    margin: 0;
    font-size: var(--text-lg);
    line-height: 28px;
    font-weight: 600;
    color: var(--color-text);
  }
  .text {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 44px;
    margin-top: var(--space-2);
    color: var(--color-text);
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .check input {
    width: 18px;
    height: 18px;
    accent-color: var(--green-500);
  }
  .primary {
    margin-top: var(--space-4);
    min-height: 44px;
    align-self: stretch;
    border-radius: var(--radius-md);
    background: var(--green-500);
    color: var(--color-on-primary);
    font-weight: 500;
    font-size: var(--text-md);
  }
  .primary:hover {
    background: var(--green-400);
  }
</style>
