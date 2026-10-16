# 01 — Current Architecture Audit

**Status:** COMPLETED (Phase 1 / M0, 2026-10-16). Every row below carries
evidence from the repository at commit `36273fb` (branch `main`, tag
`v0.7.6`, app 1.6.6). Baseline build/test results were re-run in the
audit session and are recorded in §7.

## 1. Audit objective

Identify the exact reusable networking logic, platform-specific
integration, frontend/backend interface, configuration behavior, build
tooling, and test coverage in the current Younblock implementation.
Do not infer architecture from language statistics alone.

**Result:** the objective was met. The full Connect→engine→relay→DNS/TLS
→OS-proxy→status trace is in §3; the component dispositions are in §4;
the Android compile-target evidence is in §5.

## 2. Repository state and inventory

| Item | Finding | Evidence |
|---|---|---|
| Current branch | `main` | `git branch --show-current` |
| Worktree status | clean (after local `core.fileMode=false`; see note) | `git status --porcelain` → 0 entries |
| Remote URLs | `https://***@github.com/assadigit/younblock.git` (token redacted) | `git remote -v` |
| Latest commit | `36273fb` "Add files via upload" (the Android spec zip), on top of `3ade9d6` "Release v0.7.6 record" | `git log -1 --oneline` |
| Main Go module | `github.com/assadigit/Younblock`, `go 1.26.0` directive, built with Go 1.27.2 | `go.mod` |
| Go dependencies | `github.com/wailsapp/wails/v2 v2.16.0`, `golang.org/x/net v0.61.0`, `golang.org/x/sync v0.24.0`, `golang.org/x/sys v0.49.0`, `github.com/energye/systray v1.0.3`, `github.com/go-toast/toast v0.0.0-20190211030409` — all vendored in the committed `vendor/` tree | `go.mod`, `vendor/modules.txt` |
| Frontend package manager | npm; Svelte 5 + Vite 8 + TypeScript (svelte-check); lockfile committed | `frontend/package.json`, `package-lock.json` |
| Wails version/config | CLI `v2.16.0` (must match go.mod); `wails.json` productName "Younblock", productVersion 1.6.6 | `wails.json`; worklog Task 2 |
| Existing test suites | 13 Go packages with tests (all passing — §7); svelte-check 0/0; vite production build clean; no CI (releases are cut by the agent sessions per PROGRESS.md) | `go test ./...`; §7 |
| Existing release workflow | `build.sh` (14 artifacts: 4 Windows exes, 2 cores, 2 Linux web binaries, 3 debs, 3 AppImages), `build.bat`, `build-offline-pack.sh` (offline zip), GitHub Releases with SHA-256 notes; releases v0.7.0…v0.7.6 published | `build.sh`; PROGRESS.md release records |

Note on the worktree: the sandbox rehydrated with every tracked file's
executable bit flipped (1531 files, **zero** content changes — verified
by blob-hashing every modified file against the index). Silenced with a
local `core.fileMode=false`; no file content was altered and nothing was
discarded.

## 3. Required source trace (Connect → engine → relay → DNS/TLS → OS proxy → status)

All paths verified by reading the files at `36273fb`.

1. **Entry points and process lifecycle.**
   - Desktop: `main.go` (Wails shell: creates window, binds `app.go`'s
     `App` struct, starts tray per `tray_windows.go`/`tray_other.go`).
   - Headless/web: `cmd/core/main.go` — flags `--port`, `--preset`,
     `--selftest`, `--restore-proxy`, `--web`, `--webport`, `--no-open`,
     `--version`; `--web` serves the control UI (D50). Signal-driven
     shutdown (`SIGINT`/`SIGTERM` → `eng.Stop()` → `defer eng.Shutdown()`).
   - `cmd/pacgen/main.go` — standalone PAC generator tool.

2. **UI ↔ Go binding/event mechanism.** Three adapters per screen, chosen
   at runtime by capability detection:
   - Wails: generated bindings + `younblock:status` event
     (`frontend/src/lib/backend.ts`; classic: `lib/adapters/wails.ts`).
   - HTTP: localhost JSON API + Server-Sent Events, served by
     `internal/webui/webui.go` — routes `/api/state`, `/api/start`,
     `/api/stop`, `/api/toggle`, `/api/settings` (GET/POST), `/api/appinfo`,
     `/api/test`, `/api/autofix`, plus the SSE event stream. Loopback-only,
     no auth by design (same trust boundary as the Wails window).
   - Preview: offline mock for browser/screenshots (`?fail=1` forces the
     error state). Classic additionally has `lib/adapters/android.ts` —
     a deliberate **stub that throws** ("not built yet (file 04, later
     run)") and `adapters/mock.ts`.

