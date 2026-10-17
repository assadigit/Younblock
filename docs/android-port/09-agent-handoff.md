# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M8-i-b round (2026-10-17, the
device-test feedback loop's eighth exchange — still PC-side: the
keystore ceremony completed on Ali's machine and the release-page
clarity fix shipped; the DEVICE results remain the open input) —
**Phase 2 host-complete; the test circle is on its third build,
published as regular releases in the private repo, and the keystore
helper survives PowerShell-less PATH machines (D90, shipped as
debug.4)**. This file is
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

- **Phase completed:** the device-test feedback loop's EIGHTH
  exchange (2026-10-17, Task ID M8-i-b — PC-side: the keystore
  question + the release-page confusion). Ali reported the keystore
  ceremony COMPLETED on his machine ("I created a key chain") — the
  D88/D90 loop's first finished run; the DONE output was not pasted
  (if evidence is ever needed: the certificate serial + SHA-256
  fingerprint are the shareable identifiers — never the password).
  His second point was a delivery-UX report: the Latest release
  (debug.4) carries only the helper .bat and its title said
  "(no new APK)" — accurate but not actionable. Fix: the debug.4
  release title retitled through the API to "…the keystore helper
  fixed · no APK here — the app is on debug.3" (make_latest="true"
  on the PATCH; debug.4 stays Latest, verified; tag/body/assets
  unchanged; D84 untouched — the pointer strengthened, no bytes
  duplicated). Beneath it: the M8-i-a recovery round (the toolchain
  restored, baseline green — the environment is READY), M8-h/D90
  (the PowerShell-less-PATH helper fix, debug.4), M8-g/D89 (regular
  releases in the private repo), M8-e/D88 (the ceremony script).
  Phase 2 (M0–M8) remains host-complete; the DEVICE observations
  from debug.3 have STILL not arrived.
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
- **Branch and commit:** `main`, the M8-i-a docs commit (see `git log
  -1` at handoff time); worktree clean after the commit; the release
  tag `v1.6.6-android-debug.4` points at the M8-h fix commit cb28507
  (the .bat change — helper-only, no app build);
  `v1.6.6-android-debug.3` still points at the M8-d commit 2a8b678
  (the APK is unchanged — D88 added an asset, D89 flipped flags, D90
  replaced the helper); debug.2's
  tag points at 096e8c0 and debug.1's at f61bfae (both superseded,
  their release pages keep the D85 troubleshooting guide).
- **Files changed this phase:** documentation only — doc 06 (the
  M8-i part-b record), doc 09 (this handoff), PROGRESS.md,
  SESSION_HANDOFF_PROMPT.md — plus ONE delivery-lane metadata edit
  through the GitHub API: the debug.4 release title retitled to
  point at debug.3's APK. ZERO Go/Kotlin/Svelte/app code touched;
  versionCode stays 3; the APK bytes are unchanged; no release was
  cut; no release bytes changed.
- **Tests run and actual results:** the full restored baseline, run
  BEFORE anything else on the unchanged clean tree f8b2edc:
  `go vet ./...` clean; `go test -count=1 ./...` **17/17 packages ok
  twice consecutively** (engine 9.6 s, proxy 16.2 s, mobile 36.8 s);
  svelte-check 0 errors 0 warnings; vite build clean with the webui
  dist re-synced (worktree stayed clean through the rebuild —
  byte-identical dist); Android library compile matrix 4/4 ABIs in
  the honest §A form (arm64 full tree at CGO_ENABLED=0; the 18
  non-main packages × arm/386/amd64); `./gradlew projects` BUILD
  SUCCESSFUL through the committed wrapper (JDK/SDK detection
  re-proven). The release lane re-verified live through the GitHub
  API (debug.4 regular + Latest; debug.3/.2/.1 regular; zero
  issues). The standing ⛔ re-confirmed: `ls /dev/kvm` → No such
  file or directory — no emulator, no device; and the Windows
  execution of the .bat remains unrunnable on this Linux host (the
  script self-verifies on Ali's machine).
- **The environment is RESTORED and READY** (2026-10-17, M8-i-a):
  ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1 (javac), the
  Android SDK (cmdline-tools 12.0, platform-tools 37.0.1,
  platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358), and
  gomobile/gobind at the pinned x/mobile version (~/go/bin).
  Source `~/sdk/env.sh` before any build. If a future session finds
  ~/sdk wiped again, reinstall per doc 06 §1 (mind the URL
  wrinkle) — but VERIFY first; do not reinstall over a working
  toolchain.
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
  M8-j) — the YouTube app and the installed default browsers out of
  the box (row 2), youtube.com in each installed browser (row 4), the
  rest of doc 10's test circle, and the diagnostics report for
  anything that fails. The keystore is DONE on his side (M8-i-b) —
  nothing more to do with it until a public release is planned.
  Fix what the device data surfaces (root-cause discipline; the
  D77–D90 honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.5` through the same release lane — a REGULAR
  release (D89) — WHEN an app code change warrants it. The
  environment is READY for that cycle (M8-i-a restored it);
  verify with `. ~/sdk/env.sh && go version` before building, and
  only reinstall if ~/sdk is actually wiped again (doc 06 §1 + the
  cmdline-tools URL wrinkle recorded there). Open offer on the
  table, awaiting Ali's ruling: attaching the APK to every release
  page (relaxing D84's no-duplicate discipline) — do NOT do it
  unilaterally.
