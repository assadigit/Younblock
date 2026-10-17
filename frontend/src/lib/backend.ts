// backend.ts — the one seam between the Younblock screen and the host app.
// The screen only knows getSnapshot / subscribe / connect / disconnect;
// createBackend() picks the right transport:
//   * inside Wails: the generated bindings + the "younblock:status" event
//   * served by the younblock binary itself (the Linux single-file
//     edition): the localhost JSON API + Server-Sent Events
//   * in a plain browser (vite dev, screenshots): a small offline preview
//
// =====================================================================
// CONFIG — when the host app's names differ, change only this block.
// =====================================================================
/** Method names on the bound Go App struct (window.go.main.App.<name>). */
const BINDINGS = {
  connect: "Connect",
  disconnect: "Disconnect",
  getSnapshot: "GetSnapshot",
  getAppInfo: "GetAppInfo",
  getSettings: "GetSettings",
  saveSettings: "SaveSettings",
  getDiagnosticReport: "GetDiagnosticReport",
} as const;
/** Wails event carrying a fresh Snapshot. */
const STATUS_EVENT = "younblock:status";
// =====================================================================

import type { Snapshot } from "./types";

/** App metadata from the host (internal/config on the Go side). The
 *  platform and capabilities fields gate the Android-only surfaces (the
 *  apps row, the VPN-slot copy, the privacy disclosure); a preview build
 *  has no host and they stay unset. */
export interface AppInfo {
  name: string;
  version: string;
  platform?: string;
  capabilities?: {
    perAppRouting?: boolean;
    [key: string]: boolean | undefined;
  };
}

/** The settings document (Go internal/config.Settings). Only the fields
 *  this screen reads are typed; the index signature keeps a full-document
 *  round-trip honest (fields this screen never touches survive a save). */