3. **Connection controller/state machine.** `internal/engine/engine.go`
   (700 lines) + `health.go` + `jsonapi.go` + `platform.go`:
   - Phases: `off`, `connecting`, `on`, `degraded`, `error`, `stopping`
     (`Phase` constants, engine.go:28-35).
   - `Engine.Start()` → `connectSequence()` (health.go:169): start
     listener → `platform.Attach(port, settings)` → QuickCheck
     (self-test steps 1-3, ≤8 s) → `goOn()` (phase `on`, starts stats
     loop + watchdog + netmon + prewarm) — or one automatic `healSweep`
     (tries every fragmentation method in `config.HealSweepOrder`,
     cooldown-limited) → `goDegraded()` (phase `degraded`, proxy stays
     up, watchdog keeps checking; R14/D44).
   - Total start budget: `config.StartTotalTimeout` (35 s cap) with
     `runAborted` cancel path (`cancelCurrentRun`, health.go:158).
   - Watchdog (`startWatchdog`, health.go:405): QuickCheck every
     `config.WatchdogInterval`; `wdFails ≥ WatchdogFailLimit` on a
     working link → notify + background heal sweep. `ensureProxyAttached`
     re-asserts the OS proxy if another program clobbered it (D46).
   - NetMon (`startNetMon`, health.go:498): polls `net.InterfaceAddrs`
     every 5 s; change → resolver reset + immediate watchdog tick.
   - JSON API (jsonapi.go): `GetStateJSON`, `StartJSON`, `StopJSON`,
     `ToggleJSON`, `GetSettingsJSON`, `SaveSettingsJSON`,
     `RestoreWindowsProxyJSON`, `GetAppInfoJSON`,
     `GetDiagnosticReportJSON`, `OpenLogsFolderJSON`, `QuitJSON` —
     **all string-in/string-out** (gomobile-compatible shape; see §5).
   - Events: `EventSink.OnEvent(name, json)` — one JSON contract for
     both the Wails shell and the webui SSE hub.

4. **Relay listener and accepted-connection path.**
   `internal/proxy/server.go`: raw `net.Listener` (not http.Server),
   `acceptLoop` → one goroutine per connection with `recover`
   (behavior: R2), `MaxConns` cap (default `config.MaxConcurrentConnections`)
   → 503 above it. `handle()` reads the request with
   `parseRequest` (`request.go`), then `decide()` routes to:
   - `routeConnect`: HTTP `CONNECT host:port` → resolve → dial upstream
     → `200 Connection established` → `runBidirectionalCopy` tunnel
     (`tunnel.go`), with first-TLS-record fragmentation for routed hosts
     (mode `all` or `MatchAnyDomain`).
   - `routeRedirect`: absolute-form `http://` for routed hosts → 302 to
     https; unrouted hosts → `forwardRequest` (origin-form forward
     proxy, hop-by-hop headers removed, `Connection: close`, D46).
   - `routePAC`: `/proxy.pac` (`pac.go` — PAC script with the domain
     list; this is what the OS proxy setting points at).
   - `routeHealth`: `/health` → "ok".
   - AutoPort: configured port 4525 → try 4526-4545 → any free port
     (`listenLocked`).
   - Stop: listener close → wait tunnels ≤ ctx (3 s default) →
     force-close the rest (`StopWithTimeout`).

5. **Secure DNS resolution and provider selection/fallback.**
   `internal/doh/resolver.go` (504 lines) + `cache.go`: DoH **GET**
   `?dns=<base64url no padding>` with `accept: application/dns-message`
   plus the legacy `type=A&ct=` params (replicates the old Python app's
   request shape). Servers from `settings.advanced.dohServers` tried in
   order; sticky last-good server; positive + negative caching;
   `golang.org/x/sync/singleflight`; `BlockPrivate` drops answers in
   private/reserved ranges (poisoned-answer protection); `QueryAAAA`
   off by default (IPv4-only). The DoH connection dials bootstrap IPs
   through `netx.Dialer` (never through the proxy) and its own
   ClientHello is fragmented. Error kinds: `DNS_UNREACHABLE`,
   `DNS_BLOCKED_ANSWER`, `no usable answer`, negative-cache hits.
   `SetResolver` swaps providers live while On (D44).

