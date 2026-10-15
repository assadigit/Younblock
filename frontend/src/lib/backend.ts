// backend.ts — the one seam between the Younblock screen and the host app.
// The screen only knows getSnapshot / subscribe / connect / disconnect;
// createBackend() picks the right transport:
//   * inside Wails: the generated bindings + the "younblock:status" event
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
} as const;
/** Wails event carrying a fresh Snapshot. */
const STATUS_EVENT = "younblock:status";
// =====================================================================

import type { Snapshot } from "./types";

/** The backend surface the screen talks to. */
export interface Backend {
  getSnapshot(): Promise<Snapshot>;
  subscribe(cb: (s: Snapshot) => void): () => void;
  connect(): Promise<void>;
  disconnect(): Promise<void>;
}

/** Create the backend for this environment (Wails when available). */
export function createBackend(): Backend {
  if (wailsAvailable()) {
    return new WailsBackend();
  }
  return previewBackend();
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
