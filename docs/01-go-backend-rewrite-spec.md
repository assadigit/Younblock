# 01 - Go Engine and Windows Backend Spec (v2)

**Audience:** a coding agent. **Owner:** Ali, who cannot read or patch code, so every deliverable must be complete and verified by you.
**Read first:** `00-start-here.md` (locked decisions). **Then:** this file, `02-ui-design-spec.md`, `03-reliability-suggestions.md`. File `04-android-spec.md` is a **later run**; do not build it now, but obey the "Android-ready" rules in section 3 so it stays cheap.
Rules in file 03 marked P0 and P1 are part of this task.

## 0. Plain-language summary (for Ali)

Your Python script is a small relay on your PC. Chrome sends its traffic to it, it looks up the real address of the site through a secure DNS service, and when it opens the connection it sends the very first message in small pieces so the filter doesn't recognise the site name. The Go version does the same job as one small `.exe`, with no Python inside, and adds what a one-button app needs: it switches Windows' proxy setting for you, lives in the system tray, tests itself, and repairs itself when YouTube stops working.

## 1. Goal and non-goals

**Goal:** a behavior-compatible Go rewrite of `backend.py` (`legacy/backend.py`), packaged as a Windows app named **Younblock**, built so the same engine can later run on Android (file 04).

**Non-goals:** not a VPN; no encryption layer of our own; no UDP/QUIC handling on desktop; no accounts, telemetry or background updates; Windows is the only supported desktop target (code must still compile on Linux and macOS so you can test there).

## 2. Behavior of the Python original that must be preserved

| Original behavior | Keep as |
|---|---|
| Local HTTP proxy on port 4525 | Same default port, loopback only (`127.0.0.1`) |
| Accepts `CONNECT host:port`, replies `HTTP/1.1 200 Connection established` | Same |
| Plain `GET`/`POST` with an `http://` URL gets `302` to the `https://` version | Same, extended to `HEAD`; any other method gets `405` |
| Names resolved through DNS-over-HTTPS at `https://sky.rethinkdns.com/?dns=<base64url, no padding>`; the DoH request itself also has a fragmented first message | Same, but dialed directly (no loop through our own proxy) |
| Hard-coded bootstrap IP `172.67.162.27` for `sky.rethinkdns.com` | First entry of a bootstrap list in config |
| Only the **first** client message after CONNECT (the TLS ClientHello) is split into pieces with a short sleep; everything after passes through untouched | Same idea, better splitting rules (section 5) |
| A-records only, IPv4 only | Same default; AAAA is an off-by-default option |
| IP literals in CONNECT skip DNS | Same |

Replicate the DoH request exactly as the Python sends it in v1 (GET, `?dns=` value, `accept: application/dns-message`, plus the extra `type=A&ct=application/dns-message` query parameters). It is known to work with this server; only simplify it after testing.

**The old app's UI file is not available to you.** The old app was Chrome-only ("Compatible only with Google Chrome"), so it set Chrome up in some way that `backend.py` does not show. If `legacy/` contains more than `backend.py`, read it first and record in `DECISIONS.md` how the old app made Chrome use the proxy, then compare with section 8. If `legacy/` has only `backend.py`, use section 8 and say so in your report.

## 3. Stack, dependencies, layout, Android-ready rules

- **Go:** latest stable (at least 1.22). Run `go version` first. **Verify every package API against the installed version; never invent functions or flags.**
- **Allowed dependencies:** standard library, `golang.org/x/net/dns/dnsmessage`, `golang.org/x/sys` (registry, mutex), `golang.org/x/sync/singleflight`, Wails v2 (file 02), and one tray library (section 9). Anything else needs a one-line justification in `DECISIONS.md`.
- Module path: `github.com/<github-username>/Younblock`. App name constant `Younblock`; app data folder `%APPDATA%\Younblock`; executables `younblock.exe` (app) and `younblock-core.exe` (headless).

```
Younblock/
  cmd/core/main.go          headless CLI: --port --preset --selftest --restore-proxy --version
  main.go, app.go           Wails entry and thin UI adapter (phase 4)
  internal/
    engine/    engine.go platform.go jsonapi.go     the one brain: state machine, start/stop, heal, events
    config/    defaults.go settings.go
    netx/      dialer.go                            ALL outbound connections go through here
    fragment/  plan.go conn.go sni.go(optional)
    doh/       resolver.go cache.go
    proxy/     server.go request.go tunnel.go pac.go
    health/    selftest.go watchdog.go heal.go
    stats/     stats.go
    logx/      ring.go file.go
    platform/windows/  sysproxy.go autostart.go tray.go toast.go   (//go:build windows)
    platform/stub/     stub.go                                      (used on Linux and in tests)
  assets/                   logo files from Ali, tray icons
  scripts/                  check-platform-neutral (see below), soak test
  build.bat  README.md
```

