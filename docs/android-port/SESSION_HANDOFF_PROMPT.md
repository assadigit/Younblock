# Prompt — Continue Younblock: the device-test feedback loop (M8-t — the debug.12 round: the report WITH first-byte reasons)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-s close, 2026-10-18/19)

- **THE SECOND PASTED DIAGNOSTICS REPORT ARRIVED and was acted on:
  `v1.6.6-android-debug.12` (versionCode 11, Latest, the APK asset
  named v1.6.6-debug.apk) ships the fix it drove (ADR-0018).** Ali's
  debug.11 round: "the problem persists, but when after a while,
  video plays, it can be easily rewinded and go next... only the
  problem is each video starting to play which lags considerably.
  IN pc, it's instant" + the full report — its "android shell:
  1.6.6 (versionCode 10)" line proved the debug.11 install; state
  ON (not degraded), method strong — **the M8-r fixes proven
  on-device**: the heal sweep found a working method 3 s in, no
  false "Couldn't fix it", the startup DoH blip still logging its
  three baked `dns server failed server=#N reason=DNS_UNREACHABLE`
  lines and self-healing (the M8-r sharpened question answered: a
  transient at Start, not a persistent filter). The stall itself
  finally had its exact shape: at each video start the app opens
  waves of parallel tunnels; within one wave some flows complete
  (first-byte 56-619 ms, KBs down) while others never receive one
  byte — ~0.5 s closes with `down=0 B` (origin reset) and EXACTLY
  ~60.0 s hangs with `down=0 B` (silent blackhole). Read back
  against the engine line by line: NOTHING in the engine ends a
  flow at 60 s (IdleTimeout 300 s; the TUN keepalive idle is 60 s
  but it only probes and the phone kernel answers; every other
  candidate deadline is 5/6/8/10/12/30/35/120 s) — the ~60 s closer
  is the app's own patience, and the engine AMPLIFIED it by holding
  the silent flow open; worse, because the dial SUCCEEDED, the
  M8-o Invalidate loop never fired, so the suspect answer set stayed
  fresh and every retry wave could land on the same dead node.
  **What debug.12 carries (ADR-0018): the first-byte budget — a
  processed host's origin must answer within 8 s of the first client
  record or the flow fails FAST (both ends closed, the client
  retries immediately as it already retries the ~0.5 s resets, the
  error counted, "tunnel first-byte failed reason=timeout|reset|
  closed" baked into the log message, and the host's cached answer
  set demoted exactly like a total dial failure — the M8-o loop
  extended to origins that connect but never speak). Unrouted relays
  keep the plain copy shape byte-for-byte.** The host reproduction
  is the round's proof: with the fix stashed, the two new
  firstbyte_test.go tests FAIL at ~5.0 s (the device's 60 s
  signature compressed); with it, ~250 ms.
- **The method-policy question is LIVE for Ali (D102 material, his
  ruling, NOT our code):** the sweep picks the method from its
  redirector.googlevideo.com probe while the media flows hit
  rr*.googlevideo.com cache nodes whose tolerance differs per node —
  his report ran method strong while his PC is instant on the same
  network. Should the engine retry a dead routed flow once with the
  NEXT preset in HealSweepOrder (per-flow method variation)? No
  steering or policy change without his ruling.
- **D102 stays reserved for Ali's rulings** (none landed in M8-s);
  D94-D101 are spent. The account-gate lane stays CLOSED (D97).
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6, rows 4-8, row 9's device half (udpDropped is climbing
  on record — 82 this report), rotation/TalkBack, the data-usage
  round, D93's desktop check.
- The dormant desktop release lane and public-release engineering
  remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything disagrees with the
  repository, **the repository wins** — report the discrepancy.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-t)

For each issue Ali reports from `v1.6.6-android-debug.12` (device
side) or his rebuilt desktop exe:

