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

- **Phase completed:** Phase 2 milestone **M3** — 2026-10-16 (build
  scaffolding + platform-neutral engine boundary; tasks 1–7 all ✅,
  exit criteria: APK build ✅ verified, bridge ✅ Go-level with
  on-device ⛔, desktop baseline ✅; evidence in doc 05 M3).
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit. The session
  opened from a **fresh clone at `e2986ba`** — the sandbox rehydrated
  without the repo AND without `/home/z/my-project/worklog.md` (the
  Task 1–3 entries were lost); reconciled by re-cloning, reinstalling
  Go 1.27.2 to `~/sdk/go`, and re-running the baseline before any code
  change. The worklog was recreated with this session's entry.
- **Files changed this phase:** new — `internal/platform/android/`
  (adapter + tests), `internal/platform/select_android.go`,
  `internal/proxy/socks5.go` + `socks5_test.go`, `mobile/` (facade +
  tests), `android/` (Gradle shell: wrapper, manifest, MainActivity,
  resources, network security config), `build-android.sh`; additive
  edits — `internal/platform/select_linux.go` + `select_other.go`
  (build-constraint terms only), `internal/proxy/server.go` (SOCKS5
  door fields + Stop teardown), `internal/webui/webui.go` (Events(),
  real Close()), `.gitignore` (android rules), `go.mod`/`go.sum`/
  `vendor/` (x/mobile tool directive, vendored). No desktop behavior
  changed; verified by the full regression gate.
- **Tests run and actual results:** `go vet ./...` clean;
  `go test -count=1 ./...` **16/16 packages ok** (13 pre-existing +
  platform/android + mobile + the root package already counted;
  engine 8.0 s, proxy 9.7 s); `-race` clean on the SOCKS5 and mobile
  suites; svelte-check 0/0; vite build clean; `check-platform-neutral`
  OK; cross-builds windows/amd64+386, linux/amd64, android/arm64 all
  OK; `gomobile bind` OK (AAR verified: 3 ABIs, javap'd classes);
  `assembleDebug` OK — `app-debug.apk` 27,172,909 B, badging/apksigner/
  sha256 verified (doc 05 M3 item 6).
- **Current architecture decision / confidence:** unchanged ADRs
  0003–0012; D71–D74 added (select wiring correction under the
  `android` build tag; the webui control surface on the facade;
  gomobile-vs-vendor-mode; the M3 shell pins + the permission-skeleton
  decisions). Confidence: high on the bridge and build lane (all
  verified), medium on runtime integration as before — the on-device
  gates (protect-on-device, stack selection, QUIC fallback) are M4's
  to prove.
- **Blocking questions:** none architectural. Standing environment
  blocker: **no emulator (no /dev/kvm) and no device** — blocks the
  launch half of M3's exit criteria (already carried above) and all
  M4 on-device gates. Product confirmations wanted from Ali
  (non-blocking, defaults chosen): target SDK level (36 shipped;
  trivially changeable), allow-list defaults beyond Chrome+YouTube,
  keystore handling (doc 07 §8).
- **Exact next task:** **M4** — `YounblockVpnService` + TUN with the
  per-app allow-list and virtual DNS (ADR-0009/0010), the userspace
  stack selection behind the measured ADR-0008 criteria, stack →
  SOCKS5 door integration, DNS interceptor → DoH, and the on-device
  protect-hook proof. If the environment still lacks an emulator/
  device at M4 time, build and host-test everything (the Go seams are
  all in place: `Server.StartSocks5`, `platform.DialControl`, the
  facade's protector) and report the device gates ⛔ with evidence —
  do not fake them.
