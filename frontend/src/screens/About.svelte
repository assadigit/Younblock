<script lang="ts">
  // About (file 02, section 4.5): wordmark and version, How it works with
  // the three-node diagram, What it doesn't do, Privacy, licenses.
  import Wordmark from "../components/Wordmark.svelte";
  import Icon from "../components/Icon.svelte";
  import { copy } from "../i18n";

  interface Props {
    version: string;
    onBack: () => void;
  }
  let { version, onBack }: Props = $props();

  let licensesOpen = $state(false);
</script>

<section class="about" aria-label={copy.about.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1>{copy.about.title}</h1>
  </header>

  <div class="scroll">
    <div class="head">
      <Wordmark />
      <p class="version">{version}</p>
    </div>

    <div class="block">
      <p class="block-title">{copy.about.howTitle}</p>
      <ol class="steps">
        {#each copy.about.how as step, index (index)}
          <li><span class="num">{index + 1}</span><span>{step}</span></li>
        {/each}
      </ol>
      <div class="diagram" role="img" aria-label="Your browser to Younblock to YouTube">
        <div class="node">
          <Icon name="globe" size={18} />
          <span>{copy.about.diagram.browser}</span>
        </div>
        <div class="link" aria-hidden="true"></div>
        <div class="node app">
          <span class="dot" aria-hidden="true"></span>
          <span>{copy.about.diagram.app}</span>
        </div>
        <div class="link" aria-hidden="true"></div>
        <div class="node">
          <Icon name="chevron-right" size={0} />
          <span class="play" aria-hidden="true">▶</span>
          <span>{copy.about.diagram.youtube}</span>
        </div>
      </div>
    </div>

    <div class="block">
      <p class="block-title">{copy.about.notTitle}</p>
      <ul class="plain">
        {#each copy.about.not as item (item)}
          <li>{item}</li>
        {/each}
      </ul>
    </div>

    <div class="block">
      <p class="block-title">{copy.about.privacyTitle}</p>
      <ul class="plain">
        {#each copy.about.privacy as item (item)}
          <li>{item}</li>
        {/each}
      </ul>
    </div>

    <div class="block">
      <button type="button" class="licenses-head" onclick={() => (licensesOpen = !licensesOpen)} aria-expanded={licensesOpen}>
        <span class="block-title">{copy.about.licensesTitle}</span>
        <Icon name="chevron-right" size={16} />
      </button>
      {#if licensesOpen}
        <ul class="plain">
          <li>{copy.about.licenses.inter}</li>
          <li>{copy.about.licenses.lucide}</li>
          <li>{copy.about.licenses.svelte}</li>
          <li>{copy.about.licenses.wails}</li>
          <li>{copy.about.licenses.systray}</li>
          <li>{copy.about.licenses.gotom}</li>
          <li>{copy.about.licenses.go}</li>
        </ul>
      {/if}
    </div>
  </div>
</section>

<style>
  .about {
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
  .head {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding-top: var(--space-4);
  }
  .version {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
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
  .licenses-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    min-height: 44px;
    text-align: start;
  }
  .steps {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .steps li {
    display: flex;
    gap: var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .num {
    flex: none;
    width: 22px;
    height: 22px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-full);
    background: var(--color-surface);
    color: var(--color-text);
    font-size: var(--text-xs);
    font-weight: 500;
  }
  .diagram {
    margin-top: var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .node {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-2);
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 14px;
    text-align: center;
  }
  .node.app {
    border-color: var(--red-700);
    color: var(--color-text);
  }
  .node.app .dot {
    width: 10px;
    height: 10px;
    border-radius: var(--radius-full);
    border: 2px solid var(--red-500);
  }
  .play {
    color: var(--brand-red);
    font-size: 12px;
    line-height: 18px;
  }
  .link {
    width: 14px;
    height: 2px;
    background: var(--neutral-700);
    flex: none;
  }
  .plain {
    margin: 0;
    padding-inline-start: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
</style>
