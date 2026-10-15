<script lang="ts">
  // The one bold element (file 02, section 4.1): halo ring 184 px, ring
  // 156 px, disc 132 px, glyph 48 px; hit area 208 px. Readable without
  // color: Off is an outlined glyph on a dark disc; On is a solid red
  // disc (the logo's padlock red) with a white glyph and gold echo rings
  // (the logo's lightning); degraded glows gold at the rim; error glows
  // danger. Click/Space/Enter toggle; while connecting a click
  // cancels; while stopping clicks are ignored; a double click cannot
  // start two operations.
  //
  // v1.2.0 (Ali's request): the turn-on moment is fully orchestrated —
  // a spring pop of the disc, two staggered echo ripples, a crossfaded
  // gradient disc and a slow glow breath — and the connecting spinner
  // keeps spinning even when Windows asks for reduced motion, because
  // it is functional state, not decoration (DECISIONS.md D45).
  // v1.3.0: recolored to the logo palette (D47).
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

  // One orchestrated moment: when the phase becomes on, two staggered
  // echo rings expand and fade (file 02, section 7, refined in D45).
  // (The initial capture is the point: it tracks the PREVIOUS phase.)
  // svelte-ignore state_referenced_locally
  let previousPhase = $state(phase);
  $effect(() => {
    if (phase === "on" && previousPhase !== "on") {
      ripple++;
    }
    previousPhase = phase;
  });

  const checked = $derived(phase === "on" || phase === "degraded");
  const inert = $derived(phase === "stopping" || busy);

  function release() {
    pressed = false;
  }

  function activate() {
    if (inert) return;
    const now = Date.now();
    if (now - lastClick < 250) return; // debounce double clicks
    lastClick = now;
    release(); // never leave the press stuck if pointerup is swallowed
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
  onpointerup={release}
  onpointerleave={release}
  onpointercancel={release}
  onblur={release}
>
  <span class="halo" aria-hidden="true"></span>
  <span class="ring" aria-hidden="true">
    {#if phase === "connecting"}
      <span class="arc"></span>
      <span class="arc arc-late"></span>
    {/if}
  </span>
  <span class="disc" aria-hidden="true">
    <span class="glyph"><Icon name="power" size={48} strokeWidth={4} /></span>
  </span>
  {#key ripple}
    {#if ripple > 0 && phase === "on"}
      <span class="echo echo-1" aria-hidden="true"></span>
      <span class="echo echo-2" aria-hidden="true"></span>
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
    transition: transform 200ms var(--ease-spring-out, ease-out);
    -webkit-tap-highlight-color: transparent;
  }
  .power.pressed {
    transform: scale(0.96);
    transition-duration: 110ms;
    transition-timing-function: cubic-bezier(0.2, 0, 0, 1);
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
    opacity: 0.35;
    transition:
      border-color 320ms ease-out,
      opacity 320ms ease-out,
      box-shadow 320ms ease-out;
  }

  .ring {
    position: absolute;
    width: 156px;
    height: 156px;
    border: 2px solid var(--neutral-800);
    transition: border-color 320ms ease-out;
  }

  /* The disc material: a machined dark plate. The red "lit" plate is a
     stacked layer that crossfades in over the dark one — gradients can't
     tween, opacity can. */
  .disc {
    position: relative;
    width: 132px;
    height: 132px;
    border: 1px solid var(--neutral-800);
    background:
      radial-gradient(circle at 50% 34%, #34302e 0%, var(--neutral-850) 58%, #262220 100%);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.05),
      inset 0 -10px 18px rgba(0, 0, 0, 0.26),
      0 1px 2px rgba(0, 0, 0, 0.35);
    transition:
      border-color 320ms ease-out,
      box-shadow 320ms ease-out;
  }
  .disc::after {
    content: "";
    position: absolute;
    inset: -1px;
    border-radius: inherit;
    border: 1px solid transparent;
    background:
      radial-gradient(circle at 50% 32%, #f05a5b 0%, var(--red-500) 52%, var(--red-600) 100%);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.28),
      inset 0 -10px 20px rgba(0, 0, 0, 0.22);
    opacity: 0;
    transition: opacity 320ms ease-out;
    pointer-events: none;
  }

  .glyph {
    position: relative;
    width: 48px;
    height: 48px;
    color: var(--neutral-400);
    transition: color 320ms ease-out;
    z-index: 1;
  }
  .glyph :global(svg) {
    display: block;
  }

  /* Off: outlined glyph on a dark plate. */
  .off .glyph :global(svg path) {
    fill: none;
  }

  /* On: the lit plate, layered glow, a gentle breathing aura, and the
     one-shot spring pop. */
  .on .disc::after {
    opacity: 1;
  }
  .on .disc {
    border-color: var(--red-600);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.06),
      inset 0 -10px 18px rgba(0, 0, 0, 0.26),
      0 0 0 1px rgba(221, 50, 51, 0.25),
      0 0 0 10px rgba(221, 50, 51, 0.1),
      0 0 44px rgba(221, 50, 51, 0.32);
    animation:
      disc-pop 550ms cubic-bezier(0.34, 1.56, 0.64, 1),
      glow-breathe 4.6s ease-in-out infinite alternate;
  }
  .on .glyph {
    color: var(--color-on-primary);
  }
  .on .halo {
    border-color: rgba(221, 50, 51, 0.4);
    opacity: 0.5;
    box-shadow: 0 0 30px rgba(221, 50, 51, 0.1);
  }
  .on .ring {
    border-color: rgba(221, 50, 51, 0.55);
  }
  @keyframes disc-pop {
    0% {
      transform: scale(1);
    }
    45% {
      transform: scale(1.055);
    }
    100% {
      transform: scale(1);
    }
  }
  @keyframes glow-breathe {
    from {
      box-shadow:
        inset 0 1px 0 rgba(255, 255, 255, 0.06),
        inset 0 -10px 18px rgba(0, 0, 0, 0.26),
        0 0 0 1px rgba(221, 50, 51, 0.25),
        0 0 0 10px rgba(221, 50, 51, 0.1),
        0 0 44px rgba(221, 50, 51, 0.32);
    }
    to {
      box-shadow:
        inset 0 1px 0 rgba(255, 255, 255, 0.06),
        inset 0 -10px 18px rgba(0, 0, 0, 0.26),
        0 0 0 1px rgba(221, 50, 51, 0.3),
        0 0 0 12px rgba(221, 50, 51, 0.13),
        0 0 58px rgba(221, 50, 51, 0.42);
    }
  }

  /* Connecting: gold glyph, two counter-phased arcs (functional
     loading state — kept even under reduced motion). */
  .connecting .glyph {
    color: var(--color-warn);
    animation: glyph-wait 1.6s ease-in-out infinite alternate;
  }
  .connecting .ring {
    border-color: transparent;
  }
  .arc,
  .arc-late {
    position: absolute;
    inset: -2px;
    border-radius: var(--radius-full);
    border: 3px solid transparent;
    border-top-color: var(--color-warn);
    border-right-color: var(--color-warn);
    animation: spin 1.1s linear infinite;
  }
  .arc-late {
    inset: -2px;
    border-width: 3px;
    border-top-color: transparent;
    border-right-color: transparent;
    border-bottom-color: rgba(255, 197, 61, 0.32);
    border-left-color: rgba(255, 197, 61, 0.32);
    animation-duration: 1.7s;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes glyph-wait {
    from {
      opacity: 0.72;
    }
    to {
      opacity: 1;
    }
  }

  /* Hover: the resting plate wakes up slightly — a hairline brighter
     rim and glyph, so the button feels alive before it is pressed.
     v1.5.0: scoped away from the on-state — the old selector outranked
     ".on .disc" and turned the lit red plate's border gray under the
     pointer (D57). */
  .power:enabled:not(.connecting):not(.on):hover .disc {
    border-color: var(--neutral-700);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.07),
      inset 0 -10px 18px rgba(0, 0, 0, 0.26),
      0 0 0 1px rgba(255, 255, 255, 0.02),
      0 2px 6px rgba(0, 0, 0, 0.4);
  }
  .power:enabled:not(.connecting):not(.on):hover .glyph {
    color: var(--neutral-300);
  }
  .power:enabled:not(.connecting):not(.on):hover .ring {
    border-color: var(--neutral-700);
  }

  /* Degraded: the plate stays dark, the rim glows gold — alive but
     needing attention (D45: solid ring + glow instead of the old dashed
     border, which read as a wireframe placeholder). */
  .degraded .glyph {
    color: var(--color-warn);
  }
  .degraded .ring {
    border-color: var(--color-warn);
  }
  .degraded .halo {
    border-color: rgba(255, 197, 61, 0.45);
    opacity: 0.5;
    box-shadow: 0 0 30px rgba(255, 197, 61, 0.12);
  }

  /* Error: the danger rim — harsher than degraded, no soft aura. */
  .error .glyph {
    color: var(--color-danger);
  }
  .error .ring {
    border-color: var(--color-danger);
  }
  .error .halo {
    border-color: rgba(255, 106, 95, 0.45);
    opacity: 0.5;
  }

  /* Stopping: neutral, inert. */
  .stopping {
    opacity: 0.7;
  }

  /* The turn-on echo: two staggered gold rings expanding from the ring
     and thinning out — the lightning flash of the logo, and the
     "fully animated" moment Ali asked for. */
  .echo {
    position: absolute;
    width: 156px;
    height: 156px;
    border-radius: var(--radius-full);
    border: 2px solid var(--gold-400);
    pointer-events: none;
    animation: echo 640ms cubic-bezier(0.16, 0.6, 0.36, 1) forwards;
  }
  .echo-2 {
    border-color: rgba(255, 197, 61, 0.75);
    animation-delay: 110ms;
    animation-duration: 760ms;
  }
  @keyframes echo {
    from {
      transform: scale(1);
      opacity: 0.85;
    }
    70% {
      opacity: 0.28;
    }
    to {
      transform: scale(1.42);
      opacity: 0;
      border-width: 1px;
    }
  }

  /* Reduced motion: decoration is removed, but the connecting spinner
     keeps turning (it reports state) and color crossfades stay (they
     carry information and cause no vestibular motion). */
  @media (prefers-reduced-motion: reduce) {
    .power {
      transition: none;
    }
    .power.pressed {
      transform: none;
    }
    .on .disc {
      animation: none;
    }
    .echo {
      animation: none;
      opacity: 0;
    }
    .connecting .glyph {
      animation: none;
    }
    /* .arc intentionally still spins — functional. */
  }
</style>
