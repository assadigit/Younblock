<script lang="ts">
  // OpenYouTubeCard.svelte — the connected state's card slot content
  // (docs/ui-v3/UI-SPEC.md §6): opens the YouTube app through the existing
  // Kotlin shell bridge (window.YounblockShell.openYouTube — the bridge
  // falls back to the browser when the app is not installed); where no
  // bridge exists (the Linux web edition, plain browsers) the card opens
  // the mobile site in a new tab. Appears with a 200ms fade when the state
  // becomes connected; the parent removes it immediately on leaving.

  import { ExternalLink } from "./icons";

  // the in-house play mark (the ghost icon's style — the repo has no
  // separate YouTube bitmap asset): same glyph as the brand mark.
  const launch = (): void => {
    const shell = (window as unknown as { YounblockShell?: { openYouTube?: () => boolean } }).YounblockShell;
    if (shell && typeof shell.openYouTube === "function") {
      shell.openYouTube();
      return;
    }
    window.open("https://m.youtube.com", "_blank", "noopener");
  };
</script>

<button type="button" class="card" on:click={launch} aria-label="Open the YouTube app">
  <span class="mark" aria-hidden="true">
    <svg viewBox="0 0 28 20" focusable="false">
      <rect width="28" height="20" rx="6" />
      <path d="M11 5.5l8 4.5-8 4.5z" />
    </svg>
  </span>
  <span class="text">
    <span class="title">Open YouTube</span>
    <span class="second">Launch the YouTube app</span>
  </span>
  <span class="glyph" aria-hidden="true"><ExternalLink /></span>
</button>

<style>
  /* the card: full width minus margins, --yb-card-h tall, as round as the
     track (a stadium, like the pill it echoes), lifted surface + hairline
     border (spec §6) */
  .card {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    height: var(--yb-card-h);
    padding: 0 16px;
    border-radius: calc(var(--yb-card-h) / 2);
    background: color-mix(in srgb, var(--yb-base-lift) 60%, transparent);
    border: 1px solid color-mix(in srgb, var(--yb-text) 20%, transparent);
    animation: card-in 200ms ease;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    text-align: left;
  }
  .card:focus-visible {
    outline: 3px solid var(--yb-text);
    outline-offset: 2px;
  }
  .card:active {
    filter: brightness(1.08);
  }

  .mark {
    flex: none;
    width: 40px;
    height: 29px;
    display: grid;
    place-items: center;
  }
  .mark svg {
    width: 40px;
    height: 29px;
    fill: var(--yb-red);
  }
  .mark svg path {
    fill: #fff;
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .title {
    font-family: var(--font);
    font-size: 16px;
    font-weight: 600;
    color: var(--yb-text);
  }
  .second {
    font-family: var(--font);
    font-size: 14px;
    color: var(--yb-text-dim);
  }

  /* right: one external-link icon only, no chevron (spec §6) */
  .glyph {
    flex: none;
    color: var(--yb-text-dim);
    display: grid;
    place-items: center;
  }
  .glyph :global(svg) {
    width: 20px;
    height: 20px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  @keyframes card-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
</style>
