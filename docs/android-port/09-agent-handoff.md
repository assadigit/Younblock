# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the D89 session (2026-10-17, the device-test
feedback loop's fifth round) — **Phase 2 host-complete; the test
circle is on its third build, now published as regular releases in
the private repo**. This file is
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

- **Phase completed:** the device-test feedback loop's FIFTH session
  (2026-10-17, D89, Task ID M8-g) — Ali's release-flag ruling: "Do not
  prelease , just release, the repo is private. Please finish all of
  your specs first." Two rounds sit beneath it: M8-e/D88 (the local
  keystore ceremony script) and the M8-f verification round (zero
  repo changes — every delivery surface re-verified with evidence).
  Phase 2 (M0–M8) remains host-complete beneath it all; STILL no
  device observations from debug.3 have arrived.
- **The project's state now:** the app is unchanged at the debug.3
  state — the five-app default allow-list (the YouTube app plus the
  Chrome, Firefox, Opera and Brave browsers, D87), the v4 migration,
  the pinned D85 certificate, v1.6.6-android-debug.3/versionCode 3
  as the test-circle build. New this round (D88):
  `make-release-keystore.bat` (repo root + a second asset on the
  debug.3 release; 11,527 B, sha256 82e6dded…4090, pure ASCII/CRLF,
  offline) — the one-double-click local ceremony for the future
  signed lane: locates keytool, generates the password on Ali's
  machine with Windows' CSPRNG (an exact-64 alphabet, zero bias),
  creates the keystore with every keytool prompt pre-filled (RSA
  4096, PKCS12, 10950 days), verifies it by reading it back (a wrong
  password = keytool exit 1, proven on the real tool), and writes
  younblock-keystore-backup_<date>.md (password, alias, certificate
  serial number, SHA-256 fingerprint, the full keytool record). The
  .gitignore guards the generated files; the README documents the
  signed-lane steps. New this round (D89): the three android releases
  (debug.1/.2/.3) are now REGULAR releases — prerelease off — in the
  private repo, and debug.3 is the repo's Latest release; switched and
  verified through the GitHub API (tags, assets and digests unchanged;
  the debug.3 body gained exactly one honest D89 note so the flag
  cannot be misread as a stability claim). The spec audit Ali asked
  for ran first: M0–M8 all complete host-side, every remaining ⛔ is a
  device gate that needs his physical device.
- **Branch and commit:** `main`, the M8-g commit (see `git log -1` at
  handoff time); worktree clean after the commit; the release tag
  `v1.6.6-android-debug.3` still points at the M8-d commit 2a8b678
  (the APK is unchanged — D88 added an asset, not a build, and D89
  flipped release flags, not bytes); debug.2's
  tag points at 096e8c0 and debug.1's at f61bfae (both superseded,
  their release pages keep the D85 troubleshooting guide).
- **Files changed this phase:** docs only — doc 10 (the D89 edition:
  the §1 status, the release-flag note, the superseded-builds flag
  note), doc 09 (this handoff), doc 06 (the M8-f/M8-g records), doc 07
  §8 (the D89 bullet), README (the Android download line),
  DECISIONS.md (D89), PROGRESS.md, CHANGELOG.md,
  SESSION_HANDOFF_PROMPT.md — plus the GitHub release metadata (three
  flags, debug.3's Latest marker, its one-paragraph body note). ZERO
  Go/Kotlin/Svelte/app code touched; versionCode stays 3; the APK
  bytes are unchanged.
- **Tests run and actual results:** no Go/Svelte lanes this session —
  no app code changed, and ~/sdk remains wiped (the M8-e caution
  stands: reinstall per doc 06 §1 pins BEFORE any Go/Svelte work).
  The baseline of record for the unchanged clean tree 2a8b678 is
  M8-d's (vet clean; go test 17/17 ×2; svelte-check 0/0; vite build
  clean, dist synced). What WAS verified with evidence this round:
  the GitHub API state before (three android releases,
  prerelease=true each) and after (prerelease=false; tags
  f61bfae/096e8c0/2a8b678 unchanged; every asset and sha256 digest
  unchanged; /releases/latest → v1.6.6-android-debug.3); the debug.3
  body re-fetched — the D89 note present exactly once, every other
  byte identical to the pre-edit body (8,579 → 9,017 chars). The
  M8-f round's evidence (commit/push/release/preview surfaces all
  verified live, byte-exact) stands beneath it.
- **Current architecture decision / confidence:** D89 (the android
  circle ships as regular releases in the private repo; the prerelease
  flag carried no public meaning there, and the honest debug-signed
  test-build copy lives in the release bodies, not the flag — D88's
  ceremony script stands beneath it for the future signed lane).
  Confidence: high — the change is API metadata, verified before and
  after. Next decision ID: D90.
- **Blocking questions:** none product-side. The signed-release
  decision still returns only when a public release is planned — its
  ceremony is automated and documented (D88). The standing
  input remains Ali's device observations from debug.3.
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.3 (Task ID M8-h) —
  the YouTube app and the installed default browsers out of the box
  (row 2), youtube.com in each installed browser (row 4), the rest
  of doc 10's test circle, and the diagnostics report for anything
  that fails. Fix what surfaces (root-cause discipline; the D77–D89
  honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.4` through the same release lane — as a
  REGULAR release now (D89) — WHEN a code change warrants it.
  CAUTION for the next session: ~/sdk is wiped —
  reinstall the toolchain per doc 06 §1's pins (Go, the Temurin JDK,
  the Android SDK/NDK) and re-establish the baseline BEFORE touching
  any Go/Svelte code.
