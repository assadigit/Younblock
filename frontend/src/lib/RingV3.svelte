<script lang="ts">
  // RingV3.svelte — the v3 ring decoration (docs/ui-v3/UI-SPEC.md §5, ring
  // row). The drawing code is the existing DotRings approach, recolored and
  // re-parameterized per the spec's "recolor it, do not rewrite it" rule:
  // dots computed once along a circle (static positions — all motion is
  // CSS/SVG), one set per state, cross-faded on the state change exactly
  // like DotRings cross-fades on the tone flip.
  //
  //   disconnected: sparse dashed ring, --yb-ring-idle (static)
  //   connecting:   warm-white dots + a rotating sweep
  //   failed:       dashed ring in amber (static)
  //   connected:    dense dots, breathing 0.10-0.35 (--yb-breathe)
  //
  // Under prefers-reduced-motion every set shows its static version: the
  // ring opacity still differs per state (--state-ring-opacity).
  import type { V3State } from "./v3";

  export let state: V3State = "disconnected";

  // The SVG fills the button zone (a var(--yb-ring-size) square) and works
  // in a 100x100 user space: 1 unit = ring-size/100.
  const C = 50;
  const R = 46; // just inside the zone's edge
  const TAU = Math.PI * 2;

  type Pt = { x: number; y: number };

  /** n dots evenly on the full circle (DotRings' ringDots math, full sweep). */
  function ringDots(n: number, r = R): Pt[] {
    const dots: Pt[] = [];
    for (let i = 0; i < n; i++) {
      const a = (TAU * i) / n - Math.PI / 2; // start at 12 o'clock
      dots.push({ x: C + r * Math.cos(a), y: C + r * Math.sin(a) });
    }
    return dots;
  }

  /** n dots along an arc of `deg` degrees starting at the top (the sweep). */
  function arcDots(n: number, deg: number, r = R): Pt[] {
    const dots: Pt[] = [];
    for (let i = 0; i < n; i++) {
      const a = -Math.PI / 2 + ((deg * Math.PI) / 180) * (i / (n - 1));
      dots.push({ x: C + r * Math.cos(a), y: C + r * Math.sin(a) });
    }
    return dots;
  }

  // the sets, computed once (never per frame)
  const idle = ringDots(30); // sparse
  const connectBase = ringDots(48);
  const sweep = arcDots(14, 70);
  const live = ringDots(64); // dense
</script>

<svg class="rings" viewBox="0 0 100 100" aria-hidden="true" focusable="false">
  <g class="ring idle" class:active={state === "disconnected"}>
    {#each idle as d}
      <circle cx={d.x} cy={d.y} r={1.6} />
    {/each}
  </g>

  <g class="ring connecting" class:active={state === "connecting"}>
    <g class="base">
      {#each connectBase as d}
        <circle cx={d.x} cy={d.y} r={1.4} />
      {/each}
    </g>
    <g class="sweep">
      {#each sweep as d}
        <circle cx={d.x} cy={d.y} r={1.7} />
      {/each}
    </g>
  </g>

  <g class="ring failed" class:active={state === "failed"}>
    <circle class="dash" cx={C} cy={C} r={R} />
  </g>

  <g class="ring connected" class:active={state === "connected"}>
    {#each live as d}
      <circle cx={d.x} cy={d.y} r={1.5} />
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

  /* Each set is two layers: the outer owns the cross-fade (its own element,
     so it composes with the animation inside), exactly like DotRings. The
     fill lives on the set so the outgoing color survives the fade. */
  .ring {
    opacity: 0;
    transition: opacity var(--yb-t-color) ease;
    fill: var(--yb-ring-live);
  }
  .ring.active {
    opacity: var(--state-ring-opacity);
  }
  .ring.idle {
    fill: var(--yb-ring-idle);
  }
  .ring.failed {
    fill: var(--yb-amber);
  }

  /* connecting: the base dots sit dimmer than the sweep above them */
  .connecting .base {
    opacity: 0.45;
  }
  .connecting .sweep {
    animation: sweep-rotate 2.6s linear infinite;
    transform-origin: 50px 50px;
  }

  /* connected: the whole set breathes 0.10-0.35 (the animation replaces the
     set's opacity while it runs; killed by reduced motion, the set then
     rests at its static --state-ring-opacity) */
  .ring.connected.active {
    animation: ring-breathe var(--yb-breathe) ease-in-out infinite;
  }

  .failed .dash {
    fill: none;
    stroke: var(--yb-amber);
    stroke-width: 1.6;
    stroke-dasharray: 5 5.83; /* one period: circumference / 27.5 = 27.5 dashes */
  }

  @keyframes sweep-rotate {
    to {
      transform: rotate(360deg);
    }
  }

  @keyframes ring-breathe {
    0%, 100% {
      opacity: 0.1;
    }
    50% {
      opacity: 0.35;
    }
  }
</style>
