# Prompt — Continue the Younblock Android port at M8

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M7 → M8 handoff)

- Phase 2 of the Android port, final milestone. M0–M7 complete; M7 is
  **host-complete** at `main` (the M7 commit; see `git log -1`): the
  NO_TARGET_APPS refusal door (D81 — shell→engine typed errors through
  the start gate, the phase machine stays the single truth), the
  spawn-race zombie closed by run generations (D82 — a cancelled Start
  can never be resurrected by its own goroutine; one test had been
  passing through the bug), the `-race` lane green on the seven key
  packages, the dependency/license audit (all permissive; About
  attribution added), the permission review (the built APK's manifest
  is exactly doc 07 §6's inventory), and the release-notes draft
  (doc 10). Verified debug APK: 35,298,149 B, sha256 f27b7eef…491.
- Standing environment blocker (never claimed, never dropped): **no
  /dev/kvm and no device** — the shared checklist (live consent,
  notification channel, screen-off/Doze, real DoH, QUIC fallback,
  protect(), reboot/onRevoke, rotation/font-scale/TalkBack, battery)
  rides together; M8's install/launch smoke-test halves are ⛔ too.
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
2. Read the spec pack, at minimum: doc 09 (handoff), doc 05 §M8 (your
   milestone), doc 06 (build commands + the M7 record), doc 07 (§8
   signing/keystore, §9 the checklist's open rows), doc 10 (the
   release-notes draft you will finalize), doc 08 + `DECISIONS.md`
   (D1–D82; **the next decision number is D83**), and the tail of
   `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **17/17 packages
   ok**, run twice consecutively); webui `svelte-check` (0/0) and
   `vite build`. D74 trap: a stale webui dist fails
   `TestWebUILifecycle` (in the mobile package) — sync
   `frontend/dist → internal/webui/dist` before judging.
5. Start M8 (section below).

## The M8 task — package, verify, hand off

Scope (doc 05 §M8; doc 09 "Exact next task"):

- Build BOTH variants: the debug APK (the standing lane) and the
  release APK. **The keystore question for Ali becomes blocking for
  the signed variant**: doc 07 §8 says a local keystore, never
  committed, README backup steps. If Ali has not provided one, create
  a fresh local keystore (documented command), sign, record the cert
  fingerprint — and note explicitly that losing it means
  uninstall-before-update; never commit it.
- Record for every artifact: exact path + size, variant, application
  ID, versionName/Code, ABIs, min/target SDK, signing state, SHA-256,
  and which of doc 06 §5's five verification levels actually ran
  (compiled / installed / UI smoke / engine integrated / target
  traffic path). Install + launch smoke: ⛔ without a device —
  document with evidence, never fake.
- Finalize doc 10's release notes against the shipped artifact (M8's
  copy review vs doc 07 §1); re-run the dependency-closure check and
  the platform-requirements recheck (doc 07 §9's remaining M8 rows).
- Docs/notes updated end-to-end; commit/push with the
  pre-existing-change rule.
- The D77–D82 honesty rules bind to the end: every claim in the
  release notes traces to evidence; ⛔ gates stay listed, never
  quietly dropped.

## Environment (this sandbox resets without warning — verify, don't assume)

Intact layout, as verified at handoff (the M7 session was the first
with NO wipe — but verify anyway):

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**.
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk-21.0.12.1+1`
  (the system JDK is a JRE without javac — use the Temurin one).
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools,
  platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358, licenses
  accepted). Exact PATH / JAVA_HOME / ANDROID_HOME setup: doc 06.
- No /dev/kvm (verified at the M7 handoff). Disk is a ~9.9 GB box —
  clean Go/Gradle build caches before big builds; ~3 GB free at the
  M7 exit.
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
  committed** (the M8 keystore especially — .gitignore it); preserve
  Windows/Linux desktop behavior (the full regression gate is part of
  every milestone).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth.
- The honesty rules D77–D82 bind M8: the UI/release notes may only
  claim what the engine actually enforces; privacy copy follows
  implementation evidence; ⛔ gates are listed, never dropped.
- When you finish: update doc 05 (M8 evidence), doc 06 (build record),
  doc 07 §9 (close or carry the M8 rows), doc 09 §"Current handoff
  fields" (→ Phase 2 COMPLETE), doc 10 (finalized), `PROGRESS.md`,
  `DECISIONS.md` (from D83), `CHANGELOG.md`; append the worklog entry
  (Task ID M8); commit and push.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) the exact first M8 task you will
execute. Then execute it — do not stop after repeating instructions.
