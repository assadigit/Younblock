# 05 — Implementation Roadmap

**Status:** Phase 1 (M0–M2) COMPLETE 2026-10-16 — this file now tracks
Phase 2 execution. Status legend: ✅ done (evidence linked), ▶ next,
⬜ pending, ⛔ blocked in the current environment (see doc 09).

## Phase 1: audit and finalize the blueprint — COMPLETE

### M0 — Repository baseline and source map — ✅

- ✅ Git state inspected and preserved (doc 01 §2; phantom mode-flips
  diagnosed, nothing discarded).
- ✅ Go packages, Wails runtime, frontend, scripts, tests, release
  workflow inventoried with evidence (doc 01 §2–§4).
- ✅ Connect → engine → relay → DNS/TLS → OS proxy → status traced
  with symbols and line references (doc 01 §3).
- ✅ Desktop baseline recorded: `go vet` clean; `go test ./...`
  13/13 packages `ok`; svelte-check 0/0; vite build clean (2026-10-16,
  doc 01 §7).
- ✅ `01-current-architecture-audit.md` completed — no "To verify"
  rows remain.

### M1 — Architecture feasibility decision — ✅

- ✅ Browser-proxy vs Android-proxy vs VpnService/TUN vs WebView-only
  compared with evidence; matrix scored (doc 03 §2, §7).
- ✅ Relay protocol identified: loopback HTTP proxy (CONNECT + forward
  + PAC + health) → SOCKS5 door added for the TUN path (doc 03 §3,
  ADR-0008).
- ✅ Go dependencies + Android ABI compile matrix audited (all four
  ABIs compile; doc 01 §5).
- ✅ Kotlin-native vs Svelte-WebView compared with production evidence
  (doc 03 §5).
- ✅ Highest-risk assumptions turned into M3/M4 prototype gates
  (protect hook, stack selection, QUIC fallback) — ADR-0008/0011.

### M2 — Finalize implementation specification — ✅

- ✅ Feature priorities + traceability reconciled (doc 02).
- ✅ APIs, module boundaries, state transitions, error mapping,
  storage, cleanup finalized (doc 04).
- ✅ Build/test matrix and supported-device declaration drafted with
  verified values where possible (doc 06).
- ✅ First implementation checklist = the M3 list below.

## Phase 2: implementation and APK delivery

### M3 — Build scaffolding and platform-neutral engine boundary — ▶ NEXT

Tasks (in order):
1. ⛔→▶ Install the Android toolchain (SDK cmdline-tools, NDK, JDK 17+;
   the audit sandbox has none of these — doc 09 §Environment; if
   unavailable, do the Go-side items first and stop at the AAR build
   with an honest report).
2. ⬜ `internal/platform/android` adapter (Name/Capabilities/DialControl
   /Attach no-op/Notify callback/ConfigDir; `select` wiring behind a
   `android` build tag so desktop and the existing `!windows && !linux`
   stub path stay untouched).
3. ⬜ `internal/proxy/socks5.go` + tests (CONNECT-only, address types,
   loopback, no auth, fuzz-ish edge cases like the old spec A1).
4. ⬜ `mobile/` facade (string JSON API + `NewEngine` + EventSink +
   SocketProtector) with host-runnable tests.
5. ⬜ `gomobile bind` → `younblock.aar` (arm64-v8a, armeabi-v7a;
   x86_64 for the emulator in debug builds).
6. ⬜ Minimal `android/` Gradle shell: MainActivity + WebView loading
   the loopback webui (ADR-0005) + app metadata/permissions skeleton
   (doc 07 §6).
7. ⬜ Desktop regression run (doc 01 §7 baseline) — must stay green.

Tests: clean Android build; host Go unit tests; a minimal
Kotlin→Go init/health/stop call.

Exit criteria: debug APK builds and launches; bridge works without
network routing yet; desktop baseline green.

### M4 — Prove the routing path

- ⬜ VpnService + TUN (per-app allow-list, virtual DNS, routes per
  ADR-0009/0010) + chosen stack (tun2socks engine package; switch to
  gVisor-direct per ADR-0008 criteria if measured evidence demands).
- ⬜ Stack → SOCKS5 door integration; DNS interceptor → DoH.
- ⬜ Protect-hook on-device verification (no routing loop; every dial
  protected — call-counting test first in M3's host tests).
- ⬜ Protocol matrix tests: TCP through-path, DNS A/AAAA/other, UDP
  drop incl. QUIC fallback on the real YouTube app (report, don't
  guess), IPv6 fail-safe on dual-stack.
- ⬜ Failure/cleanup: stop while active, partial-start injection,
  timeouts, resets.

Exit criteria: the core traffic path has an end-to-end test on an
emulator/device — not a UI-level success state.

### M5 — Core connection lifecycle and health reporting

- ⬜ Consent flow (explain → prepare → system dialog → handle denial),
  foreground service + notification + `onRevoke`, state reconciliation
  on resume, no duplicate starts (engine guards + service-side
  single-flight).
- ⬜ Tests: partial-start failure injection at every step; repeated
  connect/disconnect; service death; network switching; screen-off
  persistence.

Exit criteria: no route/service leak or misleading Connected state in
the test scenarios.

### M6 — Settings and mobile UX

- ⬜ `apps` settings field (schema v2 + Normalize + migration), the
  Apps-to-unblock UI row, first-run copy, About lines (key-icon
  honesty), Help/privacy disclosure (doc 07 §5), diagnostics view.
- ⬜ Accessibility/rotation/font-scaling checks in the WebView.

Exit criteria: P0 + approved P1 features meet doc 02.

### M7 — Regression, hardening, release preparation

- ⬜ Full test matrix; desktop regressions; dependency/license audit;
  permission review; battery/behavior measurement on a real device;
  reboot/revocation checks (AND-028).

### M8 — Package, verify, hand off

- ⬜ Debug + release (signed) APKs; artifact path/ID/version/ABI/
  checksum recorded; install + smoke-test; docs/notes updated;
  commit/push with the pre-existing-change rule.

## Dependency order / critical path (unchanged)

M3 → M4 → M5 → M6 → M7 → M8; M0–M2 complete.

## Change-control rules (unchanged)

Scoped commits; no Android work mixed with unrelated refactors; no
overwriting dirty work; no force-push; ADR updates when a milestone
changes a decision; limitations preserved in release notes; no task
marked complete because code was written — only when its exit
criterion is evidenced.
