// wails.ts — the adapter over the generated Wails bindings and the
// runtime events. Components never touch this directly (file 02 §9).
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

interface WailsRuntime {
  EventsOn(eventName: string, callback: (...data: any[]) => void): () => void;
  WindowHide(): void;
  WindowShow(): void;
}

interface AppBindings {
  GetState(): Promise<string>;
  Start(): Promise<string>;
  Stop(): Promise<string>;
  Toggle(): Promise<string>;
  GetSettings(): Promise<string>;
  SaveSettings(payload: string): Promise<string>;
  RunConnectionTest(): Promise<string>;
  AutoFix(): Promise<string>;
  RestoreWindowsProxy(): Promise<string>;
  GetDiagnosticReport(): Promise<string>;
  OpenLogsFolder(): Promise<string>;
  Quit(): Promise<string>;
  GetAppInfo(): Promise<string>;
}

interface WailsWindow {
  go?: {
    main?: {
      App?: AppBindings;
    };
  };
  runtime?: WailsRuntime;
}

function w(): WailsWindow {
  return window as unknown as WailsWindow;
}

/** True when running inside the Wails webview with generated bindings. */
export function wailsAvailable(): boolean {
  const win = w();
  return typeof win.go !== "undefined" && typeof win.go?.main?.App !== "undefined";
}

export class WailsBackend implements Backend {
  private bindings: AppBindings;
  private runtimeApi: WailsRuntime | undefined;
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

  constructor() {
    const app = w().go?.main?.App;
    if (!app) {
      throw new Error("wails bindings are unavailable");
    }
    this.bindings = app;
    this.runtimeApi = w().runtime;
    this.runtimeApi?.EventsOn("state", (payload: string) => {
      this.stateValue = JSON.parse(payload) as BackendState;
      for (const cb of this.stateListeners) cb(this.stateValue);
    });
    this.runtimeApi?.EventsOn("stats", (payload: string) => {
      const stats = JSON.parse(payload) as Stats;
      for (const cb of this.statsListeners) cb(stats);
    });
    this.runtimeApi?.EventsOn("test:progress", (payload: string) => {
      const step = JSON.parse(payload) as TestStep;
      for (const cb of this.testListeners) cb(step);
    });
    this.runtimeApi?.EventsOn("close-requested", () => {
      for (const cb of this.closeListeners) cb();
    });
    // Shell notices about the optional dedicated Chrome session (R13).
    for (const kind of ["opened", "closed", "failed"] as const) {
      this.runtimeApi?.EventsOn(`browser:${kind}`, () => {
        for (const cb of this.browserListeners) cb(kind);
      });
    }
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

  /** Prime the caches right after startup. */
  async prime(): Promise<void> {
    this.stateValue = JSON.parse(await this.bindings.GetState()) as BackendState;
    this.settingsValue = JSON.parse(await this.bindings.GetSettings()) as Settings;
    this.info = JSON.parse(await this.bindings.GetAppInfo()) as AppInfo;
  }

  async start(): Promise<ApiResult> {
    return JSON.parse(await this.bindings.Start()) as ApiResult;
  }

  async stop(): Promise<ApiResult> {
    return JSON.parse(await this.bindings.Stop()) as ApiResult;
  }

  async toggle(): Promise<ApiResult> {
    return JSON.parse(await this.bindings.Toggle()) as ApiResult;
  }

  async saveSettings(settings: Settings): Promise<Settings | ApiResult> {
    const payload = JSON.stringify(settings);
    const raw = await this.bindings.SaveSettings(payload);
    try {
      const parsed = JSON.parse(raw) as Settings | ApiResult;
      if ((parsed as ApiResult).error !== undefined && (parsed as ApiResult).error !== null) {
        return parsed as ApiResult;
      }
      this.settingsValue = parsed as Settings;
      return this.settingsValue;
    } catch {
      return { error: "UNKNOWN" };
    }
  }

  async runConnectionTest(): Promise<TestStep[]> {
    const raw = await this.bindings.RunConnectionTest();
    if (raw.startsWith("[")) {
      return JSON.parse(raw) as TestStep[];
    }
    return [];
  }

  async autoFix(): Promise<AutoFixResult> {
    const raw = await this.bindings.AutoFix();
    return JSON.parse(raw) as AutoFixResult;
  }

  async restoreWindowsProxy(): Promise<ApiResult> {
    return JSON.parse(await this.bindings.RestoreWindowsProxy()) as ApiResult;
  }

  async getDiagnosticReport(): Promise<DiagnosticReport> {
    return JSON.parse(await this.bindings.GetDiagnosticReport()) as DiagnosticReport;
  }

  async openLogsFolder(): Promise<LogsFolder> {
    return JSON.parse(await this.bindings.OpenLogsFolder()) as LogsFolder;
  }

  async quit(): Promise<ApiResult> {
    return JSON.parse(await this.bindings.Quit()) as ApiResult;
  }

  private stateListeners = new Set<(s: BackendState) => void>();
  private statsListeners = new Set<(s: Stats) => void>();
  private testListeners = new Set<(s: TestStep) => void>();
  private closeListeners = new Set<() => void>();
  private browserListeners = new Set<(kind: "opened" | "closed" | "failed") => void>();

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
    this.runtimeApi?.WindowHide();
  }
}
