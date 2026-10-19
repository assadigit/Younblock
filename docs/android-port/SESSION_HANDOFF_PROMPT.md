# Prompt — Continue Younblock: the device-test feedback loop (M8-y — the debug.16 round: the report WITH tier= lines AND raced lines, shell line versionCode 15)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-x close, 2026-10-19)

- **THE SEVENTH PASTED REPORT ARRIVED and it was STILL the debug.14
  build (versionCode 13) — the device NEVER RAN debug.15.** The D103
  accident class is real: before reading any log line, check the
  "android shell:" line; 13 or 14 means the old APK is still
  installed and the round is the INSTALL again, not a fix. Ali's
  words named the M8-v stall watch's regression himself: "videos
  used to have lag for starting to play, but after that you could
  easily go next (watch next few minutes) instantly without buffer,
  but now even going next and back in the same video lags a lot."
  The report's log is the smoking gun: thirteen
  `tunnel stall killed idle=15xxx ms` lines in ~70 s, every one a
  connection that had ALREADY carried real data (dur=17264 ms
  down=2.1 MB, dur=16756 ms down=1.6 MB, 723.6 KB, 633.7 KB,
  162.6 KB, plus 20.9/23.4/9.2/7.4/5.3/4.4/3.6/2.8 KB) — the
  player's POOLED video connections (buffer, quiet past 15 s, next
  range on the SAME connection) murdered at exactly the lean budget
  mid-pacing, each kill ALSO demoting a proven-good answer set and
  counting an error (errors 70/122 ≈ 57 %; a fresh dial there cost
  reset ~1.2 s / timeout ~5.4 s). The method line flipped to
  balanced (the heal sweep settling after DNS false-failures);
  dns ok 176 / fail 4; rstUnmapped=0, doorFailures=0,
  udpDropped=73.
- **`v1.6.6-android-debug.16` (versionCode 15, Latest, the APK asset
  named v1.6.6-debug16.apk — D103) ships the fix the report drove
  (ADR-0022, doc 08): the TIERED STALL WATCH.** A flow PROVES itself
  by payload (`flow.down >= StallRichBytes`, default 32 KB — past a
  handshake + headers into real payload; the murdered cohort started
  at 162.6 KB, the debug.13 dead cohort topped out ~7 KB with one
  121 KB outlier) or by pacing (the watch observes an activity gap ≥
  StallTimeout/5 = 3 s that CLOSED — fresh bytes flowed after it;
  the shapes overlap in bytes but differ in RHYTHM: the dead cohort
  never woke). A proved flow's budget escalates to
  `StallRichTimeout` (default 90 s — well past every real pacing
  gap, well under the 300 s idle timeout) and its close is QUIET:
  both ends closed, NO answer-set demotion, NO error count
  (hygiene, not a dead-origin signal). The LEAN tier keeps the exact
  M8-v semantics (demotion + error + the fast 15 s close) for the
  never-proved shape. The stall line is now `tunnel stall killed
  idle=Nms down=N tier=rich|lean` (the prefix stable). Knobs:
  `StallRichTimeout` (0 → default; negative or ≤ StallTimeout → no
  escalation, the exact M8-v shape), `StallRichBytes` (0 → default;
  negative → only a survived gap escalates). Both-ways proof: with
  the tiering stashed, `TestStallRichFlowSurvivesLeanBudget` FAILS
  (the payload flow closed at the lean budget — the regression
  reproduced in test) and `TestStallWakeEscalatesBudget` FAILS (the
  pacing flow closed at lean right after its second flight); the
  lean guards fail pre-fix on the new line format only; the four
  M8-v pins (stallrace_test.go) untouched and passing.
- **The build ALSO carries debug.15's never-run cargo UNCHANGED —
  the next report must carry BOTH signatures:** the first-dial race
  (ADR-0021: `tunnel first-byte raced dials=N` — the primary raced
  against FirstByteRaceDials=2 extra candidates from the flow's
  start, first speaker wins, an all-dead set terminals at ~1
  budget) and the honest dns probe (ClearNegative before the health
  lookup — the watchdog asks the servers NOW).
