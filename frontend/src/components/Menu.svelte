<script lang="ts">
  // The header menu button and its panel (file 02, section 4): Settings,
  // Help, About, a divider, Quit Younblock. Esc closes it.
  import Icon from "./Icon.svelte";
  import { copy } from "../i18n";

  interface Props {
    onNavigate: (view: "settings" | "help" | "about") => void;
    onQuit: () => void;
  }
  let { onNavigate, onQuit }: Props = $props();

  let open = $state(false);
  let rootEl: HTMLElement | null = $state(null);

  function toggle() {
    open = !open;
  }

  function close() {
    open = false;
  }

  function go(view: "settings" | "help" | "about") {
    close();
    onNavigate(view);
  }

  function quit() {
    close();
    onQuit();
  }

  $effect(() => {
    const onKeydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    const onClickAway = (event: MouseEvent) => {
      if (rootEl && !rootEl.contains(event.target as Node)) close();
    };
    window.addEventListener("keydown", onKeydown);
    window.addEventListener("pointerdown", onClickAway);
    return () => {
      window.removeEventListener("keydown", onKeydown);
      window.removeEventListener("pointerdown", onClickAway);
    };
  });
</script>

<div class="menu-root" bind:this={rootEl}>
  <button
    type="button"
    class="menu-button"
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={copy.a11y.menuButton}
    onclick={toggle}
  >
    <Icon name="menu" size={20} />
  </button>
  {#if open}
    <div class="panel" role="menu" aria-label={copy.a11y.menuPanel}>
      <button type="button" role="menuitem" onclick={() => go("settings")}>
        <Icon name="settings" size={18} />
        <span>{copy.menu.settings}</span>
      </button>
      <button type="button" role="menuitem" onclick={() => go("help")}>
        <Icon name="life-buoy" size={18} />
        <span>{copy.menu.help}</span>
      </button>
      <button type="button" role="menuitem" onclick={() => go("about")}>
        <Icon name="info" size={18} />
        <span>{copy.menu.about}</span>
      </button>
      <div class="divider" role="separator"></div>
      <button type="button" role="menuitem" class="quit" onclick={quit}>
        <Icon name="log-out" size={18} />
        <span>{copy.menu.quit}</span>
      </button>
    </div>
  {/if}
</div>

<style>
  .menu-root {
    position: relative;
  }
  .menu-button {
    width: 44px;
    height: 44px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    transition:
      color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .menu-button:hover {
    color: var(--color-text);
    background: var(--color-surface);
  }
  .menu-button:active {
    transform: scale(0.92);
  }
  /* v1.5.0 (D58): the panel unfolds from its own corner — the same
     decelerating curve every entrance uses, so opening it reads as one
     gesture with the press. */
  .panel {
    position: absolute;
    top: calc(100% + 4px);
    inset-inline-end: 0;
    min-width: 200px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-1);
    z-index: 20;
    box-shadow: var(--shadow-pop);
    transform-origin: top right;
    animation: menu-in var(--dur-fast) var(--ease-spring-out) backwards;
  }
  .panel button {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-height: 44px;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: var(--text-sm);
    text-align: start;
    transition: background var(--dur-fast) ease-out;
  }
  .panel button :global(svg) {
    color: var(--color-text-muted);
    transition: color var(--dur-fast) ease-out;
  }
  .panel button:hover {
    background: var(--neutral-800);
  }
  .panel button:hover :global(svg) {
    color: var(--color-text);
  }
  .panel button.quit {
    color: var(--color-text-muted);
  }
  /* Quit is the one destructive-ish item (it stops the app — after
     restoring the internet setting): its hover leans warm-red so the
     choice is legible before it is made. */
  .panel button.quit:hover {
    color: var(--color-text);
    background: rgba(255, 106, 95, 0.1);
  }
  .panel button.quit:hover :global(svg) {
    color: var(--color-danger);
  }
  .divider {
    height: 1px;
    background: var(--color-border);
    margin: var(--space-1) var(--space-2);
  }
  @keyframes menu-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-2px);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .menu-button,
    .panel button {
      transition: none;
    }
    .menu-button:active {
      transform: none;
    }
    .panel {
      animation-name: menu-in-fade;
    }
  }
  @keyframes menu-in-fade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
</style>