export interface Settings {
  version: number;
  mode: string;
  method: string;
  port: number;
  autoPort: boolean;
  startWithWindows: boolean;
  connectOnLaunch: boolean;
  openBrowserOnConnect: boolean;
  firstRunDone: boolean;
  closeNoticeShown: boolean;
  /** The Android per-app allow-list (schema v2, M6). */
  apps?: string[];
  advanced?: {
    dohServers?: Array<{ name?: string; url?: string }>;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

/** The backend surface the screen talks to. */
export interface Backend {
  getSnapshot(): Promise<Snapshot>;
  subscribe(cb: (s: Snapshot) => void): () => void;
  connect(): Promise<void>;
  disconnect(): Promise<void>;
  /** The host's name/version, or null when this build has no host (preview). */
  getAppInfo(): Promise<AppInfo | null>;
  /** The current settings document, or null without a host. */
  getSettings(): Promise<Settings | null>;
  /** Saves a full settings document; resolves with the normalized
   *  settings, rejects with a typed error. Null without a host. */
  saveSettings(s: Settings): Promise<Settings>;
  /** The redacted diagnostic report text, or null without a host. */
  getDiagnosticReport(): Promise<string | null>;
}

/** Create the backend for this environment (Wails, then HTTP, then preview). */
export function createBackend(): Backend {
  if (wailsAvailable()) {
    return new WailsBackend();
  }
  return autoBackend();
}

// ---- normalization -------------------------------------------------------
// The Go side sends 0 / -1 for "none"; the screen wants null.

function normalize(raw: any): Snapshot {
  const s: Snapshot = {
    phase: raw?.phase ?? "disconnected",
    connectedAtMs: raw?.connectedAtMs ? Number(raw.connectedAtMs) : null,
    downBps: Number(raw?.downBps ?? 0) || 0,
    upBps: Number(raw?.upBps ?? 0) || 0,
    pingMs: raw?.pingMs === null || raw?.pingMs === undefined || Number(raw.pingMs) < 0 ? null : Number(raw.pingMs),
    message: raw?.message ?? null,
  };
  return s;
}

// ---- the Wails adapter ---------------------------------------------------

interface WailsRuntime {
  EventsOn(eventName: string, callback: (...data: any[]) => void): () => void;
}

interface WailsWindow {
  go?: {
    main?: {
      App?: Record<string, (...args: any[]) => Promise<any>>;
    };
  };
  runtime?: WailsRuntime;
}

function w(): WailsWindow {
  return window as unknown as WailsWindow;
}

/** True when running inside the Wails webview with generated bindings. */
function wailsAvailable(): boolean {
  const app = w().go?.main?.App;
  return (
    typeof app !== "undefined" &&
    typeof app[BINDINGS.connect] === "function" &&
    typeof app[BINDINGS.disconnect] === "function" &&
    typeof app[BINDINGS.getSnapshot] === "function"
  );
}

class WailsBackend implements Backend {
  private bindings: Record<string, (...args: any[]) => Promise<any>>;
  private listeners = new Set<(s: Snapshot) => void>();

  constructor() {
    const app = w().go?.main?.App;
    if (!app) {
      throw new Error("wails bindings are unavailable");
    }
    this.bindings = app;
    w().runtime?.EventsOn(STATUS_EVENT, (payload: any) => {
      const snap = normalize(payload);
      for (const cb of this.listeners) cb(snap);
    });
  }

  async getSnapshot(): Promise<Snapshot> {
    return normalize(await this.bindings[BINDINGS.getSnapshot]());
  }

  subscribe(cb: (s: Snapshot) => void): () => void {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }

  async connect(): Promise<void> {
    await this.bindings[BINDINGS.connect]();
  }

  async disconnect(): Promise<void> {
    await this.bindings[BINDINGS.disconnect]();
  }

  // GetAppInfo returns the same JSON the webui serves at /api/appinfo
  // ({"name":..,"version":..,"platform":..,"capabilities":{..}}); the
  // screen keeps the platform/capability fields when the host sends them
  // (Android gates its extra surfaces on them; desktops simply lack them).
  private info: AppInfo | null = null;
  async getAppInfo(): Promise<AppInfo | null> {
    if (this.info) return this.info;
    try {
      const fn = this.bindings[BINDINGS.getAppInfo];
      if (typeof fn !== "function") return null;
      const raw = JSON.parse(String(await fn())) as Partial<AppInfo>;
      if (raw && typeof raw.version === "string" && raw.version !== "") {
        this.info = {
          name: String(raw.name ?? ""),
          version: raw.version,
          platform: typeof raw.platform === "string" ? raw.platform : undefined,
          capabilities: raw.capabilities,
        };
      }
      return this.info;
    } catch {
      return null;
    }
  }

  // Settings and the diagnostic report: same JSON strings the webui
  // serves at /api/settings and /api/report (app.go mirrors jsonapi.go).
  async getSettings(): Promise<Settings | null> {
    try {
      const fn = this.bindings[BINDINGS.getSettings];
      if (typeof fn !== "function") return null;
      return JSON.parse(String(await fn())) as Settings;
    } catch {
      return null;
    }
  }

  async saveSettings(s: Settings): Promise<Settings> {
    const fn = this.bindings[BINDINGS.saveSettings];
    const reply = JSON.parse(String(await fn(JSON.stringify(s)))) as {
      error?: string | null;
    };
    if (reply.error) throw new Error(reply.error);
    return reply as Settings;
  }

  async getDiagnosticReport(): Promise<string | null> {
    try {
      const fn = this.bindings[BINDINGS.getDiagnosticReport];
      if (typeof fn !== "function") return null;
      const raw = JSON.parse(String(await fn())) as { report?: string };
      return typeof raw.report === "string" ? raw.report : null;
    } catch {
      return null;
    }
  }
}

// ---- the Linux single-file edition (HTTP + Server-Sent Events) -----------
// The younblock binary serves this very page and a localhost JSON API
// (internal/webui): GET /api/state, POST /api/start, POST /api/stop, and
// /api/events pushing "state" and "stats" events. The mapping below mirrors
// what the Wails side does on Windows (ui_bridge.go + engine_adapter.go):
// on and degraded both count as connected, stopping is "disconnecting", and
// the one message the screen knows is the connect-failure line.

/** Phases of the engine state (internal/engine). */
type EnginePhase = "off" | "connecting" | "on" | "degraded" | "error" | "stopping";

/** The fields of /api state and "state" events that the screen needs. */
interface EngineState {
  phase: EnginePhase;
  detail: string;
  since: string;
  latencyMs?: number | null;
  /** The typed start-failure code while the phase is "error"
   *  (internal/engine StateError). The screen maps the one code this
   *  product can honestly explain; everything else stays generic. */
  error?: { code?: string } | null;
}

/** The fields of "stats" events that the screen needs (bytes per second). */
interface EngineStats {
  rateDown: number;
  rateUp: number;
}

/** Probe quickly whether this page is served by the younblock binary.
 *  Must fail fast (timeout) so a plain static server (vite preview, the
 *  screenshot harness) falls through to the preview backend without hanging. */
async function httpAvailable(timeoutMs = 1500): Promise<boolean> {
  if (typeof window === "undefined") return false;
  // Only http(s) pages can talk to a same-origin API; file:// never can.
  if (window.location.protocol !== "http:" && window.location.protocol !== "https:") return false;
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch("/api/appinfo", { signal: controller.signal, cache: "no-store" });
    if (!res.ok) return false;
    const info = (await res.json()) as { name?: string };
    return info.name === "Younblock";
  } catch {
    return false;
  } finally {
    window.clearTimeout(timer);
  }
}

/** POST one engine command and surface its typed error, if any. */
async function postCommand(path: string): Promise<void> {
  let res: Response;
  try {
    res = await fetch(path, { method: "POST", cache: "no-store" });
  } catch {
    throw new Error("network");
  }
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const body = (await res.json()) as { error?: string | null };
  if (body.error) throw new Error(body.error);
}

class HttpBackend implements Backend {
  private listeners = new Set<(s: Snapshot) => void>();
  private snap: Snapshot = { phase: "disconnected", connectedAtMs: null, downBps: 0, upBps: 0, pingMs: null };
  private readonly events: EventSource;
  private poll: number | undefined;

