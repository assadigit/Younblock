<script lang="ts">
  // OpenYouTubeCard.svelte — the connected state's card slot content
  // (the refinement round, UI-SPEC (2).md §3 item 9: glass style, only
  // while connected — unchanged behavior): opens the YouTube app through
  // the existing Kotlin shell bridge (window.YounblockShell.openYouTube —
  // the bridge falls back to the browser when the app is not installed);
  // where no bridge exists (the Linux web edition, plain browsers) the
  // card opens the mobile site in a new tab. Rises in with the
  // reference's 450 ms fade + 14px lift when the state becomes connected;
  // the parent removes it on leaving. Every visual value is ported from
  // the reference .card rules.

  // the launch behavior (unchanged)
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
  <span class="ic" aria-hidden="true"></span>
  <span class="text">
    <span class="title">Open YouTube</span>
    <span class="second">Launch the YouTube app</span>
  </span>
</button>

<style>
  /* the card: glass style — a translucent white gradient with a top
     highlight and a soft drop shadow, radius 22 (the reference values) */
  .card {
    display: flex;
    align-items: center;
    gap: 14px;
    width: 100%;
    height: var(--yb-card-h);
    padding: 12px 14px;
    border: 0;
    border-radius: 22px;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.05));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.22),
      0 8px 18px rgba(0, 0, 0, 0.3);
    animation: card-in 0.45s ease;
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

  /* the red play tile: 48x40, radius 11, gradient + top highlight, the
     white triangle drawn with borders (the reference .card .ic) */
  .ic {
    flex: none;
    width: 48px;
    height: 40px;
    border-radius: 11px;
    background: linear-gradient(180deg, #f0392c, #c3180f);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.35);
    position: relative;
  }
  .ic:after {
    content: "";
    position: absolute;
    left: 19px;
    top: 11px;
    border-left: 13px solid #fff;
    border-top: 9px solid transparent;
    border-bottom: 9px solid transparent;
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
    font-size: 17px;
    font-weight: 600;
    color: var(--yb-text);
  }
  .second {
    font-family: var(--font);
    font-size: 14px;
    color: var(--c-sub);
  }

  /* rises in with the reference's 450 ms fade + 14px lift */
  @keyframes card-in {
    from {
      opacity: 0;
      transform: translateY(14px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
