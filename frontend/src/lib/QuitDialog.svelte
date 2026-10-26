<!-- QuitDialog.svelte — the "Quit Younblock?" confirmation (D112, the
     quit round). The menu sheet's quiet Quit row dispatches "quit" and
     App.svelte opens THIS dialog over everything (the same modal tier
     the diagnostics view rides, 40 — the sheet is 30, its backdrop 20).

     One deliberate, confirmed action — deliberately separate from every
     disconnect semantics (the lever's tap and swipe, the engine's own
     stop path, the notification's ordinary life — all untouched): Quit
     ends the WHOLE app. The copy is the owner's exact prescription:

       Title:  "Quit Younblock?"
       Body:   "This stops the connection and removes the notification.
                You can open the app again at any time."
       Buttons: "Cancel" and "Quit"

     Cancel is the safe default: it takes the focus on open, Escape and
     a backdrop tap cancel too, and Tab wraps inside the two buttons
     (the sheet's own modal discipline, ported small). Quit hands the
     sequence to the Android shell bridge (lib/clipboard.ts — quitApp);
     where no quit-capable shell exists the dialog stays honest: the app
     keeps running and the line says so.

     es2017 floor (the old-WebView boot round): plain checks only, no
     runtime-API surface beyond Array — the floor holds here too. -->
<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { quitApp } from "./clipboard";

  const dispatch = createEventDispatcher<{ cancel: void; quit: void }>();

  // =====================================================================
  // CONFIG — the owner's exact copy (edit only here)
  // =====================================================================
  const TITLE = "Quit Younblock?";
  const BODY = "This stops the connection and removes the notification. You can open the app again at any time.";
  const CANCEL = "Cancel";
  const QUIT = "Quit";
  // =====================================================================

  let card: HTMLElement | undefined;
  let cancelBtn: HTMLElement | undefined;

  // The honest fallback: a host with no quit-capable shell (an older
  // APK, a preview build) keeps the app alive — the user must know the
  // quit did not happen, never a silent no-op behind a closed dialog.
  let unavailable = "";

  // Focus the safe choice when the dialog opens; give the visibility
  // transition a beat first (a hidden element cannot be focused).
  $: if (card) {
    window.setTimeout(() => cancelBtn?.focus({ preventScroll: true }), 20);
  }

  function cancel() {
    dispatch("cancel");
  }

  function onQuit() {
    if (quitApp()) {
      // The shell owns the end-of-app sequence from here: the page dies
      // with the app moments later — nothing left to dispatch locally.
      dispatch("quit");
    } else {
      unavailable = "Quit is not available here.";
    }
  }

  // Escape cancels; Tab (and Shift+Tab) wrap between the two buttons —
  // a modal dialog must not leak focus to the dimmed screen behind it.
  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      cancel();
      return;
    }
    if (e.key !== "Tab" || !card) return;
    const focusables = Array.from(
      card.querySelectorAll<HTMLElement>(
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

<!-- the backdrop: a tap cancels (the safe choice), like the sheet's -->
<div class="quit-backdrop" aria-hidden="true" on:click={cancel}></div>

<div
  class="quit-card"
  bind:this={card}
  role="alertdialog"
  aria-modal="true"
  aria-labelledby="quit-title"
  aria-describedby="quit-body"
>
  <h2 id="quit-title">{TITLE}</h2>
  <p id="quit-body">{BODY}</p>
  {#if unavailable}
    <p class="quit-unavailable" role="alert">{unavailable}</p>
  {/if}
  <div class="quit-actions" role="group" aria-label={TITLE}>
    <button type="button" class="quit-cancel" bind:this={cancelBtn} on:click={cancel}>
      {CANCEL}
    </button>
    <button type="button" class="quit-confirm" on:click={onQuit}>
      {QUIT}
    </button>
  </div>
</div>

<style>
  /* The quit dialog rides the diagnostics tier (40): above the sheet
     (30) and its backdrop (20) — the modal stack the layering round
     pinned. A dimmer backdrop than the sheet's own: the decision under
     it must read as paused, not participable. */
  .quit-backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
    background: rgba(0, 0, 0, 0.62);
    animation: quit-fade 200ms ease;
  }

  /* the card: centered, the sheet's material, a quiet top accent */
  .quit-card {
    position: fixed;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    z-index: 41;
    width: min(100%, 20.5rem);
    padding: 1.375rem 1.375rem 1.125rem;
    border-radius: 1.25rem;
    border: 1px solid var(--sheet-border);
    background: var(--sheet-bg);
    box-shadow: 0 18px 48px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    animation: quit-in 200ms var(--ease-out, ease-out);
  }

  .quit-card h2 {
    margin: 0;
    font-size: 1.0625rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.01em;
    color: var(--text-primary);
  }

  .quit-card p {
    margin: 0.625rem 0 0;
    font-size: 0.8438rem;
    font-weight: var(--fw-medium);
    line-height: 1.55;
    color: var(--text-secondary);
  }

  .quit-unavailable {
    color: var(--red-400) !important;
  }

  /* the buttons: the sheet's .action proportions; Quit carries the
     sheet's danger treatment (the Delete-history row's exact accent) —
     red text on a red-tinted border, never a filled primary shape:
     this is a destructive end-of-app action, deliberately the opposite
     of the connect lever's whole presence */
  .quit-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.625rem;
    margin-top: 1.25rem;
  }
  .quit-cancel,
  .quit-confirm {
    height: 2.75rem;
    min-width: 5.5rem;
    padding: 0 1.25rem;
    border-radius: 0.75rem;
    border: 1px solid var(--sheet-border);
    background: rgba(255, 255, 255, 0.12);
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-weight: var(--fw-semibold);
    font-family: inherit;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 150ms ease;
  }
  .quit-cancel:hover,
  .quit-confirm:hover {
    background: rgba(255, 255, 255, 0.2);
  }
  .quit-cancel:focus-visible,
  .quit-confirm:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.125rem;
  }
  .quit-confirm {
    color: var(--red-400);
    border-color: rgba(224, 107, 107, 0.4);
  }
  .quit-confirm:hover {
    background: rgba(224, 107, 107, 0.12);
  }

  @keyframes quit-fade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  @keyframes quit-in {
    from {
      opacity: 0;
      transform: translate(-50%, -50%) scale(0.96);
    }
    to {
      opacity: 1;
      transform: translate(-50%, -50%) scale(1);
    }
  }

  /* a motion-sensitive host gets the same dialog without the moves */
  @media (prefers-reduced-motion: reduce) {
    .quit-backdrop,
    .quit-card {
      animation: none;
    }
  }
</style>
