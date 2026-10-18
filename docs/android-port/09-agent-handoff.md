# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-n (2026-10-18, evening — **Ali's first
debug.6 device round: the slow first video stayed (2–3 minutes) while
Windows was instant; root-caused to a code-level asymmetry between the
two doors, FIXED as ADR-0013, and shipped as
`v1.6.6-android-debug.7` (versionCode 6). His D98 ruling landed the
same round: every release page carries the debug APK from now on**).
Earlier the same day the account-gate lane ran and closed: A0 (the
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

- **Phase completed:** M8-n (2026-10-18, evening — the ELEVENTH
  feedback-loop exchange; Ali's first device round on debug.6). His
  inputs: the video-start delay STAYED and grew ("2-3 minutes, less
  or more, for first video to actually play") while "the windows app
  doesnt have this problem" — plus the ruling "from now on, add the
  APK version (debug version) in the releases" (recorded as D98).
  The Windows-vs-Android discriminator localized the defect to the
  Android-only layers; the root cause was found in the code (the one
  structural difference M8-j's elimination had left standing): the
  TUN stack aborted every flow whose door dial exceeded 10 s (a
  default the facade never overrode), SHORTER than the SOCKS5 door's
  own honest setup budget (DoH resolve up to 3×3 s + dial up to 12 s
  — which the desktop HTTP-CONNECT door runs with NO abort, hence
  Windows instant); the sequential 5 s-per-address dial walk against
  a ≥120 s-cached DNS answer turned every app retry into the same
  wall until the cache rotated — the minutes. Fixed as ADR-0013
  (two halves): netx.DialHost now fails over Happy-Eyeballs-style
  (attempt k at k×750 ms, first success wins, losers canceled —
  benefits both doors) and the TUN handshake budget is budget-true
  (config.TunHandshakeTimeout 30 s, passed by the facade, tunstack
  default raised to match; TestHandshakeTimeoutDefaultCoversDoorBudget
  pins the lockstep). Pinned also by
  TestDialHostRacesPastBlackholedFrontAddress and
  TestDialHostWinnerCancelsLosers.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.7 / versionCode 6** (APK 34,960,649 B,
  sha256 dc01c041…dd83d, the pinned D85 debug certificate — installs
  over debug.6 in place, settings + usage history survive): the
  first-video fix. D98 now rules every release page carries the
  debug APK asset. The auth lane stays CLOSED (D97 — cancelled
  before any code; `docs/auth/` bannered history; ADR-AUTH-0001
  retired). The release-keystore ceremony is complete on Ali's
  machine; all android releases are regular releases in the private
  repo (D89).
- **Branch and commit:** `main`, the M8-n commit (the first-video
  fix + D98 + this record; see `git log -1` at handoff time) on top
  of 9f31b51 (A0-R3, the auth cancellation) on top of 6a7bce4
  (A0-R2) on top of 60bfa13 (A0) on top of cf136aa (M8-m); worktree
  clean after the commit. The release tag
  `v1.6.6-android-debug.7` points at the M8-n commit and is the
  repo's Latest; debug.6 → 078e3a5 (M8-k), debug.5 → ad02980,
  debug.4 → cb28507 (helper-only; the release whose missing APK
  triggered D98), debug.3 → 2a8b678 (the device-circle APK),
  debug.2 → 096e8c0, debug.1 → f61bfae (superseded; its page keeps
  the D85 troubleshooting guide).
- **Files changed this phase (M8-n):** `internal/netx/dialer.go` (the
  racing DialHost + FallbackDelay), `internal/netx/dialer_test.go`
  (two new pins), `internal/config/defaults.go` (TunHandshakeTimeout),
  `internal/tunstack/tunstack.go` (defaultHandshakeTimeout 30 s +
  field doc), `internal/tunstack/tunstack_test.go` (the lockstep
  pin), `mobile/tun.go` (the explicit HandshakeTimeout pass),
  `android/app/build.gradle.kts` (versionCode 6), `DECISIONS.md`
  (D98), doc 06 (the M8-n record), doc 07 (§9's closing paragraph),
  doc 08 (ADR-0013), doc 09 (this update), doc 10 (status + the
  debug.7 section + row 3's re-observation), `PROGRESS.md`,
  `CHANGELOG.md`, `SESSION_HANDOFF_PROMPT.md` (rewritten for M8-o).
- **Tests run and actual results (this session, M8-n):** svelte-check
  0/0; vite build clean; webui dist CLEAN-synced; go vet clean; go
  test -count=1 ./... **18/18 packages ok, run TWICE consecutively**;
  `GOOS=windows go build ./...` exit 0; the compile matrix in its
  exact form: CGO_ENABLED=0 `go build ./internal/... ./mobile/...` =
  19 non-main packages per ABI, **4/4** (arm64, arm, 386, amd64),
  plus arm64 linking the full tree `./...`; gomobile bind (3 ABIs,
  androidapi 26) → younblock.aar 25,002,994 B; clean assembleDebug →
  app-debug.apk 34,960,649 B, SHA-256
  dc01c0419a211ca007235ffbeaec6ab5acecb2e5930a4efd4642097209cdd83d;
  apksigner verify OK (the pinned c76bb7c6…8551d); badging
  versionCode 6 / minSdk 26 / target 36 / 3 ABIs. The standing ⛔
  unchanged: no /dev/kvm, no device — the fix's on-device
  confirmation is Ali's round.
- **The environment is INTACT this session** (no wipe — verified at
  open: Go 1.27.2, javac 21.0.12.1, gomobile present; ~/sdk/env.sh
  sources fine). Wipes happen at ANY boundary — VERIFY first
  (`. ~/sdk/env.sh && go version`), reinstall per doc 06 §1 only if
  actually wiped (the cmdline-tools `12.0` alias URL 404s — use the
  versioned commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0013 (doc 08)
  — the first-flow latency fix: fast address failover in the one
  dialer + the budget-true TUN handshake timeout; host-proven by the
  timing tests, device confirmation pending. The auth lane stays
  CLOSED (D97; ADR-AUTH-0001 retired). Live ADRs: ADR-0001…0013.
  Next decision ID: **D99**.
- **Blocking questions:** none blocking the loop. The standing inputs
  are **Ali's debug.7 round**: the first-video timing (connect →
  first video should start within seconds like Windows; if not, the
  diagnostics report at the slow moment + phone model + Android
  version), the cross-device UI verdict on debug.6 (does the phone
  match his desktops? Data usage sane? — still unreported), the rest
  of doc 10's circle (rows 4–8, rotation/TalkBack), and D93's
  desktop check on the rebuilt exe.
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-o — ingest Ali's results from v1.6.6-android-debug.7** (the
  first-video confirmation FIRST: fixed or not; if not, the
  diagnostics report is the required input — the debug.5 timing
  lines name the waiting phase; then the cross-device UI verdict,
  the data-usage round, the rest of doc 10's circle, D93's desktop
  check). Root-cause what the evidence names; fix at the correct
  layer; bump `versionCode` and cut `v1.6.6-android-debug.8` through
  the same regular-release lane (D89) WHEN an app code change
  warrants it — **D98 now ALSO requires every release page to carry
  the debug APK asset, including docs/helper-only rounds** — push
  the code commit BEFORE creating the release (the full-SHA
  target_commitish 422s against an unpushed SHA). The auth Task IDs
  (A1/A2/A3) stay retired (D97). The dormant alternatives when no
  feedback arrives: the desktop release lane (the rounds under
  CHANGELOG "Unreleased") or public-release engineering — ask Ali
  which, do not guess.
