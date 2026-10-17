# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M6 → M7 boundary (2026-10-17).
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

- **Phase completed:** Phase 2 milestone **M6** — 2026-10-17, host-
  complete (settings and mobile UX; evidence in doc 05 M6). Exit
  criteria: all host-runnable gates ✅ (the `apps` settings field D79 —
  schema v2 + Normalize validation/dedupe/cap + the v1→v2 migration
  through the existing version mechanism, with the all-invalid→defaults
  fail-closed rule so an empty allow-list can never persist; the facade
  plumbing — the engine's locked Settings() accessor and
  mobile.RoutedApps(), one normalized truth behind settings.json,
  GetSettings/SaveSettings and the Kotlin service's establish-time
  read, apply-on-next-connect semantics with the UI copy saying so;
  the Kotlin consumption — readRoutedApps fails closed, DEFAULT_APPS
  deleted so no second copy can drift; the WebView UX D80 — the
  Apps-to-unblock row, the VPN-slot/key-icon About lines, the doc 07
  §5 privacy disclosure, the diagnostics view, all capability-gated on
  AppInfo so desktop renders byte-identical sections); **on-device
  observation ⛔** (no /dev/kvm, no device — the standing blocker,
  re-verified this session; the shared M4+M5 device checklist now also
  carries the live-WebView halves: real rotation, font scaling,
  TalkBack, the apps row on a real screen); desktop baseline ✅ 17/17
  packages, run twice consecutively at both the open and the exit
  gates.
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit. The session opened
  after a **full sandbox wipe** (the third recorded one — the repo,
  ~/sdk toolchain and the worklog were all gone): everything was
  recovered from the recorded state — re-clone from the remote (token
  from the session prompt) at `4083e76`, toolchain reinstalled per doc
  06's pins (Go 1.27.2, JDK 21.0.12.1+1 Temurin with javac, cmdline-
  tools 12.0, platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358,
  gomobile/gobind at the pinned x/mobile), worklog recreated from the
  prompt + repo records, and the desktop baseline re-verified GREEN
  before any M6 code was written.
- **Files changed this phase:** new — `internal/config/apps_test.go`,
  `mobile/apps_test.go`; additive edits — `internal/config/defaults.go`
  (defaultRoutedApps + DefaultApps), `internal/config/settings.go`
  (settingsVersion 2, the Apps field, normalizeApps +
  validAndroidPackage + the maxApps cap), `internal/engine/engine.go`
  (the locked Settings() copy accessor), `mobile/mobile.go`
  (RoutedApps), `frontend/src/lib/backend.ts` (Settings type;
  getSettings/saveSettings/getDiagnosticReport on the Wails, HTTP and
  preview transports; AppInfo platform/capabilities),
  `frontend/src/lib/MenuSheet.svelte` (the M6 sections: the apps row,
  the Android About/How-it-works/Help variants, the privacy section,
  the diagnostics view — capability/platform-gated), `frontend/src/
  App.svelte` (platform + perAppRouting props), `android/.../
  YounblockVpnService.kt` (readRoutedApps at establish, fail-closed,
  DEFAULT_APPS deleted). No desktop behavior changed: the pro sheet's
  desktop sections render the exact prior copy, the classic screen is
  untouched, and the engine's Start path is unmodified (verified by
  the full regression gate run twice).
- **Tests run and actual results:** `go vet ./...` clean; `go test
  -count=1 ./...` **17/17 packages ok, run twice consecutively** at
  both the open and exit gates (engine 8.9 s, proxy 9.6 s, mobile
  33.7 s, tunstack 4.7 s); svelte-check 0/0; vite build clean;
  Android library compile matrix **4/4 ABIs** with `-mod=vendor` (the
  18 non-main packages per ABI; arm64 links the full tree at
  CGO_ENABLED=0); `gomobile bind` OK (AAR 25,097,538 B, `javap` shows
  routedApps); clean `assembleDebug` OK — app-debug.apk 35,194,757 B,
  sha256
  5669efd14a0a523249e55c9f2c7ab0d6b1edb62156c60d76c3ccc38cc59302dc,
  badging minSdk 26 / target 36, 3 native ABIs; apksigner OK (debug
  cert). One honest build iteration recorded: the first clean
  assembleDebug failed at compileDebugKotlin (Kotlin call syntax +
  a missing import — both errors printed, fixed, re-run CLEAN).
- **Current architecture decision / confidence:** D79 (the `apps`
  settings field — schema v2, one list behind three doors, empty-or-
  all-invalid restores the built-in defaults so the route-everything
  state can never persist, apply-on-next-connect with the read fresh
  at establish) and D80 (the M6 WebView UX — capability-gated honesty
  surfaces; the saved response is the validation authority; desktop
  byte-identical). Confidence: high on the host-verified chain (the
  schema/migration/facade tests run the real Normalize and the real
  engine; the build lanes verify the binding and the APK); medium on
  device behavior (the apps row in a real WebView, rotation/font-scale/
  TalkBack, the allow-list's real routing behavior per configured app)
  — all ride with the shared device checklist.
- **Blocking questions:** none architectural. Standing environment
  blocker: **no emulator (no /dev/kvm) and no device** — blocks the
  shared on-device checklist (live consent flow, notification
  behavior, screen-off persistence, real DoH through the stack, QUIC
  fallback, protect() on real sockets, reboot/revocation, plus M6's
  live-WebView observation halves). Product confirmations still wanted
  from Ali (non-blocking, defaults chosen): allow-list defaults beyond
  Chrome+YouTube (the list is now user-configurable — the M6 answer),
  keystore handling (doc 07 §8). Deferred to M7 by decision: the
  NO_TARGET_APPS typed-error surface (the shell-level failure today
  logs and fails closed honestly; a shell→engine error door is a
  D78-class design question that belongs with the hardening).
- **Exact next task:** **M7** — regression, hardening, release
  preparation (doc 05 §M7): the full test matrix and desktop
  regressions; the dependency/license audit re-run (doc 07 §7);
  permission review; battery/behavior measurement on a real device;
  reboot/revocation checks (AND-028) — the device-dependent halves
  ⛔ until a KVM-capable host or a real device appears; the host side
  (matrix re-run, audits, the NO_TARGET_APPS door decision, release
  prep) is runnable now.
