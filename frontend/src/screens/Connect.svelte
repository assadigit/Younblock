<script lang="ts">
  // The home screen (file 02, section 4.1).
  import PowerButton from "../components/PowerButton.svelte";
  import StatusBlock from "../components/StatusBlock.svelte";
  import Icon from "../components/Icon.svelte";
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
  <Icon name="globe" size={14} />
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
    gap: var(--space-2);
    padding-bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
</style>