**Android-ready rules (required now, cheap now, expensive to retrofit later):**
1. **Platform-neutral core.** `engine`, `config`, `netx`, `fragment`, `doh`, `proxy`, `health`, `stats`, `logx` must not import `golang.org/x/sys/windows`, `syscall` Windows APIs, Wails, or any tray or toast library. Only `internal/platform/windows`, `main.go` and `app.go` may.
2. **One outbound door.** Every outgoing socket (upstream connections and DoH) is created by `netx.Dialer`, which accepts an optional `Control func(network, address string, c syscall.RawConn) error`. Desktop leaves it nil. Android will use it to exempt our own sockets from its tunnel.
3. **Platform interface.** The engine talks to the OS only through this interface (agent finalizes exact signatures and verifies it compiles on all targets):

```go
type Platform interface {
    Name() string                       // "windows", "android", "stub"
    Capabilities() Capabilities         // tray, startWithOS, systemProxy, perAppRouting (bools)
    DialControl() func(network, address string, c syscall.RawConn) error // nil on desktop
    Attach(port int, s Settings) error  // windows: apply proxy/PAC; android: no-op
    Detach() error                      // windows: restore proxy
    SetAutostart(enabled bool) error
    Notify(kind NoticeKind, text string) // toast on windows; notification on android
    ConfigDir() string
}
```
4. **One JSON contract.** The UI-facing API (section 10) is implemented once in `engine/jsonapi.go` with JSON strings in and out (gomobile can only pass simple types). Wails `app.go` is a thin adapter over it. Events go through one `EventSink` interface: `OnEvent(name string, json string)`.
5. **Check:** `scripts/check-platform-neutral` (a script or test) fails if a neutral package imports anything from the forbidden list, and `GOOS=linux GOARCH=arm64 go build ./internal/engine/... ./internal/config/... ./internal/netx/... ./internal/fragment/... ./internal/doh/... ./internal/proxy/... ./internal/health/... ./internal/stats/... ./internal/logx/...` passes.

## 4. Configuration block (defaults)

`internal/config/defaults.go` must begin with a clearly marked block:

```go
// ====================== CONFIGURATION (edit here) ======================
// ... every constant below, each with a one-line plain-English comment ...
// =======================================================================
```

| Setting | Default | Notes |
|---|---|---|
| AppName | `Younblock` | one place to rename |
| ListenHost / ListenPort | `127.0.0.1` / `4525` | never `0.0.0.0` |
| AutoPort | true | if busy try 4526-4545, then any free port; actual port reported in state |
| Mode | `youtube` | `youtube` (PAC for YouTube hosts only), `all`, `manual`; changing it is in Advanced only |
| Method | `auto` | `auto` starts with `balanced` and lets auto-heal choose; or a fixed preset (section 5); Advanced only |
| DoH servers (in order) | RethinkDNS `https://sky.rethinkdns.com/?dns={dns}` bootstrap `172.67.162.27`; Cloudflare `https://cloudflare-dns.com/dns-query?dns={dns}` bootstrap `1.1.1.1`,`1.0.0.1`; Google `https://dns.google/dns-query?dns={dns}` bootstrap `8.8.8.8`,`8.8.4.4` | later ones tried only if the earlier fails; editable in Advanced |
| DoH timeout per server | 3 s | |
| DNS cache TTL clamp | min 120 s, max 3600 s | serve stale up to 6 h on failure; negative cache 15 s |
| Dial timeout per IP / overall | 5 s / 12 s | try every returned address in order |
| Request-read timeout | 10 s | max 16 KB of request headers |
| First TLS record read timeout | 10 s | after the 200 reply |
| Idle timeout | 300 s | refreshed on every read or write |
| Max concurrent connections | 1024 | above that: `503` and close |
| YouTube domain list | `youtube.com`, `youtu.be`, `googlevideo.com`, `ytimg.com`, `ggpht.com`, `youtube-nocookie.com`, `youtubei.googleapis.com` | match the domain and subdomains; editable |
| Block private targets | true | reject DNS answers in loopback, private, link-local, multicast, unspecified ranges. Test-only override `AllowPrivateTargets` |
| Start with Windows / Connect on launch | false / false | the UI asks once on first run (file 02) |
| Self-test URLs | `https://www.youtube.com/generate_204`, fallback `https://www.youtube.com/robots.txt`; TLS-handshake-only check to `redirector.googlevideo.com:443` | confirm each exists when you test; swap if not |
| Heal limits | one automatic sweep per Start press and per watchdog failure; cooldown 10 min between automatic sweeps | prevents hammering YouTube |
| Logging | in-memory ring of 500 lines + file `%APPDATA%\Younblock\logs\app.log` (2 MB x 3); hostnames not logged unless `logDomains` is on | |