- **FIRST: the pasted diagnostics report.** If he pasted one: verify
  the "android shell:" line names **versionCode 11** (a versionCode
  10 report means the old APK is still installed and the round
  starts with the install). Then read it in this order: (a) the new
  **`tunnel first-byte failed reason=…`** lines — the ADR-0018 baked
  evidence: `reason=timeout` = the blackhole shape (a tuning/data
  point for the budget), `reason=reset` = the DPI active reset
  (~0.5 s shape — now also demoted), `reason=closed` = the origin
  FIN'd without speaking; (b) the done lines — `dur≈8000ms
  down=0 B` is the budget firing exactly once per silent wave (many
  of them per video start = the retries are not converging on a
  good node: the D102 method question is the live lever);
  `dur≈500ms down=0 B` = the fast-reset shape; (c) the
  `tunnel up dns=…ms dial=…ms` lines and any dns server-failed
  lines as before; (d) the tun: line (flows/udpDropped/doorFailures).
  Root-cause from the evidence; fix at the correct layer
  (engine/facade/Kotlin/WebView); keep the D77-D101 honesty rules
  binding.
- **Also surface/collect Ali's ruling on the D102 method question**
  (doc 10 row 3 + doc 09 handoff carry its exact wording): whether a
  dead routed flow should be retried once with the NEXT preset. If
  he rules, record it in DECISIONS.md as D102 BEFORE any code.
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
  (D101) for any UI change; commit.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog.
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-t,
   M8-u, … The next decision number is D102 — reserved for Ali's
   rulings; record his rulings there BEFORE any code.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.12 edition + §"Device test circle" — row 3 is the stall row
   with the D102 question), doc 06 (the M8-c…M8-s records), doc 08 +
   `DECISIONS.md` (D1-D101; read D97, D98, D99, D100, D101 in full;
   **ADR-0018 is the M8-s decision**), and the tail of
   `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-s commit
   — the first-byte budget + ADR-0018 — on top of 10d04a1, the
   M8-r commit). Preserve any uncommitted work. If push is rejected
   (credential rotated), the session prompt carries a fresh token.
4. Check whether Ali's debug.12 device feedback has arrived. If a
   pasted diagnostics report is present: the feedback loop above,
   the report first. If not: say so plainly and put the honest
   alternatives to Ali.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER: `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, twice);
   `GOOS=windows go build ./...` (exit 0). **~/sdk was intact at the
   M8-s close; wipes happen at ANY boundary — VERIFY first
   (`. ~/sdk/env.sh && go version && javac -version`) and only
   reinstall if actually wiped (the cmdline-tools `12.0` alias URL
   404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`).**

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
  **the M8-s session cleared the Go build cache before the bind;
  expect `go clean -cache` again before any big Android build.**
- **The sandbox kills background processes that stay in a command's
  process group — a double-fork detach
  (`( setsid <cmd> > log 2>&1 < /dev/null & )`) survives across
  invocations (proven at M8-q, used at M8-r/M8-s for the webui
  server and the Android lane itself).**
- After a re-clone: `git config core.fileMode false` and
  `chmod +x android/gradlew` **and `chmod +x build-android.sh`**.
- **Formatting (the M8-n/M8-r/M8-s hazard, hit every session): the
  Edit tool WRITES SPACES file-wide into tab-indented .go files and
  often REFUSES to match tab/em-dash content at all — the M8-s
  session did its tunnel.go surgery through python3 heredocs and
  `~/sdk/go/bin/gofmt -w` on the TAB files it touched (go check
  `git diff --stat` stays surgical). Some files are SPACE-indented
  at HEAD on purpose (tunnel.go, flowtiming_test.go, health_test.go,
  settings_test.go, usage_lifecycle_test.go) — write their edits in
  the file's own style; do NOT gofmt those.**

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
- The ADRs (doc 08, ADR-0001…0018) are the ground truth.
- The honesty rules D77-D101 bind; a surface ships in the variant
  the edition serves, verified through the real webui server
  (D101/ADR-0016).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D102, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-t / M8-u / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **always attach the
  debug APK per D98, named v<versionName>-debug.apk per D99** — and
  record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.12 has arrived — if a diagnostics report was pasted, quote
its "android shell:" line (the build identity) and name the FIRST
issue you will address from its evidence; if only prose arrived, ask
for the report instead of guessing; (5) confirmation that you have
read D97 (the auth lane stays untouched), D98 (every release carries
the APK), D99 (the APK asset is named with the version), D100 (the
diagnostics surface is the loop's evidence path) and D101 (a surface
ships in the variant the edition serves). If no feedback arrived,
say so plainly and put the honest alternatives to Ali. Then execute.
