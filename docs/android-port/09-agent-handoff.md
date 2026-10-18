# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-p (2026-10-18, night — **Ali's debug.8
device round: the per-video stall PERSISTED and the promised
evidence path did not exist ("there is no diagnostic option in the
apk app setting"), so the round SHIPPED the in-app diagnostics
surface itself (ADR-0015, his D100 directive) as
`v1.6.6-android-debug.9` (versionCode 8, the APK asset named
v1.6.6-debug.apk — the Diagnostics screen on Menu AND in Settings,
the honest clipboard ladder, the native copy/share bridge, the TUN
counters + build identity inside the report, the 200-line log
window)**). Earlier the same day: M8-o (the per-video fix,
ADR-0014 + D99), M8-n (the first-video fix, ADR-0013 + D98), A0 +
A0-R2 + A0-R3 — **the account gate is CANCELLED (D97)**, nothing of
it was ever built. Phase 2 completed host-side before all of that;
the keystore helper survives PowerShell-less PATH machines (D90).
This file is the live handoff; keep the fields in §"Current handoff
fields" accurate at every session boundary.

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

- **Phase completed:** M8-p (2026-10-18, night — the THIRTEENTH
  feedback-loop exchange; Ali's device round on debug.8). His inputs:
  "The problem persists, there is no diagnostic option in the apk
  app setting. add an option to copy paste logs and diagnostics so i
  can paste it here." (recorded as D100). The stall moved past the
  ADR-0014 fix, and the loop's required evidence input was
  unreachable — the copy path existed only as Help's buried button,
  whose navigator.clipboard write can silently fail inside the
  Android WebView (toasting "Copied" over an empty clipboard). The
  round shipped the evidence-collection surface instead of a third
  root-cause guess (ADR-0015): the Diagnostics screen (menu entry +
  Settings row; Copy via the honest three-path ladder — shell bridge
  → navigator.clipboard → execCommand; Share via the system sheet on
  the APK; the selectable readonly textarea as the last resort), the
  Kotlin PageBridge (addJavascriptInterface "YounblockShell",
  write-only: copyToClipboard + shareText), the engine's
  SetReportExtra embedder hook (the facade's TUN counters — flows/
  rstUnmapped/overCap/doorFailures/dnsQueries/udpDropped — and the
  shell build identity "1.6.6 (versionCode 8)" set by Kotlin from
  PackageManager), and the report's log window doubled to 200 lines.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.9 / versionCode 8** (APK 35,026,961 B,
  sha256 1a2b2bf7…7833, the pinned D85 debug certificate — installs
  over debug.8 in place, settings + usage history survive): the
  in-app diagnostics surface. D98 rules every release page carries
  the debug APK asset; D99 names it v<versionName>-debug.apk;
  D100 makes the diagnostics surface the loop's standing evidence
  path (a "still broken" report without the pasted report is
  incomplete from this round on). The auth lane stays CLOSED (D97 —
  cancelled before any code). All android releases are regular
  releases in the private repo (D89).
- **Branch and commit:** `main`, the M8-p commit (the diagnostics
  surface + D100 + this record; see `git log -1` at handoff time) on
  top of 4b632c0 (M8-o, the per-video fix + D99) on top of 97adf0c
  (M8-n, the first-video fix) on top of 9f31b51 (A0-R3, the auth
  cancellation) on top of 6a7bce4 (A0-R2) on top of 60bfa13 (A0) on
  top of cf136aa (M8-m); worktree clean after the commit. The
  release tag `v1.6.6-android-debug.9` points at the M8-p commit and
  is the repo's Latest (its APK asset is v1.6.6-debug.apk);
  debug.8 → 4b632c0 (M8-o), debug.7 → 97adf0c (M8-n), debug.6 →
  078e3a5 (M8-k), debug.5 → ad02980, debug.4 → cb28507 (helper-only;
  the release whose missing APK triggered D98), debug.3 → 2a8b678
  (the device-circle APK), debug.2 → 096e8c0, debug.1 → f61bfae
  (superseded; its page keeps the D85 troubleshooting guide).
