# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at M8-q (2026-10-18, late — **Ali's debug.9
device round: "I still dont see diagnostic button" — and his report
was exact: the M8-p surface shipped ONLY into the classic UI tree,
while the APK bakes the pro screen (D66's dispatcher default), so the
phone never rendered it. The round ported the ADR-0015 surface into
the served variant (ADR-0016 + D101 — the variant rule: a surface
ships in the variant(s) the edition serves) as
`v1.6.6-android-debug.10` (versionCode 9, the APK asset named
v1.6.6-debug.apk — Diagnostics is now the FIRST row of the menu
sheet on every platform, opening the full-screen report view: the
clipboard ladder Copy, Share behind the shell bridge, the selectable
text; the old buried sheet section removed)**). Earlier the same
day: M8-p (the surface itself, ADR-0015 + D100 — the engine, facade
and Kotlin bridge pieces of it are variant-independent and carried
into debug.10 unchanged), M8-o/M8-n (the stall fixes, ADR-0014/0013 +
D99/D98), A0 + A0-R2 + A0-R3 — **the account gate is CANCELLED
(D97)**, nothing of it was ever built. Phase 2 completed host-side
before all of that; the keystore helper survives PowerShell-less PATH
machines (D90). This file is the live handoff; keep the fields in
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

- **Phase completed:** M8-r (2026-10-18, late night — the FIFTEENTH
  feedback-loop exchange; the debug.10 device round). Ali's input:
  "the problem persists" + THE FIRST PASTED DIAGNOSTICS REPORT —
  the exact evidence D100's loop was built for. Its "android shell:
  1.6.6 (versionCode 9)" line proved both the debug.10 install and
  that the M8-q surface rendered and copied on his phone; its log
  read back against the engine code line by line (ADR-0017's
  Context holds the full reconstruction): the QuickCheck's dns step
  failed against all three DoH servers in ~170 ms each (a startup
  blip, once), negative-caching www.youtube.com for the 15 s
  negative TTL; the automatic heal sweep triggered by that same
  failure probed that same name for every method and failed
  INSTANTLY on the negative entry each time (ErrRecentlyFailed —
  no server is re-asked inside the window) while the sweep's own
  video handshakes were completing (3 of 4 methods) — so a
  RECOVERED network was declared broken: degraded, the "Couldn't
  fix it" notification 21 s after Start, and the failed sweep left
  method=legacy live (the last-tried preset, off the saved
  choice). Collateral finding: the report's DNS lines were
  attr-stripped by the log ring (bare "dns server failed" — no
  reason). The round's fixes are host-PROVEN by reproduction: with
  the fixes reverted, the new regression test fails into the
  report's EXACT signature (Phase:degraded Method:legacy, never
  ON); with them, the engine heals to ON through the sweep.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.11 / versionCode 10** (APK 35,053,457 B,
  sha256 d4602394…1dcb6, the pinned D85 debug certificate —
  installs over debug.10 in place, settings + usage history
  survive): the heal sweep probes from zero (a sweep resets the
  DNS resolver at start — the netmon's network-change semantics),
  a failed sweep restores the previous method, and the DNS
  failure/fallback/stale/tunnel-lookup log lines bake server=#N +
  reason= into the message so the NEXT pasted report names the
  failure kind. D98 rules every release page carries the debug APK
  asset; D99 names it v<versionName>-debug.apk; D100 keeps the
  diagnostics surface as the loop's standing evidence path; D101
  keeps the served-variant rule on the ship checklist. The auth
  lane stays CLOSED (D97). All android releases are regular
  releases in the private repo (D89).
- **Branch and commit:** `main`, the M8-r commit (the three engine
  fixes + ADR-0017 + this record; see `git log -1` at handoff
  time) on top of 5fdf8b2 (M8-q, the served-variant port) on top
  of 65fe1d4 (M8-p) on top of 4b632c0 (M8-o) on top of 97adf9c
  (M8-n); worktree clean after the commit. The release tag
  `v1.6.6-android-debug.11` points at the M8-r commit and is the
  repo's Latest (its APK asset is v1.6.6-debug.apk); debug.10 →
  5fdf8b2, debug.9 → 65fe1d4, debug.8 → 4b632c0, debug.7 →
  97adf9c, debug.6 → 078e3a5, debug.5 → ad02980, debug.4 →
  cb28507, debug.3 → 2a8b678, debug.2 → 096e8c0, debug.1 →
  f61bfae (superseded; its page keeps the D85 troubleshooting
  guide).
- **Files changed this phase (M8-r):** `internal/engine/health.go`
  (healSweep: the resolver reset at sweep start + the method
  restore on the no-working-method path), `internal/doh/
  resolver.go` (the failure/fallback/stale log lines bake
  server=#N + reason= into the message; the configured NAME stays
  a file-log attr), `internal/proxy/server.go` (the tunnel-lookup
  failure line bakes reason=), `internal/engine/health_test.go`
  (TestHealSweepSurvivesFreshNegativeCache + TestFailedSweep-
  RestoresPreviousMethod + the localNet flaky-DoH seam),
  `internal/doh/resolver_test.go` (TestServerFailureLinesCarry-
  Evidence + the captureLogger), `android/app/build.gradle.kts`
  (versionCode 10), doc 06 (the M8-r record), doc 08 (ADR-0017),
  doc 09 (this update), doc 10 (the debug.11 section + row 3's
  first-report evidence + debug.10 retitled), `PROGRESS.md`,
  `CHANGELOG.md`, `SESSION_HANDOFF_PROMPT.md` (rewritten for
  M8-s). NO frontend file touched — the M8-q dist carries over.
- **Tests run and actual results (this session, M8-r):** go vet
  clean; go test -count=1 ./... **18/18 packages ok, run TWICE**;
  `GOOS=windows go build ./...` exit 0; the Android compile matrix
  in its exact form (arm64 full tree at CGO_ENABLED=0 exit 0; the
  non-main set ./internal/... ./mobile/... 4/4 ABIs); the full
  `./build-android.sh` lane exit 0 (the frontend rebuilt to the
  same bytes — tree unchanged; dist sync; gomobile bind;
  assembleDebug); the AAR's embedded Go library VERIFIED to carry
  the new engine strings ("dns server failed server=#" inside
  libgojni.so in all three ABIs); app-debug.apk **35,053,457 B**,
  SHA-256 **d46023942d158c1af0df2161ed59aed46613e1b93693c50b035
  29b0acc71dcb6**; apksigner verify OK (the pinned c76bb7c6…8551d);
  badging versionCode 10 / versionName 1.6.6 / minSdk 26 / target
  36. The three regression tests verified BOTH ways (fix reverted
  → FAIL with the report's signature; fix applied → PASS). The
  standing ⛔ unchanged: no /dev/kvm, no device.
- **The environment is INTACT this session** (no wipe — verified at
  open: Go 1.27.2, javac 21.0.12, platforms 26/36, the pinned
  gomobile; ~/sdk/env.sh sources fine). Disk was tight (604 MB
  free) — the Go build cache (1.9 GB) was cleared before the bind;
  expect that again on a ~9.9 GB box after big builds. Wipes
  happen at ANY boundary — VERIFY first (`. ~/sdk/env.sh && go
  version`), reinstall per doc 06 §1 only if actually wiped (the
  cmdline-tools `12.0` alias URL 404s — use the versioned
  commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0017 (doc
  08) — the heal sweep probes from zero: the sweep's DNS reset,
  the failed-sweep method restore, and the message-baked DNS
  evidence lines, each pinned by a regression test that fails on
  pre-fix code into the pasted report's own signature. Confidence:
  the root cause was read FROM Ali's report and reproduced on the
  host BEFORE the fix — the strongest proof class this loop has.
  What remains OPEN: the startup DoH blip's CAUSE (the next
  report's reason= line answers it — no third guess) and the stall
  row itself (the five 51 s / ~7 KB flows + udpDropped=76 are
  ISP-side interference on the direct path; other flows completed
  fine in the same session — no steering or policy change without
  a ruling). ADR-0013 + ADR-0014 stand host-proven (their layers
  are now part of the working path). The auth lane stays CLOSED
  (D97). Live ADRs: ADR-0001…0017. Next decision ID: **D102**.
- **Blocking questions:** none blocking the loop. The standing
  input is **Ali's debug.11 round**: (a) Start on the normal
  network → Connected WITHOUT the false "Couldn't fix it"
  notification, and the method in Settings stays what was set;
  (b) at the next stall, menu → Diagnostics (the first row) →
  Copy → paste THAT report — the baked reason= lines on any "dns
  server failed" entry (timeout vs network vs DNS_UNREACHABLE-
  class) are the sharpened row-3 question, plus the tunnel lines
  and the tun: line as before; (c) the surface itself (did Copy
  copy, did Share open — carried from debug.10); plus the older
  circle rows (the cross-device UI verdict on debug.6, rows 4–8,
  rotation/TalkBack, the data-usage round, D93's desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task
  ID M8-s — ingest Ali's results from v1.6.6-android-debug.11**:
  the pasted diagnostics report FIRST (verify the "android shell:"
  line names versionCode 10; a versionCode 9 report means the old
  APK is still installed and the round starts with the install);
  read the baked reason= lines + the tunnel lines + the tun: line
  to name the waiting phase; root-cause from the evidence; fix at
  the correct layer (engine/facade/Kotlin/WebView); keep the
  D77–D101 honesty rules binding — **D101: any new user-facing
  surface ships in the variant(s) the affected editions bake,
  verified through the real webui server before release**. Bump
  `versionCode` and cut `v1.6.6-android-debug.12` through the
  same regular-release lane (D89) WHEN an app code change
  warrants it — **D98: every release page carries the debug APK
  asset; D99: the asset is NAMED v<versionName>-debug.apk** —
  push the code commit BEFORE creating the release (the full-SHA
  target_commitish 422s against an unpushed SHA). The auth Task
  IDs (A1/A2/A3) stay retired (D97). The dormant alternatives when
  no feedback arrives: the desktop release lane (the rounds under
  CHANGELOG "Unreleased") or public-release engineering — ask Ali
  which, do not guess.