Settings are saved to `%APPDATA%\Younblock\settings.json` (via `os.UserConfigDir`): `version` field, validation with clamping, unknown fields ignored, atomic write (temp file then rename), corrupt file renamed `settings.json.bad` and defaults load. Fields: `mode`, `method`, `port`, `autoPort`, `startWithWindows`, `connectOnLaunch`, `firstRunDone`, `closeNoticeShown`, and an `advanced` object (piece size, pause, DoH list, domain list, `logDomains`, `idleTimeoutSec`).

## 5. Fragmentation module (`internal/fragment`)

**What to fragment:** only the first client-to-server TLS record on a tunnel. Read the 5-byte TLS header (`0x16`, `0x03`, version, 2-byte length), then read exactly that many bytes (cap 16 KB + 5) so a ClientHello that arrived in several TCP reads is handled as one unit. If the first byte is not `0x16`, pass everything through untouched.

**How:** write the record as several separate `Write` calls with a pause between them. Go enables `TCP_NODELAY` by default; **never turn it off**, or the OS may merge the pieces again.

**Presets** (starting points, not promises):

| Preset | Mode | Piece size | Pause | Pieces for a ~520-byte hello |
|---|---|---|---|---|
| `light` | chunks | 150-300 B random | 1 ms | 2-4 |
| `balanced` (default first try) | chunks | 80-200 B random | 5 ms | 3-7 |
| `strong` | chunks | 20-80 B random | 10 ms | 7-26 |
| `legacy` | random-points | 299 random split points | 1 ms | up to 299 (exact old behavior) |

Planner `Plan(length, preset) []int` is a pure function:
- Pieces are in order, lengths sum exactly to `length`, none empty.
- For `length >= 2` there are at least 2 pieces; a piece never exceeds the preset maximum except the last.
- `legacy` reproduces the Python rule: if `length < 300` send whole, else `min(299, length-2)` sorted random cut points in `1..length-2`.
- Randomness comes from an injectable source so tests are deterministic.

**Windows timing:** short sleeps can round up to about 15 ms. Measure real sleep length at startup (`time.Since` around a 1 ms sleep) and log it. Optional: `timeBeginPeriod(1)` through `winmm.dll`, kept only if measurement shows a benefit.

**Optional, later:** preset `targeted` (cut the first piece in the middle of the server name). Do not build it until everything else is accepted.

The writer is `fragment.Conn`, a `net.Conn` wrapper whose first `Write` is fragmented. The DoH client uses it under its TLS layer so the DoH handshake is fragmented too.

## 6. DoH resolver (`internal/doh`)

- Own `http.Client` with `Proxy: nil` set **explicitly**, `ForceAttemptHTTP2: true`, and a custom `DialContext` that ignores system DNS: it dials the server's bootstrap IPs (3 s each) through `netx.Dialer` and `fragment.Conn`. TLS server name comes from the URL host.
- Build queries with `dnsmessage`. A records (AAAA only if enabled). Keep all A records.
- Cache: map + mutex, max 2000 entries, TTL rules from section 4, serve-stale, negative caching. Coalesce simultaneous lookups with `singleflight`.
- Filter answers per "Block private targets"; if everything is filtered return typed error `DNS_BLOCKED_ANSWER`.
- Try servers in order on failure and remember the last good one.
- Optional prewarm after Start: `www.youtube.com`, `youtube.com`, `i.ytimg.com`, `yt3.ggpht.com`.

## 7. Proxy server (`internal/proxy`)

Raw `net.Listener`, not `http.Server`. Per accepted connection, in its own goroutine with `defer recover()` (log and count panics, never crash):

