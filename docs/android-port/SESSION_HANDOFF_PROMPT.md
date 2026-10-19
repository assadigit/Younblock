# Prompt — Continue Younblock: the device-test feedback loop (M8-x — the debug.15 round: the report WITH raced lines, shell line versionCode 14)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-w close, 2026-10-19)

- **THE SIXTH PASTED REPORT ARRIVED and it WAS the salvage-race +
  stall-watch build (the M8-w round, a build round): its shell line
  read versionCode 13 and its log carried that build's own lines.**
  It PROVED ADR-0020 on-device (the `tunnel first-byte retry` lines
  batching at the same millisecond — the re-dials race as designed;
  `tunnel stall killed idle=15059-15315 ms` — the watch kills at
  exactly the 15 s budget; saves at 738-848 ms; the 60-second
  mid-stream hangs are GONE) and named the two remaining costs with
  new precision: (1) the SERIAL FRONT — six terminal
  `reason=timeout` dones at 5320-5576 ms (attempt 0 still walked its
  2.5 s budget ALONE before the race launched: the two-budget chain)
  and reset terminals at 1131-1292 ms, while CONCURRENT flows to the
  SAME hosts answered at 72-162 ms in the same seconds (five
  sub-250 ms first-bytes right between two timeout terminals) — the
  dirt is per-origin, not per-moment; (2) the DNS degraded state —
  dns fail 16 with `last failed check: dns (DNS_UNREACHABLE)` while
  the network flowed (6.7 MB down): a dirty-window DoH blip
  negative-cached the probe name (the very name the app's flows
  resolve) and the periodic check read the 15 s negative memory
  instead of asking any server — the M8-r lesson recurring on the
  watchdog path. Counters otherwise healthy: rstUnmapped=0,
  doorFailures=0, udpDropped=104.
- **`v1.6.6-android-debug.15` (versionCode 14, Latest, the APK asset
  named v1.6.6-debug15.apk — D103) ships the fix the report drove
  (ADR-0021, doc 08):** (1) **the first-dial race** — new knob
  `FirstByteRaceDials` (default 2; 0 → default; negative → disabled,
  the exact M8-v shape: serial attempt 0 + the raced salvage
  re-dials): a processed host's budgeted flow races its primary
  origin (already dialed; its dial time is the head start) against 2
  extra candidates DIALED AT THE SAME MOMENT (each on its own
  rotated start of the fresh answer set, each candidate's DialHost
  keeping the M8-n stagger failover, each replaying the client's
  first record byte-exact through a fresh fragment conn — the same
  preset captured once per flow, D102), and the FIRST origin to
  speak wins: a raced save logs the NEW report line
  `tunnel first-byte raced dials=N` (a clean primary stays quiet —
  the race costs a healthy flow nothing observable), an all-dead set
  terminals at ~1 budget instead of the M8-v 2x, the losers are
  closed and reaped, a candidate's death is classified before it
  demotes (teardown-caused deaths never demote; a budget-timeout or
  reset demotes whenever it happens — the M8-o loop), and the client
  stays FROZEN for the round (the M8-v salvage race's own proven
  semantics: bytes beyond the first record wait in the socket and
  flow to the winner in order); with racing enabled the salvage
  rounds never run (the budget is spent at the start). (2) **The
  honest dns probe** — `Resolver.ClearNegative(host)` on the doh
  resolver drops exactly the probe name's negative-cache entry
  (success entries stay), and the health package's dns step calls it
  before looking up (optional capability): the watchdog and the
  quick check now ask what the servers say NOW; the heal sweep keeps
  its full Reset (M8-r). The both-ways proof: with the raced round
  neutered, `TestFirstDialRaceSavesDirtyPrimary` and
  `TestFirstDialRaceTerminalIsBounded` FAIL; with the ClearNegative
  call neutered, `TestSelfTestDNSProbesPastNegativeCache` FAILS; with
  the fixes all PASS; the guards pass both ways (byte-exactness, the
  quiet primary, the unrouted host, the disabled knob); the
  M8-v/M8-t/M8-s pins keep their shapes with racing disabled in
  their configs (stallrace_test.go + firstbyte_test.go; the raced
  round is firstrace_test.go's).
- **The method-policy question is LIVE for Ali (D102, his ruling,
  NOT our code):** terminal first-byte failures STILL cluster under
  method=strong (six timeouts + a reset batch inside four seconds at
  01:09:47-51); should a flow that exhausts its raced round be
  retried once with the NEXT preset in HealSweepOrder (per-flow
  method variation)? No steering or policy change without his
  ruling.
- **D102 stays reserved for Ali's rulings** (none landed in M8-w);
  D94-D103 are spent. The account-gate lane stays CLOSED (D97).
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4-8, row 9's device half (udpDropped=104 this
  report; the QUIC path already fast-fails via ICMP
  port-unreachable, so it is not a startup-lag contributor),
  rotation/TalkBack, the data-usage round, D93's desktop check.