  constructor() {
    this.events = new EventSource("/api/events");
    this.events.addEventListener("state", (ev) => {
      this.applyState(JSON.parse(ev.data) as EngineState);
    });
    this.events.addEventListener("stats", (ev) => {
      const s = JSON.parse(ev.data) as EngineStats;
      // The rates arrive as bytes per second; the screen shows bits.
      this.update({ downBps: (Number(s.rateDown) || 0) * 8, upBps: (Number(s.rateUp) || 0) * 8 });
    });
  }

  async getSnapshot(): Promise<Snapshot> {
    try {
      const res = await fetch("/api/state", { cache: "no-store" });
      if (res.ok) {
        this.applyState((await res.json()) as EngineState);
      }
    } catch {
      // The server went away; the last known state stands.
    }
    return this.snap;
  }

  subscribe(cb: (s: Snapshot) => void): () => void {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }

  async connect(): Promise<void> {
    await postCommand("/api/start");
  }

  async disconnect(): Promise<void> {
    await postCommand("/api/stop");
  }

  // /api/appinfo is the same document httpAvailable() probes; fetched once
  // on demand and cached — the About sheet asks only when it is opened.
  private infoPromise: Promise<AppInfo | null> | undefined;
  getAppInfo(): Promise<AppInfo | null> {
    if (!this.infoPromise) {
      this.infoPromise = fetch("/api/appinfo", { cache: "no-store" })
        .then((res) => (res.ok ? (res.json() as Promise<Partial<AppInfo>>) : null))
        .then((info) =>
          info && typeof info.version === "string" && info.version !== ""
            ? {
                name: String(info.name ?? ""),
                version: info.version,
                platform: typeof info.platform === "string" ? info.platform : undefined,
                capabilities: info.capabilities,
              }
            : null,
        )
        .catch(() => null);
    }
    return this.infoPromise;
  }

  // /api/settings (GET/POST) and /api/report carry the same JSON shapes
  // the Wails bindings expose; the Android WebView reaches the very same
  // server through its loopback URL (ADR-0005).
  async getSettings(): Promise<Settings | null> {
    try {
      const res = await fetch("/api/settings", { cache: "no-store" });
      if (!res.ok) return null;
      return (await res.json()) as Settings;
    } catch {
      return null;
    }
  }

