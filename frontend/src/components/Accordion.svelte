<script lang="ts">
  // One-open-at-a-time accordion (file 02, section 4.4).
  // v1.5.0 (D58): the body reveals with the app's shared expand motion
  // and the chevron is the same stroke icon every expander uses, drawn
  // rotating like the Settings one — the old text glyph "›" was the only
  // non-icon arrow left in the app.
  import Icon from "./Icon.svelte";

  interface Props {
    title: string;
    open?: boolean;
    onToggle: () => void;
    children?: import("svelte").Snippet;
  }
  let { title, open = false, onToggle, children }: Props = $props();
</script>

<div class="accordion">
  <button
    type="button"
    class="head"
    aria-expanded={open}
    onclick={onToggle}
  >
    <span class="title">{title}</span>
    <span class="chevron" class:open aria-hidden="true">
      <Icon name="chevron-right" size={18} />
    </span>
  </button>
  <div class="reveal" class:open>
    <div class="reveal-body">
      <div class="body">
        {@render children?.()}
      </div>
    </div>
  </div>
</div>

<style>
  .accordion {
    border-bottom: 1px solid var(--color-border);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-height: 44px;
    padding: var(--space-2) 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    text-align: start;
  }
  .head:hover .title {
    color: var(--color-text);
  }
  .title {
    font-weight: 500;
    transition: color var(--dur-fast) ease-out;
  }
  .chevron {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    flex: none;
    color: var(--color-text-muted);
    transition:
      transform var(--dur-med) var(--ease-spring-out),
      color var(--dur-fast) ease-out;
  }
  .head:hover .chevron {
    color: var(--color-text);
  }
  .chevron.open {
    transform: rotate(90deg);
  }
  .body {
    padding: 0 0 var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }

  @media (prefers-reduced-motion: reduce) {
    .chevron,
    .title {
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
