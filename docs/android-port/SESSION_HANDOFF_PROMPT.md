# Prompt — Continue the Younblock Android port after M8 (device-test feedback loop)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M8 close)

- **Phase 2 is host-complete. M0–M8 are all done** (2026-10-17): the
  Go engine ported and hardened behind ADRs, the gomobile facade with
  the full binding set, the Kotlin VPN service (consent gate, honest
  routing report, the NO_TARGET_APPS refusal door, run generations),
  the mobile settings/UX (the empty-default apps allow-list — D83/D84,
  confirmed by Ali — and the .ir never-processed rule), the audits,
  and the packaging.
- **The first distributable build is shipped**: the GitHub prerelease
  **`v1.6.6-android-debug.1`** (D84 — Ali's keystore ruling: the
  debug-APK-only test circle; "we're not releasing yet", so no signed
  variant exists). The artifact is the M8-a clean-build APK,
  35,567,561 B, SHA-256 ccf46a2e…7b64, re-verified on disk and against
  the uploaded release asset (apksigner v2 OK, debug cert digest
  c76bb7c6…8551d; badging io.github.assadigit.younblock 1.6.6
  versionCode 1, minSdk 26 / target 36, 3 ABIs; the four-permission
  manifest + launcher-intent `<queries>`).
- **The project's one remaining input is Ali's device observations**:
  the release notes carry the device test circle (doc 10 §"Device
  test circle") — install/first-launch, the live consent dialog,
  notification behavior, screen-off/Doze, real DoH, the YouTube app's
  QUIC→TCP fallback, protect() on real sockets, the .ir direct path on
  a real radio, stop/reboot paths, battery, rotation/font-scale/
  TalkBack. Everything device-side is ⛔ in the sandbox (no /dev/kvm,
  no device — never claimed, never faked) and is waiting on that
  feedback.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (sandbox resets have destroyed it before; if it is missing,
   recreate it from this prompt plus the repo records and say so).
   Append your own entry (Task ID + the standard template) when you
   finish; never overwrite.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 05 §M8 (the
   close-out record), doc 06 (the M8 record incl. the release lane),
   doc 10 (§"Device test circle" — Ali's checklist), doc 08 +
   `DECISIONS.md` (D1–D84; **the next decision number is D85`),
   and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Check whether Ali's device-test feedback has arrived (session
   messages). If none has arrived yet, STOP after reporting state —
   do not invent work, do not "improve" shipped code without a
   reported reason.
5. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **17/17 packages
   ok**, run twice consecutively); webui `svelte-check` (0/0) and
   `vite build`. D74 trap: a stale webui dist fails
   `TestWebUILifecycle` (in the mobile package) — sync
   `frontend/dist → internal/webui/dist` before judging.

## The task — the device-test feedback loop

For each issue Ali reports from `v1.6.6-android-debug.1`:

- Root-cause first (the project's standing discipline: failures are
  root-caused and fixed, never retried away, never hidden). The
  diagnostics report (in-app, redacted) + device model/Android
  version are the expected inputs.
- Fix at the correct layer (engine/facade/Kotlin/WebView), keeping
  the D77–D84 honesty rules binding: the UI may only claim what the
  engine actually enforces; ⛔ gates are listed, never dropped.
- Record each closed checklist row with Ali's measured evidence (doc
  07 §9 / doc 10 §"Device test circle"); rows without observations
  stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule: updates install over the previous debug build; same debug
  certificate), then run the full lane: desktop baseline; compile
  matrix; `gomobile bind`; **clean** `assembleDebug` (only clean
  builds are honest); artifact verification (sha256, apksigner,
  badging, manifest dump — doc 06's M8 record); commit; then cut the
  next debug prerelease (`v1.6.6-android-debug.2`, …) through the
  same release lane (tag the M-commit, upload the APK, verify via the
  API, record asset size + digest).
- If Ali decides to release publicly: the signed-release decision
  returns (doc 07 §8 — his local keystore or an EC key, never
  committed; README backup steps), and doc 10 §2 (the public v1
  DRAFT) is finalized against that artifact.

## Environment (this sandbox resets without warning — verify, don't assume)

Layout as verified at the M8 close (three consecutive no-wipe
sessions before it — but verify anyway):

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
- No /dev/kvm (re-verified at M8). Disk is a ~9.9 GB box — clean
  Go/Gradle build caches before big builds; ~1.9 GB free at the M8
  exit.
- The sandbox kills background processes when a Bash command ends —
  run long builds as foreground chunks.
- The honest compile-matrix form: `go build` over the 18 non-main
  packages per ABI at CGO_ENABLED=0 (arm64 links the full tree).
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
  force-push; no unrelated changes; **no secrets or signing keys
  committed**; preserve Windows/Linux desktop behavior (the full
  regression gate is part of every change set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D84 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped.
- When you finish: update doc 05/06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D85),
  `CHANGELOG.md`; append the worklog entry (Task ID M8-b, M8-c, … per
  feedback round); commit and push; cut the debug release with the
  bumped versionCode and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's device-test
feedback has arrived, and if so the exact first issue you will
address. Then execute — or, if no feedback has arrived, stop after
the state report and say so plainly.
