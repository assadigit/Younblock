# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the D90 session (2026-10-17, the device-test
feedback loop's sixth round) — **Phase 2 host-complete; the test
circle is on its third build, published as regular releases in the
private repo, and the keystore helper now survives PowerShell-less
PATH machines (D90, shipped as debug.4)**. This file is
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

- **Phase completed:** the device-test feedback loop's SIXTH session
  (2026-10-17, D90, Task ID M8-h) — Ali's first field report from
  the helper arrived: the keystore ceremony stopped at password
  generation ("'powershell' is not recognized") because his PATH
  does not include PowerShell; fixed and shipped as
  v1.6.6-android-debug.4. Beneath it: M8-g/D89 (regular releases in
  the private repo), M8-e/D88 (the ceremony script), the M8-f
  verification round. Phase 2 (M0–M8) remains host-complete; the
  DEVICE observations from debug.3 have still not arrived — the
  helper report was a PC-side observation.
- **The project's state now:** the app is unchanged at the debug.3
  state — the five-app default allow-list (the YouTube app plus the
  Chrome, Firefox, Opera and Brave browsers, D87), the v4 migration,
  the pinned D85 certificate, v1.6.6-android-debug.3/versionCode 3
  as the test-circle build. The helper changed this round (D90):
  `make-release-keystore.bat` (repo root; 13,683 B, sha256
  ee35e456…9558, pure ASCII/CRLF, offline) now resolves
  powershell.exe by its full standard install path (the
  setup-offline.bat pattern shipped since v0.7.5), then where.exe,
  and falls back to a pure-cmd password generator (six RANDOM
  draws, digits and dashes, ~90 bits) when PowerShell cannot be
  found at all — the password step can never dead-end again; the
  .md backup and DONE summary record which generator ran. Ali's
  online-generator question is ruled NO (D90): the signing identity
  never transits a third party. v1.6.6-android-debug.4 is a REGULAR
  release (D89) carrying ONLY the fixed .bat — no APK (the app is
  unchanged, D84; debug.3's APK is the current build, and its page
  carries the pointer note superseding its older helper asset) —
  tagged at the fix commit cb28507, Latest-marked. The D88/D89
  facts beneath it stand: the three android releases are regular
  releases in the private repo; the spec audit (M0–M8 host-complete,
  every remaining ⛔ a device gate) is on record.
- **Branch and commit:** `main`, the M8-h docs commit (see `git log
  -1` at handoff time); worktree clean after the commit; the release
  tag `v1.6.6-android-debug.4` points at the M8-h fix commit cb28507
  (the .bat change — helper-only, no app build);
  `v1.6.6-android-debug.3` still points at the M8-d commit 2a8b678
  (the APK is unchanged — D88 added an asset, D89 flipped flags, D90
  replaced the helper); debug.2's
  tag points at 096e8c0 and debug.1's at f61bfae (both superseded,
  their release pages keep the D85 troubleshooting guide).
- **Files changed this phase:** `make-release-keystore.bat` (the D90
  fix, 57 insertions / 17 deletions) plus docs — doc 10 (the D90
  edition: the status header and §1's debug.4 note), doc 09 (this
  handoff), doc 06 (the M8-h record), doc 07 §8 (the D90 bullet),
  README (the Android download line + the helper paragraph),
  DECISIONS.md (D90), PROGRESS.md, CHANGELOG.md,
  SESSION_HANDOFF_PROMPT.md — plus the GitHub release lane (the
  debug.4 release + asset, the Latest marker, debug.3's
  one-paragraph pointer note). ZERO Go/Kotlin/Svelte/app code
  touched; versionCode stays 3; the APK bytes are unchanged.
- **Tests run and actual results:** no Go/Svelte lanes this session —
  no app code changed, and ~/sdk remains wiped (the standing caution:
  reinstall per doc 06 §1 pins BEFORE any Go/Svelte work). The
  baseline of record for the unchanged clean tree 2a8b678 is M8-d's
  (vet clean; go test 17/17 ×2; svelte-check 0/0; vite build clean,
  dist synced). What WAS verified with evidence this round: the new
  .bat on disk (13,683 B, sha256 ee35e456…9558, pure ASCII, CRLF;
  structural desk-check clean — even quote counts per line, block
  parentheses balanced to zero, the D88 PS one-liner unchanged apart
  from the exe resolution; the full-path pattern itself has shipped
  to real machines in setup-offline.bat since v0.7.5); the release
  lane live through the GitHub API (debug.4 created regular at
  cb28507, Latest-marked; the asset uploaded, its served bytes
  re-downloaded and hashed byte-exact; debug.3's body 9,017 → 9,650
  chars, prefix byte-identical, the pointer note exactly once). The
  Windows execution itself remains ⛔ on this Linux host — the script
  self-verifies on Ali's machine (the read-back round trip).
- **Current architecture decision / confidence:** D90 (the keystore
  ceremony survives PowerShell-less PATH machines — full-path
  resolution + pure-cmd fallback; the online-generator question ruled
  NO: the signing identity never transits a third party). Confidence:
  high for the resolution pattern (shipped precedent + desk-check),
  with the honest ⛔ that the batch execution itself is unrunnable on
  the build host — Ali's next run is the proof. Beneath it: D89
  (regular releases in the private repo), D88 (the local ceremony).
  Next decision ID: D91.
- **Blocking questions:** none product-side. The signed-release
  decision still returns only when a public release is planned — its
  ceremony is automated and documented (D88/D90). The standing
  input remains Ali's DEVICE observations from debug.3 (the helper
  report was PC-side).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's DEVICE results from v1.6.6-android-debug.3 (Task ID
  M8-i) — the YouTube app and the installed default browsers out of
  the box (row 2), youtube.com in each installed browser (row 4), the
  rest of doc 10's test circle, and the diagnostics report for
  anything that fails. The first run of the FIXED helper
  (v1.6.6-android-debug.4) is also expected: its DONE output (which
  generator, serial, SHA-256 fingerprint) closes the D88/D90
  ceremony loop. Fix what surfaces (root-cause discipline; the
  D77–D90 honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.5` through the same release lane — a REGULAR
  release (D89) — WHEN an app code change warrants it.
  CAUTION for the next session: ~/sdk is wiped —
  reinstall the toolchain per doc 06 §1's pins (Go, the Temurin JDK,
  the Android SDK/NDK) and re-establish the baseline BEFORE touching
  any Go/Svelte code.
