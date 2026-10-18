# Prompt — Continue Younblock: the device-test feedback loop (M8-r — the first diagnostics-report round, on the screen the phone serves)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-q close, 2026-10-18 late)

- **The diagnostics surface finally ships on the screen the phone
  actually serves, as `v1.6.6-android-debug.10` (versionCode 9,
  Latest, the APK asset named v1.6.6-debug.apk).** Ali's debug.9
  round: "I still dont see diagnostic button" — exact, and the root
  cause was in the build: the M8-p surface (debug.9) shipped ONLY
  into the classic UI tree, but the APK bakes the PRO screen
  (D66: `build-android.sh` leaves `VITE_UI_VARIANT` unset, so
  `main.ts`'s dispatcher defaults to pro; only the desktop wails
  lanes bake classic). The phone's only diagnostics was the M6-era
  sheet section buried below Help, with the raw `navigator.clipboard`
  copy that silently fails in the WebView. **What the APK now
  carries (ADR-0016, the port): Diagnostics as the FIRST row of the
  menu sheet — every platform, no gate, no scrolling — opening the
  full-screen report view: Copy (the same honest three-path ladder —
  the Android-native shell bridge first, navigator.clipboard second,
  execCommand third), Share (the system share sheet, APK only),
  Refresh, and the selectable text itself as the last resort.** The
  old buried section is REMOVED. The engine report (TUN counters,
  the "android shell: 1.6.6 (versionCode 9)" build identity, the
  200-line window) is unchanged from debug.9 — now actually
  reachable on the device. The surface was verified END-TO-END
  through the real webui server (`cmd/core --web` — the same
  embedded dist + dispatcher the APK serves) before release.
- **D101 (this round's ruling, read it in full): a user-facing
  surface ships in the variant(s) the edition serves.** Name the
  baked variant per edition (Android + the web single-file edition →
  pro; the Windows/Linux wails desktops → classic) and verify the
  surface through the real webui server, not only in the source
  tree. A feature merged into one UI tree is invisible to every
  edition that bakes the other.
- **The stall itself is NOT root-caused a third time — by design.**
  Two host-proven fixes are device-disproven in a row (ADR-0013 in
  debug.7, ADR-0014 in debug.8). The next root-cause waits for a
  REAL PASTED REPORT, not a guess. Doc 10's circle row 3 stays
  open; its expected input is now the first row of the menu.
- **D100 stands:** the diagnostics surface is the loop's standing
  evidence path — a "still broken" device report without the pasted
  diagnostics report is incomplete.
- **The account-gate lane stays CLOSED (D97)** — cancelled before any
  code; `docs/auth/` is bannered history; ADR-AUTH-0001 retired;
  the A-series Task IDs are retired; Younblock stays ONE
  free-for-all app — no login, no server, no editions. Read D97 in
  full in DECISIONS.md before doing anything.
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4–8 (browser routing, .ir direct, stop paths,
  persistence/battery, network switch), row 9's device half (the
  QUIC fallback observation — the tun: udpDropped counter is exactly
  its evidence surface), rotation/TalkBack, the data-usage round,
  and D93's desktop check.
- **The dormant desktop release lane** (the rounds under CHANGELOG
  "Unreleased") and **public-release engineering** remain
  open-but-paused by choice. Both UI trees now carry the Diagnostics
  surface (classic since debug.9, pro since debug.10).
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-r)

For each issue Ali reports from `v1.6.6-android-debug.10` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names versionCode 9 (the report's own
  build identity — a versionCode 8 report means the old APK is still
  installed and the round starts with the install). Then read it
  against the stall: (a) the `tunnel up dns=…ms dial=…ms` lines —
  slow dns/dial = the network answer path (the DoH servers or the
  address sets); (b) `tunnel first-byte`/`tunnel done dur=…` — fast
  setup + slow dur = a throughput leg; (c) NOTHING logged for the
  stalled video = the flow never reached the door (a stack/app-side
  suspect — and the `tun:` line's flows counter plus udpDropped name
  exactly that case; udpDropped climbing = the QUIC row 9 suspect);
  (d) high on-screen speed while the player still buffers = the
  player's own transport. Root-cause from the evidence; fix at the
  correct layer (engine/facade/Kotlin/WebView); keep the D77–D101
  honesty rules binding.
