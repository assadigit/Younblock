# Prompt — Continue the Younblock Android port (device-test feedback loop, after the first observation)

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the D85 session close)

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
  35,567,561 B, SHA-256 ccf46a2e…7b64 (apksigner v2 OK, debug cert
  digest c76bb7c6…8551d; badging io.github.assadigit.younblock 1.6.6
  versionCode 1, minSdk 26 / target 36, 3 ABIs; the four-permission
  manifest + launcher-intent `<queries>`).
- **Ali's first device observation has been ingested and handled
  (D85)**: "Android didn't let me install it." The artifact was
  exonerated end-to-end first (the served bytes re-downloaded and
  verified; no `testOnly`; zip intact), and the asset's
  download_count was **0** — the probable cause is an interrupted
  download from the Iranian network, with the install-unknown-apps
  gate, Play Protect, or a sub-8.0 device as the remaining branches.
  The ordered troubleshooting path now lives ON THE LIVE RELEASE BODY
  (doc 10 §1 mirrors it), and **the debug certificate is pinned in
  the repo** at `android/app/debug.keystore` (D85) — vendored and
  wired as the debug signingConfig, verified byte-identical on a
  clean rebuild — so successive debug builds install over debug.1 as
  updates even across sandbox wipes. No debug.2 was cut (nothing
  about the artifact changed).
- **The project's remaining input is Ali's device observations**: his
  retry result for the install (the file size he gets + the exact
  on-screen wording if it still fails), then the rest of the test
  circle (doc 10 §"Device test circle") — first-launch, the live
  consent dialog, notification behavior, screen-off/Doze, real DoH,
  the YouTube app's QUIC→TCP fallback, protect() on real sockets, the
  .ir direct path on a real radio, stop/reboot paths, battery,
  rotation/font-scale/TalkBack. Everything device-side is ⛔ in the
  sandbox (no /dev/kvm, no device — never claimed, never faked) and
  is waiting on that feedback.
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
   M8-c, … (this prompt describes the state after M8-b).
2. Read the spec pack, at minimum: doc 09 (handoff), doc 10 (§"If the
   install is blocked" + §"Device test circle" — Ali's checklist),
   doc 06 (the M8 record incl. the release lane + the D85 record),
   doc 08 + `DECISIONS.md` (D1–D85; **the next decision number is
   D86**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Check whether Ali's retry result / further device-test feedback has
   arrived (session messages). If none has arrived yet, STOP after
   reporting state — do not invent work, do not "improve" shipped
   code without a reported reason.
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
  version are the expected inputs. For install-stage reports, the
  discriminating facts are the downloaded file's size (must be
  ≈ 35,567,561 B) and the exact on-screen wording — the branches are
  enumerated in doc 10 §"If the install is blocked".
- Fix at the correct layer (engine/facade/Kotlin/WebView), keeping
  the D77–D85 honesty rules binding: the UI may only claim what the
  engine actually enforces; ⛔ gates are listed, never dropped.
- Record each closed checklist row with Ali's measured evidence (doc
  07 §9 / doc 10 §"Device test circle"); rows without observations
  stay open — never mark them complete.
- **Bump `versionCode`** in `android/app/build.gradle.kts` (the D84
  rule: updates install over the previous debug build) and cut the
  next debug prerelease through the release lane (tag the commit,
  upload the APK, verify via the API, record asset size + digest)
  WHEN a code change warrants it — do not re-release identical
  bytes. The pinned keystore (D85) guarantees the same certificate,
  so updates install over debug.1 without an uninstall. Full lane
  before release: desktop baseline; compile matrix; `gomobile bind`;
  **clean** `assembleDebug` (only clean builds are honest); artifact
  verification (sha256, apksigner, badging, manifest dump — doc 06's
  records); commit.
- If Ali decides to release publicly: the signed-release decision
  returns (doc 07 §8 — his local keystore or an EC key, never
  committed; README backup steps), and doc 10 §2 (the public v1
  DRAFT) is finalized against that artifact.

## Environment (this sandbox resets without warning — verify, don't assume)

Layout as verified at the D85 session close (four consecutive
no-wipe sessions before it — but verify anyway):

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
  the same certificate that signed v1.6.6-android-debug.1. After a
  sandbox wipe + re-clone, assembleDebug signs identically with NO
  extra step (this was the hole D85 closed).
- No /dev/kvm (re-verified at M8/D85). Disk is a ~9.9 GB box — clean
  Go/Gradle build caches before big builds.
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
  force-push; no unrelated changes; **no secrets or RELEASE signing
  keys committed** (D85's deliberate exception: the debug test-circle
  keystore at `android/app/debug.keystore` is vendored — the
  well-known debug identity, not a credential; release keystores stay
  out per D84/doc 07 §8); preserve Windows/Linux desktop behavior
  (the full regression gate is part of every change set).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D85 bind: the UI/release notes may only claim
  what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped.
- When you finish: update doc 06 (the evidence record), doc 07 §9 /
  doc 10 (checklist rows with Ali's evidence), doc 09 §"Current
  handoff fields", `PROGRESS.md`, `DECISIONS.md` (from D86),
  `CHANGELOG.md`; append the worklog entry (Task ID M8-c, M8-d, … per
  feedback round); commit and push; cut the debug release with the
  bumped versionCode when code changed, and record the artifact.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) whether Ali's retry result /
device-test feedback has arrived, and if so the exact first issue you
will address. Then execute — or, if no feedback has arrived, stop
after the state report and say so plainly.
