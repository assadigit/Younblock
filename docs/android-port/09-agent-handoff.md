# 09 — Agent Handoff and Starting Checklist

**Status:** Updated at the Phase 1 → Phase 2 boundary (2026-10-16).
This file is the live handoff; keep the fields in §"Current handoff
fields" accurate at every session boundary.

## If starting Phase 2 (implementation) — CURRENT

1. Read every spec file; they are now evidence-backed, not baseline
   templates. The audit (doc 01) and ADRs (doc 08) are the ground
   truth; verify anything that looks stale against the repository.
2. Inspect the working tree; preserve unrelated/uncommitted changes.
3. Begin at **M3** (doc 05) — the Android toolchain first, then the
   Go-side additions (platform adapter, SOCKS5 door, mobile facade)
   which are host-testable even without the SDK.
4. Small changes; run the relevant tests after each; keep the desktop
   baseline (doc 01 §7) green on shared-tree edits.
5. Stop and document (ADR update) if a critical assumption fails; do
   not fake functionality or silently narrow the product scope.
6. Build the debug APK once the approved path is implementable; verify
   its actual existence/metadata; report tests honestly.
7. Update the specs with implementation reality, exact commands,
   artifact path/checksum, and commit/push status.
8. Keep the worklog discipline: read `/home/z/my-project/worklog.md`
   first; append your entry (Task ID + template) when done.

## Phase 1 record (completed 2026-10-16)

- Full source audit with symbols and line references (doc 01).
- Feasibility matrix scored; architecture accepted (docs 03, 08).
- Baseline green: go vet clean; go test 13/13; svelte-check 0/0;
  vite build clean; Android compile matrix 4/4 ABIs.
- Documentation-only phase: **zero application code changed**; the
  only repository additions are the completed specs themselves (and
  the extraction of this pack from the zip Ali uploaded).

## Environment / execution honesty

- **This sandbox has no Android SDK, no NDK, and a JRE without
  javac.** `gomobile bind`, Gradle builds, emulator and device tests
  cannot run here until a toolchain is installed (M3 task 1). The
  Go-side M3 items (platform adapter, SOCKS5 door, mobile facade +
  host tests) are fully runnable without it.
- proxy.golang.org and the Debian/Ubuntu mirrors are reachable from
  this sandbox; the wails `-m` rule (D68) still applies to desktop
  lanes.
- The sandbox kills background processes when a Bash command ends
  (verified twice across sessions); long builds run in foreground
  chunks.
- Do not claim source inspection, test success, APK creation, upload,
  or push without evidence. Do not commit secrets, signing keys, user
  settings, or private diagnostics. Do not undo unrelated working-tree
  changes to achieve a clean diff.

## Current handoff fields

- **Phase completed:** Phase 1 (M0 audit, M1 feasibility, M2 spec
  finalization) — 2026-10-16.
- **Branch and commit:** `main` @ `36273fb` + this docs commit (see
  `git log -1` at handoff time); worktree clean except the spec-pack
  changes of this session.
- **Files changed this phase:** `docs/android-port/*` (all ten docs
  completed + this status), extracted from
  `Younblock-Android-Specs.zip` (Ali's upload `36273fb`). No source
  code touched.
- **Tests run and actual results:** `go vet ./...` clean;
  `go test -count=1 ./...` 13/13 `ok`; svelte-check 0/0; vite build
  clean; `GOOS=android` build matrix — arm64 full-module exit 0,
  arm/386/amd64 packages compile (final exe link needs NDK cgo);
  `go vet` android/arm64 clean. Frontend/backend untouched otherwise.
- **Current architecture decision / confidence:** VpnService +
  per-app allow-list + userspace stack (tun2socks primary, gated) +
  SOCKS5 door + unchanged engine + gomobile AAR + Svelte WebView via
  in-process loopback webui. Confidence: high on code seams (all
  verified in source), medium on runtime integration (prototype
  pending SDK), device-level items explicitly gated at M4.
- **Blocking questions:** none architectural. Environment blocker:
  Android toolchain absent (M3 task 1). Product confirmations wanted
  from Ali (non-blocking, defaults chosen): target SDK level,
  allow-list defaults beyond Chrome+YouTube, keystore handling
  (doc 07 §8).
- **Exact next task:** **M3 task 1** — install/verify the Android
  toolchain (SDK cmdline-tools, NDK, JDK 17+); if unavailable in the
  execution environment, proceed with M3 tasks 2–4 (Go-side
  `internal/platform/android`, `internal/proxy/socks5.go`,
  `mobile/` facade — all host-testable) and report the AAR/Gradle
  items as blocked with exact evidence.
