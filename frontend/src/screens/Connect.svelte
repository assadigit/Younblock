<script lang="ts">
  // The home screen (file 02, section 4.1).
  import Icon from "../components/Icon.svelte";
  import PowerButton from "../components/PowerButton.svelte";
  import StatusBlock from "../components/StatusBlock.svelte";
  import { copy } from "../i18n";
  import type { BackendState, Stats } from "../lib/backend";

  interface Props {
    state: BackendState;
    stats: Stats | null;
    errorText: string | null;
    noticeText: string | null;
    busy: boolean;
    onToggle: () => void;
    onFixConnection: () => void;
    onOpenHelp: () => void;
  }
  let { state, stats, errorText, noticeText, busy, onToggle, onFixConnection, onOpenHelp }: Props =
    $props();
</script>

<section
  class="connect view"
  data-phase={state.phase}
  aria-label={copy.a11y.powerButton}
>
  <div class="button-wrap">
    <PowerButton phase={state.phase} {busy} onToggle={onToggle} label={copy.a11y.powerButton} />
  </div>
  <StatusBlock
    {state}
    {stats}
    {errorText}
    {noticeText}
    onOpenHelp={onOpenHelp}
    onFixConnection={onFixConnection}
  />
</section>

<footer class="footer enter">
  <Icon name="globe" size={12} strokeWidth={2} />
  <span>{copy.app.footer}</span>
</footer>

<style>
  /* D48 (Ali's report): the old fixed budget overflowed the 360x560
     window whenever the status stack grew (degraded +44 fix pill, error
     +reason stack) — the footer was clipped off-screen and the
     "Fix connection" pill landed on the window's bottom edge. The
     layout now has a hard height budget with a scroll safety net and a
     responsive power-button ladder, so it stays composed at 560 css px
     and degrades gracefully when Windows DPI scaling shrinks the
     viewport below that. */
  .connect {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    /* Between tall status stacks and short ones the extra room goes
       above and below the whole group (never inside it, where it read
       as a dead gap between the button and the timer). safe center so
       an over-tall stack can never be cut at both ends. */
    justify-content: safe center;
    gap: var(--space-6);
    padding: var(--space-2) var(--space-6);
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: none;
  }
  .connect::-webkit-scrollbar {
    display: none;
  }
  .button-wrap {
    display: grid;
    place-items: center;
    flex: none;
  }
  /* The footer always sits inside the window with real margins on both
     sides (D48): it is part of the flex column, so it can never be
     clipped by an overflowing status stack again. */
  .footer {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    margin: 0 auto var(--space-4);
    padding: 5px 12px;
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: var(--radius-full);
    color: var(--neutral-500);
    font-size: var(--text-2xs);
    line-height: 16px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-weight: 500;
  }
  .footer :global(svg) {
    display: block;
    opacity: 0.8;
  }

  /* The responsive ladder (D48): the power button shrinks a step when
     the viewport gets tight, so the degraded/error stacks keep their
     room without pushing the footer out. Degraded needs ~546 css px of
     viewport; below that the button scales to 0.88. Below ~470 css px
     (150% Windows scaling on the fixed window) the whole screen zooms
     down 0.82 — which restores the intended physical size on the
     high-DPI display, because a css px there is already 1.5 device px.
     The internal scroll stays as the safety net below that. */
  @media (max-height: 545px) {
    .button-wrap {
      transform: scale(0.88);
      margin: calc(-208px * 0.06) 0;
    }
  }
  @media (max-height: 469px) {
    .connect {
      zoom: 0.82;
    }
  }
  /* The tall warning states (degraded: metrics + fix pill; error:
     reason + help link) shrink the button one step so their stacks keep
     comfortable room above the footer — the same treatment for both,
     so the two "needs attention" states read as one family (D48, made
     comfortable in v1.5.0: degraded used to land exactly on the 15 px
     minimum, which read as cramped next to every other state). */
  .connect[data-phase="degraded"] .button-wrap,
  .connect[data-phase="error"] .button-wrap {
    transform: scale(0.88);
    margin: calc(-208px * 0.06) 0;
  }
</style>