- **Files changed this phase (M8-p):** `internal/engine/engine.go`
  (SetReportExtra), `internal/engine/jsonapi.go` (the hook call +
  the 200-line window), `internal/engine/engine_test.go`
  (TestEngineReportExtra), `mobile/mobile.go` (the facade hook,
  SetShellVersion, diagnosticsExtra), `mobile/report_test.go` (NEW —
  the two report-section pins), `frontend/src/classic/lib/clipboard.ts`
  (NEW — the clipboard ladder + the shell bridge), `frontend/src/
  classic/screens/Diagnostics.svelte` (NEW — the screen),
  `frontend/src/classic/components/Menu.svelte` (the menu entry),
  `frontend/src/classic/screens/Settings.svelte` (the Settings row),
  `frontend/src/classic/components/Icon.svelte` (activity/share/
  refresh), `frontend/src/classic/i18n/en.json` (the diagnostics
  deck + the honest copy-failure toasts), `frontend/src/classic/
  App.svelte` (the view wiring; the honest copyReport; backend →
  $state), `android/app/src/main/java/.../MainActivity.kt`
  (PageBridge + the EngineHolder shell-version call),
  `android/app/src/main/res/values/strings.xml` (share_diagnostics),
  `android/app/build.gradle.kts` (versionCode 8), `DECISIONS.md`
  (D100), doc 06 (the M8-p record), doc 08 (ADR-0015), doc 09 (this
  update), doc 10 (status + the debug.9 section + row 3's
  re-observation + the circle intro note), `PROGRESS.md`,
  `CHANGELOG.md`, `SESSION_HANDOFF_PROMPT.md` (rewritten for M8-q).
- **Tests run and actual results (this session, M8-p):** svelte-check
  0/0 (one intermediate round on App.svelte's `backend` variable —
  fixed at the source with $state + the repo's svelte-ignore
  precedent); vite build clean; webui dist CLEAN-synced
  (placeholder.txt restored); go vet clean; go test -count=1 ./...
  **18/18 packages ok, run TWICE consecutively**; `GOOS=windows go
  build ./...` exit 0; the compile matrix: CGO_ENABLED=0
  `go build ./internal/... ./mobile/...` **4/4 ABIs** (arm64, arm,
  386, amd64) + arm64 full tree `./...` exit 0; gomobile bind
  (GOFLAGS=-mod=mod, the recorded
  -target=android/arm,android/arm64,android/amd64 form, androidapi
  26) → younblock.aar 25,035,860 B (classes.jar verified to carry
  the setShellVersion binding BEFORE gradle ran); clean
  assembleDebug → app-debug.apk **35,026,961 B**, SHA-256
  **1a2b2bf75553ea2b4d9d2eddc6ba8e2f694e40e2e1e47b25954cbb04c73d7833**;
  apksigner verify OK (the pinned c76bb7c6…8551d); badging
  versionCode 8 / versionName 1.6.6 / minSdk 26 / target 36. The
  standing ⛔ unchanged: no /dev/kvm, no device — the diagnostics
  surface's on-device verification (Copy actually copying, Share
  opening the sheet) rides Ali's debug.9 round together with the
  stall evidence.
- **The environment is INTACT this session** (no wipe — verified at
  open: Go 1.27.2, javac 21.0.12.1, gomobile present; ~/sdk/env.sh
  sources fine). Wipes happen at ANY boundary — VERIFY first
  (`. ~/sdk/env.sh && go version`), reinstall per doc 06 §1 only if
  actually wiped (the cmdline-tools `12.0` alias URL 404s — use the
  versioned commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0015 (doc 08)
  — the in-app diagnostics surface: the honest clipboard ladder, the
  shell bridge, the facade's TUN/build evidence inside the report;
  host-proven by three new pins, on-device verification pending
  (debug.9). ADR-0013 + ADR-0014 (the two stall fixes) remain
  host-proven and device-disproven — the NEXT root-cause waits for a
  pasted report, not a guess. The auth lane stays CLOSED (D97).
  Live ADRs: ADR-0001…0015. Next decision ID: **D101**.
- **Blocking questions:** none blocking the loop. The standing input
  is **Ali's debug.9 round**: (a) after a stall, Menu → Diagnostics
  → Copy (or Share) and PASTE THE REPORT — the tunnel lines
  (dns=/dial=/first-byte=/dur=), the tun: line (flows/udpDropped)
  and the on-screen speed number at the slow moment split the
  remaining suspects in one read; (b) the surface itself (did Copy
  copy, did Share open); plus the older circle rows (the
  cross-device UI verdict on debug.6, rows 4–8, rotation/TalkBack,
  the data-usage round, D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-q — ingest Ali's results from v1.6.6-android-debug.9**: the
  pasted diagnostics report FIRST (it names its own build via the
  "android shell:" line — verify the versionCode matches 8); read
  the tunnel lines + the tun: line + the speed number to name the
  waiting phase; root-cause from the evidence; fix at the correct
  layer (engine/facade/Kotlin/WebView); keep the D77–D100 honesty
  rules binding. Bump `versionCode` and cut
  `v1.6.6-android-debug.10` through the same regular-release lane
  (D89) WHEN an app code change warrants it — **D98: every release
  page carries the debug APK asset; D99: the asset is NAMED
  v<versionName>-debug.apk** — push the code commit BEFORE creating
  the release (the full-SHA target_commitish 422s against an
  unpushed SHA). The auth Task IDs (A1/A2/A3) stay retired (D97).
  The dormant alternatives when no feedback arrives: the desktop
  release lane (the rounds under CHANGELOG "Unreleased") or
  public-release engineering — ask Ali which, do not guess.
