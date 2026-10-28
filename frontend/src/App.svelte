<!-- App.svelte — the Younblock main screen (the refinement round,
     UI-SPEC (2).md v7; every visual value ported from
     younblock-ui-reference (1).html; the proportion round re-centers
     the rhythm on the 360×640 design screen).
     One flex column — header, status block centered in the free space
     above the button, the key-cap lever at --yb-button-center-y, hint,
     the reserved card slot, the safe insets. State is carried by
     data-state/data-health on this root; the state backgrounds are
     three stacked gradient layers cross-fading in 700 ms (never a hard
     cut), with the halftone dot texture + 5% grain over them. No rings,
     no circles anywhere. -->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import DiagnosticsView from './lib/DiagnosticsView.svelte'
  import MenuSheet from './lib/MenuSheet.svelte'
  import LeverButton from './lib/LeverButton.svelte'
  import OpenYouTubeCard from './lib/OpenYouTubeCard.svelte'
  import QuitDialog from './lib/QuitDialog.svelte'
  import Timer from './lib/Timer.svelte'
  import { createBackend } from './lib/backend'
  import { formatBytes, formatDuration, formatDurationWords } from './lib/format'
  import { AlertTriangle, ArrowDown, ArrowUp } from './lib/icons'
  import * as haptics from './lib/haptics'
  import { failedSubLine, V3_COPY, v3State, type V3State } from './lib/v3'
  import { pushBackHandler } from './lib/shellback'
  import type { Snapshot } from './lib/types'

  // =====================================================================
  // CONFIGURATION — edit only this block
  // =====================================================================
  const APP_NAME = 'Younblock'
  /** Small line above the headline. */
  const STATUS_LABEL = 'YouTube'
  /** Path or URL of your logo image (for example '/logo.png' in public/).
   *  Empty = the built-in red play mark (the existing logo, recolored). */
  const LOGO_URL = ''
  /** M8-an (ADR-0052): the detail line while the warm gate holds the
   *  connect-time hand-off — kept from the screen this one replaces. */
  const WARMING_DETAIL = 'Getting the first video ready…'
  // =====================================================================

  const backend = createBackend()

  let snap: Snapshot = { phase: 'disconnected', connectedAtMs: null, downBps: 0, upBps: 0, pingMs: null, downBytes: -1, upBytes: -1, lastSession: null }
  let now = Date.now()
  let menuOpen = false
  // the diagnostics dialog over the stage (unchanged from the previous
  // wiring; the menu sheet's contents are out of scope per spec §1)
  let diagOpen = false
  // the "Quit Younblock" confirmation over everything (D112, the quit
  // round): the sheet's quiet Quit row opens it; the dialog's Quit hands
  // the end-of-app sequence to the Android shell. Android only — the
  // desktop editions never see the row that opens it.
  let quitOpen = false
  let menuButton: HTMLElement | undefined
  let appVersion = ''
  let platform = ''
  let perAppRouting = false
  let autoOpenTarget = false
  let tray = false
  let browserSession = false
  let hidden = false
  let stopListening: () => void = () => {}
  let clock: number | undefined
  // the Android back button's page half (the polish round, see
  // lib/shellback.ts): while any modal is open, BACK closes the topmost
  // one instead of leaving the app.
  let unregisterBack: (() => void) | null = null
  // An error raised by this window itself (for example Go refused the
  // call). It survives the next status update from Go so the person has
  // time to read it, and clears after 6 seconds — the one pre-existing
  // local error signal (spec §8: implement failed from whatever error
  // signal already exists).
  let localError = ''
  let errorTimer: number | undefined
  function showError(message: string) {
    localError = message
    window.clearTimeout(errorTimer)
    errorTimer = window.setTimeout(() => (localError = ''), 6000)
  }

  function onVisibility() {
    hidden = document.hidden
  }

  onMount(() => {
    backend.getSnapshot().then((s) => (snap = s)).catch(() => {})
    backend
      .getAppInfo()
      .then((info) => {
        appVersion = info?.version ?? ''
        platform = info?.platform ?? ''
        perAppRouting = info?.capabilities?.perAppRouting ?? false
        autoOpenTarget = info?.capabilities?.autoOpenTarget ?? false
        tray = info?.capabilities?.tray ?? false
        browserSession = info?.capabilities?.browserSession ?? false
      })
      .catch(() => {})
    stopListening = backend.subscribe((s) => (snap = s))
    clock = window.setInterval(() => (now = Date.now()), 250)
    document.addEventListener('visibilitychange', onVisibility)
  })
  onDestroy(() => {
    stopListening()
    window.clearInterval(clock)
    window.clearTimeout(errorTimer)
    document.removeEventListener('visibilitychange', onVisibility)
    if (unregisterBack !== null) unregisterBack()
  })

  // ---- the state (engine events only, never UI timers)
  $: warming = snap.phase === 'connected' && snap.warming === true && autoOpenTarget
  $: state = v3State(snap, localError, warming)
  $: slow = state === 'connected' && snap.degraded === true

  // a new live status from Go replaces any older local error (unchanged)
  $: if (snap.phase === 'connecting' || snap.phase === 'connected') localError = ''

  // ---- per-state copy (the reference T table, exact)
  $: headline = V3_COPY[state].headline
  $: hint = V3_COPY[state].hint
  $: lastSessionLine =
    state === 'idle' && snap.lastSession
      ? `Last session: ${formatDurationWords(snap.lastSession.endMs - snap.lastSession.startMs)}, ${formatBytes(snap.lastSession.downBytes + snap.lastSession.upBytes)}`
      : ''
  // only a session that really ran (>1 minute) gets the line
  $: lastSessionShown =
    !!snap.lastSession && snap.lastSession.endMs - snap.lastSession.startMs > 60_000
  $: stateSub =
    state === 'connecting'
      ? warming
        ? WARMING_DETAIL
        : V3_COPY.connecting.sub
      : state === 'failed'
        ? failedSubLine(snap.message)
        : state === 'connected'
          ? slow
            ? 'Slow connection, tuning...'
            : V3_COPY.connected.sub
          : ''
  // the sub-line: a local error wins; the last-session line only fills
  // the idle state — connecting/connected/failed always speak their own
  // line (the reference T table), never the session stats
  $: subLine =
    localError !== '' ? localError : state === 'idle' ? (lastSessionShown ? lastSessionLine : '') : stateSub
  // the amber words are never alone: the icon always sits beside them
  $: subAmber = localError === '' && stateSub !== '' && (state === 'failed' || (state === 'connected' && slow))

  // ---- the connected state's live figures
  $: showTimer = state === 'connected'
  $: showStats = state === 'connected' && snap.downBytes >= 0 && snap.upBytes >= 0
  $: elapsed = snap.connectedAtMs ? Math.max(0, now - snap.connectedAtMs) : 0

  // ---- haptics on the engine-confirmed transitions (UI-SPEC §3 item 8):
  // 12 ms when a connect or disconnect completes, error entering failed.
  let prevState: V3State = 'idle'
  $: if (prevState !== state) {
    if (state === 'failed') haptics.error()
    else if (state === 'connected' && prevState === 'connecting') haptics.confirm()
    else if (state === 'idle' && prevState !== 'idle') haptics.confirm()
    prevState = state
  }

  // ---- the back-button registration rides the modal states: the
  // diagnostics (rendered above the sheet) closes first, then the quit
  // dialog (rendered above everything), then the sheet. BACK always
  // unwinds one modal layer, never the app — the quit round's one
  // addition to the stack.
  $: {
    const modalUp = menuOpen || diagOpen || quitOpen
    if (modalUp && unregisterBack === null) {
      unregisterBack = pushBackHandler(() => {
        if (diagOpen) diagOpen = false
        else if (quitOpen) quitOpen = false
        else if (menuOpen) menuOpen = false
      })
    } else if (!modalUp && unregisterBack !== null) {
      unregisterBack()
      unregisterBack = null
    }
  }

  // ---- the lever's triggers
  async function doConnect() {
    localError = ''
    try {
      await backend.connect()
    } catch {
      showError('Could not start. Try again.')
    }
  }
  async function doDisconnect() {
    localError = ''
    try {
      await backend.disconnect()
    } catch {
      showError('Could not stop. Try again.')
    }
  }
  function onTrigger(e: CustomEvent<{ action: 'connect' | 'retry' | 'disconnect' }>) {
    switch (e.detail.action) {
      case 'connect':
      case 'retry':
        void doConnect()
        break
      case 'disconnect':
        void doDisconnect()
        break
    }
  }
