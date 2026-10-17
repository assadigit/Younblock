<!-- App.svelte — the Younblock connect screen. Place in src/ (replaces the template's App.svelte). -->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import DotRings from './lib/DotRings.svelte'
  import MenuSheet from './lib/MenuSheet.svelte'
  import PowerPill from './lib/PowerPill.svelte'
  import Timer from './lib/Timer.svelte'
  import { createBackend } from './lib/backend'
  import { formatBitrate, formatDuration } from './lib/format'
  import type { Phase, Snapshot } from './lib/types'

  // =====================================================================
  // CONFIGURATION — edit only this block
  // =====================================================================
  const APP_NAME = 'Younblock'
  /** Small line above the big status word. */
  const STATUS_LABEL = 'YouTube'
  /** Path or URL of your logo image (for example '/logo.png' placed in public/). Empty = built-in red play mark. */
  const LOGO_URL = ''
  const STATUS_TEXT: Record<Phase, string> = {
    disconnected: 'Not Connected',
    connecting: 'Connecting…',
    connected: 'Connected',
    disconnecting: 'Disconnecting…',
    error: "Can't Connect",
  }
  const HINT_OFF = 'Click or swipe down to connect'
  const HINT_ON = 'Click or swipe up to disconnect'
  // =====================================================================

  const backend = createBackend()

  let snap: Snapshot = { phase: 'disconnected', connectedAtMs: null, downBps: 0, upBps: 0, pingMs: null }
  let now = Date.now()
  let menuOpen = false
  // The host's version (backend.getAppInfo) for the About sheet — the one
  // copy of the number lives in internal/config on the Go side, so it can
  // never disagree with --version. Empty in a plain-browser preview, where
  // the sheet simply hides the line.
  let appVersion = ''
  // M6: the host's platform + capabilities gate the sheet's Android-only
  // surfaces (the apps row, the VPN-slot copy, the privacy disclosure, the
  // diagnostics view). Unset in a preview — the sheet then renders exactly
  // the desktop sections.
  let platform = ''
  let perAppRouting = false
  let stopListening: () => void = () => {}
  let clock: number | undefined
  // An error raised by this window itself (for example Go refused the call). It survives the
  // next status update from Go, so the person has time to read it, and clears after 6 seconds.
  let localError = ''
  let errorTimer: number | undefined
  function showError(message: string) {
    localError = message
    window.clearTimeout(errorTimer)
    errorTimer = window.setTimeout(() => (localError = ''), 6000)
  }

  onMount(() => {
    backend.getSnapshot().then((s) => (snap = s)).catch(() => {})
    backend
      .getAppInfo()
      .then((info) => {
        appVersion = info?.version ?? ''
        platform = info?.platform ?? ''
        perAppRouting = info?.capabilities?.perAppRouting ?? false
      })
      .catch(() => {})
    stopListening = backend.subscribe((s) => (snap = s))
    clock = window.setInterval(() => (now = Date.now()), 250)
  })
  onDestroy(() => {
    stopListening()
    window.clearInterval(clock)
    window.clearTimeout(errorTimer)
  })

  async function connect() {
    localError = ''
    try {
      await backend.connect()
    } catch {
      showError('Could not start. Try again.')
    }
  }
  async function disconnect() {
    localError = ''
    try {
      await backend.disconnect()
    } catch {
      showError('Could not stop. Try again.')
    }
  }

  // a new live status from Go (connecting or connected) replaces any older local error
  $: if (snap.phase === 'connecting' || snap.phase === 'connected') localError = ''
  let phase: Phase = 'disconnected'
  $: phase = localError !== '' && snap.phase === 'disconnected' ? 'error' : snap.phase
  let tone: 'on' | 'off' = 'off'
  $: tone = phase === 'connected' || phase === 'disconnecting' ? 'on' : 'off'
  $: busy = phase === 'connecting' || phase === 'disconnecting'
  $: showTimer = phase === 'connected' || phase === 'disconnecting'
  $: elapsed = snap.connectedAtMs ? Math.max(0, now - snap.connectedAtMs) : 0
  $: detail =
    localError !== ''
      ? localError
      : snap.message ?? (phase === 'connected' && snap.pingMs !== null ? `Ping ${snap.pingMs} ms` : '')
  $: hint = busy ? '' : tone === 'on' ? HINT_ON : HINT_OFF
</script>

<main class="app" data-tone={tone} data-phase={phase}>
  <div class="bg off" aria-hidden="true"></div>
  <div class="bg on" aria-hidden="true"></div>

  <div class="stage">
    <div class="halftone" aria-hidden="true"></div>

    <!-- faint play mark behind the status text, fills the space the world map used to fill -->
    <svg class="mark" viewBox="0 0 120 84" aria-hidden="true" focusable="false">
      <path
        fill-rule="evenodd"
        d="M26 2h68a24 24 0 0 1 24 24v32a24 24 0 0 1-24 24H26A24 24 0 0 1 2 58V26A24 24 0 0 1 26 2zM48 24v36l32-18z"
      />
    </svg>

    <header>
      <button type="button" class="menu" aria-label="Open menu" on:click={() => (menuOpen = true)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle cx="12" cy="6" r="2.1" />
          <circle cx="6" cy="12" r="2.1" />
          <circle cx="18" cy="12" r="2.1" />
          <circle cx="12" cy="18" r="2.1" />
        </svg>
      </button>
      <div class="brand">
        {#if LOGO_URL}
          <img src={LOGO_URL} alt="" />
        {:else}
          <svg class="logo" viewBox="0 0 28 20" aria-hidden="true" focusable="false">
            <rect width="28" height="20" rx="6" />
            <path d="M11 5.5l8 4.5-8 4.5z" />
          </svg>
        {/if}
        <span>{APP_NAME}</span>
      </div>
      <span class="spacer"></span>
    </header>

    <div class="timer-block" class:visible={showTimer}>
      <Timer value={formatDuration(elapsed)} />
      <div class="speeds">
        <span><svg viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 4.5L6 8l3.5-3.5" /></svg>{formatBitrate(snap.downBps)}</span>
        <span><svg viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 7.5L6 4l3.5 3.5" /></svg>{formatBitrate(snap.upBps)}</span>
      </div>
    </div>

    <div class="status" aria-live="polite">
      <div class="label">{STATUS_LABEL}</div>
      <!-- keyed by phase: the word crossfades on every change instead of
           snapping (the CSS animation dies under prefers-reduced-motion) -->
      <div class="state">
        {#key phase}
          <span class="word">{STATUS_TEXT[phase]}</span>
        {/key}
      </div>
      <div class="detail" class:visible={detail !== ''} class:error={phase === 'error'}>{detail}</div>
    </div>

    <DotRings {tone} {busy} cy={tone === 'on' ? 30.5625 : 28.3} />
    <PowerPill {phase} on:connect={connect} on:disconnect={disconnect} />

    <div class="hint" class:visible={hint !== ''}>{hint}</div>

    <MenuSheet
      open={menuOpen}
      version={appVersion}
      {platform}
      {perAppRouting}
      {backend}
      on:close={() => (menuOpen = false)}
    />
  </div>
</main>

<style>
  .app {
    position: fixed;
    inset: 0;
    overflow: hidden;
    /* tone variables, switched by data-tone (DotRings carries its own
       per-set fills so the outgoing color survives the cross-fade) */
    --track-a: #1b1b24;
    --track-b: #2f3037;
  }
  .app[data-tone='on'] {
    --track-a: #243832;
    --track-b: #3a574e;
  }

  /* two full-bleed gradient layers that cross-fade (a gradient itself cannot be animated) */
  .bg {
    position: absolute;
    inset: 0;
    transition: opacity var(--dur-tone) ease;
  }
  .bg.off {
    background: linear-gradient(180deg, var(--off-top) 0%, var(--off-mid) 55%, var(--off-bottom) 100%);
  }
  .bg.on {
    background: linear-gradient(180deg, var(--on-top) 0%, var(--on-mid) 52%, var(--on-bottom) 100%);
    opacity: 0;
  }
  .app[data-tone='on'] .bg.on {
    opacity: 1;
  }

  .stage {
    position: absolute;
    left: 50%;
    top: 50%;
    width: var(--stage-w);
    height: var(--stage-h);
    transform: translate(-50%, -50%);
  }

  /* fine dot texture that gets denser toward the bottom, like the reference */
  .halftone {
    position: absolute;
    left: -40rem;
    right: -40rem;
    bottom: -20rem;
    height: 39rem;
    background-image: radial-gradient(circle, rgba(255, 255, 255, 0.2) 0.07rem, transparent 0.13rem);
    background-size: 0.4rem 0.4rem;
    -webkit-mask-image: linear-gradient(to top, #000 0%, transparent 78%);
    mask-image: linear-gradient(to top, #000 0%, transparent 78%);
    opacity: 0.55;
    pointer-events: none;
  }

  .mark {
    position: absolute;
    left: 50%;
    top: 5.625rem;
    width: 10.2rem;
    margin-left: -5.1rem;
    fill: #fff;
    opacity: 0.04;
    pointer-events: none;
    transition: opacity var(--dur-tone) ease;
  }
  .app[data-tone='on'] .mark {
    opacity: 0; /* the timer lives here while connected */
  }

  header {
    position: absolute;
    top: 2.375rem;
    left: 1.25rem;
    right: 1.25rem;
    height: var(--menu-btn);
    display: flex;
    align-items: center;
    justify-content: space-between;
    --wails-draggable: drag; /* lets you move a frameless Wails window by this bar */
  }
  .menu,
  .spacer {
    width: var(--menu-btn);
    height: var(--menu-btn);
  }
  .menu {
    --wails-draggable: no-drag;
    display: grid;
    place-items: center;
    padding: 0;
    border-radius: 50%;
    border: 1px solid var(--chip-border);
    background: var(--chip-bg);
    box-shadow:
      0 0.375rem 0.75rem rgba(0, 0, 0, 0.22),
      inset 0 0.0625rem 0 rgba(255, 255, 255, 0.12);
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition:
      background 150ms ease,
      transform 150ms ease;
  }
  .menu:hover {
    background: rgba(255, 255, 255, 0.16);
  }
  .menu:active {
    transform: scale(0.94);
  }
  .menu:focus-visible {
    outline: 0.125rem solid #fff;
    outline-offset: 0.2rem;
  }
  .menu svg {
    width: 1.25rem;
    height: 1.25rem;
    fill: #fff;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9375rem;
    font-weight: var(--fw-semibold);
    letter-spacing: 0.01em;
  }
  .brand .logo {
    width: 1.5rem;
    height: 1.0714rem;
    fill: var(--red-500);
    filter: drop-shadow(0 0 0.5rem var(--red-glow));
  }
  .brand .logo path {
    fill: #fff;
  }
  .brand img {
    height: 1.5rem;
    width: auto;
  }

  .timer-block {
    position: absolute;
    left: 0;
    right: 0;
    top: 7rem;
    opacity: 0;
    transform: translateY(-0.4rem);
    transition:
      opacity 400ms ease,
      transform 400ms var(--ease-out);
  }
  .timer-block.visible {
    opacity: 1;
    transform: none;
  }
  .speeds {
    display: flex;
    justify-content: center;
    gap: 1.1rem;
    margin-top: 0.85rem;
    font-size: var(--fs-stats);
    font-weight: var(--fw-medium);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .speeds span {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .speeds svg {
    width: 0.75rem;
    height: 0.75rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .status {
    position: absolute;
    left: 0;
    right: 0;
    top: 13.775rem;
    text-align: center;
  }
  .label {
    font-size: var(--fs-label);
    font-weight: var(--fw-medium);
    color: var(--text-secondary);
    letter-spacing: 0.02em;
  }
  .state {
    margin-top: 0.35rem;
    font-size: var(--fs-status);
    font-weight: var(--fw-bold);
    line-height: 1.9rem;
    text-shadow: 0 0.125rem 0.5rem rgba(0, 0, 0, 0.25);
  }
  .word {
    display: inline-block;
    animation: word-in var(--dur-word) var(--ease-out);
  }
  @keyframes word-in {
    from {
      opacity: 0;
      transform: translateY(0.3rem);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
  .detail {
    margin-top: 0.55rem;
    min-height: 1rem;
    font-size: var(--fs-stats);
    font-weight: var(--fw-medium);
    color: var(--text-secondary);
    opacity: 0;
    transition: opacity 300ms ease;
  }
  .detail.visible {
    opacity: 1;
  }
  /* the error line is the one place red is allowed as text: --red-400,
     the only red that passes contrast on these surfaces (tokens.css) */
  .detail.error {
    color: var(--red-400);
  }

  .hint {
    position: absolute;
    left: 0;
    right: 0;
    top: 36rem;
    text-align: center;
    font-size: var(--fs-hint);
    font-weight: var(--fw-medium);
    letter-spacing: 0.02em;
    word-spacing: 0.14em;
    color: var(--text-tertiary);
    opacity: 0;
    transition: opacity 300ms ease;
  }
  .hint.visible {
    opacity: 1;
  }
</style>