- **The method-policy question is LIVE for Ali (D102, his ruling,
  NOT our code):** terminal first-byte failures clustered under
  method=strong in the sixth report and under balanced in the
  seventh (the sweep dragged it after DNS false-failures); should a
  flow that exhausts its raced round be retried once with the NEXT
  preset in HealSweepOrder (per-flow method variation)? No steering
  or policy change without his ruling.
- **D102 stays reserved for Ali's rulings** (none landed in M8-x);
  D94-D103 are spent. The account-gate lane stays CLOSED (D97).
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4-8, row 9's device half (udpDropped=73 this
  report; the QUIC path already fast-fails via ICMP
  port-unreachable, so it is not a startup-lag contributor),
  rotation/TalkBack, the data-usage round, D93's desktop check.
- The dormant desktop release lane and public-release engineering
  remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything disagrees with the
  repository, **the repository wins** — report the discrepancy.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-y)

For each issue Ali reports from `v1.6.6-android-debug.16` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names **versionCode 15**. (The standing
  doctrine, PROVEN by the seventh report: a versionCode 13 or 14
  report means the old APK is still installed — the round is the
  INSTALL again, not a fix. The D103 names make the wrong build
  self-identifying: the correct file is v1.6.6-debug16.apk; if the
  filename says anything else, it is the wrong build. Install over
  debug.14/15 in place — same pinned certificate, higher
  versionCode, settings and usage survive — then reconnect and
  confirm the report header reads versionCode 15.) Then read it in
  this order: (a) **the tier= split on the `tunnel stall killed`
  lines** — the round's primary material: `tier=rich` kills should
  be rare and harmless (90 s hygiene closes), `tier=lean` kills the
  dead-origin cohort only; their counts tune 32 KB / 90 s / the
  wake gap, and a rich cohort that is still large means the pacing
  gaps run longer than 90 s; (b) the **`tunnel first-byte raced
  dials=N`** lines (debug.15's never-run race): every raced line is
  a dirty primary the flow never paid a budget for; the raced count
  vs clean first-bytes vs terminals says whether the candidate
  count (2) should grow or shrink; (c) the terminal
  **`first-byte failed`** dones — they should cluster at ~2.7 s
  (one budget + dials); their ratio is the D102 material; (d) the
  dns fail counter and the method line — did the degraded state
  stop firing, did the sweep stop dragging the method; (e) the
  `tunnel up dns=… dial=…` lines and the tun: line
  (flows/udpDropped/doorFailures); and above all Ali's own verdict:
  **is going next and back in the same video instant again?** (that
  is this build's whole point). Root-cause from the evidence; fix at
  the correct layer (engine/facade/Kotlin/WebView); keep the
  D77-D103 honesty rules binding.
- **Also surface/collect Ali's ruling on the D102 method question**
  (doc 10 row 3 + doc 09 handoff carry its exact wording): whether a
  flow that exhausts its raced round under one preset should be
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
  (amending D99; N = the tag's build sequence — debug.17's asset
  is v1.6.6-debug17.apk)** —
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
  26) **INTO android/app/libs/younblock.aar — then verify the new
  engine strings actually landed in the APK's libgojni.so (the M8-w
  lesson: the first assembleDebug packed the STALE libs/ AAR and the
  artifact check caught it)**; clean `assembleDebug` (if it fails on
  "Toolchain installation … does not provide the required
  capabilities: [JAVA_COMPILER]", a stale gradle daemon holds the
  system JRE — `./gradlew --stop` then `JAVA_HOME=~/sdk/jdk
  ./gradlew clean assembleDebug`; also CLEAN, never incremental —
  the M8-w repack left 12 MB of zip slack); artifact verification
  (sha256, apksigner, badging, size sanity vs the previous build);
  the served-variant check (D101) for any UI change; commit. **Long
  builds run as FOREGROUND CHUNKS — detached jobs die (the M8-t
  lesson, reconfirmed); manage disk per the M8-t lesson (`go clean
  -cache` before the bind if free space is under ~2.5 GB).**

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog.
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-x,
   M8-y, … The next decision number is D102 — reserved for Ali's
   rulings; record his rulings there BEFORE any code.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.16 edition + §"Device test circle" — row 3 is the stall row
   with the seventh-report record and the murdered-pool evidence),
   doc 06 (the M8-c…M8-x records), doc 08 + `DECISIONS.md` (D1-D103;
   read D97, D98, D99, D100, D101, D103 in full; **ADR-0022 is the
   M8-x decision; ADR-0021 the M8-w one it carries forward
   unverified**),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-x commit
   on top — the tiered stall watch + ADR-0022, over 2a0423b, the
   M8-w first-dial race). Preserve any
   uncommitted work. If push is rejected (credential rotated), the
   session prompt carries a fresh token.
