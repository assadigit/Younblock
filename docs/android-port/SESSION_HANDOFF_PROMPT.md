# Prompt — Continue Younblock: the device-test feedback loop (M8-v — the debug.13 round: the report WITH retry/salvaged lines, shell line versionCode 12)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-u close, 2026-10-19)

- **THE FOURTH PASTED REPORT ARRIVED — but it ran the PREVIOUS
  build (M8-u, a NO-BUILD round).** Ali pasted "The problem
  persists" again with a report generated 23:41:37 UTC, but its
  "android shell: 1.6.6 (versionCode 11)" line names debug.12 — the
  build debug.13 replaces — and the identity is authoritative
  (MainActivity reads it from Android's PackageManager at runtime).
  The log confirmed it three ways: 21 `reason=timeout` dones at
  8054–8070 ms (the old 8 s budget — the 2.5 s build cannot produce
  an 8 s death), zero retry/salvaged lines in the whole 200-line
  window, and the clock: **debug.13 was published at 23:16:28 UTC —
  twenty-five minutes BEFORE the report was generated.** The report
  carried zero bytes of salvage-build evidence (it was the third
  report's signature measured again: 21 timeouts + 17 resets,
  67 errors / 114 conns, healthy first-bytes 58–548 ms, DNS ok
  118 / fail 0, udpDropped=107). **The round's lesson, now standing
  doctrine: the fix can ship while the phone still runs the old
  build — EVERY report's first check is its shell line, and D99
  gives every release the same APK asset name (v1.6.6-debug.apk), so
  the release PAGE is what disambiguates which build Ali gets.**
  debug.13 was re-verified end-to-end at M8-u (asset hash-matched,
  salvage lines in libgojni.so in all three ABIs, Latest). No
  versionCode bump, no release cut (D84: no app change).
- **THE THIRD PASTED DIAGNOSTICS REPORT ARRIVED and was acted on:
  `v1.6.6-android-debug.13` (versionCode 12, Latest, the APK asset
  named v1.6.6-debug.apk) ships the fix it drove (ADR-0019).** Ali's
  debug.12 round: "The problem persists" + the full report — its
  "android shell: 1.6.6 (versionCode 11)" line proved the debug.12
  install. **The M8-s budget was PROVEN on-device exactly as
  designed:** 22 `tunnel first-byte failed reason=timeout` dones at
  ~8.3 s (the budget firing on silent origins) + 18 `reason=reset`
  dones at ~0.5 s — the tuning evidence ADR-0018 asked for. The
  counters named the remaining per-video-start lag: 38 errors / 61
  conns — roughly half the video-start waves die at first byte and
  the app re-opens wave after wave (~35 s from the first wave to the
  first video bytes, down=4.4 MB once a flow survived). Every
  budget-killed flow was 100 % silent (`down=0 B`; the same flows
  sat out the app's full 60 s patience in the previous report —
  dead nodes, not slow ones); healthy routed origins answered in
  420–508 ms (one 2399 ms outlier carrying the fragment-pause
  write). **What debug.13 carries (ADR-0019): (1) the budget
  retuned 8 s → 2.5 s; (2) the salvage re-dial — a dead first-byte
  origin is re-dialed INSIDE the same flow (the engine closes only
  the origin, demotes the answer set per dead attempt, re-resolves
  excluding the flow's dead IPs, re-dials the SAME host with the
  SAME preset captured once per flow, and replays the client's
  buffered bytes byte-exact through a fresh fragment conn), up to
  FirstByteSalvageDials re-dials (default 2 → 3 total attempts) —
  the app never sees the failure, so its serial retry chain
  collapses into bounded in-engine attempts. New report lines:
  "tunnel first-byte retry reason=… attempt=N" (absorbed) and
  "tunnel first-byte salvaged dials=N" (saved). Unrouted relays
  keep the plain shape byte-for-byte.** The host reproduction is
  the round's proof: the three new firstbyte_test.go tests FAIL
  with the salvage stashed (the M8-s shape) and PASS with it.
- **The method-policy question is LIVE for Ali, SHARPENED (D102
  material, his ruling, NOT our code):** the salvage re-flips the
  SAME coin (same preset) up to twice more per flow; if the next
  report still shows terminal first-byte failures clustering under
  one method, should a flow that exhausts its salvages be retried
  once with the NEXT preset in HealSweepOrder (per-flow method
  variation)? No steering or policy change without his ruling.
- **D102 stays reserved for Ali's rulings** (none landed in M8-t);
  D94-D101 are spent. The account-gate lane stays CLOSED (D97).
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4-8, row 9's device half (udpDropped climbed to
  88 this report; the QUIC path already fast-fails via ICMP
  port-unreachable, so it is not a startup-lag contributor),
  rotation/TalkBack, the data-usage round, D93's desktop check.
- The dormant desktop release lane and public-release engineering
  remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything disagrees with the
  repository, **the repository wins** — report the discrepancy.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-v)