- **Also ingest the surface's own verdict** (it rides the same
  round): is Diagnostics the first row when the menu opens; does
  Copy actually copy (the honest status); does Share open the
  system sheet; does the report render. A broken evidence path is
  itself a defect — fix it before anything else, and per D101 fix
  it in the SERVED variant (the pro tree), verified through the
  real webui server.
- **If the report is absent**, say so plainly and ask for it (open
  the menu — Diagnostics is the first row — open it, Copy, paste it
  here; even with nothing stalled, its build line proves the
  round) — do NOT guess the third root-cause; two disproven fixes
  in a row make guessing the one thing this loop must not do.
  Offer the honest alternatives — the desktop release lane or
  public-release engineering — his pick, or a new feature direction
  if he has one (recorded as a decision before any code, per the
  D94 pattern).
- Record each closed checklist row with Ali's measured evidence
  (doc 07 §9 / doc 10 §"Device test circle"); rows without
  observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule) and cut the next debug release — a REGULAR release in the
  private repo, never flagged prerelease (D89), **carrying the
  debug APK asset per D98, NAMED v<versionName>-debug.apk per D99** —
  through the release lane (push the code commit BEFORE creating the
  release — the full-SHA target_commitish 422s against an unpushed
  SHA; upload the APK, verify via the API, record asset size +
  digest) WHEN a code change warrants it — do not re-release
  identical bytes. Full lane before release: desktop baseline in the
  D74 order (frontend npm ci → svelte-check 0/0 → vite build →
  CLEAN-sync dist → go vet → go test 18/18 ×2 → GOOS=windows exit
  0); the compile matrix in its exact form (CGO_ENABLED=0
  `go build ./internal/... ./mobile/...` = 19 non-main packages per
  ABI ×4, plus arm64 linking the full tree `./...` with
  CGO_ENABLED=0 — do NOT run the arm64 full-tree link with the host
  gcc); `gomobile bind` (GOFLAGS=-mod=mod, the recorded
  `-target=android/arm,android/arm64,android/amd64` form, androidapi
  26); **clean** `assembleDebug` (if it fails on "Toolchain
  installation … does not provide the required capabilities:
  [JAVA_COMPILER]", a stale gradle daemon holds the system JRE —
  `./gradlew --stop` then `JAVA_HOME=~/sdk/jdk ./gradlew clean
  assembleDebug`); artifact verification (sha256, apksigner,
  badging — doc 06's records); **the served-variant check (D101):
  verify any UI change through the real webui server
  (`go build -o /tmp/core ./cmd/core && /tmp/core --web --webport
  <port> --no-open`) in a browser at a 360×760 viewport before
  release**; commit.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-r,
   M8-s, … **The auth lane's Task IDs (A1, A2, A3) are RETIRED —
   cancelled with the feature at D97; never open them. The next
   decision number is D102 — reserved for Ali's rulings; D94–D101
   are spent (D98 the APK-on-every-release ruling, D99 the
   APK-naming ruling, D100 the diagnostics-surface ruling, D101 the
   served-variant rule).**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.10 edition + §"Device test circle" — row 3 is the stall
   row, now the first row of the menu), doc 06 (the M8-c…M8-q
   records), doc 08 + `DECISIONS.md` (D1–D101; **read D97, D98,
   D99, D100 and D101 in full**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-q commit
   — the served-variant port + D101 — on top of 65fe1d4, the M8-p
   classic-tree surface). Preserve any uncommitted work; never
   reset it just to get a clean diff. If push is rejected
   (credential rotated), the session prompt carries a fresh token —
   update the `origin` remote URL with it.
