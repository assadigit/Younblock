<script lang="ts">
  // Settings (file 02, section 4.3): a start-aligned list with 1 px
  // dividers, a collapsed Advanced section, and instant saves.
  // v1.5.0 (D56/D57): the screen mounts on the shared view chrome, the
  // Advanced groups sit on cards, the radios are drawn in the brand
  // style (the native accent-colored dots looked unfinished beside the
  // custom switch), every control keeps a 44 px target, and the section
  // reveals with the app's expand motion instead of snapping.
  import Icon from "../components/Icon.svelte";
  import { copy, fill } from "../i18n";
  import type { Settings } from "../lib/backend";

  interface Props {
    settings: Settings;
    startWithWindowsSupported: boolean;
    browserSessionSupported?: boolean;
    /** D93: the keep-in-tray row renders only where a tray exists (the
     *  Windows desktop) — every other host has nothing to keep running. */
    traySupported?: boolean;
    onSave: (settings: Settings) => void;
    onBack: () => void;
    onToast: (text: string) => void;
  }
  let {
    settings,
    startWithWindowsSupported,
    browserSessionSupported = true,
    traySupported = false,
    onSave,
    onBack,
    onToast,
  }: Props = $props();

  let advancedOpen = $state(false);

  // Local working copies; every change saves immediately.
  // (Capturing the incoming settings once is the point — a working copy.)
  // svelte-ignore state_referenced_locally
  let local = $state<Settings>(JSON.parse(JSON.stringify(settings)));

  const methods = [
    { value: "auto", label: copy.settings.methodAuto, hint: copy.settings.methodAutoHint },
    { value: "light", label: copy.settings.methodLight, hint: copy.settings.methodLightHint },
    { value: "balanced", label: copy.settings.methodBalanced, hint: copy.settings.methodBalancedHint },
    { value: "strong", label: copy.settings.methodStrong, hint: copy.settings.methodStrongHint },
    { value: "legacy", label: copy.settings.methodLegacy, hint: copy.settings.methodLegacyHint },
  ];
  const modes = [
    { value: "youtube", label: copy.settings.modeYouTube, hint: copy.settings.modeYouTubeHint },
    { value: "all", label: copy.settings.modeAll, hint: copy.settings.modeAllHint },
    { value: "manual", label: copy.settings.modeManual, hint: copy.settings.modeManualHint },
  ];

  // Known secure-DNS providers (URL template + bootstrap IPs). The three
  // defaults are here too so a removed one can be added back (D44: when the
  // current providers are blocked, switching must not require editing a
  // JSON file by hand).
  const dohPresets = [
    { name: "RethinkDNS", url: "https://sky.rethinkdns.com/?dns={dns}", bootstrap: ["172.67.162.27"] },
    { name: "Cloudflare", url: "https://cloudflare-dns.com/dns-query?dns={dns}", bootstrap: ["1.1.1.1", "1.0.0.1"] },
    { name: "Google", url: "https://dns.google/dns-query?dns={dns}", bootstrap: ["8.8.8.8", "8.8.4.4"] },
    { name: "AdGuard", url: "https://dns.adguard.com/dns-query?dns={dns}", bootstrap: ["94.140.14.14", "94.140.15.15"] },
    { name: "Quad9", url: "https://dns.quad9.net/dns-query?dns={dns}", bootstrap: ["9.9.9.9", "149.112.112.112"] },
    { name: "dns.sb", url: "https://doh.dns.sb/dns-query?dns={dns}", bootstrap: ["185.222.222.222", "45.11.45.11"] },
  ];

  function addDohServer(p: { name: string; url: string; bootstrap: string[] }) {
    if (local.advanced.dohServers.some((s) => s.url === p.url)) return;
    // New providers go to the top: the engine starts from the first server,
    // so the one the user just added is tried first (D44).
    saveAdvanced({
      dohServers: [{ ...p, bootstrap: [...p.bootstrap] }, ...local.advanced.dohServers],
    });
    onToast(fill(copy.settings.dohAdded, { name: p.name }));
  }

  function removeDohServer(url: string) {
    if (local.advanced.dohServers.length <= 1) {
      onToast(copy.settings.dohNeedOne);
      return;
    }
    saveAdvanced({ dohServers: local.advanced.dohServers.filter((s) => s.url !== url) });
  }

  function save(patch: Partial<Settings>) {
    local = { ...local, ...patch };
    onSave(local);
  }

  function saveAdvanced(patch: Partial<Settings["advanced"]>) {
    local = { ...local, advanced: { ...local.advanced, ...patch } };
    onSave(local);
  }

  async function copyAddress() {
    try {
      await navigator.clipboard.writeText(`127.0.0.1:${local.port}`);
      onToast(copy.toast.copied);
    } catch {
      onToast(copy.toast.copied);
    }
  }

  function reset() {
    // Defaults come back from the backend's normalizer.
    save({
      mode: "youtube",
      method: "auto",
      port: 4525,
      autoPort: true,
      openBrowserOnConnect: false,
      advanced: {
        ...local.advanced,
        pieceSizeMin: 0,
        pieceSizeMax: 0,
        pauseMs: 0,
        logDomains: false,
        idleTimeoutSec: 0,
      },
    });
  }
