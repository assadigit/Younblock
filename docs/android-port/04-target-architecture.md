# 04 — Target Android Architecture

**Status:** FINALIZED (Phase 1 / M2, 2026-10-16). Module and package
names below are the real ones from the audited repository (doc 01); the
Android-side names follow the repo's own prior plan (root
`04-android-spec.md`) reconciled with this pack's decisions (doc 08).
Nothing in this document is a placeholder.

## 1. Design goals

- Reuse portable and tested Younblock Go logic where feasible —
  achieved: the ten `internal/*` packages are reused unchanged.
- Isolate Android APIs from desktop-only functionality — the Android
  code lives in new packages only (`internal/platform/android`,
  `internal/proxy/socks5.go`, `mobile/`, `android/`), behind the
  existing interfaces.
- Keep the existing Windows/Linux build paths intact — no desktop file
  is edited to add Android behavior (ADR-0001).
- The Android layer owns permissions, foreground-service lifecycle,
  user-visible state, and settings UI semantics.
- The engine reports real readiness and failures to the UI — the
  existing phase machine + self-test gates remain the single truth
  (ADR-0006).
- Routing and protocol mechanics stay testable independently of UI —
  all Go-side additions get host-runnable unit tests like the rest of
  the tree.
- No traffic-coverage claims beyond the per-app allow-list plus tested
  behavior (ADR-0007/0009).

## 2. Logical components (real names)

1. **Android UI** — the existing Svelte bundle (pro screen default;
  classic available via `?ui=classic`) rendered in a WebView, served by
  the in-process webui server. No Wails code.
2. **Android application/state layer** — `android/app` Kotlin:
  MainActivity + WebView + first-run consent + settings surface; UI
  state is a projection of engine events (SSE), never independent
  truth.
3. **Android network service** — `YounblockVpnService : VpnService`
  (foreground, `connectedDevice` type on API 34+): builds the TUN,
  owns the VPN permission flow, the notification, `onRevoke`/`onDestroy`
  → engine Stop and honest `off`.
4. **Routing adapter** — per-app allow-list via
  `VpnService.Builder.addAllowedApplication` (default
  `com.android.chrome` + `com.google.android.youtube`; empty list →
  engine error `NO_TARGET_APPS`), TUN address/route/DNS configuration.
5. **Userspace packet/stream adapter** — the tun2socks engine package
  (primary) or gVisor netstack direct (alternative), turning TUN IP
  packets into TCP flows; UDP/53 to the virtual resolver → DoH; other
  UDP dropped (ADR-0009).
6. **SOCKS5 front door** — new `internal/proxy/socks5.go`: CONNECT-only
  SOCKS5 on loopback, address types IPv4/IPv6/domain, no auth,
  feeding the same `tunnel.go` path as the HTTP door. Small, testable,
  stack-agnostic (works with either stack choice and with external
  SOCKS-speaking clients).
7. **Go engine facade** — new `mobile/` package exposing
  `mobile.Engine` over gomobile: `NewEngine(configDir, protector,
  sink)` + the string-in/string-out JSON methods of
  `internal/engine/jsonapi.go` verbatim (`GetState`, `Start`, `Stop`,
  `Toggle`, `GetSettings`, `SaveSettings`, `RunConnectionTest`,
  `AutoFix`, `GetDiagnosticReport`, `GetAppInfo`).
8. **Reusable Go engine** — `internal/{engine,proxy,doh,fragment,
  health,netx,config,logx,stats,webui}` unchanged (audit doc 01 §4).
9. **Configuration repository** — `internal/config` settings.json v1 in
  the app's private storage (`ConfigDir()`), plus the Android-only
  `apps` list (validated package names; default Chrome + YouTube).
10. **Diagnostics/logging** — `internal/logx` (hostnames off by
  default), `GetDiagnosticReport` redaction unchanged; Kotlin-side
  logs stay out of release builds by default.

## 3. Module boundaries (actual layout)

