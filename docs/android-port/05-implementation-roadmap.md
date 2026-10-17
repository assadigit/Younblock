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

### M4 — Prove the routing path — ✅ host-complete 2026-10-16 (on-device gates ⛔: the standing emulator/device blocker)

The userspace stack and its wiring are done and host-verified; the
on-device halves (real DoH through the stack, QUIC fallback on the
real YouTube app, protect() on a real socket) remain ⛔ — this sandbox
still has no /dev/kvm and no device (re-verified this session:
`ls /dev/kvm` → No such file or directory). Host side of every gate:

1. ✅ **The stack**: `internal/tunstack` — gVisor netstack direct
   (ADR-0008's switch path exercised at design level, D75; the
   tun2socks engine package is a CLI global singleton whose UDP model
   demands a UDP-capable proxy — incompatible with the CONNECT-only
   door, ADR-0009). Channel-endpoint wire (the fdbased upgrade is the
   documented same-seam swap), tcp/udp forwarders wired before the NIC,
   spoofing+promiscuous; gvisor.dev/gvisor pinned and **vendored**
   (D68: vendor means vendor; +6 MB), `go 1.26.3` directive bumped by
   the dependency.
2. ✅ **Protocol policy in-tunnel (ADR-0009/0010), host-proven**: TCP
   → door (domain ATYP 0x03 for fake-pool destinations so the door's
   MatchAnyDomain fragmentation matches the browser path; literal IPs
   cross honestly; unmapped fakes RST), UDP:53 → virtual DNS
   (A=fake-pool 198.18.0.0/15 with FIFO eviction, AAAA empty, other
   types empty, FORMERR/NOTIMP per shape — any destination address:53
   answered, so hardcoded-DNS apps work), other UDP dropped
   (fail-fast), IPv6 never enabled and dropped at the wire. Ten tests,
   `internal/tunstack` 4.7 s `ok`, including the full through-path
   against the REAL proxy Server (phone netstack → synthetic TUN →
   stack → door → resolver → netx → echo origin).
3. ✅ **The facade's TUN lifecycle** (`mobile/tun.go`): StartTun (engine
   serving required; opens the door on the running server), StopTun
   (idempotent; closes the door it opened on the same server),
   TunStats, Shutdown-with-live-stack. Host tests drive the REAL engine
   over a SOCK_SEQPACKET socketpair (packet-preserving like an
   IFF_NO_PI TUN fd): DNS round-trip through the facade-built stack, a
   hand-rolled SOCKS5 CONNECT through the engine's own door resolving
   via the engine's (fake) DoH with the upstream dial protect()-ed
   (the M3 call-counting proof extended through the facade), gates and
   idempotency, Shutdown unwinds. Two real bugs the tests caught are
   recorded as D76: the O_NONBLOCK/netpoller requirement (a blocking
   foreign fd makes Stop hang forever — no Close can unwind a blocked
   read(2)) and the fd single-owner rule (StartTun consumes the fd on
   every path so Kotlin never double-closes).
4. ✅ **The Kotlin service** (compile-verified into the M4 APK):
   `YounblockVpnService` (foreground, connectedDevice type) — per-app
   allow-list (Chrome+YouTube defaults; fail-closed when none is
   installed: zero allowed apps would route EVERYTHING, the opposite
   of the policy), virtual DNS 10.111.0.53, v4 default route + v6
   blackhole route (ADR-0010), MTU 1500, protect() via the process-wide
   VpnProtector (ADR-0011: engine dials protected while the TUN lives;
   unprotected-but-correct when it does not), engine-state-driven
   startTun/stopTun, onRevoke = disconnect, honest notification.
   MainActivity wires the connect flow: page toggle → engine
   connecting → VpnService.prepare() consent → service start. Manifest
   carries the service (BIND_VPN_SERVICE, exported=false).
5. ✅ **Build + desktop baseline**: gomobile bind → younblock.aar
   (24,893,234 B, 3 ABIs — gVisor netstack added ~7 MB over M3);
   assembleDebug → app-debug.apk 34,707,469 B, SHA-256
   8e9f702a364f1e41a0bf12fa3a89971fced6e993632b56f74b44a55d06c8219b,
   badging minSdk 26/target 36, YounblockVpnService in the manifest,
   apksigner verify OK (debug cert). Toolchain restored this session
   after another sandbox reset (JDK 21.0.12.1, SDK 26+36/36.1.0, NDK
   28.2.13676358 — doc 06 §1 records the pins). Desktop baseline:
   vet clean; `go test -count=1 ./...` **17/17 packages ok** (tunstack
   and mobile grew); svelte-check 0/0; vite build clean; Android
   library compile matrix 4/4 ABIs with `-mod=vendor`.
