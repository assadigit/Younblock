# Prompt — Continue Younblock: the device-test feedback loop (M8-w — the debug.14 round: the report WITH stall-killed lines, shell line versionCode 13)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-v close, 2026-10-19)

- **THE FIFTH PASTED REPORT ARRIVED and it WAS the salvage build (the
  M8-v round, a build round): its shell line read versionCode 12 and
  its log carried the salvage's own lines — the round the M8-u
  no-build round was waiting for.** It PROVED the salvage on-device
  (`tunnel first-byte retry reason=… attempt=N` lines absorbing dead
  attempts, `tunnel first-byte salvaged dials=1/2` saving flows at
  0.7–1.4 s, errors down to 37/81 ≈ 46 % from 67/114 ≈ 59 %) and
  named the two remaining start-lag sources with new precision:
  (1) the SERIAL salvage chain — 21 terminal `reason=timeout` dones
  at 8014–8227 ms = three 2.5 s budgets plus dials walked one after
  another (+ 11 terminal `reason=reset` dones at 0.6–1.9 s); (2) the
  mid-stream stall — five video-start flows done in the same instant
  at 00:18:17.2, dur=57290–64620 ms, up=2.4–5.0 KB, down=6.6–7.7 KB
  (one 121.1 KB): first byte plus ~7 KB, then TOTAL silence until the
  APP's own ~60 s patience killed them; the retry wave right after
  answered in 60–261 ms first-byte (the video took ~65 s to start).
  Counters otherwise healthy: DNS ok 150 / fail 0, rstUnmapped=0,
  doorFailures=0, udpDropped=78.
- **`v1.6.6-android-debug.14` (versionCode 13, Latest, the APK asset
  named v1.6.6-debug14.apk — D103) ships the fix the report drove
  (ADR-0020, doc 08):** (1) **the salvage race** — when attempt 0
  dies at first byte, the engine no longer walks the remaining
  `FirstByteSalvageDials` re-dials one budget at a time; it RACES
  them: re-resolve, dead-IP exclusion, dedupe, rotated lists so the
  candidates start on distinct live origins (each candidate's
  DialHost keeps the M8-n stagger failover), the buffered client
  bytes replayed byte-exact through each, and the FIRST origin to
  speak wins (losers closed and reaped; one demotion per dead
  attempt, the M8-o loop preserved; the retry lines keep their
  format, one per launched candidate; the attempt count is
  UNCHANGED at 1+2, so the same-IP transient-reset recovery
  survives) — a silent-host terminal costs ~2x the budget, never
  the serial chain's 3x; (2) **the stall watch** — new knob
  `StallTimeout` (default 15 s; 0 → default; negative → disabled,
  the M8-u behavior exactly): a budgeted flow's relay that goes
  silent in BOTH directions for the budget is closed engine-side
  (both ends so the app's read errors NOW, the answer set demoted,
  the error counted, new report line `tunnel stall killed idle=Nms`)
  — the app retries onto a fresh origin (with its own budget +
  salvage) in seconds instead of a minute; one-direction silence
  never fires (a pure download or upload is healthy), and unrouted
  relays keep the plain shape byte-for-byte (D46). The both-ways
  proof: with tunnel.go stashed (the M8-t serial shape),
  `TestStallKillClosesSilentRelayFlow`, `TestSalvageRaceRedialsInParallel`,
  `TestSalvageRaceTerminalIsBounded` and the updated
  `TestFirstByteSalvageRecoversDeadOrigin` all FAIL; with the fix
  all sixteen first-byte/salvage/stall tests PASS; the guards pass
  both ways (the streaming-download guard caught an inverted-stamp
  bug mid-round — the watch must measure silence from the NEWEST
  activity clock, or a download whose client never writes dies
  mid-stream).
- **The method-policy question is LIVE for Ali (D102, his ruling,
  NOT our code):** this report's evidence makes it concrete —
  terminal first-byte failures STILL cluster under method=strong
  (21 timeouts + 11 resets, ~40 % of conns) even with the salvage
  absorbing per-node deaths; should a flow that exhausts its RACED
  salvages be retried once with the NEXT preset in HealSweepOrder
  (per-flow method variation)? No steering or policy change without
  his ruling.
- **D102 stays reserved for Ali's rulings** (none landed in M8-v);
  D94-D103 are spent. The account-gate lane stays CLOSED (D97).
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4-8, row 9's device half (udpDropped=78 this
  report; the QUIC path already fast-fails via ICMP
  port-unreachable, so it is not a startup-lag contributor),
  rotation/TalkBack, the data-usage round, D93's desktop check.
