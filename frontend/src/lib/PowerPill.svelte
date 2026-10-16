<script lang="ts">
  // PowerPill.svelte — the one big control (design.md): a vertical pill in a
  // recessed track. Down = connect, up = disconnect; click works too; the
  // threshold is halfway. Labels: START / CANCEL / STOP / WAIT.
  import { createEventDispatcher, onMount } from "svelte";
  import type { Phase } from "./types";

  export let phase: Phase = "disconnected";

  const dispatch = createEventDispatcher<{ connect: void; disconnect: void }>();

  const LABELS: Record<Phase, string> = {
    disconnected: "START",
    connecting: "CANCEL",
    connected: "STOP",
    disconnecting: "WAIT",
    error: "START",
  };

  $: on = phase === "connected" || phase === "disconnecting";
  $: busy = phase === "connecting" || phase === "disconnecting";
  $: label = LABELS[phase];

  // The travel lives in tokens.css (--travel, 4.875rem = 78px at the
  // design size); the window may scale the root font, so keep the pixel
  // value current for drags.
  let remPx = 16;
  let travelRem = 4.875;
  onMount(() => {
    const measure = () => {
      remPx = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
      const t = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--travel"));
      if (t > 0) travelRem = t;
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  });
  $: travelPx = travelRem * remPx;

  // Drag state. The thumb rests at one end (top when off, bottom when on)
  // and follows the pointer between them while dragging.
  let dragging = false;
  let pressed = false;
  let startY = 0;
  let dragPx = 0;
  let movedPx = 0;

  $: restingPx = on ? travelPx : 0;
  $: dragOffset = dragging ? dragPx : 0;
  $: offsetPx = Math.max(0, Math.min(travelPx, restingPx + dragOffset));

  // Armed = the drag has crossed the halfway threshold, so releasing now
  // would commit. The LED previews the outcome (green when a connect is
  // armed, the resting red when a disconnect is) and the chevron lights up
  // — the control answers "what happens if I let go" before it happens.
  $: armed =
    dragging && !busy && (on ? offsetPx <= travelPx / 2 : offsetPx >= travelPx / 2);

  // Any phase change ends a drag (the commit already happened, or a cancel
  // arrived from elsewhere) so the thumb returns to its resting end.
  let lastPhase: Phase = phase;
  $: if (phase !== lastPhase) {
    lastPhase = phase;
    dragging = false;
    pressed = false;
    dragPx = 0;
    movedPx = 0;
  }

  function down(e: PointerEvent) {
    if (e.button !== 0 && e.pointerType === "mouse") return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    startY = e.clientY;
    movedPx = 0;
    pressed = true;
    dragging = !busy;
    dragPx = 0;
  }

  function move(e: PointerEvent) {
    if (!dragging) return;
    dragPx = e.clientY - startY;
    movedPx = Math.max(movedPx, Math.abs(dragPx));
  }

  function up() {
    const total = offsetPx; // capture the visual offset before the drag state resets
    const moved = movedPx;
    const wasDragging = dragging;
    dragging = false;
    pressed = false;
    dragPx = 0;
    if (moved < 6) {
      activate(); // a tap, not a drag (this also cancels while busy)
      return;
    }
    if (!wasDragging) return;
    if (!on && total >= travelPx / 2) {
      dispatch("connect");
    } else if (on && total <= travelPx / 2) {
      dispatch("disconnect");
    } // else: short drag snaps back
  }

  function activate() {
    switch (phase) {
      case "disconnected":
      case "error":
        dispatch("connect");
        break;
      case "connecting":
      case "connected":
        dispatch("disconnect");
        break;
      // disconnecting: nothing to do while it settles
    }
  }

  function keydown(e: KeyboardEvent) {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        if (phase === "disconnected" || phase === "error") dispatch("connect");
        break;
      case "ArrowUp":
        e.preventDefault();
        if (phase === "connected") dispatch("disconnect");
        break;
      case "Enter":
      case " ":
        e.preventDefault();
        activate();
        break;
    }
  }
</script>

<div class="track" aria-hidden="true">
  <!-- double chevron in the empty half: points where a swipe would go, and
       breathes gently toward it; lights up and pauses once the drag is
       armed (past halfway) -->
  <svg class="chev down" class:visible={!busy && !on} class:armed={armed && !on} viewBox="0 0 16 16">
    <path d="M2 3.5L8 9l6-5.5" />
    <path d="M2 8.5L8 14l6-5.5" />
  </svg>
  <svg class="chev up" class:visible={!busy && on} class:armed={armed && on} viewBox="0 0 16 16">
    <path d="M2 12.5L8 7l6 5.5" />
    <path d="M2 7.5L8 2l6 5.5" />
  </svg>
</div>

<button
  type="button"
  class="thumb"
  class:dragging
  role="switch"
  aria-checked={on ? "true" : "false"}
  aria-label="Connection"
  style:transform={`translateY(${offsetPx}px) scale(${pressed ? 0.98 : 1})`}
  on:pointerdown={down}
  on:pointermove={move}
  on:pointerup={up}
  on:pointercancel={() => {
    dragging = false;
    pressed = false;
    dragPx = 0;
  }}
  on:keydown={keydown}
