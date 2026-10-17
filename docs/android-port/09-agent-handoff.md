# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M5 → M6 boundary (2026-10-17).
This file is the live handoff; keep the fields in §"Current handoff
fields" accurate at every session boundary.

## If starting Phase 2 (implementation) — CURRENT

1. Read every spec file; they are now evidence-backed, not baseline
   templates. The audit (doc 01) and ADRs (doc 08) are the ground
   truth; verify anything that looks stale against the repository.
2. Inspect the working tree; preserve unrelated/uncommitted changes.
3. Begin at **M3** (doc 05) — the Android toolchain first, then the
   Go-side additions (platform adapter, SOCKS5 door, mobile facade)
   which are host-testable even without the SDK.
4. Small changes; run the relevant tests after each; keep the desktop
   baseline (doc 01 §7) green on shared-tree edits.
5. Stop and document (ADR update) if a critical assumption fails; do
   not fake functionality or silently narrow the product scope.
6. Build the debug APK once the approved path is implementable; verify
   its actual existence/metadata; report tests honestly.
7. Update the specs with implementation reality, exact commands,
   artifact path/checksum, and commit/push status.
8. Keep the worklog discipline: read `/home/z/my-project/worklog.md`
   first; append your entry (Task ID + template) when done.

## Phase 1 record (completed 2026-10-16)

- Full source audit with symbols and line references (doc 01).
- Feasibility matrix scored; architecture accepted (docs 03, 08).
- Baseline green: go vet clean; go test 13/13; svelte-check 0/0;
  vite build clean; Android compile matrix 4/4 ABIs.
- Documentation-only phase: **zero application code changed**; the
  only repository additions are the completed specs themselves (and
  the extraction of this pack from the zip Ali uploaded).

## Environment / execution honesty (updated 2026-10-16, end of M3)

- **The Android toolchain is installed and working now**: JDK 21.0.8
  (Temurin, with javac), Android SDK (cmdline-tools 12.0,
  platform-tools, platforms 26+36, build-tools 36.1.0), NDK
  28.2.13676358, gomobile + gobind (x/mobile v0.0.0-20261011034528
  via a `go get -tool` directive), Gradle 9.8.1 via the committed
  wrapper. `dl.google.com`, `services.gradle.org`, `repo1.maven.org`,
  `proxy.golang.org` and the Adoptium GitHub release were all reachable
  and used this session.
- **What is still ⛔ is runtime verification**: this sandbox has no
  /dev/kvm (verified: `ls /dev/kvm` → No such file or directory), so no
  emulator, and no physical device. The Kotlin-level init/health/stop
  bridge call and every M4 on-device gate wait for a KVM-capable host
  or a real device. The bridge itself is proven at the Go level
  (mobile tests) and its Kotlin side is written and compiled into the
  verified APK.
- The wails `-m` rule (D68) still applies to desktop lanes; the
  Android lane never invokes wails. The sandbox still kills background
  processes when a Bash command ends; long builds ran as foreground
  chunks (go/gradle caches carried the progress).
- Do not claim source inspection, test success, APK creation, upload,
  or push without evidence. Do not commit secrets, signing keys, user
  settings, or private diagnostics. Do not undo unrelated working-tree
  changes to achieve a clean diff.

## Current handoff fields

- **Phase completed:** Phase 2 milestone **M5** — 2026-10-17, host-
  complete (core connection lifecycle and health reporting; evidence
  in doc 05 M5). Exit criteria: all host-runnable gates ✅ (the consent
  gate D77 — an unpermitted engine holds at "connecting", outside the
  start budget, cancellations quiet; the honest VpnState report D78 —
  routing requires serving AND bound AND the live server's door, the
  stale-stack case observable for reconciliation; the lifecycle suite
  against the real engine+stack — consent hold/grant/deny, three
  connect/disconnect cycles, PORT_BUSY injection, stale-stack refusal
  + reconciliation, service death; the Kotlin consent flow +
  single-flight + notification-from-vpnState + resume/death
  reconciliation, compiled into the verified APK); **on-device
  observation ⛔** (no /dev/kvm, no device — the standing blocker,
  re-verified this session; the M4+M5 device checklist rides together);
  desktop baseline ✅ 17/17 packages (run twice; two real bugs found
  and fixed while building the suite — the M4 DOOR-path fd leak
  violating D76, and the harness's AutoPort=false fixed to the product
  default).
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit. The session
  opened after a **full sandbox wipe** (the repo, ~/sdk toolchain and
  the worklog were all gone): everything was recovered from the
  recorded state — re-clone from the remote (token from the session
  prompt) at `98cac30`, toolchain reinstalled per doc 06's pins
  (Go 1.27.2, JDK 21.0.12.1+1 Temurin with javac, cmdline-tools 12.0,
  platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358, gomobile/
  gobind at the pinned x/mobile), worklog recreated from the prompt +
  repo records, and the desktop baseline re-verified GREEN before any
  M5 code was written.