  async saveSettings(s: Settings): Promise<Settings> {
    let res: Response;
    try {
      res = await fetch("/api/settings", {
        method: "POST",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(s),
      });
    } catch {
      throw new Error("network");
    }
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const reply = (await res.json()) as { error?: string | null } & Settings;
    if (reply.error) throw new Error(String(reply.error));
    return reply;
  }

  async getDiagnosticReport(): Promise<string | null> {
    try {
      const res = await fetch("/api/report", { cache: "no-store" });
      if (!res.ok) return null;
      const raw = (await res.json()) as { report?: string };
      return typeof raw.report === "string" ? raw.report : null;
    } catch {
      return null;
    }
  }

  /** Fold one fresh engine state into the snapshot, the bridge's mapping. */
  private applyState(state: EngineState): void {
    const prev = this.snap;
    const phase: Snapshot["phase"] =
      state.phase === "connecting"
        ? "connecting"
        : state.phase === "on" || state.phase === "degraded"
          ? "connected"
          : state.phase === "stopping"
            ? "disconnecting"
            : state.phase === "error"
              ? "error"
              : "disconnected";
    // Speeds only mean something while connected (or turning off); every
    // other state starts clean, exactly like a fresh Snapshot in the bridge.
    const keepStats = phase === "connected" || phase === "disconnecting";
    this.snap = {
      phase,
      // "since" is when this engine phase began; keep the first one across
      // on↔degraded flaps so the on-screen timer does not reset.
      connectedAtMs:
        phase === "connected"
          ? prev.phase === "connected" && prev.connectedAtMs
            ? prev.connectedAtMs
            : Date.parse(state.since) || null
          : null,
      downBps: keepStats ? prev.downBps : 0,
      upBps: keepStats ? prev.upBps : 0,
      pingMs:
        phase === "disconnecting"
          ? prev.pingMs
          : phase === "connected" && typeof state.latencyMs === "number" && state.latencyMs > 0
            ? state.latencyMs
            : null,
      message:
        phase === "error"
          ? state.error?.code === "NO_TARGET_APPS"
            ? "Your apps-to-unblock list is empty, or none of its apps is installed. Add an app in the menu, then try again."
            : "Could not connect. Try again."
          : null,
    };
    this.schedulePoll();
    this.emit();
  }

  /** Patch the live speeds from a "stats" event. */
  private update(patch: Pick<Snapshot, "downBps" | "upBps">): void {
    this.snap = { ...this.snap, ...patch };
    this.emit();
  }

  /** The ping lives in the engine state, but only state *changes* are
   *  pushed over SSE — so while connected the state is re-read once per
   *  second, the same cadence as the Wails bridge's RunStats. */
  private schedulePoll(): void {
    const need = this.snap.phase === "connected";
    if (need && this.poll === undefined) {
      this.poll = window.setInterval(() => {
        if (this.snap.phase !== "connected") return;
        fetch("/api/state", { cache: "no-store" })
          .then((res) => (res.ok ? res.json() : null))
          .then((state) => {
            if (state) this.applyState(state as EngineState);
          })
          .catch(() => {});
      }, 1000);
    } else if (!need && this.poll !== undefined) {
      window.clearInterval(this.poll);
      this.poll = undefined;
    }
  }

