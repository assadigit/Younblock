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
    <div class="panel" role="menu" aria-label="Younblock menu">
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
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
  }
  .menu-button:hover {
    color: var(--color-text);
    background: var(--color-surface);
  }
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
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
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
  }
  .panel button:hover {
    background: var(--neutral-800);
  }
  .panel button.quit {
    color: var(--color-text-muted);
  }
  .panel button.quit:hover {
    color: var(--color-text);
  }
  .divider {
    height: 1px;
    background: var(--color-border);
    margin: var(--space-1) var(--space-2);
  }
</style>