6. ⛔ **On-device gates** (M4's exit criterion — an end-to-end traffic
   test on an emulator/device): real DoH through the stack, the QUIC
   fallback on the real YouTube app, protect() on real sockets, the
   notification/consent flow live. Blocked by the environment, carried
   in doc 09 as the standing blocker — not claimed, not dropped.

Exit criteria verdict: **every host-runnable gate green with evidence
above; the emulator/device end-to-end half ⛔ (no /dev/kvm, no device).
M4 is marked ✅ host-complete on the M3 precedent (build + bridge +
desktop verified, on-device half recorded), with the on-device gates
folded into the M5 device checklist.**

### M5 — Core connection lifecycle and health reporting — ✅ host-complete 2026-10-17 (on-device gates ⛔: the standing emulator/device blocker)

The lifecycle core is done and host-verified against the REAL engine
and stack; the on-device halves (the live consent dialog, notification
channel behavior on a real shell, screen-off/Doze persistence,
reboot/revocation on-device) remain ⛔ — this sandbox still has no
/dev/kvm and no device (re-verified this session: `ls /dev/kvm` → No
such file or directory). Host side of every gate:

1. ✅ **The consent gate (D77 — the engine's connect run cannot reach
   "on" before the product may route)**: the M4 reality had the page
   starting the engine directly through the webui, so consent arrived
   AFTER Start — an engine that turns "on" with no TUN would have been
   a misleading "Connected". The fix is the optional platform door
   `engine.StartGater` (internal/engine, an interface the Android
   adapter implements; desktop adapters do not, and their Start path
   is unchanged, byte for byte): the connect run waits at the door
   BEFORE the listener, holding the phase at "connecting", outside the
   start budget (a permission dialog is not a network failure —
   `internal/engine/gate_test.go` proves the hold outlives a 200 ms
   StartTimeout and still reaches "on"; Stop and a second Start cancel
   the held run cleanly, and the abandoned goroutine never touches the
   phase — the canceller owns the transition). The facade arms the
   gate closed at construction (`mobile.NewEngine`); Kotlin opens it
   (`setVpnReady(true)`) once consent is granted AND the TUN is
   established, and re-arms it closed on every unwind.
2. ✅ **The honest routing report (D78 — VpnState)**: `mobile/vpnstate.go`
   answers the product's "Connected" question in one call —
   `{"phase":…,"tun":…,"routing":…,"door":…}` — where routing requires
   the engine serving AND a stack bound AND that stack's door being
   the LIVE server's door (an engine restart builds a new server; a
   stack still holding the old door is stale and must not report
   routing — the lifecycle suite caught exactly that case and the
   derivation now carries the server-identity check). The stale signal
   (`tun:true, routing:false`) is D76's "missed event" surface made
   observable for the shell's reconciliation.
3. ✅ **The lifecycle suite** (`mobile/lifecycle_test.go`, against the
   real engine + real tunstack, the M4 harness split for gate-driven
   starts): consent hold → grant → "on"; denial modeled as
   Stop-during-hold → clean "off"; three full connect/disconnect
   cycles with the report honest at every step (bound → routing; user
   stop → stale stack REPORTED; unwind → quiet); partial-start failure
   injection at the engine step (a pinned busy port → typed
   PORT_BUSY error, routing never claimed); the stale-stack refusal
   (TUN_ACTIVE after a missed event) with reconciliation (StopTun →
   fresh bind → routing again); service death (Shutdown with routing
   live → all-false, never a zombie "Connected"). Adapter-level gate
   semantics have their own suite (`internal/platform/android/
   gate_test.go`: zero-value open, hold/open/re-arm, cancellation,
   all-waiters release).
