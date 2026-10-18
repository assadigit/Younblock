# Prompt — Continue Younblock: the device-test feedback loop (M8-q — the first diagnostics-report round)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-p close, 2026-10-18 night)

- **The in-app diagnostics surface — the evidence path the device
  loop was missing — is SHIPPED as `v1.6.6-android-debug.9`
  (versionCode 8, Latest, the APK asset named v1.6.6-debug.apk).**
  Ali's debug.8 round: the per-video stall PERSISTED and the
  promised input did not exist — "there is no diagnostic option in
  the apk app setting. add an option to copy paste logs and
  diagnostics so i can paste it here" (recorded as D100; shipped as
  ADR-0015). What the APK now carries: a **Diagnostics screen** in
  the menu (between Help and About) AND as a row in Settings, with
  **Copy** (an honest three-path clipboard ladder — the
  Android-native shell bridge first, navigator.clipboard second, an
  execCommand fallback third; "Copied" toasts only on real success),
  **Share** (the system share sheet, APK only) and the selectable
  text itself as the path of last resort; the report now carries the
  **TUN counters** (flows/rstUnmapped/overCap/doorFailures/
  dnsQueries/udpDropped — the one surface naming the "flow never
  reached the door" case and the QUIC suspect) and the **exact build
  identity** ("android shell: 1.6.6 (versionCode 8)" — a pasted
  report names its own release); the report's log window doubled to
  **200 lines**.
- **The stall itself is NOT root-caused a third time — by design.**
  Two host-proven fixes are device-disproven in a row (ADR-0013 in
  debug.7: fast dial failover + budget-true handshake timeout;
  ADR-0014 in debug.8: dial-failure → DNS-requery feedback). The
  next root-cause waits for a REAL PASTED REPORT, not a guess. Doc
  10's circle row 3 stays open; its expected input is now one
  reachable button away.
- **D100 (Ali's ruling, this round):** the diagnostics surface is the
  loop's standing evidence path — from now on a "still broken"
  device report without the pasted diagnostics report is incomplete.
- **The account-gate lane stays CLOSED (D97)** — cancelled before any
  code; `docs/auth/` is bannered history; ADR-AUTH-0001 retired;
  the A-series Task IDs are retired; Younblock stays ONE
  free-for-all app — no login, no server, no editions. Read D97 in
  full in DECISIONS.md before doing anything.
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4–8 (browser routing, .ir direct, stop paths,
  persistence/battery, network switch), row 9's device half (the
  QUIC fallback observation — the new tun: udpDropped counter is
  exactly its evidence surface), rotation/TalkBack, the data-usage
  round, and D93's desktop check.
- **The dormant desktop release lane** (the rounds under CHANGELOG
  "Unreleased") and **public-release engineering** remain
  open-but-paused by choice. Note: the debug.9 tree's desktop
  build now carries the Diagnostics screen too (one UI, every
  edition — minus Share).
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-q)

For each issue Ali reports from `v1.6.6-android-debug.9` (device side)
or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names versionCode 8 (the report's own
  build identity — if not, he is still on an older APK and the round
  starts with the install). Then read it against the stall:
  (a) the `tunnel up dns=…ms dial=…ms` lines — slow dns/dial = the
  network answer path (the DoH servers or the address sets);
  (b) `tunnel first-byte`/`tunnel done dur=…` — fast setup + slow dur
  = a throughput leg; (c) NOTHING logged for the stalled video = the
  flow never reached the door (a stack/app-side suspect — and the
  `tun:` line's flows counter plus udpDropped name exactly that
  case; udpDropped climbing = the QUIC row 9 suspect); (d) high
  on-screen speed while the player still buffers = the player's own
  transport. Root-cause from the evidence; fix at the correct layer
  (engine/facade/Kotlin/WebView); keep the D77–D100 honesty rules
  binding.
- **Also ingest the surface's own verdict** (it rides the same
  round): did Copy actually copy (the honest toast), did Share open
  the system sheet, does the report render. A broken evidence path
  is itself a defect — fix it before anything else.
- **If the report is absent**, say so plainly and ask for it (Menu →
  Diagnostics → Copy after a stall) — do NOT guess the third
  root-cause; two disproven fixes in a row make guessing the one
  thing this loop must not do. Offer the honest alternatives — the
  desktop release lane or public-release engineering — his pick, or
  a new feature direction if he has one (recorded as a decision
  before any code, per the D94 pattern).
- Record each closed checklist row with Ali's measured evidence
  (doc 07 §9 / doc 10 §"Device test circle"); rows without
  observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule) and cut the next debug release — a REGULAR release in the
  private repo, never flagged prerelease (D89), **carrying the debug
  APK asset per D98, NAMED v<versionName>-debug.apk per D99** —
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
  badging — doc 06's records); commit.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-q,
   M8-r, … **The auth lane's Task IDs (A1, A2, A3) are RETIRED —
   cancelled with the feature at D97; never open them. The next
   decision number is D101 — reserved for Ali's rulings; D94–D100
   are spent (D98 the APK-on-every-release ruling, D99 the
   APK-naming ruling, D100 the diagnostics-surface ruling).**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.9 edition + §"Device test circle" — row 3 is the stall row,
   now one button from its evidence), doc 06 (the M8-c…M8-p
   records), doc 08 + `DECISIONS.md` (D1–D100; **read D97, D98, D99
   and D100 in full**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-p commit
   — the diagnostics surface + D100 — on top of 4b632c0, the M8-o
   per-video fix). Preserve any uncommitted work; never reset it just
   to get a clean diff. If push is rejected (credential rotated), the
   session prompt carries a fresh token — update the `origin` remote
   URL with it.
4. Check whether Ali's debug.9 device feedback has arrived (session
   messages). If a pasted diagnostics report is present: the feedback
   loop above, the report first. If not: say so plainly and put the
   honest alternatives to Ali — do not guess, do not invent work.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER (a fresh clone fails the Go lanes until the frontend is
   built): `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); then `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, run twice
   consecutively); `GOOS=windows go build ./...` (exit 0).
   **~/sdk was intact at the M8-p close; wipes happen at ANY
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
- The sandbox kills background processes when a Bash command ends —
  run long builds as foreground chunks.
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew`.
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
  the ADRs (doc 08, now ADR-0001…0015) are the ground truth.
- The honesty rules D77–D100 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped; no
  recommendation may be recorded as a ruling; a cancellation is
  recorded as exactly what it is (D97 is the pattern); a copy button
  may only toast success when a path delivered (ADR-0015's ladder).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D101, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-q / M8-r / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug.apk per D99** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.9 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity) and name the FIRST
issue you will address from its evidence; if only prose arrived, ask
for the report (Menu → Diagnostics → Copy) instead of guessing;
(5) confirmation that you have read D97 (the auth lane stays
untouched), D98 (every release carries the APK), D99 (the APK asset
is named with the version) and D100 (the diagnostics surface is the
loop's evidence path). If no feedback arrived, say so plainly and put
the honest alternatives to Ali. Then execute.
