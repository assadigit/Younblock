// android.ts — a stub for the later Android run (file 04). It throws so an
// accidental use is loud and immediate.
import type { Backend } from "../backend";

export class AndroidBackend implements Backend {
  constructor() {
    throw new Error("The Android adapter is not built yet (file 04, later run).");
  }

  get state(): never {
    throw new Error("not built yet");
  }

  get settings(): never {
    throw new Error("not built yet");
  }

  get appInfo(): never {
    throw new Error("not built yet");
  }

  start(): never {
    throw new Error("not built yet");
  }

  stop(): never {
    throw new Error("not built yet");
  }

  toggle(): never {
    throw new Error("not built yet");
  }

  saveSettings(): never {
    throw new Error("not built yet");
  }

  runConnectionTest(): never {
    throw new Error("not built yet");
  }

  autoFix(): never {
    throw new Error("not built yet");
  }

  restoreWindowsProxy(): never {
    throw new Error("not built yet");
  }

  getDiagnosticReport(): never {
    throw new Error("not built yet");
  }

  openLogsFolder(): never {
    throw new Error("not built yet");
  }

  quit(): never {
    throw new Error("not built yet");
  }

  onState(): () => void {
    throw new Error("not built yet");
  }

  onStats(): () => void {
    throw new Error("not built yet");
  }

  onTestProgress(): () => void {
    throw new Error("not built yet");
  }

  onCloseRequested(): () => void {
    throw new Error("not built yet");
  }

  hideWindow(): void {
    throw new Error("not built yet");
  }
}