4. ✅ **The Kotlin lifecycle shell** (compile-verified into the M5 APK,
   device behavior ⛔): the consent flow — first-run explanation
   dialog (once, SharedPreferences) → `VpnService.prepare()` → system
   dialog → grant: service starts, establishes the TUN, opens the
   gate; denial: the held engine stops (never reached "on") plus the
   honest denial copy; consent is single-flight (process-wide flag,
   rotation-safe) and the service start is single-flight through its
   running flag. The notification text is derived from the facade's
   vpnState report — "active" only when it actually reports routing,
   "degraded" when it routes degraded, "connecting" otherwise (the
   same JSON the host tests verify). Reconciliation: MainActivity
   re-derives the state on resume (connecting + no service → restart
   the service path; serving + no service → stop the engine — nothing
   routes); YounblockVpnService.onDestroy stops the engine whenever
   it is serving or connecting (service death never leaves a lying
   "Connected" or an invisible zombie) and re-arms the gate; the
   service restarts the engine itself on its next onStartCommand when
   a connect intent survives (e.g. a consent result after rotation).
   onRevoke = engine stop + TUN unwind + gate re-arm + stop (re-consent
   on the next connect — revoked permissions make prepare() ask again).
5. ✅ **Build + desktop baseline**: gomobile bind → younblock.aar
   24,893,759 B (3 ABIs; the M5 bindings setVpnReady/vpnState verified
   in classes.jar via javap); clean `assembleDebug` → app-debug.apk
   34,711,941 B, SHA-256
   c46026a7984d9fda69b45954436c630f625a74fecf451e6e50c2a3d2b6d5bf26,
   badging io.github.assadigit/younblock 1.6.6 (versionCode 1),
   minSdk 26/target 36, YounblockVpnService + BIND_VPN_SERVICE in the
   dumped manifest, apksigner verify OK (debug cert). Toolchain
   reinstalled this session after another full sandbox wipe (repo
   re-clone at 98cac30, Go/JDK/SDK/NDK per doc 06 pins; the worklog
   records the recovery). Desktop baseline: vet clean; `go test
   -count=1 ./...` **17/17 packages ok** (engine +8.9 s, mobile 33.7 s
   — the lifecycle suite; run twice consecutively, no flakes);
   svelte-check 0/0; vite build clean; Android library compile matrix
   4/4 ABIs with `-mod=vendor`.
6. ✅ **Two real bugs found and fixed while building the suite** (the
   M5 "failure injection at every step" working as intended): (a) the
   M4 facade's DOOR refusal paths returned without closing the fd,
   violating D76's documented single-owner rule ("every refusal closes
   it") — both DOOR paths in `mobile/tun.go` now close the fd; (b) the
   M4 test harness saved settings with AutoPort=false (zero value) +
   the fixed default port 4525 — harmless while mobile tests ran
   alone, but under concurrent package execution (go test ./...) the
   engine package can hold 4525 during the M5 gate's delayed bind and
   the honest PORT_BUSY failure surfaced it; the harness now writes
   the product default (AutoPort=true), matching DefaultSettings.
7. ⛔ **On-device gates** (M5's exit criterion — the lifecycle observed
   live): the consent dialog sequence (explain → prepare → grant/deny)
   with real rotation/background/resume; notification channel behavior
   across connecting/active/degraded; screen-off persistence (Doze);
   network switching on a real radio; stop from the system VPN
   settings (onRevoke); reboot (nothing self-starts, AND-028); the M4
   checklist folded here (real DoH through the stack, QUIC fallback on
   the real YouTube app, protect() on real sockets). Blocked by the
   environment, carried in doc 09 as the standing blocker — not
   claimed, not dropped.

Exit criteria verdict: **no route/service leak or misleading
"Connected" state in the host-runnable scenarios — every one ends with
the report quiet (off/false/false) or honestly degraded, the gate keeps
an unpermitted engine at "connecting", and every unwind path re-arms.
The emulator/device observation half ⛔ (no /dev/kvm, no device). M5 is
marked ✅ host-complete on the M3/M4 precedent (build + bridge +
desktop verified, on-device half recorded, never faked).**

### M6 — Settings and mobile UX — ✅ host-complete 2026-10-17 (on-device gates ⛔: the standing emulator/device blocker)

Settings schema v2 and the honest mobile UX are done and host-verified;
the live-WebView observation halves (rotation, font scaling, TalkBack,
the apps row on a real screen) remain ⛔ — no /dev/kvm, no device
(re-verified this session: `ls /dev/kvm` → No such file or directory).
Host side of every gate:

