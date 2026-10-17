# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M8-j round (2026-10-17, the device-test
feedback loop's ninth exchange — the FIRST device feedback arrived
and was answered: debug.5 shipped with the numbers-spacing fix and
the video-start instrumentation) — **Phase 2 host-complete; the test
circle is on its FOURTH build, published as regular releases in the
private repo, and the keystore helper survives PowerShell-less PATH
machines (D90)**. This file is
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

- **Phase completed:** the device-test feedback loop's NINTH
  exchange (2026-10-17, Task ID M8-j) — **the FIRST DEVICE feedback
  from the debug.3 circle, answered with a shipped build**. Ali
  reported: (1) "it worked on android" — the install, first launch
  and routing all work, the feed refreshes fast through the tunnel
  (test-circle rows 1 and 2 PASS, row 3 PARTIAL); (2) the Play
  Store/Play Protect popup at install — expected for a sideloaded
  debug-signed APK, documented (D91); (3) the release keystore was
  created — nothing to do with it now, and it must NOT sign the
  test builds (would break the pinned-cert update chain); (4) **the
  video-start delay: 30 s–1 min+ to show a video while the feed is
  fast** — the host-side suspects eliminated with evidence (ICMP
  fast-fail pinned by test, 1 MiB TCP window, tiny fragment pauses,
  cached DoH, 1024 conn caps, the honest connect gate), the
  remaining suspects are device/network-side, so debug.5
  INSTRUMENTS every tunnel's phases into the diagnostics ring
  (dns/dial/first-byte/dur/bytes); (5) **the numbers-spacing
  complaint** — root-caused (the fixed 22.5 rem stage outgrowing the
  viewport at narrow widths or font scale) and FIXED. Beneath it:
  M8-i-b (keystore done + the debug.4 title fix), the M8-i-a
  recovery round, M8-h/D90, M8-g/D89, M8-e/D88.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.5 / versionCode 4** (built at ad02980, APK
  34,831,081 B, sha256 5df2acc3…02b4, the pinned D85 debug
  certificate — installs over debug.3 in place): the five-app
  default allow-list (D87), the v4 migration, PLUS this round's two
  changes — the pro screen's stage capped to the viewport with
  wider number spacing (the numbers-spacing fix), and per-flow
  tunnel timing (tunnel up dns/dial, tunnel first-byte, tunnel done
  dur/up/down) riding the log ring into the redacted diagnostics
  report, hostnames gated behind LogDomains. The helper facts
  beneath stand: make-release-keystore.bat (13,683 B, sha256
  ee35e456…9558) with the D90 full-path + cmd-fallback resolution;
  the keystore ceremony is COMPLETE on Ali's machine (M8-i-b) —
  nothing further until a signed public release is planned. All the
  android releases are regular releases in the private repo (D89);
  the spec audit (M0–M8 host-complete, every remaining ⛔ a device
  gate) is on record.
