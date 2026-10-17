# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the D87 session (2026-10-17, the device-test
feedback loop's third round) — **Phase 2 host-complete; the test
circle is on its third build**. This file is
the live handoff; keep the fields in §"Current handoff fields" accurate
at every session boundary.

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

- **Phase completed:** the device-test feedback loop's THIRD session
  (2026-10-17, D87, Task ID M8-d) — Ali's second ruling ("Add chrome
  to the default addlist as well, also firefox, opera, brave
  browser") implemented, closing the open question D86 carried.
  Phase 2 (M0–M8) remains host-complete beneath it; no new device
  observations arrived in this round (the ruling was the whole
  input).
- **The project's state now:** the default allow-list is five apps —
  the YouTube app plus the Chrome, Firefox, Opera and Brave browsers
  (D87; the stable mainline package names). The one risk of a
  multi-app default — devices lacking the apps — was audited BEFORE
  shipping: the Kotlin establish loop skips NameNotFound entries per
  package and refuses only when zero resolve, so missing browsers
  cannot break the connect (recorded in D87). A one-time v4 settings
  migration (settingsVersion 3→4) carries every era forward with the
  agency rules intact: v1/v2 born-empty files fill the new default;
  a v3 file still holding EXACTLY the D86 [youtube] default (the
  debug.2 shipped state, never curated) upgrades to the five apps;
  a v3 empty and every custom list survive verbatim. The chip row
  covers all five defaults (+ YouTube / + Chrome / + Firefox / +
  Opera / + Brave) and the friendly-name aliases extend to
  firefox/opera/brave; the honest copy updated everywhere (menu
  intro, empty state, note, How-it-works, the NO_TARGET_APPS
  message, README, doc 07 §1, doc 10). The new build is
  **v1.6.6-android-debug.3** (versionCode 3, the pinned D85
  certificate — installs over debug.1/debug.2 in place, settings
  survive through the migration). The engine/chain behavior is
  unchanged and remains unclaimed device-side pending Ali's next
  round.
- **Branch and commit:** `main`, the M8-d commit (see `git log -1` at
  handoff time); worktree clean after the commit; the release tag
  `v1.6.6-android-debug.3` (prerelease) points at it; debug.2's tag
  points at 096e8c0 and debug.1's at f61bfae (both superseded, their
  release pages keep the D85 troubleshooting guide).
- **Files changed this phase:** internal/config (defaults.go,
  settings.go — settingsVersion 4 + the migration + the isD86Default
  helper; apps_test.go — the D87 matrix), mobile (mobile.go comment;
  apps_test.go incl. the debug.2-file migration test), frontend
  (MenuSheet.svelte — the five-app labels/aliases/chips/copy;
  backend.ts — the NO_TARGET_APPS message), android
  (build.gradle.kts versionCode 3; YounblockVpnService.kt comments),
  README, doc 06 (M8-d record), doc 07 §1, doc 09 (this handoff),
  doc 10 (the debug.3 edition), DECISIONS.md (D87), PROGRESS.md,
  CHANGELOG.md, SESSION_HANDOFF_PROMPT.md.
- **Tests run and actual results:** vet clean; go test 17/17 ×2 (exit
  0 both passes); -race green on config/engine/mobile; svelte-check
  0/0; vite build clean; webui dist synced + the webui suite green
  on the new bundle; compile matrix 4/4 ABIs (18 non-main packages
  each; arm64 links the full tree); gomobile bind (AAR 25,174,619 B,
  javap bindings verified; the D87 copy verified embedded in
  libgojni.so); clean assembleDebug (APK 35,816,233 B, sha256
  81515f0d…d6a1, versionCode 3, pinned cert, badging + manifest
  verified); the GitHub release verified through the API after
  creation (served bytes re-downloaded and hashed).
- **Current architecture decision / confidence:** D87 (the five-app
  default + the v4 migration + the widened chips/aliases). Confidence:
  high host-side — the missing-app risk was audited and closed by the
  M6 establish design before anything shipped. Device-side: the
  routing chain is audited but its on-device behavior (real DoH
  reachability, QUIC fallback, DPI evasion on his radio) is still ⛔
  pending his observations; the diagnostics report (Menu →
  Diagnostics → copy) is the requested evidence if anything still
  fails. Next decision ID: D88.
- **Blocking questions:** none product-side. The default-list
  question D86 left open is CLOSED by the owner's ruling (D87). The
  signed-release decision returns only when a public release is
  planned (doc 07 §8).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.3 (Task ID M8-e) —
  the YouTube app and the installed default browsers out of the box
  (row 2), youtube.com in each installed browser (row 4), the rest
  of doc 10's test circle, and the diagnostics report for anything
  that fails. Fix what surfaces (root-cause discipline; the D77–D87
  honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.4` through the same release lane WHEN a code
  change warrants it.
