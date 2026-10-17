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

- **Phase completed:** Phase 2 milestone **M8-a** — Ali's two product
  rulings, implemented — 2026-10-17, host-complete (DECISIONS.md D83,
  evidence in doc 05 M8-a; sits between M7 and M8 because the answers
  changed shipped behavior before the release freeze). Ruling 1: no
  allow-list default — the list starts empty, the M6 restore-defaults
  rule removed, RoutedApps fails toward empty, the Kotlin establish
  routes the empty list through failStart("NO_TARGET_APPS"), the UI
  shows the empty state and the message covers empty/none-installed;
  saved lists kept. Ruling 2: .ir never touches the app —
  config/direct.go + Server.routedHost (the shared door decision), the
  PAC's first line, no fragmentation/redirect ever, wins over ModeAll
  and the user's list; honest limit stated (.ir lookups still use the
  DoH providers on Android). The latent <queries> bug fixed
  (launcher-intent visibility for user-added packages). **On-device
  halves ⛔** (the standing blocker, re-verified); desktop baseline ✅
  17/17 twice at the exit gate (ModeAll's .ir exclusion is the ruled,
  disclosed desktop behavior change).
- **Branch and commit:** `main`, this session's commit (see `git log -1`
  at handoff time); worktree clean after the commit.
- **Files changed this phase:** new — `internal/config/direct.go`,
  `internal/config/direct_test.go`; additive edits —
  `internal/config/{defaults.go,settings.go,apps_test.go}` (the
  empty-default rule + inverted expectations),
  `internal/proxy/{pac.go,pac_test.go,server.go,socks5.go,behavior_
  test.go,socks5_test.go}` (routedHost + the .ir guard + the door
  proofs), `mobile/{mobile.go,apps_test.go}` (fail-toward-empty),
  `android/.../YounblockVpnService.kt` (the empty-list refusal through
  the typed door) + `AndroidManifest.xml` (launcher-intent queries),
  `frontend/src/lib/{backend.ts,MenuSheet.svelte}` + classic `en.json`
  (the honest copy), doc 04 §9 / 05 (M8-a) / 07 (§1/§5/§6) / 09 / 10,
  README, DECISIONS (D83) / PROGRESS / CHANGELOG.
- **Tests run and actual results:** `go vet ./...` clean; `go test
  -count=1 ./...` **17/17 packages ok, run twice consecutively**;
  `go test -race -count=1` green on the seven key packages (engine
  10.4 s, proxy 17.9 s, mobile 38.3 s); svelte-check 0/0; vite build
  clean + webui dist synced (TestWebUILifecycle green on the new
  bundle); compile matrix **4/4 ABIs** with `-mod=vendor` (arm64 full
  tree; 18 non-main × arm/386/amd64); `gomobile bind` OK — AAR
  25,160,550 B, `javap` shows the nine bindings; clean `assembleDebug`
  OK — app-debug.apk 35,567,561 B, sha256
  ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64,
  badging minSdk 26 / target 36, 3 native ABIs; the dumped manifest
  carries the launcher-intent `<queries>` and exactly doc 07 §6's
  permission inventory; apksigner OK (debug cert).
- **Current architecture decision / confidence:** D83 (no allow-list
  default — the empty list is a first-class state whose safety is the
  M7 refusal door; and the .ir never-processed rule at the one
  decision point both doors share, winning over every mode and list,
  with the DoH-resolution limit stated). Confidence: high on the host
  chain (truth-table + door + PAC suites, the full gates); the device
  halves (the .ir direct path on a real radio included) ride with the
  checklist. Next decision ID: D84.
- **Blocking questions:** one product decision remains — **the
  keystore handling (doc 07 §8), now BLOCKING for M8's signed release
  APK**: who generates the release keystore and where it lives (the
  spec's rule: local, never committed, README backup steps); the
  recommended split — Ali generates his own locally per README steps,
  and the sandbox's M8 release lane is proven with a clearly-labeled
  placeholder he replaces. The allow-list question is ANSWERED and
  implemented (D83). Standing environment blocker: **no emulator (no
  /dev/kvm) and no device** — the shared checklist.
- **Exact next task:** **M8** — package, verify, hand off (doc 05
  §M8): debug + release (signed) APKs with artifact path/ID/version/
  ABI/checksum recorded; install + smoke-test (the device-dependent
  install/launch halves ⛔ without a device — document with evidence);
  doc 10's release notes finalized against the artifact (the M8-a
  copy review already reflects D83); docs/notes updated; commit/push
  with the pre-existing-change rule. The host side (build both
  variants, record everything, finalize docs) is runnable now — the
  release keystore answer is the one open input.
