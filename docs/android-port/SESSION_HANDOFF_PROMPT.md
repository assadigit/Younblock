# Prompt — Continue the Younblock Android port at M5

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M4 → M5 handoff)

- Phase 2 of the Android port. Milestones M0–M4 complete; M4 is
  **host-complete** at `main @ 9431477` (pushed, worktree clean):
  `internal/tunstack` (gVisor netstack direct, D75), the facade TUN
  lifecycle `mobile/tun.go` (D76), the `YounblockVpnService` Kotlin
  shell, and a verified debug APK (34,707,469 B, sha256 8e9f702a…8219b).
- Standing environment blocker (never claimed, never dropped): **no
  /dev/kvm and no device** — M4's on-device gates and M5's device tests
  are ⛔ with evidence and ride along until a KVM-capable host or a real
  device appears.
- The canonical live handoff is `docs/android-port/09-agent-handoff.md`
  §"Current handoff fields". If anything (including this prompt)
  disagrees with the repository, **the repository wins** — report the
  discrepancy instead of improvising.

## First actions, in order

1. Read `/home/z/my-project/worklog.md` — the shared agent worklog
   (it lives outside the repo; sandbox resets have destroyed it before
   — if it is missing, recreate it from this prompt plus the repo
   records and say so). Append your own entry (Task ID + the standard
   template) when you finish; never overwrite.
2. Read the spec pack, at minimum: doc 09 (handoff), doc 05 §M5 (your
   milestone), doc 06 (exact build/test commands and toolchain pins),
   doc 08 + `DECISIONS.md` (D1–D76; **the next decision number is
   D77**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **17/17 packages
   ok**); webui `svelte-check` (0/0) and `vite build`. D74 trap: a
   stale webui dist fails `TestWebUILifecycle` — sync
   `frontend/dist → internal/webui/dist` (build.sh) before judging.
5. Start M5 (section below). Host-testable work first.

## The M5 task — core connection lifecycle and health reporting

Scope (doc 05 §M5):

- Consent flow: explain → `prepare()` → system dialog → denial copy.
- Foreground-service state machine on top of the M4 service: honest
  notification states, `onRevoke` = disconnect, state reconciliation on
  resume, no duplicate starts (engine guards + service-side
  single-flight).
- Tests: partial-start failure injection at every step; repeated
  connect/disconnect; service death; network switching; screen-off
  persistence.
- Exit criterion: **no route/service leak or misleading "Connected"
  state in the test scenarios.** "Connected" must track genuine engine
  and connectivity readiness; never overstate VPN/privacy coverage.
- If (and only if) an emulator or device is available, fold in M4's
  device checklist: real DoH through the stack, QUIC fallback on the
  real YouTube app, protect() on real sockets, live
  notification/consent flow. Otherwise report those gates ⛔ with
  evidence — do not fake them.

## Environment (this sandbox resets without warning — verify, don't assume)

Intact layout, as verified at handoff:

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; export it**.
- JDK 21.0.12.1 at `~/sdk/jdk-21.0.12.1+1` (javac present).
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools,
  platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358, licenses
  accepted). Exact PATH / JAVA_HOME / ANDROID_HOME setup: doc 06.
- No /dev/kvm (verified again at handoff). Disk is a ~9.9 GB box —
  clear Go/Gradle build caches before big builds (a 100%-full rootfs
  once caused a mass test failure; see the worklog).
- The sandbox kills background processes when a Bash command ends —
  run long builds as foreground chunks; the caches carry the progress.

If the sandbox was wiped (repo / toolchain / Go missing):

1. Re-clone into `/home/z/my-project/Younblock` from the remote above
   (the token lives in the old clone's `origin` if any copy survives;
   otherwise ask the user for it). Then
   `git config core.fileMode false`.
2. Reinstall the toolchain exactly per doc 06's recorded pins (Go to
   `~/sdk/go` per D13; JDK, SDK and NDK at the pinned versions; module
   cache via proxy.golang.org; then rely on the committed `vendor/`).
3. Re-run the desktop baseline before any new code, and expect the
   first cold-cache run to be slow.

## Standing rules (unchanged)

- Evidence or it didn't happen: no claimed test pass, build, APK or
  push without a verified result. ⛔ with evidence for anything the
  environment cannot run — never fake completion, never silently
  narrow scope.
- Scoped commits; small steps; baseline green after each; no
  force-push; no unrelated changes; no secrets or signing keys
  committed; preserve Windows/Linux desktop behavior (the full
  regression gate is part of every milestone).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth for what was decided and why.
- When you finish: update doc 05 (M5 evidence), doc 09 §"Current
  handoff fields" (→ M6), `PROGRESS.md`, `DECISIONS.md` (from D77),
  `CHANGELOG.md`; append the worklog entry; commit and push.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) the exact first M5 task you will
execute. Then execute it — do not stop after repeating instructions.
