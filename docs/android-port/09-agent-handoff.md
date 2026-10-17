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

- **Phase completed:** the device-test feedback loop's FOURTH session
  (2026-10-17, D88, Task ID M8-e) — Ali's script ruling: "Can you fix
  the serial number and etc issue as well? like giving me a bat file
  that auto creates it's password locally on my system and backs up
  that password as an md file. if you insist it must be done locally.
  at least you can create the script." The local release-keystore
  ceremony is now a shipped script. Phase 2 (M0–M8) remains
  host-complete beneath it; STILL no device observations from debug.3
  have arrived.
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
  signed-lane steps.
- **Branch and commit:** `main`, the M8-e commit (see `git log -1` at
  handoff time); worktree clean after the commit; the release tag
  `v1.6.6-android-debug.3` still points at the M8-d commit 2a8b678
  (the APK is unchanged — D88 added an asset, not a build); debug.2's
  tag points at 096e8c0 and debug.1's at f61bfae (both superseded,
  their release pages keep the D85 troubleshooting guide).
- **Files changed this phase:** make-release-keystore.bat (new, repo
  root), .gitignore (the release-keystore/backup patterns), README
  (the signed-lane section — closing the hole where doc 07 §8 pointed
  at "README backup steps" that did not exist), doc 06 (M8-e record),
  doc 07 §8 (D88), doc 09 (this handoff), doc 10 (the helper-asset
  note), DECISIONS.md (D88), PROGRESS.md, CHANGELOG.md,
  SESSION_HANDOFF_PROMPT.md. ZERO Go/Kotlin/Svelte/app code touched;
  versionCode stays 3; the APK bytes are unchanged.
- **Tests run and actual results:** no Go/Svelte lanes this session —
  no app code changed, and the ~/sdk toolchain was found WIPED at the
  session open (Go 1.27.2, the Temurin JDK, the Android SDK/NDK all
  gone; the repo, worklog and Next.js scaffold survived). The
  baseline of record for the unchanged clean tree 2a8b678 is M8-d's
  (vet clean; go test 17/17 ×2; svelte-check 0/0; vite build clean,
  dist synced). What WAS verified with evidence: the real keytool
  flow on the host's system JRE 21.0.12.1 keytool (genkeypair
  prompt-free, exit 0, valid to Oct 2056; the -list -v line shapes
  match the script's findstr parsing; a wrong storepass exits 1);
  the password alphabet exactly 64 unique characters with the
  algorithm simulated end-to-end; the .bat pure ASCII, 253/253 CRLF,
  byte-identical through repo/preview/release surfaces (the served
  asset re-downloaded and hashed). ⛔ the Windows batch execution
  itself (Linux host) — the script self-verifies on its first run on
  Ali's machine.
- **Current architecture decision / confidence:** D88 (the local
  keystore ceremony script; the security boundary — signing secrets
  are generated where they are used — kept and made concrete).
  Confidence: high on the keytool flow (real-tool evidence); the
  batch wrapper is audited but Windows-unverified by the host — its
  own read-back verification and per-failure messages carry the proof
  onto Ali's machine. Next decision ID: D89.
- **Blocking questions:** none product-side. The signed-release
  decision still returns only when a public release is planned — but
  its ceremony is now automated and documented (D88). The standing
  input remains Ali's device observations from debug.3.
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.3 (Task ID M8-f) —
  the YouTube app and the installed default browsers out of the box
  (row 2), youtube.com in each installed browser (row 4), the rest
  of doc 10's test circle, and the diagnostics report for anything
  that fails. Fix what surfaces (root-cause discipline; the D77–D88
  honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.4` through the same release lane WHEN a code
  change warrants it. CAUTION for the next session: ~/sdk is wiped —
  reinstall the toolchain per doc 06 §1's pins (Go, the Temurin JDK,
  the Android SDK/NDK) and re-establish the baseline BEFORE touching
  any Go/Svelte code.