>
  <span
    class="led"
    class:lit={armed ? !on : phase === "connected"}
    class:blink={busy}
    aria-hidden="true"
  ></span>
  <span class="label" aria-hidden="true">{label}</span>
  {#if phase === "connecting"}
    <span class="well" aria-hidden="true"><span class="spinner"></span></span>
  {:else}
    <span class="well" aria-hidden="true">
      <svg class="power" viewBox="0 0 24 24">
        <path d="M12 3v8" />
        <circle cx="12" cy="14.5" r="5.6" />
      </svg>
    </span>
  {/if}
</button>

<style>
  .track {
    position: absolute;
    top: var(--track-top);
    left: 50%;
    width: var(--track-w);
    height: var(--track-h);
    margin-left: calc(var(--track-w) / -2);
    border-radius: 3rem;
    background: linear-gradient(180deg, var(--track-a), var(--track-b));
    box-shadow:
      inset 0 0.25rem 0.5rem rgba(0, 0, 0, 0.45),
      inset 0 -0.0625rem 0 rgba(255, 255, 255, 0.05),
      0 0.5rem 1rem rgba(0, 0, 0, 0.2);
  }

  .chev {
    position: absolute;
    left: 50%;
    width: 1.25rem;
    height: 1.25rem;
    margin-left: -0.625rem;
    fill: none;
    stroke: rgba(255, 255, 255, 0.92);
    stroke-width: 2.2;
    stroke-linecap: round;
    stroke-linejoin: round;
    opacity: 0;
    transition:
      opacity var(--dur-fast) ease,
      stroke var(--dur-fast) ease,
      filter var(--dur-fast) ease;
    pointer-events: none;
  }
  .chev.down {
    top: 9.25rem;
  }
  .chev.up {
    top: 2rem;
  }
  .chev.visible {
    opacity: 1;
    animation: nudge var(--dur-pulse) ease-in-out infinite;
  }
  .chev.down.visible {
    animation-name: nudge-down;
  }
  .chev.up.visible {
    animation-name: nudge-up;
  }
  /* armed: past halfway — the nudge pauses and the chevron lights up */
  .chev.armed {
    animation-play-state: paused;
    stroke: #fff;
    filter: drop-shadow(0 0 0.375rem rgba(255, 255, 255, 0.5));
  }
  @keyframes nudge-down {
    0%,
    100% {
      transform: translateY(0);
    }
    50% {
      transform: translateY(0.125rem);
    }
  }
  @keyframes nudge-up {
    0%,
    100% {
      transform: translateY(0);
    }
    50% {
      transform: translateY(-0.125rem);
    }
  }

  .thumb {
    position: absolute;
    top: calc(var(--track-top) + 0.375rem);
    left: 50%;
    width: var(--thumb-w);
    height: var(--thumb-h);
    margin-left: calc(var(--thumb-w) / -2);
    border-radius: 2.5625rem;
    background: linear-gradient(180deg, var(--thumb-a), var(--thumb-b));
    box-shadow:
      0 0 1.75rem var(--red-glow),
      0 0.5rem 1.25rem rgba(0, 0, 0, 0.4),
      inset 0 0.125rem 0 rgba(255, 255, 255, 0.35),
      inset 0 -0.1875rem 0.375rem rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 2.85rem;
    gap: 0.75rem;
    cursor: pointer;
    touch-action: none;
    user-select: none;
    -webkit-user-select: none;
    transition:
      transform var(--dur-slide) var(--ease-out),
      box-shadow var(--dur-fast) ease;
  }
  .thumb.dragging {
    transition: box-shadow var(--dur-fast) ease;
  }
  .thumb:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.25rem;
  }

  .led {
    position: absolute;
    top: 1.5rem;
    left: 50%;
    width: 1.125rem;
    height: 0.3125rem;
    margin-left: -0.5625rem;
    border-radius: 0.3125rem;
    background: var(--red-led-dim);
    transition:
      background var(--dur-fast) ease,
      box-shadow var(--dur-fast) ease;
  }
  .led.lit {
    background: var(--green-led);
    box-shadow: 0 0 0.5rem var(--green-led-peak);
  }
  .led.blink {
    background: #fff;
    animation: led-blink 0.9s steps(2, start) infinite;
  }

  .label {
    font-size: 0.9375rem;
    font-weight: var(--fw-bold);
    letter-spacing: 0.1em;
    color: #fff;
    text-shadow: 0 0.0625rem 0.25rem rgba(0, 0, 0, 0.3);
  }

  .power {
    width: 1.375rem;
    height: 1.375rem;
    fill: none;
    stroke: #fff;
    stroke-width: 2.4;
    stroke-linecap: round;
    opacity: 0.97;
  }

  /* the icon sits in a slightly darker well, like the reference render */
  .well {
    width: 1.875rem;
    height: 1.875rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.18);
    box-shadow: inset 0 0.0625rem 0.25rem rgba(0, 0, 0, 0.25);
  }

  .spinner {
    width: 1.3125rem;
    height: 1.3125rem;
    border-radius: 50%;
    border: 0.1875rem solid rgba(255, 255, 255, 0.25);
    border-top-color: #fff;
    animation: spin var(--dur-spin) linear infinite;
  }

  @keyframes led-blink {
    to {
      background: var(--red-led-dim);
    }
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
