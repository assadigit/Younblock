<script lang="ts">
  // The home screen (file 02, section 4.1).
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

<section class="connect" aria-label={copy.a11y.powerButton}>
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

<footer class="footer">
  <span>{copy.app.footer}</span>
</footer>

<style>
  .connect {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-8);
    padding: var(--space-6) var(--space-6) var(--space-4);
    padding-top: max(var(--space-6), env(safe-area-inset-top));
  }
  .button-wrap {
    display: grid;
    place-items: center;
  }
  .footer {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
    padding: 5px 12px;
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: var(--radius-full);
    color: var(--neutral-400);
    font-size: 11px;
    line-height: 16px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-weight: 500;
  }

</style>