6. **TLS ClientHello detection and splitting.** `internal/fragment/`:
   `Plan(length, preset, rng, cfg)` is a **pure function** returning
   ordered piece lengths (presets: light 150-300 B/1 ms, balanced
   80-200 B/5 ms (default), strong 20-80 B/10 ms, legacy = the old
   Python rule ≤299 cuts); `conn.go` wraps a `net.Conn` so the first
   TLS record is written in planned pieces with pauses. Applied in the
   tunnel's client→upstream direction (`proxy/tunnel.go`) and on the
   DoH connection. Works on any ordered byte stream (no IP-packet
   awareness) — portable to any transport that preserves TCP ordering.

7. **Rules that pass unrelated traffic through unchanged.** In `youtube`
   mode (default) only `MatchAnyDomain(host, domains)` hosts are
   fragmented; every other host is relayed untouched through the same
   tunnel/forward paths (D46). `all` mode fragments everything.

8. **Connection test and what its latency value measures.**
   `internal/health/selftest.go` — four steps, each with code+detail+ms:
   `listener` (TCP connect + `/health`), `dns` (DoH lookup of
   `config.SelfTestDNSName`), `youtube` (HTTPS GET of
   `config.SelfTestURL` **through the proxy** with the DoH resolver),
   `video` (TLS handshake only to `config.SelfTestVideoHost` through the
   proxy). The UI's "response time" (`State.LatencyMs`) is the `youtube`
   step's duration — an application round trip through the whole chain
   (secure lookup + connect + request), not a network ping (D41).

9. **Automatic retries, repair behavior, timers, cancellation.** See §3.3:
   one heal sweep per Start press (cooldown-limited), watchdog light
   checks + episode-rate-limited notifications (10 min), 35 s total
   start cap, `context`-based cancellation of the start run, netmon
   network-change recovery. Bounded everywhere; no unbounded retry loops.

10. **Settings schema, defaults, persistence, validation, migrations.**
    `internal/config/settings.go`: schema `version: 1` — `mode`
    (youtube/all), `method` (fragmentation preset), `port`, `autoPort`,
    `startWithWindows`, `connectOnLaunch`, `openBrowserOnConnect`,
    `firstRunDone`, `closeNoticeShown`, `advanced{pieceSizeMin,
    pieceSizeMax, pauseMs, dohServers[], domains[], logDomains,
    idleTimeoutSec}`. `Normalize()` clamps/validates everything;
    `Save/Load` to `settings.json` in the platform `ConfigDir()`. No
    secrets. Defaults in `defaults.go` (AppName, AppVersion "1.6.6",
    ListenHost 127.0.0.1, ListenPort 4525, timeouts, DoH defaults,
    domain list, heal-sweep order).

11. **Windows proxy apply/restore/monitoring.**
    `internal/platform/windows/windows.go` (build tag `windows`) +
    `sysproxy_logic.go` (platform-neutral logic, unit-tested) +
    `autostart_logic.go`: registry `HKCU\…\Internet Settings`
    (ProxyEnable/ProxyServer/ProxyOverride/AutoConfigURL) + `wininet`
    InternetSetOption refresh (39/37); state captured before change and
    restored verbatim on Detach; crash recovery via backup +
    `--restore-proxy`; toast notifications (`go-toast`); autostart via
    Run key. Called by `engine.ensureProxyAttached` every watchdog tick
    (anti-clobber, D46).

12. **Linux proxy apply/restore/monitoring.**
    `internal/platform/linux/linux.go` (build tag `linux`): `gsettings`
    (user-space glib 2.84 + compiled proxy schema; `get`/`set` literal
    round-trip, no GVariant parsing), memory-only backup (D50),
    `notify-send` notices, `~/.config/autostart` .desktop for
    start-with-OS. Cross-process dconf persistence could not be
    demonstrated in the sandbox — flagged as Ali's manual check (the
    code comments record exactly what was and wasn't verified).

13. **Browser-specific mode and browser launching.**
    `browser_session.go` + `browser_windows.go` / `browser_other.go` +
    `internal/browser/browser.go`: the optional dedicated Chrome session
    (D40) — Windows-only launch with proxy flags + separate profile;
    no-op elsewhere. Capability-gated via
    `Capabilities.BrowserSession` (web edition reports false, D50).

