# Prompt — Continue Younblock: the device-test feedback loop (M8-o — confirm the first-video fix on debug.7)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL; the
session prompt carries a fresh token if push is rejected).

## Where the project stands (verified at the M8-n close, 2026-10-18 evening)

- **The slow first video — the project's longest-open defect — is
  ROOT-CAUSED and FIXED at host level (ADR-0013), shipped as
  `v1.6.6-android-debug.7` (versionCode 6, Latest).** Ali's debug.6
  device round: first video 2–3 minutes while "the windows app doesnt
  have this problem". The Windows-vs-Android discriminator localized
  it to the Android-only layers; the cause was IN THE CODE: the TUN
  stack aborted every flow whose door dial exceeded 10 s (a default
  the mobile facade never overrode) — SHORTER than the SOCKS5 door's
  own honest setup budget (DoH resolve ≤ 3×3 s + upstream dial ≤ 12 s,
  which the desktop HTTP-CONNECT door runs with NO abort, hence
  Windows instant). googlevideo answers on filtered networks carry
  silently-blackholed addresses; the dialer walked them sequentially
  at 5 s each; the DNS answer is cached ≥120 s, so every app retry
  hit the same wall until the cache rotated — the minutes. The fix:
  `netx.DialHost` now races addresses (attempt k at k×750 ms, first
  success wins, losers canceled — BOTH doors benefit) and the TUN
  handshake budget is budget-true (`config.TunHandshakeTimeout` 30 s,
  facade passes it, tunstack default raised to match; the lockstep is
  pinned by TestHandshakeTimeoutDefaultCoversDoorBudget).
- **D98 (Ali's ruling, this round):** "from now on, add the APK
  version (debug version) in the releases" — EVERY release page
  carries the debug APK asset from debug.7 on, including
  docs/helper-only rounds (debug.4's missing APK was the trigger).
- **The account-gate lane stays CLOSED (D97)** — cancelled before any
  code; `docs/auth/` is bannered history; ADR-AUTH-0001 retired; the
  A-series Task IDs (A1/A2/A3) are retired; Younblock stays ONE
  free-for-all app — no login, no server, no editions. Read D97 in
  full in DECISIONS.md before doing anything; if Ali ever re-opens
  auth, a new decision supersedes D97 and the pack must be
  revalidated against the then-current tree first.
- **The rest of the device test circle is unchanged and largely
  unrun** (doc 10 §"Device test circle"): the cross-device UI verdict
  on debug.6 (does the phone match his desktops? Data usage sane?),
  rows 4–8 (browser routing, .ir direct, stop paths, persistence/
  battery, diagnostics copy), rotation/TalkBack, the data-usage
  round, and D93's desktop check (Ali's rebuilt exe walks README's
  manual-check steps 4–5).
- **The dormant desktop release lane** (the rounds under CHANGELOG
  "Unreleased") and **public-release engineering** (release signing,
  CI, update mechanism) remain open-but-paused by choice.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-o)

For each issue Ali reports from `v1.6.6-android-debug.7` (device side)
or his rebuilt desktop exe:

- **FIRST: the first-video confirmation.** Connect → open the YouTube
  app → tap the first video right after the feed loads. It should
  start within seconds, like Windows. If it does, doc 10's circle
  row 3 finally closes with his measured evidence (the standing
  rule: no row closes without it). If it does NOT, the required
  input is the diagnostics report copied at the slow moment (Menu →
  Diagnostics → Copy) + phone model + Android version + which video
  (first only or every) — the debug.5 per-flow timing lines
  (`tunnel up dns=…ms dial=…ms`, `tunnel first-byte …ms`) name the
  waiting phase; root-cause from there, fix at the correct layer
  (engine/facade/Kotlin/WebView), keep the D77–D98 honesty rules
  binding (the UI may only claim what the engine enforces; ⛔ gates
  are listed, never dropped).
- Record each closed checklist row with Ali's measured evidence
  (doc 07 §9 / doc 10 §"Device test circle"); rows without
  observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule) and cut the next debug release — a REGULAR release in the
  private repo, never flagged prerelease (D89), and **carrying the
  debug APK asset per D98** — through the release lane (push the code
  commit BEFORE creating the release — the full-SHA target_commitish
  422s against an unpushed SHA; upload the APK, verify via the API,
  record asset size + digest) WHEN a code change warrants it — do not
  re-release identical bytes. Full lane before release: desktop
  baseline in the D74 order (frontend npm ci → svelte-check 0/0 →
  vite build → CLEAN-sync dist → go vet → go test 18/18 ×2 →
  GOOS=windows exit 0); the compile matrix in its exact form
  (CGO_ENABLED=0 `go build ./internal/... ./mobile/...` = 19
  non-main packages per ABI ×4, plus arm64 linking the full tree
  `./...` with CGO_ENABLED=0 — do NOT run the arm64 full-tree link
  with the host gcc; the M8-n session burned 90 s on exactly that);
  `gomobile bind` (GOFLAGS=-mod=mod, 3 ABIs, androidapi 26);
  **clean** `assembleDebug`; artifact verification (sha256, apksigner,
  badging — doc 06's records); commit.
- If feedback has NOT arrived when the session opens, do not block
  and do not guess: say so plainly and put the honest alternatives
  to Ali — the desktop release lane or public-release engineering —
  his pick, or a new feature direction if he has one (recorded as a
  decision before any code, per the D94 pattern).

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-o,
   M8-p, … **The auth lane's Task IDs (A1, A2, A3) are RETIRED —
   cancelled with the feature at D97; never open them. The next
   decision number is D99 — reserved for Ali's rulings; D94–D98 are
   spent (D97 is the cancellation, D98 is the APK-on-every-release
   ruling).**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.7 edition + §"Device test circle" — row 3 is the fix's
   confirmation row), doc 06 (the M8-c…M8-n records), doc 08 +
   `DECISIONS.md` (D1–D98; **read D97 and D98 in full**), and the
   tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the M8-n commit —
   the first-video fix + D98 — on top of 9f31b51, the A0-R3 auth
   cancellation). Preserve any uncommitted work; never reset it just
   to get a clean diff. If push is rejected (credential rotated), the
   session prompt carries a fresh token — update the `origin` remote
   URL with it.
4. Check whether Ali's debug.7 device feedback has arrived (session
   messages). If yes: the feedback loop above, the first-video
   confirmation first. If no: say so plainly and offer the honest
   alternatives — do not guess, do not invent work.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER (a fresh clone fails the Go lanes until the frontend is
   built): `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); then `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, run twice
   consecutively); `GOOS=windows go build ./...` (exit 0).
   **~/sdk was intact at the M8-n close; wipes happen at ANY
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
  the ADRs (doc 08, now ADR-0001…0013) are the ground truth.
- The honesty rules D77–D98 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped; no
  recommendation may be recorded as a ruling; a cancellation is
  recorded as exactly what it is (D97 is the pattern).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D99, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-o / M8-p / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed — **and always attach the
  debug APK per D98** — and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.7 has arrived, and if so the exact first issue you will
address (the first-video confirmation leads if present); (5)
confirmation that you have read D97 (the auth lane stays untouched)
and D98 (every release carries the APK). If no feedback arrived, say
so plainly and put the honest alternatives to Ali. Then execute.