```text
android/                  # NEW: Gradle project (Kotlin shell)
  app/                    #   MainActivity + WebView + consent + settings UI glue
  service/                #   YounblockVpnService, notification, tile
mobile/                   # NEW: gomobile facade over the engine (small)
internal/platform/android/ # NEW: engine.Platform implementation (protect hook,
                          #      no-op Attach/Detach, Notify callback, ConfigDir)
internal/proxy/socks5.go  # NEW: SOCKS5 front door (+ socks5_test.go)
internal/...              # UNCHANGED: engine, proxy, doh, fragment, health,
                          #      netx, config, logx, stats, webui
frontend/                 # UNCHANGED: both screens; Android uses the http
                          #      adapter (SSE) exactly as the Linux web edition
build-android.sh / .bat   # NEW: UI build → assets, gomobile bind, gradle
build.sh / build.bat      # UNCHANGED: desktop lanes
```

No existing package moves. The only edits inside existing packages are
additive (a new file in `internal/proxy`, an optional `apps` field in
`internal/config` guarded by validation), each with desktop regression
gates (doc 06).

## 4. Engine API contract (binding-level)

gomobile constraint honored by design: **string-in/string-out**. The
facade forwards to `internal/engine/jsonapi.go` verbatim, so the JSON
contracts are byte-identical across Wails / webui / Android.

```text
mobile.NewEngine(configDir string, protector SocketProtector, sink EventSink) (*Engine, error)
Engine.GetState() string            // {"phase":"on",...} — engine.State JSON
Engine.Start() string               // StartJSON
Engine.Stop() string                // StopJSON
Engine.Toggle() string
Engine.GetSettings() string
Engine.SaveSettings(payload string) string
Engine.RunConnectionTest() string
Engine.AutoFix() string
Engine.GetDiagnosticReport() string
Engine.GetAppInfo() string
Engine.Shutdown()                   // idempotent, bounds resource teardown
SocketProtector.Protect(fd int32) bool   // → VpnService.protect in Kotlin
EventSink.OnEvent(name string, json string) // → SSE/webview push in Kotlin
```

