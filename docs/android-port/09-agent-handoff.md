# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the Phase 1 → Phase 2 boundary (2026-10-16).
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

- **Phase completed:** Phase 2 milestone **M4** — 2026-10-16, host-
  complete (the routing path: userspace stack + facade TUN lifecycle +
  VpnService Kotlin shell; evidence in doc 05 M4). Exit criteria: all
  host-runnable gates ✅ (tunstack 10 tests incl. the full through-
  path against the real proxy Server; the facade suite through the
  real engine incl. protect() call-counting through the door; Kotlin
  compiled + verified APK 34,707,469 B); **on-device end-to-end ⛔**
  (no /dev/kvm, no device — the standing blocker, re-verified this
  session); desktop baseline ✅ 17/17 packages.
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit. The session
  opened at `main @ 1d20460` with **M4 work in progress and
  uncommitted** (internal/tunstack, go.mod/go.sum with gVisor) after
  yet another sandbox reset that wiped the toolchain and
  `/home/z/my-project/worklog.md` but left the working tree; the
  uncommitted work was reviewed, completed (one compile bug in the
  domain-flows test, the debug dump file deleted, the flaky
  stop-while-active test made deterministic), and verified from
  scratch (toolchain + module cache + vendor + baseline reinstalled
  before any new code). The worklog was recreated with this session's
  entry.
- **Files changed this phase:** new — `internal/tunstack/` (stack,
  wire, SOCKS5 client half, virtual DNS, 10 host tests), `mobile/
  tun.go` + `tun_test.go` (StartTun/StopTun/TunStats),
  `mobile/nonblock_unix.go` + `nonblock_other.go`,
  `android/.../YounblockVpnService.kt`, `android/.../Bridge.kt`
  (EngineEvents + VpnProtector); additive edits — `mobile/mobile.go`
  (the tun field + Shutdown order + package doc),
  `android/.../MainActivity.kt` (connect flow: consent → service),
  `AndroidManifest.xml` (the service element), `strings.xml`
  (notification), `go.mod`/`go.sum`/`vendor/` (gVisor pinned +
  vendored, D68/D75). No desktop behavior changed; verified by the
  full regression gate.
- **Tests run and actual results:** `go vet ./...` clean;
  `go test -count=1 ./...` **17/17 packages ok** (16 pre-existing +
  tunstack; engine 8.3 s, proxy 9.7 s, mobile 9.8 s, tunstack 4.7 s —
  the flaky first run of TestStopWhileActive was fixed, not retried
  away: buffered-echo draining + the no-path-after-Stop assertion);
  svelte-check 0/0; vite build clean; Android library compile matrix
  4/4 ABIs with `-mod=vendor`; `gomobile bind` OK (AAR 24,893,234 B);
  `assembleDebug` OK — app-debug.apk 34,707,469 B, badging/apksigner/
  sha256 verified (doc 05 M4 item 5).
- **Current architecture decision / confidence:** ADR-0008's gated
  selection **resolved to gVisor netstack direct** (D75 — design-level
  evidence: the tun2socks engine package cannot talk to a CONNECT-only
  door); D76 records the facade's TUN contract (O_NONBLOCK before
  os.NewFile or Stop deadlocks; the fd crosses the bridge exactly
  once; one stack per engine on-phase; TUN_ACTIVE is the loud refusal).
  Confidence: high on the host-verified chain (both stack and facade
  suites exercise the REAL proxy Server/engine, not mocks); medium on
  device behavior (the fdbased-vs-channel syscall question, real DoH
  through the stack, QUIC fallback, protect() on real sockets) — all
  ride with the M5 device checklist.
- **Blocking questions:** none architectural. Standing environment
  blocker: **no emulator (no /dev/kvm) and no device** — blocks M4's
  on-device end-to-end gate (carried in doc 05 M4 item 6) and will
  block M5's lifecycle/device tests. Product confirmations wanted from
  Ali (non-blocking, defaults chosen): target SDK level (36 shipped),
  allow-list defaults beyond Chrome+YouTube (M6 makes the list
  configurable), keystore handling (doc 07 §8).
- **Exact next task:** **M5** — core connection lifecycle and health
  reporting (doc 05): the full consent flow (explain → prepare →
  dialog → denial copy), the foreground-service state machine on top
  of the M4 service (notification honesty, onRevoke, reconciliation,
  no duplicate starts), partial-start failure injection at every step,
  repeated connect/disconnect, network switching. Fold in M4's device
  checklist whenever a KVM-capable host or a device appears (real DoH
  through the stack, QUIC fallback on the real YouTube app, protect()
  on real sockets). If the environment still lacks an emulator/device,
  build and host-test everything and report the device gates ⛔ with
  evidence — do not fake them.
