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

- **Phase completed:** M8-q (2026-10-18, late — the FOURTEENTH
  feedback-loop exchange; Ali's device round on debug.9). His input:
  "commit and push and new release. I still dont see diagnostic
  button" — precise, and the root cause was in OUR build, not his
  phone: the M8-p diagnostics surface (menu entry, Settings row,
  Diagnostics screen) shipped into `frontend/src/classic/*` only,
  but `main.ts`'s variant dispatcher defaults to the PRO screen when
  `VITE_UI_VARIANT` is unset — and `build-android.sh` never sets it
  (only the desktop wails lanes bake classic; D66). The APK therefore
  served the pro screen, whose only diagnostics surface was the
  M6-era sheet section buried at the very bottom behind an android
  gate, using the raw `navigator.clipboard` copy that silently fails
  in the WebView. The round ported the surface into the pro tree
  (ADR-0016) and recorded the rule (D101): a user-facing surface
  ships in the variant(s) the edition serves — name the baked variant
  per edition (Android + web single-file → pro; Windows/Linux wails
  → classic) and verify through the real webui server, not only in
  the source tree.
- **The project's state now:** the CURRENT test-circle build is
  **v1.6.6-android-debug.10 / versionCode 9** (APK 35,053,265 B,
  sha256 4264996a…f9e7, the pinned D85 debug certificate — installs
  over debug.9 in place, settings + usage history survive): the
  diagnostics surface in the served variant. D98 rules every release
  page carries the debug APK asset; D99 names it
  v<versionName>-debug.apk; D100 keeps the diagnostics surface as
  the loop's standing evidence path; D101 adds the served-variant
  rule to the ship checklist. The auth lane stays CLOSED (D97 —
  cancelled before any code). All android releases are regular
  releases in the private repo (D89).
- **Branch and commit:** `main`, the M8-q commit (the port + D101 +
  this record; see `git log -1` at handoff time) on top of 65fe1d4
  (M8-p, the classic-tree surface + D100) on top of 4b632c0 (M8-o)
  on top of 97adf9c (M8-n) on top of 9f31b51 (A0-R3, the auth
  cancellation) on top of 6a7bce4 (A0-R2) on top of 60bfa13 (A0) on
  top of cf136aa (M8-m); worktree clean after the commit. The
  release tag `v1.6.6-android-debug.10` points at the M8-q commit
  and is the repo's Latest (its APK asset is v1.6.6-debug.apk);
  debug.9 → 65fe1d4 (M8-p), debug.8 → 4b632c0 (M8-o), debug.7 →
  97adf9c (M8-n), debug.6 → 078e3a5 (M8-k), debug.5 → ad02980,
  debug.4 → cb28507, debug.3 → 2a8b678, debug.2 → 096e8c0, debug.1 →
  f61bfae (superseded; its page keeps the D85 troubleshooting guide).
- **Files changed this phase (M8-q):** `frontend/src/lib/
  DiagnosticsView.svelte` (NEW — the pro screen's full-screen report
  view: auto-load, the ladder Copy imported cross-tree from
  `classic/lib/clipboard.ts`, Share behind canShare(), Refresh,
  honest inline status, the readonly selectable textarea, Escape/
  Back/Tab-focus handling), `frontend/src/lib/MenuSheet.svelte` (the
  Diagnostics LEAD row — every platform, no gate; the `diagnostics`
  dispatch; the old bottom section + its raw-clipboard copy REMOVED),
  `frontend/src/App.svelte` (the diagOpen state + view wiring +
  focus return to the menu button), `frontend/src/lib/icons.ts` (the
  six new Lucide exports), `android/app/build.gradle.kts`
  (versionCode 9), `DECISIONS.md` (D101), doc 06 (the M8-q record),
  doc 08 (ADR-0016), doc 09 (this update), doc 10 (status + the
  debug.10 section + debug.9 retitled + row 3's re-observation),
  `PROGRESS.md`, `CHANGELOG.md`, `SESSION_HANDOFF_PROMPT.md`
  (rewritten for M8-r).
- **Tests run and actual results (this session, M8-q):** svelte-check
  0/0 (two quick intermediate rounds — a class: directive on a
  component, then an unused CSS selector; both fixed at the source);
  vite build clean; webui dist CLEAN-synced (placeholder.txt
  restored); **the served surface verified END-TO-END through the
  real webui server** (`go run ./cmd/core --web --webport 8097`,
  mobile viewport 360×760: menu → Diagnostics is the FIRST row →
  view opens → the REAL report loads → Copy resolves to the honest
  "Copied" state → Back returns; no console errors; `?ui=classic`
  re-verified: the M8-p Menu → Diagnostics entry intact); go vet
  clean; go test -count=1 ./... **18/18 packages ok, run TWICE**
  (plus an uncached engine+mobile third round); `GOOS=windows go
  build ./...` exit 0; the full `./build-android.sh` lane exit 0
  (frontend rebuild → dist sync → gomobile bind → assembleDebug);
  the AAR's embedded Go library VERIFIED to carry the new UI
  (`diag-entry` ×2 + the row's hint string inside libgojni.so —
  go:embed compiles at bind time); app-debug.apk **35,053,265 B**,
  SHA-256
  **4264996ae39bb3e268bcf28913d3884d038c0833bb35ffd2233d60edd51ef9e7**;
  apksigner verify OK (the pinned c76bb7c6…8551d); badging
  versionCode 9 / versionName 1.6.6 / minSdk 26 / target 36. The
  standing ⛔ unchanged: no /dev/kvm, no device — on-device Copy/
  Share verification still rides Ali's round.
- **The environment is INTACT this session** (no wipe — verified at
  open: Go 1.27.2, javac present, gomobile present; ~/sdk/env.sh
  sources fine). Wipes happen at ANY boundary — VERIFY first
  (`. ~/sdk/env.sh && go version`), reinstall per doc 06 §1 only if
  actually wiped (the cmdline-tools `12.0` alias URL 404s — use the
  versioned commandlinetools-linux-11076708_latest.zip).
- **Current architecture decision / confidence:** ADR-0016 (doc 08)
  — the served-variant port: the pro screen's lead-row entry + the
  full report view, host-proven through the real webui server
  (the strongest host-side proof available — same dist, same
  dispatcher, same server the WebView loads). ADR-0015's engine/
  facade/Kotlin pieces carry into debug.10 unchanged. ADR-0013 +
  ADR-0014 (the two stall fixes) remain host-proven and
  device-disproven — the NEXT root-cause waits for a pasted report,
  not a guess. The auth lane stays CLOSED (D97). Live ADRs:
  ADR-0001…0016. Next decision ID: **D102**.
- **Blocking questions:** none blocking the loop. The standing input
  is **Ali's debug.10 round**: (a) open the menu — Diagnostics is
  the FIRST row now — open it, Copy, PASTE THE REPORT (even with
  nothing stalled: the "android shell: 1.6.6 (versionCode 9)" line
  proves the round); (b) after a stall, Diagnostics → Copy again and
  paste THAT report — the tunnel lines (dns=/dial=/first-byte=/
  dur=), the tun: line (flows/udpDropped) and the on-screen speed
  number at the slow moment split the remaining suspects in one
  read; (c) the surface itself (did Copy copy, did Share open);
  plus the older circle rows (the cross-device UI verdict on
  debug.6, rows 4–8, rotation/TalkBack, the data-usage round, D93's
  desktop check).
- **Exact next task:** ONE workstream — the feedback loop. **Task ID
  M8-r — ingest Ali's results from v1.6.6-android-debug.10**: the
  pasted diagnostics report FIRST (it names its own build via the
  "android shell:" line — verify the versionCode matches 9; a
  versionCode 8 report means the old build is still installed);
  read the tunnel lines + the tun: line + the speed number to name
  the waiting phase; root-cause from the evidence; fix at the
  correct layer (engine/facade/Kotlin/WebView); keep the D77–D101
  honesty rules binding — **D101: any new user-facing surface ships
  in the variant(s) the affected editions bake, verified through
  the real webui server before release**. Bump `versionCode` and
  cut `v1.6.6-android-debug.11` through the same regular-release
  lane (D89) WHEN an app code change warrants it — **D98: every
  release page carries the debug APK asset; D99: the asset is NAMED
  v<versionName>-debug.apk** — push the code commit BEFORE creating
  the release (the full-SHA target_commitish 422s against an
  unpushed SHA). The auth Task IDs (A1/A2/A3) stay retired (D97).
  The dormant alternatives when no feedback arrives: the desktop
  release lane (the rounds under CHANGELOG "Unreleased") or
  public-release engineering — ask Ali which, do not guess.