1. Enforce the connection cap. Read deadline 10 s. Parse with `http.ReadRequest` on a `bufio.Reader` wrapped in a 16 KB limit. **Keep using that same `bufio.Reader` afterwards**; it may already hold bytes the client sent after the headers.
2. Route:
   - `GET /proxy.pac` and `GET /health` with `Host` equal to `127.0.0.1:<port>` or `localhost:<port>`: serve them (section 8). Any other `Host` on those paths: `404`.
   - `CONNECT`: parse the target with `net.SplitHostPort` (supports bracketed IPv6); default port 443.
   - `GET`/`HEAD`/`POST` with an absolute `http://` URL: `302` to the `https://` URL, then close.
   - Anything else: `405`, then close.
3. Resolve the target (IP literal skips DNS). Dial each candidate IP (5 s each, 12 s overall) through `netx.Dialer`. Total failure: `502` and close.
4. Reply `HTTP/1.1 200 Connection established\r\n\r\n`.
5. Read the first TLS record (10 s timeout), fragment it to the upstream (section 5), then run two copy loops with pooled 32 KB buffers (`sync.Pool`).
6. Both sides use an idle-timeout wrapper. When one direction ends, half-close the other with `CloseWrite`; when both end, close both. **Every exit path closes both sockets exactly once** (one `closeBoth` helper and `defer`).
7. Count bytes with atomic counters (section 9).

Keep the tunnel code (steps 3-6) in `tunnel.go` as a function that takes an already-accepted client connection, a target address and the bytes already buffered. The Android version will call it from a different front door (file 04).

Shutdown: `Stop(ctx)` closes the listener, waits up to 3 s for tunnels, then force-closes the rest. `Start` and `Stop` are safe to call repeatedly from different goroutines.

## 8. Windows proxy switching and autostart (`internal/platform/windows`, `proxy/pac.go`)

**Browser target: Google Chrome.** Chrome on Windows follows the Windows proxy setting, including a PAC script, so the system-wide approach below makes Chrome work with no manual setup. Do not build Edge or Firefox-specific work; the UI only claims "Works with Chrome".

Modes (`mode`):
- `youtube` (default): the app serves a PAC script at `http://127.0.0.1:<port>/proxy.pac`. It returns `PROXY 127.0.0.1:<port>; DIRECT` for hosts matching the YouTube domain list (exact match, or ends with `.` + domain) and `DIRECT` for everything else. Because `DIRECT` is second, if our app dies the browser goes direct instead of losing internet.
- `all`: same PAC, returns the proxy for every host except localhost and private ranges.
- `manual`: touch nothing in Windows; the UI shows `127.0.0.1:<port>` with a Copy button.

