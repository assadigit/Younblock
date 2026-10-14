// backend.ts — the one TypeScript contract mirroring file 01, section 10.
// Components never call Wails directly; an adapter implements this
// interface (wails.ts in the app, mock.ts in a plain browser, android.ts
// later).

export interface Notice {
  code: string;
  arg: string;
}

export interface StateError {
  code: string;
}

/** Which built-in check failed last (only while degraded). */
export interface StateFail {
  step: "listener" | "dns" | "youtube" | "video";
  code: string;
}

export interface BackendState {
  phase: "off" | "connecting" | "on" | "degraded" | "error" | "stopping";
  detail: string;
  since: string;
  port: number;
  mode: "youtube" | "all" | "manual";
  method: string;
  /** Last measured YouTube round-trip in ms (only while on/degraded). */
  latencyMs?: number | null;
  fail?: StateFail | null;
  notice?: Notice | null;
  error?: StateError | null;
}

export interface Stats {
  bytesDown: number;
  bytesUp: number;
  rateDown: number;
  rateUp: number;
  activeConns: number;
  uptimeSec: number;
}

export interface DoHServer {
  name: string;
  url: string;
  bootstrap: string[];
}

export interface Settings {
  version: number;
  mode: "youtube" | "all" | "manual";
  method: string;
  port: number;
  autoPort: boolean;
  startWithWindows: boolean;
  connectOnLaunch: boolean;
  openBrowserOnConnect: boolean;
  firstRunDone: boolean;
  closeNoticeShown: boolean;
  advanced: {
    pieceSizeMin: number;
    pieceSizeMax: number;
    pauseMs: number;
    dohServers: DoHServer[];
    domains: string[];
    logDomains: boolean;
    idleTimeoutSec: number;
  };
}

export interface TestStep {
  id: "listener" | "dns" | "youtube" | "video";
  ok: boolean;
  ms: number;
  code: string;
  detail: string;
}

export interface AppInfo {
  name: string;
  version: string;
  platform: string;
  capabilities: {
    tray: boolean;
    startWithOS: boolean;
    systemProxy: boolean;
    perAppRouting: boolean;
  };
  configPath: string;
  logDir: string;
}

export interface ApiResult {
  error: string | null;
}

export interface AutoFixResult {
  method: string | null;
  error?: string | null;
}

export interface DiagnosticReport {
  report: string;
}

export interface LogsFolder {
  path: string;
}

/** The backend contract (same method names and JSON shapes as file 01 §10). */
export interface Backend {
  get state(): BackendState;
  get settings(): Settings;
  get appInfo(): AppInfo;

  start(): Promise<ApiResult>;
  stop(): Promise<ApiResult>;
  toggle(): Promise<ApiResult>;

  saveSettings(settings: Settings): Promise<Settings | ApiResult>;

  runConnectionTest(): Promise<TestStep[]>;
  autoFix(): Promise<AutoFixResult>;
  restoreWindowsProxy(): Promise<ApiResult>;
  getDiagnosticReport(): Promise<DiagnosticReport>;
  openLogsFolder(): Promise<LogsFolder>;
  quit(): Promise<ApiResult>;

  onState(cb: (state: BackendState) => void): () => void;
  onStats(cb: (stats: Stats) => void): () => void;
  onTestProgress(cb: (step: TestStep) => void): () => void;
  onCloseRequested(cb: () => void): () => void;
  /** Shell notices about the optional dedicated Chrome session. */
  onBrowserEvent(cb: (kind: "opened" | "closed" | "failed") => void): () => void;

  hideWindow(): void;
}

export function parseApiResult(json: string): ApiResult {
  try {
    return JSON.parse(json) as ApiResult;
  } catch {
    return { error: "UNKNOWN" };
  }
}
