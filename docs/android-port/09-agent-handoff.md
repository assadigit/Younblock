# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the D86 session (2026-10-17, the device-test
feedback loop's second round) — **Phase 2 host-complete; the test
circle is on its second build**. This file is
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

- **Phase completed:** the device-test feedback loop's SECOND session
  (2026-10-17, D86, Task ID M8-c) — Ali's ruling ("mAKE the youtube
  default added list") implemented, and his first FUNCTIONAL
  observation ("the first version didnt work, whether in youtube app
  or youtube in chrome") root-caused and fixed. Phase 2 (M0–M8)
  remains host-complete beneath it; debug.1's install had completed
  (download_count 2).
- **The project's state now:** the observation was root-caused BEFORE
  any fix by reading the whole routing chain (tunstack DNS/UDP policy,
  the SOCKS5 door, routedHost, the Kotlin establish) — all sound; the
  failure was the SHELL: debug.1's born-empty list (D83) plus an entry
  path that demanded exact package names, so typing "youtube" was
  refused, the list never grew, every connect took the NO_TARGET_APPS
  refusal, and nothing ever routed. D86 implements the ruling at the
  right layer: DefaultApps = [com.google.android.youtube]; a one-time
  v3 settings migration carries debug.1's saved born-empty files to
  the default (their empty was the shipped default, never a choice; a
  v3 empty stays empty — user agency); the Apps-to-unblock row gains
  one-tap quick-add chips (+ YouTube / + Chrome) and friendly-name
  aliases ("youtube", "chrome"); the honest copy updated everywhere
  (menu sheet, How-it-works, the NO_TARGET_APPS message, README, doc
  07 §1, doc 10). CHROME IS NOT IN THE DEFAULT (the ruling names
  YouTube only) — the + Chrome chip and test-circle row 4 teach the
  one-tap add; adding Chrome to the default is an open product
  question if Ali asks. The new build is **v1.6.6-android-debug.2**
  (versionCode 2, the pinned D85 certificate — installs over debug.1
  in place, settings survive). The engine/chain behavior is unchanged
  and remains unclaimed device-side pending Ali's next round.
- **Branch and commit:** `main`, the M8-c commit (see `git log -1` at
  handoff time); worktree clean after the commit; the release tag
  `v1.6.6-android-debug.2` (prerelease) points at it; debug.1's tag
  still points at f61bfae (superseded, its release page keeps the D85
  troubleshooting guide).
- **Files changed this phase:** internal/config (defaults.go,
  settings.go — settingsVersion 3 + the migration; apps_test.go),
  mobile (mobile.go comment; apps_test.go incl. the debug.1-file
  migration test), frontend (MenuSheet.svelte — chips/aliases/copy;
  backend.ts — the NO_TARGET_APPS message), android (build.gradle.kts
  versionCode 2; YounblockVpnService.kt comments), README, doc 06
  (M8-c record), doc 07 §1, doc 09 (this handoff), doc 10 (the
  debug.2 edition + the DoH-list correction), DECISIONS.md (D86),
  PROGRESS.md, CHANGELOG.md, SESSION_HANDOFF_PROMPT.md.
- **Tests run and actual results:** vet clean; go test 17/17 ×2 (exit
  0 both passes); -race green on config/engine/mobile; svelte-check
  0/0; vite build clean; webui dist synced + TestWebUILifecycle green
  on the new bundle; compile matrix 4/4 ABIs (18 non-main packages
  each; arm64 links the full tree); gomobile bind (AAR 25,162,951 B,
  javap bindings verified); clean assembleDebug (APK 35,716,265 B,
  sha256 9cca38c9…811b, versionCode 2, pinned cert, badging +
  manifest verified); the D86 copy verified embedded in the AAR; the
  GitHub release verified through the API after creation.
- **Current architecture decision / confidence:** D86 (the YouTube
  default + the v3 migration + the usable entry path). Confidence:
  high host-side — the root cause is closed at the exact layer that
  produced it. Device-side: the routing chain is audited but its
  on-device behavior (real DoH reachability, QUIC fallback, DPI
  evasion on his radio) is still ⛔ pending his observations; the
  diagnostics report (Menu → Diagnostics → copy) is the requested
  evidence if anything still fails. Next decision ID: D87.
- **Blocking questions:** none product-side. Open (non-blocking):
  whether Chrome should join the default list (currently one tap
  away). The signed-release decision returns only when a public
  release is planned (doc 07 §8).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.2 — first the
  YouTube app out of the box (row 2), then youtube.com in Chrome via
  the + Chrome chip (row 4), the rest of doc 10's test circle, and the
  diagnostics report for anything that fails. Fix what surfaces
  (root-cause discipline; the D77–D86 honesty rules binding as ever),
  bump `versionCode` and cut `v1.6.6-android-debug.3` through the
  same release lane WHEN a code change warrants it.