14. **Logging, diagnostics, version reporting, tray/desktop.**
    `internal/logx` (ring buffer + rotating file, hostnames off by
    default — R21); `GetDiagnosticReport` (self-test summary, settings
    snapshot, log tail, redacted); `GetAppInfo` (backend-driven since
    1.6.6 — the About version cannot go stale); tray icons
    `tray_windows.go` (energye/systray) / no-op `tray_other.go`;
    window client size `clientsize_windows.go` 360×628 /
    `clientsize_other.go`.

15. **Build scripts and runtime dependencies.** `build.sh` (npm ci →
    vite build → copy to `internal/webui/dist` → go build ×6 +
    `wails build -m` ×4 + debs + AppImages + flatpak-when-tools-exist);
    **every wails invocation must pass `-m`** (SkipModTidy; D68 — plain
    calls hit proxy.golang.org and fail on filtered networks);
    `build.bat` (Windows-native mirror, ASCII+CRLF enforced by
    .gitattributes); `build-offline-pack.sh` (git archive of the tag +
    vendored deps + Windows node_modules + Go/Node toolchains + wails.exe;
    refuses to zip without the WebView2 bootstrapper since v0.7.6/D70);
    `setup.ps1`/`setup-*.bat` installers; `packaging/` (deb, AppImage,
    flatpak, native-stack fetcher). The dev sandbox has no webkit2gtk →
    GUI `wails dev` is impossible; E2E runs through the vite preview
    adapter at exactly 360×628.

## 4. Component inventory (with Android disposition)

