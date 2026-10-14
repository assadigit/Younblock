<script lang="ts">
  // Settings (file 02, section 4.3): a start-aligned list with 1 px
  // dividers, a collapsed Advanced section, and instant saves.
  import Icon from "../components/Icon.svelte";
  import { copy, fill } from "../i18n";
  import type { Settings } from "../lib/backend";

  interface Props {
    settings: Settings;
    startWithWindowsSupported: boolean;
    onSave: (settings: Settings) => void;
    onBack: () => void;
    onToast: (text: string) => void;
  }
  let { settings, startWithWindowsSupported, onSave, onBack, onToast }: Props = $props();

  let advancedOpen = $state(false);

  // Local working copies; every change saves immediately.
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

<section class="settings" aria-label={copy.settings.title}>
  <header class="view-header">
    <button type="button" class="back" onclick={onBack} aria-label={copy.a11y.back}>
      <Icon name="chevron-left" size={20} />
    </button>
    <h1>{copy.settings.title}</h1>
  </header>

  <div class="rows">
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

    <button type="button" class="row advanced-head" onclick={() => (advancedOpen = !advancedOpen)} aria-expanded={advancedOpen}>
      <div class="row-text">
        <p class="row-title">{copy.settings.advanced}</p>
        <p class="row-hint">{copy.settings.advancedHint}</p>
      </div>
      <Icon name="chevron-right" size={18} />
    </button>

    {#if advancedOpen}
      <div class="advanced">
        <div class="group">
          <p class="group-title">{copy.settings.mode}</p>
          {#each modes as mode (mode.value)}
            <label class="choice">
              <input
                type="radio"
                name="mode"
                value={mode.value}
                checked={local.mode === mode.value}
                onchange={() => save({ mode: mode.value as Settings["mode"] })}
              />
              <span class="choice-text">
                <span class="choice-label">{mode.label}</span>
                <span class="choice-hint">{mode.hint}</span>
                {#if mode.value === "manual" && local.mode === "manual"}
                  <span class="manual-address">
                    <code>127.0.0.1:{local.port}</code>
                    <button type="button" class="copy" onclick={copyAddress}>
                      <Icon name="copy" size={14} />
                    </button>
                  </span>
                {/if}
              </span>
            </label>
          {/each}
        </div>

        <div class="group">
          <p class="group-title">{copy.settings.method}</p>
          {#each methods as method (method.value)}
            <label class="choice">
              <input
                type="radio"
                name="method"
                value={method.value}
                checked={local.method === method.value}
                onchange={() => save({ method: method.value })}
              />
              <span class="choice-text">
                <span class="choice-label">{method.label}</span>
                <span class="choice-hint">{method.hint}</span>
              </span>
            </label>
          {/each}
        </div>

        <label class="field">
          <span class="group-title">{copy.settings.port}</span>
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

        <div class="group">
          <p class="group-title">{copy.settings.dohServers}</p>
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

        <div class="group">
          <p class="group-title">{copy.settings.domains}</p>
          <ul class="plain-list">
            {#each local.advanced.domains as domain (domain)}
              <li>{domain}</li>
            {/each}
          </ul>
        </div>

        <div class="row">
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

        <button type="button" class="reset" onclick={reset}>{copy.settings.reset}</button>
      </div>
    {/if}
  </div>
</section>

<style>
  .settings {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .view-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    padding-top: max(var(--space-2), env(safe-area-inset-top));
  }
  .view-header h1 {
    margin: 0;
    font-size: var(--text-md);
    line-height: 24px;
    font-weight: 500;
  }
  .back {
    width: 44px;
    height: 44px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    margin-inline-start: calc(-1 * var(--space-2));
  }
  .back:hover {
    color: var(--color-text);
    background: var(--color-surface);
  }
  .rows {
    flex: 1;
    overflow-y: auto;
    max-height: 100%;
    padding: 0 var(--space-4);
    padding-bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
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
    margin: 2px 0 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .advanced-head {
    width: 100%;
    text-align: start;
  }
  .advanced-head:hover {
    background: transparent;
  }
  .advanced {
    padding: var(--space-2) 0 var(--space-4);
    border-bottom: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .group-title {
    margin: 0 0 var(--space-1);
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
  }
  .choice {
    display: flex;
    gap: var(--space-3);
    align-items: flex-start;
    padding: var(--space-2) 0;
    cursor: pointer;
    min-height: 44px;
  }
  .choice input {
    width: 18px;
    height: 18px;
    margin-top: 1px;
    accent-color: var(--red-500);
    flex: none;
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
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    font-size: var(--text-xs);
  }
  .copy {
    width: 32px;
    height: 32px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
  }
  .copy:hover {
    color: var(--color-text);
    background: var(--color-surface);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .field input {
    width: 120px;
    min-height: 44px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text);
    padding: var(--space-2) var(--space-3);
    font-variant-numeric: tabular-nums;
  }
  .plain-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .plain-list li {
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
    display: flex;
    flex-direction: column;
  }
  .server-name {
    color: var(--color-text);
  }
  .group-hint {
    margin: 0 0 var(--space-2);
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 16px;
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
  .server-first {
    margin-left: var(--space-2);
    color: var(--color-text-muted);
    font-size: 10px;
    line-height: 14px;
    vertical-align: middle;
  }
  .server-remove {
    flex: none;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
  }
  .server-remove:hover {
    background: var(--color-surface);
    color: var(--color-text);
  }
  .doh-add {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
  .chip {
    min-height: 36px;
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-full);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    font-size: var(--text-xs);
  }
  .chip:hover {
    background: var(--color-surface);
  }
  .reset {
    align-self: flex-start;
    min-height: 44px;
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    font-size: var(--text-sm);
  }
  .reset:hover {
    background: var(--color-surface);
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
    transition: background 280ms ease-out;
  }
  .thumb {
    width: 20px;
    height: 20px;
    border-radius: var(--radius-full);
    background: var(--neutral-50);
    transition: transform 280ms ease-out;
  }
  .switch input:checked + .track {
    background: var(--red-500);
  }
  .switch input:checked + .track .thumb {
    transform: translateX(18px);
  }
  .switch input:focus-visible + .track {
    outline: 3px solid var(--color-focus);
    outline-offset: 3px;
  }
</style>