For each issue Ali reports from `v1.6.6-android-debug.13` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names **versionCode 12**. (M8-u proved
  this check is not a formality: a versionCode 11 report means the
  old APK is still installed — the round is the INSTALL again, not a
  fix. Before anything else, establish WHICH release page the APK
  came from: D99 names every release's asset v1.6.6-debug.apk, so
  the page — not the filename — is what disambiguates. The install
  itself is ordinary: download v1.6.6-debug.apk from the
  v1.6.6-android-debug.13 release page, install over the old app in
  place — same pinned certificate, higher versionCode, settings and
  usage survive — then reconnect and confirm the report header reads
  versionCode 12.) Then read it in this order: (a) the NEW
  **`tunnel first-byte retry reason=… attempt=N`** lines — dead
  attempts the salvage ABSORBED (the app never saw them); (b) the
  NEW **`tunnel first-byte salvaged dials=N`** lines — flows the
  salvage SAVED (video starts that would have been app retries);
  (c) the terminal **`tunnel first-byte failed reason=…`** lines —
  flows that exhausted all 3 attempts (their ratio against the
  salvaged lines is the tuning evidence for the budget AND the
  D102 material: persistent terminal failures under one method =
  the NEXT-preset question is live); (d) the done lines —
  `dur≈2500ms down=0 B` dones are the retuned budget firing;
  `dur` in the ~1-3 s range with `down>0` on salvaged flows is the
  salvage's cost, working as designed; (e) the `tunnel up dns=…
  dial=…` lines and any dns server-failed lines as before; (f) the
  tun: line (flows/udpDropped/doorFailures). Root-cause from the
  evidence; fix at the correct layer (engine/facade/Kotlin/WebView);
  keep the D77-D101 honesty rules binding.
- **Also surface/collect Ali's ruling on the D102 method question**
  (doc 10 row 3 + doc 09 handoff carry its exact wording): whether a
  flow that exhausts its salvages under one preset should be retried
  once with the NEXT preset. If he rules, record it in DECISIONS.md
  as D102 BEFORE any code.
- **If the report is absent**, say so plainly and ask for it (menu —
  Diagnostics is the first row — Copy, paste it here) — do NOT guess.
  Offer the honest alternatives; do not invent work.
- Record each closed checklist row with Ali's measured evidence;
  rows without observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule) and cut the next debug release — a REGULAR release in the
  private repo, never flagged prerelease (D89), **carrying the debug
  APK asset per D98, NAMED v<versionName>-debug.apk per D99** —
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
  (D101) for any UI change; commit. **The M8-t lane ran as FOREGROUND
  CHUNKS — see the environment section: detached jobs died.**

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog.
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-u,
   M8-v, … The next decision number is D102 — reserved for Ali's
   rulings; record his rulings there BEFORE any code.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.13 edition + §"Device test circle" — row 3 is the stall row
   with the D102 question and the fourth-report note), doc 06 (the
   M8-c…M8-u records), doc 08 + `DECISIONS.md` (D1-D101; read D97,
   D98, D99, D100, D101 in full; **ADR-0019 is the M8-t decision**),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-u commit
   on top — a docs-only record of the no-build round (doc 06/09/10,
   PROGRESS, this prompt) over 6b3520a, the M8-t commit — the
   salvage re-dial + ADR-0019). Preserve any uncommitted work. If
   push is rejected (credential rotated), the session prompt carries
   a fresh token.
4. Check whether Ali's debug.13 device feedback has arrived. If a
   pasted diagnostics report is present: the feedback loop above,
   the report first. If not: say so plainly and put the honest
   alternatives to Ali.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER: `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, twice);
   `GOOS=windows go build ./...` (exit 0). **~/sdk was REINSTALLED
   at M8-t after the SIXTH wipe; wipes happen at ANY boundary —
   VERIFY first (`. ~/sdk/env.sh && go version && javac -version`)
   and only reinstall if actually wiped (the cmdline-tools `12.0`
   alias URL 404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`, 153,607,504 B).**

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
  **the M8-t session cleared the Go build cache mid-round (before
  the SDK install; 4.0 GB free after); expect `go clean -cache`
  again before any big Android build.**
- **The sandbox's background killer got MORE AGGRESSIVE at M8-t: it
  killed BOTH detached jobs mid-run (the setsid sdkmanager install —
  2.5 GB of stale .temp — AND the setsid build-android.sh lane at
  its vite step). Run long builds as FOREGROUND CHUNKS with the
  caches carrying the progress (frontend / gomobile bind / gradle,
  each within a command timeout); short detaches may still work but
  VERIFY liveness before trusting a log that stops growing.**
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew` **and `chmod +x build-android.sh`**.
- **Formatting (the M8-n…M8-t hazard, hit every session): the Edit
  tool WRITES SPACES file-wide into tab-indented .go files — the
  M8-t session edited tab files with the Edit tool then ran
  `~/sdk/go/bin/gofmt -w` on each (check `git diff --stat` stays
  surgical; the engine.go struct-literal realignment from a longer
  field name is the expected gofmt column shift). Some files are
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
- The ADRs (doc 08, ADR-0001…0019) are the ground truth.
- The honesty rules D77-D101 bind; a surface ships in the variant
  the edition serves, verified through the real webui server
  (D101/ADR-0016).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-v / M8-w / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug.apk per D99** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.13 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity) and name the FIRST
issue you will address from its evidence; if only prose arrived, ask
for the report instead of guessing; (5) confirmation that you have
read D97 (the auth lane stays untouched), D98 (every release carries
the APK), D99 (the APK asset is named with the version), D100 (the
diagnostics surface is the loop's evidence path) and D101 (a surface
ships in the variant the edition serves). If no feedback arrived,
say so plainly and put the honest alternatives to Ali. Then execute.