- **Branch and commit:** `main`, the M8-j code commit ad02980 (the
  app changes) plus the M8-j docs commit (see `git log -1` at
  handoff time); worktree clean after the commits; the release tag
  `v1.6.6-android-debug.5` points at ad02980 and is the repo's
  Latest; debug.4 → cb28507 (helper-only), debug.3 → 2a8b678 (the
  device-circle APK), debug.2 → 096e8c0, debug.1 → f61bfae
  (superseded; debug.1's page keeps the D85 troubleshooting guide).
- **Files changed this phase:** the app changes (commit ad02980):
  `frontend/src/App.svelte` + `frontend/src/lib/Timer.svelte` (the
  spacing fix), `internal/proxy/tunnel.go` / `server.go` / `socks5.go`
  (the per-flow timing), `internal/proxy/flowtiming_test.go` +
  `internal/tunstack/probe_icmp_test.go` (the new tests),
  `android/app/build.gradle.kts` (versionCode 3 → 4); plus docs (the
  follow-up commit): DECISIONS.md (D91), doc 06 (the M8-j record),
  doc 09, doc 10 (the debug.5 edition + the test-circle rows with
  Ali's evidence), PROGRESS.md, CHANGELOG.md, README, doc 05's M6
  gate note if applicable, SESSION_HANDOFF_PROMPT.md — plus the
  GitHub release lane (debug.5 created regular + Latest + APK).
- **Tests run and actual results:** after the changes, all green:
  `go vet ./...` clean; `go build ./...` clean; `go test -count=1
  ./...` **17/17 packages ok, run twice consecutively** (the new
  tests included: TestUDP443FastFailICMP — the ICMP 3/3 answer with
  the dport-443 quote; TestFlowTimingLines / HostnamesGated /
  NoLoggerNilSafe — the timing lines' format, privacy gating and
  nil-Logger safety); svelte-check 0/0; vite build clean; the webui
  dist CLEAN-synced (the stale hashed assets purged from the embed
  for the first time — file sets identical to frontend/dist; the
  webui+mobile packages re-run green after the sync). The build
  lane: gomobile bind → younblock.aar 24,914,100 B; assembleDebug →
  app-debug.apk 34,831,081 B, sha256 5df2acc3…02b4; aapt2 badging:
  versionCode 4, minSdk 26, targetSdk 36, 3 ABIs; apksigner: the
  pinned c76bb7c6…8551d debug cert. The release lane live through
  the API: debug.5 regular + Latest (id 416019596), the APK asset
  (651411314) served bytes re-downloaded through the API asset
  endpoint and hashed byte-exact. The standing ⛔ re-confirmed:
  `ls /dev/kvm` → No such file or directory — no emulator, no
  device; the video-start delay itself can only be measured on
  Ali's phone (that is what the instrumentation is for). The
  spacing fix was verified through the rendered app (vite preview
  + browser at 320 px/1.4× and 360 px/1×, VLM-checked).
- **The environment is RESTORED and READY** (2026-10-17, M8-i-a):
  ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1 (javac), the
  Android SDK (cmdline-tools 12.0, platform-tools 37.0.1,
  platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358), and
  gomobile/gobind at the pinned x/mobile version (~/go/bin).
  Source `~/sdk/env.sh` before any build. If a future session finds
  ~/sdk wiped again, reinstall per doc 06 §1 (mind the URL
  wrinkle) — but VERIFY first; do not reinstall over a working
  toolchain.
- **Current architecture decision / confidence:** D91 (the first
  device feedback: root-cause what the host can prove — the spacing
  fix; instrument what only the device can measure — the per-flow
  tunnel timing; Play Protect expected; the keystore untouched
  until the signed public release). Confidence: high — every claim
  carries its evidence (the ICMP probe→test, the before/after
  geometry, the byte-exact release lane). Beneath it: D90, D89,
  D88. Next decision ID: D92.
- **Blocking questions:** none product-side. The signed-release
  decision still returns only when a public release is planned —
  its ceremony is automated and documented (D88/D90) and its
  keystore exists on Ali's machine. The standing input is **Ali's
  diagnostics report from a slow-video moment on debug.5** (the
  timing lines name the phase) plus the rest of the test circle
  (rows 4–8, rotation/TalkBack).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.5 (Task ID M8-k) —
  first the video-start timing evidence (Menu → Diagnostics → Copy
  while a video is slow; phone model + Android version; every video
  or just the first after connecting), then the rest of doc 10's
  circle (youtube.com per browser, .ir direct, stop paths, battery,
  network switch). Root-cause what the timing lines name (the
  D77–D91 honesty rules binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.6` through the same release lane — a REGULAR
  release (D89) — WHEN an app code change warrants it. The
  environment is READY (verify with `. ~/sdk/env.sh && go version`
  first; the FIFTH wipe happened MID-SESSION this round — expect
  wipes at any boundary and reinstall per doc 06 §1 + the URL
  wrinkle when they land). Open offer on the table, awaiting Ali's
  ruling: attaching the APK to every release page (relaxing D84's
  no-duplicate discipline) — do NOT do it unilaterally.
