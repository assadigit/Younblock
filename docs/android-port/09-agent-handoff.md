# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the first device-observation session (2026-10-17,
D85) — **Phase 2 host-complete; the test circle is live**. This file is
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

- **Phase completed:** the device-test feedback loop's FIRST session
  (2026-10-17, D85) — Ali's first observation ingested ("Android
  didn't let me install it"), diagnosed with evidence, and the fixes
  shipped. Phase 2 (M0–M8) remains host-complete beneath it.
- **The project's state now:** the observation was diagnosed BEFORE
  any fix: the published artifact was exonerated end-to-end (the
  bytes GitHub serves were re-downloaded this session and hash to the
  recorded sha256 ccf46a2e…7b64; apksigner v2 OK with the recorded
  debug cert c76bb7c6…8551d; badging minSdk 26/target 36/vc 1; the
  binary manifest has debuggable=true but NO testOnly; the zip is
  intact — 20 entries, testzip clean), and the release asset's
  download_count was **0** — the probable cause is an interrupted
  download on Ali's phone (GitHub release assets are served from
  release-assets.githubusercontent.com, frequently unreliable from
  Iranian networks), with the two expected gates
  (install-unknown-apps; Play Protect) or a sub-8.0 device as the
  remaining branches. Three things shipped: (1) the ordered
  troubleshooting path is now ON THE LIVE RELEASE BODY (PATCHed via
  the API, HTTP 200 — the help sits where the retry happens) and in
  doc 10 §1; (2) **D85 — the pinned debug keystore**: the exact key
  that signed debug.1 is vendored at `android/app/debug.keystore` and
  wired as the debug signingConfig, making the "successive debug
  builds install over this one (same certificate)" promise
  wipe-proof (verified: clean assembleDebug → same cert digest, and
  the rebuild is byte-identical, sha256 match); (3) no debug.2 was cut
  — nothing about the artifact changed, so a re-release would churn
  tags without fixing a device/network-side cause. Awaiting from Ali:
  the retry result (the file size he gets + the exact on-screen
  wording if it still fails) and the rest of the test-circle rows
  (doc 10 §"Device test circle").
- **Branch and commit:** `main`, the D85 session commit (see
  `git log -1` at handoff time); worktree clean after the commit; the
  release tag `v1.6.6-android-debug.1` (prerelease) still points at
  the M8 commit f61bfae — the release body was amended in place, the
  asset unchanged.
- **Files changed this phase:** `android/app/build.gradle.kts` (the
  pinned debug signingConfig), `android/app/debug.keystore` (NEW —
  the vendored test-circle key), DECISIONS.md (D85), doc 09 (this
  handoff), doc 10 (the troubleshooting section + the D85 pin note),
  doc 06 (the D85 build-lane record), PROGRESS.md, CHANGELOG.md,
  SESSION_HANDOFF_PROMPT.md. No Go, Kotlin or frontend code changed.
- **Tests run and actual results:** the artifact lanes on the SERVED
  release bytes (download → sha256 match; apksigner verify +
  --print-certs; aapt2 badging + manifest dump) — all green; the
  keystore pinning lane (clean assembleDebug 37/37 tasks, BUILD
  SUCCESSFUL; apksigner on the rebuild → the same certificate digest;
  byte-identical sha256 vs the release); the exit gates (vet clean,
  go test 17/17 ×2, svelte-check 0/0, vite build clean, webui dist
  zero-diff) — see the session worklog for the evidence.
- **Current architecture decision / confidence:** D85 (the pinned
  debug keystore + the first observation's diagnosis). Confidence:
  high — the artifact chain is verified end-to-end and the update
  promise is now wipe-proof. Every device claim remains explicitly
  unclaimed pending Ali's observations. Next decision ID: D86.
- **Blocking questions:** none product-side. The keystore question is
  ANSWERED (D84 — debug-only for now; the debug key is pinned per
  D85; release signing returns only for the public release). The
  allow-list question is ANSWERED and implemented (D83, confirmed by
  Ali at M8). Standing environment blocker: no /dev/kvm, no device —
  mitigated by Ali's own device testing through the debug release;
  the first install attempt surfaced a network-side failure mode
  instead (the truncated-download branch, D85).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's retry result and the remaining test-circle rows
  (doc 10 §"Device test circle" is the checklist), fix what they
  surface (root-cause discipline, the D77–D85 honesty rules binding
  as ever), bump `versionCode` and cut the next debug build
  (`v1.6.6-android-debug.2`…) through the same release lane (doc 06's
  M8 record) WHEN a code change warrants it, and close checklist rows
  with his measured evidence. When Ali decides to release publicly,
  the signed-release decision returns (doc 07 §8: his local keystore
  or an EC key; never committed).
