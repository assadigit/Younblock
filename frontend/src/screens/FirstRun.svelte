<script lang="ts">
  // First run (file 02, section 4.2): shown once when firstRunDone is false.
  // v1.5.0 (D59): the welcome was a plain left-aligned text block — the
  // app's first impression now matches its personality: the wordmark sits
  // above a soft brand glow, the copy is centered inside a readable
  // measure, the autostart choice is a card row (still one checkbox, no
  // new questions), and the whole page staggers in (decoration off under
  // reduced motion, D45).
  import Wordmark from "../components/Wordmark.svelte";
  import { copy } from "../i18n";

  interface Props {
    startWithWindowsSupported: boolean;
    hasTray?: boolean;
    onGetStarted: (autostart: boolean) => void;
  }
  let { startWithWindowsSupported, hasTray = true, onGetStarted }: Props = $props();

  let autostart = $state(true); // checked by default per the spec
</script>

<section class="first-run view" aria-label={copy.firstRun.title}>
  <div class="stack">
    <div class="logo rise-1">
      <Wordmark size={40} />
    </div>
    <h1 class="title rise-2">{copy.firstRun.title}</h1>
    <p class="text rise-3">{hasTray ? copy.firstRun.text : copy.firstRun.textNoTray}</p>
    {#if startWithWindowsSupported}
      <label class="check rise-4">
        <input type="checkbox" bind:checked={autostart} />
        <span class="box" aria-hidden="true">
          <svg viewBox="0 0 12 12" width="12" height="12" focusable="false">
            <path d="M10.5 1.5 4.2 8.6 1.5 5.9" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </span>
        <span>{copy.firstRun.autostart}</span>
      </label>
    {/if}
    <button type="button" class="primary rise-5" onclick={() => onGetStarted(autostart && startWithWindowsSupported)}>
      {copy.buttons.getStarted}
    </button>
  </div>
</section>

<style>
  .first-run {
    align-items: center;
    justify-content: center;
    padding: var(--space-6) var(--space-6) calc(var(--space-8) + env(safe-area-inset-bottom));
  }
  .stack {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--space-4);
    width: 100%;
    max-width: 300px;
  }
  /* The wordmark sits over the same warm glow the home screen's button
     breathes in — the page and the machine share one atmosphere. */
  .logo {
    position: relative;
    display: grid;
    place-items: center;
    padding: var(--space-6) 0 var(--space-2);
  }
  .logo::before {
    content: "";
    position: absolute;
    inset: -30px -40px;
    background: radial-gradient(closest-side, rgba(221, 50, 51, 0.14), transparent 72%);
    pointer-events: none;
  }
  .title {
    margin: 0;
    font-size: var(--text-xl);
    line-height: 34px;
    font-weight: 600;
    letter-spacing: -0.015em;
    color: var(--color-text);
  }
  .text {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 22px;
  }
  /* The autostart choice: a card row with a brand checkbox — 44 px+ of
     touch target, the whole row toggles. */
  .check {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 52px;
    width: 100%;
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 18px;
    text-align: start;
    cursor: pointer;
    transition:
      border-color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out;
  }
  .check:hover {
    border-color: var(--color-border-strong);
    background: var(--color-surface-2);
  }
  .check input {
    position: absolute;
    inset: 0;
    opacity: 0;
    margin: 0;
    cursor: pointer;
  }
  .box {
    flex: none;
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border-radius: 6px;
    border: 2px solid var(--neutral-600);
    background: var(--neutral-900);
    color: var(--color-on-primary);
    transition:
      border-color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out;
  }
  .box svg {
    opacity: 0;
    transform: scale(0.6);
    transition:
      opacity var(--dur-fast) ease-out,
      transform var(--dur-med) var(--ease-spring-out);
  }
  .check:hover .box {
    border-color: var(--neutral-400);
  }
  .check input:checked ~ .box {
    border-color: var(--red-500);
    background: var(--red-500);
  }
  .check input:checked ~ .box svg {
    opacity: 1;
    transform: scale(1);
  }
  .check input:focus-visible ~ .box {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  .primary {
    margin-top: var(--space-2);
    min-height: 48px;
    align-self: stretch;
    border-radius: var(--radius-md);
    background: var(--red-500);
    color: var(--color-on-primary);
    font-weight: 600;
    font-size: var(--text-md);
    box-shadow: 0 8px 22px rgba(221, 50, 51, 0.28);
    transition:
      background var(--dur-fast) ease-out,
      box-shadow var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .primary:hover {
    background: var(--red-400);
    box-shadow: 0 10px 26px rgba(221, 50, 51, 0.34);
  }
  .primary:active {
    transform: scale(0.98);
  }

  /* The stagger: each element rises 60 ms after the one above it. */
  .rise-1,
  .rise-2,
  .rise-3,
  .rise-4,
  .rise-5 {
    animation: rise var(--dur-slow) var(--ease-spring-out) backwards;
  }
  .rise-2 {
    animation-delay: 60ms;
  }
  .rise-3 {
    animation-delay: 120ms;
  }
  .rise-4 {
    animation-delay: 180ms;
  }
  .rise-5 {
    animation-delay: 240ms;
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .rise-1,
    .rise-2,
    .rise-3,
    .rise-4,
    .rise-5 {
      animation: none;
    }
    .check,
    .box,
    .box svg,
    .primary {
      transition: none;
    }
    .primary:active {
      transform: none;
    }
  }
</style>