Implementation (`//go:build windows`, `x/sys/windows/registry`, no admin rights; manifest `asInvoker`):
- Key `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`. Back up the previous values of `AutoConfigURL`, `ProxyEnable`, `ProxyServer`, `ProxyOverride` to `%APPDATA%\Younblock\sysproxy-backup.json` with `weSetIt: true` **before** changing anything. Set `AutoConfigURL` to the PAC URL.
- After every change call `InternetSetOptionW(nil, INTERNET_OPTION_SETTINGS_CHANGED /*39*/, nil, 0)` then `INTERNET_OPTION_REFRESH /*37*/`.
- **Crash recovery:** at every app start, if the backup says we changed settings and the proxy is not running, restore. The same function backs `--restore-proxy` and the UI button "Restore internet settings".
- Restore on Stop and on normal exit; restore only values we changed and only if the current value is still ours.
- **Windows-only verification:** after enabling `youtube` mode, Chrome must reach YouTube through the proxy with no manual proxy setup. If PAC-via-registry is not honored on the test machine, fall back to a static `ProxyServer` for `all` mode and report that `youtube` mode needs another method.
- **Autostart:** value `Younblock` under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` set to `"<full path>\younblock.exe" --minimized` when `startWithWindows` is on; removed when off. On every launch, if enabled and the stored path differs from the current exe path, rewrite it (handles moved files). Never create duplicates. A launch with `--minimized` starts hidden in the tray, state `off`, unless `connectOnLaunch` is on.
- `platform/stub` is a no-op that records calls so everything else is testable on Linux.

## 9. Tray, notifications, stats, logging

**Tray (required).** The window is rarely open, so the tray icon is the daily surface. Use a tray library that supports running beside the Wails event loop (for example `energye/systray` or a fork of `getlantern/systray` with an external-loop mode; verify it works with Wails v2 before committing). If, after 3 honest attempts, tray and Wails cannot coexist, stop and tell Ali with the options (do not silently drop the tray).
- Three icons (`assets/tray-off.ico`, `tray-on.ico`, `tray-attention.ico`; 16, 20, 24, 32 px) differing by **shape**, not only color: outlined ring (off), filled ring (on), ring with a dot (attention). Design in file 02.
- Tooltip: `Younblock: Off|On|Starting…|Check needed`.
- Left-click: show or hide the window. Right-click menu: **Turn on / Turn off**, **Open Younblock**, **Quit**.
- Closing the window hides it to the tray; the first time, show the one-time message from file 02 (`closeNoticeShown`). **Quit** always runs Stop, restores Windows settings, then exits.
- Windows notifications (via the tray library or a toast library; verify, and if none works rely on the tray icon and say so): **only** for failure and recovery, never for normal on/off: "YouTube stopped responding. Trying to fix it…", then "Fixed" or "Couldn't fix it. Open Younblock." Rate-limit to one notification per 10 minutes.

**Stats (`internal/stats`):** atomic counters `bytesDown`, `bytesUp`, `connsTotal`, `connsActive`, `connErrors`, `dnsOK`, `dnsFail`, `panics`, plus `startedAt`; a 1-second sampler computes `rateDown`/`rateUp`. **No per-IP maps.**

**Logging (`internal/logx`):** `log/slog`. Never log URLs, paths, headers or payloads. Hostnames only when `logDomains` is true (default false). The ring buffer feeds the diagnostic report (file 03, R20).

## 10. Engine behavior, self-healing and the UI-facing API

### Start sequence ("self-healing, ask second")
1. `connecting`, `detail: "starting"`: start the listener, then `Platform.Attach` (Windows proxy per mode).
2. `detail: "checking"`: quick check (self-test steps 1-3, max 8 s). Pass: `on`.
3. Fail: **automatic heal**: `detail: "trying:<preset>"` for each of `balanced`, `strong`, `light`, `legacy` (re-run steps 3 and 4, 6 s each). First pass: save it as the working method, set `notice: METHOD_SWITCHED(<preset>)` for 10 s, go `on`.
4. None pass: `degraded` with error `SELFTEST_FAILED` (the listener keeps running). Total time to a final answer: 35 s at most.
5. Pressing the button while `connecting` cancels cleanly.

### Self-test steps (`health.RunSelfTest`), each `{id, ok, ms, code, detail}`
1. `listener`: the local proxy accepts a connection.
2. `dns`: resolve `www.youtube.com` through the DoH servers; report which one answered.
3. `youtube`: HTTPS GET of the self-test URL through our own proxy (explicit `http.ProxyURL`), expecting 204 (or 200 for the fallback URL).
4. `video`: TLS handshake only to the googlevideo host through the proxy; a completed handshake passes.

### Watchdog (while On)
Steps 2 and 3 every 120 s. Three failures in a row: run **one** automatic heal sweep (respecting the 10-minute cooldown); if it fixes things, set the notice and stay `on`; if not, go `degraded` and send the notification. One later success returns to `on`. Nothing happens silently: every automatic change is announced through `notice` and the log.

### App API
Implemented in `engine/jsonapi.go`, exposed by Wails bindings and (later) the Android bridge. All data is JSON.

| Method | Returns | Notes |
|---|---|---|
| `GetState()` | `State` | |
| `Start()` / `Stop()` / `Toggle()` | `error` (typed code) | Start follows the sequence above |
| `GetSettings()` / `SaveSettings(Settings)` | `Settings` (normalized) | `mode` or `method` changes apply immediately while On |
| `RunConnectionTest()` | `[]TestStep` | also emits `test:progress` per step |
| `AutoFix()` | `string` (method that worked) | the heal sweep, on demand |
| `RestoreWindowsProxy()` | `error` | |
| `GetDiagnosticReport()` | `string` | plain text, redacted (file 03, R20) |
| `OpenLogsFolder()`, `Quit()` | | |
| `GetAppInfo()` | `AppInfo` | name, version, `platform`, `capabilities`, config path |

Events to the UI: `state` (every change), `stats` (1 per second while On), `test:progress`.

```json
// State
{ "phase": "off|connecting|on|degraded|error|stopping",
  "detail": "starting|checking|trying:strong|",
  "since": "2026-10-13T09:30:00Z",
  "port": 4525, "mode": "youtube", "method": "balanced",
  "notice": { "code": "METHOD_SWITCHED", "arg": "strong" },
  "error": { "code": "PORT_BUSY" } }