- The dormant desktop release lane and public-release engineering
  remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything disagrees with the
  repository, **the repository wins** — report the discrepancy.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-w)

For each issue Ali reports from `v1.6.6-android-debug.14` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names **versionCode 13**. (The M8-u
  lesson, now standing doctrine: a versionCode 12 report means the
  old APK is still installed — the round is the INSTALL again, not a
  fix. The D103 names make the wrong build self-identifying: the
  correct file is v1.6.6-debug14.apk; if the filename says anything
  else, it is the wrong build. Install over debug.13 in place —
  same pinned certificate, higher versionCode, settings and usage
  survive — then reconnect and confirm the report header reads
  versionCode 13.) Then read it in this order: (a) the NEW
  **`tunnel stall killed idle=Nms`** lines — the stall watch firing
  on dead relays (how many, at what idle values, and whether video
  starts still stall when they do not fire — the StallTimeout tuning
  material: too many kills on healthy-looking flows → raise the
  budget; no kills while starts still stall → the stall is
  elsewhere); (b) the **`tunnel first-byte retry … attempt=N`** and
  **`salvaged dials=N`** lines vs the terminal **`first-byte
  failed`** lines — the raced salvage's absorption ratio (terminal
  dones should now cluster at ~5 s = 2x the 2.5 s budget, not ~8 s
  = 3x); (c) the done lines — a stall-killed flow's done line
  carries its byte totals right after the stall line; (d) the
  `tunnel up dns=… dial=…` lines and any dns server-failed lines as
  before; (e) the tun: line (flows/udpDropped/doorFailures).
  Root-cause from the evidence; fix at the correct layer
  (engine/facade/Kotlin/WebView); keep the D77-D103 honesty rules
  binding.
- **Also surface/collect Ali's ruling on the D102 method question**
  (doc 10 row 3 + doc 09 handoff carry its exact wording): whether a
  flow that exhausts its RACED salvages under one preset should be
  retried once with the NEXT preset. If he rules, record it in
  DECISIONS.md as D102 BEFORE any code.
- **If the report is absent**, say so plainly and ask for it (menu —
  Diagnostics is the first row — Copy, paste it here) — do NOT guess.
  Offer the honest alternatives; do not invent work.
- Record each closed checklist row with Ali's measured evidence;
  rows without observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule) and cut the next debug release — a REGULAR release in the
  private repo, never flagged prerelease (D89), **carrying the debug
  APK asset per D98, NAMED v<versionName>-debug<N>.apk per D103
  (amending D99; N = the tag's build sequence — debug.15's asset
  is v1.6.6-debug15.apk)** —
  through the release lane (push the code commit BEFORE creating the
  release; upload the APK, verify via the API, record size + digest)
  WHEN a code change warrants it. Full lane before release: desktop
  baseline in the D74 order (frontend npm ci → svelte-check 0/0 →
  vite build → CLEAN-sync dist → go vet → go test 18/18 ×2 →
  GOOS=windows exit 0); the compile matrix in its exact form
  (CGO_ENABLED=0 `go build ./internal/... ./mobile/...` per ABI ×4,
  plus arm64 linking the full tree `./...` with CGO_ENABLED=0);
  `gomobile bind` (GOFLAGS=-mod=mod, the recorded
  `-target=android/arm,android/arm64,android/amd64` form, androidapi
  26); clean `assembleDebug` (if it fails on "Toolchain installation
  … does not provide the required capabilities: [JAVA_COMPILER]", a
  stale gradle daemon holds the system JRE — `./gradlew --stop` then
  `JAVA_HOME=~/sdk/jdk ./gradlew clean assembleDebug`); artifact
  verification (sha256, apksigner, badging); the served-variant check
  (D101) for any UI change; commit. **Long builds run as FOREGROUND
  CHUNKS — detached jobs die (the M8-t lesson, reconfirmed); manage
  disk per the M8-t lesson (`go clean -cache` before the bind if
  free space is under ~2.5 GB).**

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog.
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-v,
   M8-w, … The next decision number is D102 — reserved for Ali's
   rulings; record his rulings there BEFORE any code.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.14 edition + §"Device test circle" — row 3 is the stall row
   with the fifth-report record and the two named evidence lines),
   doc 06 (the M8-c…M8-v records), doc 08 + `DECISIONS.md` (D1-D103;
   read D97, D98, D99, D100, D101, D103 in full; **ADR-0020 is the
   M8-v decision**),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-v commit
   on top — the salvage race + the stall watch + ADR-0020, over
   af292ee, the M8-u-b D103 commit). Preserve any uncommitted work.
   If push is rejected (credential rotated), the session prompt
   carries a fresh token.