| Component | Actual path/package | Responsibility | Platform/API deps | Tests | Android disposition | Evidence |
|---|---|---|---|---|---|---|
| Desktop entry | `main.go`, `app.go`, `ui_bridge.go`, `engine_adapter.go`, `tray_*.go`, `browser_*.go`, `clientsize_*.go` | Wails window, tray, browser session, size | Wails v2, systray, Windows APIs | `ui_bridge_test.go` | **Replace** — Android shell (Kotlin) owns UI/service; not reused | §3.1-3.2 |
| Headless/web entry | `cmd/core/main.go` | Engine process + web control UI | none (pure Go + embed) | via engine/webui tests | **Reuse as reference** for the in-app loopback server variant (ADR-0005) | §3.1 |
| Engine (state machine) | `internal/engine` | phases, start/stop, heal, watchdog, netmon, JSON API | only via `Platform` interface | `engine_test.go`, `health_test.go`, `privacy_test.go` (all green) | **Reuse unchanged** — designed for it ("so the same engine can later run on Android", platform.go:1-4) | §3.3 |
| Platform abstraction | `internal/engine/platform.go`, `internal/platform/{windows,linux,stub}` + `select_*.go` | the only door to OS behavior | per-impl | `linux_test.go`, `sysproxy_logic_test.go` | **Add `internal/platform/android` adapter** (Attach/Detach no-ops — the VPN service owns routing; DialControl = protect hook; Notify = callback; ConfigDir = app dir). `select_other.go` already falls back to stub for `GOOS=android` | §3.3, §5 |
| HTTP relay | `internal/proxy` | CONNECT tunnels, forward proxy, PAC, health endpoint | none | `behavior/integration/soak/pac/request` tests | **Reuse unchanged** as the loopback front door; Android TUN flows terminate into it (or a new SOCKS5 door — ADR-0008) | §3.4 |
| Secure DNS | `internal/doh` | DoH resolver, cache, fallback | none; `netx` Control hook present | `resolver_test.go` | **Reuse unchanged**; the TUN stack's DNS interceptor feeds it | §3.5 |
| TLS splitting | `internal/fragment` | pure piece-planning + conn wrapper | none | `plan_test.go`, `conn_test.go` | **Reuse unchanged** — stream-level, transport-agnostic | §3.6 |
| Health/self-test | `internal/health` | 4-step connection test | none | `selftest_test.go` | **Reuse unchanged** — provides the honest "Connected" gate (ADR-0006) | §3.8 |
| Outbound dialer | `internal/netx` | one outbound door, per-IP/overall timeouts, Control hook | `syscall.RawConn` hook only | `dialer_test.go` | **Reuse** — the `Control` hook exists **specifically** for `VpnService.protect(fd)` (dialer.go:16-18) | §3, §5 |
| Settings | `internal/config` | schema, defaults, normalize, save/load | filesystem only | `settings_test.go` | **Reuse**; add an Android-only `apps` field (roadmap M6) | §3.10 |
| Web control UI | `internal/webui` | loopback JSON API + SSE | net/http + embed | `webui_test.go` | **Reuse** (candidate primary transport for the WebView UI — ADR-0005) | §3.2 |
| Frontend (pro) | `frontend/src` (App.svelte + lib/) | redesigned connect screen | 3 adapters: wails/http/preview | svelte-check; E2E via preview adapter | **Reuse via WebView** (Ali's locked "one engine, one UI"; responsive ladder verified 300×620 in Phase 20) | §3.2, §6 |
| Frontend (classic) | `frontend/src/classic` | v0.6.0 screen, frozen (D66) | 4 adapters incl. `android.ts` stub | svelte-check | **Reuse option** — both screens ship in one bundle, `?ui=classic` switches (D50/D66); the stub marks the old plan | §3.2 |
| Logs/diagnostics | `internal/logx`, `internal/stats` | ring/file logs, counters | filesystem | both tested | **Reuse unchanged** | §3.14 |
| Build/release | `build.sh`, `build.bat`, `build-offline-pack.sh`, `packaging/` | 14 artifacts + offline pack | Go, Node, wails CLI, dpkg/ AppImage tooling | release records (v0.7.0-0.7.6) | **Add an Android lane** without touching desktop lanes (ADR-0001) | §3.15 |

## 5. Go portability audit (evidence, not assumptions)

Commands run at `36273fb` with Go 1.27.2, `CGO_ENABLED=0`,
`-mod=vendor`:

```text
GOOS=android GOARCH=arm64 go build ./...   → exit 0 (the WHOLE module,
                                            including the wails desktop
                                            entry, compiles for Android)
GOOS=android GOARCH=arm   go build ./...   → packages compile; final
GOOS=android GOARCH=386   go build ./...     executable LINK requires
GOOS=android GOARCH=amd64 go build ./...     cgo (NDK) — irrelevant for
                                            the AAR route, which is cgo
                                            by definition
GOOS=android GOARCH=arm64 go vet ./internal/... ./cmd/... → exit 0
```

Findings:

- **OS-specific imports are fully isolated** behind build tags:
  `internal/platform/windows` (`windows` tag; registry/wininet/toast),
  `internal/platform/linux` (`linux` tag; execs `gsettings`,
  `notify-send`), desktop shell files in the root package
  (`tray_windows.go` `windows` tag, `browser_windows.go`, …).
  `GOOS=android` selects `select_other.go` → the call-recording `stub`
  platform — exactly the seam an `internal/platform/android` adapter
  fills (the stub is even used by the engine tests as a call recorder).
- **`syscall`/`unsafe` usage**: confined to the Windows platform
  adapter, the wails dependency (its own `!windows` stubs compile clean
  for android), and `netx`'s `syscall.RawConn` Control hook (portable
  pattern — Android needs it). No cgo anywhere in the engine tree.
- **Dependencies with Android/ARM64 limitations**: none found — the
  module's direct deps are wails (desktop shell only), x/net (dns
  message), x/sync (singleflight), x/sys (windows registry), systray +
  toast (Windows shell only). `golang.org/x/mobile` (BSD-3-Clause,
  actively maintained — pushed 2026-10-11) is NOT yet a dependency; it
  would be added for `gomobile bind` only.
- **Global state**: the engine is an instance (`engine.New(opts)`), not
  a singleton; `cmd/core` creates one per process. gomobile exposes
  instances fine. The desktop root package (`main`) holds globals
  (tray, browser session) but is not reused on Android.
- **Engine API surface for a platform-neutral binding**: already exists
  — `internal/engine/jsonapi.go` is string-in/string-out (a constraint
  gomobile imposes: no slices-of-structs/maps across the boundary; the
  repo anticipated this — root `04-android-spec.md` §4 documents it).
- **Licensing**: all current deps permissive (BSD/Apache-2.0/MIT- family);
  the Android candidates audited: gVisor Apache-2.0, xjasonlyu/tun2socks
  MIT (5.5k stars, pushed 2026-10-11), hev-socks5-tunnel MIT (pushed
  2026-10-15), golang/mobile BSD-3-Clause, Jigsaw Intra Apache-2.0
  (reference implementation only). No GPL contamination risk on the
  primary path.
