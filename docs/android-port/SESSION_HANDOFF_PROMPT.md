# Prompt — Continue the Younblock Android port at M7

You are resuming a long-running, multi-session engineering project — not
starting fresh. Read this prompt fully, then act.

Repository: <https://github.com/assadigit/Younblock> (private; the
working clone carries the credential in its `origin` remote URL).

## Where the project stands (verified at the M6 → M7 handoff)

- Phase 2 of the Android port. Milestones M0–M6 complete; M6 is
  **host-complete** at `main` (the M6 commit; see `git log -1`):
  the `apps` settings field (schema v2 + Normalize + v1→v2 migration,
  D79 — one list behind three doors, apply-on-next-connect), the
  `RoutedApps()` facade binding the VPN service reads at TUN
  establish, and the mobile UX (D80 — the Apps-to-unblock row, the
  VPN-slot/key-icon About lines, the doc 07 §5 privacy disclosure, the
  diagnostics view, all capability-gated so desktop renders its exact
  prior copy). Verified debug APK: 35,194,757 B, sha256 5669efd1…02dc.
- Standing environment blocker (never claimed, never dropped): **no
  /dev/kvm and no device** — the shared M4+M5 on-device checklist
  (live consent dialog, notification channel behavior, screen-off/
  Doze, real DoH through the stack, QUIC fallback, protect() on real
  sockets, reboot/onRevoke) now also carries M6's live-WebView halves
  (real rotation, font scaling, TalkBack, the apps row on a real
  screen). All ⛔ with evidence; all ride together.
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
2. Read the spec pack, at minimum: doc 09 (handoff), doc 05 §M7 (your
   milestone), doc 06 (exact build/test commands and toolchain pins),
   doc 07 (§7 dependencies/licenses, §8 signing, §9 the
   release-blocking checklist), doc 08 + `DECISIONS.md` (D1–D80; **the
   next decision number is D81**), and the tail of `PROGRESS.md`.
3. Verify the environment (section below) and the tree:
   `git status -sb`, `git log --oneline -3`. Preserve any uncommitted
   work; never reset it just to get a clean diff.
4. Make the desktop baseline GREEN before writing any code:
   `go vet ./...`; `go test -count=1 ./...` (expect **17/17 packages
   ok**, run twice consecutively); webui `svelte-check` (0/0) and
   `vite build`. D74 trap: a stale webui dist fails
   `TestWebUILifecycle` — sync `frontend/dist → internal/webui/dist`
   (build.sh) before judging.
5. Start M7 (section below). Host-testable work first.

## The M7 task — regression, hardening, release preparation

Scope (doc 05 §M7; doc 09 "Exact next task"):

- Full test matrix + desktop regressions (the standing gate, re-run
  and recorded).
- Dependency/license audit re-run (doc 07 §7 — record transitive
  natives from the gVisor module set; attribution notices in About
  suffice if all permissive).
- Permission review (doc 07 §6's inventory vs the built APK's actual
  manifest — verify no permission the code does not use).
- The NO_TARGET_APPS typed-error door decision (deferred from M6 by
  D79's session note: a shell→engine error surface is a D78-class
  design question — decide it deliberately, not incidentally).
- Battery/behavior measurement on a real device; reboot/revocation
  checks (AND-028) — **⛔ without a device; document, never fake**.
- Release notes drafting (doc 02 §4 limitations, doc 07 §1 truthful
  claims, AND-020/AND-021 disclosure notes).

## Environment (this sandbox resets without warning — verify, don't assume)

Intact layout, as verified at handoff:

- Repo: `/home/z/my-project/Younblock`, `vendor/` committed (D68: use
  `-mod=vendor`; the Android lane never invokes wails; desktop lanes
  keep the `wails -m` rule).
- Go 1.27.2 at `~/sdk/go/bin` — **not on PATH; source `~/sdk/env.sh`**.
- JDK 21.0.12.1+1 (Temurin, javac present) at `~/sdk/jdk-21.0.12.1+1`
  (the system JDK is a JRE without javac — use the Temurin one).
- Android SDK at `~/sdk/android` (cmdline-tools 12.0, platform-tools,
  platforms 26/36, build-tools 36.1.0, NDK 28.2.13676358, licenses
  accepted). Exact PATH / JAVA_HOME / ANDROID_HOME setup: doc 06.
- No /dev/kvm (verified again at handoff). Disk is a ~9.9 GB box —
  clear Go/Gradle build caches before big builds (a 100%-full rootfs
  once caused a mass test failure; see the worklog).
- The sandbox kills background processes when a Bash command ends —
  run long builds as foreground chunks; the caches carry the progress.
- The honest compile-matrix form (sharpened at M6): `go build` over
  the 18 non-main packages per ABI at CGO_ENABLED=0 (arm64 links the
  full tree; arm/386/amd64 main packages refuse the link without NDK
  cgo — that refusal is the documented, expected behavior).

If the sandbox was wiped (repo / toolchain / Go missing):

1. Re-clone into `/home/z/my-project/Younblock` from the remote above
   (the token lives in the old clone's `origin` if any copy survives;
   otherwise the session prompt carries it — re-cloning works without
   asking anyone). Then `git config core.fileMode false` and
   `chmod +x android/gradlew` (both bit someone at M6).
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
  force-push; no unrelated changes; no secrets or signing keys
  committed; preserve Windows/Linux desktop behavior (the full
  regression gate is part of every milestone).
- Do not assume Wails or desktop proxy semantics transfer to Android;
  the ADRs (doc 08) are the ground truth for what was decided and why.
- The M5/M6 honesty rules (D77–D80) bind M7 too: the UI may only
  claim what the engine actually enforces; privacy copy follows
  implementation evidence; never a false "Connected", never an
  overstated scope.
- When you finish: update doc 05 (M7 evidence), doc 06 (build record),
  doc 09 §"Current handoff fields" (→ M8), `PROGRESS.md`,
  `DECISIONS.md` (from D81), `CHANGELOG.md`; append the worklog entry
  (Task ID M7); commit and push.

## Required first response

Report, briefly and with evidence: (1) environment — intact or
recovered, and how you know; (2) repo state — branch, HEAD,
cleanliness; (3) baseline results; (4) the exact first M7 task you will
execute. Then execute it — do not stop after repeating instructions.
