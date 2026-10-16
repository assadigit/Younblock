<script lang="ts">
  // MenuSheet.svelte — the sheet behind the menu button: About / How it
  // works / Help. Escape and the red Close button dismiss it. While open
  // it behaves like a modal: focus moves in, Tab wraps inside, and focus
  // returns to the menu button on close.
  import { createEventDispatcher } from "svelte";

  export let open = false;
  /** Host app version from the backend (backend.getAppInfo); empty hides
   *  the line — a dev preview has no host, and a hard-coded copy of the
   *  number here would go stale on the next bump (it already did once). */
  export let version = "";

  const dispatch = createEventDispatcher<{ close: void }>();

  // =====================================================================
  // CONFIG — edit the three texts here
  // =====================================================================
  const ABOUT = [
    "Younblock helps YouTube load in your browser when the network gets in the way. It runs only on your PC, with no account and no analytics.",
    "It is not a VPN: it does not hide your IP address and adds no encryption — YouTube's own HTTPS still protects the content.",
  ];

  const HOW_IT_WORKS = [
    "Your browser sends its request to Younblock, which runs only on your PC.",
    "Younblock looks up YouTube's address with a secure DNS service, so the lookup can't be tampered with on the way.",
    "The first “hello” message to YouTube is sent in small pieces. Filters that scan it for the site's name usually don't recognise the pieces.",
    "After that, everything flows normally, protected by YouTube's own HTTPS.",
  ];

  const HELP = [
    "YouTube still doesn't load — turn Younblock off and on again, then fully close your browser and reopen it.",
    "The window disappeared — Younblock keeps running in the tray, near the clock. Click its icon to reopen it; choose Quit to stop it completely.",
    "My internet stopped after the app closed — the setting restores itself the next time the app starts.",
  ];
  // =====================================================================

  let closeBtn: HTMLElement | undefined;
  let sheetEl: HTMLElement | undefined;
  let restoreFocus: HTMLElement | null = null;

  // A modal sheet takes focus when it opens and hands it back when it
  // closes. The tiny delay lets the visibility transition release the
  // element first (a hidden element cannot be focused). preventScroll
  // matters: the closed sheet still sits below the stage (translateY),
  // and a plain focus() would scroll .app to reveal it, shoving the
  // whole screen up for a moment.
  $: if (open) {
    if (!restoreFocus) {
      restoreFocus = document.activeElement as HTMLElement | null;
    }
    window.setTimeout(() => closeBtn?.focus({ preventScroll: true }), 20);
  } else {
    restoreFocus?.focus?.();
    restoreFocus = null;
  }

  function close() {
    dispatch("close");
  }

  // Escape closes; Tab (and Shift+Tab) wrap inside the sheet — a modal
  // dialog must not leak focus to the dimmed screen behind it. With the
  // Close button as the only control, every Tab lands back on it.
  function onKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    if (e.key !== "Tab" || !sheetEl) return;
    const focusables = Array.from(
      sheetEl.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      ),
    ).filter((el) => el.offsetParent !== null);
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (e.shiftKey ? document.activeElement === first : document.activeElement === last) {
      e.preventDefault();
      (e.shiftKey ? last : first).focus({ preventScroll: true });
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="backdrop" class:open aria-hidden="true" on:click={close}></div>

<div
  class="sheet"
  class:open
  bind:this={sheetEl}
  role="dialog"
  aria-modal="true"
  aria-label="About Younblock"
  aria-hidden={!open}
>
  <span class="grabber" aria-hidden="true"></span>

  <button type="button" class="close" bind:this={closeBtn} on:click={close} aria-label="Close">
    <svg viewBox="0 0 14 14" aria-hidden="true">
      <path d="M2 2l10 10M12 2L2 12" />
    </svg>
  </button>

  <div class="content">
    <section>
      <h2>About</h2>
      {#each ABOUT as line}
        <p>{line}</p>
      {/each}
      {#if version}<p class="version">Younblock {version}</p>{/if}
    </section>

    <section>
      <h2>How it works</h2>
      {#each HOW_IT_WORKS as line}
        <p>{line}</p>
      {/each}
    </section>

    <section>
      <h2>Help</h2>
      {#each HELP as line}
        <p>{line}</p>
      {/each}
    </section>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    opacity: 0;
    pointer-events: none;
    transition: opacity 300ms ease;
  }
  .backdrop.open {
    opacity: 1;
    pointer-events: auto;
  }

  .sheet {
    position: fixed;
    left: 50%;
    bottom: 0;
    width: min(100%, 22.5rem);
    max-height: min(78%, 30.5rem);
    display: flex;
    flex-direction: column;
    margin-left: calc(min(50%, 11.25rem) * -1);
    background: var(--sheet-bg);
    border: 1px solid var(--sheet-border);
    border-bottom: none;
    border-radius: 1.5rem 1.5rem 0 0;
    padding: 0.875rem 1.375rem calc(1.5rem + env(safe-area-inset-bottom, 0px));
    transform: translateY(105%);
    transition: transform var(--dur-slide) var(--ease-out);
    visibility: hidden;
  }
  .sheet.open {
    transform: none;
    visibility: visible;
  }

  .grabber {
    flex: none;
    width: 2.5rem;
    height: 0.25rem;
    margin: 0 auto 1rem;
    border-radius: 0.25rem;
    background: rgba(255, 255, 255, 0.18);
  }

  .close {
    position: absolute;
    top: 1.1rem;
    right: 1.1rem;
    width: 2.25rem;
    height: 2.25rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--red-500);
    box-shadow: 0 0.25rem 0.625rem rgba(0, 0, 0, 0.35);
    transition: transform var(--dur-fast) ease;
  }
  .close:hover {
    transform: scale(1.06);
  }
  .close:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .close svg {
    width: 0.875rem;
    height: 0.875rem;
    fill: none;
    stroke: #fff;
    stroke-width: 2;
    stroke-linecap: round;
  }

  .content {
    overflow-y: auto;
    padding-right: 0.375rem;
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
  }

  section {
    padding: 1.1rem 0.25rem 1.25rem;
  }
  section + section {
    border-top: 1px solid var(--sheet-border);
  }

  h2 {
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--red-400);
    margin-bottom: 0.6rem;
  }

  p {
    font-size: 0.8125rem;
    font-weight: var(--fw-medium);
    line-height: 1.55;
    color: var(--text-secondary);
  }
  p + p {
    margin-top: 0.5rem;
  }

  .version {
    margin-top: 0.75rem;
    color: var(--text-tertiary);
  }
</style>
