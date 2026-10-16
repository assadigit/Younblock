<script lang="ts">
  // DotRings.svelte — the only decoration (design.md): a single red dotted
  // ring around the track when not connected; five green dotted arcs around
  // the thumb when connected. Both sets are always mounted and cross-fade
  // on the tone flip (App.svelte recenters the family: cy is the track's
  // center when off, the resting thumb when on). Each ring pulses on its
  // own phase, staggered so the connected arcs ripple from the inner one
  // outward; everything runs faster while busy and stops under
  // prefers-reduced-motion (app.css).
  export let tone: "on" | "off" = "off";
  export let busy = false;
  /** Vertical center of the ring family, in rem. */
  export let cy = 28.3;

  const CENTER_X = 11.25; // rem — the middle of the 22.5rem stage
  const START_DEG = -40; // the arcs stay open at the top
  const SWEEP_DEG = 260;
  /** Phase offset between neighbouring rings, in seconds. */
  const RIPPLE = 0.45;

  type Ring = { r: number; dot: number; opacity: number };

  // Radii measured from the reference render, rescaled with the smaller
  // control in the v1.6.3 proportion pass (outer arc −17%).
  const ON_ARCS: Ring[] = [
    { r: 4.375, dot: 0.15, opacity: 0.95 }, // 70px
    { r: 5.25, dot: 0.14, opacity: 0.85 }, // 84px
    { r: 6.125, dot: 0.13, opacity: 0.72 }, // 98px
    { r: 7.0, dot: 0.12, opacity: 0.55 }, // 112px
    { r: 7.875, dot: 0.11, opacity: 0.4 }, // 126px
  ];
  const OFF_RING: Ring[] = [{ r: 6.625, dot: 0.14, opacity: 0.85 }]; // 106px

  function ringDots(r: number): { x: number; y: number }[] {
    const arc = (SWEEP_DEG / 360) * 2 * Math.PI * r;
    const count = Math.max(8, Math.round(arc / 0.45));
    const dots: { x: number; y: number }[] = [];
    for (let i = 0; i < count; i++) {
      const a = ((START_DEG + (SWEEP_DEG * i) / (count - 1)) * Math.PI) / 180;
      dots.push({ x: CENTER_X + r * Math.cos(a), y: r * Math.sin(a) });
    }
    return dots;
  }

  // Negative delays around the middle ring mean the ripple is already
  // running the moment the family appears (a positive delay on the outer
  // rings would leave them frozen for their first delay seconds).
  function rippleDelay(index: number, total: number): number {
    return (index - (total - 1) / 2) * RIPPLE;
  }
</script>

<svg class="rings" class:busy data-tone={tone} viewBox="0 0 22.5 39.25" aria-hidden="true" focusable="false">
  <g class="family" style:--cy={`${cy}`}>
    {#each OFF_RING as ring, ri}
      <g class="ring off" class:active={tone === "off"}>
        <g class="pulse" style:animation-delay={`${rippleDelay(ri, OFF_RING.length)}s`}>
          {#each ringDots(ring.r) as dot}
            <circle cx={dot.x} cy={dot.y} r={ring.dot} fill-opacity={ring.opacity} />
          {/each}
        </g>
      </g>
    {/each}
    {#each ON_ARCS as ring, ri}
      <g class="ring on" class:active={tone === "on"}>
        <g class="pulse" style:animation-delay={`${rippleDelay(ri, ON_ARCS.length)}s`}>
          {#each ringDots(ring.r) as dot}
            <circle cx={dot.x} cy={dot.y} r={ring.dot} fill-opacity={ring.opacity} />
          {/each}
        </g>
      </g>
    {/each}
  </g>
</svg>

<style>
  .rings {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }

  .family {
    /* Inside an SVG, CSS px = user units, and one user unit is one rem in
       this viewBox (360px / 22.5 units = 16px), so --cy is unitless. */
    transform: translateY(calc(var(--cy) * 1px));
    transition: transform var(--dur-tone) ease;
  }

  /* Each ring is two groups: the outer one owns the cross-fade opacity
     (its own element, so it composes with — instead of being overridden
     by — the pulse animation on the inner one). The fill lives on the
     set, not a shared token, so the outgoing color stays correct while
     it fades. */
  .ring {
    opacity: 0;
    transition: opacity var(--dur-swap) ease;
  }
  .ring.active {
    opacity: 1;
  }
  .ring.off {
    fill: var(--red-400);
  }
  .ring.on {
    fill: var(--green-led);
  }

  .pulse {
    animation: pulse var(--dur-pulse) ease-in-out infinite;
  }
  .rings.busy .pulse {
    animation-duration: var(--dur-pulse-busy);
  }
  /* no reason to animate a set nobody can see */
  .ring:not(.active) .pulse {
    animation-play-state: paused;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.62;
    }
  }
</style>
