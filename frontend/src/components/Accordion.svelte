<script lang="ts">
  // One-open-at-a-time accordion (file 02, section 4.4).
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
    <span class="chevron" class:open aria-hidden="true">›</span>
  </button>
  {#if open}
    <div class="body">
      {@render children?.()}
    </div>
  {/if}
</div>

<style>
  .accordion {
    border-bottom: 1px solid var(--color-border);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    min-height: 44px;
    padding: var(--space-2) 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    text-align: start;
  }
  .title {
    font-weight: 500;
  }
  .chevron {
    color: var(--color-text-muted);
    font-size: 18px;
    transition: transform 160ms ease-out;
    transform: rotate(0deg);
  }
  .chevron.open {
    transform: rotate(90deg);
  }
  .body {
    padding-bottom: var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 20px;
  }
</style>