- The dormant desktop release lane and public-release engineering
  remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything disagrees with the
  repository, **the repository wins** — report the discrepancy.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-x)

For each issue Ali reports from `v1.6.6-android-debug.15` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names **versionCode 14**. (The standing
  doctrine: a versionCode 13 report means the old APK is still
  installed — the round is the INSTALL again, not a fix. The D103
  names make the wrong build self-identifying: the correct file is
  v1.6.6-debug15.apk; if the filename says anything else, it is the
  wrong build. Install over debug.14 in place — same pinned
  certificate, higher versionCode, settings and usage survive — then
  reconnect and confirm the report header reads versionCode 14.)
  Then read it in this order: (a) the NEW **`tunnel first-byte raced
  dials=N`** lines — the round's primary material: every raced line
  is a dirty primary the flow never paid a budget for; the
  raced-line count vs clean first-bytes vs terminals says whether
  the candidate count (2) should grow (an all-dead set still
  terminaling) or shrink (every raced line a save the primary would
  have made anyway); (b) the terminal **`first-byte failed`** dones
  — they should now cluster at ~2.7 s (one budget + dials), not ~5.3
  s (the M8-v two-budget chain) — their ratio is the D102 material;
  (c) the `tunnel stall killed idle=Nms` lines (still the mid-stream
  story; kills at 15 s are the budget firing as designed); (d) the
  dns fail counter — does the degraded state still fire? (it should
  now clear itself in one probe cycle); (e) the `tunnel up dns=…
  dial=…` lines and the tun: line (flows/udpDropped/doorFailures).
  Root-cause from the evidence; fix at the correct layer
  (engine/facade/Kotlin/WebView); keep the D77-D103 honesty rules
  binding.
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
  (amending D99; N = the tag's build sequence — debug.16's asset
  is v1.6.6-debug16.apk)** —
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
   finish; never overwrite. The feedback-round Task IDs run M8-w,
   M8-x, … The next decision number is D102 — reserved for Ali's
   rulings; record his rulings there BEFORE any code.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.15 edition + §"Device test circle" — row 3 is the stall row
   with the sixth-report record and the three named evidence lines),
   doc 06 (the M8-c…M8-w records), doc 08 + `DECISIONS.md` (D1-D103;
   read D97, D98, D99, D100, D101, D103 in full; **ADR-0021 is the
   M8-w decision**),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-w commit
   on top — the first-dial race + the honest dns probe + ADR-0021,
   over 1da5368, the M8-v README follow-up). Preserve any
   uncommitted work. If push is rejected (credential rotated), the
   session prompt carries a fresh token.
4. Check whether Ali's debug.15 device feedback has arrived. If a
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
  /tmp/e file` and re-check the diff.** (firstrace_test.go, the
  M8-w test file, is tab/gofmt-clean.)

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
- The ADRs (doc 08, ADR-0001…0021) are the ground truth.
- The honesty rules D77-D103 bind; a surface ships in the variant
  the edition serves, verified through the real webui server
  (D101/ADR-0016).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-x / M8-y / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug<N>.apk per D103
  (amending D99)** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.15 has arrived — if a diagnostics report was pasted, quote
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