4. Check whether Ali's debug.10 device feedback has arrived (session
   messages). If a pasted diagnostics report is present: the
   feedback loop above, the report first. If not: say so plainly
   and put the honest alternatives to Ali — do not guess, do not
   invent work.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER (a fresh clone fails the Go lanes until the frontend is
   built): `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); then `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, run twice
   consecutively); `GOOS=windows go build ./...` (exit 0).
   **~/sdk was intact at the M8-q close; wipes happen at ANY
   boundary — VERIFY first (`. ~/sdk/env.sh && go version &&
   javac -version`) and only reinstall if actually wiped (the
   cmdline-tools `12.0` alias URL 404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`).**

## Environment (this sandbox resets without warning — verify, don't assume)

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor` for host builds; the gomobile bind runs
  GOFLAGS=-mod=mod; the Android lane never invokes wails; desktop
  lanes keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**
  (the env.sh also sets JAVA_HOME, ANDROID_HOME and puts ~/go/bin,
  platform-tools and cmdline-tools on PATH).
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk`.
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools
  37.0.1, platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358,
  licenses accepted; gomobile/gobind in `~/go/bin` at the pinned
  x/mobile). Exact pins: doc 06 §1.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore` — after a wipe + re-clone,
  assembleDebug signs identically with NO extra step.
- No /dev/kvm (re-verified every session). Disk is a ~9.9 GB box —
  clean Go/Gradle build caches before big builds.
- **The sandbox kills background processes when a Bash command ends
  when they stay in the command's process group — a double-fork
  detach (`( setsid <cmd> > log 2>&1 < /dev/null & )`) survives
  across invocations (proven at M8-q for the webui verification
  server).**
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew` **and `chmod +x build-android.sh`** (the
  lane script lost its +x at the M8-q session — restored, but a
  fresh clone needs it again).
- **Formatting note (learned the hard way at M8-n): the .go sources
  are TAB-indented (gofmt-clean); some _test.go files are not. The
  Read tool renders tabs as spaces — write Go changes carefully and
  run `~/sdk/go/bin/gofmt -l <file>` against HEAD vs worktree, or
  `gofmt -w` your touched files, so the diff stays surgical.**

If the sandbox was wiped (repo / toolchain / Go missing):

1. Re-clone into `/home/z/my-project/Younblock` from the remote above
   (the token lives in the old clone's `origin` if any copy survives;
   otherwise the session prompt carries it — re-cloning works without
   asking anyone).
2. Reinstall the toolchain exactly per doc 06's recorded pins (Go to
   `~/sdk/go`; JDK, SDK and NDK at the pinned versions; gomobile/
   gobind via `go install` at x/mobile v0.0.0-20261011034528-197770444867;
   module cache via proxy.golang.org; then rely on the committed
   `vendor/`).
3. Re-run the desktop baseline before any new code, and expect the
   first cold-cache run to be slow.

## Standing rules (unchanged)

- Evidence or it didn't happen: no claimed test pass, build, APK or
  push without a verified result. ⛔ with evidence for anything the
  environment cannot run — never fake completion, never silently
  narrow scope.
- Scoped commits; small steps; baseline green after each; no
  force-push; no unrelated changes; **no secrets or RELEASE signing
  keys committed** (D85's deliberate exception: the debug test-circle
  keystore is vendored — the well-known debug identity, not a
  credential); preserve Windows/Linux desktop behavior (the full
  regression gate is part of every change set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08, now ADR-0001…0016) are the ground truth.
- The honesty rules D77–D101 bind: the UI/release notes may only
  claim what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped; no
  recommendation may be recorded as a ruling; a cancellation is
  recorded as exactly what it is (D97 is the pattern); a copy button
  may only toast success when a path delivered (ADR-0015's ladder);
  **a surface ships in the variant the edition serves, verified
  through the real webui server (D101/ADR-0016).**
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-r / M8-s / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug.apk per D99** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.10 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity) and name the FIRST
issue you will address from its evidence; if only prose arrived, ask
for the report (open the menu — Diagnostics is the first row — open
it, Copy, paste it here) instead of guessing; (5) confirmation that
you have read D97 (the auth lane stays untouched), D98 (every release
carries the APK), D99 (the APK asset is named with the version),
D100 (the diagnostics surface is the loop's evidence path) and D101
(a surface ships in the variant the edition serves). If no feedback
arrived, say so plainly and put the honest alternatives to Ali. Then
execute.
