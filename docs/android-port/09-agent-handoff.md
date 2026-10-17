# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8 close (2026-10-17) — **Phase 2
host-complete**. This file is the live handoff; keep the fields in
§"Current handoff fields" accurate at every session boundary.

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

- **Phase completed:** Phase 2 milestone **M8** — package, verify,
  hand off — host-complete 2026-10-17 (DECISIONS.md D84; evidence in
  doc 05 §M8 and doc 06's M8 record). **M0–M8 are all complete
  host-side; the port is packaged and distributed.** The keystore
  ruling arrived at the open and reshaped M8: the debug-APK-only test
  circle — no signed variant exists; the debug APK
  (`app-debug.apk`, 35,567,561 B, SHA-256 ccf46a2e…7b64 — the M8-a
  clean-build output, re-verified on disk: apksigner v2 OK with the
  debug cert digest recorded, badging 1.6.6/vc1/minSdk 26/target 36/
  3 ABIs, the four-permission manifest, javap bindings) is published
  as the GitHub prerelease **`v1.6.6-android-debug.1`** for Ali's own
  device testing. No application code changed at M8 (docs + release
  only); the baseline gate (vet clean, 17/17 ×2, svelte-check 0/0,
  vite clean, dist synced) ran at the open and stands.
- **The project's state now:** every host-runnable lane is green and
  evidenced; every device-side gate is ⛔ in the sandbox (no /dev/kvm,
  no device — re-verified) and **handed to Ali's device** through the
  release's test circle (doc 10 §"Device test circle"): install +
  launch smoke, the live consent dialog, notification channel
  behavior, screen-off/Doze, real DoH, the YouTube app's QUIC→TCP
  fallback, protect() on real sockets, the .ir direct path on a real
  radio, rotation/font-scale/TalkBack, battery, reboot/onRevoke, and
  the IPv4/IPv6 + TCP/UDP measured rows. His observations are the
  next session's input and become the recorded evidence that closes
  the checklist.
- **Branch and commit:** `main`, the M8 commit (see `git log -1` at
  handoff time); worktree clean after the commit; the release tag
  `v1.6.6-android-debug.1` (prerelease) points at it.
- **Files changed this phase:** no application code — doc 05 §M8 +
  critical-path, doc 06's M8 record, doc 07 §8/§9 (D84 + three M8
  rows closed), doc 09 (this handoff), doc 10 (the debug test-circle
  edition + the device test circle), README (the Android sideload
  section), DECISIONS.md (D84), PROGRESS.md, CHANGELOG.md,
  SESSION_HANDOFF_PROMPT.md (the post-M8 edition).
- **Tests run and actual results:** `go vet ./...` clean; `go test
  -count=1 ./...` **17/17 packages ok, run twice consecutively**;
  svelte-check 0/0; vite build clean; webui dist synced (zero diff);
  artifact lanes re-run on the shipped APK (sha256, apksigner
  --print-certs, aapt2 badging, manifest dump, javap — outputs
  recorded in doc 06's M8 record); dependency closure re-run (same
  audited set as M7); platform requirements re-verified against
  developer.android.com primary sources. The GitHub release verified
  through the API after creation (tag, prerelease flag, asset size +
  digest match).
- **Current architecture decision / confidence:** D84 (the
  debug-APK-only test circle — the debug APK distributed via GitHub
  Releases on the private repo; successive debug builds bump
  versionCode; the signed-release alternatives stay documented for
  the future public release). Confidence: high on the entire host
  chain; every device claim remains explicitly unclaimed pending
  Ali's observations. Next decision ID: D85.
- **Blocking questions:** none product-side. The keystore question is
  ANSWERED (D84 — debug-only for now; revisit only when a public
  release is planned). The allow-list question is ANSWERED and
  implemented (D83, confirmed by Ali at M8). Standing environment
  blocker: no /dev/kvm, no device — now mitigated by Ali's own
  device testing through the debug release.
- **Exact next task:** the **device-test feedback loop** (no fixed
  milestone number): ingest Ali's observations from
  `v1.6.6-android-debug.1` (doc 10 §"Device test circle" is the
  checklist), fix what his testing surfaces (root-cause discipline,
  the D77–D83 honesty rules binding as ever), bump `versionCode` and
  cut the next debug build (`v1.6.6-android-debug.2`…) with the same
  release lane (doc 06's M8 record), and close checklist rows with
  his measured evidence. When Ali decides to release publicly, the
  signed-release decision returns (doc 07 §8: his local keystore or
  an EC key; never committed).
