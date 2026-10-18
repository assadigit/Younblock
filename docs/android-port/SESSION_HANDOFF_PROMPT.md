# Prompt — Continue Younblock: the device-test feedback loop (after the auth lane was cancelled)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the A0-R3 close, 2026-10-18)

- **Phase 2 is host-complete. M0–M8 are all done.** The account-gate
  lane ran its full design arc in one day and is now CLOSED: A0
  delivered the six-document spec pack under `docs/auth/`, A0-R2
  recorded Ali's rulings D95 (admin-only accounts) and D96 (the
  two-edition direction), and **A0-R3 (2026-10-18, the third exchange
  of the day) CANCELLED the whole feature as D97** — his words: "You
  know I've decided to defer and cancel this auth version, only work
  on non-auth version. it doesnt worth this headache. cancel it
  altogether, no auth needed anymore."
- **What D97 means (read it in full in DECISIONS.md before doing
  anything):** Younblock stays ONE free-for-all app — no login, no
  accounts, no auth server, no admin surface, no editions, no build
  flavors. D95/D96 are superseded (history); every auth open question
  — (a), (c)–(f), (g1)–(g4) — closed as MOOT, never ruled on its
  merits; ADR-AUTH-0001 (doc 08) is RETIRED; the A1/A2/A3
  implementation ladder is cancelled and those Task IDs are RETIRED —
  do not open them. Nothing was ever built (design-first per D94 held
  to the end), so nothing was removed: no login code, no server, no
  database, no keys exist anywhere. Today's privacy claims — "no
  account, no data collection" — stay literally true, permanently.
  `docs/auth/` stays in the repo as the bannered (CANCELLED) design
  record; its 02 §6 Iran-reachability finding stays on record as a
  network constraint for any FUTURE server-side feature, not as an
  auth plan. If Ali ever re-opens auth, a new decision supersedes
  D97 and the pack must be revalidated against the then-current tree
  before anything is trusted.
- **What D97 does NOT decide:** the cancellation governs the gate
  only. A public website, the APK distribution channel, the dormant
  desktop release lane, and public-release engineering (release
  signing, CI, update mechanism) are all still open questions —
  none of them is cancelled, none of them is decided.
- **The open slow-video defect and the device test circle are
  unchanged** (the feedback loop is the ONE remaining workstream):
  1. **The slow-video defect** (video takes 30 s–1 min+ to start;
     feed loads fast). Every host-level suspect is eliminated WITH
     EVIDENCE (UDP/443 gets an instant ICMP port-unreachable — pinned
     by test; the per-flow timing instrumentation ships in debug.5/6).
     The missing input is **Ali's diagnostics report copied at a
     slow-video moment** (Menu → Diagnostics → Copy) + phone model +
     Android version + whether every video or only the first after
     connecting.
  2. **The largely unrun device test circle** (doc 10 §"Device test
     circle"): rows 4–8, rotation/TalkBack/font scaling, the
     cross-device UI verdict on debug.6, the data-usage round, and
     D93's desktop check (Ali's rebuilt exe walked through README
     manual-check steps 4–5).
  3. **The dormant desktop release lane**: v0.7.6 predates D86–D93;
     nine rounds sit under CHANGELOG "Unreleased".
  4. **Public-release engineering not started** (by choice, so far):
     debug-only signing (D84), no CI (no .github/), no frontend unit
     tests, unsigned Windows exe, no update mechanism.
- **The current test-circle build is `v1.6.6-android-debug.6`**
  (versionCode 5, built at 078e3a5, the pinned debug certificate —
  installs over debug.5 in place, settings survive): the cross-platform
  UI unification (D92) + the engine-traced data usage. Play Protect's
  popup at install stays expected; the release keystore stays
  NOT-for-the-test-circle (D88/D90; the ceremony is complete on Ali's
  machine).
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## The session's ONE workstream — the device-test feedback loop (Task ID M8-n)

For each issue Ali reports from `v1.6.6-android-debug.6` (device side)
or his rebuilt desktop exe (the D93 close ruling):

- Root-cause first (the standing discipline: failures are root-caused
  and fixed, never retried away, never hidden). The diagnostics report
  (in-app, redacted) + device model/Android version are the expected
  inputs; for "YouTube still doesn't load" the discriminating evidence
  is the report's DoH/self-test sections plus whether the key icon was
  up (TUN established) and which surface (app vs browser, which
  browser).
