<script lang="ts">
  // LeverButton.svelte — the physical key cap on its recessed track
  // (the refinement round, UI-SPEC (2).md §2/§3, every value ported from
  // younblock-ui-reference (1).html).
  //
  // The pill is a 3D KEY CAP: a satin red top face with a solid 9px side
  // wall (var(--edge)) and a soft cast shadow; it rests 18px above the
  // track floor so the wall is visible. Pressing it sinks the cap 6px and
  // shrinks the wall to 3px — like pressing a real key.
  //
  // The lever: the cap rests at the BOTTOM of the track (the thumb zone);
  // pulling it UP connects, pulling it back DOWN disconnects. While the
  // finger is down the cap FOLLOWS the finger; releasing past 45% of the
  // travel commits, otherwise it springs back. A plain tap also works.
  // Gestures are inert while connecting (the reference ignores them there).
  //
  // No rings, no circles, no dot decorations anywhere: the track's own
  // pill shape carries every state through its inset shadows + the soft
  // --c-aura glow (keyframes aura / aurabeat). All motion is CSS.

  import { createEventDispatcher, onMount } from "svelte";
  import { press as hapticPress } from "./haptics";
  import { V3_COPY, type LeverAction, type V3State } from "./v3";

  export let state: V3State = "idle";

  const dispatch = createEventDispatcher<{
    trigger: { action: LeverAction; source: "tap" | "swipe" | "key" };
  }>();

  // ---- geometry: travel is measured from the live DOM with the same
  // formula as --travel (track height - cap height - 8 - 18), so the JS
  // and the CSS can never drift apart.
  let trackEl: HTMLElement;
  let pillEl: HTMLElement;
  let travel = 122; // 272 - 124 - 8 - 18 (the reference values)
  const clamp = (v: number): number => Math.max(0, Math.min(travel, v));

  function measure(): void {
    if (!trackEl || !pillEl) return;
    const th = trackEl.getBoundingClientRect().height;
    const ch = pillEl.getBoundingClientRect().height;
    const pad = parseFloat(getComputedStyle(trackEl).paddingTop) || 8;
    const padBottom = parseFloat(getComputedStyle(trackEl).paddingBottom) || 18;
    travel = Math.max(0, th - ch - pad - padBottom);
  }

  onMount(() => {
    measure();
    // The any-version floor (the 1.6.7-debug.2 round): ResizeObserver is
    // Chrome 64+ — an Android 8.0 factory WebView throws here and kills
    // the main screen. The window resize listener covers every real
    // resize case (the layout is viewport-driven); the observer is a
    // belt-and-braces refinement old WebViews simply skip.
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

  // ---- the one action this state's committed trigger maps to
  function actionFor(s: V3State): LeverAction | null {
    switch (s) {
      case "idle":
        return "connect"; // tap or swipe up starts connecting
      case "failed":
        return "retry"; // tap or swipe up retries
      case "connected":
        return "disconnect"; // tap or swipe down disconnects
      case "connecting":
        return null; // inert: the reference ignores gestures while connecting
    }
  }

  // ---- gesture state (the reference's pointerdown/move/up, ported)
  let dragging = false;
  let y0: number | null = null;
  let base = 0; // the cap's offset when the gesture started
  let moved = 0; // the farthest the finger has moved
  let activePointer = -1; // one pointer at a time
  let lastFire = -1000;

  // a committed trigger's visual head start: the cap finishes the move
  // immediately while the engine's confirming event is in flight (the
  // app's existing refinement — the reference demo flips state on a
  // timer, the real engine answers asynchronously). Cleared when the
  // state lands on the same end, or after 500ms (a refused call reverts
  // to the honest resting end).
  let pendingTimer: number | undefined;

  function fire(a: LeverAction, via: "tap" | "swipe" | "key"): boolean {
    const now = performance.now();
    if (now - lastFire < 600) return false; // debounce every trigger
    lastFire = now;
    // the cap finishes the move in the gesture's direction: up connects
    // (translateY(0)), down disconnects (translateY(travel))
    if (pillEl) pillEl.style.transform = `translateY(${a === "disconnect" ? travel : 0}px)`;
    window.clearTimeout(pendingTimer);
    pendingTimer = window.setTimeout(() => {
      if (pillEl) pillEl.style.transform = "";
    }, 500);
    dispatch("trigger", { action: a, source: via });
    return true;
  }

  function go(via: "tap" | "swipe" | "key"): void {
    const a = actionFor(state);
    if (a) fire(a, via);
  }

  function down(e: PointerEvent): void {
    if (state === "connecting") return; // inert while connecting
    if (activePointer !== -1) return; // one pointer at a time
    if (e.pointerType === "mouse" && e.button !== 0) return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    activePointer = e.pointerId;
    y0 = e.clientY;
    moved = 0;
    base = state === "connected" ? 0 : travel;
    dragging = true;
    hapticPress(); // the key press haptic (UI-SPEC §3 item 12)
    // the cap is the finger while it is down: no transition, no glow loop
    if (pillEl) {
      pillEl.style.transition = "none";
      pillEl.style.animation = "none";
    }
  }

  function move(e: PointerEvent): void {
    if (y0 === null || !dragging || e.pointerId !== activePointer) return;
    const dy = e.clientY - y0;
    moved = Math.max(moved, Math.abs(dy));
    // the cap follows the finger, clamped to the track
    if (pillEl) pillEl.style.transform = `translateY(${clamp(base + dy)}px)`;
  }

  function release(): void {
    dragging = false;
    y0 = null;
    activePointer = -1;
    if (pillEl) {
      pillEl.style.transition = "";
      pillEl.style.animation = "";
    }
  }

  function up(e: PointerEvent): void {
    if (y0 === null || e.pointerId !== activePointer) return;
    const dy = e.clientY - y0;
    const wasMoved = moved >= 10;
    const fromTop = state === "connected";
    // releasing past 45% of the travel in the counted direction commits
    const commit = wasMoved && (fromTop ? dy > travel * 0.45 : dy < -travel * 0.45);
    // release the gesture: the transition + the glow loop return
    dragging = false;
    y0 = null;
    activePointer = -1;
    if (pillEl) {
      pillEl.style.transition = "";
      pillEl.style.animation = "";
    }
    if (!wasMoved) {
      // a plain tap: spring back from any sub-threshold jitter, then fire
      // (fire() pins the committed end)
      if (pillEl) pillEl.style.transform = "";
      go("tap");
    } else if (commit) {
      go("swipe"); // fire() pins the committed end
    } else {
      // released before the threshold: the cap springs back to its end
      if (pillEl) pillEl.style.transform = "";
    }
  }

  function cancel(e: PointerEvent): void {
    if (e.pointerId !== activePointer) return;
    release(); // restore the transition + the glow loop
    if (pillEl) pillEl.style.transform = ""; // springs back, never commits
  }

  function keydown(e: KeyboardEvent): void {
    if (e.key !== "Enter" && e.key !== " ") return;
    e.preventDefault();
    go("key");
  }

  // ---- a state change ends any active gesture and clears the pending
  // head start: the state's own resting end rules now (connect = top,
  // disconnect = the honest resting end after a refused call).
  let lastState: V3State = "idle";
  $: if (state !== lastState) {
    lastState = state;
    dragging = false;
    y0 = null;
    activePointer = -1;
    window.clearTimeout(pendingTimer);
    if (pillEl) {
      pillEl.style.transition = "";
      pillEl.style.animation = "";
      pillEl.style.transform = "";
    }
  }

  $: capLabel = V3_COPY[state].cap;
  $: ariaLabel = V3_COPY[state].hint; // the track's aria-label IS the current hint (UI-SPEC §5)
</script>

<!-- the hit area: the track plus --yb-touch-padding of invisible padding
     on all sides. touch-action none + user-select none. -->
<div
  class="hit"
  class:dragging
  role="button"
  tabindex="0"
  aria-label={ariaLabel}
  on:pointerdown={down}
  on:pointermove={move}
  on:pointerup={up}
  on:pointercancel={cancel}
  on:keydown={keydown}
>
  <div class="track" bind:this={trackEl}>
    <!-- the connected state's single static down chevron -->
    <svg class="chev down" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M5 4l5 5 5-5M5 10l5 5 5-5" />
    </svg>

    <!-- the swipe-up cue (idle and failed only): three chevrons light up
         one after another, bottom to top, 1.8s loop. It fades out
         instantly while the user is dragging. -->
    <div class="swipe-up" aria-hidden="true">
      <svg viewBox="0 0 28 16" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 13l10-9 10 9" />
      </svg>
      <svg viewBox="0 0 28 16" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 13l10-9 10 9" />
      </svg>
      <svg viewBox="0 0 28 16" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 13l10-9 10 9" />
      </svg>
    </div>

    <!-- the key cap -->
    <div class="pill" bind:this={pillEl}>
      <div class="led"></div>
      <div class="grip"><i></i><i></i><i></i></div>
      <div class="label">{capLabel}</div>
      <div class="pwr">
        <svg viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.9" stroke-linecap="round">
          <path d="M12 3v8M6.3 6.8a8 8 0 1 0 11.4 0" />
        </svg>
      </div>
    </div>
  </div>
</div>

<style>
  /* the hit area: the track centered in the button zone, the padding
     expanding the touch target */
  .hit {
    position: absolute;
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
    border-radius: calc(var(--r-track) + var(--yb-touch-padding));
  }

  /* ---- the track: a recessed slot with inset shadows, NO outline.
       The padding (8 / 8 / 18) mirrors --pad/--pad-bottom so measure()
       reads the real geometry; the cap is positioned by the same vars. */
  .track {
    position: relative;
    width: var(--track-w);
    height: var(--track-h);
    padding: var(--pad) var(--pad) var(--pad-bottom);
    border-radius: var(--r-track);
    background: linear-gradient(180deg, rgba(0, 0, 0, 0.38), rgba(0, 0, 0, 0.16) 60%, rgba(255, 255, 255, 0.03));
    box-shadow:
      inset 0 10px 16px rgba(0, 0, 0, 0.55),
      inset 0 -3px 5px rgba(255, 255, 255, 0.08),
      0 1px 0 rgba(255, 255, 255, 0.1),
      0 0 0 1px rgba(0, 0, 0, 0.28);
    transition: box-shadow 0.6s;
  }

  /* ---- the connected state's single static down chevron */
  .chev {
    position: absolute;
    left: 50%;
    width: 20px;
    height: 20px;
    margin-left: -10px;
    color: rgba(255, 255, 255, 0.85);
    transition: all 0.5s;
    pointer-events: none;
  }
  .chev.down {
    bottom: 18px;
    display: none;
  }
  :global(.yb-screen[data-state="connected"]) .chev.down {
    display: block;
  }

  /* ---- the swipe-up cue: three chevrons in the empty top part of the
       track, lighting up one after another, bottom to top, 1.8s loop,
       peak opacity .45, drifting 8px upward as they fade. */
  .swipe-up {
    position: absolute;
    left: 50%;
    top: 30px;
    width: 30px;
    margin-left: -15px;
    display: none;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    color: #fff;
    pointer-events: none;
    transition: opacity 0.2s;
  }
  .swipe-up svg {
    width: 28px;
    height: 16px;
    opacity: 0;
    animation: swipecue 1.8s ease-in-out infinite;
  }
  .swipe-up svg:nth-child(1) {
    animation-delay: 0.5s;
  }
  .swipe-up svg:nth-child(2) {
    animation-delay: 0.25s;
  }
  .swipe-up svg:nth-child(3) {
    animation-delay: 0s;
  }
  :global(.yb-screen[data-state="idle"]) .swipe-up,
  :global(.yb-screen[data-state="failed"]) .swipe-up {
    display: flex;
  }
  /* they fade out instantly while the user is dragging */
  .hit.dragging .swipe-up {
    opacity: 0;
  }
  /* reduced motion: static at .4 */
  @media (prefers-reduced-motion: reduce) {
    .swipe-up svg {
      opacity: 0.4 !important;
      transform: none !important;
    }
  }

  /* ---- the key cap: satin red top face, a solid 9px side wall
       (var(--edge)) and a soft cast shadow on the track. The travel
       rides on transform; the 6px press sink rides on margin-top (the
       reference uses the separate `translate` property — same numbers —
       because the app's WebView floor predates it). */
  .pill {
    position: absolute;
    left: var(--pad);
    top: var(--pad);
    width: var(--pill-w);
    height: var(--pill-h);
    border-radius: var(--r-cap);
    background: linear-gradient(180deg, #ec3d34 0%, #d62a20 55%, #c8231d 100%);
    box-shadow: var(--cap-rest), 0 0 30px var(--pill-glow);
    transform: translateY(var(--travel)); /* rests at the BOTTOM */
    transition:
      transform var(--yb-t-cap) var(--yb-ease-cap),
      margin-top 0.12s ease-out,
      box-shadow 0.12s ease-out;
    touch-action: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: space-between;
    padding: 13px 0 12px;
    z-index: 1;
  }
  /* the very soft sheen across the top face */
  .pill:before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    border-radius: inherit;
    background: radial-gradient(130% 40% at 50% 0%, rgba(255, 255, 255, 0.2), transparent 70%);
    pointer-events: none;
  }
  /* connected and connecting: the cap sits at the TOP */
  :global(.yb-screen[data-state="connected"]) .pill,
  :global(.yb-screen[data-state="connecting"]) .pill {
    transform: translateY(0);
  }
  /* PRESSED: while the finger is down the cap sinks 6px and its side
     wall shrinks from 9px to 3px — like pressing a real key */
  .hit.dragging .pill {
    margin-top: 6px;
    box-shadow: var(--cap-press), 0 0 20px var(--pill-glow) !important;
    animation: none !important;
  }

  /* ---- the controls on the cap ---- */
  /* the status LED slot */
  .led {
    width: 24px;
    height: 4px;
    border-radius: 2px;
    background: var(--c-led);
    box-shadow: inset 0 1px 1px rgba(0, 0, 0, 0.4), 0 0 10px var(--c-led-glow);
    transition: background var(--yb-t-color), box-shadow var(--yb-t-color);
    position: relative;
    z-index: 1;
  }
  :global(.yb-screen[data-state="connecting"]) .led {
    animation: blink 1s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="connected"]) .led {
    animation: ledbreath 3.4s ease-in-out infinite;
  }

  /* the 3 grip ridges (engraved look) */
  .grip {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 34px;
    position: relative;
    z-index: 1;
  }
  .grip i {
    height: 2px;
    border-radius: 1px;
    background: rgba(70, 0, 0, 0.4);
    box-shadow: 0 1px 0 rgba(255, 255, 255, 0.18);
  }

  /* the cap label */
  .label {
    font-family: var(--font);
    font-weight: 700;
    font-size: var(--yb-fs-cap);
    letter-spacing: 2.2px;
    text-transform: uppercase;
    text-shadow: 0 1px 1px rgba(90, 0, 0, 0.45);
    position: relative;
    z-index: 1;
  }

  /* the thin line power icon (no dark disc behind it) */
  .pwr {
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    position: relative;
    z-index: 1;
  }
  .pwr svg {
    width: 24px;
    height: 24px;
    filter: drop-shadow(0 1px 0 rgba(90, 0, 0, 0.45));
  }

  /* ---- ANIMATION (keyframes ported verbatim from the reference) ---- */

  /* the track's own glow, breathing (aura) or double-beating (aurabeat):
     the full inset shadow stack plus the soft --c-aura glow */
  @keyframes aura {
    0%, 100% {
      box-shadow:
        inset 0 10px 16px rgba(0, 0, 0, 0.55),
        inset 0 -3px 5px rgba(255, 255, 255, 0.08),
        0 1px 0 rgba(255, 255, 255, 0.1),
        0 0 0 1px rgba(0, 0, 0, 0.28),
        0 0 14px 1px var(--c-aura);
    }
    50% {
      box-shadow:
        inset 0 10px 16px rgba(0, 0, 0, 0.55),
        inset 0 -3px 5px rgba(255, 255, 255, 0.08),
        0 1px 0 rgba(255, 255, 255, 0.1),
        0 0 0 1px rgba(0, 0, 0, 0.28),
        0 0 46px 8px var(--c-aura);
    }
  }
  @keyframes aurabeat {
    0%, 100%, 48% {
      box-shadow:
        inset 0 10px 16px rgba(0, 0, 0, 0.55),
        inset 0 -3px 5px rgba(255, 255, 255, 0.08),
        0 1px 0 rgba(255, 255, 255, 0.1),
        0 0 0 1px rgba(0, 0, 0, 0.28),
        0 0 14px 1px var(--c-aura);
    }
    12%, 36% {
      box-shadow:
        inset 0 10px 16px rgba(0, 0, 0, 0.55),
        inset 0 -3px 5px rgba(255, 255, 255, 0.08),
        0 1px 0 rgba(255, 255, 255, 0.1),
        0 0 0 1px rgba(0, 0, 0, 0.28),
        0 0 40px 7px var(--c-aura);
    }
    24% {
      box-shadow:
        inset 0 10px 16px rgba(0, 0, 0, 0.55),
        inset 0 -3px 5px rgba(255, 255, 255, 0.08),
        0 1px 0 rgba(255, 255, 255, 0.1),
        0 0 0 1px rgba(0, 0, 0, 0.28),
        0 0 22px 3px var(--c-aura);
    }
  }
  @keyframes ledbreath {
    0%, 100% {
      box-shadow: 0 0 6px var(--c-led-glow);
    }
    50% {
      box-shadow: 0 0 16px 2px var(--c-led-glow);
    }
  }
  @keyframes blink {
    50% {
      opacity: 0.25;
    }
  }
  @keyframes swipecue {
    0% {
      opacity: 0;
      transform: translateY(8px);
    }
    35% {
      opacity: 0.45;
    }
    65% {
      opacity: 0.45;
    }
    100% {
      opacity: 0;
      transform: translateY(-8px);
    }
  }
  @keyframes pulsepill {
    0%, 100% {
      box-shadow: var(--cap-rest), 0 0 22px rgba(255, 40, 30, 0.25);
    }
    45% {
      box-shadow: var(--cap-rest), 0 0 80px rgba(255, 60, 45, 0.85);
    }
  }
  @keyframes pulselabel {
    0%, 100% {
      opacity: 0.55;
    }
    45% {
      opacity: 1;
    }
  }
  @keyframes glow {
    0%, 100% {
      box-shadow: var(--cap-rest), 0 0 26px rgba(255, 40, 30, 0.28);
    }
    50% {
      box-shadow: var(--cap-rest), 0 0 52px rgba(255, 40, 30, 0.5);
    }
  }

  /* the state animations: connecting pulses everything in one 1.1s
     rhythm; connected breathes slowly (3.4s, LED in sync); failed
     double-beats calmly (3.6s); idle and failed keep the slow red pill
     glow breathing (4s) */
  :global(.yb-screen[data-state="connected"]) .track {
    animation: aura 3.4s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="connecting"]) .track {
    animation: aura 1.1s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="failed"]) .track {
    animation: aurabeat 3.6s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="connecting"]) .pill {
    animation: pulsepill 1.1s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="connecting"]) .label {
    animation: pulselabel 1.1s ease-in-out infinite;
  }
  :global(.yb-screen[data-state="idle"]) .pill,
  :global(.yb-screen[data-state="failed"]) .pill {
    animation: glow 4s ease-in-out infinite;
  }
</style>
