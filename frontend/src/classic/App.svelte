<script lang="ts">
  // App.svelte — the shell: header (logo + menu), the current view, the
  // one-time close notice, toasts, and all backend wiring.
  import { onMount } from "svelte";
  import Menu from "./components/Menu.svelte";
  import Toast from "./components/Toast.svelte";
  import Wordmark from "./components/Wordmark.svelte";
  import About from "./screens/About.svelte";
  import Connect from "./screens/Connect.svelte";
  import FirstRun from "./screens/FirstRun.svelte";
  import Help from "./screens/Help.svelte";
  import SettingsScreen from "./screens/Settings.svelte";
  import { copy, setOsWord } from "./i18n";
  import { mockBackend } from "./lib/adapters/mock";
  import { wailsAvailable, WailsBackend } from "./lib/adapters/wails";
  import { httpAvailable, HttpBackend } from "./lib/adapters/http";
  import type { Backend, BackendState, Settings, Stats, TestStep } from "./lib/backend";

  type View = "connect" | "settings" | "help" | "about";

  let backend: Backend = mockBackend;
  let ready = $state(false);
  let view: View = $state("connect");
  let uiState = $state<BackendState>(backend.state);
  let stats = $state<Stats | null>(null);
  let settings = $state<Settings | null>(null);
  let version = $state("1.0.0");
  let startWithOS = $state(false);
  let hasTray = $state(true);
  let browserSession = $state(true);
  let steps = $state<TestStep[]>([]);
  let testing = $state(false);
  let busy = $state(false);
  let toasts = $state<{ id: number; text: string }[]>([]);
  let closeNotice = $state(false);
  let toastSeq = 1;

  const noticeText = $derived.by(() => {
    if (!uiState.notice) return null;
    const known: Record<string, string> = {
      METHOD_SWITCHED: copy.notice.METHOD_SWITCHED,
      RECOVERED: copy.notice.RECOVERED,
      PROXY_REAPPLIED: copy.notice.PROXY_REAPPLIED,
    };
    return known[uiState.notice.code] ?? null;
  });

  const errorText = $derived.by(() => {
    if (!uiState.error) return null;
    const known: Record<string, string> = {
      PORT_BUSY: copy.errors.PORT_BUSY,
      DNS_UNREACHABLE: copy.errors.DNS_UNREACHABLE,
      DNS_BLOCKED_ANSWER: copy.errors.DNS_BLOCKED_ANSWER,
      PROXY_SETTING_FAILED: copy.errors.PROXY_SETTING_FAILED,
      SELFTEST_FAILED: copy.errors.SELFTEST_FAILED,
      UNKNOWN: copy.errors.UNKNOWN,
    };
    return known[uiState.error.code] ?? copy.errors.UNKNOWN;
  });

  function toast(text: string) {
    const id = toastSeq++;
    toasts = [...toasts, { id, text }];
    window.setTimeout(() => {
      toasts = toasts.filter((t) => t.id !== id);
    }, 3000);
  }

  onMount(async () => {
    // Pick the adapter at startup (file 02, section 9): wails when the
    // bindings exist; the http adapter when this page is served by the
    // younblock web server (the Linux single-executable edition, D50);
    // mock in a plain browser (?mock=…); android later.
    if (wailsAvailable()) {
      const wailsBackend = new WailsBackend();
      await wailsBackend.prime();
      backend = wailsBackend;
    } else if (await httpAvailable()) {
      const httpBackend = new HttpBackend();
      await httpBackend.prime();
      backend = httpBackend;
    }
    backend.onState((s) => (uiState = s));
    backend.onStats((s) => (stats = s));
    backend.onTestProgress((step) => {
      steps = [...steps.filter((s) => s.id !== step.id), step];
    });
    backend.onCloseRequested(() => {
      if (settings && !settings.closeNoticeShown) {
        closeNotice = true;
      } else {
        backend.hideWindow();
      }
    });
    // R13: the shell announces every automatic Chrome-session action.
    backend.onBrowserEvent((kind) => {
      if (kind === "opened") toast(copy.toast.browserOpened);
      else if (kind === "closed") toast(copy.toast.browserClosed);
      else toast(copy.toast.browserFailed);
    });
    settings = backend.settings;
    uiState = backend.state;
    version = backend.appInfo.version;
    startWithOS = backend.appInfo.capabilities.startWithOS;
    hasTray = backend.appInfo.capabilities.tray;
    browserSession = backend.appInfo.capabilities.browserSession;
    // D50: on the Linux edition the Windows-flavored copy deck gets its
    // OS word swapped before the first render (one deck, every version).
    if (backend.appInfo.platform === "linux") {
      setOsWord("Linux");
    }
    ready = true;
  });

  async function toggle() {
    if (busy) return;
    busy = true;
    try {
      await backend.toggle();
    } finally {
      window.setTimeout(() => (busy = false), 300);
    }
  }

  async function fixConnection() {
    if (busy) return;
    busy = true;
    try {
      await backend.autoFix();
    } finally {
      window.setTimeout(() => (busy = false), 300);
    }
  }

  async function runTest() {
    if (testing) return;
    testing = true;
    steps = [];
    try {
      await backend.runConnectionTest();
    } finally {
      testing = false;
    }
  }

  async function saveSettings(next: Settings) {
    const result = await backend.saveSettings(next);
    if (result && "version" in result) {
      settings = result as Settings;
    }
  }

  async function getStarted(autostart: boolean) {
    if (!settings) return;
    await saveSettings({ ...settings, firstRunDone: true, startWithWindows: autostart });
    view = "connect";
  }

  function acknowledgeCloseNotice() {
    closeNotice = false;
    if (settings) {
      saveSettings({ ...settings, closeNoticeShown: true });
    }
    backend.hideWindow();
  }

  async function copyReport() {
    const report = await backend.getDiagnosticReport();
    try {
      await navigator.clipboard.writeText(report.report);
    } catch {
      // Clipboard may be unavailable; still show the toast for the attempt.
    }
    toast(copy.toast.copied);
  }

  async function restoreSettings() {
    await backend.restoreWindowsProxy();
    toast(copy.toast.restored);
  }

  async function openLogs() {
    await backend.openLogsFolder();
  }

  function quit() {
    backend.quit();
  }

  function navigate(next: "settings" | "help" | "about") {
    view = next;
  }