4. Check whether Ali's debug.14 device feedback has arrived. If a
   pasted diagnostics report is present: the feedback loop above,
   the report first. If not: say so plainly and put the honest
   alternatives to Ali.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER: `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, twice);
   `GOOS=windows go build ./...` (exit 0). **Wipes happen at ANY
   boundary — VERIFY the toolchain first (`. ~/sdk/env.sh && go
   version && javac -version`) and only reinstall if actually wiped
   (the cmdline-tools `12.0` alias URL 404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`, 153,607,504 B;
   reinstall per doc 06 §1).**

## Environment (this sandbox resets without warning — verify, don't assume)

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor` for host builds; the gomobile bind runs
  GOFLAGS=-mod=mod; the Android lane never invokes wails; desktop
  lanes keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**
  (it also sets JAVA_HOME, ANDROID_HOME and puts ~/go/bin,
  platform-tools and cmdline-tools on PATH).
- JDK 21.0.12.1+1 (Temurin) at `~/sdk/jdk`; Android SDK at
  `~/sdk/android` (platforms 26/36, build-tools 36.1.0, NDK
  28.2.13676358, gomobile/gobind at the pinned x/mobile). Exact
  pins: doc 06 §1.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore` — after a wipe + re-clone,
  assembleDebug signs identically with NO extra step.
- No /dev/kvm (re-verified every session). Disk is a ~9.9 GB box —
  **`go clean -cache` before the gomobile bind when free space is
  under ~2.5 GB (the M8-t/M8-v lesson).**
- **The sandbox's background killer kills detached jobs mid-run.
  Run long builds as FOREGROUND CHUNKS with the caches carrying the
  progress (frontend / gomobile bind / gradle, each within a command
  timeout); short detaches may still work but VERIFY liveness before
  trusting a log that stops growing.**
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew` **and `chmod +x build-android.sh`**.
- **Formatting (the M8-n…M8-t hazard, hit every session): the Edit
  tool WRITES SPACES file-wide into tab-indented .go files — edit
  tab files with the Edit tool then run `~/sdk/go/bin/gofmt -w` on
  each (check `git diff --stat` stays surgical). Some files are
  SPACE-indented at HEAD on purpose (tunnel.go, flowtiming_test.go,
  health_test.go, settings_test.go, usage_lifecycle_test.go) —
  write their edits in the file's own style; do NOT gofmt those.**

If the sandbox was wiped (repo / toolchain / Go missing):

1. Re-clone into `/home/z/my-project/Younblock` from the remote above.
2. Reinstall the toolchain exactly per doc 06's recorded pins.
3. Re-run the desktop baseline before any new code.

## Standing rules (unchanged)

- Evidence or it didn't happen: no claimed test pass, build, APK or
  push without a verified result. ⛔ with evidence for anything the
  environment cannot run — never fake completion, never silently
  narrow scope.
- Scoped commits; small steps; baseline green after each; no
  force-push; no unrelated changes; no secrets or RELEASE signing
  keys committed (D85's deliberate exception: the debug test-circle
  keystore is vendored); preserve Windows/Linux desktop behavior.
- The ADRs (doc 08, ADR-0001…0020) are the ground truth.
- The honesty rules D77-D103 bind; a surface ships in the variant
  the edition serves, verified through the real webui server
  (D101/ADR-0016).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-w / M8-x / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug<N>.apk per D103
  (amending D99)** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.14 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity) and name the FIRST
issue you will address from its evidence; if only prose arrived, ask
for the report instead of guessing; (5) confirmation that you have
read D97 (the auth lane stays untouched), D98 (every release carries
the APK), D99 (the APK asset is named with the version — as amended
by D103: the build number rides in the filename), D100 (the
diagnostics surface is the loop's evidence path), D101 (a surface
ships in the variant the edition serves) and D103 (the APK asset is
named v<versionName>-debug<N>.apk). If no feedback arrived,
say so plainly and put the honest alternatives to Ali. Then execute.
