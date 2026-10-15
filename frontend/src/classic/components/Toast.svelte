<script lang="ts">
  // Toasts (file 02, section 8): announced, last 3 s.
  interface Toast {
    id: number;
    text: string;
  }
  interface Props {
    toasts: Toast[];
  }
  let { toasts }: Props = $props();
</script>

<div class="toasts" aria-live="polite">
  {#each toasts as toast (toast.id)}
    <div class="toast" role="status">{toast.text}</div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
    inset-inline: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    pointer-events: none;
    z-index: 50;
  }
  .toast {
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--gold-400);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    font-size: var(--text-sm);
    line-height: 20px;
    box-shadow: var(--shadow-pop);
    animation: fade 160ms ease-out backwards;
  }
  @keyframes fade {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  /* Reduced motion (D45): fade only, no slide. */
  @media (prefers-reduced-motion: reduce) {
    .toast {
      animation-name: fade-flat;
    }
  }
  @keyframes fade-flat {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
</style>
