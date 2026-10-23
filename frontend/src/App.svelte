<!-- App.svelte — the Younblock UI v3 main screen (docs/ui-v3/UI-SPEC.md).
     The ONE screen every edition loads (the Android WebView, the Windows
     apps, the Linux window and web editions — the variant switch is
     retired): one flex column — header, status block
     centered in the free space above the button, the fixed button zone at
     --yb-button-center-y, hint, the reserved card slot, the safe insets.
     State is carried by data-state/data-health on this root; every value
     comes from docs/ui-v3/younblock-tokens.css (imported by pro.ts). -->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import DiagnosticsView from './lib/DiagnosticsView.svelte'
  import MenuSheet from './lib/MenuSheet.svelte'
  import RingV3 from './lib/RingV3.svelte'
  import LeverButton from './lib/LeverButton.svelte'
  import OpenYouTubeCard from './lib/OpenYouTubeCard.svelte'
  import Timer from './lib/Timer.svelte'
  import { createBackend } from './lib/backend'
  import { formatBytes, formatDuration, formatDurationWords } from './lib/format'
  import { AlertTriangle, ArrowDown, ArrowUp, Ellipsis } from './lib/icons'
  import * as haptics from './lib/haptics'
  import { failedSubLine, V3_COPY, v3State, type V3State } from './lib/v3'
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
  // the diagnostics dialog over the stage (unchanged from the v2 wiring;
  // the menu sheet's contents are out of scope per spec §1)
  let diagOpen = false
  let menuButton: HTMLElement | undefined
  let appVersion = ''
  let platform = ''
  let perAppRouting = false
  let autoOpenTarget = false
  let tray = false
  let hidden = false
  let stopListening: () => void = () => {}
  let clock: number | undefined
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
  })

  // ---- the v3 state (spec §5/§8): engine events only, never UI timers
  $: warming = snap.phase === 'connected' && snap.warming === true && autoOpenTarget
  $: state = v3State(snap, localError, warming)
  $: slow = state === 'connected' && snap.degraded === true

  // a new live status from Go replaces any older local error (unchanged)
  $: if (snap.phase === 'connecting' || snap.phase === 'connected') localError = ''

  // ---- per-state copy (spec §5, exact)
  $: headline = V3_COPY[state].headline
  $: hint = V3_COPY[state].hint
  $: lastSessionLine =
    state === 'disconnected' && snap.lastSession
      ? `Last session: ${formatDurationWords(snap.lastSession.endMs - snap.lastSession.startMs)}, ${formatBytes(snap.lastSession.downBytes + snap.lastSession.upBytes)}`
      : ''
  // only a session that really ran (>1 minute) gets the line (spec §5)
  $: lastSessionShown =
    !!snap.lastSession && snap.lastSession.endMs - snap.lastSession.startMs > 60_000
  $: stateSub =
    state === 'connecting'
      ? warming
        ? WARMING_DETAIL
        : 'Opening a route to YouTube...'
      : state === 'failed'
        ? failedSubLine(snap.message)
        : state === 'connected'
          ? slow
            ? 'Slow connection, tuning...'
            : 'YouTube unblocked.'
          : ''
  $: subLine = localError !== '' ? localError : lastSessionShown ? lastSessionLine : stateSub
  // amber is never alone: the icon always sits beside the words (spec §5)
  $: subAmber = localError === '' && stateSub !== '' && (state === 'failed' || (state === 'connected' && slow))

  // ---- the connected state's live figures
  $: showTimer = state === 'connected'
  $: showStats = state === 'connected' && snap.downBytes >= 0 && snap.upBytes >= 0
  $: elapsed = snap.connectedAtMs ? Math.max(0, now - snap.connectedAtMs) : 0

  // ---- haptics on the engine-confirmed transitions (spec §4): confirm
  // when a connect or disconnect completes, error entering failed.
  let prevState: V3State = 'disconnected'
  $: if (prevState !== state) {
    if (state === 'failed') haptics.error()
    else if (state === 'connected' && prevState === 'connecting') haptics.confirm()
    else if (state === 'disconnected' && prevState !== 'disconnected') haptics.confirm()
    prevState = state
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
  function onTrigger(e: CustomEvent<{ action: 'connect' | 'cancel' | 'retry' | 'disconnect' }>) {
    switch (e.detail.action) {
      case 'connect':
      case 'retry':
        void doConnect()
        break
      case 'cancel':
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
  <header class="yb-header">
    <div class="brand">
      {#if LOGO_URL}
        <img src={LOGO_URL} alt="" />
      {:else}
        <!-- the existing logo, recolored (spec §1: keep it, recolor it) -->
        <svg class="logo" viewBox="0 0 28 20" aria-hidden="true" focusable="false">
          <rect width="28" height="20" rx="6" />
          <path d="M11 5.5l8 4.5-8 4.5z" />
        </svg>
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
      <Ellipsis class="menu-icon" />
    </button>
  </header>

  <!-- the status block, centered in the free space above the button -->
  <section class="status-area">
    <div class="status">
      {#if state !== 'connected'}
        <!-- the ghost play mark (existing style, very low opacity) -->
        <svg class="ghost" viewBox="0 0 120 84" aria-hidden="true" focusable="false">
          <path
            fill-rule="evenodd"
            d="M26 2h68a24 24 0 0 1 24 24v32a24 24 0 0 1-24 24H26A24 24 0 0 1 2 58V26A24 24 0 0 1 26 2zM48 24v36l32-18z"
          />
        </svg>
      {/if}
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

  <!-- the fixed button zone: ring + track + cap, identical in every state -->
  <div class="yb-button-zone" aria-hidden="false">
    <RingV3 {state} />
    <LeverButton {state} on:trigger={onTrigger} />
  </div>

  <MenuSheet
    open={menuOpen}
    version={appVersion}
    {platform}
    {perAppRouting}
    {autoOpenTarget}
    {tray}
    {backend}
    on:close={() => (menuOpen = false)}
    on:diagnostics={() => {
      menuOpen = false
      diagOpen = true
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
</main>

<style>
  /* the screen: the tokens' .yb-screen paints the constant background;
     this adds the v3 column (spec §3). The safe insets are items 1 and 7. */
  .yb-screen {
    position: fixed;
    inset: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding-top: max(env(safe-area-inset-top), 0px);
    padding-bottom: max(env(safe-area-inset-bottom), 0px);
    --yb-safe-top: max(env(safe-area-inset-top), 0px);
    --yb-header-h: calc(var(--yb-safe-top) + 12px + var(--yb-menu-btn));
  }

  /* ---- 2. the header row, anchored to the top, 12px below the safe area */
  .yb-header {
    flex: none;
    margin: 12px var(--yb-page-margin) 0;
    height: var(--yb-menu-btn);
    display: flex;
    align-items: center;
    justify-content: space-between;
    --wails-draggable: drag; /* frameless Wails windows drag by this bar */
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: var(--font);
    font-size: 15px;
    font-weight: 600;
    letter-spacing: 0.01em;
    color: var(--yb-text);
  }
  .brand .logo {
    width: 28px;
    height: 20px;
    fill: var(--yb-red);
  }
  .brand .logo path {
    fill: #fff;
  }
  .brand img {
    height: 24px;
    width: auto;
  }
  /* the "..." button: 48x48 minimum with a visible outline (spec §3.2).
     1.5px --yb-text at 45%: 3.38:1 non-text contrast on the background
     (checked in docs/ui-v3/REPORT.md; raise the mix if it ever drops). */
  .menu {
    --wails-draggable: no-drag;
    flex: none;
    width: var(--yb-menu-btn);
    height: var(--yb-menu-btn);
    display: grid;
    place-items: center;
    padding: 0;
    border-radius: 50%;
    border: 1.5px solid color-mix(in srgb, var(--yb-text) 45%, transparent);
    background: transparent;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background var(--yb-t-color) ease;
  }
  .menu:hover {
    background: color-mix(in srgb, var(--yb-text) 8%, transparent);
  }
  .menu:active {
    transform: scale(0.94);
  }
  .menu :global(.menu-icon) {
    width: 22px;
    height: 22px;
    color: var(--yb-text);
    stroke-width: 2.4;
  }

  /* ---- 3. the status block, centered in the free space above the button:
     the area ends exactly where the button zone begins. */
  .status-area {
    flex: none;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: calc(
      var(--yb-button-center-y) - var(--yb-ring-size) / 2 - var(--yb-header-h) - 12px
    );
  }
  .status {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 0 var(--yb-page-margin);
  }
  .ghost {
    width: 120px;
    height: auto;
    margin-bottom: 10px;
    fill: var(--yb-text);
    opacity: 0.05;
    transition: opacity var(--yb-t-color) ease;
  }
  .label {
    font-family: var(--font);
    font-size: var(--yb-fs-stats);
    font-weight: 500;
    letter-spacing: 0.05em;
    color: var(--yb-text-dim);
  }
  .live {
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .headline {
    margin-top: 6px;
    font-family: var(--font);
    font-size: var(--yb-fs-headline);
    font-weight: 700;
    line-height: 1.25;
    color: var(--yb-text);
  }
  .sub {
    margin-top: 8px;
    min-height: 22px;
    display: flex;
    align-items: center;
    gap: 6px;
    justify-content: center;
    font-family: var(--font);
    font-size: var(--yb-fs-sub);
    font-weight: 500;
    color: var(--state-sub); /* per-state: dim, or amber with the icon */
    transition: color var(--yb-t-color) ease;
  }
  .sub.amber {
    color: var(--yb-amber);
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

  /* the connected state's timer + stats (spec §5) */
  .yb-timer {
    margin-top: 10px;
  }
  .yb-timer :global(.timer) {
    font-size: var(--yb-fs-timer); /* smaller than the headline (28 < 32) */
    font-weight: 700;
    color: var(--yb-text);
  }
  .stats {
    margin-top: 10px;
    display: flex;
    justify-content: center;
    gap: 24px;
    font-family: var(--font);
    font-size: var(--yb-fs-stats);
    font-weight: 500;
    color: var(--yb-text-dim);
    font-variant-numeric: tabular-nums;
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

  /* ---- 5/6. the hint and the reserved card slot, pushed to the bottom */
  .below-spacer {
    flex: 1 1 0;
    min-height: 12px;
  }
  .hint {
    flex: none;
    margin: 0 var(--yb-page-margin);
    text-align: center;
    font-family: var(--font);
    font-size: var(--yb-fs-hint);
    font-weight: 500;
    letter-spacing: 0.01em;
    color: var(--yb-text-dim);
  }
  .card-slot {
    flex: none;
    position: relative; /* above the ring where they meet on short screens */
    z-index: 1;
    display: flex;
    margin: 12px var(--yb-page-margin) var(--yb-page-margin);
    height: var(--yb-card-h); /* always reserved; blank when not connected */
  }

  /* ---- 4. the fixed button zone: identical in every state (spec §3.4) */
  .yb-button-zone {
    position: absolute;
    left: 50%;
    top: var(--yb-button-center-y);
    width: var(--yb-ring-size);
    height: var(--yb-ring-size);
    transform: translate(-50%, -50%);
    z-index: 0;
  }

  /* short phones (spec §3): the track already shrank via clamp(); shrink
     the status spacing first, never the button */
  @media (max-height: 700px) {
    .ghost {
      width: 96px;
      margin-bottom: 6px;
    }
    .headline {
      margin-top: 4px;
    }
    .sub {
      margin-top: 6px;
      min-height: 20px;
    }
    .yb-timer {
      margin-top: 6px;
    }
    .stats {
      margin-top: 6px;
    }
    .below-spacer {
      min-height: 8px;
    }
    .card-slot {
      margin-top: 10px;
    }
  }

  /* performance (spec §9): page hidden or screen off — every animation
     pauses (position and words still carry the state) */
  .yb-screen[data-hidden='true'] :global(*),
  .yb-screen[data-hidden='true'] {
    animation-play-state: paused !important;
  }
</style>