</script>

{#if !ready}
  <div class="boot" aria-hidden="true"></div>
{:else}
  {#if view === "connect" && settings && !settings.firstRunDone}
    <FirstRun startWithWindowsSupported={startWithOS} hasTray={hasTray} onGetStarted={getStarted} />
  {:else if view === "settings" && settings}
    <SettingsScreen {settings} startWithWindowsSupported={startWithOS} browserSessionSupported={browserSession} traySupported={hasTray} onSave={saveSettings} onBack={() => (view = "connect")} onToast={toast} />
  {:else if view === "help"}
    <Help {steps} {testing} hasTray={hasTray} onBack={() => (view = "connect")} onRunTest={runTest} onFixConnection={fixConnection} onCopyReport={copyReport} onOpenLogs={openLogs} onRestore={restoreSettings} />
  {:else if view === "about"}
    <About {version} onBack={() => (view = "connect")} />
  {:else}
    <header class="app-header enter">
      <Wordmark />
      <Menu onNavigate={navigate} onQuit={quit} />
    </header>
    <main class="app-main">
      <Connect state={uiState} {stats} {errorText} {noticeText} {busy} onToggle={toggle} onFixConnection={fixConnection} onOpenHelp={() => (view = "help")} />
    </main>
  {/if}
{/if}

{#if closeNotice}
  <div class="overlay" role="dialog" aria-modal="true" aria-label={copy.closeNotice.text}>
    <div class="sheet">
      <p>{copy.closeNotice.text}</p>
      <button type="button" class="primary" onclick={acknowledgeCloseNotice}>
        {copy.buttons.ok}
      </button>
    </div>
  </div>
{/if}

<Toast {toasts} />

<style>
  .boot {
    position: fixed;
    inset: 0;
    background: var(--color-bg);
  }
  .app-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 44px;
    padding: 0 var(--space-3) 0 var(--space-4);
    padding-top: env(safe-area-inset-top);
    flex: none;
  }
  /* D48: the shell is the height budget owner — #app is exactly the
     viewport height (base.css), the header and footer are flex:none,
     and the main area absorbs the rest. Nothing can push the footer
     out of the window any more, and the never-scrolling body keeps it
     glued to the bottom of the flex column. */
  .app-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
  }
  /* D58: the close notice arrives — backdrop fades in, the sheet rises
     the same way every panel in the app does. */
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: grid;
    place-items: center;
    z-index: 100;
    animation: overlay-in var(--dur-med) ease-out backwards;
  }
  .sheet {
    width: 300px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: var(--space-5) var(--space-4) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    text-align: center;
    box-shadow: var(--shadow-pop);
    animation: sheet-in var(--dur-slow) var(--ease-spring-out) backwards;
  }
  .sheet p {
    margin: 0;
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 20px;
  }
  .sheet .primary {
    min-height: 44px;
    border-radius: var(--radius-md);
    background: var(--red-500);
    color: var(--color-on-primary);
    font-weight: 500;
    transition:
      background var(--dur-fast) ease-out,
      transform var(--dur-fast) var(--ease-press-in);
  }
  .sheet .primary:hover {
    background: var(--red-400);
  }
  .sheet .primary:active {
    transform: scale(0.97);
  }
  @keyframes overlay-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  @keyframes sheet-in {
    from {
      opacity: 0;
      transform: translateY(10px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .overlay {
      animation-name: overlay-in;
    }
    .sheet {
      animation-name: overlay-in;
    }
    .sheet .primary {
      transition: none;
    }
    .sheet .primary:active {
      transform: none;
    }
  }
</style>
