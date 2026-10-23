<script lang="ts">
  // LeverButton.svelte — the v3 mechanical lever (docs/ui-v3/UI-SPEC.md §4).
  // A dark pill (track) with a red cap that travels top↔bottom; the cap's
  // end carries the state, the words on it say what a tap does. The v2
  // PowerPill's gesture model, retimed to the v3 tokens: tap under 10px and
  // under 400ms; a drag moves the cap with the finger and COMMITS at 60% of
  // the travel (the cap then finishes the move); released earlier it springs
  // back with a small overshoot. Every trigger is debounced 600ms. All
  // motion is CSS; positions are computed once (never per frame).

  import { createEventDispatcher, onMount } from "svelte";
  import { ChevronsDown, ChevronsUp } from "./icons";
  import { tick as hapticTick } from "./haptics";
  import { V3_COPY, type LeverAction, type V3State } from "./v3";

  export let state: V3State = "disconnected";

  const dispatch = createEventDispatcher<{
    trigger: { action: LeverAction; source: "tap" | "swipe" | "key" };
  }>();

  // ---- geometry, measured from the live DOM (the track's clamp() and the
  // cap sizes are token-driven). travel = trackH - capH - 2*trackPadding.
  let trackEl: HTMLElement;
  let capEl: HTMLElement;
  let travel = 120;
  const clamp = (v: number): number => Math.max(0, Math.min(travel, v));

  function measure(): void {
    if (!trackEl || !capEl) return;
    const th = trackEl.getBoundingClientRect().height;
    const ch = capEl.getBoundingClientRect().height;
    const pad = parseFloat(getComputedStyle(trackEl).paddingTop) || 8;
    travel = Math.max(0, th - ch - pad * 2);
  }

  onMount(() => {
    measure();
    // The any-version floor (the 1.6.7-debug.2 round): ResizeObserver
    // is Chrome 64+ — an Android 8.0 factory WebView (Chrome ~58, common
    // where Play cannot update it) throws a ReferenceError right here and
    // kills the main screen, the white-page failure class. The window
    // resize listener below already covers the track's every real
    // resize case (the layout is viewport-driven; rotation arrives as a
    // window resize), so the observer is a belt-and-braces refinement
    // old WebViews simply skip.
    if (typeof ResizeObserver !== "undefined") {
      const ro = new ResizeObserver(measure);
      ro.observe(trackEl);
      window.addEventListener("resize", measure);
      return () => {
        ro.disconnect();
        window.removeEventListener("resize", measure);
      };
    }
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  });

  // ---- resting end: the cap waits at the BOTTOM while connecting and
  // while connected; top for disconnected and failed (spec §4).
  $: resting = state === "connected" || state === "connecting" ? travel : 0;

  // ---- a committed trigger's visual head start: the cap finishes the
  // move immediately (spec: "the cap then finishes the move") while the
  // engine's confirming event is in flight. Cleared when the state lands
  // on the same end, or after 500ms (a refused call reverts to the honest
  // resting end).
  let pendingRest: number | null = null;
  let pendingTimer: number | undefined;

  // ---- gesture state
  let dragging = false;
  let pressed = false;
  let committed = false;
  let activePointer = -1;
  let startX = 0;
  let startY = 0;
  let startT = 0;
  let dragOffset = 0;
  let movedPx = 0;
  let lastFire = -1000;

  // the rendered cap offset: the finger's clamp while dragging, the
  // resting (or pending) end otherwise.
  $: capOffset = dragging ? clamp(resting + dragOffset) : (pendingRest ?? resting);

  function fire(a: LeverAction, via: "tap" | "swipe" | "key"): boolean {
    const now = performance.now();
    if (now - lastFire < 600) return false; // debounce every trigger (spec §4)
    lastFire = now;
    // the cap finishes the move in the gesture's direction
    pendingRest = a === "connect" || a === "retry" ? travel : 0;
    window.clearTimeout(pendingTimer);
    pendingTimer = window.setTimeout(() => (pendingRest = null), 500);
    dispatch("trigger", { action: a, source: via });
    return true;
  }

  /** The one action a committed trigger maps to, per state (spec §4). */
  function actionFor(s: V3State): LeverAction | null {
    switch (s) {
      case "disconnected":
        return "connect"; // tap or swipe down starts connecting
      case "connecting":
        return "cancel"; // any committed trigger cancels
      case "failed":
        return "retry"; // tap or swipe down retries
      case "connected":
        return "disconnect"; // tap or swipe up disconnects
    }
  }

  function commitFromDrag(): void {
    const a = actionFor(state);
    if (!a) return;
    if (!fire(a, "swipe")) return; // debounced: no tick either
    committed = true;
    hapticTick(); // the crossing haptic (spec §4)
    dragging = false;
    pressed = false;
    activePointer = -1;
  }

  function down(e: PointerEvent): void {
    if (activePointer !== -1) return; // one pointer at a time
    if (e.pointerType === "mouse" && e.button !== 0) return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    activePointer = e.pointerId;
    startX = e.clientX;
    startY = e.clientY;
    startT = performance.now();
    movedPx = 0;
    dragOffset = 0;
    committed = false;
    pressed = true;
    dragging = true;
  }

  function move(e: PointerEvent): void {
    if (!dragging || e.pointerId !== activePointer) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    // vertical only: horizontal movement is ignored (spec §4)
    dragOffset = dy;
    movedPx = Math.max(movedPx, Math.hypot(dx, dy));
    const offset = clamp(resting + dy);
    // committed the moment the drag crosses 60% of the travel in the
    // counted direction (down while at the top, up while at the bottom)
    if (!committed && resting === 0 && dy > 0 && offset >= travel * 0.6) {
      commitFromDrag();
    } else if (!committed && resting === travel && dy < 0 && offset <= travel * 0.4) {
      commitFromDrag();
    }
  }

  function up(e: PointerEvent): void {
    if (e.pointerId !== activePointer) return;
    const dur = performance.now() - startT;
    const moved = movedPx;
    const wasDragging = dragging;
    dragging = false;
    pressed = false;
    activePointer = -1;
    dragOffset = 0;
    if (committed) return; // the commit already ended this gesture
    if (!wasDragging) return;
    if (moved < 10 && dur < 400) {
      // a tap (spec §4)
      const a = actionFor(state);
      if (a) fire(a, "tap");
      return;
    }
    if (moved >= 10) {
      // released before the threshold: spring back + tick (spec §4)
      hapticTick();
    }
  }

  function cancel(e: PointerEvent): void {
    if (e.pointerId !== activePointer) return;
    dragging = false;
    pressed = false;
    activePointer = -1;
    dragOffset = 0;
    committed = false;
  }

  function keydown(e: KeyboardEvent): void {
    if (e.key !== "Enter" && e.key !== " ") return;
    e.preventDefault();
    const a = actionFor(state);
    if (a) fire(a, "key");
  }

  // ---- state changes end any active drag (the commit already happened,
  // or an engine event arrived from elsewhere), and entering failed shakes
  // the cap (spec §5). The pre-mount "disconnected" makes a ?demo=failed
  // load play the shake too, exactly like a live failure.
  let lastState: V3State = "disconnected";
  let shaking = false;
  let shakeTimer: number | undefined;
  $: if (state !== lastState) {
    lastState = state;
    dragging = false;
    pressed = false;
    activePointer = -1;
    dragOffset = 0;
    committed = false;
    window.clearTimeout(pendingTimer);
    pendingRest = null; // the state landed; the honest resting end rules
    if (state === "failed") {
      shaking = true;
      window.clearTimeout(shakeTimer);
      shakeTimer = window.setTimeout(() => (shaking = false), 400);
    }
  }

  $: capLabel = V3_COPY[state].cap;
  $: ariaLabel = V3_COPY[state].aria;
  // chevrons sit in the empty half of the track: down at the bottom while
  // the cap rests at the top, up at the top while it rests at the bottom
  $: showDownChev = state === "disconnected" || state === "failed";
  $: showUpChev = state === "connected" || state === "connecting";
