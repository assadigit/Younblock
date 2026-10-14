<script lang="ts">
  // About (file 02, section 4.5): wordmark and version, How it works with
  // the three-node diagram, What it doesn't do, Privacy, licenses.
  // v1.5.0 (D56/D59): the sections sit on cards; the version reads as a
  // pill; the diagram is drawn in the brand language (the app node is
  // the power glyph on the padlock red, YouTube's node the white play
  // badge) instead of a text "▶" and a size-zero icon hack; the license
  // list reveals with the app's expand motion.
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

<section class="about view" aria-label={copy.about.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1 class="view-title">{copy.about.title}</h1>
  </header>

  <div class="view-scroll">
    <div class="head">
      <Wordmark size={34} />
      <p class="version">{copy.about.versionLabel} {version}</p>
    </div>

    <div class="card">
      <p class="overline">{copy.about.howTitle}</p>
      <div class="diagram" role="img" aria-label={copy.about.diagram.aria}>
        <div class="node">
          <Icon name="globe" size={18} />
          <span>{copy.about.diagram.browser}</span>
        </div>
        <div class="link" aria-hidden="true"></div>
        <div class="node app">
          <Icon name="power" size={16} strokeWidth={2.5} />
          <span>{copy.about.diagram.app}</span>
        </div>
        <div class="link" aria-hidden="true"></div>
        <div class="node">
          <span class="play-badge" aria-hidden="true">
            <svg viewBox="0 0 12 14" width="12" height="14" focusable="false">
              <path d="M1 1.2v11.6c0 .8.9 1.3 1.6.9l9-5.8c.6-.4.6-1.4 0-1.8l-9-5.8C1.9-.1 1 .4 1 1.2z" fill="currentColor" />
            </svg>
          </span>
          <span>{copy.about.diagram.youtube}</span>
        </div>
      </div>
      <ol class="steps">
        {#each copy.about.how as step, index (index)}
          <li><span class="num" aria-hidden="true">{index + 1}</span><span>{step}</span></li>
        {/each}
      </ol>
    </div>

    <div class="card">
      <p class="overline">{copy.about.notTitle}</p>
      <ul class="plain">
        {#each copy.about.not as item (item)}
          <li>{item}</li>
        {/each}
      </ul>
    </div>

    <div class="card">
      <p class="overline">{copy.about.privacyTitle}</p>
      <ul class="plain">
        {#each copy.about.privacy as item (item)}
          <li>{item}</li>
        {/each}
      </ul>
    </div>

    <div class="card">
      <button type="button" class="licenses-head" onclick={() => (licensesOpen = !licensesOpen)} aria-expanded={licensesOpen}>
        <span class="overline">{copy.about.licensesTitle}</span>
        <span class="chevron" class:open={licensesOpen} aria-hidden="true">
          <Icon name="chevron-right" size={16} />
        </span>
      </button>
      <div class="reveal" class:open={licensesOpen}>
        <div class="reveal-body">
          <ul class="plain licenses">
            <li>{copy.about.licenses.inter}</li>
            <li>{copy.about.licenses.lucide}</li>
            <li>{copy.about.licenses.svelte}</li>
            <li>{copy.about.licenses.wails}</li>
            <li>{copy.about.licenses.systray}</li>
            <li>{copy.about.licenses.gotom}</li>
            <li>{copy.about.licenses.go}</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</section>

<style>
  .head {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4) 0 var(--space-1);
  }
  .version {
    margin: 0;
    padding: 3px 10px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-full);
    color: var(--color-text-muted);
    font-size: var(--text-2xs);
    line-height: 16px;
    letter-spacing: 0.05em;
    font-variant-numeric: tabular-nums;
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
    background: var(--red-900);
    border: 1px solid var(--red-800);
    color: var(--red-200);
    font-size: var(--text-xs);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }
  /* The three-node diagram, drawn in the logo's language: your browser
     (a quiet globe), Younblock (the power glyph on the padlock red —
     the same mark as the big button), YouTube (the white play badge on
     the same red). It leads the card: the picture summarizes, the
     numbered steps below elaborate (v1.5.0 — it used to sit under the
     steps, below the fold where nobody met it). */
  .diagram {
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
    font-size: var(--text-2xs);
    line-height: 14px;
    text-align: center;
  }
  .node :global(svg) {
    height: 18px;
  }
  .node.app {
    border-color: var(--red-700);
    background: rgba(221, 50, 51, 0.08);
    color: var(--color-text);
  }
  .node.app :global(svg) {
    color: var(--red-400);
  }
  .play-badge {
    display: grid;
    place-items: center;
    width: 22px;
    height: 18px;
  }
  .play-badge svg {
    color: var(--red-500);
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.35));
  }
  .link {
    width: 12px;
    height: 2px;
    border-radius: 1px;
    background: var(--neutral-700);
    flex: none;
    margin-top: -14px;
  }
  .plain {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .plain li {
    padding-left: var(--space-3);
    position: relative;
  }
  .plain li::before {
    content: "–";
    position: absolute;
    left: 0;
    color: var(--neutral-600);
  }
  .licenses-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    width: 100%;
    min-height: 44px;
    text-align: start;
    border-radius: var(--radius-sm);
  }
  .chevron {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    color: var(--color-text-muted);
    transition: transform var(--dur-med) var(--ease-spring-out);
  }
  .chevron.open {
    transform: rotate(90deg);
  }
  .licenses {
    padding-top: var(--space-2);
  }

  @media (prefers-reduced-motion: reduce) {
    .chevron {
      transition: none;
    }
    .reveal {
      transition: none;
    }
    .reveal:not(.open) .reveal-body {
      transition: none;
    }
  }
</style>