  private emit(): void {
    for (const cb of this.listeners) cb(this.snap);
  }
}

/** Wails is not here, but two very different hosts can still serve this
 *  page: the younblock binary itself (the Linux edition, a real engine
 *  behind /api) or a plain dev server (vite, the screenshot harness) with
 *  no engine at all. The probe tells them apart; until it settles the
 *  screen simply shows "off". */
function autoBackend(): Backend {
  let resolved: Backend | null = null;
  const waiting: Array<() => void> = [];
  const listeners = new Set<(s: Snapshot) => void>();

  const settle = (b: Backend): void => {
    resolved = b;
    b.subscribe((s) => {
      for (const cb of listeners) cb(s);
    });
    for (const wake of waiting) wake();
  };

  void httpAvailable().then((ok) => settle(ok ? new HttpBackend() : previewBackend()));

  const whenReady = (): Promise<Backend> =>
    resolved !== null
      ? Promise.resolve(resolved)
      : new Promise<Backend>((wake) => waiting.push(() => wake(resolved as Backend)));

  return {
    async getSnapshot(): Promise<Snapshot> {
      return (await whenReady()).getSnapshot();
    },
    subscribe(cb: (s: Snapshot) => void): () => void {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    async connect(): Promise<void> {
      return (await whenReady()).connect();
    },
    async disconnect(): Promise<void> {
      return (await whenReady()).disconnect();
    },
    async getAppInfo(): Promise<AppInfo | null> {
      return (await whenReady()).getAppInfo();
    },
    async getSettings(): Promise<Settings | null> {
      return (await whenReady()).getSettings();
    },
    async saveSettings(s: Settings): Promise<Settings> {
      return (await whenReady()).saveSettings(s);
    },
    async getDiagnosticReport(): Promise<string | null> {
      return (await whenReady()).getDiagnosticReport();
    },
  };
}

// ---- the offline preview (plain-browser development only) ----------------
// Not part of the shipped app: it exists so `npm run dev` in a plain
// browser — and screenshot/E2E runs — can exercise every state without
// Wails or a real proxy. Add ?fail=1 to the URL to watch the error path.

function previewBackend(): Backend {
  const fail = new URLSearchParams(window.location.search).has("fail");
  let snap: Snapshot = {
    phase: "disconnected",
    connectedAtMs: null,
    downBps: 0,
    upBps: 0,
    pingMs: null,
  };
  const listeners = new Set<(s: Snapshot) => void>();
  let seq = 0;
  let statsTimer: number | undefined;

  function emit(): void {
    for (const cb of listeners) cb(snap);
  }

  function runStats(): void {
    window.clearInterval(statsTimer);
    statsTimer = window.setInterval(() => {
      if (snap.phase !== "connected") return;
      const drift = (n: number) => Math.max(0, n * (0.75 + Math.random() * 0.5));
      snap = {
        ...snap,
        downBps: drift(136.6e6),
        upBps: drift(0.33e6),
        pingMs: 38 + Math.floor(Math.random() * 12),
      };
      emit();
    }, 1000);
  }

  return {
    async getSnapshot(): Promise<Snapshot> {
      return snap;
    },
    subscribe(cb: (s: Snapshot) => void): () => void {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    // No host behind a plain browser: null makes the About sheet hide its
    // version line instead of inventing one that would go stale.
    async getAppInfo(): Promise<AppInfo | null> {
      return null;
    },
    // Same honesty for the M6 surfaces: without a host there are no real
    // settings to show, no list to save, no report to render — the sheet
    // hides the Android-only sections and the diagnostics button.
    async getSettings(): Promise<Settings | null> {
      return null;
    },
    async saveSettings(): Promise<Settings> {
      throw new Error("no host");
    },
    async getDiagnosticReport(): Promise<string | null> {
      return null;
    },
    async connect(): Promise<void> {
      if (snap.phase !== "disconnected" && snap.phase !== "error") return;
      const id = ++seq;
      snap = { ...snap, phase: "connecting" };
      emit();
      await new Promise((r) => window.setTimeout(r, 900));
      if (id !== seq) return; // canceled meanwhile
      if (fail) {
        snap = { ...snap, phase: "error", message: "Could not connect. Try again." };
      } else {
        snap = {
          phase: "connected",
          connectedAtMs: Date.now(),
          downBps: 0,
          upBps: 0,
          pingMs: null,
        };
        runStats();
      }
      emit();
    },
    async disconnect(): Promise<void> {
      if (snap.phase === "disconnected") return;
      if (snap.phase === "error") {
        snap = { ...snap, phase: "disconnected", message: null };
        emit();
        return;
      }
      const id = ++seq;
      snap = { ...snap, phase: "disconnecting" };
      emit();
      await new Promise((r) => window.setTimeout(r, 450));
      if (id !== seq) return;
      window.clearInterval(statsTimer);
      snap = { phase: "disconnected", connectedAtMs: null, downBps: 0, upBps: 0, pingMs: null };
      emit();
    },
  };
}