4. Check whether Ali's debug.16 device feedback has arrived. If a
   pasted diagnostics report is present: the feedback loop above,
   the report first — and the "android shell:" line FIRST of all
   (13 or 14 = the old APK still installed). If not: say so plainly
   and put the honest alternatives to Ali.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER: `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp — **cp, never rsync
   --delete: the M8-x round's rsync deleted the tracked
   placeholder.txt; the lane's own script copies without
   --delete**); `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, twice);
   `GOOS=windows go build ./...` (exit 0). **Wipes happen at ANY
   boundary — VERIFY the toolchain first (`. ~/sdk/env.sh && go
   version && javac -version`; the M8-x wipe lost ~/sdk AND ~/go but
   kept the system JRE — which LACKS javac: the JDK must be
   reinstalled to ~/sdk/jdk) and only reinstall if actually wiped
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
- JDK 21.0.12.1+1 (Temurin, WITH javac) at `~/sdk/jdk`; Android SDK
  at `~/sdk/android-sdk` (platforms 26/36, build-tools 36.1.0, NDK
  28.2.13676358, gomobile/gobind at the pinned x/mobile). Exact
  pins: doc 06 §1.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore` — after a wipe + re-clone,
  assembleDebug signs identically with NO extra step.
- No /dev/kvm (re-verified every session). Disk is a ~9.9 GB box —
  **`go clean -cache` before the gomobile bind when free space is
  under ~2.5 GB (the M8-t/M8-v/M8-x lesson).**
- **The sandbox's background killer kills detached jobs mid-run.
  Run long builds as FOREGROUND CHUNKS with the caches carrying the
  progress (frontend / gomobile bind / gradle, each within a command
  timeout); short detaches may still work but VERIFY liveness before
  trusting a log that stops growing.**
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew` **and `chmod +x build-android.sh`**.
- **Formatting (the recurring hazard, sharpened at M8-w): the edit
  tooling WRITES SPACES file-wide into .go files — most of the tree
  is TAB-indented, so after editing a tab file run
  `~/sdk/go/bin/gofmt -w` on it (check `git diff --stat` stays
  surgical). Some files are SPACE-indented at HEAD on purpose
  (tunnel.go, flowtiming_test.go, health_test.go,
  settings_test.go, usage_lifecycle_test.go, linux.go, windows.go,
  sysproxy_logic.go + its test, tunstack_test.go, usage_test.go,
  webui_test.go — `gofmt -l internal/` names them) — do NOT gofmt
  those: gofmt converts the WHOLE file to tabs (a 1000-line
  whitespace diff). If a space-style file was gofmt'd by accident,
  restore the style with `expand -t 8 < file > /tmp/e && mv
  /tmp/e file` and re-check the diff.** (stalltier_test.go, the
  M8-x test file, is tab/gofmt-clean.)

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
- The ADRs (doc 08, ADR-0001…0022) are the ground truth.
- The honesty rules D77-D103 bind; a surface ships in the variant
  the edition serves, verified through the real webui server
  (D101/ADR-0016).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-y / M8-z / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug<N>.apk per D103
  (amending D99)** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.16 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity — 13 or 14 means the
old APK is still installed and the round is the install) and name the
FIRST issue you will address from its evidence; if only prose
arrived, ask for the report instead of guessing; (5) confirmation
that you have read D97 (the auth lane stays untouched), D98 (every
release carries the APK), D99 (the APK asset is named with the
version — as amended by D103: the build number rides in the
filename), D100 (the diagnostics surface is the loop's evidence
path), D101 (a surface ships in the variant the edition serves) and
D103 (the APK asset is named v<versionName>-debug<N>.apk). If no
feedback arrived, say so plainly and put the honest alternatives to
Ali. Then execute.
