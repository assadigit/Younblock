# Younblock — Progress Log

A running log: what was done, the exact commands run with their real results, what could not
be verified, and decisions made. Newest entries at the bottom. Interpretations and deviations
are in `DECISIONS.md`; the phase checklists are in `PLAN.md`.

## Environment (Step 1)

Date: 2026-10-13 (Asia/Tehran), machine: Linux sandbox, user `z`, no `sudo`.

Tools found missing and installed **user-space** (no system changes were possible or made):

| Tool | Result |
|---|---|
| git | already present — `git version 2.47.3` |
| Node.js / npm | already present — `v24.21.0` / `11.19.0` |
| gcc | already present — `14` (Linux target only; no mingw) |
| Go | installed to `~/sdk/go` from go.dev — `go version go1.27.2 linux/amd64` |
| gh CLI | installed to `~/sdk/gh` from github.com/cli/cli releases — `gh version 2.102.0` |
| Wails v2 | installed with `go install github.com/wailsapp/wails/v2/cmd/wails@latest` — `v2.16.0` |

Commands and real output (abbreviated where repetitive):

```
$ go version                -> go version go1.27.2 linux/amd64      (after install)
$ gh --version              -> gh version 2.102.0 (2026-09-30)
$ wails version             -> v2.16.0
$ gh auth status            -> ✓ Logged in to github.com account assadigit (token scopes include repo)
$ gh repo clone Younblock   -> Cloning into 'Younblock'... exit 0
$ git ls-files              -> 5 specs + logo.png (commit 8eda994)
$ git pull (after Ali's upload) -> fast-forward, adds backend.py (244 lines) + frontend.py (463 lines)
```

`gh auth setup-git` was run once so git push/pull uses gh's credential helper (the token lives
only in `~/.config/gh/hosts.yml`, outside this repo; never in code, logs or commits).

## Step 1 — Reading everything

- Read `00-start-here.md`, `01-go-backend-rewrite-spec.md`, `02-ui-design-spec.md`,
  `03-reliability-suggestions.md` completely. Read `04-android-spec.md` only far enough to
  honor the Android-ready rules in file 01 §3; nothing from file 04 is being built now.
- Read `legacy/backend.py` and `legacy/frontend.py` completely (both were uploaded at the repo
  root; moved into `legacy/` with `git mv`, see `DECISIONS.md` D1).
- How the old app made Chrome use the proxy: recorded in `DECISIONS.md` D2 (it launched a
  dedicated Chrome with `--proxy-server` and a separate profile; it never touched Windows
  proxy settings).