Contract behaviors (inherited from the audited engine, restated for the
binding): Start/Stop idempotent-or-rejecting (engine guards phases);
Stop cancels the start run and force-closes tunnels within the 3 s
budget; cancellation propagates via contexts (verified in engine
tests); no callbacks after Shutdown (engine's sink contract); settings
validated by `config.Normalize` before apply; error categories are the
existing stable codes (`PORT_BUSY`, `PROXY_SETTING_FAILED`,
`SELFTEST_FAILED`, `DNS_UNREACHABLE`, …) plus Android-only
`NO_TARGET_APPS`; the bridge documents threading (gomobile calls run on
dedicated goroutines; events arrive on the sink goroutine — Kotlin
marshals to main/services).

## 5. State ownership

The engine remains the single source of truth (ADR-0006). Android adds
one product state before the engine starts:

```text
Disconnected → AwaitingConsent (VpnService.prepare needed) → Starting
  Starting = engine "connecting" (listener → attach no-op → checks)
  Connected  ⇔ engine "on"          (self-test steps 1-3 passed)
  Degraded   ⇔ engine "degraded"    (honest partial state, watchdog on)
  Recovering ⇔ engine heal/watchdog activity (surfaces as notices)
  Error      ⇔ engine "error"
  Stopping   ⇔ engine "stopping" → Disconnected
+ service-death reconciliation: onRevoke/onDestroy → engine.Stop(),
  UI re-projects from GetState on next resume (never a stale
  "Connected").
```

The engine's existing phase machine maps 1:1; no new Go states are
invented. `AwaitingConsent` is Kotlin-only and precedes `Start()`.

## 6. Connect sequence (final ordering)

```text
User taps Connect
→ Kotlin: first-run explanation (once) → VpnService.prepare() → system consent
→ start YounblockVpnService (foreground, notification)
→ Service: build TUN (allow-list apps, virtual DNS, routes)
→ Service: mobile.NewEngine(...) (once) → Engine.Start()
→ engine.connectSequence(): listener → platform.Attach (no-op on
   Android) → QuickCheck (listener/dns/youtube through the live path)
→ on: notification "Younblock is on", UI shows Connected (SSE event)
→ any failure: engine phase error/degraded (typed code) → service
   tears the TUN down on hard error, keeps honest state, UI shows the
   actionable error. Permission revoked mid-run: onRevoke → Stop → off.
```

Ordering rationale: the engine starts its listener and checks before
the UI can claim anything; the TUN exists before flows can arrive; the
rollback on failure is "TUN down + engine stopped + typed error" —
traffic reverts to direct (the platform's safe default).

## 7. Traffic flow (final)

```text
allowed app (Chrome / YouTube app)
→ TUN interface (VpnService, per-app allow-list)
→ userspace stack (tun2socks engine / gvisor netstack)
    TCP flow ──► SOCKS5 door (127.0.0.1, internal/proxy/socks5.go)
                   └► same tunnel path as desktop: DoH-resolve,
                      dial via netx.Dialer (protect()ed), fragment
                      first TLS record for routed domains, copy both ways
    UDP:53 ────► virtual DNS → engine DoH resolver (A answered;
                   AAAA empty → apps stay IPv4)
    other UDP ──► dropped (routed apps fall back to TCP — M4 device gate)
engine upstream sockets → netx.Dialer.Control → VpnService.protect(fd)
                            (never re-enter the TUN)
UI ↔ in-process webui (loopback JSON+SSE) ↔ WebView (http adapter)
```

Scope statement for all user-facing text (ADR-0007/0009): only the
allow-listed apps' TCP traffic is processed (DNS through the configured
DoH providers); other apps and protocols are untouched or fail closed
as documented; nothing is tunneled to any Younblock-operated server.

## 8. Failure and rollback design

Startup as a transaction, mapped to the audited engine behavior:

1. Validate settings (`config.Normalize`) and the allow-list
   (`NO_TARGET_APPS` when empty).
2. Consent (Kotlin) — denial leaves Disconnected with an explanation.
3. Engine instance + listener (port busy → AutoPort ladder — verified).
4. TUN establish; failure → stop engine, typed error, TUN down.
5. QuickCheck (existing self-test steps) — pass → on; fail → one heal
   sweep → on or degraded (all existing, tested behavior).
6. Every failure path ends with: notification honest, UI state from
   the engine, no routes leaked (TUN teardown is atomic from the
   service side), upstream sockets closed by engine Stop (3 s budget).

Disconnect: engine.Stop (cancels start run, closes tunnels) → service
stops the TUN → notification removed → Disconnected. onRevoke equals
Disconnect plus re-permission on next connect.

## 9. Data and configuration

- `settings.json` v1 (existing schema + Android-only `apps` list)
  in app-private storage via `ConfigDir()`.
- No desktop paths, no OS proxy state, no secrets (schema verified —
  audit §3.10). Migration = existing `settingsVersion` mechanism
  (bump to 2 when adding `apps`; `Normalize` fills defaults).
- Diagnostics redaction unchanged (R21: hostnames off by default).

## 10. Desktop separation

Verified separation mechanism: build tags already isolate the desktop
platform adapters; the Android additions create new packages and never
edit desktop-only files. The `apps` settings field is additive JSON —
desktop `Normalize` ignores unknown-but-valid fields per existing
load behavior (test in M3). Every shared-tree change runs the desktop
baseline (doc 06 §A) before commit — the rule that kept v0.7.x green.

## 11. Architecture exit criteria — MET

Routing ingress (VpnService per-app), packet-to-relay mapping (stack →
SOCKS5 door → tunnel), TCP/UDP/IPv4/IPv6 policy (ADR-0009/0010), Go
binding (gomobile AAR, ADR-0004), UI approach (WebView + loopback
webui, ADR-0005), API levels and ABIs (doc 06 table), lifecycle and
notification (connectedDevice FGS, doc 07), settings schema (v1+apps),
error contract (existing codes + NO_TARGET_APPS), rollback logic (§8).
Each decision is recorded in `08-architecture-decisions.md`.
