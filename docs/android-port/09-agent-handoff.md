# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M8-l round (2026-10-18, the device-test
feedback loop's eleventh exchange — a DESKTOP-side ruling between
device rounds: D93, the close button is a real shutdown by default
and tray-keeping is the "Keep running in the tray when closed" opt-in;
no Android code changed, debug.6 stays Latest) — **Phase 2
host-complete; the test circle is on its FIFTH build,
published as regular releases in the private repo, and the keystore
helper survives PowerShell-less PATH machines (D90)**. This file is
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

- **Phase completed:** the device-test feedback loop's TENTH
  exchange (2026-10-18, Task ID M8-k, decision D92) — **the
  cross-platform UI unification: Ali's uploaded instruction rounds
  (mobile.zip at the repo root) applied to the ONE shared screen**.
  Ali reported the mobile UI is not consistent with his Windows/
  Linux builds; the zip carried the instruction documents those
  builds follow (round-2's roast + fixes, round-3's data-usage
  feature spec + usage-screens.png, site-test-plan). Applied to
  frontend/src (the pro screen every edition embeds — consistency
  by construction): the Lucide icon set through one file, the
  top-right 44 px “…” menu button, the sheet's in-header neutral
  close, 0.80 em timer digit boxes, the ring family capped clear
  of a bigger brighter hint, ping verdicts, honest -1 speeds, the
  ChitaCodes credit, the Other-websites section, the 360×640
  window floored at 320×560 — PLUS the round-3 data usage tracked
  ONCE in the engine (internal/usage; the Wails bridge and the
  webui's /api/usage both read it; usage.json holds times and
  byte totals only, 500 records, 15 s crash autosave). Beneath
  it: M8-j/D91 (the first device feedback answered), M8-i-b,
  M8-i-a, M8-h/D90, M8-g/D89, M8-e/D88.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.6 / versionCode 5** (built at 078e3a5, APK
  34,951,881 B, sha256 bc4b239e…d1fe8, the pinned D85 debug
  certificate — installs over debug.5 in place): everything
  debug.5 carried (the five-app default allow-list D87, the v4
  migration, the viewport-capped stage with its wider number
  spacing, the per-flow tunnel timing) PLUS this round's
  unification — the shared screen now follows the instruction
  rounds on every platform (round-2 fixes + round-3 usage lines,
  Last-session line, the menu's Data usage section with a
  confirmed Clear, the engine-tracked history in usage.json).
  The helper facts beneath stand: make-release-keystore.bat
  (13,683 B, sha256 ee35e456…9558) with the D90 full-path +
  cmd-fallback resolution; the keystore ceremony is COMPLETE on
  Ali's machine (M8-i-b) — nothing further until a signed public
  release is planned. All the android releases are regular
  releases in the private repo (D89); the spec audit (M0–M8
  host-complete, every remaining ⛔ a device gate) is on record.
- **Branch and commit:** `main`, the M8-k code commit 078e3a5 (the
  app changes + D92, pushed BEFORE the release was cut — the full-
  SHA target_commitish 422s until the commit exists on the remote)
  plus the M8-k docs commit (see `git log -1` at handoff time);
  worktree clean after the commits; the release tag
  `v1.6.6-android-debug.6` points at 078e3a5 and is the repo's
  Latest; debug.5 → ad02980, debug.4 → cb28507 (helper-only),
  debug.3 → 2a8b678 (the device-circle APK), debug.2 → 096e8c0,
  debug.1 → f61bfae (superseded; debug.1's page keeps the D85
  troubleshooting guide).
- **Files changed this phase:** the app changes (commit 078e3a5):
  `frontend/package.json` + lock (lucide-svelte), the shared screen
  (`frontend/src/App.svelte`, `lib/Timer.svelte`, `lib/DotRings.svelte`,
  `lib/PowerPill.svelte`, `lib/MenuSheet.svelte`, `lib/backend.ts`,
  `lib/format.ts`, `lib/types.ts`, the new `lib/icons.ts`, tokens.css),
  the usage lane (`internal/usage/` new package + tests,
  `internal/engine/usage.go` new + engine.go/health.go/jsonapi.go
  wired + the lifecycle tests, `internal/stats` Totals,
  `internal/webui` the two routes + tests, `ui_bridge.go` +
  `usage_bridge_test.go`, `engine_adapter.go`, `app.go`, `main.go`
  the 360×640 window), `android/app/build.gradle.kts` (versionCode
  4 → 5); plus docs (the follow-up commit): DECISIONS.md (D92),
  doc 06 (the M8-k record), doc 09, doc 10 (the debug.6 edition),
  PROGRESS.md, CHANGELOG.md, README, SESSION_HANDOFF_PROMPT.md —
  plus the GitHub release lane (debug.6 created regular + Latest +
  APK, byte-exact).