1. ✅ **The `apps` settings field (D79 — schema v2 + Normalize +
   migration)**: `config.Settings` gains `Apps []string` (json `apps`);
   `settingsVersion` bumps to 2; migration is the existing version
   mechanism — a v1 file simply lacks the field and `normalizeApps`
   fills the built-in defaults (Chrome + YouTube, ADR-0003) so the
   list can never persist empty (an empty allow-list would route EVERY
   app — the opposite of the product scope; the Kotlin builder fails
   closed independently when none of the listed packages is
   installed). Validation: trim, Android-package shape (dot-separated
   segments, each starting with a letter; case preserved — package
   names are case-sensitive), dedupe first-seen, cap 32. Evidence:
   `internal/config/apps_test.go` — defaults copy-ness, the full
   validation matrix, the cap, the v1→v2 file migration (custom fields
   survive, `apps` defaults filled, persisted file carries version 2 +
   the list), round-trip, and the exact JSON shape the facade and UI
   depend on. Desktop unchanged: the classic screen's spread-based
   saves round-trip the field untouched; desktop platforms never read
   it; the full regression gate stayed green.
2. ✅ **The facade plumbing (one list, three doors)**: the engine gained
   a locked `Settings()` copy accessor; `mobile.RoutedApps()` reports
   `{"apps":[…]}` from the same normalized truth — the identical list
   `GetSettings` serves and the WebView's Apps-to-unblock row edits.
   Evidence: `mobile/apps_test.go` — default list, tracked-through-save,
   all-invalid→defaults-restored (RoutedApps and GetSettings agree),
   and persistence across an engine recreate (the process-death path).
   `javap` on the rebuilt AAR's classes.jar shows `routedApps()` bound
   alongside the M4/M5 methods.
3. ✅ **The Kotlin consumption (apply-on-next-connect, D79)**:
   `YounblockVpnService.establishTun` reads the configured list fresh
   at establish time through `readRoutedApps` (fail closed: an
   unreadable or empty list refuses to establish — never a silent
   route-everything fallback; `DEFAULT_APPS` is deleted so no second
   copy can drift). A change saved while the TUN is live applies on
   the NEXT connect — the list is baked into the VpnService builder,
   and rebuilding an established TUN under live traffic is riskier
   than the delay; the UI copy says so. The NO_TARGET_APPS
   typed-error surface stays deferred to M7 with the other hardening
   (the shell-level failure currently logs and fails closed honestly).
4. ✅ **The WebView UX (D80 — capability-gated honesty surfaces)**: the
   pro screen's `Backend` grew `getSettings`/`saveSettings`/
   `getDiagnosticReport` on all three transports (Wails bindings, the
   webui's /api/settings + /api/report, preview null — no invented
   data). The menu sheet gains, gated on what the host actually
   reports (`AppInfo.platform`, `capabilities.perAppRouting`): the
   **Apps-to-unblock row** (the scope intro line — the first-run copy,
   present always because the disclosure IS the feature; friendly
   labels only for the two known packages, anything else shows its
   package name; remove with a last-one guard; add-by-package-name
   with honest feedback when Normalize refuses the entry — the saved
   response is the authority, the UI duplicates no validation rules);
   the **Android About lines** (the VPN-slot/key-icon statement
   replaces the desktop "not a VPN" line with the honest slot
   disclosure — ADR-0007/doc 07 §1: no overstated VPN/privacy
   language); the Android **How-it-works/Help** variants (the first
   line's flow is the VPN slot, not a browser sending requests; the
   key-icon explanation; apply-on-next-connect advice); the **Privacy
   section** (doc 07 §5 → user copy: no analytics, no Younblock-run
   server, DNS to the user-configured DoH providers, IP not hidden,
   no extra encryption, local logs — every sentence
   implementation-verified); and the **diagnostics view** (the
   existing redacted GetDiagnosticReport in a scrollable view with a
   copy button — no new report content invented). Desktop rendering
   is unchanged: the sheet's desktop sections keep the exact copy,
   byte for byte; the classic screen is untouched. Evidence:
   svelte-check 0/0, vite build clean, the bundle synced into
   internal/webui/dist (TestWebUILifecycle green) and into the M6 APK.
