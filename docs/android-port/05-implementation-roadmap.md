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

### M3 — Build scaffolding and platform-neutral engine boundary — ✅ DONE 2026-10-16 (launch-verification ⛔, see below)

Tasks (in order):
1. ✅ Android toolchain INSTALLED (was ⛔ per doc 09 — the environment
   changed this session; dl.google.com, services.gradle.org,
   repo1.maven.org, proxy.golang.org and the Adoptium GitHub release
   were all reachable and used). Evidence: JDK 21.0.8 with `javac` at
   `~/sdk/jdk-21`; SDK cmdline-tools 12.0, platform-tools,
   `platforms;android-26` + `platforms;android-36`, `build-tools;36.1.0`
   at `~/sdk/android`; NDK 28.2.13676358 (clang 19) at
   `~/sdk/android/ndk/28.2.13676358`; gomobile + gobind installed
   (x/mobile v0.0.0-20261011034528, recorded as a `go get -tool`
   directive; D73).
2. ✅ `internal/platform/android` adapter + host tests
   (`android_test.go`: capabilities, no-op surface, ConfigDir, Notify
   forwarding, the ADR-0011 netx-level call-counting/refusal tests).
   `select` wiring behind the `android` build tag
   (`select_android.go`); the wiring required correcting doc 01 §5:
   **GOOS=android implies the `linux` tag**, so `select_linux.go` needed
   `linux && !android` (D71). Desktop and stub paths untouched in
   behavior (`select_other.go` gained only `&& !android`).
3. ✅ `internal/proxy/socks5.go` + `socks5_test.go` — CONNECT-only,
   no-auth, loopback-enforced (StartSocks5 refuses non-loopback binds),
   address types IPv4/domain/IPv6, edge cases (non-SOCKS input, wrong
   version, truncated greeting/request, zero-length domain, short
   domain, port 0, unknown ATYP, non-CONNECT commands → 0x07), resolver
   failure → 0x04, dial failure → 0x01, shared MaxConns (0x01 over
   limit), fragmentation through the door (routed ≥2 reads / unrouted
   1 read), Stop-with-Server. Race detector clean; feeds the same
   `tunnel.go` path (resolve → netx dial → first-record rule → copy).
4. ✅ `mobile/` facade + host tests — `NewEngine(configDir, protector,
   sink)` (nil protector/sink supported), the ten jsonapi methods
   verbatim, `Shutdown`, plus `StartWebUI(port)/StopWebUI` (ADR-0005's
   transport; D72) with a sink fanout so the bridge keeps events while
   the webui serves the page. Tests: API surface, start/stop cycle,
   notice forwarding, webui lifecycle (JSON + static page + fanout +
   restart), and the ADR-0011 engine-level protect-counting proof
   (≥2 calls after a full connect sequence, ≥3 after the connection
   test, phase `on` with a real latency, all against local fakes).
5. ✅ `gomobile bind` → `younblock.aar` (arm64-v8a 11.98 MB,
   armeabi-v7a 15.81 MB, x86_64 12.89 MB `libgojni.so`; `classes.jar`
   with `mobile/{Mobile,Engine,SocketProtector,EventSink}` — signatures
   verified via javap; manifest `minSdkVersion 26`). The bind runs with
   `GOFLAGS=-mod=mod` (vendor mode cannot answer `go list -m all`;
   D73). Regenerable via `build-android.sh`; never committed.
6. ✅ `android/` Gradle shell — Gradle 9.8.1 wrapper (committed),
   AGP 9.2.0 (Kotlin built into AGP 9; the separate kotlin-android
   plugin is rejected by AGP 9 — D74), minSdk 26 / target+compile 36,
   MainActivity + WebView → in-process loopback webui (ADR-0005),
   loopback-only WebViewClient, network security config (cleartext
   127.0.0.1 only), permission skeleton per doc 07 §6 (ACCESS_NETWORK_
   STATE deliberately omitted — D74), `younblock.aar` as the only
   dependency. `assembleDebug` → `app-debug.apk` 27,172,909 B,
   verified: package `io.github.assadigit.younblock` versionCode 1
   versionName 1.6.6, minSdk 26, targetSdk 36, launchable MainActivity,
   native-code arm64-v8a/armeabi-v7a/x86_64, debug-signed
   (apksigner verify OK), SHA-256
   `6db3f44d3c4af068707500ea1b5fb11d15fec9ca61cde6d3a685a2e77565ca77`.
7. ✅ Desktop regression green after every shared-tree change and again
   at the end: `go vet` clean; `go test -count=1 ./...` 16/16 `ok`
   (engine 8.0 s, proxy 9.7 s); svelte-check 0/0; vite build clean;
   `check-platform-neutral` OK; windows/amd64, windows/386, linux/amd64
   and android/arm64 cross-builds OK (arm/386/amd64 android final-exe
   link still needs NDK cgo — known, unchanged, irrelevant for the AAR
   route, which is cgo by definition).

Tests: host Go unit tests above; the Kotlin→Go init/health/stop call is
written (MainActivity + BridgeEventSink) but its runtime proof needs a
device/emulator — ⛔ this sandbox has neither (no /dev/kvm; verified
`ls /dev/kvm` → No such file or directory).

Exit criteria verdict: **debug APK builds ✅ (verified above); bridge
works without routing ✅ at the Go level (mobile tests) with the
on-device call ⛔ (no emulator/device); desktop baseline green ✅.**
The only missing piece of M3's exit criteria is the *launch* half —
environment-blocked, not silently dropped: re-verify on a KVM-capable
host or a physical device at M4 (doc 09 carries it as the standing
blocker). M3 is marked ✅ on the strength of the verified build +
bridge + desktop criteria with that one recorded exception.

### M4 — Prove the routing path — ▶ NEXT (needs the emulator/device blocker cleared for its on-device gates)

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
