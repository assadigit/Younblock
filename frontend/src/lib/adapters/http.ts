// http.ts — the adapter for the single-executable Linux edition (D50):
// the same engine and the same Svelte UI, but the JSON API is served over
// localhost HTTP by the younblock binary itself (`--web`), and events
// arrive as Server-Sent Events instead of Wails runtime events.
// Components never touch this directly (file 02 §9); App.svelte picks it
// automatically when the page is served by the younblock web server.
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

/** Events the server pushes over /api/events (same names as Wails). */
type EventName = "state" | "stats" | "test:progress" | "close-requested" | "browser:opened" | "browser:closed" | "browser:failed";

/** Probe quickly whether this page is served by the younblock web server.
 *  Must fail fast (timeout) so a plain static server (vite preview, the
 *  screenshot harness) falls through to the mock backend without hanging. */
export async function httpAvailable(timeoutMs = 1500): Promise<boolean> {
  if (typeof window === "undefined") return false;
  // Only http(s) pages can talk to a same-origin API; file:// never.
  if (window.location.protocol !== "http:" && window.location.protocol !== "https:") return false;
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch("/api/appinfo", { signal: controller.signal, cache: "no-store" });
    if (!res.ok) return false;
    const info = (await res.json()) as Partial<AppInfo>;
    return info.name === "Younblock";
  } catch {
    return false;
  } finally {
    window.clearTimeout(timer);
  }
}

export class HttpBackend implements Backend {
  private stateValue: BackendState = {
    phase: "off",
    detail: "",
    since: new Date().toISOString(),
    port: 4525,
    mode: "youtube",
    method: "auto",
  };
  private settingsValue: Settings | null = null;
  private info: AppInfo | null = null;
  private events: EventSource | null = null;

  private stateListeners = new Set<(s: BackendState) => void>();
  private statsListeners = new Set<(s: Stats) => void>();
  private testListeners = new Set<(s: TestStep) => void>();
  private closeListeners = new Set<() => void>();
  private browserListeners = new Set<(kind: "opened" | "closed" | "failed") => void>();

  constructor() {
    this.events = new EventSource("/api/events");
    const on = (name: EventName, parse: (payload: string) => void) => {
      this.events?.addEventListener(name, (ev) => {
        parse((ev as MessageEvent).data as string);
      });
    };
    on("state", (payload) => {
      this.stateValue = JSON.parse(payload) as BackendState;
      for (const cb of this.stateListeners) cb(this.stateValue);
    });
    on("stats", (payload) => {
      const stats = JSON.parse(payload) as Stats;
      for (const cb of this.statsListeners) cb(stats);
    });
    on("test:progress", (payload) => {
      const step = JSON.parse(payload) as TestStep;
      for (const cb of this.testListeners) cb(step);
    });
    // close-requested never fires in the web edition (there is no window
    // to close); browser:* events are Windows-shell behavior, kept for
    // contract completeness.
    on("close-requested", () => {
      for (const cb of this.closeListeners) cb();
    });
    const browserKind = (name: EventName, kind: "opened" | "closed" | "failed") => {
      on(name, () => {
        for (const cb of this.browserListeners) cb(kind);
      });
    };
    browserKind("browser:opened", "opened");
    browserKind("browser:closed", "closed");
    browserKind("browser:failed", "failed");
  }

  get state(): BackendState {
    return this.stateValue;
  }

  get settings(): Settings {
    if (!this.settingsValue) {
      throw new Error("settings not loaded");
    }
    return this.settingsValue;
  }

  get appInfo(): AppInfo {
    if (!this.info) {
      throw new Error("app info not loaded");
    }
    return this.info;
  }

  /** Prime the caches and start from the server's current state. */
  async prime(): Promise<void> {
    const [state, settings, info] = await Promise.all([
      this.getJson<BackendState>("/api/state"),
      this.getJson<Settings>("/api/settings"),
      this.getJson<AppInfo>("/api/appinfo"),
    ]);
    this.stateValue = state;
    this.settingsValue = settings;
    this.info = info;
  }

  private async getJson<T>(path: string): Promise<T> {
    const res = await fetch(path, { cache: "no-store" });
    if (!res.ok) {
      throw new Error(`api ${path} failed: ${res.status}`);
    }
    return (await res.json()) as T;
  }

  private async postJson<T>(path: string, body?: unknown): Promise<T> {
    const res = await fetch(path, {
      method: "POST",
      headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      throw new Error(`api ${path} failed: ${res.status}`);
    }
    return (await res.json()) as T;
  }

  async start(): Promise<ApiResult> {
    return this.postJson<ApiResult>("/api/start");
  }

  async stop(): Promise<ApiResult> {
    return this.postJson<ApiResult>("/api/stop");
  }

  async toggle(): Promise<ApiResult> {
    return this.postJson<ApiResult>("/api/toggle");
  }

  async saveSettings(settings: Settings): Promise<Settings | ApiResult> {
    const raw = await this.postJson<Settings | ApiResult>("/api/settings", settings);
    if ((raw as ApiResult).error !== undefined && (raw as ApiResult).error !== null) {
      return raw as ApiResult;
    }
    this.settingsValue = raw as Settings;
    return this.settingsValue;
  }

  async runConnectionTest(): Promise<TestStep[]> {
    const raw = await this.postJson<TestStep[] | ApiResult>("/api/test");
    if (Array.isArray(raw)) {
      return raw;
    }
    return [];
  }

  async autoFix(): Promise<AutoFixResult> {
    return this.postJson<AutoFixResult>("/api/autofix");
  }

  async restoreWindowsProxy(): Promise<ApiResult> {
    return this.postJson<ApiResult>("/api/restore-proxy");
  }

  async getDiagnosticReport(): Promise<DiagnosticReport> {
    return this.getJson<DiagnosticReport>("/api/report");
  }

  async openLogsFolder(): Promise<LogsFolder> {
    return this.getJson<LogsFolder>("/api/logs");
  }

  async quit(): Promise<ApiResult> {
    // The server replies first, then exits.
    return this.postJson<ApiResult>("/api/quit");
  }

  onState(cb: (state: BackendState) => void): () => void {
    this.stateListeners.add(cb);
    return () => this.stateListeners.delete(cb);
  }

  onStats(cb: (stats: Stats) => void): () => void {
    this.statsListeners.add(cb);
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

  onBrowserEvent(cb: (kind: "opened" | "closed" | "failed") => void): () => void {
    this.browserListeners.add(cb);
    return () => this.browserListeners.delete(cb);
  }

  hideWindow(): void {
    // There is no window to hide in the web edition.
  }
}
