# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-o (2026-10-18, later evening — **Ali's
debug.7 device round: the stall moved past the M8-n fix — feed and UI
instant, but EVERY video 1–2 minutes to start and re-buffering
mid-play, Windows instant on the same ISP; root-caused to the
surviving amplifier (the 120 s DNS cache serving a blackholed answer
set to every retry), FIXED as ADR-0014, and shipped as
`v1.6.6-android-debug.8` (versionCode 7, the APK asset now NAMED
v1.6.6-debug.apk per his D99 ruling)**). Earlier the same day the account-gate lane ran and closed: A0 (the
`docs/auth/` spec pack), A0-R2 (D95/D96), A0-R3 — **the account gate
is CANCELLED (D97)**, nothing of it was ever built. Before that,
Phase 2 completed host-side; the keystore helper survives
PowerShell-less PATH machines (D90). This file is the live handoff;
keep the fields in §"Current handoff fields" accurate at every session
boundary.

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

- **Phase completed:** M8-o (2026-10-18, later evening — the TWELFTH
  feedback-loop exchange; Ali's device round on debug.7). His inputs:
  the stall PERSISTED past the ADR-0013 fix and SHARPENED ("youtube
  feed refreshes, but it takes a while until first video… even after
  1-2 mins of first video to be played, every other video has same
  buffer problem… apk is laggy and buffers a lot, but in UI-wise it
  connects as quickly… same ISP and net connection works instantly
  using windows version") — plus the standing naming rule ("mention
  the version name in the apk. like v16.6-debug.apk", recorded as
  D99). The every-video discriminator localized the defect to
  per-hostname setup (each video = fresh rr*.googlevideo.com
  hostnames; the feed's hosts are prewarmed and long-lived). The
  whole flow path was re-read end-to-end; the door/tunnel/upstream
  legs are byte-identical on both platforms, so the surviving
  amplifier was the one mechanism racing cannot fix: when the WHOLE
  address set of an rr hostname is blackholed (rr hosts carry the
  few addresses of one cache node each), every retry re-dials the
  same cached set and fails, and the 120 s-minimum-TTL DNS cache
  keeps serving the suspect set until it rotates — the per-video
  1–2 min wait and (URLs rotate rr hosts mid-play) the mid-play
  re-buffering. Fixed as ADR-0014: a TOTAL dial failure now demotes
  the host's cached answer to stale (doh.Resolver.Invalidate; the
  next retry re-queries the servers; serve-stale preserved), wired
  at all three dial sites (HTTP-CONNECT, plain-http forward, SOCKS5
  door) via an optional-capability probe; plus the resolver's own
  bootstrap dial now races its IPs (ADR-0013's dialer on the
  resolver's path). Pinned by seven new tests (four doh, three
  proxy — the new dialfeedback_test.go).
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.8 / versionCode 7** (APK 34,978,761 B,
  sha256 c0c4019f…cd5f6, the pinned D85 debug certificate — installs
  over debug.7 in place, settings + usage history survive): the
  dial-failure → DNS-requery feedback loop fix. D98 rules every
  release page carries the debug APK asset; D99 extends it — the
  asset is NAMED v<versionName>-debug.apk (v1.6.6-debug.apk here).
  The auth lane stays CLOSED (D97 — cancelled before any code;
  `docs/auth/` bannered history; ADR-AUTH-0001 retired). The
  release-keystore ceremony is complete on Ali's machine; all android
  releases are regular releases in the private repo (D89).
- **Branch and commit:** `main`, the M8-o commit (the feedback-loop
  fix + D99 + this record; see `git log -1` at handoff time) on top
  of 97adf0c (M8-n, the first-video fix) on top of 9f31b51 (A0-R3,
  the auth cancellation) on top of 6a7bce4 (A0-R2) on top of 60bfa13
  (A0) on top of cf136aa (M8-m); worktree clean after the commit.
  The release tag `v1.6.6-android-debug.8` points at the M8-o commit
  and is the repo's Latest (its APK asset is v1.6.6-debug.apk);
  debug.7 → 97adf0c (M8-n), debug.6 → 078e3a5 (M8-k), debug.5 →
  ad02980, debug.4 → cb28507 (helper-only; the release whose missing
  APK triggered D98), debug.3 → 2a8b678 (the device-circle APK),
  debug.2 → 096e8c0, debug.1 → f61bfae (superseded; its page keeps
  the D85 troubleshooting guide).
- **Files changed this phase (M8-o):** `internal/doh/cache.go`
  (invalidate), `internal/doh/resolver.go` (Invalidate + the racing
  bootstrap dial), `internal/doh/resolver_test.go` (four new pins),
  `internal/proxy/server.go` (invalidateDNS + the two HTTP-door call
  sites), `internal/proxy/socks5.go` (the SOCKS5 call site),
  `internal/proxy/dialfeedback_test.go` (NEW — three door-side
  pins), `android/app/build.gradle.kts` (versionCode 7),
  `DECISIONS.md` (D99), doc 06 (the M8-o record), doc 08 (ADR-0014),
  doc 09 (this update), doc 10 (status + the debug.8 section + row
  3's re-observation), `PROGRESS.md`, `CHANGELOG.md`,
  `SESSION_HANDOFF_PROMPT.md` (rewritten for M8-p).
- **Tests run and actual results (this session, M8-o):** svelte-check
  0/0; vite build clean; webui dist CLEAN-synced; go vet clean; go
  test -count=1 ./... **18/18 packages ok, run TWICE consecutively**;
  `GOOS=windows go build ./...` exit 0; the compile matrix:
  CGO_ENABLED=0 `go build ./internal/... ./mobile/...` **4/4 ABIs**
  (arm64, arm, 386, amd64) + arm64 full tree `./...` exit 0; gomobile
  bind (GOFLAGS=-mod=mod, the recorded
  -target=android/arm,android/arm64,android/amd64 form, androidapi
  26) → younblock.aar 24,993,512 B; clean assembleDebug →
  app-debug.apk 34,978,761 B, SHA-256
  c0c4019ffd3fb9b458b67a2cbffdc62d8f1f6632a3007347a78022ab5b9cd5f6;
  apksigner verify OK (the pinned c76bb7c6…8551d); badging
  versionCode 7 / versionName 1.6.6 / minSdk 26 / target 36. One
  build wrinkle recorded for the next session: a stale gradle daemon
  held the system JRE (no javac) — `./gradlew --stop` then an
  explicit `JAVA_HOME=~/sdk/jdk` clears it. The standing ⛔
  unchanged: no /dev/kvm, no device — the fix's on-device
  confirmation is Ali's round.
- **The environment is INTACT this session** (no wipe — verified at
  open: Go 1.27.2, javac 21.0.12.1, gomobile present; ~/sdk/env.sh
  sources fine). Wipes happen at ANY boundary — VERIFY first
  (`. ~/sdk/env.sh && go version`), reinstall per doc 06 §1 only if
  actually wiped (the cmdline-tools `12.0` alias URL 404s — use the
  versioned commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0014 (doc 08)
  — the dial-failure → DNS-requery feedback loop: a total dial
  failure demotes the cached answer to stale; host-proven by seven
  new tests, device confirmation pending (debug.8). ADR-0013's
  racing dialer is now also on the resolver's own bootstrap path.
  The auth lane stays CLOSED (D97; ADR-AUTH-0001 retired). Live
  ADRs: ADR-0001…0014. Next decision ID: **D100**.
- **Blocking questions:** none blocking the loop. The standing inputs
  are **Ali's debug.8 round**: the per-video verification (connect →
  play the first video, then several OTHER videos back to back —
  each should start within seconds and play without re-buffering;
  if not, the diagnostics report AFTER a stall is the required
  input — the tunnel lines carry dns=/dial=/first-byte=/dur= per
  flow, and the on-screen speed number at the slow moment splits a
  starved door path from a player-side transport), the cross-device
  UI verdict on debug.6 (still unreported), the rest of doc 10's
  circle (rows 4–8, rotation/TalkBack), and D93's desktop check on
  the rebuilt exe.
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-p — ingest Ali's results from v1.6.6-android-debug.8** (the
  per-video verification FIRST: fixed or not; if not, the
  diagnostics report after a stall + the speed number at the slow
  moment are the required inputs — they split the remaining suspects
  in one read: slow dns/dial = the network answer path, fast setup +
  slow dur = a throughput leg, nothing logged = the flow never
  reached the door, high speed while buffering = the player's own
  transport; then the cross-device UI verdict, the data-usage
  round, the rest of doc 10's circle, D93's desktop check).
  Root-cause what the evidence names; fix at the correct layer;
  bump `versionCode` and cut `v1.6.6-android-debug.9` through the
  same regular-release lane (D89) WHEN an app code change warrants
  it — **D98: every release page carries the debug APK asset; D99:
  the asset is NAMED v<versionName>-debug.apk (v1.6.6-debug.apk
  while the version name stays 1.6.6)** — push the code commit
  BEFORE creating the release (the full-SHA target_commitish 422s
  against an unpushed SHA). The auth Task IDs (A1/A2/A3) stay
  retired (D97). The dormant alternatives when no feedback arrives:
  the desktop release lane (the rounds under CHANGELOG "Unreleased")
  or public-release engineering — ask Ali which, do not guess.
