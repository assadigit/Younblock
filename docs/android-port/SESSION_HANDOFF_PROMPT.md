# Prompt — Continue the Younblock Android port (device-test feedback loop, after debug.3)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M8-d / D87 session close)

- **Phase 2 is host-complete. M0–M8 are all done** (2026-10-17): the
  Go engine ported and hardened behind ADRs, the gomobile facade with
  the full binding set, the Kotlin VPN service (consent gate, honest
  routing report, the NO_TARGET_APPS refusal door, run generations),
  the mobile settings/UX, the audits, and the packaging.
- **The current test-circle build is `v1.6.6-android-debug.3`** (D87,
  versionCode 3, the pinned debug certificate — installs over
  debug.1/debug.2 in place, settings survive): the default allow-list
  is FIVE apps — the YouTube app plus the Chrome, Firefox, Opera and
  Brave browsers (Ali's ruling "Add chrome to the default addlist as
  well, also firefox, opera, brave browser", closing D86's open
  question). A one-time v4 settings migration carries every era
  forward: v1/v2 born-empty files fill the five-app default; a v3
  file still holding exactly the debug.2 default [youtube] upgrades
  to it; a v3 empty (agency) and every custom list survive verbatim.
  Entries whose apps are not installed are skipped at establish
  (audited: a missing browser can never break the connect — D87
  records the audit). The chip row covers all five defaults and the
  aliases extend to firefox/opera/brave.
- **`v1.6.6-android-debug.2` and `v1.6.6-android-debug.1` are
  superseded** (kept as history; debug.1's release page carries the
  D85 install-blocked troubleshooting guide, which still applies to
  downloads from Iranian networks).
- **The project's remaining input is Ali's device observations** from
  debug.3: doc 10 §"Device test circle" is the checklist — the YouTube
  app and the installed default browsers out of the box (row 2),
  youtube.com in each installed browser (row 4), .ir direct, stop
  paths, battery, network switch, QUIC fallback. Everything
  device-side is ⛔ in the sandbox (no /dev/kvm, no device — never
  claimed, never faked) and waits on that feedback.
  **The diagnostics report (Menu → Diagnostics → copy) is the single
  most valuable evidence when anything fails** — it carries the
  engine's own redacted view (DoH provider state, self-test results,
  recent log ring).
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite. The feedback-round Task IDs run M8-b,
   M8-c, M8-d, … (this prompt describes the state after M8-d; the
   next is M8-e).
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (the
   debug.3 edition + §"Device test circle" — Ali's checklist), doc 06
   (the M8-c/M8-d records), doc 08 + `DECISIONS.md` (D1–D87; **the
   next decision number is D88**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Check whether Ali's device-test feedback from debug.3 has arrived
   (session messages). If none has arrived yet, STOP after reporting
   state — do not invent work, do not "improve" shipped code without a
   reported reason.
5. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **17/17 packages
   ok**, run twice consecutively); webui `svelte-check` (0/0) and
   `vite build`. D74 trap: a stale webui dist fails
   `TestWebUILifecycle` (in the mobile package) — sync
   `frontend/dist → internal/webui/dist` before judging.

## The task — the device-test feedback loop

For each issue Ali reports from `v1.6.6-android-debug.3`:

- Root-cause first (the project's standing discipline: failures are
  root-caused and fixed, never retried away, never hidden). The
  diagnostics report (in-app, redacted) + device model/Android version
  are the expected inputs; for "YouTube still doesn't load" the
  discriminating evidence is the report's DoH/self-test sections plus
  whether the key icon was up (TUN established) and which surface
  (app vs browser, which browser).
- Fix at the correct layer (engine/facade/Kotlin/WebView), keeping
  the D77–D87 honesty rules binding: the UI may only claim what the
  engine actually enforces; ⛔ gates are listed, never dropped. The
  D86 audit (DECISIONS.md) records the chain's verified shape —
  UDP:53 to ANY destination is answered by the virtual DNS; fake-IP
  destinations reverse to domains at the SOCKS5 door; routedHost is
  the mode/list rule minus .ir; QUIC fallback is the designed device
  gate (row 9).
- Record each closed checklist row with Ali's measured evidence (doc
  07 §9 / doc 10 §"Device test circle"); rows without observations
  stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule: updates install over the previous debug build) and cut the
  next debug prerelease through the release lane (tag the commit,
  upload the APK, verify via the API, record asset size + digest)
  WHEN a code change warrants it — do not re-release identical bytes.
  The pinned keystore (D85) guarantees the same certificate. Full lane
  before release: desktop baseline; the compile matrix in its exact
  form (`go build ./internal/... ./mobile/...` = the 18 non-main
  packages per ABI ×4, plus arm64 linking the full tree `./...` — do
  not batch-build with an empty package list: that builds the MAIN
  package, which legitimately requires cgo on arm/386/amd64);
  `gomobile bind`; **clean** `assembleDebug`; artifact verification
  (sha256, apksigner, badging, manifest dump — doc 06's records);
  commit.
- If Ali decides to release publicly: the signed-release decision
  returns (doc 07 §8 — his local keystore or an EC key, never
  committed; README backup steps), and doc 10 §2 (the public v1
  DRAFT) is finalized against that artifact.

## Environment (this sandbox resets without warning — verify, don't assume)

Layout as verified at the M8-d close (sixth consecutive no-wipe
session before it — but verify anyway):

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**.
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk-21.0.12.1+1`
  (the system JDK is a JRE without javac — use the Temurin one; its
  `bin/javap` is the AAR inspector).
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools,
  platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358, licenses
  accepted). Exact PATH / JAVA_HOME / ANDROID_HOME setup: doc 06.
- **The debug signing key is IN THE REPO** (D85):
  `android/app/debug.keystore`, wired as the debug signingConfig —
  the same certificate that signed debug.1, debug.2 AND debug.3.
  After a sandbox wipe + re-clone, assembleDebug signs identically
  with NO extra step.
- No /dev/kvm (re-verified at M8/M8-b/M8-c/M8-d). Disk is a ~9.9 GB
  box — clean Go/Gradle build caches before big builds.
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
  keystore at `android/app/debug.keystore` is vendored — the
  well-known debug identity, not a credential; release keystores stay
  out per D84/doc 07 §8); preserve Windows/Linux desktop behavior
  (the full regression gate is part of every change set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D87 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped.
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D88),
  `CHANGELOG.md`; append the worklog entry (Task ID M8-e, M8-f, … per
  feedback round); commit and push; cut the debug release with the
  bumped versionCode when code changed, and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's debug.3 feedback
has arrived, and if so the exact first issue you will address. Then
execute — or, if no feedback has arrived, stop after the state report
and say so plainly.