- **Cross-compilation is not runtime proof**: compiling for
  `GOOS=android` does not exercise Android's networking stack, the
  ART/JNI boundary, or VpnService behavior. The M3/M4 prototype gates
  (05-implementation-roadmap) exist for exactly that; this sandbox has
  **no Android SDK, no NDK, and a JRE without javac** — the prototype
  cannot run here and is the first blocked item for Phase 2 (recorded
  in 09-agent-handoff).

## 6. Frontend and desktop-runtime audit

- The frontend builds as **pure static assets** (`vite build` →
  `frontend/dist`, copied into `internal/webui/dist` and embedded).
  Wails bindings are consumed via `frontend/wailsjs/` (git-ignored,
  generated) but only through the adapter layer; the http adapter
  talks plain JSON+SSE and is proven in production (the Linux
  single-executable edition since v0.7.2).
- Wails-only API touchpoints in the UI: window close request
  (`onCloseRequested`), hide-window, open-logs-folder, quit, tray-ish
  behaviors — all already abstracted behind the `Backend` interface
  and capability-gated in the UI.
- Mobile readiness: the pro screen's responsive ladder was verified in
  Phase 20 (2026-10-16) down to 300×620 with no scroll and correct root
  font scaling; the design was built mobile-first (360 px stage). The
  classic screen is fluid by design (root spec file 02 §3).
- Reduced-motion support exists (`prefers-reduced-motion` verified in
  Phase 20) — relevant to Android's "remove animations" setting.
- Conclusion: **the Svelte UI is genuinely portable to an Android
  WebView**; the missing piece is only a transport (loopback webui —
  zero frontend changes — or a JS-bridge adapter; ADR-0005).

## 7. Desktop baseline (commands and actual results, 2026-10-16)

Environment: sandbox (Debian 13 user-space, Go 1.27.2 at `~/sdk/go`,
Node v24.21.0, wails CLI v2.16.0, `core.fileMode=false` local).

| Command | Result |
|---|---|
| `go vet ./...` | clean (exit 0) |
| `go test -count=1 ./...` | 13/13 packages `ok` (engine 8.0 s, proxy 9.5 s; `cmd/*` and `internal/platform{,stub}` have no test files) |
| `cd frontend && svelte-check` | **0 errors, 0 warnings** |
| `cd frontend && vite build` | clean, ~1 s |
| `wails dev` / GUI | **not possible in this sandbox** (no webkit2gtk — standing limitation since v0.7.0 sessions); E2E uses the vite preview adapter |
| Full artifact build (`./build.sh`) | last executed 2026-10-16 at tag v0.7.6: 14/14 artifacts + offline pack, release published and round-trip verified (16/16 SHA-256) — see PROGRESS.md "Release v0.7.6" |

Desktop regression comparison baseline for Android work = this table,
re-run on every milestone that touches shared code (06-build-and-test
§3).

## 8. Findings summary

- **Reusable unchanged (verified):** `internal/engine`, `internal/proxy`,
  `internal/doh`, `internal/fragment`, `internal/health`, `internal/netx`,
  `internal/config`, `internal/logx`, `internal/stats`, `internal/webui`,
  both Svelte screens.
- **Platform adapters to add:** `internal/platform/android` (small,
  interface already exists; stub records calls today); a TUN→TCP
  userspace bridge (external dependency decision — ADR-0008); Android
  Kotlin shell (UI WebView + VpnService + foreground service).
- **Components requiring rewrite:** none. The desktop shell (Wails
  window/tray/browser-session) is *replaced*, not rewritten — Android
  equivalents are native (WebView, notification, per-app routing).
- **Critical dependencies and licensing:** current tree all permissive;
  Android additions all permissive (§5); no GPL on the primary path.
- **Existing test gaps:** no Android-runtime tests possible in this
  sandbox (no SDK/emulator/device) — device coverage is a documented
  Phase-2 blocker, not a silent gap; desktop suites are green and
  broad (behavior, integration, soak, fuzz-style edge tests).
- **Desktop regression risks:** low — Android work adds packages and
  build lanes; shared-code edits must keep the §7 baseline green
  (enforced per milestone).
- **Evidence references:** every claim above cites the file/section it
  came from; baseline commands are reproducible from a clean checkout
  (README + build.sh).

## 9. Audit exit criteria — MET

Every major runtime component has an evidence-backed disposition (§4),
the real connection flow is traced end-to-end with symbols (§3), and
the next agent can locate every relevant file without guessing.
