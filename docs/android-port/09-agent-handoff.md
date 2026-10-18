# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the M8-m round (2026-10-18, the device-test
feedback loop's twelfth exchange — an ANALYSIS-ONLY round between
device rounds: the complete gap analysis Ali asked for, delivered
as a report, plus the D94 ruling that opens the account-gate
feature lane: login authorization — a verified username/password
gate with a user database and admin user management, making the
app members-only; design-first, nothing app-facing changed,
debug.6 stays Latest) — **Phase 2 host-complete; the test circle
is on its FIFTH build, published as regular releases in the
private repo, and the keystore helper survives PowerShell-less
PATH machines (D90)**. This file is the live handoff; keep the
fields in §"Current handoff fields" accurate at every session
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

- **Phase completed:** the device-test feedback loop's TWELFTH
  exchange (2026-10-18, Task ID M8-m, decision D94) — **the
  complete gap analysis round (analysis-only, zero code changes)
  + the ruling that opens the account-gate lane**. Ali asked for
  a full gap analysis; the report was delivered in chat from two
  read-only inventory passes (the Go host: every package, the
  D93 close path, all 287 tests across 18 packages, every
  deliberate limit — including the connect-path timeout constants
  that are the slow-video diagnosis candidates; the frontend +
  Android: the capability matrix, the settings asymmetries, the
  lane facts) plus a doc baseline and an independent mobile.zip
  re-verification (100% of every round's instructions applied —
  the open item is data, VERIFIED_SITES, not code). The verdict:
  host-complete and green everywhere the sandbox can measure; the
  real gaps are the open slow-video defect (awaiting the
  diagnostics report), the largely unrun device test circle
  (rows 4–8, rotation/TalkBack, the cross-device UI verdict, the
  data-usage check, the D93 real-Windows check), the dormant
  desktop release lane (v0.7.6 predates D86–D93 — nine rounds
  sit under CHANGELOG "Unreleased"), and public-release
  engineering not started (debug-only signing, no CI — no
  .github/ at all, no frontend tests, unsigned exe, no update
  mechanism). At the wrap-up Ali opened the account-gate lane
  (D94): login authorization — a verified username/password gate
  with a user database and admin user management, members-only —
  design-first. Beneath it: M8-l/D93 (the close ruling), M8-k/D92
  (the UI unification), M8-j/D91, M8-h/D90, M8-g/D89, M8-e/D88.
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
  D94 (2026-10-18) opens the account-gate lane on top: login
  authorization on every edition, backed by a user database and
  admin user management — nothing built yet; the design lane
  (A0) comes first, and the feedback loop continues in parallel
  (M8-n).
- **Branch and commit:** `main`, the M8-m docs-only commit (the
  gap-analysis record + D94 — see `git log -1` at handoff time)
  on top of 9615936 (M8-l, the D93 close ruling); worktree clean
  after the commit. The release tag `v1.6.6-android-debug.6`
  points at 078e3a5 (M8-k) and is the repo's Latest; debug.5 →
  ad02980, debug.4 → cb28507 (helper-only), debug.3 → 2a8b678
  (the device-circle APK), debug.2 → 096e8c0, debug.1 → f61bfae
  (superseded; debug.1's page keeps the D85 troubleshooting
  guide).
- **Files changed this phase:** documentation only (the M8-m
  commit): DECISIONS.md (D94 — the account-gate ruling, four
  rulings), CHANGELOG.md (a Planned section under Unreleased —
  the members-only announcement, honest about the privacy-copy
  change it brings), doc 06 (the M8-m record), doc 09 (this
  update), doc 10 (the status note), PROGRESS.md, and
  SESSION_HANDOFF_PROMPT.md rewritten for the two-workstream
  session (the feedback loop as M8-n + the account-gate spec
  lane as A0). Zero app code, zero frontend, zero Android: the
  analysis round changed no behavior and cut no release; no
  release lane ran (nothing app-facing changed — the D84/D89
  discipline).