</script>

<main
  class="yb-screen"
  data-state={state}
  data-health={slow ? 'slow' : null}
  data-hidden={hidden ? 'true' : null}
>
  <!-- the state backgrounds: stacked layers, cross-fade 700 ms (no hard cut) -->
  <div class="bg idle" aria-hidden="true"></div>
  <div class="bg connected" aria-hidden="true"></div>
  <div class="bg failed" aria-hidden="true"></div>
  <!-- the texture: fine halftone dots fading in toward the bottom + 5% grain -->
  <div class="tex" aria-hidden="true"></div>
  <div class="grain" aria-hidden="true"></div>

  <header class="yb-header">
    <div class="brand">
      {#if LOGO_URL}
        <img src={LOGO_URL} alt="" />
      {:else}
        <!-- the logo mark: red gradient tile with the white play triangle -->
        <span class="mark" aria-hidden="true"></span>
      {/if}
      <span>{APP_NAME}</span>
    </div>
    <button
      type="button"
      class="menu"
      bind:this={menuButton}
      aria-label="Open menu"
      on:click={() => (menuOpen = true)}
    >
      <span class="dots" aria-hidden="true">•••</span>
    </button>
  </header>

  <!-- the status block, centered in the free space above the button -->
  <section class="status-area">
    <div class="status">
      <!-- the ghost play mark (fades away while connected) -->
      <div class="ghost" aria-hidden="true"></div>
      <div class="label">{STATUS_LABEL}</div>
      <div class="live" aria-live="polite">
        <h1 class="headline">{headline}</h1>
        {#if subLine !== ''}
          <p class="sub" class:amber={subAmber}>
            {#if subAmber}<AlertTriangle class="sub-icon" aria-hidden="true" />{/if}
            {subLine}
          </p>
        {/if}
      </div>
      {#if showTimer}
        <div class="yb-timer" aria-label={formatDuration(elapsed)}>
          <Timer value={formatDuration(elapsed)} />
        </div>
      {/if}
      {#if showStats}
        <div class="stats">
          <span><ArrowDown class="arr" aria-hidden="true" />{formatBytes(snap.downBytes)}</span>
          <span><ArrowUp class="arr" aria-hidden="true" />{formatBytes(snap.upBytes)}</span>
        </div>
      {/if}
    </div>
  </section>

  <!-- the spacer that pushes the hint + card slot to the bottom -->
  <div class="below-spacer" aria-hidden="true"></div>

  <p class="hint">{hint}</p>

  <!-- the reserved card slot (spec §3.6): blank when not connected so
       nothing jumps between states -->
  <div class="card-slot">
    {#if state === 'connected'}
      <OpenYouTubeCard />
    {/if}
  </div>

  <!-- the fixed button zone: the key-cap lever, identical in every state -->
  <div class="yb-button-zone" aria-hidden="false">
    <LeverButton {state} on:trigger={onTrigger} />
  </div>

  <MenuSheet
    open={menuOpen}
    version={appVersion}
    {platform}
    {perAppRouting}
    {autoOpenTarget}
    {tray}
    {browserSession}
    {backend}
    on:close={() => (menuOpen = false)}
    on:diagnostics={() => {
      menuOpen = false
      diagOpen = true
    }}
    on:quit={() => {
      // the sheet's quiet row opens the confirmation; the sheet itself
      // steps out from under the dialog (the diagnostics pattern)
      menuOpen = false
      quitOpen = true
    }}
  />

  {#if diagOpen}
    <DiagnosticsView
      {backend}
      on:close={() => {
        diagOpen = false
        menuButton?.focus({ preventScroll: true })
      }}
    />
  {/if}

  {#if quitOpen && platform === 'android'}
    <QuitDialog
      on:cancel={() => {
        quitOpen = false
        menuButton?.focus({ preventScroll: true })
      }}
      on:quit={() => {
        // The shell owns the end-of-app sequence now; the page dies with
        // the app moments later. Closing the dialog locally is cosmetic —
        // the app is already leaving.
        quitOpen = false
      }}
    />
  {/if}
</main>

<style>
  /* the screen: the state backgrounds are the stacked .bg layers (the
     tokens file scopes every value); the body pre-paints --yb-base so
     there is never a flash of the wrong color on app start. The safe
     insets ride the column paddings. */
  .yb-screen {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding-top: max(env(safe-area-inset-top), 0px);
    padding-bottom: max(env(safe-area-inset-bottom), 0px);
    --yb-safe-top: max(env(safe-area-inset-top), 0px);
    --yb-header-h: calc(var(--yb-safe-top) + 12px + var(--yb-menu-btn));
  }

  /* ---- the state backgrounds: stacked layers, cross-fade 700 ms ---- */
  .bg {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: -3;
    opacity: 0;
    transition: opacity var(--yb-t-bg) ease;
  }
  .bg.idle {
    background: linear-gradient(180deg, #3b3a4a 0%, #2d2d37 35%, #26262e 70%, #26262e 100%);
  }
  .bg.connected {
    background: linear-gradient(180deg, #54756a 0%, #46665b 38%, #2d443c 78%, #253a33 100%);
  }
  .bg.failed {
    background: linear-gradient(180deg, #7b5a3f 0%, #664a33 38%, #3a2b20 78%, #2e2219 100%);
  }
  .yb-screen[data-state='idle'] .bg.idle,
  .yb-screen[data-state='connecting'] .bg.idle,
  .yb-screen[data-state='connected'] .bg.connected,
  .yb-screen[data-state='failed'] .bg.failed {
    opacity: 1;
  }

  /* ---- the texture: fine halftone dots fading in toward the bottom ---- */
  .tex {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: -2;
    pointer-events: none;
    background-image: radial-gradient(rgba(255, 255, 255, 0.075) 1px, transparent 1.5px);
    background-size: 7px 7px;
    -webkit-mask-image: linear-gradient(180deg, transparent 35%, #000 100%);
    mask-image: linear-gradient(180deg, transparent 35%, #000 100%);
  }

  /* ---- and the 5% grain overlay ---- */
  .grain {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: -1;
    pointer-events: none;
    opacity: 0.05;
    mix-blend-mode: overlay;
    background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/></filter><rect width='160' height='160' filter='url(%23n)'/></svg>");
  }

  /* ---- the header row, anchored to the top, 12px below the safe area ---- */
  .yb-header {
    flex: none;
    margin: 12px var(--yb-page-margin) 0;
    height: var(--yb-menu-btn);
    display: flex;
    align-items: center;
    justify-content: space-between;
    position: relative;
    z-index: 1;
    --wails-draggable: drag; /* frameless Wails windows drag by this bar */
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    font-family: var(--font);
    font-size: 17px;
    font-weight: 600;
    color: var(--yb-text);
  }
  /* the logo mark: red gradient tile with the white play triangle */
  .brand .mark {
    width: 34px;
    height: 24px;
    flex: none;
    background: linear-gradient(180deg, #f0392c, #c3180f);
    border-radius: 7px;
    position: relative;
    box-shadow:
      0 0 14px rgba(255, 40, 30, 0.55),
      inset 0 1px 0 rgba(255, 255, 255, 0.35);
  }
  .brand .mark:after {
    content: '';
    position: absolute;
    left: 13px;
    top: 7px;
    border-left: 9px solid #fff;
    border-top: 5px solid transparent;
    border-bottom: 5px solid transparent;
  }
  .brand img {
    height: 24px;
    width: auto;
  }
  /* the "..." button: a FILLED glass circle with a top highlight (not an
     outlined ring) — --yb-menu-btn keeps the 48px minimum touch size */
  .menu {
    --wails-draggable: no-drag;
    flex: none;
    width: var(--yb-menu-btn);
    height: var(--yb-menu-btn);
    display: grid;
    place-items: center;
    padding: 0;
    border-radius: 50%;
    border: 0;
    color: #fff;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.2), rgba(255, 255, 255, 0.08));
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.28),
      0 6px 14px rgba(0, 0, 0, 0.35);
  }
  .menu .dots {
    font-family: var(--font);
    font-size: 20px;
    line-height: 1;
    letter-spacing: 1px;
  }
  .menu:active {
    transform: scale(0.94);
  }

  /* ---- the status block, centered in the free space above the button ---- */
  .status-area {
    flex: none;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: calc(
      var(--yb-button-center-y) - (var(--track-h) + 2 * var(--yb-touch-padding)) / 2 - var(--yb-header-h) - 12px
    );
  }
  .status {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 0 var(--yb-page-margin);
    position: relative;
    z-index: 1;
  }
  /* the ghost play mark: a low glass tile that fades away while connected
     (the compact margins are the 640 design's; the tall-screen lane below
     restores the reference's airier 12px) */
  .ghost {
    width: 92px;
    height: 66px;
    flex: none;
    margin: 0 auto 6px;
    border-radius: 20px;
    background: rgba(255, 255, 255, 0.07);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.1);
    position: relative;
    transition: opacity 0.4s, height 0.4s, margin 0.4s;
  }
  .ghost:after {
    content: '';
    position: absolute;
    left: 37px;
    top: 19px;
    border-left: 24px solid rgba(255, 255, 255, 0.12);
    border-top: 14px solid transparent;
    border-bottom: 14px solid transparent;
  }
  .yb-screen[data-state='failed'] .ghost:after {
    border-left-color: rgba(255, 190, 90, 0.3);
  }
  .yb-screen[data-state='connected'] .ghost {
    opacity: 0;
    height: 0;
    margin: 0;
  }
  /* the small line above the headline */
  .label {
    font-family: var(--font);
    font-size: var(--yb-fs-eyebrow);
    font-weight: 500;
    letter-spacing: 0.3px;
    color: var(--c-sub);
    opacity: 0.85;
    transition: color var(--yb-t-color) ease;
  }
  .live {
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .headline {
    margin-top: 4px;
    font-family: var(--font);
    font-size: var(--yb-fs-headline);
    font-weight: 700;
    line-height: 1.1;
    white-space: nowrap;
    color: var(--yb-text);
    text-shadow: 0 2px 12px rgba(0, 0, 0, 0.25);
  }
  .sub {
    margin-top: 6px;
    min-height: 20px;
    display: flex;
    align-items: center;
    gap: 6px;
    justify-content: center;
    font-family: var(--font);
    font-size: var(--yb-fs-sub);
    font-weight: 500;
    color: var(--c-sub); /* per-state: dim, light green, or readable amber */
    transition: color var(--yb-t-color) ease;
  }
  .sub :global(.sub-icon) {
    width: 18px;
    height: 18px;
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  /* the connected state's timer + stats (the Timer component carries its
     values itself: 16/400 — half the headline, regular weight — with
     tabular boxes; the compact margins are the 640 design's) */
  .yb-timer {
    margin-top: 6px;
  }
  .stats {
    margin-top: 6px;
    display: flex;
    justify-content: center;
    gap: 18px;
    font-family: var(--font);
    font-size: var(--yb-fs-stats);
    font-weight: 500;
    color: var(--c-sub);
    font-variant-numeric: tabular-nums;
    transition: color var(--yb-t-color) ease;
  }
  .stats span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }
  .stats :global(svg) {
    width: 14px;
    height: 14px;
    stroke-width: 2;
  }

  /* ---- the hint and the reserved card slot, pushed to the bottom ---- */
  .below-spacer {
    flex: 1 1 0;
    min-height: 8px;
  }
  .hint {
    flex: none;
    margin: 0 var(--yb-page-margin);
    text-align: center;
    font-family: var(--font);
    font-size: var(--yb-fs-hint);
    font-weight: 500;
    letter-spacing: 0.01em;
    color: var(--yb-hint); /* raised to #d9d8e2 (10:1 on the idle bg) */
    opacity: 0.9;
    position: relative;
    z-index: 1;
  }
  .card-slot {
    flex: none;
    position: relative; /* above the lever where they meet on short screens */
    z-index: 1;
    display: flex;
    margin: 10px var(--yb-page-margin) var(--yb-page-margin);
    height: var(--yb-card-h); /* always reserved; blank when not connected */
  }

  /* ---- the fixed button zone: the key-cap lever, identical in every state ---- */
  .yb-button-zone {
    position: absolute;
    left: 50%;
    top: var(--yb-button-center-y);
    width: calc(var(--track-w) + 2 * var(--yb-touch-padding));
    height: calc(var(--track-h) + 2 * var(--yb-touch-padding));
    transform: translate(-50%, -50%);
    z-index: 0;
  }

  /* tall screens (phone-class viewports 700px and up — the Android
     WebView among them): the reference 780 rhythm — the airier spacing
     the v7 screen was drawn against. The 640 design window and every
     height between keep the compact base above; the button itself
     never changes size, only where the extra air sits around it. */
  @media (min-height: 700px) {
    .ghost {
      margin-bottom: 12px;
    }
    .sub {
      margin-top: 8px;
      min-height: 23px;
    }
    .yb-timer {
      margin-top: 14px;
    }
    .stats {
      margin-top: 8px;
    }
    .below-spacer {
      min-height: 12px;
    }
    .card-slot {
      margin-top: 12px;
    }
  }

  /* performance: page hidden or screen off — every animation pauses
     (position and words still carry the state) */
  .yb-screen[data-hidden='true'] :global(*),
  .yb-screen[data-hidden='true'] {
    animation-play-state: paused !important;
  }
</style>
