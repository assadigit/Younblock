# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M7 → M8 boundary (2026-10-17).
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

- **Phase completed:** Phase 2 milestone **M7** — 2026-10-17, host-
  complete (regression, hardening, release preparation; evidence in
  doc 05 M7). Exit criteria: all host-runnable gates ✅ (the
  NO_TARGET_APPS typed-error door D81 — the gate refusal through the
  engine's ordinary failure path, engine/adapter/facade suites; the
  M5-era spawn-race zombie closed by run generations D82, with the
  test that had codified it fixed to its true intent; the -race lane
  green on the seven key packages; the dependency/license audit with
  the AAR's real closure and the About attribution line; the
  permission review — the built APK's manifest is exactly doc 07 §6's
  inventory, ACCESS_NETWORK_STATE absent; the release-notes draft
  doc 10; doc 02 §7 traceability updated); **on-device halves ⛔**
  (battery/behavior, reboot/revocation AND-028, live
  consent/notification/WebView observation — the standing blocker,
  re-verified); desktop baseline ✅ 17/17 packages, run twice
  consecutively at both the open and exit gates.
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit. The session
  opened with the environment INTACT (the first recorded session
  without a wipe): repo at `57adf1a`, toolchain verified with
  commands, worklog carried over — only the standing blocker
  re-verified.
- **Files changed this phase:** new — `mobile/failstart_test.go`,
  `docs/android-port/10-release-notes.md`; additive edits —
  `internal/engine/platform.go` (PlatformRefusalError + the StartGater
  refusal contract), `internal/engine/health.go` (the connect run's
  refusal branch + the D82 generation registration replacing
  setRunCancel), `internal/engine/engine.go` (mapErrorCode's refusal
  case + runGen + Start's generation capture),
  `internal/platform/android/android.go` (the gate's refuse/latch/
  deliver machinery + RefuseStart), `mobile/mobile.go` (FailStart),
  `frontend/src/lib/backend.ts` (the state error.code surface + the
  NO_TARGET_APPS message), `frontend/src/lib/MenuSheet.svelte` (the
  About attribution line), `android/.../YounblockVpnService.kt`
  (establishTun calls failStart on the zero-targets path),
  `internal/engine/gate_test.go` + `internal/platform/android/
  gate_test.go` + `internal/engine/engine_test.go` (the refusal and
  generation suites; the idempotency test's honest form), doc 05/06/
  07/09, doc 02 §7, README's reading order, DECISIONS/PROGRESS/
  CHANGELOG.
- **Tests run and actual results:** `go vet ./...` clean; `go test
  -count=1 ./...` **17/17 packages ok, run twice consecutively**
  (engine 9.1 s, mobile 33.7 s); `go test -race -count=1` green on
  the seven key packages (engine 10.4 s, android 2.1 s, mobile 38.4 s,
  tunstack 2.3 s, proxy 11.4 s, webui 1.1 s, config 1.0 s);
  svelte-check 0/0; vite build clean; compile matrix **4/4 ABIs**
  with `-mod=vendor`; `gomobile bind` OK (AAR 25,072,133 B, `javap`
  shows failStart); clean `assembleDebug` OK — app-debug.apk
  35,298,149 B, sha256
  f27b7eef014f61eff35405f4406bfe9c33d4e34a918f9949eaad417d561c6491,
  badging minSdk 26 / target 36, 3 native ABIs; the manifest's
  permission set dumped and verified against the inventory; apksigner
  OK (debug cert). One honest regression iteration: D82's honest
  cancel broke TestEngineStartIsIdempotent — root-caused (the test
  codified the zombie), fixed the test to its true intent, full suite
  green twice after.
- **Current architecture decision / confidence:** D81 (the platform
  refusal door: shell→engine typed error through the start gate, the
  phase machine stays the single truth, atomic delivery that survives
  the re-arm race, NOT_HELD on an open gate, the facade's code
  whitelist) and D82 (run generations: every cancel supersedes spawned
  runs; the stale-check and registration are one critical section).
  Confidence: high on the host chain (refusal/generation/race suites
  + the -race lane); the device halves ride with the checklist.
- **Blocking questions:** none architectural. Standing environment
  blocker: **no emulator (no /dev/kvm) and no device** — the shared
  checklist (consent live, notification channel, screen-off/Doze,
  real DoH, QUIC fallback, protect(), reboot/onRevoke, live
  rotation/font-scale/TalkBack, battery measurement). Product
  confirmations still wanted from Ali (non-blocking, defaults
  chosen): allow-list defaults beyond Chrome+YouTube, keystore
  handling (doc 07 §8) — M8 needs the keystore decision to produce
  the signed release APK.
- **Exact next task:** **M8** — package, verify, hand off (doc 05
  §M8): debug + release (signed) APKs with artifact path/ID/version/
  ABI/checksum recorded; install + smoke-test (the device-dependent
  install/launch halves ⛔ without a device — document with evidence);
  doc 10's release notes finalized against the artifact; docs/notes
  updated; commit/push with the pre-existing-change rule. The host
  side (build both variants, record everything, finalize docs) is
  runnable now; the release keystore question for Ali becomes
  blocking for the signed variant.
