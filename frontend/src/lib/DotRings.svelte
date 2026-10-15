<script lang="ts">
  // DotRings.svelte — the only decoration (design.md): a single red dotted
  // ring around the track when not connected; five green dotted arcs around
  // the thumb when connected. App.svelte switches the --ring-dot token and
  // recenters (cy): the track's center when off, the resting thumb when on.
  // They pulse slowly, faster while busy, and never move under
  // prefers-reduced-motion.
  export let tone: "on" | "off" = "off";
  export let busy = false;
  /** Vertical center of the ring family, in rem. */
  export let cy = 32.4;

  const CENTER_X = 11.25; // rem — the middle of the 22.5rem stage
  const START_DEG = -40; // the arcs stay open at the top
  const SWEEP_DEG = 260;

  type Ring = { r: number; dot: number; opacity: number };

  // Radii measured from the reference render.
  const ON_ARCS: Ring[] = [
    { r: 5.25, dot: 0.15, opacity: 0.95 }, // 84px
    { r: 6.375, dot: 0.14, opacity: 0.85 }, // 102px
    { r: 7.5, dot: 0.13, opacity: 0.72 }, // 120px
    { r: 8.625, dot: 0.12, opacity: 0.55 }, // 138px
    { r: 9.75, dot: 0.11, opacity: 0.4 }, // 156px
  ];
  const OFF_RING: Ring[] = [{ r: 7.75, dot: 0.14, opacity: 0.85 }]; // 124px

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

  $: rings = (tone === "on" ? ON_ARCS : OFF_RING).map((ring) => ({
    ...ring,
    dots: ringDots(ring.r),
  }));
</script>

<svg class="rings" class:busy data-tone={tone} viewBox="0 0 22.5 47.5" aria-hidden="true" focusable="false">
  <g class="family" style:--cy={`${cy}`}>
    {#each rings as ring, ri}
      {#each ring.dots as dot, di}
        <circle
          cx={dot.x}
          cy={dot.y}
          r={ring.dot}
          fill-opacity={ring.opacity}
          style:animation-delay={`${(ri * 5 + di) * 0.045}s`}
        />
      {/each}
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
    fill: var(--ring-dot);
    /* Inside an SVG, CSS px = user units, and one user unit is one rem in
       this viewBox (360px / 22.5 units = 16px), so --cy is unitless. */
    transform: translateY(calc(var(--cy) * 1px));
    transition: transform var(--dur-tone) ease;
    animation: pulse var(--dur-pulse) ease-in-out infinite;
  }

  .rings.busy .family {
    animation-duration: var(--dur-pulse-busy);
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
