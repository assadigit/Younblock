<script lang="ts">
  // The one bold element (file 02, section 4.1): halo ring 184 px, ring
  // 156 px, disc 132 px, glyph 48 px; hit area 208 px. Readable without
  // color: Off is an outlined glyph on a dark disc; On is a solid green
  // disc with a dark glyph; degraded has a dashed ring; error a solid
  // danger ring. Click/Space/Enter toggle; while connecting a click
  // cancels; while stopping clicks are ignored; a double click cannot
  // start two operations.
  import Icon from "./Icon.svelte";

  interface Props {
    phase: string;
    busy?: boolean;
    onToggle: () => void;
    label: string;
  }
  let { phase, busy = false, onToggle, label }: Props = $props();

  let pressed = $state(false);
  let lastClick = 0;
  let ripple = $state(0);

  // One orchestrated moment: when the phase becomes on, a single ring
  // ripple expands over 600 ms and fades (file 02, section 7).
  let previousPhase = $state(phase);
  $effect(() => {
    if (phase === "on" && previousPhase !== "on") {
      ripple++;
    }
    previousPhase = phase;
  });

  const checked = $derived(phase === "on" || phase === "degraded");
  const inert = $derived(phase === "stopping" || busy);

  function activate() {
    if (inert) return;
    const now = Date.now();
    if (now - lastClick < 250) return; // debounce double clicks
    lastClick = now;
    onToggle();
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      activate();
    }
  }
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  aria-label={label}
  aria-busy={phase === "connecting"}
  class="power"
  class:off={phase === "off"}
  class:on={phase === "on"}
  class:connecting={phase === "connecting"}
  class:degraded={phase === "degraded"}
  class:error={phase === "error"}
  class:stopping={phase === "stopping"}
  class:pressed
  disabled={inert}
  onclick={activate}
  onkeydown={onKeydown}
  onpointerdown={() => (pressed = true)}
  onpointerup={() => (pressed = false)}
  onpointerleave={() => (pressed = false)}
>
  <span class="halo" aria-hidden="true"></span>
  <span class="ring" aria-hidden="true">
    {#if phase === "connecting"}
      <span class="arc"></span>
    {/if}
  </span>
  <span class="disc" aria-hidden="true">
    <span class="glyph"><Icon name="power" size={48} strokeWidth={3.5} /></span>
  </span>
  {#key ripple}
    {#if ripple > 0 && phase === "on"}
      <span class="ripple" aria-hidden="true"></span>
    {/if}
  {/key}
</button>

<style>
  .power {
    position: relative;
    width: 208px;
    height: 208px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-full);
    transition: transform 80ms ease-out;
    -webkit-tap-highlight-color: transparent;
  }
  .power.pressed {
    transform: scale(0.97);
  }
  .power:disabled {
    cursor: default;
  }

  .halo,
  .ring,
  .disc,
  .glyph {
    display: grid;
    place-items: center;
    border-radius: var(--radius-full);
  }

  .halo {
    position: absolute;
    width: 184px;
    height: 184px;
    border: 1px solid var(--neutral-800);
    opacity: 0.3;
    transition: border-color 280ms ease-out;
  }

  .ring {
    position: absolute;
    width: 156px;
    height: 156px;
    border: 2px solid var(--neutral-800);
    transition: border-color 280ms ease-out;
  }

  .disc {
    width: 132px;
    height: 132px;
    background: var(--neutral-850);
    border: 1px solid var(--neutral-800);
    transition:
      background 280ms ease-out,
      border-color 280ms ease-out,
      box-shadow 280ms ease-out;
  }

  .glyph {
    width: 48px;
    height: 48px;
    color: var(--neutral-400);
    transition: color 280ms ease-out;
  }
  .glyph :global(svg) {
    display: block;
  }

  /* Off: outlined glyph on a dark disc. */
  .off .glyph :global(svg path) {
    fill: none;
  }

  /* On: solid green disc with a dark glyph and the static glow. */
  .on .disc {
    background: var(--green-500);
    border-color: var(--green-500);
    box-shadow: var(--glow-on);
  }
  .on .glyph {
    color: var(--color-on-primary);
  }
  .on .halo {
    border-color: var(--green-500);
    opacity: 0.3;
  }
  .on .ring {
    border-color: var(--green-500);
  }

  /* Connecting: amber glyph, rotating 90-degree arc (functional loading). */
  .connecting .glyph {
    color: var(--color-warn);
  }
  .connecting .ring {
    border-color: transparent;
  }
  .arc {
    position: absolute;
    inset: -2px;
    border-radius: var(--radius-full);
    border: 2px solid transparent;
    border-top-color: var(--color-warn);
    border-right-color: var(--color-warn);
    animation: spin 1.2s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* Degraded: dashed amber ring. */
  .degraded .glyph {
    color: var(--color-warn);
  }
  .degraded .ring {
    border-style: dashed;
    border-color: var(--color-warn);
  }
  .degraded .halo {
    border-color: var(--color-warn);
    opacity: 0.3;
  }

  /* Error: solid danger ring. */
  .error .glyph {
    color: var(--color-danger);
  }
  .error .ring {
    border-color: var(--color-danger);
  }
  .error .halo {
    border-color: var(--color-danger);
    opacity: 0.3;
  }

  /* Stopping: neutral, inert. */
  .stopping {
    opacity: 0.7;
  }

  /* The single ripple when the phase becomes on. */
  .ripple {
    position: absolute;
    width: 156px;
    height: 156px;
    border-radius: var(--radius-full);
    border: 2px solid var(--green-500);
    animation: ripple 600ms ease-out forwards;
    pointer-events: none;
  }
  @keyframes ripple {
    from {
      transform: scale(1);
      opacity: 0.8;
    }
    to {
      transform: scale(1.6);
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .arc {
      animation-duration: 1.2s;
      animation-iteration-count: 1;
    }
    .ripple {
      animation: none;
      opacity: 0;
    }
    .power,
    .disc,
    .glyph,
    .ring,
    .halo {
      transition: none;
    }
  }
</style>