// Stats
{ "bytesDown": 0, "bytesUp": 0, "rateDown": 0, "rateUp": 0, "activeConns": 0, "uptimeSec": 0 }
// TestStep
{ "id": "dns", "ok": true, "ms": 84, "code": "", "detail": "RethinkDNS answered" }
// AppInfo
{ "name": "Younblock", "version": "1.0.0", "platform": "windows",
  "capabilities": { "tray": true, "startWithOS": true, "systemProxy": true, "perAppRouting": false } }
```

Error codes (the backend returns codes only; wording lives in the UI files): `PORT_BUSY`, `DNS_UNREACHABLE`, `DNS_BLOCKED_ANSWER`, `PROXY_SETTING_FAILED`, `SELFTEST_FAILED`, `UNKNOWN`. Notice codes: `METHOD_SWITCHED`, `RECOVERED`.

## 11. Delivery phases and acceptance criteria

Work in this order; do not start a phase until the previous one passes. After each phase run the listed checks and paste real output in your report.

**Phase 1 - headless engine (`cmd/core`).** config, netx, fragment, doh, proxy, stats, logx, engine skeleton with the stub platform.
- `go vet ./...`, `go test -race ./...`, `GOOS=windows GOARCH=amd64 go build ./...` pass; the platform-neutral check and the `GOOS=linux GOARCH=arm64` build from section 3 pass.
- Unit tests: planner (sums, no empty piece, size range, `legacy` at most 299 pieces, deterministic with a seeded source); DoH resolver against an `httptest` fake server (A parsing, TTL clamp, cache hit, fallback after first server fails, private-IP filtering, stale serving); request parser including a fuzz test (`go test -fuzz`, 60 s, no panic).
- Integration: fake DoH + local TLS echo server (with `AllowPrivateTargets`); an `http.Client` through the proxy completes an HTTPS request; a recording TCP listener shows the ClientHello arrived in at least 2 separate reads **and** the handshake still succeeded.
- Behavior: idle timeout closes a silent tunnel; the 1025th connection gets `503`; `Stop` returns within 3 s with open tunnels; byte counters match transferred bytes.
- Soak (script included): 5,000 short tunnels including failing ones; goroutine and handle counts return to baseline and memory stays flat.

**Phase 2 - health and self-healing.** Self-test, heal sweep, watchdog, cooldowns, notices. Tests use fake networks that only accept one preset; assert the announced `notice`, the 35 s cap, and the cooldown.

**Phase 3 - Windows adapters.** sysproxy, PAC, autostart. PAC tests (YouTube hosts match including subdomains and not look-alikes such as `notyoutube.com`; everything else `DIRECT`). Registry code compiles for Windows; backup and restore logic is unit-tested against an in-memory fake; autostart path-rewrite logic tested. State clearly which Windows-only behaviors you could not run.

**Phase 4 - Wails app, tray, first run.** Bindings over `jsonapi`, events, single-instance lock (`SingleInstanceLock` in Wails v2; verify), tray and notifications (section 9), first-run flow and the whole UI from file 02.

**Phase 5 - remaining P1 items from file 03.**

**Size and speed targets:** `younblock-core.exe` built with `-trimpath -ldflags "-s -w"` is 12 MB or less; the Wails app is 20 MB or less; the proxy is listening within 300 ms of launch. No UPX (it triggers antivirus false alarms).

**Manual Windows check for Ali** (numbered, plain English, in `README.md`): run the app, press the button, open YouTube in Chrome, press it again to turn off, confirm Windows proxy settings are back; turn on, close the window and confirm the tray icon stays and YouTube still works; use Quit and confirm settings are restored; turn on and kill the app from Task Manager and confirm internet still works and the next launch cleans up; enable "Open when Windows starts", restart Windows and confirm the app appears in the tray, off.

## 12. Rules for you, the agent

1. Deliver **complete files**: no `TODO`, no placeholders, no "rest of code here".
2. Provide `build.bat` (builds core and app, prints exact output paths) and `README.md` in plain English: install Go, Node and Wails, build, run, uninstall.
3. Run everything before saying it works (`go vet`, tests with `-race`, both builds). Never weaken a test to make it pass.
4. Finish with a short report: what was built; commands run and real results; what you could **not** verify (real filtering behavior, Windows-only behavior if you were not on Windows); assumptions; problems you noticed but did not change.
5. You cannot prove the app unblocks YouTube on Ali's network. You can only prove the pieces are sent and reassemble correctly. Say so plainly.