</script>

<section class="settings view" aria-label={copy.settings.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1 class="view-title">{copy.settings.title}</h1>
  </header>

  <div class="view-scroll rows">
    {#if startWithWindowsSupported}
      <div class="row">
        <div class="row-text">
          <p class="row-title">{copy.settings.startWithWindows}</p>
        </div>
        <label class="switch">
          <input
            type="checkbox"
            checked={local.startWithWindows}
            onchange={(e) => save({ startWithWindows: e.currentTarget.checked })}
          />
          <span class="track" aria-hidden="true"><span class="thumb"></span></span>
        </label>
      </div>
    {/if}
    <div class="row">
      <div class="row-text">
        <p class="row-title">{copy.settings.connectOnLaunch}</p>
      </div>
      <label class="switch">
        <input
          type="checkbox"
          checked={local.connectOnLaunch}
          onchange={(e) => save({ connectOnLaunch: e.currentTarget.checked })}
        />
        <span class="track" aria-hidden="true"><span class="thumb"></span></span>
      </label>
    </div>

    {#if traySupported}
      <div class="row">
        <div class="row-text">
          <p class="row-title">{copy.settings.keepInTray}</p>
          <p class="row-hint">{copy.settings.keepInTrayHint}</p>
        </div>
        <label class="switch">
          <!-- D93: off is the shipped default — the close button is a real
               shutdown; only an explicit opt-in keeps the app in the tray. -->
          <input
            type="checkbox"
            role="switch"
            aria-label={copy.settings.keepInTray}
            checked={local.keepInTrayOnClose}
            onchange={(e) => save({ keepInTrayOnClose: e.currentTarget.checked })}
          />
          <span class="track" aria-hidden="true"><span class="thumb"></span></span>
        </label>
      </div>
    {/if}

    <div class="row">
      <div class="row-text">
        <p class="row-title">{copy.settings.routeAll}</p>
        <p class="row-hint">{copy.settings.routeAllHint}</p>
      </div>
      <label class="switch">
        <!-- Maps the existing mode: on = "all", off = "youtube" (or manual,
             which stays set in Advanced below). -->
        <input
          type="checkbox"
          role="switch"
          aria-label={copy.settings.routeAll}
          checked={local.mode === "all"}
          onchange={(e) => save({ mode: e.currentTarget.checked ? "all" : "youtube" })}
        />
        <span class="track" aria-hidden="true"><span class="thumb"></span></span>
      </label>
    </div>

    {#if browserSessionSupported}
      <div class="row">
        <div class="row-text">
          <p class="row-title">{copy.settings.openBrowserOnConnect}</p>
          <p class="row-hint">{copy.settings.openBrowserOnConnectHint}</p>
        </div>
        <label class="switch">
          <input
            type="checkbox"
            checked={local.openBrowserOnConnect}
            onchange={(e) => save({ openBrowserOnConnect: e.currentTarget.checked })}
          />
          <span class="track" aria-hidden="true"><span class="thumb"></span></span>
        </label>
      </div>
    {/if}

    <button type="button" class="row advanced-head" onclick={() => (advancedOpen = !advancedOpen)} aria-expanded={advancedOpen}>
      <div class="row-text">
        <p class="row-title">{copy.settings.advanced}</p>
        <p class="row-hint">{copy.settings.advancedHint}</p>
      </div>
      <span class="chevron" class:open={advancedOpen} aria-hidden="true">
        <Icon name="chevron-right" size={18} />
      </span>
    </button>

    <div class="reveal" class:open={advancedOpen}>
      <div class="reveal-body">
        <div class="advanced">
          <div class="card">
            <p class="overline">{copy.settings.mode}</p>
            {#each modes as mode (mode.value)}
              <label class="choice">
                <input
                  type="radio"
                  name="mode"
                  value={mode.value}
                  checked={local.mode === mode.value}
                  onchange={() => save({ mode: mode.value as Settings["mode"] })}
                />
                <span class="radio" aria-hidden="true"></span>
                <span class="choice-text">
                  <span class="choice-label">{mode.label}</span>
                  <span class="choice-hint">{mode.hint}</span>
                  {#if mode.value === "manual" && local.mode === "manual"}
                    <span class="manual-address">
                      <code>127.0.0.1:{local.port}</code>
                      <button type="button" class="copy" onclick={copyAddress} aria-label={copy.settings.manualAddress}>
                        <Icon name="copy" size={15} />
                      </button>
                    </span>
                  {/if}
                </span>
              </label>
            {/each}
          </div>

          <div class="card">
            <p class="overline">{copy.settings.method}</p>
            {#each methods as method (method.value)}
              <label class="choice">
                <input
                  type="radio"
                  name="method"
                  value={method.value}
                  checked={local.method === method.value}
                  onchange={() => save({ method: method.value })}
                />
                <span class="radio" aria-hidden="true"></span>
                <span class="choice-text">
                  <span class="choice-label">{method.label}</span>
                  <span class="choice-hint">{method.hint}</span>
                </span>
              </label>
            {/each}
          </div>

          <div class="card">
            <label class="field">
              <span class="overline">{copy.settings.port}</span>
              <input
                type="number"
                min="1"
                max="65535"
                value={local.port}
                onchange={(e) => {
                  const port = Number(e.currentTarget.value);
                  if (Number.isFinite(port) && port >= 1 && port <= 65535) save({ port });
                }}
              />
            </label>
          </div>

          <div class="card">
            <p class="overline">{copy.settings.dohServers}</p>
            <p class="group-hint">{copy.settings.dohHint}</p>
            <ul class="plain-list server-list">
              {#each local.advanced.dohServers as server, i (server.url)}
                <li>
                  <div class="server-row">
                    <div class="server-text">
                      <span class="server-name">
                        {server.name}
                        {#if i === 0}<span class="server-first">{copy.settings.dohFirst}</span>{/if}
                      </span>
                      <span class="server-url">{server.url}</span>
                    </div>
                    <button
                      type="button"
                      class="server-remove"
                      aria-label={fill(copy.settings.dohRemove, { name: server.name })}
                      title={fill(copy.settings.dohRemove, { name: server.name })}
                      onclick={() => removeDohServer(server.url)}
                    >
                      <Icon name="x" size={16} />
                    </button>
                  </div>
                </li>
              {/each}
            </ul>
            <div class="doh-add" role="group" aria-label={copy.settings.dohAddGroup}>
              {#each dohPresets as p (p.url)}
                {#if !local.advanced.dohServers.some((s) => s.url === p.url)}
                  <button type="button" class="chip" onclick={() => addDohServer(p)}>
                    + {p.name}
                  </button>
                {/if}
              {/each}
            </div>
          </div>

          <div class="card">
            <p class="overline">{copy.settings.domains}</p>
            <ul class="plain-list domain-list">
              {#each local.advanced.domains as domain (domain)}
                <li>{domain}</li>
              {/each}
            </ul>
            <div class="row in-card">
              <div class="row-text">
                <p class="row-title">{copy.settings.logDomains}</p>
              </div>
              <label class="switch">
                <input
                  type="checkbox"
                  checked={local.advanced.logDomains}
                  onchange={(e) => saveAdvanced({ logDomains: e.currentTarget.checked })}
                />
                <span class="track" aria-hidden="true"><span class="thumb"></span></span>
              </label>
            </div>
          </div>

          <button type="button" class="reset" onclick={reset}>{copy.settings.reset}</button>
        </div>
      </div>
    </div>
  </div>
</section>

<style>
  .rows {
    gap: 0;
    padding-top: var(--space-2);
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
    min-height: 44px;
  }
  .row-title {
    margin: 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .row-hint {
    margin: 4px 0 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 17px;
  }
  .advanced-head {
    width: 100%;
    text-align: start;
    border-bottom: 1px solid var(--color-border);
  }
  .advanced-head:hover {
    background: transparent;
  }
  .advanced-head:hover .row-title,
  .advanced-head:hover .chevron {
    color: var(--color-text);
  }
  /* The chevron now answers like every expander in the app (D57): it
     rotates to point down while the section below unfolds. */
  .chevron {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    color: var(--color-text-muted);
    transition:
      transform var(--dur-med) var(--ease-spring-out),
      color var(--dur-fast) ease-out;
  }
  .chevron.open {
    transform: rotate(90deg);
  }
  .advanced {
    padding: var(--space-4) 0 var(--space-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .in-card {
    /* The log-domains row keeps the list rhythm inside its card. */
    border-bottom: none;
    border-top: 1px solid var(--color-border);
    margin-top: var(--space-2);
    padding-top: var(--space-3);
  }
  .choice {
    position: relative;
    display: flex;
    gap: var(--space-3);
    align-items: flex-start;
    padding: var(--space-2) 0;
    cursor: pointer;
    min-height: 44px;
  }
  .choice input {
    position: absolute;
    inset: 0;
    opacity: 0;
    margin: 0;
    cursor: pointer;
  }
  /* The brand radio (D57): a ring that fills with the padlock red.
     Drawn to match the switch's weight and motion. */
  .radio {
    flex: none;
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    margin-top: 1px;
    border-radius: var(--radius-full);
    border: 2px solid var(--neutral-600);
    background: var(--neutral-900);
    transition:
      border-color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out;
  }
  .radio::after {
    content: "";
    width: 10px;
    height: 10px;
    border-radius: var(--radius-full);
    background: var(--red-500);
    transform: scale(0);
    transition: transform var(--dur-med) var(--ease-spring-out);
  }
  .choice:hover .radio {
    border-color: var(--neutral-400);
  }
  .choice input:checked ~ .radio {
    border-color: var(--red-500);
    background: var(--red-900);
  }
  .choice input:checked ~ .radio::after {
    transform: scale(1);
  }
  .choice input:focus-visible ~ .radio {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  .choice-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .choice-label {
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .choice-hint {
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .manual-address {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }
  .manual-address code {
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 3px var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    letter-spacing: 0;
  }
  .copy {
    width: 44px;
    height: 44px;
    display: grid;
    place-items: center;
    margin: calc(-1 * var(--space-2)) calc(-1 * var(--space-2)) calc(-1 * var(--space-2)) 0;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    transition:
      color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out;
  }
  .copy:hover {
    color: var(--color-text);
    background: var(--color-surface-2);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .field input {
    width: 140px;
    min-height: 44px;
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text);
    padding: var(--space-2) var(--space-3);
    font-variant-numeric: tabular-nums;
    transition: border-color var(--dur-fast) ease-out;
  }
  .field input:hover {
    border-color: var(--color-border-strong);
  }
  .field input:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 1px;
    border-color: var(--color-focus);
  }
  .plain-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .domain-list li {
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 18px;
  }
  .server-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 44px;
  }
  .server-text {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }
  .server-name {
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .server-url {
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-2xs);
    line-height: 16px;
    overflow-wrap: anywhere;
  }
  /* "tried first" — a tiny gold badge, the lightning's color, so the
     live ordering of the list is readable at a glance (D57). */
  .server-first {
    display: inline-block;
    margin-left: var(--space-2);
    padding: 1px 6px;
    border: 1px solid rgba(255, 197, 61, 0.35);
    border-radius: var(--radius-full);
    color: var(--gold-300);
    font-size: 10px;
    line-height: 13px;
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    vertical-align: 1px;
  }
  .group-hint {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .server-remove {
    flex: none;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    transition:
      color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out;
  }
  .server-remove:hover {
    background: var(--color-surface-2);
    color: var(--color-text);
  }
  .doh-add {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }
  .chip {
    min-height: 44px;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-full);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    font-size: var(--text-xs);
    transition:
      background var(--dur-fast) ease-out,
      border-color var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .chip:hover {
    background: var(--color-surface-2);
    border-color: rgba(255, 197, 61, 0.4);
  }
  .chip:active {
    transform: scale(0.96);
  }
  .reset {
    align-self: flex-start;
    min-height: 44px;
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    transition:
      color var(--dur-fast) ease-out,
      background var(--dur-fast) ease-out,
      border-color var(--dur-fast) ease-out;
  }
  .reset:hover {
    color: var(--color-text);
    background: var(--color-surface);
    border-color: var(--color-border-strong);
  }

  /* The switch (WCAG-touch-friendly toggle). */
  .switch {
    position: relative;
    display: inline-grid;
    flex: none;
  }
  .switch input {
    position: absolute;
    inset: 0;
    opacity: 0;
    width: 44px;
    height: 26px;
    cursor: pointer;
    margin: 0;
  }
  .track {
    width: 44px;
    height: 26px;
    border-radius: var(--radius-full);
    background: var(--neutral-700);
    display: grid;
    align-items: center;
    padding: 3px;
    transition: background var(--dur-med) ease-out;
  }
  .switch:hover .track {
    background: var(--neutral-600);
  }
  .thumb {
    width: 20px;
    height: 20px;
    border-radius: var(--radius-full);
    background: var(--neutral-50);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
    transition: transform var(--dur-med) var(--ease-spring-out);
  }
  .switch input:checked + .track {
    background: var(--red-500);
  }
  .switch input:checked:hover + .track {
    background: var(--red-400);
  }
  .switch input:checked + .track .thumb {
    transform: translateX(18px);
  }
  .switch input:focus-visible + .track {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }

  @media (prefers-reduced-motion: reduce) {
    .chevron,
    .radio,
    .radio::after,
    .track,
    .thumb,
    .chip,
    .copy,
    .server-remove,
    .reset {
      transition: none;
    }
    .chip:active {
      transform: none;
    }
  }
</style>