- **Files changed this phase:** new — `internal/engine/gate_test.go`,
  `internal/platform/android/gate_test.go`, `mobile/lifecycle_test.go`,
  `mobile/vpnstate.go`; additive edits — `internal/engine/platform.go`
  (the optional `StartGater` interface, D77),
  `internal/engine/health.go` (connectSequence: run-lifetime ctx +
  the gate wait before the listener, outside the start budget),
  `internal/platform/android/android.go` (the start gate:
  SetStartReady/WaitStartReady), `mobile/mobile.go` (NewEngine arms
  the gate closed; the SetVpnReady facade door),
  `mobile/tun.go` (door recorded in tunState; the D76 fd-close fix on
  the two DOOR refusal paths), `mobile/tun_test.go` (the harness split
  + AutoPort=true product default), `mobile/mobile_test.go`
  (StartStopCycle opens the gate), `android/.../MainActivity.kt` (the
  consent flow: explain-once → prepare → dialog → denial copy;
  process-wide single-flight; resume reconciliation; activity-gone
  cleanup), `android/.../YounblockVpnService.kt` (setVpnReady on TUN
  establish, re-arm on every unwind; the vpnState-derived honest
  notification; service-death engine stop; bind-refusal unwinding),
  `android/.../strings.xml` (the consent + degraded notification
  copy). No desktop behavior changed: desktop adapters do not
  implement StartGater (the type assertion fails, the Start path is
  byte-for-byte identical), verified by the full regression gate.
- **Tests run and actual results:** `go vet ./...` clean; `go test
  -count=1 ./...` **17/17 packages ok**, run twice consecutively with
  no flakes (engine 8.9 s, proxy 9.6 s, mobile 33.7 s — the lifecycle
  suite dominates; tunstack 4.7 s); the concurrent-package PORT_BUSY
  interaction that failed the first full run was root-caused (the
  harness's AutoPort=false + fixed 4525 under `go test ./...`
  concurrency) and fixed at the source, not retried away; svelte-check
  0/0; vite build clean; Android library compile matrix 4/4 ABIs with
  `-mod=vendor`; `gomobile bind` OK (AAR 24,893,759 B, `javap` shows
  setVpnReady/vpnState); clean `assembleDebug` OK — app-debug.apk
  34,711,941 B, sha256
  c46026a7984d9fda69b45954436c630f625a74fecf451e6e50c2a3d2b6d5bf26,
  badging/apksigner verified (doc 05 M5 item 5).
- **Current architecture decision / confidence:** D77 (the platform
  start gate: the engine's connect run waits for the platform's
  permission/readiness door before the listener, holding "connecting"
  — consent precedes routing, no new phase invented, desktops
  byte-identical) and D78 (VpnState: the honest combined routing
  report with the live-server identity check, plus the shell
  reconciliation policy — service death stops the engine, the activity
  cleans a gate-held connect, the notification only claims what the
  report says). Confidence: high on the host-verified chain (every
  lifecycle scenario runs the REAL engine and stack, not mocks);
  medium on device behavior (the live dialog flow, notification
  channel semantics under OEM shells, Doze/screen-off, real radio
  switching) — all ride with the shared device checklist.
- **Blocking questions:** none architectural. Standing environment
  blocker: **no emulator (no /dev/kvm) and no device** — blocks the
  M4+M5 on-device checklist (live consent flow, notification behavior,
  screen-off persistence, real DoH through the stack, QUIC fallback,
  protect() on real sockets, reboot/revocation). Product confirmations
  still wanted from Ali (non-blocking, defaults chosen): allow-list
  defaults beyond Chrome+YouTube (M6 makes the list configurable),
  keystore handling (doc 07 §8).
- **Exact next task:** **M6** — settings and mobile UX (doc 05 §M6):
  the `apps` settings field (schema v2 + Normalize + migration from
  v1), the Apps-to-unblock UI row in the Svelte page (first-run copy,
  About lines with the key-icon honesty, Help/privacy disclosure per
  doc 07 §5), the diagnostics view, and accessibility/rotation/
  font-scaling checks in the WebView. Host-testable first (the schema
  + Normalize + migration + facade plumbing); the WebView UX halves
  verify through the compile matrix and the M5 APK lane, with the
  device checklist still ⛔ until a KVM-capable host or a real device
  appears.
