// mock.ts — a browser backend controlled by the URL (?mock=off|connecting|
// on|degraded|error|firstrun) so the whole UI can be developed and
// screenshot-tested without the Go engine (file 02, section 9).
import type {
  ApiResult,
  AppInfo,
  AutoFixResult,
  Backend,
  BackendState,
  DiagnosticReport,
  LogsFolder,
  Settings,
  Stats,
  TestStep,
} from "../backend";

type MockMode = "off" | "connecting" | "on" | "degraded" | "error" | "firstrun" | "stopping" | "notice";

function modeFromUrl(): MockMode {
  const params = new URLSearchParams(window.location.search);
  const raw = params.get("mock") ?? "off";
  if (["off", "connecting", "on", "degraded", "error", "firstrun", "stopping", "notice"].includes(raw)) {
    return raw as MockMode;
  }
  return "off";
}

/** ?test=fail makes the mock connection test fail every step. */
function testFails(): boolean {
  return new URLSearchParams(window.location.search).get("test") === "fail";
}

function nowIso(): string {
  return new Date().toISOString();
}

const ok: ApiResult = { error: null };

export class MockBackend implements Backend {
  private mode: MockMode;
  private stateValue: BackendState;
  private settingsValue: Settings;
  private statsValue: Stats;
  private info: AppInfo;
  private stateListeners = new Set<(s: BackendState) => void>();
  private statsListeners = new Set<(s: Stats) => void>();
  private testListeners = new Set<(s: TestStep) => void>();
  private closeListeners = new Set<() => void>();
  private ticker: number | undefined;

  constructor() {
    this.mode = modeFromUrl();
    this.settingsValue = defaultSettings();
    if (this.mode === "firstrun") {
      this.settingsValue.firstRunDone = false;
      this.mode = "off";
    }
    const uptime = this.mode === "on" || this.mode === "notice" ? 42 * 60 + 18 : 0;
    this.stateValue = this.stateFor(this.mode);
    const onLike = this.mode === "on" || this.mode === "notice";
    this.statsValue = {
      bytesDown: onLike ? 12_600_000 : 0,
      bytesUp: onLike ? 420_000 : 0,
      rateDown: onLike ? 320_000 : 0,
      rateUp: onLike ? 21_000 : 0,
      activeConns: onLike ? 3 : 0,
      uptimeSec: uptime,
    };
    this.info = {
      name: "Younblock",
      version: "1.1.0",
      platform: "windows",
      capabilities: { tray: true, startWithOS: true, systemProxy: true, perAppRouting: false },
      configPath: "C:\\Users\\Ali\\AppData\\Roaming\\Younblock\\settings.json",
      logDir: "C:\\Users\\Ali\\AppData\\Roaming\\Younblock\\logs",
    };
    if (onLike || this.mode === "degraded") {
      this.startTicker();
    }
    if (this.mode === "stopping") {
      this.stateValue = { ...this.stateValue, phase: "stopping", detail: "" };
    }
    if (this.mode === "notice") {
      this.stateValue = {
        ...this.stateValue,
        phase: "on",
        notice: { code: "METHOD_SWITCHED", arg: "strong" },
      };
    }
    if (this.mode === "connecting") {
      // A scripted connect: starting -> checking -> on with a notice.
      window.setTimeout(() => this.setPhase("connecting", "checking"), 400);
      window.setTimeout(() => this.setPhase("on", ""), 900);
      window.setTimeout(() => {
        this.stateValue = { ...this.stateValue, notice: { code: "METHOD_SWITCHED", arg: "strong" } };
        this.emitState();
        this.startTicker();
      }, 1000);
    }
    if (this.mode === "degraded") {
      this.stateValue = {
        ...this.stateValue,
        error: { code: "SELFTEST_FAILED" },
      };
    }
  }

  private stateFor(mode: MockMode): BackendState {
    const base: BackendState = {
      phase: "off",
      detail: "",
      since: nowIso(),
      port: 4525,
      mode: "youtube",
      method: "auto",
      notice: null,
      error: null,
    };
    switch (mode) {
      case "on":
      case "notice":
        return { ...base, phase: "on", latencyMs: 84 };
      case "stopping":
        return { ...base, phase: "stopping" };
      case "degraded":
        return { ...base, phase: "degraded", latencyMs: 84, error: { code: "SELFTEST_FAILED" } };
      case "error":
        return { ...base, phase: "error", error: { code: "PORT_BUSY" } };
      case "connecting":
        return { ...base, phase: "connecting", detail: "starting" };
      default:
        return base;
    }
  }

  private setPhase(phase: BackendState["phase"], detail: string) {
    this.stateValue = { ...this.stateValue, phase, detail, since: nowIso() };
    if (phase === "on" || phase === "degraded") {
      this.startTicker();
    } else {
      this.stopTicker();
    }
    this.emitState();
  }

  private emitState() {
    for (const cb of this.stateListeners) cb(this.stateValue);
  }

  private startTicker() {
    if (this.ticker !== undefined) return;
    this.ticker = window.setInterval(() => {
      this.statsValue = {
        ...this.statsValue,
        bytesDown: this.statsValue.bytesDown + this.statsValue.rateDown,
        bytesUp: this.statsValue.bytesUp + this.statsValue.rateUp,
        uptimeSec: this.statsValue.uptimeSec + 1,
      };
      for (const cb of this.statsListeners) cb(this.statsValue);
    }, 1000);
  }

  private stopTicker() {
    if (this.ticker !== undefined) {
      window.clearInterval(this.ticker);
      this.ticker = undefined;
    }
  }

  get state(): BackendState {
    return this.stateValue;
  }

  get settings(): Settings {
    return this.settingsValue;
  }