- `assets/logo.png` verified: valid PNG, 213 x 59, 8-bit RGBA, fully opaque, dominant colors
  red (#C02010/#C01010 ≈ brand red #CE201F) and charcoal — the UNBLOCK wordmark, usable in
  the header at ~28 px height.
- Self-test URLs reachability from this sandbox: to be checked during Phase 2 with real
  commands; if the sandbox cannot reach YouTube, that stays on Ali's manual Windows check.

Could not verify in Step 1: nothing pending — all required inputs (specs, `legacy/backend.py`,
logo) are now present.

## Step 2 — Planning docs

- Created `PLAN.md` (phase checklists with acceptance criteria copied from file 01 §11 and
  file 02 §10), this `PROGRESS.md`, and `DECISIONS.md`.
- Reorganized the repo to the layout file 01 §3 expects: `legacy/backend.py`,
  `legacy/frontend.py`, `assets/logo.png` (all via `git mv`, history preserved).
- Spec files stay at the repo root (they reference each other by bare filename; noted in
  `DECISIONS.md` D1).
- Committed and pushed to `main`.

## Phase 1 — Headless engine

**Status: COMPLETE (all acceptance criteria green).**

Built (file 01 §3 layout): `cmd/core/main.go` (headless CLI: --port --preset --selftest
--restore-proxy --version), `internal/config` (CONFIGURATION block + settings.json with
validation, clamping, atomic write, corrupt-file rename), `internal/netx` (the one outbound
door with the Android-ready Control hook and try-every-address dialing), `internal/fragment`
(pure Plan function with injectable RNG + first-write Conn wrapper), `internal/doh`
(v1-exact DoH request shape, bootstrap dialing, TTL clamp, 2000-entry cache with
serve-stale and negative caching, singleflight, sticky server, private-range filtering),
`internal/proxy` (raw listener, real http.ReadRequest parsing, CONNECT tunnels with
first-record fragmentation, PAC/health endpoints, auto-port, 1024 cap, panic recovery,
3-second graceful stop), `internal/stats` (atomic counters, no per-IP maps), `internal/logx`
(500-line ring + 2 MB x 3 rotating file), `internal/health` (the four-step self-test),
`internal/engine` (state machine, JSON API, events), `internal/platform/stub` (call-recording
no-op platform) and `internal/platform` factory (stub for every build until Phase 3).

### Verification (exact commands and real results)

```
$ go vet ./...                                   -> exit 0 (no findings)
$ go test -race ./...                            -> ok config, doh, engine, fragment,
                                                     health, logx, netx, proxy (5.05s,
                                                     includes the 5,000-tunnel soak), stats
$ GOOS=windows GOARCH=amd64 go build ./...       -> exit 0
$ ./scripts/check-platform-neutral               -> platform-neutral check: OK
$ GOOS=linux GOARCH=arm64 go build ./internal/{engine,config,netx,fragment,doh,proxy,health,stats,logx}/... -> exit 0
$ go test -fuzz=FuzzParseRequest -fuzztime=60s ./internal/proxy/
   -> PASS: 482,339 executions, 0 panics
$ ./scripts/soak.sh (go test -race -run TestSoak ./internal/proxy/)
   -> 4,800 ok + 200 failed tunnels; goroutines 4 -> 4; fds 11 -> 11; heap 761,856 -> 876,544 bytes
```

Additional unit/integration coverage that ran green: planner rules and spec piece-count
table (light 2-4, balanced 3-7, strong 7-26 for a 520-byte hello); legacy Python-rule
reproduction including the length-300 edge the Python crashes on; deterministic seeded
plans; DoH fake-server tests (A parsing, TTL clamp to 120 s, cache hit, fallback to the
second server with stickiness, private-IP filtering on/off, stale serving, negative cache,
singleflight coalescing); request routing (CONNECT default port, bracketed IPv6, 302 for
absolute http:// GET/HEAD/POST, 405 otherwise, PAC/health host checks, 400 for malformed);
read-first-record (non-TLS passthrough, exact TLS record reassembly across split reads,
16 KB + 5 cap); PAC generator (youtube/all/manual) and Go-side domain matching including
`notyoutube.com` and `youtube.com.evil.example` rejections; proxy behavior (idle timeout
closes a silent tunnel, first-record timeout, 503 over the cap, Stop force-closes within
its budget, byte counters match transferred bytes exactly, Start/Stop repeatedly and
concurrently); engine lifecycle (state events connecting->on->off, PORT_BUSY error phase,
settings save + immediate method application, autostart flip reaches the platform,
diagnostic report redaction scan, ring cleared on Stop, quit callback).

The Phase 1 acceptance integration test (fake DoH + TLS echo server + AllowPrivateTargets)
passed: an http.Client completed an HTTPS request through the proxy and a recording
listener showed the ClientHello arriving in >= 2 separate reads while the handshake still
succeeded.

### Live end-to-end evidence (development sandbox, unfiltered network)

```
$ /tmp/younblock-core --selftest
  State: on (port 4525)
  [OK  ] listener    0 ms  Listening on port 4525.
  [OK  ] dns        45 ms  RethinkDNS answered.
  [OK  ] youtube    84 ms  YouTube answered.
  [OK  ] video      98 ms  Video server handshake completed.
  Self-test passed.   (exit 0)
```

This proves the pieces work against the real services: the exact v1 DoH request to
sky.rethinkdns.com via bootstrap 172.67.162.27 with a fragmented ClientHello, a real 204
from https://www.youtube.com/generate_204 fetched through our own proxy, and a real TLS
handshake to redirector.googlevideo.com:443 through the tunnel. It does NOT prove
unblocking on Ali's filtered network — only Ali's manual check can.

```
$ launch-to-listening measurement                  -> 13 ms (target <= 300 ms)
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> 7,425,024 bytes = 7.08 MB (target <= 12 MB), no UPX
$ /tmp/younblock-core --restore-proxy              -> Internet settings restored. (stub platform)
```

### Could not verify in Phase 1

- Anything Windows-specific at runtime (registry, tray, WinINet refresh): only compiled
  (`GOOS=windows` build) and unit-tested against fakes; the real check is Ali's manual step.
- The 1025th-connection 503 was verified with a reduced cap (same code path) because of the
  sandbox file-descriptor limit (DECISIONS.md D15); the 1024 default is asserted separately.
- Real filtering behavior on Ali's network (only Ali can test that).

### Decisions made in Phase 1 (see DECISIONS.md)

D19-D26: go.mod raised to go 1.26.0 by the dependency floor; Platform gained RestoreProxy;
OpenLogsFolder returns the path and the shell opens the browser; terminating replies carry
Content-Length/Connection: close; first-record units cap at 16 KB + 5; health tests inject
trust pools via WebTLS/VideoTLS; CLI overrides (--port/--preset) are not persisted.


## Phase 2 — Health and self-healing

**Status: COMPLETE (all acceptance criteria green).**

Built: the full Start sequence (file 01 §10): connecting/starting (listener + platform
attach) -> connecting/checking (quick check, self-test steps 1-3, max 8 s) -> pass: on;
fail: one automatic heal sweep over balanced, strong, light, legacy (steps 3-4, 6 s each,
`trying:<preset>` detail announced per attempt) -> first pass becomes the working method
with a METHOD_SWITCHED notice for 10 s and is persisted; none pass: degraded with error
SELFTEST_FAILED and the listener keeps running. Total budget capped at 35 s (test-proven
with a 1.2 s override). Watchdog while On: steps 2-3 every 120 s; three consecutive
failures trigger one automatic sweep (respecting the 10-minute cooldown); success from
degraded returns to on with a RECOVERED notice. Notifications only for failure/recovery,
rate-limited as episodes (see DECISIONS.md D27). R19: interface addresses polled every 5 s
while On; on change the DNS cache and DoH client reset and the watchdog check runs
immediately. RunConnectionTest emits `test:progress` per step; AutoFix runs the sweep on
demand; the diagnostic report now carries the self-test summary.

### Verification (exact commands and real results)

```
$ go vet ./...                                        -> exit 0
$ go test -race ./...                                 -> all 9 packages ok (engine 4.08s)
$ GOOS=windows GOARCH=amd64 go build ./...            -> exit 0
$ ./scripts/check-platform-neutral                    -> OK
$ GOOS=linux GOARCH=arm64 go build ./internal/...     -> exit 0
$ /tmp/younblock-core --selftest (live, real services)
   -> State: on; listener OK; dns 0 ms RethinkDNS answered;
      youtube 84 ms; video 81 ms; Self-test passed. (exit 0)
```

Phase 2 acceptance tests (fake networks that only accept one preset):
- only-strong network: Start switches to strong, announces `METHOD_SWITCHED(strong)`,
  shows `trying:` details, persists strong. PASS
- nothing-works network: degraded + `SELFTEST_FAILED`, listener stays up, failure
  notification sent. PASS
- 35 s cap: with a 1.2 s override and 400 ms checks the engine produces its final
  degraded answer inside ~1.2 s (no hang). PASS
- cooldown: a second automatic sweep inside the window is skipped (heal-call count
  frozen); after the window a new sweep runs. PASS
- watchdog: network breaks -> notifications ("Trying to fix it…", "Couldn't fix it"),
  degraded; network returns -> `RECOVERED` notice, back to on, "Fixed" notification. PASS
- watchdog sweep switching method announces `METHOD_SWITCHED(light)` and persists it. PASS
- AutoFix from degraded returns {"method":"light"} and turns on; AutoFix from off starts. PASS
- pressing the button while connecting cancels cleanly (final state off, no drift). PASS
- notice expires after 10 s (fake clock advanced 11 s). PASS
- R19: interface change -> resolver reset (ResetCount grows) + immediate check. PASS
- RunConnectionTest on a fully local fake internet (DoH + web + video): 4 steps OK,
  4 `test:progress` events, report contains the self-test summary. PASS

Could not verify: nothing new beyond Phase 1's Windows-only limits; the real 35 s and
120 s timings only matter on Ali's machine, but their logic is test-proven with overrides.

## Phase 3 — Windows adapters

**Status: COMPLETE (all acceptance criteria green).**

Built: `internal/platform/windows` with the platform-neutral logic files
(`sysproxy_logic.go`, `autostart_logic.go` — no build tag, unit-tested on this Linux
machine against an in-memory fake registry) and the windows-only glue (`windows.go`,
`//go:build windows`: real registry via `golang.org/x/sys/windows/registry`, the
InternetSetOptionW 39/37 refresh via `wininet.dll`, the Platform implementation with
capabilities tray/startWithOS/systemProxy, crash recovery at construction, autostart
path rewrite at launch). The platform factory is now build-tagged: real adapter on
Windows, stub elsewhere. `cmd/pacgen` prints the generated PAC so `scripts/pac-test.mjs`
can evaluate the real JavaScript.

Behavior implemented (file 01 §8): backup of AutoConfigURL/ProxyEnable/ProxyServer/
ProxyOverride to `%APPDATA%\Younblock\sysproxy-backup.json` with `weSetIt: true` BEFORE
any change; AutoConfigURL set to `http://127.0.0.1:<port>/proxy.pac`; restore on Stop,
on exit, at every app start (crash recovery), and behind `--restore-proxy` and the Help
button; restore only values we changed and only when they are still ours; a second
Enable keeps the original backup; autostart value `Younblock` under the Run key set to
`"...\younblock.exe" --minimized`, removed when off, never duplicated (one named value),
rewritten when the exe moves; manual mode touches nothing.

### Verification (exact commands and real results)

```
$ go vet ./...                                  -> exit 0
$ go test -race ./...                           -> all packages ok (platform/windows 1.0s)
$ GOOS=windows GOARCH=amd64 go build ./...      -> exit 0 (registry + wininet code compiles)
$ ./scripts/check-platform-neutral              -> OK
$ GOOS=linux GOARCH=arm64 go build ./internal/… -> exit 0
$ node scripts/pac-test.mjs                     -> PAC behavior test: OK (43 checks)
```

Unit tests against the in-memory fake registry: enable writes the PAC URL, records the
previous values (old PAC, ProxyEnable=1) and refreshes once; enable with nothing
pre-existing restores by deleting; double-enable keeps the ORIGINAL backup; restore puts
back the old value, removes the backup file and refreshes; crash recovery restores a
leftover setting; a foreign value is left untouched (and logged); restore without backup
is a no-op; a corrupt backup is ignored safely; autostart command format
`"C:\...\younblock.exe" --minimized`; enable/disable cycles; no duplicates; rewrite after
a move; enabled reads back.

PAC behavior (real JS): youtube.com and every subdomain of all seven domains -> PROXY;
notyoutube.com, youtube.com.evil.example, evilyoutube.com, youtubecom, x-youtube.com,
example.com, google.com -> DIRECT; case-insensitive; all mode: public hosts PROXY,
plain names/*.local DIRECT, 127/8, 10/8, 172.16/12, 192.168/16, 169.254/16 DIRECT,
172.32.0.1 PROXY (just outside the range), 8.8.8.8 PROXY; manual: everything DIRECT.

### Windows-only behaviors that could NOT be run here (honest list)

- The real effect of writing AutoConfigURL + InternetSetOptionW(39/37): whether Chrome
  on Ali's machine picks up the PAC with no manual setup (file 01 §8's verification).
- The Run-key autostart actually launching the app at Windows start, and `--minimized`
  starting hidden in the tray (the flag is parsed by the phase 4 app shell).
- wininet.dll loading and the syscall return values (compiled and reasoned, not run).
- The one-shot fallback to a static ProxyServer for `all` mode is NOT implemented: the
  spec makes it conditional on PAC not being honored on the test machine, which only
  Ali's manual Windows check can show (DECISIONS.md D30).

## Phase 4 — Wails app, tray, first run, UI

**Status: COMPLETE (all acceptance criteria green).**

Built: the Wails v2.16.0 shell (`main.go`: fixed 360 x 560 dark window, DisableResize,
background --neutral-950 so there is no white flash, dark Windows title bar,
SingleInstanceLock with second-instance window reveal, `--minimized` starts hidden) and
the thin JSON adapter (`app.go`: every engine/jsonapi method as a string-in/string-out
binding, engine events forwarded as Wails events, close-to-tray interception with the
frontend running the one-time notice, OpenLogsFolder via Explorer, Quit path that stops
and restores first). The tray (`tray_windows.go` + energye/systray v1.0.3 external-loop
mode): three shape-different icons at 16/20/24/32 px plus light-taskbar variants (chosen
by reading the Windows AppsUseLightTheme registry value), tooltip
Off|On|Starting…|Check needed, left-click show/hide, right-click menu (Turn on/off, Open
Younblock, Quit), menu label follows the state. Toasts for failure/recovery go through
go-toast (file 01 §9's "or a toast library"; DECISIONS.md D33). The frontend is the full
file 02 UI: tokens.css exactly as specified, base.css with the bundled Inter subset
(50.6 KB for weights 400/500/600 + OFL license), all components (PowerButton with
halo/ring/disc/glyph layers, rotating connecting arc, dashed degraded ring, one ripple
when turning on; StatusBlock with stable-height timer and quiet line; Menu; Accordion;
TestSteps; Toast; inline-SVG Lucide Icon; Wordmark using Ali's logo at 28 px), all five
screens (Connect, FirstRun, Settings with Advanced, Help with the live test and rescue
buttons, About with the three-node diagram), the copy deck in i18n/en.json with zero
hard-coded component text, and the adapter layer (wails.ts over the generated bindings,
mock.ts driven by ?mock=..., android.ts stub). A strict CSP
(default-src 'self'; no inline script; no eval) ships in index.html.

### Verification (exact commands and real results)

```
$ npm run build                       -> 84.7 KB JS + 23.6 KB CSS (target <= 250 KB excl. fonts)
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> build/bin/younblock.exe, 12,720,128 bytes = 12.13 MB (target <= 20 MB), no UPX
$ go vet ./...                        -> exit 0
$ go test -race ./...                 -> all packages ok
$ GOOS=windows GOARCH=amd64 go build ./... -> exit 0
$ ./scripts/check-platform-neutral    -> OK
$ GOOS=linux GOARCH=arm64 go build ./internal/… -> exit 0
$ /tmp/younblock-core --selftest      -> still all four steps OK against the real services
```

Screenshots (file 02, section 10) taken with the mock backend at 360 x 560 into
docs/screenshots/: Connect in all six phases (off, connecting with the amber arc,
on with the green disc and timer, degraded with the Fix connection button, error with
the PORT_BUSY sentence and Open help link, stopping), Connect with the notice line,
menu open, first-run, Settings with Advanced closed and open, Help before/mid/passed/
failed test, About, and Connect on at 320 x 568 and 480 x 900. Each was reviewed with
a vision model against sections 2 and 4 of file 02 (layout, colors, copy) and the
differences found were fixed before capture (one real bug: the power button received
the state object instead of the phase, so On never showed green — fixed and re-shot).

Interaction checks (browser over the built bundle): the switch role and aria-checked
track the state; Escape closes the menu (4 menu items -> 0); the network panel shows
only local requests (fonts, JS, CSS, favicon — all from 127.0.0.1), proving R21's
"frontend loads nothing remote"; keyboard Space/Enter handlers are wired on the button.
The 35 s connect sequence, tray coexistence with Wails and the toast rendering are
Windows-runtime behaviors that could not be verified on this sandbox (below).

### Could not verify in Phase 4 (honest list)

- The tray icon actually appearing beside the Wails event loop on Windows (compiled and
  wired through the external-loop API designed for it; the 3-attempt rule from file 01
  §9 could not be exercised without a Windows session).
- Toast notifications rendering (go-toast drives SnoreToast; only a real Windows
  machine can show it).
- The 360 x 560 window chrome (title bar color, no white flash) as real WebView2
  windows; the web layout itself is screenshot-verified.
- Single-instance lock behavior with a real second launch (API used per docs).

### Notes

- `npm install` needs `--legacy-peer-deps` on this sandbox (a vite/devtools peer
  conflict in the tooling chain); plain `npm install` works where npm resolves it.
- The screenshots were taken from the production bundle served by `vite preview`,
  driven through an automated browser; the app had to be mounted with Svelte 5's
  `mount()` API (the legacy `new App()` instantiation broke $effect).

## Phase 5 — Remaining P1 items, build script, README, final deliverables

**Status: COMPLETE.**

Built: the R21 automated privacy scan (engine tests that run a real session, generate
the diagnostic report and log files, and scan every sample for visited-site hostnames
and non-loopback IPs — nothing is found with the default settings; the opt-in
"logDomains" setting round-trips), `build.bat` (builds both executables with
`-trimpath -ldflags "-s -w"`, prints the exact output paths and SHA-256 checksums),
and `README.md` in plain English (what the app is and is not, tool installation,
build, run, uninstall, the built-in test, privacy, and Ali's 7-step manual Windows
check from file 01 section 11).

### Final deliverables and their verification

```
dist/younblock-core.exe   6.98 MB (7,315,719 bytes)   target <= 12 MB  PASS
dist/younblock.exe       12.13 MB (12,720,128 bytes)  target <= 20 MB  PASS
SHA-256  core: 4ebcd15580509afd3a8ce0d8ada6f5359df724a43784d95c7072f355ba070d2f
SHA-256  app : fb61818fd4dc659d82e4267083b45c15af91d228957f565603ff068cbac908ec
launch-to-listening      12 ms                          target <= 300 ms PASS
```

Both were built with `-trimpath -ldflags "-s -w"`, no UPX. The complete final
verification run (all green): `go vet ./...`; `go test -race ./...` (all packages,
including the new privacy tests); `GOOS=windows GOARCH=amd64 go build ./...`;
`scripts/check-platform-neutral`; the `GOOS=linux GOARCH=arm64` neutral-package build;
`node scripts/pac-test.mjs` (43 checks); `scripts/soak.sh` (5,000 tunnels, baseline
goroutines/fds, flat heap); a 30 s fuzz spot-check (the full 60 s run was done in
Phase 1 with 482,339 executions); the live `--selftest` against real RethinkDNS and
YouTube (all four steps OK).

### The complete P0 review (file 03, section 1) — all done

| # | Item | Status and the check that ran |
|---|---|---|
| R1 | Loopback only | Done — `ListenHost = 127.0.0.1` asserted in config tests; no other bind exists. |
| R2 | Sockets closed on every path | Done — one `tunnelCloser` per tunnel; soak test: 5,000 tunnels, goroutines 4→4, fds 11→11. |
| R3 | No hidden errors | Done — typed codes (PORT_BUSY, DNS_* …) with tests per code; empty error handling appears nowhere. |
| R4 | Every DNS address tried | Done — `TestDialHostTriesEveryAddressInOrder`. |
| R5 | TTL cache, DoH direct | Done — cache tests (clamp, stale, negative) and DoH transport has `Proxy: nil` with bootstrap dialing. |
| R6 | Idle 300 s / dial 5 s | Done — `TestIdleTimeoutClosesSilentTunnel`, dialer timeout tests. |
| R7 | No per-IP maps | Done — atomic counters only; soak heap flat. |
| R8 | Connection cap 1024 | Done — `TestConnectionCapReturns503` (reduced cap, same code path) + default asserted. |
| R9 | Fewer/larger default pieces | Done — balanced default; piece-count table tests; measured sleep logged and in the report. |
| R10 | TCP_NODELAY never disabled | Done — `SetNoDelay(false)` appears nowhere (verified by search); Go default kept. |
| R11 | Real HTTP parsing + fuzz | Done — http.ReadRequest + net.SplitHostPort; fuzz 482,339 execs, no panic. |

### The complete P1 review (file 03, section 2) — all done

| # | Item | Status and the check that ran |
|---|---|---|
| R12 | Built-in self-test | Done — 4 steps with hints; `--selftest` CLI; health tests + live run (all OK). |
| R13 | Self-healing that announces | Done — fake-network tests assert the METHOD_SWITCHED notice, the 35 s cap and the cooldown; every change is also logged and notified. |
| R14 | Watchdog and "Check needed" | Done — `TestWatchdogHealsAndRecovers` covers on→degraded→on with notifications. |
| R15 | Several secure-DNS providers | Done — RethinkDNS/Cloudflare/Google with sticky fallback (`TestResolverFallbackAfterFirstServerFails`). |
| R16 | Windows proxy safety | Done — PAC with DIRECT fallback, backup-before-change, restore-only-ours, crash recovery (all unit-tested with the fake registry); kill-while-on remains Ali's manual test. |
| R17 | Lifecycle hygiene | Done — single-instance lock (Wails), port busy auto-pick, Stop ≤ 3 s with open tunnels, recover() per connection (soak panics = 0). |
| R18 | Tray and autostart safety | Done — close→tray with the one-time notice, Quit stops+restores, Run-key dedupe/rewrite logic tested; `--minimized` starts hidden; the restart test is Ali's. |
| R19 | Network changes and sleep | Done — interface polling resets DNS and re-checks (`TestNetworkChangeResetsDNSAndRechecks`). |
| R20 | Settings file robustness | Done — truncated/corrupt/wrong-type/future-version tests with atomic writes. |
| R21 | Privacy by default | Done — strict CSP (no inline script, no eval), local-only network panel check, ring cleared on Stop, automated report/log hostname+IP scan (new test). |
| R22 | Android-ready core | Done — `scripts/check-platform-neutral` + the arm64 build ran green in every phase. |

### Honest limits (what only Ali can verify)

- Whether YouTube actually loads on the filtered network (the whole point) — the
  self-test and repair machinery is proven; the filtering behavior is not.
- Tray icon and toast rendering beside the Wails window; the 360 x 560 window chrome;
  the single-instance lock with a real second launch; Chrome honoring the PAC;
  autostart after a real restart; kill-while-on leaving the internet working. All are
  compiled, wired per the documented APIs and logic-tested with fakes, but they need a
  real Windows session — that is exactly Ali's 7-step manual check in the README.

## Release v0.1.0 — first Windows test build, published to GitHub Releases

Date: 2026-10-13 (Asia/Tehran). Trigger: Ali asked for a single executable he can
test on his Windows PC, published to the repo's GitHub Releases.

### Gates re-run on the unchanged Phase 5 tree (all green)

```
$ go vet ./...                          -> exit 0
$ go test -race ./...                   -> all packages ok
$ GOOS=windows GOARCH=amd64 go build ./... -> exit 0
```

### Fresh builds with the required flags (no UPX)

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 9,550,336 bytes (9.11 MB)   target <= 12 MB  PASS
      SHA-256 f22426881f9bd6c5922d67b5cd2e1b19470156b1f8c7526bfbf72df6b9b348c6
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,720,128 bytes (12.13 MB)     target <= 20 MB  PASS
      SHA-256 fb61818fd4dc659d82e4267083b45c15af91d228957f565603ff068cbac908ec
      (byte-identical to the Phase 5 build — the app build is reproducible)
$ /tmp/younblock-core --selftest (Linux build of the same tree, live network)
   -> listener OK, dns OK (RethinkDNS), youtube OK, video OK — all four steps pass
```

### Correction to the Phase 5 record (found during this rebuild, nothing silent)

The Phase 5 line "dist/younblock-core.exe 6.98 MB (7,315,719 bytes)" was wrong about
what the file was: a stripped Linux build of the same command is **exactly**
7,315,719 bytes, which proves the old file was a Linux ELF binary named `.exe` — it
was built without `GOOS=windows` and would not have run on Windows. The exe attached
to this release is the first genuinely-Windows core exe; the numbers above replace
the Phase 5 core line. `build.bat` was always correct (it runs on Windows, where
GOOS is windows by default). The app exe was and is correct.

### Release

- Tag `v0.1.0` on `main`, release title "Younblock v0.1.0 — first Windows test
  build", both exes attached, plain-English notes: which file to run, the 2-minute
  quick test, the unsigned-file SmartScreen instructions, the WebView2 note, the
  SHA-256 numbers, and the honest what-is-only-provable-on-real-Windows list.
- The repository stays source-only: `dist/` and `build/bin/` are git-ignored, so the
  release page is the single source of truth for binaries and checksums.

## Phase 6 — Ali's five post-test requests (v1.1.0)

**Status: COMPLETE (all five features built, tested, screenshotted).**

Ali ran v0.1.0 on real Windows: "the backend and frontend both works." He then
asked for five additions; all five shipped, default-off or additive.

1. **Route-all switch** — the existing "All websites" mode surfaced as a
   top-level Settings switch (DECISIONS.md D43).
2. **Dedicated Chrome session** — new `openBrowserOnConnect` setting
   (default off); neutral `internal/browser` package with the exact legacy
   command line; Windows shell launcher with registry + standard-path lookup;
   session ends on off/error/quit; every action logged, toasted and emitted
   as a `browser:*` event (D40).
3. **Background texture** — two soft brand glows + a 4.5%-opacity SVG noise
   grain, all inline, no network (D42).
4. **Response time ("ping")** — every successful health check records the
   YouTube round-trip; shown as `latencyMs` in the state, "NN ms · … used"
   under the timer, and "response time:" in the diagnostic report; resets on
   Stop (D41).
5. **Ali's icon.ico** — moved to `assets/icon.ico`, installed as
   `build/windows/icon.ico` (exe/window icon) and `build/appicon.png`;
   tray keeps its state icons per file 01 §9.

### Verification (exact commands and real results)

```
$ go vet ./...                                    -> exit 0
$ go test -race ./...                             -> all packages ok, including
                                                     the new internal/browser
                                                     tests (legacy command-line
                                                     parity, candidate paths,
                                                     lookup order) and the new
                                                     latency lifecycle test
                                                     (present after start/test,
                                                     absent before/after stop,
                                                     report line present)
$ GOOS=windows GOARCH=amd64 go build ./...        -> exit 0 (includes
                                                     browser_windows.go)
$ ./scripts/check-platform-neutral                -> OK
$ GOOS=linux GOARCH=arm64 go build ./internal/... -> exit 0 (internal/browser
                                                     is neutral like the rest)
$ npm run build                                   -> 87.1 KB JS + 24.2 KB CSS
                                                     (target <= 250 KB excl. fonts)
```

Screenshots: all 19 re-taken at 360x560 (plus 320x568 and 480x900) from the
production bundle; DOM checks confirmed the "84 ms · 12.7 MB used" quiet line
and both new Settings rows; a vision model confirmed the textured background,
the ping line, the styled toggles and the absence of glitches on the key
screens.

New settings field round-trips through settings.json (`openBrowserOnConnect`
in TestSaveLoadRoundTrip); old v0.1.0 settings files simply default it to off.

### Could not verify in Phase 6 (honest list)

- The dedicated Chrome window actually appearing on Windows (compiled and
  logic-tested; the spawn path runs only on a real Windows session).
- Chrome lookup via the App Paths registry key on a real machine.
- The exe icon showing in the Windows taskbar/exe properties (the .ico is
  installed in the Wails build assets; rendering is Windows-side).
- The toast for "Chrome opened in a separate session" rendering beside the
  tray (same Windows-only surface as the Phase 4 toasts).

## Release v0.2.0 — Ali's five improvements, published to GitHub Releases

Date: 2026-10-14 (Asia/Tehran). Tag on the Phase 6 commit (a768e84); the same
verification gates had just run green (see the Phase 6 entry).

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 9,552,384 bytes (9.11 MB)   target <= 12 MB  PASS
      SHA-256 fea375cd124512f00a0f519d237ab5373ff324f3ad53a1abdb375a6249925f26
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,718,080 bytes (12.13 MB)     target <= 20 MB  PASS
      SHA-256 99d57561e839e9458539eb781c0c70f081653eaee0ace03c9a1eb546ef8f2e15
$ /tmp/yb-core --version  -> Younblock core 1.1.0
$ /tmp/yb-core --selftest (live) -> listener OK, dns OK, youtube OK (89 ms), video OK (94 ms)
```

Release notes list the five features (both new switches off by default), the
upgrade path from v0.1.0 (replace the exe; settings are kept), SmartScreen
instructions, checksums and the honest only-on-Windows list. Assets re-downloaded
from the release page: checksums identical.

## Hotfix 1.1.1 / Release v0.2.1 — the "didn't even connect" report

Date: 2026-10-14 (Asia/Tehran). Ali's screenshot after v0.2.0: degraded, timer
frozen at 00:00:00, never one successful YouTube step.

Investigation (recorded in DECISIONS.md D44):
- `git diff v0.1.0..v0.2.0 --name-only`: no network-stack package changed
  between the release that worked on his machine and the one that didn't.
- Live self-test of the exact v0.2.0 code from the dev box: listener OK,
  dns OK, youtube OK (81 ms), video OK — the tunnel code is healthy; the
  failure is environmental (his network's blocking changed, or a local
  firewall/AV on the new unsigned exe hash).
- What the report exposed in the app itself: degraded-from-start never
  started the watchdog/net monitor (R14 violation), froze the timer (no
  stats loop), and said nothing about WHICH part failed.

Fixes (engine + UI, all tested):
- `fail` in the state JSON (step + code, root-cause ordered), degraded
  subtitle distinguishes secure DNS from the YouTube tunnel, "last failed
  check" line in the diagnostic report.
- `goDegraded` starts the stats loop, watchdog and network monitor: the
  timer runs and the app self-recovers from degraded.
- A changed secure-DNS list swaps the resolver live (atomic pointer +
  `Server.SetResolver`); previously the old servers were kept until restart.
- The Advanced DoH list is editable: remove buttons, one-tap add of
  AdGuard/Quad9/dns.sb/the defaults; a new provider is added at the top and
  tried first; "tried first" marker; keep-at-least-one guard.
- New tests: degraded root-cause recording + live stats, youtube-failure
  variant, and live DoH switch recovering from degraded via the watchdog
  (previously impossible). Version 1.1.1 everywhere.

Gates: go vet; go test -race (all packages); GOOS=windows full-app build
(PE32+); scripts/check-platform-neutral; arm64 neutral build; npm run build
(89.7 KB JS / 25.3 KB CSS); live self-test; degraded + settings screenshots
re-taken and vision-checked; add/remove provider interaction verified via
agent-browser.