- **Tests run and actual results:** after the changes, all green:
  `go vet ./...` clean; `go build ./...` clean; `go test -count=1
  ./...` **18/18 packages ok, run twice consecutively** (the new
  tests included: the usage store's save/recover/cap/summary/clear
  + the no-hostname field rule; the engine's session lifecycle,
  cancelled-connect, clear-drops-lastSession and
  persists-across-engines; the bridge's UsageSource fold-in,
  honest -1, zero-vs-unknown and construction seeding; the webui's
  usage routes with the explicit-null contract); svelte-check 0/0;
  vite build clean; the webui dist CLEAN-synced (identical to
  frontend/dist). Beyond the harness: the rendered app exercised
  with a browser at 320×560, 356×625 and 360×760 across every
  state (no overlap, no scrollbars; Keep keeps, Delete empties
  and drops the Last-session line), and the byte path proven on
  the REAL binary — a 949,386-byte YouTube page through the
  engine's own tunnel = session downBytes 980,303 / upBytes
  1,933, the 80-second record in usage.json after the stop,
  todayBytes = allTimeBytes = 982,236 in the summary. The build
  lane: gomobile bind → younblock.aar 25,003,698 B; assembleDebug
  → app-debug.apk 34,951,881 B, sha256 bc4b239e…d1fe8; aapt2
  badging: versionCode 5, minSdk 26, targetSdk 36, 3 ABIs;
  apksigner: the pinned c76bb7c6…8551d debug cert. The release
  lane live through the API: debug.6 regular + Latest
  (id 416096215), the APK asset (651728792) served bytes
  re-downloaded through the API asset endpoint and hashed
  byte-exact. The standing ⛔ re-confirmed: no /dev/kvm, no
  device — the UI consistency on Ali's actual devices and the
  video-start delay both wait for his round.
- **The environment is RESTORED and READY** (2026-10-17, M8-i-a):
  ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1 (javac), the
  Android SDK (cmdline-tools 12.0, platform-tools 37.0.1,
  platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358), and
  gomobile/gobind at the pinned x/mobile version (~/go/bin).
  Source `~/sdk/env.sh` before any build. If a future session finds
  ~/sdk wiped again, reinstall per doc 06 §1 (mind the URL
  wrinkle) — but VERIFY first; do not reinstall over a working
  toolchain.
- **Current architecture decision / confidence:** D93 (the close
  button is a real shutdown by default; "Keep running in the tray
  when closed" is the explicit opt-in — one settings field honored
  only where a tray exists, surfaced on both screens behind
  capabilities.tray, no schema migration owed). Confidence: high —
  pinned by close_test.go against a real engine, compile-checked for
  GOOS=windows, exercised end-to-end on a real --web engine (the
  disclosed scratch tray:true lane) with the clean-build negative
  check. Beneath it: D92 (one UI across every device — the shared
  screen + engine-tracked usage), D91, D90, D89, D88. Next decision
  ID: D94.
- **Blocking questions:** none product-side. The signed-release
  decision still returns only when a public release is planned —
  its ceremony is automated and documented (D88/D90) and its
  keystore exists on Ali's machine. The standing inputs are
  **Ali's cross-device UI check on debug.6** (does the phone now
  match his Windows/Linux builds? the Data usage numbers sane?),
  the still-open **diagnostics report from a slow-video moment**
  (the debug.5 timing lines name the phase), and the rest of the
  test circle (rows 4–8, rotation/TalkBack). D93 added one desktop
  input: Ali rebuilds the Windows exe and walks README's updated
  manual-check steps 4–5 (close = full stop by default; the opt-in
  keeps the tray).
- **Exact next task:** continue the **device-test feedback loop**:
  ingest Ali's results from v1.6.6-android-debug.6 (Task ID M8-m) —
  first the UI-consistency verdict across his devices (phone vs
  the Windows/Linux builds he rebuilt from this tree), the
  data-usage round (connect, browse, watch "X used" climb,
  disconnect, Last-session line, menu's Today/This month/All time,
  Clear with both buttons), then the video-start timing evidence
  if the delay persists (Menu → Diagnostics → Copy while a video
  is slow; phone model + Android version; every video or just the
  first after connecting), then the rest of doc 10's circle
  (youtube.com per browser, .ir direct, stop paths, battery,
  network switch). Root-cause what the evidence names (the
  D77–D92 honesty rules binding as ever), bump `versionCode` and
  cut `v1.6.6-android-debug.7` through the same release lane — a
  REGULAR release (D89) — WHEN an app code change warrants it
  (remember: push the code commit BEFORE creating the release; the
  full-SHA target_commitish 422s against an unpushed SHA). The
  environment is READY (verify with `. ~/sdk/env.sh && go version`
  first; the FIFTH wipe happened MID-SESSION in M8-j — expect
  wipes at any boundary and reinstall per doc 06 §1 + the URL
  wrinkle when they land). Open offer on the table, awaiting Ali's
  ruling: attaching the APK to every release page (relaxing D84's
  no-duplicate discipline) — do NOT do it unilaterally.