</script>

<div
  class="hit"
  role="button"
  tabindex="0"
  aria-label={ariaLabel}
  on:pointerdown={down}
  on:pointermove={move}
  on:pointerup={up}
  on:pointercancel={cancel}
  on:keydown={keydown}
>
  <div class="track" bind:this={trackEl} aria-hidden="true">
    <ChevronsDown class={`chev down${showDownChev ? " visible" : ""}`} />
    <ChevronsUp class={`chev up${showUpChev ? " visible" : ""}`} />
  </div>

  <div
    class="cap"
    bind:this={capEl}
    class:dragging
    aria-hidden="true"
    style:transform={`translateY(${capOffset}px)`}
  >
    <div class="cap-inner" class:shaking>
      <span class="led"></span>
      <span class="cap-label">{capLabel}</span>
    </div>
  </div>
</div>

<style>
  /* the hit area: the track plus --yb-touch-padding of invisible padding
     on all sides (spec §4). touch-action none + user-select none. */
  .hit {
    position: absolute;
    /* the track centered in the zone, the padding expanding the target */
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    padding: var(--yb-touch-padding);
    touch-action: none;
    user-select: none;
    -webkit-user-select: none;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
  }
  .hit:focus-visible {
    outline: 3px solid var(--yb-text);
    outline-offset: 2px;
    border-radius: calc(var(--yb-track-w) / 2 + var(--yb-touch-padding));
  }

  /* the dark pill (spec §4): charcoal lift with the dark rim around it
     (the rim keeps the cap readable on the background, tokens §cap-lo). */
  .track {
    position: relative;
    width: var(--yb-track-w);
    height: var(--yb-track-h);
    border-radius: calc(var(--yb-track-w) / 2);
    background: var(--yb-base-lift);
    border: 1.5px solid var(--yb-cap-lo);
    box-shadow: inset 0 3px 10px rgba(0, 0, 0, 0.35);
    transition: box-shadow var(--yb-t-color) ease;
  }
  /* connected: the track rim glows faintly red (spec §4) */
  :global(.yb-screen[data-state="connected"]) .track {
    box-shadow:
      inset 0 0 18px var(--state-glow),
      inset 0 3px 10px rgba(0, 0, 0, 0.35);
  }

  /* the chevrons in the empty half (Lucide, child components — :global) */
  .track :global(.chev) {
    position: absolute;
    left: 50%;
    width: 22px;
    height: 22px;
    margin-left: -11px;
    fill: none;
    stroke: var(--yb-text-dim);
    stroke-width: 2.2;
    stroke-linecap: round;
    stroke-linejoin: round;
    opacity: 0;
    transition: opacity var(--yb-t-color) ease;
    pointer-events: none;
  }
  .track :global(.chev.down) {
    bottom: 14px;
  }
  .track :global(.chev.up) {
    top: 14px;
  }
  .track :global(.chev.visible) {
    opacity: 0.9;
  }

  /* the cap: red, one highlight edge, a slit LED at its top edge (spec §4).
     Position and travel are token-driven; the transform carries the state. */
  .cap {
    position: absolute;
    /* anchored to the hit box (symmetric padding): its center is the
       track's center; the margin lifts the cap to the track's inner top */
    left: 50%;
    top: 50%;
    margin-left: calc(var(--yb-cap-w) / -2);
    margin-top: calc(var(--yb-track-padding) - var(--yb-track-h) / 2);
    width: var(--yb-cap-w);
    height: var(--yb-cap-h);
    transition: transform var(--yb-t-cap) cubic-bezier(0.34, 1.3, 0.64, 1);
    will-change: transform;
    z-index: 1;
  }
  .cap.dragging {
    transition: none; /* the cap is the finger while it is down */
  }

  .cap-inner {
    position: absolute;
    inset: 0;
    border-radius: calc(var(--yb-cap-w) / 2);
    background: var(--yb-red);
    /* one highlight edge at the top; the glow behind the cap is the
       state's (0 / 12% / 28%) and the drop shadow shrinks when the cap
       is pressed in (connected) */
    box-shadow:
      inset 0 3px 0 var(--yb-cap-hi),
      inset 0 -3px 6px rgba(0, 0, 0, 0.22);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    transition: box-shadow var(--yb-t-color) ease;
  }
  /* connected: pressed in — the cap shadow shrinks and the ember glows
     behind it (the glow itself is --state-glow, 28% red) */
  :global(.yb-screen[data-state="connected"]) .cap-inner {
    box-shadow:
      0 0 42px var(--state-glow),
      inset 0 3px 0 var(--yb-cap-hi),
      inset 0 -2px 4px rgba(0, 0, 0, 0.16);
  }
  :global(.yb-screen[data-state="connecting"]) .cap-inner {
    box-shadow:
      0 0 42px var(--state-glow),
      inset 0 3px 0 var(--yb-cap-hi),
      inset 0 -3px 6px rgba(0, 0, 0, 0.22);
  }

  /* the LED slit at the cap's top edge (spec §4) */
  .led {
    position: absolute;
    top: 11px;
    left: 50%;
    width: 28px;
    height: 4px;
    margin-left: -14px;
    border-radius: 2px;
    background: var(--yb-led);
    opacity: var(--state-led-opacity);
    transition: opacity var(--yb-t-color) ease;
  }
  /* connecting: pulses 0.3 to 1 (spec §5) */
  :global(.yb-screen[data-state="connecting"]) .led {
    animation: led-pulse var(--yb-led-pulse) ease-in-out infinite;
  }
  /* failed: two quick blinks, then dim (spec §5) */
  :global(.yb-screen[data-state="failed"]) .led {
    animation: led-failed 0.6s ease-out 1 both;
  }

  .cap-label {
    font-family: var(--font);
    font-size: var(--yb-fs-cap);
    font-weight: 700;
    letter-spacing: 0.09em;
    color: #fff; /* white on --yb-red: 5.07:1 (tokens) */
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
  }

  /* entering failed: the cap shakes 300ms, +/-6px, 3 cycles (spec §5) */
  .cap-inner.shaking {
    animation: cap-shake var(--yb-t-shake) ease-in-out 1;
  }

  @keyframes cap-shake {
    0% { transform: translateX(0); }
    8.33% { transform: translateX(-6px); }
    25% { transform: translateX(6px); }
    41.67% { transform: translateX(-6px); }
    58.33% { transform: translateX(6px); }
    75% { transform: translateX(-6px); }
    91.67% { transform: translateX(6px); }
    100% { transform: translateX(0); }
  }

  @keyframes led-pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 1; }
  }

  @keyframes led-failed {
    0% { opacity: 0.35; }
    12% { opacity: 1; }
    25% { opacity: 0.35; }
    37% { opacity: 1; }
    50% { opacity: 0.35; }
    100% { opacity: 0.35; }
  }
</style>