  get appInfo(): AppInfo {
    return this.info;
  }

  async start(): Promise<ApiResult> {
    this.setPhase("connecting", "starting");
    window.setTimeout(() => this.setPhase("connecting", "checking"), 500);
    window.setTimeout(() => this.setPhase("on", ""), 1200);
    return ok;
  }

  async stop(): Promise<ApiResult> {
    this.setPhase("stopping", "");
    window.setTimeout(() => this.setPhase("off", ""), 600);
    return ok;
  }

  async toggle(): Promise<ApiResult> {
    if (this.stateValue.phase === "on" || this.stateValue.phase === "degraded") {
      return this.stop();
    }
    return this.start();
  }

  async saveSettings(settings: Settings): Promise<Settings | ApiResult> {
    this.settingsValue = { ...settings };
    this.stateValue = {
      ...this.stateValue,
      mode: settings.mode,
      method: settings.method,
      port: settings.port,
    };
    this.emitState();
    return this.settingsValue;
  }

  async runConnectionTest(): Promise<TestStep[]> {
    const fail = testFails();
    const steps: TestStep[] = [
      { id: "listener", ok: !fail, ms: 1, code: fail ? "LISTENER_DOWN" : "", detail: fail ? "The app is not listening." : "Listening on port 4525." },
      { id: "dns", ok: !fail, ms: fail ? 3000 : 55, code: fail ? "DNS_UNREACHABLE" : "", detail: fail ? "The secure lookup failed." : "RethinkDNS answered." },
      { id: "youtube", ok: !fail, ms: fail ? 3000 : 84, code: fail ? "NET_ERROR" : "", detail: fail ? "The request through the app failed." : "YouTube answered." },
      { id: "video", ok: !fail, ms: fail ? 3000 : 81, code: fail ? "TLS_ERROR" : "", detail: fail ? "The video server handshake failed." : "Video server handshake completed." },
    ];
    let i = 0;
    return new Promise((resolve) => {
      const next = () => {
        if (i >= steps.length) {
          resolve(steps);
          return;
        }
        const step = steps[i];
        for (const cb of this.testListeners) cb(step);
        i++;
        window.setTimeout(next, 450);
      };
      window.setTimeout(next, 300);
    });
  }

  async autoFix(): Promise<AutoFixResult> {
    this.setPhase("connecting", "trying:strong");
    window.setTimeout(() => {
      this.setPhase("on", "");
      this.stateValue = { ...this.stateValue, notice: { code: "METHOD_SWITCHED", arg: "strong" } };
      this.emitState();
    }, 900);
    return { method: "strong", error: null };
  }

  async restoreWindowsProxy(): Promise<ApiResult> {
    return ok;
  }

  async getDiagnosticReport(): Promise<DiagnosticReport> {
    return {
      report:
        "Younblock 1.0.0 diagnostic report\nplatform: windows\ngenerated: 2026-10-13T22:30:00Z\nstate: on (detail \"\")\nmode: youtube method: auto port: 4525\nmeasured sleep for 1ms request: 15 ms\ncounters: conns total 42, live 3, errors 0, panics 0\ncounters: dns ok 30, fail 0, up 420.0 KB, down 12.4 MB\nself-test: listener=ok dns=ok youtube=ok video=ok\nlast 12 log lines:\n  2026-10-13T22:29:12.000 INFO engine ready platform=windows\n",
    };
  }

  async openLogsFolder(): Promise<LogsFolder> {
    return { path: this.info.logDir };
  }

  async quit(): Promise<ApiResult> {
    return ok;
  }

  onState(cb: (state: BackendState) => void): () => void {
    this.stateListeners.add(cb);
    cb(this.stateValue);
    return () => this.stateListeners.delete(cb);
  }

  onStats(cb: (stats: Stats) => void): () => void {
    this.statsListeners.add(cb);
    cb(this.statsValue);
    return () => this.statsListeners.delete(cb);
  }

  onTestProgress(cb: (step: TestStep) => void): () => void {
    this.testListeners.add(cb);
    return () => this.testListeners.delete(cb);
  }

  onCloseRequested(cb: () => void): () => void {
    this.closeListeners.add(cb);
    return () => this.closeListeners.delete(cb);
  }

  onBrowserEvent(_cb: (kind: "opened" | "closed" | "failed") => void): () => void {
    // The mock shell never launches a browser.
    return () => {};
  }

  hideWindow(): void {
    // In a browser there is nothing to hide.
  }
}

function defaultSettings(): Settings {
  return {
    version: 1,
    mode: "youtube",
    method: "auto",
    port: 4525,
    autoPort: true,
    startWithWindows: false,
    connectOnLaunch: false,
    openBrowserOnConnect: false,
    firstRunDone: true,
    closeNoticeShown: false,
    advanced: {
      pieceSizeMin: 0,
      pieceSizeMax: 0,
      pauseMs: 0,
      dohServers: [
        { name: "RethinkDNS", url: "https://sky.rethinkdns.com/?dns={dns}", bootstrap: ["172.67.162.27"] },
        { name: "Cloudflare", url: "https://cloudflare-dns.com/dns-query?dns={dns}", bootstrap: ["1.1.1.1", "1.0.0.1"] },
        { name: "Google", url: "https://dns.google/dns-query?dns={dns}", bootstrap: ["8.8.8.8", "8.8.4.4"] },
      ],
      domains: [
        "youtube.com",
        "youtu.be",
        "googlevideo.com",
        "ytimg.com",
        "ggpht.com",
        "youtube-nocookie.com",
        "youtubei.googleapis.com",
      ],
      logDomains: false,
      idleTimeoutSec: 0,
    },
  };
}

export const mockBackend = new MockBackend();