5. ✅ **Accessibility (host-verifiable half)**: the sheet's generic
   focus-trap query automatically includes every new control (remove
   buttons, the add field, diagnostics buttons); aria-labels on every
   icon-only control; 44 px touch targets (2.75 rem); rem typography
   throughout for system font scaling; `role="alert"` on errors; the
   report view is a labeled, scrollable region. The live halves —
   TalkBack, real rotation, real font-scale observation in the
   WebView — are ⛔ with the device checklist.
6. ✅ **The build lanes, re-verified end-to-end after all M6 changes**:
   desktop baseline `go vet ./...` clean; `go test -count=1 ./...`
   **17/17 packages ok, run twice consecutively**; svelte-check 0/0;
   vite build clean; Android library compile matrix **4/4 ABIs** with
   `-mod=vendor` (arm64 additionally links the full tree); `gomobile
   bind` → younblock.aar **25,097,538 B** (3 ABIs, `routedApps()` in
   classes.jar); clean `assembleDebug` → **app-debug.apk 35,194,757
   B**, SHA-256
   `5669efd14a0a523249e55c9f2c7ab0d6b1edb62156c60d76c3ccc38cc59302dc`,
   badging minSdk 26/target 36, `YounblockVpnService` +
   `BIND_VPN_SERVICE` in the dumped manifest, `apksigner verify
   --print-certs` OK (debug cert). The session opened after yet
   another full sandbox wipe (repo re-cloned at 4083e76, toolchain
   reinstalled per doc 06 pins, worklog rebuilt from the prompt +
   repo records; the desktop baseline re-verified green before any
   M6 code — see doc 06's M6 record).

Exit criteria verdict: **the M6 scope of doc 02's P0/P1 set is
met host-side — AND-010 (settings schema v2 + Normalize rejection +
restart survival: apps_test.go) and AND-017 (the in-app scope/privacy
disclosure: the sheet's copy is doc 07 §5 wired) now carry evidence;
AND-012's "no universal claims" is enforced by the same copy; the
desktop baseline never regressed (17/17 twice at the exit gate). The
live-WebView observation half ⛔ (the shared device checklist). M6 is
marked ✅ host-complete on the M3/M4/M5 precedent.**

### M7 — Regression, hardening, release preparation — ✅ host-complete 2026-10-17 (on-device gates ⛔: the standing emulator/device blocker)

The hardening is done and host-verified; the device-dependent halves
(battery/behavior measurement, reboot/revocation AND-028, the live
WebView/consent/notification observations) remain ⛔ — no /dev/kvm, no
device (re-verified this session: `ls /dev/kvm` → No such file or
directory). Host side of every gate:

1. ✅ **The NO_TARGET_APPS typed-error door (D81 — the decision M6
   deferred)**: the start gate (D77) now also refuses. The Android
   adapter's `RefuseStart(code)` hands a typed `engine.PlatformRefusalError`
   to the waiting connect run — or latches it when the run has not
   reached the gate yet (the start race) — and the engine's connect run
   routes a refusal through its ORDINARY failure path: phase "error"
   with the typed code (mapErrorCode whitelists NO_TARGET_APPS), the
   state emitted, the page showing "Can't Connect" plus the specific
   reason (the HttpBackend maps the code to "None of the apps on your
   list is installed…"). The phase machine stays the single truth
   (ADR-0006): no parallel Kotlin error UI, no silent unwind. The
   facade `FailStart(code)` whitelists the vocabulary (BAD_CODE
   otherwise) and reports NOT_HELD when the gate is open (an open gate
   holds no run; the shell's honest-stop owns a serving engine).
   Kotlin's establishTun calls it on the zero-installed-targets path.
   Evidence: 3 engine gate tests (refusal→error with the code,
   refusal-then-recovery, cancellation-beats-refusal), 6 adapter gate
   tests (delivery, THE re-arm race regression, latch-for-late-run,
   latch cleared by re-arm/open, open-gate-not-held), 4 facade tests
   (held-run refused with the typed code in the state JSON,
   recovery-on-next-connect, NOT_HELD, BAD_CODE).
2. ✅ **A real M5-era concurrency hole found and fixed (D82 — run
   generations)**: building the refusal tests exposed that a connect
   goroutine delayed past its own Stop/second-Start registered a FRESH
   uncancellable context at entry (and reset runAborted) — a zombie run
   that could resurrect a phase after its cancellation, the exact thing
   D77's quiet-cancellation exists to prevent. `TestEngineStartIsIdempotent`
   had been passing THROUGH the bug (its immediate double-Start passed
   only because the zombie completed to "on" after the documented
   cancel); the test now waits for on (its true intent — idempotency
   while ON), and D82 makes the cancel honest: every cancel bumps a run
   generation; a stale goroutine exits at its first instruction without
   registering anything. The desktop Start/Stop behavior is unchanged
   in practice — the registered-run cancel path (the overwhelmingly
   common case) always behaved this way; D82 closed the microscopic
   unscheduled-goroutine inconsistency.
3. ✅ **The -race hardening lane (new, all green)**: `go test -race` on
   the seven key packages — engine 10.4 s, android adapter 2.1 s,
   mobile 38.4 s, tunstack 2.3 s, proxy 11.4 s, webui 1.1 s, config
   1.0 s — zero data races, including through the D81/D82 gate and
   generation machinery and the M5 lifecycle suite.
4. ✅ **Dependency/license audit re-run (doc 07 §7)**: the AAR's actual
   dependency closure (`go list -deps ./mobile`) is gVisor's netstack
   pkg subset (Apache-2.0, LICENSE vendored), golang.org/x/{net,sync,
   sys,time,exp} (BSD-3), x/mobile (BSD-3, LICENSE vendored), and
   github.com/google/btree (Apache-2.0) — all permissive; the About
   attribution line added (D80's sheet: "gVisor network stack
   (Apache-2.0) and gomobile (BSD-3)").
5. ✅ **Permission review (doc 07 §6 vs the built APK)**: the dumped
   manifest carries EXACTLY the inventory — INTERNET,
   FOREGROUND_SERVICE, FOREGROUND_SERVICE_CONNECTED_DEVICE,
   POST_NOTIFICATIONS, the BIND_VPN_SERVICE service attribute, and
   `<queries>` for com.android.chrome + com.google.android.youtube.
   ACCESS_NETWORK_STATE is absent (the D74 discipline holds). No
   permission the code does not use.
6. ✅ **Release notes drafted**: docs/android-port/10-release-notes.md —
   the honest headline, what it does/does not, the doc 02 §4 limitation
   obligations (IPv4 DNS posture, UDP drop + QUIC gate, per-app scope,
   apply-on-next-connect, AND-028), install/sideload, the ⛔ open-gates
   section, open-source attribution. M8 finalizes against the artifact.
   Doc 02 §7's traceability rows updated for the M6/M7 evidence
   (AND-010/015/017/023).
7. ✅ **The build lanes, re-verified end-to-end after all M7 changes**:
   `go vet ./...` clean; `go test -count=1 ./...` **17/17 packages ok,
   run twice consecutively**; svelte-check 0/0; vite build clean; the
   webui dist synced; compile matrix **4/4 ABIs** with `-mod=vendor`
   (arm64 links the full tree); `gomobile bind` → younblock.aar
   **25,072,133 B** (`javap` shows `failStart(String)` bound alongside
   routedApps/setVpnReady/vpnState); clean `assembleDebug` →
   **app-debug.apk 35,298,149 B**, SHA-256
   `f27b7eef014f61eff35405f4406bfe9c33d4e34a918f9949eaad417d561c6491`,
   badging minSdk 26/target 36/3 native ABIs, manifest verified above,
   apksigner OK (debug cert).

Exit criteria verdict: **every host-runnable hardening gate green with
evidence; two real defects found and fixed at the source (the M5
spawn-race zombie D82 closed, and the test that codified it); the
battery/reboot/revocation and live-observation halves ⛔ with the shared
device checklist. M7 is marked ✅ host-complete on the M3–M6
precedent.**

### M8 — Package, verify, hand off

- ⬜ Debug + release (signed) APKs; artifact path/ID/version/ABI/
  checksum recorded; install + smoke-test; docs/notes updated;
  commit/push with the pre-existing-change rule.

## Dependency order / critical path (unchanged)

M3 → M4 → M5 → M6 → M7 → M8; M0–M7 complete (M4–M7
host-complete; their on-device gates ride with the shared device
checklist — doc 09's standing blocker).

## Change-control rules (unchanged)

Scoped commits; no Android work mixed with unrelated refactors; no
overwriting dirty work; no force-push; ADR updates when a milestone
changes a decision; limitations preserved in release notes; no task
marked complete because code was written — only when its exit
criterion is evidenced.