- Fix at the correct layer (engine/facade/Kotlin/WebView), keeping
  the D77–D94 honesty rules binding: the UI may only claim what the
  engine actually enforces; ⛔ gates are listed, never dropped.
- Record each closed checklist row with Ali's measured evidence
  (doc 07 §9 / doc 10 §"Device test circle"); rows without
  observations stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule: updates install over the previous debug build) and cut the
  next debug release — a REGULAR release in the private repo, never
  flagged prerelease (D89) — through the release lane (push the
  code commit BEFORE creating the release — the full-SHA
  target_commitish 422s against an unpushed SHA; upload the APK,
  verify via the API, record asset size + digest) WHEN a code
  change warrants it — do not re-release identical bytes. Full lane
  before release: desktop baseline; the compile matrix in its exact
  form (`go build ./internal/... ./mobile/...` = the 18 non-main
  packages per ABI ×4, plus arm64 linking the full tree `./...` — do
  not batch-build with an empty package list: that builds the MAIN
  package, which legitimately requires cgo on arm/386/amd64);
  `gomobile bind`; **clean** `assembleDebug`; artifact verification
  (sha256, apksigner, badging, manifest dump — doc 06's records);
  commit.
- If feedback has NOT arrived when the session opens, do not block
  and do not guess: say so plainly and put the honest alternatives
  to Ali — the desktop release lane (gap 3) or public-release
  engineering (gap 4) — his pick, or a new feature direction if he
  has one (recorded as a decision before any code, per the D94
  pattern).

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-n,
   M8-o, … **The auth lane's Task IDs (A1, A2, A3) are RETIRED —
   cancelled with the feature at D97; never open them. The next
   decision number is D98 — reserved for Ali's rulings; D94–D97 are
   spent (D97 is the cancellation).**
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.6 edition + §"Device test circle"), doc 06 (the M8-c…M8-m +
   A0/A0-R3 records), doc 08 + `DECISIONS.md` (D1–D97; **read D97 in
   full** — it is the cancellation ruling and states its own bounds),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3` (expect the A0-R3
   docs-only commit — the auth cancellation — on top of the A0-R2
   commit on top of the A0 commit). Preserve
   any uncommitted work; never reset it just to get a clean diff. If
   push is rejected (credential rotated), the session prompt carries a
   fresh token — update the `origin` remote URL with it.
4. Check whether Ali's device-test feedback from debug.6 has arrived
   (session messages). If yes: the feedback loop above, first issue
   first. If no: say so plainly and offer the honest alternatives
   (desktop release lane / public-release engineering / a new
   direction he names) — do not guess, do not invent work.
5. Make the desktop baseline GREEN before writing any code, in the
   D74 ORDER (a fresh clone fails the Go lanes until the frontend is
   built): `cd frontend && npm ci`; `npx svelte-check` (0/0);
   `npx vite build`; CLEAN-sync `frontend/dist → internal/webui/dist`
   (rm + restore placeholder.txt + cp); then `go vet ./...`;
   `go test -count=1 ./...` (expect **18/18 packages ok**, run twice
   consecutively); `GOOS=windows go build ./...` (exit 0 — it has
   compiled since the A0 fix; a failure there is a regression).
   **~/sdk was wiped before A0 and RESTORED at A0; wipes happen at ANY
   boundary — VERIFY first (`. ~/sdk/env.sh && go version &&
   javac -version`) and only reinstall if actually wiped (the
   cmdline-tools `12.0` alias URL 404s — use the versioned
   `commandlinetools-linux-11076708_latest.zip`).**

## Environment (this sandbox resets without warning — verify, don't assume)

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**
  (the env.sh written at A0 also sets JAVA_HOME, ANDROID_HOME and
  puts ~/go/bin, platform-tools and cmdline-tools on PATH).
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
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D97 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped; no
  recommendation may be recorded as a ruling; a cancellation is
  recorded as exactly what it is (D97 is the pattern).
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D98, as
  rulings land), `CHANGELOG.md`; append the worklog entry (Task ID
  M8-n / M8-o / …); commit and push; cut the debug release with the
  bumped versionCode when APP code changed, and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's DEVICE feedback
from debug.6 has arrived, and if so the exact first issue you will
address; (5) confirmation that you have read D97 and will not touch
the retired auth lane (docs/auth/ stays bannered history unless a
future ruling re-opens it). If no feedback arrived, say so plainly
and put the honest alternatives to Ali. Then execute.