- **Tests run and actual results:** none re-run this round — the
  tree is byte-identical to the M8-l close (9615936) where the
  baseline was last verified fully green: `go vet ./...` clean;
  `go test -count=1 ./...` 18/18 packages ok run twice
  consecutively; svelte-check 0/0; vite build clean; the webui
  dist clean-synced; `GOOS=windows` compiling the whole shell.
  The gap analysis verified structural facts instead: 287 test
  functions across 18 packages (the "18/18" gate's true extent),
  zero gofmt drift at HEAD, the dist sync current, and mobile.zip
  100% instruction compliance. The standing ⛔ re-confirmed: no
  /dev/kvm, no device — the UI consistency on Ali's actual
  devices and the video-start delay both wait for his round.
- **The environment is RESTORED and READY** (2026-10-17, M8-i-a):
  ~/sdk carries Go 1.27.2, the Temurin JDK 21.0.12.1+1 (javac), the
  Android SDK (cmdline-tools 12.0, platform-tools 37.0.1,
  platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358), and
  gomobile/gobind at the pinned x/mobile version (~/go/bin).
  Source `~/sdk/env.sh` before any build. If a future session finds
  ~/sdk wiped again, reinstall per doc 06 §1 (mind the URL
  wrinkle) — but VERIFY first; do not reinstall over a working
  toolchain.
- **Current architecture decision / confidence:** D94 (the account
  gate: Younblock becomes members-only — a verified username/
  password gate on every edition, backed by a real authentication
  backend: a user database, login/verification endpoints, and an
  admin surface for managing users; design-first, nothing built;
  the privacy-claim conflict named up front — "no account, no
  data collection" changes when this ships, disclosed never
  silently, while the proxying itself stays local and unchanged).
  Confidence: high as a DIRECTION — the open architecture
  questions (backend hosting; admin-created vs open signup;
  fail-open vs fail-closed when the auth server is unreachable;
  token/session design; password storage; brute-force
  protection; the login surface on the shared screen) each need
  Ali's ruling in the A0 design lane before implementation.
  Beneath it: D93 (the close ruling — shipped in the tree,
  awaiting Ali's rebuilt-exe check), D92 (one UI across every
  device), D91, D90, D89, D88. Next decision ID: D95.
- **Blocking questions:** none blocking the feedback loop. The
  account-gate lane brings DESIGN questions for Ali (D94 ruling
  3: where the backend lives; who creates accounts; fail-open vs
  fail-closed offline; token/session design; password storage;
  brute-force protection) — they gate the A0 spec, not the device
  round. The signed-release decision still returns only when a
  public release is planned — its ceremony is automated and
  documented (D88/D90) and its keystore exists on Ali's machine.
  The standing inputs are **Ali's cross-device UI check on
  debug.6** (does the phone now match his Windows/Linux builds?
  the Data usage numbers sane?), the still-open **diagnostics
  report from a slow-video moment** (the debug.5 timing lines
  name the phase), and the rest of the test circle (rows 4–8,
  rotation/TalkBack). D93 added one desktop input: Ali rebuilds
  the Windows exe and walks README's updated manual-check steps
  4–5 (close = full stop by default; the opt-in keeps the tray).
- **Exact next task:** TWO workstreams, the feedback loop first.
  **(1) Task ID M8-n — ingest Ali's results from
  v1.6.6-android-debug.6:** first the UI-consistency verdict
  across his devices (phone vs the Windows/Linux builds he
  rebuilt from this tree), the data-usage round (connect, browse,
  watch "X used" climb, disconnect, Last-session line, menu's
  Today/This month/All time, Clear with both buttons), then the
  video-start timing evidence if the delay persists (Menu →
  Diagnostics → Copy while a video is slow; phone model + Android
  version; every video or just the first after connecting), then
  the rest of doc 10's circle (youtube.com per browser, .ir
  direct, stop paths, battery, network switch), plus D93's
  desktop check (the rebuilt exe, README manual-check steps 4–5).
  Root-cause what the evidence names (the D77–D94 honesty rules
  binding as ever), bump `versionCode` and cut
  `v1.6.6-android-debug.7` through the same release lane — a
  REGULAR release (D89) — WHEN an app code change warrants it
  (remember: push the code commit BEFORE creating the release;
  the full-SHA target_commitish 422s against an unpushed SHA).
  **(2) Task ID A0 — the account-gate design lane:** write the
  spec (audit + architecture + ADRs, the M0–M2 pattern the
  Android port followed) for the login authorization system D94
  records: the login page on the shared screen (a signed-out
  state before the connect control), the auth backend (a user
  database, login/verification endpoints, admin user management —
  create, disable, reset, expire), and the six open questions
  that need Ali's rulings; design ahead of code, nothing built
  before the spec exists. The environment is READY (verify with
  `. ~/sdk/env.sh && go version` first; expect wipes at any
  boundary and reinstall per doc 06 §1 + the URL wrinkle when
  they land). Open offer on the table, awaiting Ali's ruling:
  attaching the APK to every release page (relaxing D84's
  no-duplicate discipline) — do NOT do it unilaterally.
