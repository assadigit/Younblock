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

## Release v0.2.1 — hotfix published to GitHub Releases

Date: 2026-10-14 (Asia/Tehran). Tag on the hotfix commit (390e32f); gates had
just run green on the same tree.

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 8,980,992 bytes (8.56 MB)
      SHA-256 5e9ebae6b4ea43c3385ecc98698dbfba71b46915af1f1cde0a2db62acd6a4301
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,225,536 bytes (11.66 MB)
      SHA-256 bd06ebdb1e4369477814bfc9f7e5026fe972d42209eb73438fdc1017da33095a
$ /tmp/yb-core-linux --version  -> Younblock core 1.1.1
$ /tmp/yb-core-linux --selftest (live) -> listener OK, dns OK, youtube OK (85 ms), video OK
```

Release notes explain the honest diagnosis (identical network code between
v0.1.0/v0.2.0; the block is on the network side), the three visible changes,
and a step-by-step "what to do if it still won't connect" (including running
the old v0.1.0 exe once to prove the network changed). Assets re-downloaded
from the release page: checksums identical.

## Phase 8 — power-button animation fix + professional UI polish (v1.2.0)

Ali's report after v0.2.1 worked on real Windows: the power button animation
"is buggy… like it stuck midway", the UI should look more professional, and
the footer should not imply Chrome-only.

Reproduction: recorded the button click in the sandbox browser with
`prefers-reduced-motion` emulated — the connecting arc froze mid-circle and
the turn-on ripple never ran (frame-by-frame VLM verification). The cause was
the blanket `animation/transition-duration: 0.001s !important` rule in
base.css: Windows mirrors the "system animations off" (and RDP) setting into
WebView2's reduced-motion media query. The arc is a functional progress
indicator, not decoration, so freezing it read as a stuck button.

Changes (all frontend; no engine/network code touched):

- base.css: blanket reduced-motion kill-rule removed; per-component policy
  (decoration off, spinner + color/opacity fades stay) — D45.
- PowerButton: orchestrated turn-on (spring pop 550 ms, two staggered echo
  rings 640/760 ms, opacity-crossfaded gradient lit plate 320 ms, slow glow
  breath), press release hardening (click/pointercancel/blur), thicker glyph
  stroke (4), resting hover feedback, connecting arcs refined (3 px, layered
  counter-phase second arc, gentle glyph shimmer), degraded rim = solid amber
  glow, error rim = danger glow, ring/halo color choreography.
- StatusBlock: state-colored status dot, timer weight 500 + tracking, quieter
  quiet-line, degraded subtitle warmed to amber, "Fix connection" restyled as
  an amber ghost pill (a warning repair, not a second "turn on").
- Connect footer: badge chip, "WORKS WITH EVERY BROWSER" micro-caption.
- Copy: first-run/Help(a1,a3)/About(how[0], diagram) now say "your browser"
  and explain the Windows-proxy coverage precisely; footer key updated; the
  Chrome-session setting keeps its honest Chrome-only wording.
- Version 1.2.0 in defaults.go / wails.json / mock.

Gates: `go vet` clean; `go test -race ./...` all green (11 packages);
`GOOS=windows` full build OK; check-platform-neutral OK; linux/arm64 core
build OK; `npm run build` OK (90.2 KB JS / 28.2 KB CSS). All 20 doc
screenshots re-shot at 360x560 (+320x568, +480x900 responsive); VLM QA grid
passed all six state checks; the animation was verified frame-by-frame in
both motion modes — the arc now keeps spinning under reduced motion and the
disc still turns green.

Also fixed here (found while re-running the gates): App.svelte imported the
Settings *component* under the same name as the Settings *type*, which broke
`svelte-check` and made `vite dev` (dependency scan) unusable — the component
import is now `SettingsScreen`. That clears 4 of the 16 pre-existing
svelte-check errors; the remaining 12 are the known runes/tsconfig quirk
(`$state` untyped to svelte-check) and the Android stub, identical on HEAD —
`npm run build`, the gate this project has always used, is green.

## Release v0.3.0 — published to GitHub Releases

Date: 2026-10-14 (Asia/Tehran). Tag v0.3.0 on the phase-8 commit (614f19a);
gates had just run green on the same tree (go vet, go test -race, GOOS=windows
build, platform-neutral, arm64 core build, npm run build, live selftest).

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 8,980,992 bytes (8.56 MB)
      SHA-256 ca13938a44efebdc7c03e83b9590e215dbcb718c556fb5c9aa2736f99af8f585
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,229,120 bytes (11.66 MB)
      SHA-256 5e505b53be2a8018c89f42002d14030374d475c3d4c5caa767abdd88be599518
$ /tmp/yb-core-linux --version  -> Younblock core 1.2.0
$ /tmp/yb-core-linux --selftest (live) -> listener OK, dns OK, youtube OK (88 ms), video OK (107 ms)
```

Release notes explain the reduced-motion root cause in plain language, the
new orchestrated turn-on, the polish, and the every-browser wording. Assets
re-downloaded from the release page: checksums identical.

## Phase 9 — every browser + logo palette (v0.4.0 / app 1.3.0)

Ali's v0.3.0 field report: only the dedicated Chrome session (launched with
--proxy-server) actually used the proxy; other Chrome profiles and Opera did
not, and he disliked the green palette.

- Root cause: the registry PAC (AutoConfigURL) is a mechanism several
  real-world things quietly break (stale browser caches, other tools
  overwriting it, PAC-fetch quirks). D46 switches to the manual proxy
  values (ProxyEnable/ProxyServer/ProxyOverride + bypass list) — the
  v2rayN/Clash mechanism every browser follows — with the backup/restore
  state machine kept, any AutoConfigURL cleared while active, a
  WM_SETTINGCHANGE broadcast beside InternetSetOption(39/37), a legacy-PAC
  cleanup at startup, and a watchdog re-assert (EnsureAttached +
  PROXY_REAPPLIED notice).
- Proxy consequences handled: youtube mode fragments only routed hosts
  (unfragmented relay for the rest), plain-http to non-routed hosts is
  forwarded as a real forward proxy (origin-form, hop-by-hop headers
  stripped, Connection: close) so plain-http sites keep working.
- UI restyled to the logo (D47): warm charcoal neutrals, padlock-red
  primary/on state, lightning-gold accent (warn, focus, echo rings, OK
  checks); danger keeps its distinct coral; tray icons regenerated.
- Tests: sysproxy manual-proxy suite rewritten (+ upgrade, orphan, clobber
  cases), proxy forward/fragmentation tests, watchdog ensure test.

Gates on the committed tree: go vet; go test -race (11 packages);
GOOS=windows full build; check-platform-neutral; arm64 core build; npm run
build (90.3 KB JS / 28.6 KB CSS); live selftest 1.3.0 all OK (youtube 87 ms,
video 102 ms). Real-browser E2E through the running core on port 4599:
YouTube loads (fragmented tunnel), example.com https loads (plain tunnel),
info.cern.ch http loads (forward path). Screenshots of all states re-shot
and vision-checked: no green anywhere, red on-state confirmed, red checked
switch, no console or page errors.

## Release v0.4.0 — published to GitHub Releases

Date: 2026-10-14 (Asia/Tehran). Tag v0.4.0 on the phase-9 commit (4c199f3);
gates had just run green on the same tree.

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 9,581,568 bytes (9.14 MB)
      SHA-256 78de068e4175c79ee61381520d6d4f6c1987a10b4562afa91705dde6c260873c
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,750,848 bytes (12.16 MB)
      SHA-256 5c7d13cf06b8fe82875bc9486857d69fcd1bf78f809eca66ec534562345b1cc2
$ /tmp/yb-core-linux --version  -> Younblock core 1.3.0
$ /tmp/yb-core-linux --selftest (live) -> listener OK, dns OK, youtube OK (86 ms), video OK (103 ms)
```

Release notes explain the PAC→manual-proxy switch in plain language, the
anti-clobber watchdog, the new logo palette, and the safer pass-through
browsing. Assets re-downloaded from the release page: checksums identical.

## Phase 10 — the "white space" + "Fix connection is on the edge" report (v1.3.1)

Ali sent two screenshots of the degraded state. Forensics first: VLM reads
plus pixel scans of both images (48.png: running timer, gold ghost pill —
current 1.3.0 UI; 4545.png: green pill + dashed rings + frozen 00:00:00 —
an old 1.1.x-era image he kept, same complaint). The "blue pixels" in both
turned out to be ClearType subpixel fringes around white text, and the strip
under the green button is the Windows taskbar — i.e. the button really sat
on the window's bottom edge.

Measurements on the live 1.3.0 frontend (vite preview + agent-browser at
360x560): degraded document height 574 in a 560 viewport (footer fully
clipped, Fix-connection pill 486..530 with the window ending at 560), error
state 634 (74 px clipped). The centered flex with `overflow: hidden` on
body distributed the overflow badly — the top of the stack stayed pinned
while the bottom fell out of the window, which is exactly the dead gap /
edge-slam Ali photographed. A second, deeper cause: Wails' initial
`SetSize(360, 560)` lands on the OUTER window rectangle after DPI scaling
(winc `scaleWithWindowDPI` + `MoveWindow`), so the WebView2 client — the
CSS viewport — measures ~336x522 at Ali's scaling: even the "on" state was
drawing in a window smaller than designed.

Fixes (D48):

- Window: `clientsize_windows.go` finds the `wailsWindow`-class top-level
  window of our PID, reads `GetDpiForWindow`, measures the real frame
  (`GetWindowRect` − `GetClientRect`), and `MoveWindow`s the outer window
  so the client is exactly 360x560 logical at that DPI (top-left shifted
  by half the growth to keep the visual center). Failures log one info
  line and keep the Wails size. `clientsize_other.go` is the dev no-op
  (app.go/main.go stay build-tag free, D35).
- Shell budget: header 52→44; `#app` got `max-height: 100dvh` +
  `overflow: hidden`; `.app-main` is the flex owner; the footer is
  `flex: none` inside the column with `margin: 0 auto var(--space-4)` —
  it can never be pushed out of the window again.
- Connect: `justify-content: safe center`, internal scroll safety net
  (scrollbars hidden), `min-height: 0`; the timer/quiet block collapses
  to 0 height in states that can never show it (off/connecting/error/
  stopping) via the 0fr↔1fr grid-rows animation, and grows back smoothly;
  the error reason text is capped at 100 px with its own scroll; the error
  state also steps the button down to 0.88.
- Responsive ladder: button 0.88 under 546 css px of viewport height;
  whole-screen `zoom: 0.82` under 470 (physically correct on a 150%-scaled
  display, where a css px is already 1.5 device px).
- `BackgroundColour` 30,31,33 → 37,33,32 (the real #252121-family
  neutral; D48).

Verified: all six states at 360x560 → docH exactly 560, footer bottom 544,
zero internal scroll (degraded fix pill 457..501, error help link ends
492.5); at 336x522 (Ali's viewport) degraded fits with zero scroll; at
288x448 (150% scaling) degraded/error fit via the ladder (0–2 px scroll).
All 19 doc screenshots re-shot; VLM QA pass (after a false "cut off"
hallucination was disproved by same-session pixel measurement: badge text
rows 526..533, 27 px clear of the bottom). Golden-path E2E (toggle → on,
Settings, Help and back) with zero console messages and zero page errors.
svelte-check: 12 errors — the documented pre-existing runes quirk, count
unchanged.

Gates on the committed tree: go vet; go test -race (13 packages); GOOS=windows
full build; check-platform-neutral; arm64 core build; npm run build (90.4 KB
JS / 29.4 KB CSS); live selftest 1.3.1 all OK (youtube 90 ms, video 138 ms).

## Release v0.4.1 — published to GitHub Releases

Date: 2026-10-14 (Asia/Tehran). Tag v0.4.1 on the phase-10 commit (53e7a58);
gates had just run green on the same tree.

```
$ GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/younblock-core.exe ./cmd/core
   -> PE32+ console, 8,999,424 bytes (8.58 MB)
      SHA-256 ace242fac62f33eaf4f47e1706a6ee387fa1b9ba68c9b621355d13e5b858bf7c
$ wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
   -> PE32+ GUI, 12,251,648 bytes (11.69 MB)
      SHA-256 386d3679862c89e39e12137d85ad814193a68c80c5795db046b92a322f04afab
$ /tmp/yb-core-linux --version  -> Younblock core 1.3.1
$ /tmp/yb-core-linux --selftest (live) -> listener OK, dns OK, youtube OK (78 ms), video OK (99 ms)
```

Release notes explain the too-small-inside window and the overflowing
warning states in plain language, plus the three-step check for Ali.
Both assets re-downloaded from the release page: checksums identical.

---

## Phase 11 — v1.4.0 / release v0.5.0: the 32-bit and Linux Mint versions

Ali's request: "from now on, I want this version as well: a 32-bit version, a
version working on linux mint as a single executable. Every change we do on
the main version must be applied on them as well." Plus the pending error-state
screen fix from the previous session.

### What was built

- **StatusBlock (D49):** in the error state the reason now renders as the
  subtitle under the state label (danger color, 280/100 px caps with scroll);
  the separate error-text paragraph and its dead reserved gap are gone; the
  error row keeps the "Open help" link only.
- **internal/platform/linux (D50):** gsettings system proxy (mode/http/https/
  ignore-hosts to every installed supported schema; raw-literal verbatim
  backup/restore; EnsureAttached anti-clobber; marker-guarded crash cleanup),
  ~/.config/autostart desktop entry (Exec carries --web), notify-send notices,
  capability probing (SystemProxy only when gsettings + org.gnome.system.proxy
  exist; honest false on headless boxes). 9 test groups against a fake
  gsettings runner.
- **internal/webui (D50):** go:embed frontend bundle + the JSON API
  (state/start/stop/toggle/settings/appinfo/test/autofix/restore-proxy/report/
  logs/quit) + SSE /api/events; loopback bind only; 405-with-Allow method
  guards; 1 MB settings body cap; index.html no-store. 7 test groups with a
  real engine (stub platform + fake network), including a live SSE stream
  assertion.
- **Frontend:** http.ts adapter (EventSource + fetch, auto-detected via
  /api/appinfo probe with 1.5 s timeout so vite preview still falls back to
  the mock); i18n setOsWord("Linux") in-place word swap before first render;
  firstRun.textNoTray; help q2 and the separate-Chrome settings row gated on
  the new `browserSession` capability (true on Windows, false on Linux/stub);
  mock reports 1.4.0.
- **cmd/core --web (D50):** web edition launcher — default port 17500 with a
  free-port fallback + honest print, connectOnLaunch parity with app.go,
  best-effort xdg-open, SetOnQuit(os.Exit). Also usable on Windows.
- **32-bit (D51):** wails windows/386 GUI + core; parseDword's `1<<31` int
  overflow fixed (uint64 accumulator, uint32 return); linux/386 static build.
- **Sync (D52):** build.bat → 4 Windows files; build.sh → all 6 targets from
  one tree; one version constant (config.AppVersion 1.4.0) shared by every
  edition.

### Real verification (commands and outcomes)

- `go vet ./...` — clean. `gofmt -l` — clean for every file this phase touched
  (linux.go reformatted to tabs; 5 pre-existing space-indented files left
  alone on purpose).
- `go test -race ./...` — all 14 packages ok, including the new
  internal/platform/linux and internal/webui.
- Cross builds — GOOS=windows GOARCH=amd64 ./... OK; windows/386 ./... OK
  (after the parseDword fix); linux/amd64 OK; linux/386 OK; linux/arm64
  ./internal/... OK (neutral check).
- `GOARCH=386 go test` — CANNOT run in this sandbox (kernel seccomp blocks
  32-bit syscalls: "signal: bad system call"); 386 is verified by compile +
  vet + the amd64 suite, the same standard the Windows builds always had.
- gsettings literal verification — real glib 2.84.4: `get` prints 'manual'/
  plain int32 ports (`8080`, no prefix)/comma+space arrays; `set` accepts our
  literals; `uint32 N` would never compare equal on the int32 schema keys.
  Cross-process dconf persistence NOT verifiable here (no dconf-service) —
  recorded in linux.go and README as Ali's manual check.
- `scripts/check-platform-neutral` — OK.
- `npm run build` — 94.40 KB JS / 29.32 KB CSS. `svelte-check` — 12 errors /
  2 warnings, all pre-existing ($state runes/tsconfig quirk + two
  state_referenced_locally warnings on HEAD-era code); nothing new.
- **End-to-end on the real Linux binary** (agent-browser against
  127.0.0.1:17500, real network):
  - First-run screen shows the Linux word swap ("Open Younblock when Linux
    starts") and the no-tray text; Get started works.
  - Toggle ON: phase "on", latencyMs 86–91 (real YouTube round-trip), SSE
    stats live ("86 ms · 6.8 KB used"), timer running.
  - Settings: "when Linux starts" present; "Open a separate Chrome" hidden
    (browserSession=false). Help: 6 accordions (tray q2 hidden). About: 1.4.0.
  - Forced PORT_BUSY (python listener + autoPort=false via the API): error
    state reached; subtitle = the PORT_BUSY reason; `.subtitle.error` present;
    `.error-text` gone; "Open help" present; docH=560@360x560 and 522@336x522
    with the footer visible and an 8 px label→reason gap.
  - Recovery: port freed → toggle → on again. Quit via the page menu: process
    exits (measured 0.11 s from off, 3.14 s from on).
  - Autostart: FirstRun checkbox (default on) → engine → linux adapter wrote
    ~/.config/autostart/younblock.desktop with the real Exec path + --web.
  - `agent-browser errors`/`console` — zero page errors across every batch.
- Live selftest on the final binary — listener 0 ms, dns 0 ms (RethinkDNS),
  youtube 81 ms, video 92 ms: passed.
- VLM QA of the six Linux screenshots — all OK; one "About text clipped"
  claim disproved by pixel measurement (scrollWidth == clientWidth on every
  li, no document overflow), same pattern as the phase-10 hallucination.
- Binary matrix (file(1) verified): younblock.exe PE32+ x64 GUI 12.76 MB;
  younblock-x86.exe PE32 i386 GUI 12.18 MB; younblock-core.exe PE32+ x64
  10.93 MB; younblock-core-x86.exe PE32 i386 10.50 MB; younblock-linux-amd64
  ELF x86-64 static 8.28 MB; younblock-linux-386 ELF i386 static 8.02 MB.

### Honest limits

- A real Mint desktop could not be driven from here: whether an ALREADY-RUNNING
  browser picks up the gsettings change the moment the button is pressed is
  Ali's manual check (README list, step 1). The write path's literal forms and
  acceptance are verified; persistence over dconf is not.
- 386 test binaries cannot execute in this sandbox (seccomp); compile + vet
  only.
- The orphan-process pitfalls during E2E (a `cd repo && ./bin &` backgrounds
  the compound, so `kill $!` missed the binary; the sandbox also reaps
  background processes between tool calls) were worked around by batching
  server + browser in single calls — noted here so future phases don't repeat
  the confusion.

### Release v0.5.0 record

- Commit 1857302 (tag phase-11) pushed to main.
- Release live: https://github.com/assadigit/Younblock/releases/tag/v0.5.0
  with six assets, all round-trip re-downloaded and SHA-256 identical:
  - younblock.exe 12.76 MB (5245cddbc499…)
  - younblock-x86.exe 12.18 MB (53366138f055…)
  - younblock-core.exe 10.93 MB (64adbc33cd9c…)
  - younblock-core-x86.exe 10.50 MB (68e11b92eff7…)
  - younblock-linux-amd64 8.28 MB (7a8a0b0a10aa…)
  - younblock-linux-386 8.02 MB (eb37a5873f5d…)
- Upgrade notes in the release body; Ali's Mint manual check list in README.

## Phase 12 — v1.4.1 / release v0.5.1: the proper Linux Mint packages

Ali asked for three more Mint-friendly formats — .deb, AppImage and Flatpak —
on top of the single file, with every change still applied to all versions.

### What was built

- The .deb, AppImage and Flatpak all wrap the SAME static binary as the
  plain-file edition (D53); packaging/ holds the shared pieces (desktop
  template, generated hicolor icons, one script per format).
- younblock_1.4.1_amd64.deb 3.0 MB and younblock_1.4.1_i386.deb 2.9 MB:
  /usr/bin/younblock, menu entry, 9 icon sizes, dpkg metadata with
  Depends: libglib2.0-bin (gsettings) and Recommends: libnotify-bin,
  xdg-utils; postinst/postrm refresh the caches when the helpers exist.
- Younblock-1.4.1-x86_64.AppImage 4.0 MB and Younblock-1.4.1-i686.AppImage
  3.9 MB: AppRun forwards to the binary with --web; built FUSE-free via
  appimagetool --appimage-extract-and-run (i686 through --runtime-file).
- Younblock-1.4.1-x86_64.flatpak 3.0 MB: manifest + metainfo for
  io.github.assadigit.Younblock on org.gnome.Platform 48; permissions
  network + session bus + host filesystem + notifications talk (D55).
- Version 1.4.1 everywhere; autostart resolution D54 with 4 new tests.

### Verification actually run from the build machine

- .deb: extracted, desktop file Exec=/usr/bin/younblock --web, binary
  --version 1.4.1, live control page + /api/appinfo + settings round-trip.
- AppImage x86_64: through the real AppImage runtime
  (--appimage-extract-and-run): version, web edition, live API, and the
  autostart entry names the .AppImage file itself. i686: built + ELF i386
  confirmed (executing needs a real 32-bit host, as with the 386 binaries).
- Flatpak: installed with `flatpak --user install` (bwrap overlay +
  session-bus shim + tmpfs data dir to work around the container's
  O_TMPFILE/RO-root restrictions), then: deploy metadata (command,
  shared=network, sockets=session-bus, filesystems=host), /app/bin/younblock
  byte-identical (sha 07ea6373…) to dist/younblock-linux-amd64, desktop file
  with the flatpak-wrapped Exec, 9 icons, live control-page API, and the
  start-with-OS entry reading Exec=flatpak run io.github.assadigit.Younblock
  --web (D54 through FLATPAK_ID).
- Gates: go vet; go test -race (14 pkgs); full build.sh (all six binaries +
  four packages in one run; the linux-amd64 binary reproducible across
  rebuilds); live selftest 1.4.1 all OK (youtube 81 ms, video 95 ms);
  platform-neutral + arm64 neutral; svelte-check 12 pre-existing only.

### Honest limits

- `flatpak run` cannot execute in this container (nested pidns + proc mount
  denied — the same restriction blocks flatpak-builder's sandbox, which is
  why build-flatpak.sh has the low-level fallback). Launching the Flatpak on
  real Mint, and confirming the session-bus system-proxy writes reach the
  desktop, is Ali's manual check (README list, step 8).
- The AppStream branch (store metadata) is best-effort on the fallback path
  (Debian's appstreamcli rejects the icons with a policy catch-22, D55);
  the flatpak-builder path regenerates it on a normal host. Bundle installs
  and runs identically without it.
- The deb/AppImage/flatpak wrappers embed build timestamps, so only the
  inner binaries are byte-reproducible, not the container files.

### Release v0.5.1 record

- Commit 979dcbf (tag phase-12) pushed to main.
- Release live: https://github.com/assadigit/Younblock/releases/tag/v0.5.1
  with eleven assets, all round-trip re-downloaded and SHA-256 identical:
  - younblock.exe 12.76 MB (e498d7dd11ea…)
  - younblock-x86.exe 12.18 MB (c0157b14df70…)
  - younblock-core.exe 11.02 MB (ee9d0c505e54…)
  - younblock-core-x86.exe 10.60 MB (0b21b0950a29…)
  - younblock-linux-amd64 8.38 MB (07ea63735fe9…)
  - younblock-linux-386 8.12 MB (a37e8643602f…)
  - younblock_1.4.1_amd64.deb 3.04 MB (7125c1edc3d1…)
  - younblock_1.4.1_i386.deb 2.94 MB (4ef9814fd478…)
  - Younblock-1.4.1-x86_64.AppImage 4.04 MB (c1bc8f6b8a3a…)
  - Younblock-1.4.1-i686.AppImage 3.96 MB (631068cddacc…)
  - Younblock-1.4.1-x86_64.flatpak 3.05 MB (0987e40f37de…)
- Upgrade notes in the release body; Ali's Mint manual check list in README
  (steps 7–8 cover the packages, including the one honest manual check this
  format needs).

## Phase 13 — the look & feel pass (v0.6.0 / app 1.5.0)

Date: 2026-10-15 (Asia/Tehran). Scope locked up front: visual/interaction
layer only, no engine or network changes; the logo palette (D47), the
single-button personality and the plain-language copy tone stay.

Sandbox restored after the reset: repo cloned from GitHub (token in the
origin URL), Go 1.27.2 to ~/sdk/go, wails v2.16.0, gh 2.102.0, frontend
npm ci --legacy-peer-deps (the documented vite-peer-range escape hatch).
Baseline re-verified before touching anything: npm run build 94.40 KB JS /
29.32 KB CSS, svelte-check 12 known errors, go vet clean, go test -race
all green — identical to the v0.5.1 tree.

The audit (all screens, six states, components, tokens, i18n) found:
three drifted copies of the view chrome; a :focus-visible rule that set
border-radius on focused elements (squaring pill buttons); chips at
36 px and the copy button at 32 px (under the 44 px rule); native
accent-color radios beside the custom switch; the accordion chevron as
a text glyph "›" while Settings' advanced chevron never rotated; zero
entrance/reveal motion anywhere except the power button; subtitleFor()
hard-coding English while en.json carried an unused twin table; "Open
help" hard-coded in StatusBlock; an empty stopping subtitle; a size-0
Icon hack in About's diagram; the PowerButton hover rule graying the
lit disc; FirstRun as a plain left-aligned text block.

Implemented (D56–D59, full detail in DECISIONS.md): shared views.css
chrome + token ramp extensions; the control standard (brand radios,
brand checkbox, 44 px targets, focus fix, hover fix); the motion system
(screen entrances, FirstRun stagger, menu unfold, close-notice rise,
grid-rows reveals for Advanced/licenses/accordion, Run-test spinner +
Checking row + check pop — all D45-gated); the screen passes (FirstRun
welcome, About diagram-led card + version pill, degraded button-step
shrink) and the copy wiring (subtitleFor reads the deck, stopping
subtitle, two hard-coded strings moved to keys). Version 1.5.0 in
defaults.go / wails.json / mock.ts.

Two real defects surfaced during verification and were fixed:

- **The fill-mode stacking bug** (found by agent-browser E2E): clicking
  Settings in the open menu failed — "element covered by section.connect".
  Diagnosis: `animation-fill-mode: both` kept a filled identity-matrix
  transform on the header (`matrix(1,0,0,1,0,0)` per getComputedStyle),
  and any transform — even identity — creates a stacking context; the
  menu panel's z-index 20 was scoped inside the header's context, and
  the connect section (later in paint order) covered the open panel.
  Fix: every one-shot animation fills `backwards` only (identical end
  state, no lingering transform). Verified: the click point resolves
  inside the panel, the whole golden path passes.
- **The About diagram below the fold** (found by VLM QA, confirmed by
  DOM measurement: diagramTop 560 == viewport bottom): reordered to lead
  the card (diagramTop 224, fully visible).

Verification gates run:
- `npm run build` — 96.45 KB JS / 41.08 KB CSS (grew ~2 KB for the
  shared chrome + new components' styles).
- svelte-check — 12 errors, exactly the pre-existing runes/tsconfig
  quirk + Android stub; nothing new.
- `go vet ./...` clean; `go test -race ./...` all 14 packages green.
- Screenshots re-shot at 360×560 (six states, notice, first-run, menu,
  settings ×3, help ×4, about ×2) plus 320×568 and 480×900 on-state,
  from the production bundle via vite preview + agent-browser. VLM QA:
  all PASS. Two "severe overlap" claims in Settings were disproved by
  pixel measurement (title→hint gaps 2 px positive, rows stack cleanly)
  and a 2× zoomed re-check returned PASS — the hints did gain 4 px of
  air and 17 px line-height anyway. One true finding (the About fold)
  fixed and re-verified.
- agent-browser E2E on the running bundle: golden path (press →
  connecting → on → press → off, state labels confirmed via DOM), every
  screen reached by real clicks, `agent-browser errors` and `console`
  empty at the end of the session.
- D48 layout budget: docH == viewport at 360×560, 336×522 and 288×448
  in off/on/degraded/error; degraded's fix-to-footer gap 15 → 27 px at
  560 (was exactly on the minimum); the 288×448 floor re-measured
  against the v0.5.1 baseline (git stash) — identical, the ladder +
  scroll net unchanged.
- Cross-builds: ./build.sh cut all six binaries and the four packages
  from the one tree (logs in the release section below).

### Release verification (v0.6.0 artifacts, from this tree)

- ./build.sh cut all six binaries + the two .debs + the two AppImages in
  one run; the Flatpak via packaging/flatpak/build-flatpak.sh after the
  sandbox's flatpak stack was restored user-space (apt-get download +
  dpkg-deb -x: flatpak 1.16.6, flatpak-builder 1.4.4, ostree; GNOME 48
  Platform+SDK installed --user from flathub; FLATPAK_SYSTEM_DIR pointed
  at the user installation for the build commands).
- The fallback flatpak path needed two small host adaptations, both
  recorded here honestly: the compiled-in /usr/libexec/flatpak-validate-
  icon path does not exist in this container, so the user-space flatpak
  binary was patched to an equal-length copy under /home/z (the validator
  itself is the stock Debian one); and build-flatpak.sh now passes
  --disable-sandbox to build-export when bwrap is absent — the same
  conditional spirit as the existing --disable-rofiles-fuse /dev/fuse
  check, and a no-op on any real Mint (committed).
- Linux binary E2E (agent-browser against the live --web core 1.5.0):
  first-run → Get started → off; power button → real on (engine live);
  Settings shows the Linux copy + hides the Chrome-session row
  (capability gating); Help Run test — all four steps pass live; About;
  toggle off. Forced PORT_BUSY (fixed port + occupier listener) for the
  error state. `agent-browser errors`/`console` — empty across the whole
  session.
- .deb amd64: extracts cleanly, /usr/bin/younblock --version 1.5.0,
  desktop file Exec=/usr/bin/younblock --web, nine icon sizes,
  Depends/Recommends intact; i386 .deb binary is ELF i386.
- AppImage x86_64: through the real AppImage runtime
  (--appimage-extract-and-run): live control page, appinfo 1.5.0.
- Flatpak bundle: installed --user (dbus-run-session + a fake system bus
  for the parental-controls lookup + a bwrap passthrough shim — the
  openh264 extra-data step warns and is skipped, our app has no
  extra-data); deploy metadata correct (runtime org.gnome.Platform 48),
  /app/bin/younblock byte-identical to dist/younblock-linux-amd64
  (sha ef64037b…), desktop + 9 icons, live API 1.5.0, and the D54
  autostart entry written through FLATPAK_ID reads
  Exec=flatpak run io.github.assadigit.Younblock --web. `flatpak run`
  itself stays Ali's manual check on real Mint (same container limits
  as v0.5.1).
- check-platform-neutral OK; arm64 core builds; PE32+/PE32/ELF64/ELF32
  formats confirmed for every artifact.

## Release v0.6.0 — published to GitHub Releases

Date: 2026-10-15 (Asia/Tehran). Tag v0.6.0 on commit 56c1ed3; the same
tree had just passed every gate above (build, svelte-check, vet, race
tests, screenshot+VLM QA, agent-browser E2E on the mock bundle AND the
live Linux core, package verifications, cross-builds).

Published with all 11 assets: younblock.exe, younblock-x86.exe,
younblock-core.exe, younblock-core-x86.exe, younblock-linux-amd64,
younblock-linux-386, younblock_1.5.0_amd64.deb, younblock_1.5.0_i386.deb,
Younblock-1.5.0-x86_64.AppImage, Younblock-1.5.0-i686.AppImage,
Younblock-1.5.0-x86_64.flatpak. Round-trip verified: every asset
downloaded back from the release page is byte-identical (sha256) to the
built file — 11/11 MATCH. Release notes in the project's plain-language
voice; CHANGELOG retitled to [v0.6.0] with the compare links updated.

## Phase 14 — the new connect screen (v0.7.0 / app 1.6.0)

Ali's design package `younblock-ui` (uploaded to the repo root) replaced
the app's whole face. Integration followed the package's own
agent-instructions (commit 5396191), and this release run closed the one
gap the package left, cleaned the leftovers and shipped it.

- Frontend: the package's `App.svelte`, `tokens.css`, `app.css` and the
  seven `lib/` components; Sora Variable installed; `index.html` viewport
  set to `width=device-width, initial-scale=1, viewport-fit=cover`; the
  app CSP allows inline styles (vite dev injection + the screen's own
  styles); `@sveltejs/vite-plugin-svelte` ^7.3.1 for the vite 8 peer
  range. The old screens, components, i18n layer and per-edition
  adapters are gone — recoverable at tag v0.6.0 (D60).
- Go: `ui_bridge.go` (the package's state bridge) with its 330-line test
  file; `engine_adapter.go` wraps the existing engine without touching
  proxy logic (Start polls `GetState()` until the async connect settles:
  on/degraded = ready, error = failed, off = canceled elsewhere); App
  gains `Connect` / `Disconnect` / `GetSnapshot`; the tray Toggle and
  connect-on-launch route through the bridge; the window is 360×760 with
  the D48 client-size fix applied to the new height.
- The gap (D61): the package shipped Wails + offline-preview transports
  only, so the Linux single-file editions would have served a screen
  whose button did nothing real. The localhost JSON + Server-Sent-Events
  transport was rebuilt inside the screen's one seam (`backend.ts`),
  with the bridge's exact state mapping and a once-per-second state
  re-read while connected (the engine only pushes state *changes*; the
  ping lives in the state).

### Verification (v0.7.0 tree)

- `go vet ./...` clean; `go test -race ./...` — every package passes,
  including the new bridge tests.
- `svelte-check` 0 errors / 0 warnings; `vite build` clean.
- Live Linux E2E (agent-browser against the live `--web` core 1.6.0):
  fresh page → "Not Connected"; power pill → real connect (engine `on`,
  port 4525) with the timer ticking, "Ping 79 ms" live from the state
  poll and the speeds row fed by stats events; disconnect → "Not
  Connected", engine `off`. Forced a genuine failure the honest way —
  `autoPort` off plus an occupier on 4525 → engine `error`
  (PORT_BUSY) → the screen shows "Can't Connect" + "Could not connect.
  Try again.", and the retry connects after the port is freed. (An
  earlier attempt to force failure by holding the port with autoPort on
  taught us the engine just takes 4526+ — AutoPort works as designed;
  a dead DoH server is likewise healed around. Both noted here so the
  next person doesn't re-learn it.)
- Preview fallback (plain vite dev server, no engine): fake connect for
  browser development, and `?fail=1` exercises the same error screen.
- `agent-browser errors` / `console` — zero errors and zero warnings
  across the whole live session.
- The design-package leftovers were removed from the repo root (D60):
  `agent-instructions.md`, `design.md`, `design-report.md`, the uploaded
  `App.svelte`, `reference-vs-younblock.png` and the stray one-byte
  `younblock-ui` marker. The integrated copies under `frontend/src/`,
  and `ui_bridge.go` / `ui_bridge_test.go`, are the living ones.

## Release v0.7.0 — published to GitHub Releases

Date: 2026-10-15 (Asia/Tehran). Tag v0.7.0 on commit d0fd1fd; the same
tree passed every gate above (vet, race tests — the root package forced
to re-run after the version bump, svelte-check 0/0, production build,
live Linux E2E on the final 1.6.0 binary, preview fallback,
platform-neutral check).

Published with all 11 assets: younblock.exe, younblock-x86.exe,
younblock-core.exe, younblock-core-x86.exe, younblock-linux-amd64,
younblock-linux-386, younblock_1.6.0_amd64.deb, younblock_1.6.0_i386.deb,
Younblock-1.6.0-x86_64.AppImage, Younblock-1.6.0-i686.AppImage,
Younblock-1.6.0-x86_64.flatpak. Round-trip verified: every asset
downloaded back through the API is byte-identical (sha256) to the built
file — 11/11 MATCH. This time the SHA-256 list is actually in the
release notes (v0.6.0's notes pointed at checksums they never listed).

- Flatpak: the sandbox's flatpak stack restored user-space again
  (apt-get download + dpkg-deb -x: flatpak 1.16.6, flatpak-builder
  1.4.4, ostree, appstream + the libs; GNOME 48 Platform+SDK installed
  --user from flathub under dbus-run-session; the compiled-in
  validate-icon path patched to the equal-length /home/z/vali/ copy; a
  fake system bus for the install). Bundle built through the committed
  fallback path; installed --user: runtime org.gnome.Platform 48,
  /app/bin/younblock byte-identical to dist/younblock-linux-amd64,
  desktop + 9 icons, deployed binary reports 1.6.0. `flatpak run`
  itself stays Ali's manual check on real Mint (same container limits
  as v0.5.1/v0.6.0).
- .deb amd64 extracts cleanly, /usr/bin/younblock --version 1.6.0,
  Depends/Recommends intact, nine icon sizes; the i386 .deb binary is
  ELF i386. AppImage x86_64 runs through the real AppImage runtime
  (--appimage-extract-and-run): 1.6.0. The i686 AppImage cannot execute
  in this container (no 32-bit loader, same as v0.6.0) — format-checked
  only; it wraps the same-recipe build around the verified 32-bit
  binary.
- The Windows walkthrough (wails dev, the 8-point list) is still Ali's
  step, and the release notes say so plainly.

## Phase 15 — the one-click scripts (v0.7.1 / app 1.6.1)

Ali asked for a Windows script that "automatically installs and does
this" — the whole setup-and-run walkthrough in one double-click. The
app itself is unchanged from 1.6.0; this release is the tooling around
it (the v0.5.1 tradition: the tag moves, the app code does not).

- `setup-dev.bat` (shipped first, commit 4304773) — winget presence;
  install Git / Go / Node.js LTS when missing, each followed by an
  in-session PATH reload (PowerShell reads Machine+User PATH back from
  the registry, so a just-installed tool works with no "open a new
  window" step); Node >= 20 and Go >= 1.26 gates with plain fix-it
  notes; a WebView2 registry probe with a winget fallback (non-fatal —
  a warning, since neither dev nor build needs it to proceed); the
  Wails CLI via `go install ...wails@latest` with
  `%USERPROFILE%\go\bin` on PATH; clone or `git pull --ff-only`
  (GIT_TERMINAL_PROMPT=0, so a private-repo auth failure surfaces as
  an explained stop, not a hang); then `npm ci` on first run / quick
  `npm install` after, and `wails dev`.
- `setup-build.bat` — the same eight checks, then `build.bat` (all
  four Windows exes + their SHA-256) and Explorer opens on `dist/`.
  A missing build.bat is caught before the build starts.
- Both: pure ASCII, CRLF (enforced repo-wide by the `.gitattributes`
  rule `*.bat text eol=crlf`), `call` before every .cmd shim (npm,
  wails — the classic batch pitfall), balanced pushd/popd on every
  failure path, and a pause at the end so nothing closes before it is
  read (D62).
- README's build-it-yourself section now leads with the two scripts.

### Verification (v0.7.1 tree)

- `go vet ./...` clean; `go test -race -count=1 ./...` — every package
  passes; `scripts/check-platform-neutral` OK; `younblock-core
  --version` prints 1.6.1.
- `svelte-check` 0 errors / 0 warnings; `vite build` clean.
- The app code is byte-identical to the v0.7.0 release except the two
  version constants (defaults.go, wails.json) — the v0.7.0 Linux E2E
  and its 8-point Windows walkthrough carry over to this tree.

## Release v0.7.1 — published to GitHub Releases

Date: 2026-10-15 (Asia/Tehran). Tag v0.7.1 on commit 5991766; the same
tree passed every gate above (vet, race tests, platform-neutral check,
`--version` 1.6.1, svelte-check 0/0, production build). The app code is
byte-identical to v0.7.0's except the two version constants.

Published with 12 assets: the four Windows exes, the two Linux single
files, both .debs, both AppImages — and, new this time, the two
one-click scripts (`setup-dev.bat`, `setup-build.bat`) riding along as
release assets so a fresh PC needs nothing but a download and a
double-click (D62). Round-trip verified: every asset downloaded back
through the API is byte-identical (sha256) to the built file —
12/12 MATCH. The SHA-256 list is in the release notes.

- Spot checks: `younblock.exe` is PE32+ x86-64 (GUI) and
  `younblock-x86.exe` PE32 i386; the amd64 .deb carries Version 1.6.1
  and its wrapped /usr/bin/younblock reports 1.6.1; the i386 .deb
  binary is ELF i386; `dist/younblock-linux-amd64` reports 1.6.1.
- The Flatpak bundle was NOT rebuilt this time: the sandbox's flatpak
  stack (restored user-space for v0.5.1 / v0.6.0 / v0.7.0) was wiped by
  the sandbox reset, and re-restoring it (flatpak + flatpak-builder +
  ostree + the GNOME 48 runtimes — about a gigabyte of downloads, plus
  the validate-icon patch and the fake system bus) buys only a
  version-string change: the app code is identical to what the 1.6.0
  Flatpak on v0.7.0 already ships. The release notes say so plainly;
  the Flatpak catches up on the next release that touches it.
- The Windows walkthrough (wails dev, the 8-point list) is still Ali's
  step — `setup-dev.bat` now performs every step of the setup for it.

## Phase 16 — mirrors and the offline pack (v0.7.2 / app 1.6.2)

Ali reported the real-world failure: on a filtered network the setup
scripts die — the private repository needs a login, and the
dependencies sit on sites that are blocked (proxy.golang.org, go.dev).
The fix is mirror resilience plus a fully offline pack (D63, D64). The
app itself is unchanged from 1.6.0.

- `setup.ps1` — the whole brain now lives in one PowerShell file; the
  two .bat files became thin double-clickable launchers (D62 refined;
  PowerShell was already allowed inside, this just moves the logic
  there where try/catch and real string handling live). Mirror lists
  at the top are the only configuration: Go module proxies
  (proxy.golang.org / goproxy.io / goproxy.cn), Go toolchain zips
  (Aliyun / go.dev), Node.js zips (nodejs.org / npmmirror), npm
  registries (npmjs / npmmirror). Every step probes its mirrors (6 s
  request, curl.exe first, Invoke-WebRequest fallback) and uses the
  first that answers; winget is still tried first for Git / Go / Node
  / WebView2.
- The private repo gets three arrival paths: plain clone (GitHub's
  browser login), clone with `$GH_TOKEN` (an http extraheader — the
  token never lands in .git/config), and a zipball straight from the
  GitHub API when git cannot be installed (the script offers the
  token prompt there). GOPROXY is written once with
  `go env -w` so a later plain build.bat keeps the mirrors.
- `vendor/` — the Go dependencies are vendored into the repository
  (25 MB, 1102 files), so build/test never touch a module proxy; only
  the Wails CLI install still needs one (it is not importable, so it
  cannot be vendored — D64).
- `Younblock-offline` — the no-internet pack, assembled by the new
  `build-offline-pack.sh` at release time: the tagged source (with
  vendor/), a pre-installed Windows node_modules, the Go toolchain
  zip, the Node.js zip and a cross-compiled Wails CLI.
  `setup-offline.bat` unpacks the tools into
  %LOCALAPPDATA%\YounblockTools and runs; `setup-offline build`
  builds the exes instead. The `younblock-setup` zip carries the
  three online scripts together (a .bat downloaded alone would be a
  trap — its launcher refuses to run without setup.ps1 and says so).

### Verification (v0.7.2 tree)

- `go mod vendor`; `go build ./...` (linux), `GOOS=windows
  GOARCH=amd64 go build ./...` and `GOARCH=386` — all three pass
  against the vendor tree, proving the vendor is complete for every
  platform the release builds.
- `go vet ./...` clean; `go test -race -count=1 ./...` — every
  package passes against the vendor tree;
  `scripts/check-platform-neutral` OK; `younblock-core --version`
  prints 1.6.2; `svelte-check` 0 errors / 0 warnings; `vite build`
  clean.
- setup.ps1: parsed clean by PowerShell 7.4's AST parser (0 errors);
  smoke-run on Linux stops gracefully at step 1 (no winget) with the
  right message and exit code; its real `Test-Url` / `Download-File`
  functions (extracted via AST and executed) were run against every
  configured endpoint live: all three Go proxies, both Go zip
  mirrors, both Node zip mirrors, both npm registries answered True;
  the token zipball (11 MB) and the Aliyun Go zip (79 MB) downloaded
  through the actual function code.
- The zipball's inner layout (assadigit-Younblock-<sha>/) matches
  the extraction logic; `npm ci --os=win32 --cpu=x64` on the Linux
  host installs @rolldown/binding-win32-x64-msvc (the Windows native
  binding vite 8 needs) instead of the Linux one — verified by
  direct inspection of the installed tree.

## Release v0.7.2 — published to GitHub Releases

Date: 2026-10-15 (Asia/Tehran). Tag v0.7.2 on commit 22ae7d9 (vendor
commit 8d8aaf4 beneath it); the same tree passed every gate above.

Published with 12 assets: the four Windows exes, the two Linux single
files, both .debs, both AppImages — plus the two new bundles,
`younblock-setup_1.6.2.zip` (7.8 KB: setup-dev.bat, setup-build.bat,
setup.ps1, README-FIRST.txt) and `Younblock-offline_1.6.2.zip`
(178.8 MB: tagged source + vendor + Windows node_modules + the Go
toolchain + Node.js + the cross-compiled Wails CLI). Round-trip
verified: every asset downloaded back through the API is
byte-identical (sha256) to the built file — 12/12 MATCH. The
SHA-256 list is in the release notes. The individual .bat scripts
are deliberately no longer separate assets: they are launchers that
need setup.ps1 beside them, so the setup zip is the download (the
launcher itself explains this if it is missing its partner).

- Offline pack spot checks: setup-offline.bat at the zip root, the
  three tools present, vendor/modules.txt present, the Windows
  rolldown binding present and the Linux one absent, wails.exe is a
  PE binary (27.6 MB), every packed .bat/.ps1 carries CRLF only, and
  the setup zip holds exactly its four files.
- The Flatpak bundle stays the 1.6.0 one from v0.7.0 (unchanged app
  code); same reasoning as v0.7.1 — it catches up on the next
  release that touches it.
- Still Ali's to check: the Windows walkthrough (wails dev, the
  8-point list) — on a blocked network, the offline pack is now the
  zero-internet path to it.

## Phase 17 — the proportion pass (v0.7.3 / app 1.6.3)

Ali's brief: the 388×782 window was too tall for what it shows —
target ≈388×650 including the title bar, −15–20% height, keep every
other property of the design language.

- Audited the old 47.5rem vertical stack gap by gap before touching
  anything: top padding 3.1, header 2.5, gap 0.7, illustration 8.4,
  gap 1.5, status 5.25, gap 3.4, track 14.25, gap 4.475, hint ~1,
  bottom 2.9. The two clear wastes: the hint's dead zone below the
  control (4.475+2.9) and the generous top padding.
- New stage 22.5×39.25rem = 360×628 client (window ≈388×650 with the
  22px title bar — Ali's measured chrome on 782−760). main.go Height
  628; ensureDesignClientSize(360, 628) (D48 still guards the client
  area at any DPI); app.css root-font ladder now 100vh/39.25.
- Cut deliberately (D65 has the full table): header top 2.375rem,
  20px horizontal padding; play mark −15%; status 0.5rem closer; the
  control −12% (thumb height 110px, width and touch targets kept);
  OFF ring r 6.625rem; ON arcs 70–126px; hint 1.625rem under the
  track; chevrons recentered exactly in their now-shorter empty
  halves; the halftone texture's fade retuned for the shorter stage.
- PowerPill reads --travel from tokens.css on mount/resize — the drag
  math can no longer disagree with the CSS (the old code kept a
  second copy of 5.75rem in a comment-synced constant).
- Found and fixed while validating: MenuSheet's 20ms-delayed
  closeBtn.focus() scrolled the whole stage up ~320px on open (the
  closed sheet sits below the scaled stage and counts as scrollable
  overflow; the browser chased the still-hidden button) — now
  focus({preventScroll: true}). Also the About sheet's hard-coded
  "1.5.0" → 1.6.3, and its max-height cap rebased to the shorter
  stage (min(78%, 30.5rem)).
- Gates on the final tree: svelte-check 0/0; vite build clean;
  go vet clean; go test -race -count=1 all packages pass;
  check-platform-neutral OK; core --version reports 1.6.3.
- Browser E2E at exactly 360×628 (vite preview adapter): every
  element measured at its planned pixel (header 38, mark 90×163,
  status 220.4, track 350–550, thumb 356–466, hint 576; docH 628,
  no scrollbar); click-connect (spinner dead-centre in its well,
  CANCEL, then timer at 112 + speeds + ping); full-drag connect and
  short-drag snap-back; swipe-up disconnect; ArrowDown/ArrowUp/Enter;
  ?fail=1 error state with retry; menu sheet opens bottom-anchored
  140–628, scrolls inside, Escape + focus restore; zero console or
  page errors; VLM design review of all five states found no defect
  (its four notes were by-design affordances). Responsive ladder
  re-checked at 300×620: root font 13.33px, everything fits, no
  scroll.

## Release v0.7.3 — published to GitHub Releases

Date: 2026-10-16 (Asia/Tehran). Tag v0.7.3 on commit 2a64548; the
same tree passed every gate in Phase 17.

Published with 13 assets: the four Windows exes, the two Linux single
files, both .debs, both AppImages, the Flatpak bundle — plus the two
bundles from v0.7.2's policy rebuilt for this version,
`younblock-setup_1.6.3.zip` (7.7 KB) and `Younblock-offline_1.6.3.zip`
(171 MB). Round-trip verified: every asset downloaded back through
the API is byte-identical (sha256) to the built file — 13/13 MATCH.
The SHA-256 list is in the release notes.

- The Flatpak caught up as promised: the user-space stack was
  restored again (flatpak 1.16.6 / flatpak-builder 1.4.4 / ostree
  2025.2 via apt-get download + dpkg-deb -x; GNOME 48 Platform+SDK
  --user from flathub; the equal-length validate-icon path patch).
  Built through the committed fallback path; installed --user under
  dbus-run-session with the fake system bus + bwrap passthrough shim;
  deploy metadata runtime org.gnome.Platform 48, /app/bin/younblock
  byte-identical to dist/younblock-linux-amd64 (sha f507da36…),
  9 icons, desktop Exec correct, wrapped --version reports 1.6.3.
- Offline pack spot checks: tagged source at 1.6.3, vendor/modules.txt
  present, @rolldown/binding-win32-x64-msvc present and the Linux
  binding absent, tools/ holds the Go zip + Node zip + PE wails.exe,
  every packed .bat/.ps1 CRLF, setup zip exactly its four files.
- Still Ali's to check: the Windows look with the real title bar at
  the new 628 height (the 8-point walkthrough from v0.7.0, now on
  1.6.3's proportions).

## Phase 18 — two editions and the native Linux window (v0.7.4 / app 1.6.4)

The request had three parts: ship the simple UI alongside the new one
(the 8.5 MB exe was gone; the offline pack had grown to 171 MB), fix the
AppImage opening in a browser instead of as an app on Linux Mint, and
make the Linux line two editions as well.

**The classic edition (D66).** The v0.6.0 frontend tree was restored
verbatim from the git tag into `frontend/src/classic/` — every import in
that tree is relative, so it moved without touching a single path, and
its adapters talk to the current binding surface (a superset of
v0.6.0's). `src/main.ts` became a dispatcher: both screens build as
separate dynamic chunks, `?ui=classic|pro` overrides at runtime, the
baked default comes from `VITE_UI_VARIANT`. The Go shell sizes its
window from `-X main.windowSize` (pro 360×628, classic 360×760). The
classic tree's 12 "known" svelte-check errors were fixed rather than
tolerated: the `$state` runes false-positives turned out to be a local
variable literally named `state` colliding with the rune in svelte2tsx's
legacy-mode view (renamed to `uiState` — 8 occurrences), the android
adapter got the missing `onBrowserEvent`, and the two intentional
initial-capture patterns carry documented `svelte-ignore` comments.
svelte-check is 0/0 with both trees in the project.

**The native Linux window (D67).** The Linux editions had been the
headless core serving its page to the browser since D50 — which is
exactly what the user saw on Mint. The fix is a real Wails build for
linux/amd64 with `-tags webkit2_41`. The build sandbox (Debian 13, no
root, no webkit anywhere) got a user-space jammy stack:
`packaging/native/fetch-stack.sh` resolves the dependency closure of
libwebkit2gtk-4.1-dev/libgtk-3-dev/libsoup-3.0-dev from the jammy
package indexes (~200 packages after the noise filter), downloads the
debs, `dpkg -x` them into ~/.cache/younblock/webkitstack, rewrites the
.pc paths, merges jammy's legacy /lib payloads into /usr/lib, and
recreates the SONAME symlinks with `ldconfig -n`. The link needed
`-Wl,--allow-shlib-undefined` (the deep NEEDED tree resolves on the
target desktop, not at link time) and `-Wl,-rpath-link`. The result
verifies GLIBC_2.34 max — every Mint since 21. The .deb depends on the
system webkit (apt-matched, ~10 MB); the AppImage bundles the whole
webkit closure (NEEDED BFS minus the desktop-guaranteed set) plus bwrap
and xdg-dbus-proxy, ~75 MB. WebKit's helper processes live at a
compile-time /usr/lib path with no 2.50 override, so the bundled
libwebkit is binary-patched at build time to two fixed /tmp names that
AppRun symlinks to the extraction. Under Xvfb the whole chain was
verified as far as the sandbox allows: the bundle loads, the window
opens at the exact design size, and the helpers spawn through the
patched path — only the GL compositing itself cannot run in the
GL-less container. Shell-side, the Linux close button now quits (no
tray there; the engine restores first), the logs folder opens with
xdg-open, and the window carries the Younblock icon.

**Packaging matrix.** build.sh now produces, per platform, both
editions: 4 Windows GUI exes + 2 cores, 2 Linux web binaries, 3 debs
(native amd64 pro + classic, web i386), 3 AppImages (native x86_64 pro
+ classic, web i686). The 32-bit Linux line stays a browser edition —
old 32-bit Mint's glibc 2.31 is below the native floor. build.bat grew
the two classic exes. The Flatpak stayed at 1.6.3 (the sandbox lost
the flatpak stack again; the wrapped app code is unchanged) — disclosed
in the release notes, with a native flatpak on the list.

## Release v0.7.4 — published to GitHub Releases

Date: 2026-10-16 (Asia/Tehran). Tag v0.7.4 on commit 49ddfe9; the same
tree passed every gate (go vet, full -race suite, platform-neutral,
svelte-check 0/0 with both screens, vite build, Xvfb smoke of the native
window, live browser E2E of both screens against the real engine).

Published with 16 assets: the four Windows exes (pro + classic, 64/32
bit), the two cores, the two Linux web binaries, three .debs (native
amd64 pro + classic, web i386), three AppImages (native x86_64 pro +
classic at ~71 MB each with the bundled webkit, web i686 at 3.9 MB), the
setup zip and the offline pack (166 MB). Round-trip verified: every
asset downloaded back through the API is byte-identical (sha256) —
16/16 MATCH. The SHA-256 list is in the release notes.

- The Flatpak was not rebuilt (the sandbox lost the flatpak stack
  again); it stays at 1.6.3 and the release notes say so.
- Live E2E covered the classic screen for the first time since v0.6.0:
  first-run welcome → connect screen reflecting the live engine state →
  menu → Settings/About → disconnect via the switch; the pro screen's
  connect toggle and states re-verified; zero console/page errors on
  both; VLM review of the screenshots found no defect.
- The native window path was verified as far as a GL-less container
  allows (loader, geometry, helper spawning); the GL compositing itself
  needs real hardware — Ali's Linux Mint is the final visual check,
  alongside the standing Windows walkthrough.

## Phase 19 — offline means offline (v0.7.5 / app 1.6.5)

The bug report was the offline pack failing on a real machine:
`'powershell' is not recognized` (a noisy but harmless line) followed
by the real killer — `wails dev` printing a wall of `go: downloading`
lines and dying with `403 Forbidden` from proxy.golang.org before the
first compile. Root cause: wails runs `go mod tidy` unconditionally
before every dev/build, and tidy does not read `vendor/` — it
resolves the module graph through GOPROXY, so the "no internet
needed" pack needed the internet after all.

**The fix (D68).** Every wails invocation in the repo's scripts now
carries `-m`, wails' own SkipModTidy flag (`wails dev -help`: "Skip
mod tidy before compile"): setup-offline.bat's dev launch, all four
builds in build.bat, all six in build.sh, setup.ps1's dev mode. With
tidy skipped the flow goes straight to `go build`, which uses
`vendor/` automatically; the pack stages source and vendor from the
same `git archive` of the tag, so consistency is structural. The
PowerShell line now calls `%SystemRoot%\System32\WindowsPowerShell\
v1.0\powershell.exe` by full path (some machines have a PATH without
it), output silenced, errorlevel never checked — a convenience step
must not be able to stop a setup. The stale "360 x 760" startup text
became 360 x 628 (the pro window since v0.7.3).

**Proof it is offline now.** Simulated the user's machine exactly:
empty GOMODCACHE, `GOPROXY=off` (any network access would abort),
`GOTOOLCHAIN=local` — `wails build -m -platform windows/amd64`
compiled the GUI exe in 4.5 s. The same `-m` gate covers `wails dev`
(one shared code path, `cmd/wails/internal/dev/dev.go:63`), and the
npm side was already covered by the package.json.md5 skip marker
shipped in the pack.

**A build-pipeline bug found on the way.** build.sh's web-edition
packaging calls pass the package name as an extra argument, but
build-deb.sh and build-appimage.sh accepted only the bare 3-arg web
form — so the committed tree could not build the i386 deb or the i686
AppImage at all (the v0.7.4 ones had been built with an uncommitted
local fix that a sandbox reset then lost; caught when this release's
build stopped at the same lines). Both scripts now accept 3 or 4 args
in web mode, the name defaulting to "younblock" — the committed tree
builds every asset by itself again.

Gates on the bumped tree: go vet clean; the full `-race` suite passes
every package (with the webkit stack env for the cgo main package);
check-platform-neutral OK; svelte-check 0/0; vite build clean; the
core reports 1.6.5. App code is byte-identical to v0.7.4 except the
two version constants and the scripts, so the v0.7.4 live E2E carries
over.

**Release record — v0.7.5 (app 1.6.5).** Commit ef26a3a + annotated
tag v0.7.5 pushed. build.sh produced the 14 platform builds at 1.6.5
(4 Windows exes, 2 headless cores, 2 Linux web binaries, 3 debs,
3 AppImages; flatpak skipped — tools gone again, stays 1.6.3,
disclosed) and build-offline-pack.sh the setup zip (7.7 KB) plus the
offline pack (166 MB → 174 MB as released; contents re-verified in the
zip: fixed setup-offline.bat with `wails dev -m` and the full-path
PowerShell, vendor/, tools/, the win32 rolldown binding). GitHub
release id 414466717 "Younblock 1.6.5 — offline means offline"
published with 16 assets and the real SHA-256 list in the notes;
round-trip verified 16/16 byte-identical via the asset API; v0.7.5
confirmed latest. The release notes also carry the one-line workaround
for owners of the v0.7.4 pack (`call wails dev` → `call wails dev -m`
in Notepad) so nobody has to re-download 174 MB to get unstuck.

## Phase 20 — the feel pass (UI/UX, post-v0.7.5, unreleased)

Ali's brief this session was an open-ended UI/UX pass (the goals list
arrived unfilled, so the work was chosen from the repo's own evidence:
a frame-by-frame review of the connect flip, the VLM gate history, and
the two defects found on the way).

**What the evidence said before anything was touched.** A recorded
connect flow, extracted at 30 fps and reviewed frame by frame, confirmed
three rough edges in the v1.6.5 choreography: the dotted rings POPPED
(the single red ring vanished and the five green arcs appeared in the
same frame), the status word snapped between states, and the timer block
appeared with a jolt next to them. Plus one plain bug found by reading:
MenuSheet's About section still said "Younblock 1.6.3" in a 1.6.5 app —
the exact staleness class fixed once before (v0.7.3's hard-coded "1.5.0"),
regressed at v0.7.4 because the number was typed into the frontend again.

**The version fix (structural, not another bump).** `getAppInfo()` joined
the Backend interface: the Wails adapter calls the existing `GetAppInfo`
binding (same JSON the classic screen already parses), the HTTP adapter
fetches `/api/appinfo` once and caches, and the preview adapter returns
null — an honest "no host here", which makes MenuSheet hide the version
line instead of inventing a number. The one copy of the version now lives
where `--version` lives (internal/config). Verified in the browser that
the line is gone in preview mode; the Wails/HTTP paths carry the classic
screen's proven contract.

**The choreography.** DotRings was restructured so both tone sets are
always mounted: the off ring and the five on arcs cross-fade through
250 ms (`--dur-swap`) while the family's cy transition (700 ms) carries
them to the new center — no DOM swap, no timer, pure CSS. Each ring is
two groups (outer owns the fade opacity, inner owns the pulse animation,
so they compose instead of fighting); the fill moved from the shared
`--ring-dot` tone token to per-set classes so the outgoing color stays
correct while it dissolves. The five arcs pulse as an outward ripple
(staggered by 0.45 s, negative delays around the middle ring so the wave
is already running at mount — replacing a per-dot `animation-delay` that
had been dead on arrival since the component was written). The status
word is a `{#key phase}` span that rises in over 320 ms (`--dur-word`).

**The micro-interactions.** PowerPill gained `pressed` (scale 0.98 under
the pointer, inline with the drag transform so the 1:1 drag tracking is
untouched) and `armed` — a drag past the halfway threshold now previews
the release: the LED lights green while a connect is armed, dims back to
resting red while a disconnect is (the first cut of this had a bug the
E2E caught: while dragging up from connected the phase is still
"connected", so the lit class stuck — the condition is now
`armed ? !on : phase === 'connected'`), and the chevron in the direction
of travel goes white, gains a glow and pauses its new idle nudge (a
2 px breathe at the ring-pulse cadence). The error detail line moved to
`--red-400` (the only red approved for text), and MenuSheet became a
real modal: Tab and Shift+Tab now wrap inside the sheet instead of
leaking to the dimmed screen behind it.

**Verification (all on the final tree).** svelte-check 0/0; vite build
clean; no Go file touched (go vet/test vacuous this phase — disclosed).
Browser E2E at exactly 360×628 through the vite preview adapter: click
connect → connecting → connected; cancel mid-connect; retry from the
error state; full-drag connect with the armed state asserted mid-drag
(LED lit + chevron armed at +40 px); drag-up disconnect with the dim-LED
armed asserted; short-drag snap-back (thumb returns to rest, state
unchanged); keyboard ArrowDown/ArrowUp/Enter (Escape correctly does
nothing while connected); menu opens, focus lands on Close, Tab and
Shift+Tab wrap, Escape closes and focus returns to the menu button; the
version line absent in preview; zero console or page errors throughout.
The three transition claims were verified by measurement, not eyeballing:
computed-style polling through a live flip shows the ring cross-fade
running 1→0 / 0→1 over ~250 ms and the timer block 0→1 with its rise
over ~400 ms (two VLM "pop" judgments on sampled frames were refuted by
these numbers); the word rise-in was VLM-confirmed on dense 33 ms frames.
prefers-reduced-motion emulation: every new animation collapses to
0.01 ms and connect still works. Responsive ladder re-checked at
300×620 (root font 13.33 px, doc height = viewport, no scroll). The
classic screen (`?ui=classic`) loads and runs clean — untouched, but
sanity-checked because both screens share the dispatcher. VLM design
review of the five states: its notes were each checked and came out
by-design or refuted by measurement (spinner offset 0,0; word right
edge 277.7 < 360; lowest arc dot 47.5 px above the stage bottom; the
sheet scrolls by design, 810 px of content in a 429 px well).

Not done this phase, deliberately: no version bump and no release (the
work sits in CHANGELOG's [Unreleased]; the README's "What's new" gets
its entry at release time), and the classic screen stays frozen (D66).

## Release v0.7.6 — published to GitHub Releases

Date: 2026-10-16 (Asia/Tehran). The feel pass (Phase 20) promoted from
[Unreleased] to v0.7.6 / app 1.6.6: defaults.go and wails.json bumped,
the CHANGELOG section dated, the README "What's new in version 1.6.6"
written. Gates re-run on the release tree: svelte-check 0/0, vite
build clean, go vet, the full -race suite green, and the built core
reports "Younblock core 1.6.6".

Three commits: a7a676b (the release: bump + changelog + README),
99feb31 (executable bits back on fetch-stack.sh and
build-offline-pack.sh — without them a fresh clone silently skipped
the native Linux builds), c130ef1 (the WebView2 bootstrapper vendored
— D70). The annotated tag v0.7.6 moved with the work and finally
points at c130ef1, the exact tree every artifact below was built from;
nothing public ever depended on an earlier position (the release and
its assets were created last, from the final tree).

Published with 16 assets: the four Windows exes (pro + classic, 64/32
bit — byte-identical across all rebuilds: wails does not vcs-stamp),
the two cores, the two Linux web binaries, three .debs (native amd64
pro + classic, web i386), three AppImages (native x86_64 pro + classic
at ~71 MB with the bundled webkit, web i686 at 3.8 MB), the setup zip
(7.7 KB) and the offline pack (168 MB — now carrying the WebView2
bootstrapper in vendor/ at 1.75 MB, verified inside the zip alongside
vendor/modules.txt, tools/wails.exe and the win32 rolldown binding).
Round-trip verified: every asset's GitHub-side sha256 digest matches
the built file — 16/16 MATCH, and v0.7.6 confirmed latest. The
SHA-256 list is in the release notes (release id 414970242).

- The Flatpak was not rebuilt (flatpak/flatpak-builder missing from
  this sandbox again); it stays at 1.6.3 and the release notes say so.
- Also disclosed in the notes: the v0.7.4 and v0.7.5 offline packs
  could not build the Windows apps (the missing bootstrapper, D70) —
  owners of those packs hit `no matching files found`; this release's
  pack fixes it.
- This sandbox killed every background process the moment its Bash
  command ended (verified with a marker process — nohup/setsid change
  nothing), so the builds ran as foreground chunks; the jammy webkit
  stack (211 packages), the Windows toolchain zips and the go/npm
  caches carried the progress between chunks. proxy.golang.org was
  reachable this session (the wails CLI, then the bootstrapper, came
  out of its module cache).
- The Windows and Linux-native visual walkthroughs remain Ali's, as
  always.

## Phase 21 — Android port, Phase 1: audit and architecture (docs only)

Ali uploaded the Android specification pack (Younblock-Android-Specs.zip
→ docs/android-port/, an 11-file planning baseline prepared without
repository access) and asked for a safe wrap-up of the release session
plus continuation from the specs. The pack itself demanded Phase 1
first: source audit, feasibility validation, and the architecture
decisions — documentation only, no app code, no APK.

Wrap-up first: the session opened with 1531 files showing "modified" —
diagnosed as sandbox-rehydration mode flips (every file blob-hashed
against the index: zero content changes), silenced with a local
core.fileMode=false, nothing discarded. Fast-forwarded main to Ali's
36273fb and extracted the zip.

The audit (doc 01 completed): the full Connect→engine→relay→DNS/TLS→
OS-proxy→status trace with symbols — the relay is a loopback HTTP proxy
(CONNECT + forward + PAC + health), the engine's phase machine gates
"on" behind real self-tests, the DoH resolver and fragmentation are
pure and portable, and the whole OS surface sits behind engine.Platform.
Two finds that shaped everything: netx.Dialer's Control hook was
written years ago *for* VpnService.protect(fd) (dialer.go says so), and
the repo carries Ali's own prior Android plan (root 04-android-spec.md)
locking "one engine, one UI".

Feasibility (doc 03): the entire module compiles for GOOS=android on
all four ABIs (arm64 links pure-Go; the others need NDK cgo only at
final link); gomobile/x/mobile, tun2socks, gVisor netstack, hev and
Intra are all permissive-licensed and actively maintained (fetched
2026-10-16); Android 14+ FGS "connectedDevice" type requirements
confirmed from current docs. Chrome Android has no user proxy setting
and Android exposes no app-settable system proxy → VpnService with a
per-app allow-list is the only path that meets the product goal; the
browser-proxy mode survives as a documented Firefox-only fallback.

Architecture accepted (doc 08, ADR-0003/0004/0005/0008-0012):
VpnService + per-app allow-list (Chrome + YouTube app default) +
userspace stack (tun2socks primary, gVisor-direct alternative, gated by
M4 measurements) + a new SOCKS5 front door feeding the existing tunnel
code + gomobile AAR over the engine's string JSON API + the existing
Svelte bundle in a WebView served by the in-process webui server (the
Linux web edition's transport, zero frontend changes — a simplification
over the old spec's JS bridge that D50 made possible). DNS: A answered
through DoH, AAAA empty (IPv4-only, fail-safe v6 routing), other UDP
dropped — all disclosed.

Baseline recorded green on the untouched tree: go vet clean, go test
13/13 packages, svelte-check 0/0, vite build clean. Docs 02/05/06/07/09
and the pack README were completed to match; the roadmap now shows
M0-M2 done and M3 next. No application code changed — Phase 1 was
documentation-only, as the pack requires. Environment blocker recorded
honestly: this sandbox has no Android SDK/NDK/javac, so M3's AAR build
cannot run here; the Go-side M3 items (platform adapter, SOCKS5 door,
mobile facade) are host-testable without it.

## Phase 22 — Android port, Phase 2 M3: the build lane is real (toolchain, doors, bridge, APK)

The session opened from a fresh clone: the sandbox rehydrated without the
repository AND without the worklog (Tasks 1–3 lost) — reconciled by
re-cloning main @ e2986ba, reinstalling Go 1.27.2 to ~/sdk/go (D13's
convention), and re-running the full desktop baseline BEFORE any code
change (vet clean, 13+1 packages ok, svelte-check 0/0, vite clean).

M3 task 1 — the toolchain, previously the standing blocker, is now
INSTALLED and verified in use: JDK 21.0.8 with javac, Android SDK
(cmdline-tools 12.0, platform-tools, platforms 26+36, build-tools
36.1.0), NDK 28.2.13676358 (clang 19), gomobile + gobind via a
`go get -tool` directive. dl.google.com, services.gradle.org,
repo1.maven.org and proxy.golang.org were all reachable this session.

The Go-side additions (all host-tested, race-clean, desktop-gated):

- `internal/platform/android` — the platform adapter as pure Go (no build
  tag): capabilities {perAppRouting}, the protect hook as a ControlFunc
  (nil-protector = desktop behavior), the notice callback, no-op
  OS-proxy surface. Its tests include the netx-level ADR-0011 proofs:
  every dial (success, refusal, multi-address) calls protect exactly
  once, and a refused protect aborts the dial. The `select` wiring
  needed a spec correction — GOOS=android implies the `linux` tag, so
  select_linux.go became `linux && !android` (D71); the audit's claim
  that android fell through to the stub was wrong.
- `internal/proxy/socks5.go` — the SOCKS5 front door: CONNECT-only,
  no-auth, loopback-enforced, IPv4/domain/IPv6, feeding the same
  tunnel.go path as the HTTP door (same resolve, same netx dial, same
  first-record fragmentation rules). Ten test functions cover the
  address matrix, the fuzz-ish edges, failure mapping (0x04 resolve /
  0x01 dial), the shared MaxConns cap, fragmentation through the door
  (routed ≥2 reads / unrouted 1) and Stop-with-Server.
- `mobile/` — the gomobile facade: NewEngine(configDir, protector, sink)
  with nil-tolerant doors, the ten jsonapi methods verbatim, Shutdown,
  and StartWebUI/StopWebUI with a sink fanout so webui.New cannot
  silently disconnect the bridge (D72 — a host test pins that exact
  regression). The engine-level ADR-0011 proof runs a real connect
  sequence and connection test against local fakes: ≥2 protect calls
  after connecting, ≥3 after the test, phase `on` with a real latency.
  webui gained additive Events()/Close() (a real Close: the old
  listener-close left keep-alive pages half-alive — found by a failing
  test, fixed in webui).

The build lane: `gomobile bind` (GOFLAGS=-mod=mod; D73) produced
younblock.aar — 3 ABIs, classes verified by javap — and the new
`android/` Gradle shell (Gradle 9.8.1 wrapper committed, AGP 9.2.0 with
built-in Kotlin, minSdk 26/target 36, loopback-only WebView, network
security config for cleartext 127.0.0.1, the doc 07 §6 permission
skeleton with ACCESS_NETWORK_STATE deliberately omitted — D74)
assembled `app-debug.apk` (27,172,909 B), verified end-to-end: badging
(package io.github.assadigit.younblock 1.6.6, minSdk 26, targetSdk 36,
launchable MainActivity), native-code arm64-v8a/armeabi-v7a/x86_64,
apksigner verify (debug cert), SHA-256
6db3f44d3c4af068707500ea1b5fb11d15fec9ca61cde6d3a685a2e77565ca77.
`build-android.sh` encodes the whole lane; the AAR is never committed.

The honest ⛔: this sandbox has no /dev/kvm (verified) and no device, so
the launch half of M3's exit criteria and the Kotlin-level bridge call
could not be RUN — everything up to that point is verified evidence,
and the blocker is recorded in docs 05/09 for M4. Desktop regression
after every shared-tree change and once more at the end: vet clean,
16/16 packages ok, svelte-check 0/0, vite clean, platform-neutral OK,
windows×2 + linux + android cross-builds OK.

## M4 — the routing path (2026-10-16, host-complete)

The session opened after another sandbox reset: the working tree had
survived at `main @ 1d20460` with the M4 stack work in progress and
uncommitted (internal/tunstack, gVisor in go.mod), but the toolchain,
module cache and the worklog were gone. Recovery before any new code:
Go 1.27.2 to ~/sdk/go (aliyun mirror), module cache via
proxy.golang.org, `go mod vendor` (gVisor +6 MB — D68: vendor means
vendor), the frontend bundle synced into internal/webui/dist (D74),
and the desktop baseline re-run green BEFORE touching code.

The uncommitted stack work was reviewed and completed rather than
rewritten: one compile bug (a test discarded the echo origin it later
asserted against), a leftover debug file (deleted), and a flaky
TestStopWhileActive — root-caused, not retried away: buffered echo
bytes can drain after Stop (a nil read is not a survived flow), while
the flow stall that follows is correct TCP-over-a-dead-link behavior
(the harness has no kernel to reset the flow; on a device the TUN
interface teardown does it). The test now asserts what the stack owns:
no traffic path survives Stop (a post-Stop write must never echo
back), 5/5 stable.

internal/tunstack is gVisor netstack direct (ADR-0008's switch path,
resolved at design level — D75: the tun2socks engine package is a CLI
global singleton whose UDP model demands a UDP-capable proxy; it
cannot talk to our CONNECT-only door at all). Wire = channel endpoint
over one-packet-per-Read/Write TUN semantics (fdbased is the
documented same-seam upgrade); handlers before NIC; spoofing +
promiscuous; IPv4-only by construction with v6 dropped at the wire
(ADR-0010). In-tunnel policy (ADR-0009), all host-proven: TCP flows
enter the SOCKS5 door with fake-pool destinations REVERSED to their
domains (ATYP 0x03) so the door's MatchAnyDomain fragmentation matches
the browser path exactly; literal IPs cross honestly; unmapped fakes
RST; UDP:53 answered by the virtual DNS from the 198.18.0.0/15 pool
(any destination address — hardcoded-DNS apps included), AAAA empty,
other types empty; other UDP dropped fail-fast. Ten tests, including
the full through-path against the REAL proxy Server: phone netstack →
synthetic packet-preserving wire → stack → door → resolver → netx →
echo origin.

The facade's TUN lifecycle (mobile/tun.go): StartTun requires the
engine serving and opens the door on the running server; StopTun is
idempotent and closes the door it opened (same-server check across
engine restarts); TunStats is diagnostics-only JSON; Shutdown always
unwinds a live stack; a second StartTun on a live stack refuses
TUN_ACTIVE loudly. The host tests drive the REAL engine over a
SOCK_SEQPACKET socketpair (packet-preserving like an IFF_NO_PI TUN
fd): a DNS round-trip through the facade-built stack, then a
hand-rolled SOCKS5 CONNECT through the engine's own door — resolving
via the engine's (fake) DoH, dialing through the protect()ed netx
dialer, echoing payload — plus gates, idempotency and fd-consumption
proofs. The suite caught two real bugs, now D76: (1) os.NewFile only
netpoller-registers a foreign fd when O_NONBLOCK is already set — a
blocking fd made StopTun deadlock forever (no Close can unwind a
blocked read(2)); the facade sets O_NONBLOCK first (the production
tun2socks pattern) with build-tagged helpers so Windows still
compiles; (2) fd ownership is single-owner: StartTun consumes the fd
on EVERY return, so Kotlin never double-closes (the refused-fd
EPIPE test proves it).

The Kotlin service: YounblockVpnService (foreground, connectedDevice)
— per-app allow-list (Chrome+YouTube; fail-closed when none is
installed: zero allowed apps would route EVERYTHING), virtual DNS
10.111.0.53, v4 default route + v6 blackhole route, MTU 1500,
VpnProtector attaching the service as the engine's protect hook
(ADR-0011: dials protected while the TUN lives, honestly unprotected
when it does not), engine-state-driven bind/unbind, onRevoke =
disconnect, an honest low-importance notification. Bridge.kt holds
the process-wide EngineEvents fan-out + VpnProtector; MainActivity
wires the connect flow (page toggle → engine connecting → prepare()
consent → service start → TUN on serving). Manifest carries the
service (BIND_VPN_SERVICE, exported=false, connectedDevice type).

Build lane, re-verified from the reinstalled toolchain (JDK 21.0.12.1,
cmdline-tools 12.0, platforms 26+36, build-tools 36.1.0, NDK
28.2.13676358, gomobile/gobind go-install'ed at the pinned x/mobile
version): gomobile bind → younblock.aar 24,893,234 B (3 ABIs; the
netstack added ~7 MB); assembleDebug → app-debug.apk 34,707,469 B,
SHA-256 8e9f702a364f1e41a0bf12fa3a89971fced6e993632b56f74b44a55d06c8219b,
badging minSdk 26/target 36, YounblockVpnService in the dumped
manifest, apksigner verify OK (debug cert). The Kotlin compile
validates every generated binding the shell calls (startTun/stopTun/
getState).

The honest ⛔, unchanged and re-verified: no /dev/kvm, no device —
M4's on-device end-to-end gate (real DoH through the stack, QUIC
fallback on the real YouTube app, protect() on real sockets, the
notification/consent flow live) could not run; it is carried in docs
05/06/09 and folded into M5's device checklist, never claimed. Desktop
regression green at the end: vet clean; go test -count=1 ./...
17/17 packages ok (tunstack 4.7 s, mobile 9.8 s); svelte-check 0/0;
vite build clean; Android library compile matrix 4/4 ABIs with
-mod=vendor.

## M5 — the connection lifecycle (2026-10-17, host-complete)

The session opened after a **full sandbox wipe** — repo, ~/sdk
toolchain and the shared worklog all gone (the third such reset; only
the sandbox's own Node/npm and a javac-less JRE survived). Recovery
from the records: re-clone from the remote at `98cac30` (token from
the session prompt), toolchain reinstalled per doc 06's pins (Go
1.27.2 → ~/sdk/go, Temurin JDK 21.0.12.1+1 from the Adoptium GitHub
release, cmdline-tools 12.0 — the 12266719 zip turned out to be
revision 16.0, the 11076708 zip is the pinned 12.0 — platform-tools,
platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358, licenses
accepted, gomobile/gobind `go install` at x/mobile
v0.0.0-20261011034528-197770444867), worklog recreated from the prompt
plus repo records, npm ci + dist sync (D74) and the desktop baseline
re-verified GREEN before any M5 code (vet clean; 17/17 packages;
svelte-check 0/0; vite clean).

The milestone: consent before routing, honest reporting, and
reconciliation. The design problem the M4 shell left open: the page
starts the engine directly through the webui, so consent happened
AFTER Start — an engine "on" without a TUN (or a denied consent) would
have been a misleading "Connected", exactly what M5's exit criterion
forbids. The fix is architectural, not cosmetic (D77): an optional
`engine.StartGater` door that only the Android adapter implements; the
connect run waits at the door BEFORE the listener, holding
"connecting", with the wait OUTSIDE the start budget (the run context
is split: run-lifetime cancel + post-gate timeout — a permission
dialog is not a network failure). Cancellations release the waiter
quietly; the canceller owns the phase. Desktops do not implement the
interface — their Start path is unchanged, byte for byte, proven by
the full gate. The facade arms the gate closed at `NewEngine`; Kotlin
opens it (`setVpnReady(true)`) only when consent is granted AND the
TUN is established, re-arming closed on every unwind.

D78 is the honest report: `VpnState()` answers the product's
"Connected" question — routing requires the engine serving AND a stack
bound AND that stack's door being the LIVE server's door. The third
condition is the one the lifecycle suite CAUGHT as a bug in my first
derivation (a stack bound to a dead server's door reported routing
after an engine restart — now the stale case is observable:
tun:true, routing:false, the shell's reconciliation signal). The
notification text in Kotlin is derived from the same JSON the host
tests verify — one truth, not two. The shell policy: explain-once →
prepare() → dialog; denial stops the held engine plus honest copy;
consent and service start are single-flight (rotation-safe);
MainActivity.onResume re-derives the world; the service's onDestroy
stops the engine whenever it serves or connects (a dead service leaves
neither a lying "Connected" nor an invisible zombie); onRevoke =
disconnect + re-arm.

The host evidence (`mobile/lifecycle_test.go` against the REAL engine
and stack — the M4 harness split for gate-driven starts): consent hold
→ grant → on; denial → clean off; three connect/disconnect cycles
with the report honest at every step; PORT_BUSY injection (typed
error, routing never claimed); the stale-stack TUN_ACTIVE refusal with
reconciliation; Shutdown-with-routing → all-false. Engine-level gate
tests (hold outlives the budget, quiet cancellation, second-Start
semantics) and adapter-level gate tests (zero-value open, re-arm,
all-waiters, ctx release). Two real M4-era bugs found and fixed while
building it: the facade's DOOR refusal paths leaked the fd (violating
D76's own single-owner rule), and the harness's AutoPort=false + fixed
4525 only held up while the package ran alone — under `go test ./...`
concurrent package execution the gate's delayed bind hit the engine
package's 4525 and the honest PORT_BUSY exposed it (root-caused with
a log-ring dump + a listen probe + an instrumented ladder — not
retried away; the harness now writes the product default).

Build lane, re-verified from the recovered toolchain: `gomobile bind`
→ younblock.aar 24,893,759 B (3 ABIs; javap on classes.jar shows
`setVpnReady(boolean)` and `vpnState()` alongside the M4 bindings);
clean `assembleDebug` → app-debug.apk 34,711,941 B, SHA-256
c46026a7984d9fda69b45954436c630f625a74fecf451e6e50c2a3d2b6d5bf26,
badging minSdk 26/target 36, YounblockVpnService + BIND_VPN_SERVICE
in the dumped manifest, apksigner verify OK (debug cert). One build
hygiene note recorded: an incremental assembleDebug after an AAR swap
produced a 46 MB APK (stale merge artifacts) — the clean build is the
honest 34,711,941 B recorded above.

The honest ⛔, unchanged and re-verified (`ls /dev/kvm` → No such file
or directory): no emulator, no device — the live consent dialog, the
notification channel behavior, screen-off/Doze persistence, real-radio
network switching, onRevoke from system settings, reboot (AND-028),
plus the folded M4 checklist (real DoH through the stack, QUIC
fallback on the real YouTube app, protect() on real sockets) could not
run; they ride with the shared device checklist, never claimed.
Desktop regression green at the end: vet clean; `go test -count=1
./...` 17/17 packages ok, run twice consecutively with no flakes;
svelte-check 0/0; vite build clean; compile matrix 4/4 ABIs with
-mod=vendor.

## M6 — settings and mobile UX (2026-10-17, host-complete)

The session opened after the THIRD recorded full sandbox wipe — the
repo, the ~/sdk toolchain and the shared worklog were all gone.
Recovery per the recorded state: re-clone at 4083e76 (clean, synced),
toolchain rebuilt exactly per doc 06's pins (Go 1.27.2, JDK 21.0.12.1+1
Temurin with javac — the system JDK is a JRE only, cmdline-tools 12.0,
platforms 26+36, build-tools 36.1.0, NDK 28.2.13676358, gomobile/
gobind at the pinned x/mobile), worklog rebuilt from the session
prompt + repo records, and the desktop baseline re-verified GREEN
before any M6 code: vet clean, 17/17 packages twice, svelte-check 0/0,
vite clean (the webui dist synced first — the D74 trap).

The milestone (doc 05 §M6, D79/D80):

- **Schema v2**: `config.Settings` gains `Apps []string`;
  `settingsVersion` → 2; migration is the existing mechanism (a v1
  file lacks the field, `normalizeApps` fills Chrome+YouTube). The
  load-bearing rule: empty-or-all-invalid RESTORES THE DEFAULTS — an
  empty allow-list persisted would route every app (the opposite of
  the product scope). Validation: package shape, dedupe, cap 32, case
  preserved. `internal/config/apps_test.go`: the matrix, the cap, the
  v1→v2 file migration with custom fields surviving, round-trips, the
  exact JSON shape.
- **The facade plumbing**: the engine's locked `Settings()` accessor +
  `mobile.RoutedApps()` → `{"apps":[…]}` — the same normalized truth
  behind settings.json, the jsonapi settings verbs and the Kotlin
  service's read. `mobile/apps_test.go`: default, tracked-through-save,
  all-invalid→defaults (both doors agree), persistence across an
  engine recreate.
- **The Kotlin consumption**: `readRoutedApps` at TUN establish, fail
  closed (unreadable/empty → refuse to route), `DEFAULT_APPS` deleted
  (no second copy to drift). Apply-on-next-connect by construction:
  the list is baked into the builder at establish; the UI copy says
  so. NO_TARGET_APPS deferred to M7 (a shell→engine error door is a
  D78-class design question for the hardening milestone).
- **The WebView UX**: `Backend` grew getSettings/saveSettings/
  getDiagnosticReport (Wails/HTTP/preview — the preview returns null,
  never invented data). The menu sheet's Android-only surfaces, gated
  on AppInfo.platform/capabilities.perAppRouting so the desktop sheet
  keeps its exact prior copy byte-for-byte: the Apps-to-unblock row
  (scope intro line as the standing first-run copy; friendly labels
  only for the two known packages; add-by-package-name with the saved
  response as the validation authority — refused entries surface as
  inline feedback, no client-side duplicate of the Go rules), the
  VPN-slot/key-icon About lines, the Android How-it-works/Help
  variants, the doc 07 §5 privacy section, and the diagnostics view
  (the existing redacted report in a labeled scrollable region with
  copy). A11y host-half: the generic focus trap covers every new
  control, aria-labels, 2.75rem touch targets, rem typography,
  role="alert" errors.

Build lane, re-verified end-to-end after all M6 changes: `gomobile
bind` → younblock.aar 25,097,538 B (3 ABIs; javap shows `routedApps()`
bound alongside the M4/M5 surface); clean `assembleDebug` →
app-debug.apk 35,194,757 B, SHA-256
5669efd14a0a523249e55c9f2c7ab0d6b1edb62156c60d76c3ccc38cc59302dc,
badging minSdk 26/target 36, YounblockVpnService + BIND_VPN_SERVICE in
the dumped manifest, apksigner OK (debug cert). One honest build
iteration: the first clean attempt failed at compileDebugKotlin (Kotlin
call syntax for the new binding + a missing JSONArray import) — both
errors printed, fixed, re-run CLEAN. The compile-matrix evidence
sharpened: the honest 4/4 form is `go build` over the 18 non-main
packages per ABI (arm64 links the full tree even at CGO_ENABLED=0;
arm/386/amd64 main packages refuse the link without NDK cgo, exactly
as doc 06 §A records).

The honest ⛔, unchanged and re-verified (`ls /dev/kvm` → No such file
or directory): no emulator, no device — the shared M4+M5 checklist
(consent dialog live, notification channel, screen-off/Doze, real DoH
through the stack, QUIC fallback on the real YouTube app, protect() on
real sockets, onRevoke, reboot) now also carries M6's live-WebView
halves (real rotation, font scaling, TalkBack, the apps row on a real
screen). Never claimed, never dropped. Desktop regression green at the
exit gate: vet clean; `go test -count=1 ./...` 17/17 packages ok, run
twice consecutively; svelte-check 0/0; vite build clean; matrix 4/4
ABIs with -mod=vendor.

Next: M7 — regression, hardening, release preparation (doc 05 §M7).

## M7 — regression, hardening, release preparation (2026-10-17, host-complete)

The first session in the port's record to open with the environment
INTACT — no wipe: the repo at 57adf1a (clean, synced), the ~/sdk
toolchain, and the shared worklog all survived the boundary, verified
with commands; the desktop baseline re-run green before any M7 code
(vet clean, 17/17 twice, svelte-check 0/0, vite clean).

The milestone (doc 05 §M7, D81/D82):

- **The NO_TARGET_APPS door (D81)** — the question M6 deferred,
  settled: the start gate now refuses as well as opens. The Android
  adapter's RefuseStart hands a typed engine.PlatformRefusalError to
  the gate-held connect run (or latches it for a not-yet-arrived run),
  and the engine routes it through its ordinary failure path — phase
  "error" with the typed code, the page showing "Can't Connect" plus
  the specific reason. The phase machine stays the single truth
  (ADR-0006). Delivery is atomic: the shell's immediate re-arm can
  never claw back a delivered refusal (the race has its own regression
  test), the latch clears on open/re-arm, an open gate reports
  NOT_HELD, and the facade whitelists the vocabulary twice. Suites:
  3 engine gate tests, 6 adapter gate tests, 4 facade tests.
- **The spawn-race zombie closed (D82)** — building the refusal tests
  exposed a real M5-era hole: a connect goroutine delayed past its own
  Stop registered a fresh uncancellable context at entry and could
  resurrect a phase after its cancellation. TestEngineStartIsIdempotent
  had been passing THROUGH the bug (its immediate double-Start saw "on"
  only because the zombie completed after the documented cancel). Fix:
  run generations — every cancel supersedes spawned runs; the
  stale-check and registration are one critical section. The test was
  rewritten to its true intent (idempotency while ON). Desktop
  behavior unchanged in practice.
- **The -race lane (new)**: gcc is present; `go test -race` green on
  the seven key packages (engine 10.4 s, android 2.1 s, mobile 38.4 s,
  tunstack 2.3 s, proxy 11.4 s, webui 1.1 s, config 1.0 s) — zero
  data races through the new gate/generation machinery and the M5
  lifecycle suite.
- **The audit + permission review**: the AAR's real dependency closure
  (go list -deps ./mobile) is the gVisor netstack subset, golang.org/
  x/{net,sync,sys,time,exp}, x/mobile, google/btree — all permissive,
  LICENSEs vendored; the About attribution line added. The built APK's
  dumped manifest is EXACTLY doc 07 §6's inventory (four permissions +
  the BIND_VPN_SERVICE service attribute + the two query packages;
  ACCESS_NETWORK_STATE absent).
- **Release prep**: doc 10 (the v1 release-notes draft — the honest
  headline, does/does-not, the doc 02 §4 limitation obligations, the
  ⛔ open-gates section, sideload install, attribution); doc 02 §7's
  traceability rows updated for the M6/M7 evidence; doc 07 §9's
  checklist closed for the five host-testable items.

Build lane, re-verified end-to-end: `gomobile bind` → younblock.aar
25,072,133 B (3 ABIs; javap shows failStart(String) bound); clean
`assembleDebug` → app-debug.apk 35,298,149 B, SHA-256
f27b7eef014f61eff35405f4406bfe9c33d4e34a918f9949eaad417d561c6491,
badging minSdk 26/target 36, manifest verified above, apksigner OK
(debug cert). One honest regression iteration: D82's honest cancel
broke the idempotency test — root-caused (the test codified the
zombie), fixed at the test, green twice after.

The honest ⛔, unchanged (`ls /dev/kvm` → No such file or directory):
battery/behavior measurement, reboot/revocation (AND-028), and every
live-observation gate (consent dialog, notification channel,
screen-off/Doze, real DoH, QUIC fallback, protect(), rotation/font-
scale/TalkBack) ride with the shared device checklist — documented in
doc 10's open-gates section, never claimed. Desktop regression green
at the exit gate: vet clean; 17/17 packages twice; svelte-check 0/0;
vite clean; matrix 4/4 ABIs.

Next: M8 — package, verify, hand off (doc 05 §M8; the keystore
question for Ali becomes blocking for the signed release APK).

## Android port — M8-a: Ali's two product rulings, implemented (2026-10-17)

Between M7 and M8, Ali answered the two standing product questions, and
the answers changed shipped behavior — so they were implemented and
verified as their own change set (D83, doc 05 M8-a) before M8 freezes
the release artifacts:

- **"There is no allow-list defaults."** The apps allow-list starts
  EMPTY; nothing is routed until the user adds an app. The M6
  restore-defaults Normalize rule is removed; RoutedApps fails toward
  the empty list; the Kotlin establish routes the empty list through
  the M7 typed door failStart("NO_TARGET_APPS") (the old silent stop
  would have hung a gate-held run — empty is the normal fresh state
  now); the UI drops the last-entry guard, shows the empty state, and
  the NO_TARGET_APPS message covers both empty and none-installed.
  Saved files keep their lists — the default changes, never user data.
- **"Keep every domain with .ir blocked — when opening any domain with
  .ir, this app must not be used and its routes should not work."**
  internal/config/direct.go (UnderDirectTLD) + Server.routedHost — the
  one decision both doors share: the mode/list rule minus the
  never-processed TLDs, which win over ModeAll AND over the user's own
  domain list. The PAC emits the .ir DIRECT guard as its first line
  (youtube + all modes — youtube mode already kept .ir out by
  fallthrough; ModeAll is the real, ruled behavior change, disclosed in
  the changelog). The SOCKS5 door makes the same decision: .ir traffic
  that transits the Android TUN exits as a plain direct dial — never
  fragmented, never redirected, never claimed. Honest limit stated
  in-app and in doc 07 §5: .ir lookups still resolve through the
  configured DoH providers (no other resolver on Android;
  ACCESS_NETWORK_STATE deliberately not requested, D74).
- **The latent <queries> bug the ruling exposed**: the manifest listed
  only the two former defaults, so a user-added package would have been
  invisible (Android 11+ package filtering) and refused as "not
  installed" even when installed. The block is now the launcher-intent
  query (every launchable app visible, no QUERY_ALL_PACKAGES;
  non-launchable packages cannot be verified — the stated limit).

Build lane, re-verified end-to-end after the changes: gofmt-canonical
on every touched file; vet clean; 17/17 packages twice; -race green on
the seven key packages; svelte-check 0/0; vite build clean + webui
dist synced (TestWebUILifecycle green on the new bundle); compile
matrix 4/4 ABIs; `gomobile bind` → younblock.aar 25,160,550 B (javap:
the nine bindings); clean `assembleDebug` → app-debug.apk 35,567,561
B, SHA-256 ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bf
d37b64, badging minSdk 26/target 36/3 ABIs, the dumped manifest
carries the launcher-intent <queries> and exactly doc 07 §6's
permission inventory, apksigner OK (debug cert).

The honest ⛔, unchanged (`ls /dev/kvm` → No such file or directory):
all device-side gates (live consent, notification behavior, screen-off/
Doze, real DoH, QUIC fallback, protect() on real sockets, the .ir
direct path observed on a real radio, live rotation/font-scale/
TalkBack) ride with the shared device checklist — carried, never
claimed.

Next: M8 — package, verify, hand off (doc 05 §M8). The allow-list
question is now ANSWERED and implemented; the keystore question
(doc 07 §8) is the one remaining product decision, and it is blocking
for M8's signed release APK.

## Android port — M8: package, verify, hand off (2026-10-17) — Phase 2 host-complete

The keystore question — the last open product decision — was answered
at the open and reshaped the milestone into **the debug-APK-only test
circle (D84)**: "yes keep it debug version. we're not releasing yet.
but release in github for me to download and debug." No signed
variant exists; the debug APK is the distributed artifact, published
as the GitHub prerelease **v1.6.6-android-debug.1** on the private
repo for Ali's own device testing. Ruling 1 was confirmed as shipped
("Allow-list starts empty; nothing is routed until apps are added.
youtube is allow list." — D83 stands; the YouTube app is the canonical
list entry, the YouTube domains the domain-level unblock target).

M8 changed **no application code** (docs + release only), so the
shipped artifact is the M8-a clean-build output, re-verified on disk
before distribution rather than pointlessly rebuilt: sha256
`ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64`
(35,567,561 B — the exact M8-a record); `apksigner verify
--print-certs` v2-scheme OK with the debug certificate digest
`c76bb7c6…8551d` recorded; badging `io.github.assadigit.younblock`
1.6.6 (versionCode 1), minSdk 26 / targetSdk 36, native-code
arm64-v8a/armeabi-v7a/x86_64; the dumped manifest carries exactly
doc 07 §6's four permissions and the launcher-intent `<queries>`;
javap shows the full binding set (failStart, routedApps and the rest).

The M8 lanes, all with evidence: the baseline gate at the open (vet
clean; **17/17 ×2 consecutive**; svelte-check 0/0; vite build clean;
webui dist synced with zero diff); the dependency-closure re-run
(`go list -deps ./mobile` — the same audited set as M7); the
platform-requirements recheck against developer.android.com primary
sources (VpnService/Builder per-app routing, BIND_VPN_SERVICE
"only the system can bind", package-visibility `<queries>` on API
30+, foreground-service types at API 34+ — connectedDevice declared
and verified in the dumped manifest, POST_NOTIFICATIONS as an API 33+
runtime permission); and the release lane — tag, prerelease flag, the
APK uploaded as the release asset, verified through the GitHub API
after creation (asset size and digest match the on-disk artifact).

Three of doc 07 §9's M8 rows closed (dependency review, platform
recheck, signing/version/checksums — the release-keystore half
superseded by D84); the IPv4/IPv6 + TCP/UDP device row keeps its open
status, now with its closure path: **Ali's own device testing through
the debug release**. The honest ⛔, unchanged and re-verified
(`ls /dev/kvm` → No such file or directory): install/launch smoke,
the live consent dialog, notification behavior, screen-off/Doze, real
DoH, QUIC fallback, protect() on real sockets, the .ir direct path on
a real radio, rotation/font-scale/TalkBack, battery, reboot/onRevoke
— all handed to Ali's device through the release's test circle
(doc 10 §"Device test circle"), never claimed here. His observations
are the next session's input.

The D84 versionCode rule is carried for every successive debug build
(bump it so updates install over the previous; same debug
certificate). Docs finalized end-to-end: doc 05 §M8 (the evidence
record + exit verdict), doc 06 (the M8 record incl. the release
lane), doc 07 §8/§9, doc 09 (handoff fields → Phase 2 COMPLETE),
doc 10 (the shipped test-circle edition + the device test circle),
README (the Android sideload section), DECISIONS.md D84, CHANGELOG,
and the session prompt refreshed to the post-M8 edition.

**M0–M8 complete. Phase 2 of the Android port is host-complete: the
port is packaged, verified, and handed to the owner's device.** Next:
the device-test feedback loop — ingest Ali's observations from
v1.6.6-android-debug.1, fix what they surface, cut
v1.6.6-android-debug.2 (versionCode bumped) through the same release
lane, and close checklist rows with measured evidence. The
signed-release decision returns only when a public release is planned.

## Android port — the device-test feedback loop, session 1: the install observation (2026-10-17)

Ali's first observation from `v1.6.6-android-debug.1` arrived: "Android
didn't let me install it." Diagnosed before any fix, per the honesty
rules, with the artifact exonerated end-to-end first:

- The bytes GitHub actually serves were downloaded through the API and
  hash to exactly the recorded SHA-256 (ccf46a2e…7b64) — the serving
  lane is sound. apksigner v2-verifies the served file with the
  recorded debug certificate; badging shows minSdk 26 / target 36 /
  versionCode 1 / the four permissions; the binary manifest carries
  `debuggable=true` (normal for a debug build; does not block
  sideloading) and no `testOnly` (the flag that would make the package
  uninstallable); the zip central directory is intact (20 entries,
  testzip clean).
- The decisive clue: the release asset's **download_count was 0** —
  only completed downloads count, so the most probable story is that
  the download never finished on his phone (release assets are served
  from release-assets.githubusercontent.com, frequently slow or
  unreliable from Iranian networks), and Android refused the truncated
  file. The remaining branches: the install-unknown-apps gate, Play
  Protect's "unsafe app blocked," or a device below Android 8.0.

What shipped in response (D85):

- **The ordered troubleshooting path is now on the live release body**
  (PATCHed through the GitHub API, HTTP 200) and mirrored in doc 10 §1:
  check the file size first (must be ≈ 35,567,561 B — a few-KB file is
  an error page), retry on Wi-Fi or download on a PC and copy via USB;
  then the unknown-apps gate; then Play Protect's "install anyway";
  then the parse-error/storage branches. The help sits exactly where
  the retry happens — on his phone, on the release page.
- **The pinned debug keystore**: the exact key that signed debug.1 is
  vendored at `android/app/debug.keystore` and wired as the debug
  signingConfig. The latent hole it closes: doc 10 promised
  "successive debug builds install over this one (same certificate),"
  but that promise rested on the machine-local
  ~/.android/debug.keystore, which regenerates on every sandbox wipe
  (three wipes already in the record). Verified: clean assembleDebug →
  the same certificate digest c76bb7c6…8551d → and the rebuild is
  **byte-identical to the published release** (sha256 match). The
  update promise is now wipe-proof. A debug keystore is not a secret
  (the well-known "android" password); the no-committed-keys rule
  covers release keystores, which stay out per D84/doc 07 §8.
- **No debug.2 was cut**: nothing about the artifact changed — it is
  byte-reproducible — so a re-release would churn tags without fixing
  a device-side or network-side cause. The next release rides on the
  next observation batch that changes code.

Exit gates: vet clean; go test 17/17 ×2 (exit 0 both passes);
svelte-check 0/0; vite build clean; webui dist zero-diff (the D74 trap
checked). No Go, Kotlin or frontend code changed this session. Standing
⛔ re-verified implicitly by the observation itself: the device gates
close on Ali's device, and the first one (install) is now mid-loop —
his retry result is the next input.

## Android port — the device-test feedback loop, session 2: the YouTube default (D86, 2026-10-17)

Ali's second message carried the first product ruling of the loop and
the first functional observation: "mAKE the youtube default added list.
the first version didnt work, whether in youtube app or youtube in
chrome." The release asset's download_count had reached 2 — the
install completed this time, so the observation is a real on-device
result.

Root-caused before any fix, by reading the whole routing chain
end-to-end (tunstack.go/dns.go — UDP:53 to any destination answered by
the virtual DNS, fake-IP reversal to domains, AAAA/HTTPS answered
empty with correct client-fallback semantics; socks5.go + server.go —
the routedHost mode/list-minus-.ir decision and first-record
fragmentation; YounblockVpnService.kt — the establish path). The chain
is sound; the failure was the shell: debug.1's list was born empty
(D83), and the only entry path was a text field demanding exact Android
package names — typing "youtube" was refused by package-shape
validation, so the product's own namesake could not realistically be
added, the list never grew, every connect took the NO_TARGET_APPS
refusal, and nothing ever routed. Exactly "didn't work, whether in
youtube app or youtube in chrome" (Chrome was never on the list, so
youtube.com in Chrome was untouched by design).

D86 implements the ruling at the right layer:

- DefaultApps = [com.google.android.youtube]; a SAVED empty list still
  means empty (user agency — removing the last entry must stick; the
  NO_TARGET_APPS door owns the connect, never a restore-to-default).
- settingsVersion 2→3 with a one-time migration: v1/v2 files whose
  apps list is absent or empty (the shipped default of their era, never
  a choice — the exact state on Ali's device) migrate to the default;
  v3 empties are the user's own and stay; custom lists survive
  verbatim. The update installs over debug.1 (versionCode 2, the pinned
  D85 certificate) with settings intact — the migration is what makes
  the ruling reach the device that reported the problem.
- The entry path became usable: one-tap quick-add chips (+ YouTube /
  + Chrome) and friendly-name aliases ("youtube", "chrome") resolved
  client-side, the engine's saved response staying the validation
  authority. The honest copy updated everywhere it claimed "starts
  empty" (menu sheet, How-it-works, the NO_TARGET_APPS message, README,
  doc 07 §1, doc 10) — plus doc 10's stale DoH default list corrected
  to the code truth (RethinkDNS, Cloudflare, Google).
- Chrome is NOT in the default (the ruling names YouTube only); the +
  Chrome chip and test-circle row 4 teach the one-tap add, and the
  open question is carried in the handoff.

Lanes, all with evidence: vet clean; go test 17/17 ×2 (exit 0 both);
-race green on config/engine/mobile; svelte-check 0/0; vite build clean
+ dist synced (TestWebUILifecycle green on the new bundle); the compile
matrix re-derived in its exact historical form (18 non-main packages
per ABI ×4; arm64 links the full tree — including resolving a probing
subtlety: `go build` with an empty package list builds the main
package, which legitimately needs cgo on arm/386/amd64); gomobile bind
→ AAR 25,162,951 B (javap bindings); clean assembleDebug → APK
35,716,265 B, sha256 9cca38c9…811b, versionCode 2, the pinned
certificate, the four-permission manifest, no testOnly, the D86 copy
verified embedded in libgojni.so. Released as v1.6.6-android-debug.2
through the same lane (tag, upload, API verification).

The device gates remain ⛔ and ride with doc 10's test circle; the
diagnostics report (Menu → Diagnostics → copy) is the requested
evidence if anything still fails on his radio — especially real DoH
reachability and the QUIC→TCP fallback, the two behaviors no host lane
can prove.

## Android port — the device-test feedback loop, session 3: the browsers join the default (D87, 2026-10-17)

Ali's third message of the loop was a pure product ruling, no new
device observation: "Add chrome to the default addlist as well, also
firefox, opera, brave browser." It closes the open question D86
carried (whether Chrome should join the default, then one tap away)
and extends it to three more browsers.

The one risk of a multi-app default was audited BEFORE the change:
devices that lack the apps. The Kotlin establish loop has caught
PackageManager.NameNotFoundException per entry since M6 — a missing
package is skipped with a log line, and the refusal only fires when
ZERO entries resolve — so Firefox/Opera/Brave being rarely
preinstalled cannot break the connect; the five-app default is safe
everywhere. That audit is recorded in D87.

D87 implements the ruling:

- DefaultApps = [com.google.android.youtube, com.android.chrome,
  org.mozilla.firefox, com.opera.browser, com.brave.browser] — the
  mainline Play Store package names; Beta/Nightly/GX/Mini variants
  deliberately not chosen.
- settingsVersion 3→4, the era-precise one-time migration: v1/v2
  absent-or-empty → the five-app default (the D83 born-empty
  remediation); a v3 file holding EXACTLY the D86 default upgrades
  (the debug.2 build itself wrote that state — fresh default or the
  v2-empty migration — never a curation; this is the exact
  requested outcome on the one device carrying debug.2); a v3 empty
  (agency, D86's ruling) and every custom list survive verbatim,
  including a v2 [youtube] that was hand-typed under debug.1's
  refusing entry path (under v2 only a human hand could write an
  entry; under v3 the build did).
- The chip row widened to all five defaults, the aliases extended
  (firefox / opera / brave / "brave browser"), the honest copy
  updated everywhere the default list is named (menu intro, empty
  state, note, How-it-works line 1, the NO_TARGET_APPS message,
  README, doc 07 §1, doc 10); versionCode 2→3 (the D84 rule).

Lanes, all with evidence: gofmt-canonical on every touched Go file
(an Edit-tool tab-to-space corruption on the three legacy-tabbed
files was caught by the gofmt lane and reverted with gofmt -w);
vet clean; go test 17/17 ×2 (exit 0 both); -race green on
config/engine/mobile; svelte-check 0/0; vite build clean + dist
synced (the webui suite green on the new bundle; com.brave.browser
verified in the built assets); the compile matrix in its exact
historical form (18 non-main packages per ABI ×4; arm64 links the
full tree); gomobile bind → AAR 25,174,619 B (javap bindings; the
D87 WebView copy verified embedded in libgojni.so); clean
assembleDebug → APK 35,816,233 B, sha256 81515f0d…d6a1, versionCode
3, the pinned certificate (c76bb7c6…8551d), badging minSdk 26 /
target 36 / 3 ABIs, the four-permission manifest, no testOnly.
Released as v1.6.6-android-debug.3 through the same lane (tag,
upload, API verification with the served bytes re-downloaded and
hashed).

The device gates remain ⛔ and ride with doc 10's test circle (rows
2 and 4 now cover the five-app default out of the box and
youtube.com in each installed browser); the diagnostics report
(Menu → Diagnostics → copy) is the requested evidence if anything
still fails on his radio.

## M8-e — the release-keystore ceremony script (D88, 2026-10-17)

No device observations arrived; Ali's ask reshaped the round: "Can
you fix the serial number and etc issue as well? like giving me a
bat file that auto creates it's password locally on my system and
backs up that password as an md file. if you insist it must be done
locally. at least you can create the script." The "serial number and
etc" was keytool's interactive ceremony; the ruling keeps the
security boundary (the signing-key password is generated where it is
used, never through chat/agent/repo) and ships the automation:
make-release-keystore.bat — locate keytool, generate a 32-char
zero-bias password with Windows' CSPRNG, create the keystore with
every prompt pre-filled (RSA 4096, PKCS12, 10950 days), verify it by
reading it back, and write the .md backup (password, alias,
certificate serial number, SHA-256 fingerprint, the full keytool
record). Guardrails: refuses to overwrite an existing keystore,
warns inside a git repo, offline; the .gitignore now guards both
generated patterns.

Lanes with evidence: the real keytool flow verified on the host
(system JRE 21.0.12.1's keytool — genkeypair prompt-free exit 0,
valid to Oct 2056; -list -v line shapes match the script's parsing;
wrong storepass exits 1); the password alphabet proven exactly 64
unique chars with the algorithm simulated; the shipped file pure
ASCII, 253/253 CRLF, 11,527 B, sha256
82e6dded40a8eb415127c6d46b3cb4acb0633ab2cd13ac8b809a8ae8c7c74090,
byte-identical through repo, preview and release surfaces (the
served asset re-downloaded and hashed). ⛔ stated plainly: the .bat's
Windows execution cannot run on this Linux host — the script
self-verifies on Ali's machine (the read-back round-trip) and every
failure path carries its own message. Zero Go/Svelte/app code
touched: the M8-d gates stand as the baseline of record for the
unchanged clean tree (the ~/sdk toolchain wipe between sessions is
recorded in doc 06's M8-e entry; the next code-touching session
reinstalls per doc 06 §1 first). Delivery: committed at the repo
root and pushed; attached to the EXISTING v1.6.6-android-debug.3
release as a second asset (no new tag, no new APK — the D84 rule);
README's signed-lane section written (closing the doc 07 §8 "README
steps" hole); docs 06/07/09/10, DECISIONS.md D88, CHANGELOG and the
session prompt all updated. The device gates and Ali's debug.3
observations remain the project's open input (next Task ID M8-f).

M8-g — the fifth feedback round (2026-10-17): D89, the release-flag
ruling. Ali: "Do not prelease , just release, the repo is private.
Please finish all of your specs first." The spec audit ran first:
doc 05's Phase 2 host-complete end to end (M0–M8 all ✅; every
remaining ⛔ an on-device gate needing his physical device), docs
current through D88, the M8-f round having verified every delivery
surface with zero repo changes — nothing host-side left unfinished.
Then the flip, with evidence: the three android releases switched to
regular (non-prerelease) releases through the GitHub API — flags,
tags, assets and digests checked before and after; debug.3 explicitly
marked the repo's Latest release (/releases/latest returns it — the
plain flag flip does not move the marker by itself); its body gained
exactly one honest D89 note (still the debug-signed device-test
build; the repo is private so the releases stay collaborator-only;
nothing about the artifacts changed), every other byte of the body
identical to the pre-edit one. Docs: D89 recorded; docs 06/07/09/10,
README, CHANGELOG and the session prompt updated. Zero app code; the
APK bytes untouched (D84 respected — a flag flip is metadata, not a
build); versionCode stays 3. Ali's device observations from debug.3
remain the open input (next Task ID M8-h).

M8-h — the sixth feedback round (2026-10-17): D90, the ceremony
survives PowerShell-less PATH machines. Ali's first field report
arrived — not a device observation yet but the helper's first real
run: make-release-keystore.bat stopped at password generation with
"'powershell' is not recognized as an internal or external command"
(his PATH leaves PowerShell out; Windows itself ships it — the same
environment class the desktop setup scripts fixed at v0.7.5). Fix:
the script resolves powershell.exe by its full standard install path
first (setup-offline.bat's shipped pattern), then where.exe, and
falls back to a pure-cmd six-RANDOM-draw generator (digits and
dashes, ~90 bits) when PowerShell cannot be found at all — the step
can never dead-end; the .md backup and DONE summary record the
generator used. His online-generator question ruled NO (D90: the
signing identity never transits a third party; D88's boundary
stands); a one-line manual keytool alternative documented in the
release notes. Delivery: v1.6.6-android-debug.4 — a regular release
(D89) carrying ONLY the fixed .bat (13,683 B, sha256
ee35e456…9558; no APK — the app is unchanged, debug.3's APK stays
current per D84), tagged at the fix commit cb28507, Latest-marked
(the asset's served bytes re-downloaded and verified byte-exact);
debug.3's page gained a pointer note superseding its older helper
asset (body 9,017 → 9,650 chars, prefix byte-identical). Zero app
code; versionCode stays 3; ~/sdk still wiped (reinstall before
Go/Svelte work stands). The device-test observations from debug.3
remain the open input (next Task ID M8-i).

M8-i-a — the seventh feedback-loop session, the recovery round
(2026-10-17): NO device feedback arrived (checked the session
messages and the GitHub API — zero issues on the repo), so per the
standing rule no code was touched and no release cut. What the
round did: the fourth sandbox wipe was found (~/sdk absent; the
repo itself intact at f8b2edc, clean and in sync, untracked build
state surviving) and repaired — the toolchain reinstalled per doc
06 §1's pins (Go 1.27.2, Temurin JDK 21.0.12.1+1 with javac,
cmdline-tools 12.0 — via the versioned 11076708 URL, because the
12.0 alias now 404s; a discovery recorded in doc 06 — platform-
tools 37.0.1, platforms 26+36, build-tools 36.1.0, NDK
28.2.13676358, gomobile/gobind at the pinned x/mobile version;
~/sdk/env.sh + android/local.properties rewritten). The full
baseline re-verified GREEN on the unchanged tree BEFORE anything
else: vet clean; go test 17/17 ×2 (engine 9.6 s, proxy 16.2 s,
mobile 36.8 s); svelte-check 0/0; vite build clean + webui dist
re-synced (the worktree stayed clean through the rebuild); the
compile matrix 4/4 ABIs in the honest form (arm64 full tree; 18
non-main × arm/386/amd64); ./gradlew projects BUILD SUCCESSFUL.
The release lane re-verified live: debug.4 regular + Latest, the
older android releases regular, zero issues. /dev/kvm still
absent. Zero app code; versionCode stays 3; the APK bytes
unchanged. The device-test observations from debug.3 and the
fixed helper's first DONE output remain the open input (next Task
ID M8-i-b) — the environment is READY for the immediate
fix-→-bump-→-debug.5 cycle when they land.

M8-i-b — the eighth feedback-loop exchange, still PC-side
(2026-10-17): Ali's report: "I created a key chain now what should
i have to do? also the last release only has keychain, no apk to
download." (1) The keystore ceremony COMPLETED on his machine —
the D88/D90 loop's first finished run (the DONE output was not
pasted; the shareable identifiers if ever needed are the
certificate serial + SHA-256 fingerprint — never the password).
(2) A delivery-UX report: the Latest release (debug.4) carries
only the helper .bat, and its title said "(no new APK)" —
accurate but not actionable. Fix, delivery-lane metadata only
(D84 untouched): the debug.4 release title retitled through the
API to "…the keystore helper fixed · no APK here — the app is on
debug.3" (make_latest="true" on the PATCH — debug.4 stays the
repo's Latest, verified through /releases/latest; tag, body, and
assets unchanged). The body's first line (with the direct APK
link) and the README's download line already carried the story —
unchanged. Ali was told: the APK to install is debug.3's
app-debug.apk (35,816,233 B; installs over debug.1/.2 in place —
the pinned debug certificate); the keystore needs nothing now
(keep younblock-release.keystore + the .md backup in two
different safe places, private, never pasted — the .md holds the
password; it is used only when the signed public release is
planned); and the real next input is the device test circle
(doc 10) with the diagnostics report for anything that fails.
Offered for his ruling, not decided: attaching the APK to every
release page (relaxing D84). Zero app code; no release cut;
versionCode stays 3; the APK bytes unchanged. The DEVICE results
from debug.3 remain the open input (next Task ID M8-j).

M8-j — the ninth feedback-loop exchange, the FIRST DEVICE feedback
(2026-10-17, D91): Ali reported from the debug.3 circle: "it worked
on android. Play store message popped up… I generated key ring, but
I didn't attach to APK if needed… [video] took like 30 sec - 1 min or
more to actually show that video… feed quickly refreshed but actual
video playing delayed… The numbers in the app are too near each
other, they need regular spacings." The round shipped
v1.6.6-android-debug.5 / versionCode 4 (ad02980, APK 34,831,081 B,
sha256 5df2acc3…02b4, the pinned debug certificate — installs over
debug.3 in place): (1) the numbers-spacing fix — root cause: the pro
screen's stage was fixed at 22.5 rem and outgrew the viewport at
narrow widths/font scale (measured 495 px of stage in a 320 px
viewport at 1.4×; the two speed values sat 24 px apart with 29 px
screen margins); the stage is now capped to the viewport, the speeds
row widened (gap 1.75 rem, icon 0.5 rem, nowrap) and the timer
digits given air (0.66 em, tabular-nums); verified before/after
through the rendered app at the worst and canonical cases. (2) The
video-start instrumentation — every host-level suspect eliminated
with evidence (the netstack DOES answer non-DNS UDP with ICMP 3/3
port-unreachable — proven by a temporary probe, then pinned
permanently by TestUDP443FastFailICMP; TCP rcvWnd 1 MiB; fragment
pauses 1-10 ms; DoH cached/singleflight/keep-alive; conn caps 1024;
the connect gate already runs a real proxied YouTube check before
"active" — Ali's "make the connection time longer" suggestion is
already the built behavior); every tunnel now logs dns/dial/
first-byte/dur/bytes into the diagnostics ring (hostnames gated
behind LogDomains; the first-record write counts on the flow meter
— caught by the new flow-timing tests). Also: the webui dist
clean-synced for the first time (the stale hashed assets purged
from the go:embed), Play Protect documented as expected for
sideloaded debug APKs, and the keystore ruled NOT for the test
circle (signing a test build with it would break the pinned-cert
update chain). The session survived the FIFTH sandbox wipe
(mid-session; the toolchain reinstalled per doc 06 §1). Baseline
after: vet clean; go test 17/17 ×2 (with the two new test files);
svelte-check 0/0; vite clean. Release lane: debug.5 created regular
at ad02980, Latest-marked, APK asset served byte-exact (one API
wrinkle recorded: a SHORT target_commitish 422s — use the full
SHA). Test-circle rows updated with the evidence: row 1 PASS with
the Play Protect note, row 2 PASS, row 3 PARTIAL (the round's
subject), row 9's host half pinned by test. Next Task ID M8-k
(ingest the debug.5 device round — the diagnostics report with the
timing lines is the expected input); next decision D92.

M8-k — the tenth feedback-loop exchange, the cross-platform UI
unification (2026-10-18, D92). Ali uploaded mobile.zip to the repo
carrying the UI instruction documents his Windows/Linux builds
follow (round-2's roast-report + fix-instructions; round-3's
feature-spec + agent-instructions + usage-screens.png;
site-test-plan) and asked for one consistent UI across devices.
The compliance point is the ONE shared screen every edition embeds
(D92): the pro screen now carries the round-2 fixes (Lucide icons
through src/lib/icons.ts with lucide-svelte@1.0.1; the "…" menu
button top-right at 44 px; the sheet's fixed header with a neutral
44 px close; 0.80 em timer digit boxes; the connected ring family
capped clear of the 12 px / 82 % hint at 37.25 rem; ping verdicts
Good/OK/Slow; honest -1 speeds; the ChitaCodes credit; the
Other-websites section with VERIFIED_SITES; the Wails window
360×640 floored at 320×560) and the round-3 data usage, tracked
ONCE in the engine so the Wails bridge and the webui's /api/usage
agree by construction (internal/usage: usage.json with times and
byte totals only, 500 cap, 15 s crash autosave, recovery at open;
the engine's session = its "on" run at the relay-counter baseline;
State gains downBytes/upBytes/lastSession with an explicit null
after a clear; GetUsage/ClearUsage on every transport; the screen
shows "X used" + the ↓/↑ split and the Last-session line; the
menu's Data usage section with a confirmed Clear). The reference
code folder was not in the upload — implementation follows the
documents, every rule traceable (D92 ruling 3). Verified the
honest way: svelte-check 0/0, vite clean; the rendered app at
320×560, 356×625, 360×760 across every state (no overlap, no
scrollbars; Keep keeps, Delete empties and drops the line); the
REAL binary's byte path — a 949,386-byte YouTube page through the
tunnel = session downBytes 980,303 / upBytes 1,933, the 80-second
record on disk after the stop, todayBytes = allTimeBytes =
982,236. Baseline after: vet clean; go test 18/18 ×2 (the new
internal/usage package among them); the webui dist CLEAN-synced.
Build lane: AAR 25,003,698 B; app-debug.apk 34,951,881 B (bc4b239e
…d1fe8), versionCode 5, the pinned D85 certificate. Release lane:
debug.6 created regular at 078e3a5, Latest-marked, APK served
byte-exact (second API wrinkle recorded: the full-SHA
target_commitish 422s until the commit is PUSHED — create releases
only after the push). Next Task ID M8-l (ingest the debug.6 device
round: the cross-device UI verdict + the data-usage check + the
still-open slow-video diagnostics); next decision D93.

M8-l (2026-10-18, decision D93): the desktop close ruling, between
device rounds. Ali's instruction — "By default, when exe file is
closed and app is shut down, the app must not remain on the taskbar;
only keep being in taskbar if user selected this option on the
settings" — inverted the Windows close contract that had shipped
since the first build: closeHidesToTray answered a hard-coded true
on Windows, so every close hid into the tray and the app lived on.
Now the close button is a REAL shutdown by default (the shutdown
path stops the browser session, quits the tray, disconnects through
the bridge and runs eng.Shutdown — Stop restores the Windows proxy
first, the path Linux has run for years), and tray-keeping is the
explicit `keepInTrayOnClose` opt-in in settings.json (default
false; settingsVersion stays 4 — a pre-D93 file never recorded the
old behavior as a choice, so no migration is owed). closeHidesToTray
takes the setting on Windows and always answers false without a
tray; beforeClose composes them; --minimized and autostart are
untouched (launch choices, not close semantics). The opt-in surfaces
where a tray exists, on BOTH screens behind capabilities.tray (the
pro sheet's new SETTINGS section — the settings load now runs for
perAppRouting OR tray — and the classic Settings row beside
connect-on-launch); the first-run tray variant now promises "Closing
the window stops Younblock.", both Help decks reword the "window
disappeared" entry around the opt-in, and the dormant one-time close
notice stays unwired as it has always been (Go never emitted the
event; the setting's hint + Help carry the disclosure). Verified:
close_test.go pins the contract against a real engine (parse table,
no-tray answer, quit on defaults, quit on opt-in-without-tray, quit
always allowed mid-Quit) + the config round-trip test; gofmt
normalized on exactly the touched files (the frozen 14 untouched);
vet clean; go test 18/18 ×2, -race on the touched two; svelte-check
0/0; vite build + webui dist CLEAN-synced; GOOS=windows compiles
the whole shell; the UI exercised end-to-end on a REAL cmd/core
--web engine through a DISCLOSED scratch build reporting tray:true
(reverted before commit) — both screens' toggles persisted through
/api/settings into the real settings.json and back (VLM-checked),
and the clean rebuild hid both sections (the capability gate's
negative check). README's close/uninstall/manual-check copy updated.
No Android change: debug.6 stays Latest; Ali rebuilds the Windows
exe and walks README manual-check steps 4–5. Next Task ID M8-m
(ingest the debug.6 device round: the cross-device UI verdict, the
data-usage check, the still-open slow-video diagnostics, the D93
desktop check); next decision D94.

M8-m (2026-10-18): the complete gap analysis + the wrap-up that opened
the account-gate lane (D94). Ali asked for "a complete Gap analysis of
this APP" as a report; the round was analysis-only — zero code changes,
worktree clean at 9615936. Two read-only inventory passes (the Go host:
every package, the D93 close path, all 287 tests across 18 packages,
every deliberate limit — the connect-path timeout constants that are
the slow-video diagnosis candidates; the frontend + Android: the
capability matrix, the settings asymmetries, the Android lane facts)
plus a doc baseline (doc 02's AND-001…028 matrix, doc 05/07/09/10,
CHANGELOG/PROGRESS/DECISIONS tails) and an independent re-verification
of mobile.zip: every instruction from every round applied — the one
open item is data (VERIFIED_SITES empty pending the two-day site
tests), not code. The gap register delivered in chat: host-complete
and green everywhere the sandbox can measure; the real gaps are
(1) the open slow-video defect (awaiting the diagnostics report),
(2) the largely unrun device test circle (rows 4–8, rotation/TalkBack,
the cross-device UI verdict, the data-usage check, the D93 real-Windows
check on the rebuilt exe), (3) the dormant desktop release lane
(v0.7.6 predates D86–D93 — nine rounds sit under CHANGELOG
"Unreleased"), and (4) public-release engineering not started
(debug-only signing, no CI — no .github/ at all, no frontend tests,
unsigned exe, no update mechanism). At the wrap-up Ali opened the next
feature lane (D94): login authorization — a verified username/password
gate with a user database and admin user management, making the app
members-only; design-first (the spec lane before any code), with the
privacy-claim conflict named up front ("no account, no data collection"
cannot survive unchanged — D94 ruling 2) and the feedback loop
explicitly continuing in parallel (ruling 4). The wrap-up commit:
DECISIONS.md (D94), CHANGELOG.md (the Planned section under
Unreleased), doc 06 (the M8-m record), doc 09 (the twelfth exchange +
handoff fields → M8-n + A0 / D95), PROGRESS.md (this entry), and
SESSION_HANDOFF_PROMPT.md rewritten for the two-workstream session.
No release: nothing app-facing changed; debug.6 (versionCode 5) stays
the repo's Latest. Next Task IDs: M8-n (ingest the debug.6 device
round) and A0 (the account-gate spec lane). Next decision D95.

A0 (2026-10-18, docs-plus-one-fix; no new decision — D95+ stays
reserved for Ali's six rulings): the account-gate spec round, run as
the second workstream because NO device feedback had arrived (zero
issues on the repo, all states; none in the session messages). The
session opened on the FIFTH sandbox wipe (~/sdk and the repo both
gone; the worklog missing — recreated at /home/z/my-project/worklog.md
with a loss note; the repo re-cloned at cf136aa exactly as the handoff
expected, core.fileMode false + gradlew +x re-applied). The toolchain
was reinstalled per doc 06 §1's pins and verified (Go 1.27.2, JDK
21.0.12.1+1, sdkmanager 12.0 via the versioned 11076708 URL at
153,607,504 B exact, platform-tools 37.0.1, platforms 26+36,
build-tools 36.1.0, NDK 28.2.13676358, gomobile/gobind at the pinned
x/mobile). Baseline first, in the D74 order (frontend before judging
the Go lanes): npm ci; svelte-check 0/0; vite build clean; the webui
dist CLEAN-synced; then vet clean and 18/18 ×2. The full-sweep gate
`GOOS=windows go build ./...` exposed a latent M4 defect:
mobile/nonblock_unix.go had no build constraint, so on Windows it
redeclared setNonblock/closeFd against nonblock_other.go (whose
!unix stub exists precisely to keep ./... compilable everywhere);
the scoped shell builds had always compiled — that is what "the
whole shell" measured — but the full sweep never had, since 9431477.
Root-caused and fixed at the source: one line, `//go:build unix`.
Verified on the fixed tree: gofmt clean (touched file), vet clean,
go test 18/18 — three consecutive green passes, zero FAIL lines —
and the Windows full sweep exit 0 for the first time since M4. No
Android behavior change by construction (every android ABI is unix);
no release (D84: nothing app-facing; versionCode stays 5, debug.6
stays Latest). Then the session's main output: `docs/auth/` — the
six-document A0 spec pack (00-start-here with the consolidated six
questions + recommendations, 01-requirements AUTH-001…031, 02
-architecture, 03-auth-protocol, 04-admin-and-operations, 05-security
-review with the drafted privacy copy), plus ADR-AUTH-0001 in doc 08
(engine-enforced gate, grounded in D94 ruling 1 — the only ADR,
because nothing beyond it has Ali's ruling), doc 07 §9's new open row
(the gate ships only with its privacy-copy change), and the wrap-up
sync of docs 06/09/10, README's docs map, CHANGELOG (Fixed + Planned
under Unreleased) and SESSION_HANDOFF_PROMPT rewritten for the next
session (M8-n first; A1 only after D95+). Next Task IDs: M8-n (the
device round, still awaiting Ali's feedback) and A1 (the auth
backend, gated on the rulings). Next decision D95.

A0-R2 (2026-10-18, later the same day; docs only — no code, no
release): Ali's reply to the A0 wrap-up arrived as one message with
three parts, and the session recorded them per the D94 pattern —
rulings recorded verbatim before anything else, never faked, never
narrowed. Part one, his ruling on open question (b): "I create
account - admin-only" → recorded as D95 (admin-only creation; no
open signup, no approval queue; the surface FORM stays a
recommendation riding (f)'s sibling). Part two, a new direction:
"Also I want two apk version, one with auth, one without (free for
all)" → recorded as D96 (the two-edition split: members carries the
full gate; free contains zero auth code and no server dependency —
build-time only, Go build tags + Gradle product flavors; a runtime
toggle is ruled out by the direction itself; the privacy copy splits
per edition and both copies disclose together at the first split
release; OPEN (g) added with sub-rulings g1–g4: install identity,
distribution, scope, naming — none a decision yet). Part three, his
hosting question — "Can we host it via cloudflare workers/github io
pages/ vercel/netlify etc?" — is NOT a ruling; the session answered
it in the spec pack (02 §6, the honest platform matrix): GitHub
Pages can never host a backend (static only — useful for the website
+ APK downloads); Cloudflare Workers + D1 is the only single-vendor
$0 path (TS/Go-WASM service, ~10 ms/request CPU budgets that cannot
run the A0 argon2id parameters — re-tuned down and re-worded
honestly, platform lock-in, *.workers.dev intermittently filtered
from Iranian ISPs); Vercel and Netlify both force the DB to a second
vendor (no persistent disk) and carry sanctioned-region account
risk; the recommendation stays the small always-on host (the whole
A0 spec runs unmodified; a few US$/month, or Oracle's always-free
ARM if a card at signup can be arranged; the practical constraint
from Iran is the payment method). The cross-cutting hard requirement
is stated in the same section: the auth endpoint must be reachable
from Iranian ISPs as ordinary HTTPS — login happens before connect
and never rides the tunnel — so reachability from inside Iran is the
primary hosting criterion, verifiable only by Ali (a phone, no VPN,
one minute); the sandbox cannot test Iranian-ISP paths and says so.
Docs revised in place with dated A0-R2 notes: docs/auth/00 (the
question table — b RULED, row g added, row a points at 02 §6), 01
(status, §0's objective-split note, new §6 AUTH-040…044), 02 (§1
qualified per D96, §6 the platform matrix, §7 the edition build
mechanism), 03 (§5 now also rides (a) — the argon2/CPU note), 04
(§2's WHO ruled), 05 (§2 and §6 addenda — the free edition has no
gate to bypass; the privacy copy splits per edition). CHANGELOG's
Planned section rewritten for D95/D96. His request for "your
questionaire. in bullet points" was answered in chat and rendered on
the session preview page (the one user-visible route): hosting
sub-questions, edition sub-questions, and the still-open (c)–(f),
each with its recommendation marked as a recommendation. No device
feedback arrived with the message (no repo issues, all states
unchanged); the device round stays open exactly as before. Next
decision: D97 ((a) and (c)–(g) remain open; D95/D96 are spent).

A0-R3 (2026-10-18, the third exchange of the A0 day; docs only — no
code, no release): Ali cancelled the account gate outright. His
message — "You know I've decided to defer and cancel this auth
version, only work on non-auth version. it doesnt worth this
headache. cancel it altogether, no auth needed anymore." — was
recorded verbatim as **D97** (cancellation, not deferral: the closing
words govern). Younblock stays ONE free-for-all app: no login, no
accounts, no auth server, no admin surface, no editions. D95
(admin-only accounts) and D96 (the two-edition split) are superseded
— kept as history, their subject no longer exists; every open auth
question ((a), (c)–(f), (g1)–(g4)) closed as MOOT, never ruled on
its merits; ADR-AUTH-0001 (doc 08) retired; the A1/A2/A3 ladder
cancelled with those Task IDs. Nothing needed removing from the code
because nothing was ever added (A0, A0-R2 and this round were all
docs-only commits — the design-first rule held to the end); today's
"no account, no data collection" claims stay literally true,
permanently, and the privacy-copy change D94 anticipated never
ships. Session actions, all docs: DECISIONS.md D97 (with its bounds:
the cancellation governs the gate only — website, distribution,
release-lane and public-release questions are untouched); docs/auth/
bannered CANCELLED on all six files (full notice in 00, one-line
banners in 01–05, bodies kept as history; 02 §6's Iran-reachability
finding noted as a surviving network constraint for any future
server-side feature); doc 07 §9's account-gate row closed as
cancelled; doc 08's ADR-AUTH-0001 retired + the file Status line;
doc 10's status block closing note + the D1–D97 range fix; doc 09's
handoff fields rewritten (single workstream, next decision D98, A-
series retired); doc 06's A0-R3 session record; README's docs-map
line; CHANGELOG's Planned section rewritten as Cancelled;
SESSION_HANDOFF_PROMPT.md rewritten for the M8-n-only next session.
Verification: docs-only — `go vet ./...` on the untouched tree clean
(exit 0) as the health check; no build/test/release lane applies; no
release cut (D84/D89), versionCode stays 5, debug.6 stays Latest.
No device feedback arrived with this message either; the device
round (M8-n) stays the first workstream exactly as before. Next
decision: D98.

## 2026-10-18 (evening) — M8-n: the first-video defect root-caused and fixed (ADR-0013); D98 (the APK on every release); debug.7 shipped

Ali's first debug.6 device round arrived: "The problem of video
waiting to load stays. the windows app doesnt have this problem.
youtube instantly refreshes and video instantly open. but the APP
android needs 2-3 minutes, less or more, for first video to actually
play." Plus the standing ruling: "from now on, add the APK version
(debug version) in the releases" — recorded as D98 (the M8-i-era
offer, answered; debug.4's missing APK was the observable trigger).

The Windows-instant/Android-minutes discriminator localized the
defect to the Android-only layers, and the root cause was found in
the code — the one structural difference M8-j's host-level
elimination had left standing: the TUN stack accepted the phone's TCP
handshake locally, then asked the SOCKS5 door bounded by
HandshakeTimeout — a 10 s default the facade never overrode — while
the door's own honest setup (DoH resolve up to 3×3 s + the upstream
dial up to 12 s, run with NO client-side abort on the desktop
HTTP-CONNECT door) could legitimately exceed it. googlevideo answers
on filtered networks carry silently-blackholed addresses, and
netx.DialHost walked them sequentially at 5 s each; with the DNS
answer cached at a 120 s minimum TTL, every app retry repeated the
same walk into the same 10 s wall until the cache rotated — the
reported minutes. The feed never hit it (PrewarmNames + the
QuickCheck warm youtube.com/redirector.googlevideo.com before PhaseOn
— the first video is the first cold rr*.googlevideo.com flow).

The fix (ADR-0013, two halves at the two correct layers):
netx.DialHost now races the address list Happy-Eyeballs-style
(attempt k at k×750 ms, full per-IP timeout each, first success
wins, losers canceled — both doors benefit); and the TUN handshake
budget is budget-true: config.TunHandshakeTimeout 30 s (= 3×3 s DoH
+ 12 s dial + slack), passed explicitly by the facade, the tunstack
default raised to the same value. Pinned by
TestDialHostRacesPastBlackholedFrontAddress,
TestDialHostWinnerCancelsLosers, and
TestHandshakeTimeoutDefaultCoversDoorBudget (the lockstep with the
config constant + the covers-the-door-budget invariant).

Full lane, all green (D74 order): svelte-check 0/0; vite build clean;
webui dist CLEAN-synced; go vet clean; go test 18/18 ×2;
GOOS=windows exit 0; compile matrix 4/4 ABIs (19 non-main packages
per ABI — the count grew with internal/usage at M8-k) + arm64 full
tree; gomobile bind → younblock.aar 25,002,994 B; versionCode 5 → 6;
clean assembleDebug → app-debug.apk 34,960,649 B, SHA-256
dc01c0419a211ca007235ffbeaec6ab5acecb2e5930a4efd4642097209cdd83d;
apksigner OK (the pinned c76bb7c6…8551d); badging versionCode 6 /
minSdk 26 / target 36 / 3 ABIs. Release: v1.6.6-android-debug.7, a
regular release (D89), Latest, carrying app-debug.apk per D98 (asset
bytes verified through the API). Docs: doc 06 M8-n record; doc 08
ADR-0013; doc 10 status + the debug.7 section + row 3 re-observation
(the row stays OPEN for the device confirmation); doc 07 §9 closing
paragraph; doc 09 handoff fields; DECISIONS.md D98; CHANGELOG Fixed
entry. One sandbox note: the first compile-matrix attempt ran the
arm64 full-tree link with the host gcc — the recorded matrix form
(CGO_ENABLED=0) is the lane's actual form; no app-facing
consequence. The device circle: row 3 now carries the fix and awaits
Ali's debug.7 confirmation; rows 4–8 and the cross-device UI verdict
still unreported. Next Task ID: M8-o. Next decision: D99.

## M8-o — the per-video stall loop closed (ADR-0014) + D99 (the APK naming rule) — 2026-10-18, later evening

Ali's debug.7 device round: the stall moved past the M8-n fix —
feed and UI instant, but EVERY video 1–2 minutes to start and
re-buffering mid-play, Windows instant on the same ISP ("so it's
NOT routing or method or even internet speed… it's something wrong
with the APK version"). Plus the standing naming rule: "mention
the version name in the apk. like v16.6-debug.apk" (recorded as
D99). The round: the every-video discriminator localized the
defect to per-hostname setup (each video = fresh rr*.googlevideo.com
hostnames; the feed's hosts are prewarmed and long-lived); the
whole flow path re-read end-to-end (the door, the tunnel relay and
the upstream dial are byte-identical on both platforms — one
engine, one resolver, one dialer); the surviving amplifier found:
racing cannot fix a fully-blackholed answer set, and the 120 s
minimum-TTL DNS cache serves the suspect set to every retry until
it rotates — the per-video wait and (URLs rotate rr hosts
mid-play) the mid-play re-buffering. Fixed as ADR-0014: a TOTAL
dial failure demotes the host's cached answer to stale
(doh.Resolver.Invalidate — the next retry re-queries the servers;
serve-stale preserved), wired at all three dial sites via an
optional-capability probe; the resolver's own bootstrap dial now
races its IPs (ADR-0013's dialer on the resolver's path). Seven
new tests pin it (4 doh + the new proxy dialfeedback_test.go's 3).
Baseline green on the fixed tree: svelte-check 0/0; vite clean;
dist CLEAN-synced; vet clean; 18/18 ×2; GOOS=windows exit 0;
compile matrix 4/4 + arm64 full tree. versionCode 6 → 7 (one
gradle wrinkle recorded: a stale daemon held the system JRE —
--stop + JAVA_HOME clears it); gomobile bind → younblock.aar
24,993,512 B; clean assembleDebug → app-debug.apk 34,978,761 B,
SHA-256 c0c4019ffd3fb9b458b67a2cbffdc62d8f1f6632a3007347a78022ab5b9cd5f6;
apksigner OK (the pinned c76bb7c6…8551d); badging versionCode 7 /
versionName 1.6.6. Released v1.6.6-android-debug.8 (regular,
Latest) with the APK asset NAMED v1.6.6-debug.apk (D99's first
application), served bytes re-downloaded and hashed byte-exact.
The honest ask rides the release notes: if any video still stalls,
the diagnostics report's tunnel lines (dns=/dial=/first-byte=/dur=)
+ the on-screen speed number at the slow moment are the required
inputs — they split the remaining suspects in one read. Row 3
stays OPEN for Ali's measured evidence. Next Task ID: M8-p. Next
decision: D100.

M8-p (2026-10-18, night; THIRTEENTH feedback-loop exchange; Ali's
debug.8 device round). Inputs: the per-video stall PERSISTED, and
the evidence path itself was missing — "there is no diagnostic
option in the apk app setting. add an option to copy paste logs and
diagnostics so i can paste it here" (recorded as D100). The round
therefore shipped the evidence-collection surface instead of a
third root-cause guess (ADR-0015): the Diagnostics screen (menu
entry + Settings row; Copy through the honest three-path clipboard
ladder, Share via the system sheet on the APK, the selectable text
as last resort), the Android shell bridge (PageBridge:
copyToClipboard/shareText, write-only, loopback-page-only), the
engine's SetReportExtra embedder hook carrying the facade's TUN
counters + the shell build identity inside every report, and the
report's log window doubled to 200 lines. Three new test pins
(TestEngineReportExtra, TestReportSectionWithoutTun,
TestReportSectionWithLiveTun — the last through the full serving
harness + socketpair TUN). Baseline green: svelte-check 0/0; vite
clean; dist CLEAN-synced; vet clean; 18/18 ×2; GOOS=windows exit 0;
compile matrix 4/4 + arm64 full tree. versionCode 7 → 8; gomobile
bind → younblock.aar 25,035,860 B (setShellVersion binding
verified in classes.jar); clean assembleDebug → app-debug.apk
35,026,961 B, SHA-256
1a2b2bf75553ea2b4d9d2eddc6ba8e2f694e40e2e1e47b25954cbb04c73d7833;
apksigner OK (the pinned c76bb7c6…8551d); badging versionCode 8 /
versionName 1.6.6. Released v1.6.6-android-debug.9 (regular,
Latest) with the APK asset named v1.6.6-debug.apk (D99), served
bytes re-verified. The ask for the debug.9 round: after the next
stall, Menu → Diagnostics → Copy and paste the report here — the
tunnel lines + the tun: line + the speed number split the remaining
suspects in one read; row 3 stays OPEN. Next Task ID: M8-q. Next
decision: D101.

M8-q (2026-10-18, late; FOURTEENTH feedback-loop exchange; Ali's
debug.9 device round). Inputs: "commit and push and new release. I
still dont see diagnostic button" — precise, and the root cause was
in the build, not the phone: the M8-p surface shipped ONLY into the
classic UI tree, but the APK bakes the pro screen (D66's dispatcher
default when VITE_UI_VARIANT is unset — build-android.sh never sets
it; only the desktop wails lanes bake classic). The phone rendered
the pro screen, whose lone diagnostics surface was the M6-era sheet
section buried below Help behind an android gate, with the raw
navigator.clipboard copy that silently fails in the WebView. The
round ported the surface into the served variant (ADR-0016) and
recorded the rule (D101 — a surface ships in the variant(s) the
edition serves; served-variant verification joins the ship
checklist): DiagnosticsView.svelte (NEW — the pro screen's
full-screen report view: auto-load, the cross-tree clipboard-ladder
Copy, Share behind canShare(), Refresh, honest inline status, the
readonly selectable textarea), the MenuSheet LEAD row (every
platform, no gate, 56px target; the old buried section + its broken
copy removed), the App.svelte wiring (diagOpen, focus return to the
menu button). Verification is the point of the round: the served
surface proven END-TO-END through the real webui server (cmd/core
--web, the same embedded dist + dispatcher the APK serves; mobile
viewport: menu → Diagnostics is the FIRST row → the real report
loads → Copy → honest "Copied" → Back; no console errors; ?ui=
classic re-verified intact). Baseline green: svelte-check 0/0; vite
clean; dist CLEAN-synced (placeholder restored); vet clean; 18/18
×2 + an uncached engine/mobile round; GOOS=windows exit 0.
versionCode 8 → 9; the full build-android.sh lane exit 0; the AAR's
embedded Go library verified to carry the new UI (diag-entry ×2 +
the row hint inside libgojni.so); app-debug.apk 35,053,265 B,
SHA-256 4264996ae39bb3e268bcf28913d3884d038c0833bb35ffd2233d60edd
51ef9e7; apksigner OK (the pinned c76bb7c6…8551d); badging
versionCode 9 / versionName 1.6.6. Released
v1.6.6-android-debug.10 (regular, Latest) with the APK asset named
v1.6.6-debug.apk (D99), served bytes re-verified. The ask for the
debug.10 round: TWO pastes — one now (its "android shell:
1.6.6 (versionCode 9)" line proves the round), one at the next
stall (the tunnel lines + the tun: line + the speed number split
the remaining suspects in one read); row 3 stays OPEN. Next Task
ID: M8-r. Next decision: D102.

M8-r (2026-10-18, late night — the FOURTEENTH feedback-loop
exchange; Task ID M8-r; ADR-0017; no new D — D102 stays reserved
for Ali's rulings). THE FIRST PASTED REPORT ARRIVED and the loop
worked exactly as D100 designed it: Ali's "the problem persists"
carried the full diagnostics report — the "android shell:
1.6.6 (versionCode 9)" line proving both the debug.10 install AND
that the M8-q surface rendered and copied on his phone. The report
read back against the code line by line (ADR-0017's Context holds
the full reconstruction): a startup secure-DNS blip (three ~170 ms
fast-failures, once) negative-cached www.youtube.com for the 15 s
negative TTL; the automatic heal sweep triggered by that same
failure probed that same name for all four methods and failed
instantly on the memory every time (ErrRecentlyFailed — no server
re-asked inside the window) while its own video handshakes were
completing; the sweep declared "no working method", the engine
degraded, and the "Couldn't fix it" notification fired 21 s after
Start on a recovered network. Collateral: the failed sweep left
method=legacy live (the last-tried preset, off the user's saved
choice), and the report's DNS lines were attr-stripped by the ring
(bare "dns server failed" — no reason). THE HOST REPRODUCTION IS
THE ROUND'S PROOF: TestHealSweepSurvivesFreshNegativeCache (a DoH
that fails exactly the first request, the dns and youtube steps
coupled on one name — production's shape) fails on pre-fix code
into the report's EXACT signature (Phase:degraded Method:legacy,
never ON) and passes with the fix (ON, method balanced, the baked
failure line in the report); TestFailedSweepRestoresPreviousMethod
and TestServerFailureLinesCarryEvidence (internal/doh) likewise
verified BOTH ways. The fixes (ADR-0017): the sweep resets the
resolver at start (the netmon's network-change semantics — a sweep
probes from zero), a failed sweep restores the previous method, and
the DNS failure/fallback/stale/tunnel-lookup lines bake server=#N
+ reason= into the message (the ring keeps only the message; the
configured NAME stays a file-log attr; no user-chosen strings
enter the report). The stall itself now has its first real
telemetry — five video flows that connected, handshook and then
moved ~7 KB in 51 s while other flows completed fine on the same
session (122 KB in ~1 s, 2.5 MB in ~7 s): ISP-side throttling of
the direct path to specific googlevideo cache nodes, plus
udpDropped=76 (the QUIC row-9 measurement) adding per-video start
latency — row 3 STAYS OPEN with a sharpened question for the next
report (reason=timeout vs reason=network on the startup blip); no
steering or policy change without a ruling. Baseline green:
frontend untouched (the M8-q dist carried over, verified intact,
tree clean); vet clean; 18/18 ×2; GOOS=windows exit 0; the Android
compile matrix in its exact form (arm64 full tree + non-main 4/4
ABIs). versionCode 9 → 10; the full build-android.sh lane exit 0
(disk managed — the Go build cache cleared first); the AAR's
embedded Go library verified to carry the new engine strings
("dns server failed server=#" in all three libgojni.so ABIs);
apksigner OK (the pinned c76bb7c6…8551d); badging versionCode 10 /
versionName 1.6.6. Released v1.6.6-android-debug.11 (regular,
Latest) with the APK asset named v1.6.6-debug.apk (D99), served
bytes re-verified. Next Task ID: M8-s. Next decision: D102.


## M8-s — the second pasted report acted on: the first-byte budget (ADR-0018) — 2026-10-18/19

Ali's debug.11 round: "the problem persists, but when after a while,
video plays... IN pc, it's instant" + the SECOND pasted diagnostics
report (versionCode 10, state on, method strong). The M8-r fixes
proven on-device (the sweep found its method 3 s in; the startup DoH
blip logged its baked reason=DNS_UNREACHABLE lines and self-healed —
the sharpened question answered: a transient, not a filter). The
stall re-observed with its exact shape: video-start waves where some
flows complete (first-byte 56-619 ms) while others dial fine, send
the ~1.7 KB hello, and never receive one byte — ~0.5 s closes and
EXACTLY ~60.0 s hangs, all down=0 B. Nothing in the engine closes at
60 s (idle 300 s; the TUN keepalive only probes) — the app's own
patience, amplified by an engine holding the dead flow open, with
the M8-o demotion never firing because the dial SUCCEEDED.

Fixed in debug.12 (versionCode 11, ADR-0018): FirstByteTimeout = 8 s
on processed hosts — silent origins fail the flow fast (both ends
closed, the client retries immediately, "tunnel first-byte failed
reason=..." lands in the report, and the suspect answer set is
demoted like a total dial failure). Unrouted relays keep the plain
shape byte-for-byte. Both-ways proof: the two new
firstbyte_test.go tests FAIL at ~5.0 s with the fix stashed (the
device's 60 s signature compressed) and PASS with it.

The question surfaced for Ali (D102 material, his ruling, NOT
decided): the sweep picks the method from its
redirector.googlevideo.com probe while the media flows hit
rr*.googlevideo.com nodes whose tolerance differs per node (method
strong on the phone, PC instant) — should the engine retry a dead
routed flow once with the NEXT preset? No steering without his
ruling. Next: M8-t expects the debug.12 report with
reason=/dur≈8000ms evidence.

## M8-t (2026-10-18/19, late) — the salvage re-dial (ADR-0019): dead
first-byte origins retried inside the flow

Ali's third pasted diagnostics report ("The problem persists",
versionCode 11 — the debug.12 build verified by its "android shell:
1.6.6 (versionCode 11)" line) proved the M8-s budget on-device
EXACTLY as designed: 22 "tunnel first-byte failed reason=timeout"
dones at ~8.3 s (the budget firing on silent origins) plus 18
reason=reset dones at ~0.5 s (the origin/path resetting right after
the fragmented hello lands) — the tuning evidence ADR-0018 asked for.
The counters named what remained: 38 errors / 61 conns — roughly half
the video-start waves die at first byte, and the app re-opens wave
after wave until one lands (~35 s from the first wave to the first
video bytes, down=4.4 MB once a flow survived). The serial chain of
8.3 s budget deaths IS the lag; the budget made each link cheaper
(8 s vs 60 s) but the chain itself is the app's.

Fixed in debug.13 (versionCode 12, ADR-0019): FirstByteTimeout
retuned 8 s → 2.5 s (every budget-killed flow was 100 % silent —
dead nodes, not slow ones; healthy routed origins answered in
420–508 ms, one 2399 ms outlier carrying the fragment-pause write),
plus the salvage re-dial: a dead first-byte origin is re-dialed
INSIDE the same flow — the engine closes only the dead origin,
demotes the answer set per dead attempt (the M8-o loop), re-resolves
skipping the flow's dead IPs, re-dials the SAME host with the SAME
preset (captured once per flow — no per-flow method variation, that
is D102, Ali's ruling), and replays the client's buffered bytes
byte-exact through a fresh fragment conn (first record
re-fragmented identically; early data rides after). Up to
FirstByteSalvageDials re-dials (default 2 → 3 total attempts,
bounded); the app never sees the failure. New report lines: "tunnel
first-byte retry reason=... attempt=N" (absorbed) and "tunnel
first-byte salvaged dials=N" (saved). Both-ways proof: the three new
firstbyte_test.go tests (recovers, exhausts to the M8-s terminal,
byte-exact replay) FAIL with the tunnel.go salvage stashed (the
M8-s shape at HEAD) and PASS with it; the two guards (unrouted
spared, negative knob = exact M8-s) pass both ways; the two M8-s
tests now pin the terminal shape with the knob disabled.

The environment note: the sandbox was wiped a SIXTH time before this
round (~/sdk and ~/go erased; the repo, node_modules, webui dist and
the old AAR survived). Toolchain reinstalled per doc 06 §1 pins and
verified: Go 1.27.2, javac 21.0.12.1 (Temurin), cmdline-tools 12.0
(the versioned 11076708 zip), platform-tools 37.0.1, platforms
26+36, build-tools 36.1.0, NDK 28.2.13676358, gomobile/gobind at
the pinned x/mobile. One operational note: the detached sdkmanager
run died mid-install (stale .temp, 2.5 GB) — cleaned, re-run in the
foreground to completion.

The D102 question SHARPENS but stays Ali's: the salvage re-flips the
same coin (same preset) up to twice more per flow; if the next
report still shows terminal first-byte failures clustering under one
method, the NEXT-preset retry (per-flow method variation) remains
his ruling. udpDropped=88 keeps measuring (row 9); the QUIC path
already fast-fails via ICMP port-unreachable, so it is not a
startup-lag contributor. Next: M8-u expects the debug.13 report
with retry/salvaged line ratios as the tuning evidence.


## M8-u (2026-10-19, early) — the fourth report ran the previous build: a no-build round

Ali pasted "The problem persists" with a fourth diagnostics report
generated 23:41:37 UTC — but the report names itself versionCode 11,
the PREVIOUS build (debug.12), the one debug.13 replaces. Identity
proven three ways: the PackageManager-read "android shell:" line
(authoritative — it names the installed build); 21 reason=timeout
dones at 8054-8070 ms, the old 8 s budget, which the 2.5 s build
cannot produce; zero retry/salvaged lines in the 200-line window.
And the release clock: debug.13 published 23:16:28 UTC — the report
was generated twenty-five minutes AFTER the fix shipped. The report
carries zero bytes of salvage-build evidence; it is the third
report's signature measured again (21 timeouts + 17 resets at
64-527 ms, 67 errors / 114 conns, ~24 healthy first-bytes at
58-548 ms — all inside the new 2.5 s budget with 4.5x headroom, DNS
ok 118 / fail 0, udpDropped=107, first video-data responses ~80 s
into the window).

No new build (D84: no app change, no bump — nothing to tune). This
round's work: debug.13 re-verified end-to-end (the release asset
re-downloaded through the API and hash-matched — SHA-256
18b4ea648b0aa02d7eebf84f2dc596d13894093a1469dfc4f111ddc5b286b518,
35,130,385 B; the embedded engine checked to carry both salvage
lines in libgojni.so in all three ABIs; Latest confirmed via
/releases/latest; the HEAD knobs re-read: FirstByteTimeout 2.5 s,
FirstByteSalvageDials 2), and the reply + the preview page walk Ali
through the install (the debug.13 release page specifically — D99
names every release's APK asset v1.6.6-debug.apk — install over
debug.12 in place, settings survive, then verify the report's shell
line reads versionCode 12).

The D102 question stands unchanged, reserved for Ali. Next: M8-v
expects the ACTUAL debug.13 report — shell line versionCode 12,
retry/salvaged line ratios as the tuning evidence.

D103 (same session, Ali's ruling — "please add debug13 in the apks
name, so such accidents does not happen"): all six D99-era APK
assets renamed in place through the API (v1.6.6-debug8.apk …
v1.6.6-debug13.apk — the shared v1.6.6-debug.apk name could not
tell the builds apart once downloaded); bytes unchanged (the
debug.13 asset re-served and hash-matched after the rename), every
affected release body updated with the new name + a rename note,
and the naming rule amended for every future release: the asset is
v<versionName>-debug<N>.apk, N = the release tag's build sequence.
The downloaded file now names its own build — the accident class
the fourth-report round found is closed. No app change; versionCode
stays 12; debug.13 stays CURRENT.
M8-v (2026-10-19, later — the fifth pasted-report round, the first
from the salvage build itself): the report's shell line read
versionCode 12 and its log carried the salvage's own lines, so the
round was a real fix round. The report proved the salvage on-device
(retry attempts absorbed, salvaged dials=1/2 saving flows at
0.7-1.4 s, errors 37/81 ≈ 46 % from 59 %) and named the two
remaining start-lag sources: the SERIAL salvage chain (21 terminal
reason=timeout dones at 8014-8227 ms = three 2.5 s budgets walked
one after another, + 11 terminal resets) and the mid-stream stall
(five video-start flows done in the same instant at 00:18:17.2,
dur=57290-64620 ms, first byte + ~7 KB down then total silence
until the app's own ~60 s patience killed them; the retry wave
right after answered in 60-261 ms — the video took ~65 s to start).
Shipped v1.6.6-android-debug.14 (versionCode 13, ADR-0020): the
salvage RACE (all remaining re-dials launched in parallel on the
rotated live answer set, first speaker wins, a silent-host terminal
costs ~2x the budget never 3x, the attempt count and preset policy
unchanged) + the stall watch (StallTimeout 15 s default: a
budgeted-flow relay silent in BOTH directions is closed engine-side,
answer set demoted, new report line "tunnel stall killed idle=Nms";
one-direction silence never fires; unrouted relays keep the plain
shape — D46). Both-ways proof: with tunnel.go stashed the stall
test, both race tests and the updated salvage test FAIL; with the
fix all 16 first-byte/salvage/stall tests pass, guards both ways
(the streaming-download guard caught an inverted-stamp bug mid-round
— the watch measures from the NEWEST activity clock). Baseline
green (vet, 18/18 ×2, GOOS=windows), compile matrix 4/4 ABIs +
arm64 full tree, gomobile bind with the stall string verified in
libgojni.so in all three ABIs, clean assembleDebug; APK 35,182,673 B
(SHA-256 154d40355bee702fcc751ead595688b5732abeb44cf186913dddbc5a9
9cad5e7), apksigner OK (the pinned cert), badging versionCode 13 /
versionName 1.6.6. D102 stays Ali's ruling, now LIVE (terminal
first-byte failures still cluster under method=strong). Next: M8-w
expects the debug.14 report — shell line versionCode 13; the
"tunnel stall killed" lines and the retry/salvaged-vs-terminal ratio
are the evidence.

M8-w (2026-10-19, late — the sixth pasted-report round, the first
from the salvage-race + stall-watch build): the report's shell line
read versionCode 13 and its log carried that build's own lines, so
the round was a real fix round. The report proved ADR-0020
on-device (the retry lines batch at the same millisecond — the
re-dials race; "tunnel stall killed idle=15059-15315 ms" — the
watch kills at exactly the 15 s budget; saves at 738-848 ms; the
60-second mid-stream hangs are GONE) and named the two remaining
costs: the SERIAL FRONT (six terminal reason=timeout dones at
5320-5576 ms — attempt 0 walked its 2.5 s budget ALONE before the
race launched, the two-budget chain — while CONCURRENT flows to the
SAME hosts answered at 72-162 ms in the same seconds: the dirt is
per-origin, not per-moment) and the DNS degraded state (dns fail 16
/ DNS_UNREACHABLE with the network flowing — a dirty-window DoH
blip negative-cached the probe name and the periodic check read the
15 s memory instead of asking any server; the M8-r lesson recurring
on the watchdog path). Shipped v1.6.6-android-debug.15 (versionCode
14, ADR-0021): the FIRST-DIAL RACE (new knob FirstByteRaceDials=2
default: the primary plus extra candidates dial in parallel from
the flow's start, each replaying the client's first record
byte-exact through a fresh fragment conn onto its rotated start of
the answer set, first speaker wins, an all-dead set terminals at
~1 budget instead of 2, new report line "tunnel first-byte raced
dials=N", a clean primary stays quiet; the client stays frozen for
the round — the M8-v salvage race's proven semantics; teardown-
caused deaths never demote, budget/reset deaths demote whenever
they happen — the M8-o loop; with racing enabled the salvage
rounds never run) + the honest dns probe (Resolver.ClearNegative
drops exactly the probe name's negative-cache entry before the
health lookup; success entries stay; the heal sweep keeps its full
Reset). Both-ways proof: with the raced round neutered
TestFirstDialRaceSavesDirtyPrimary and TestFirstDialRaceTerminalIs-
Bounded FAIL; with the ClearNegative call neutered
TestSelfTestDNSProbesPastNegativeCache FAILS; with the fixes all
pass, guards both ways (byte-exactness, the quiet primary, the
unrouted host, the disabled knob); the M8-v/M8-t/M8-s pins keep
their shapes with racing disabled in their configs. Baseline green
(vet, 18/18), compile matrix 4/4 ABIs + arm64 full tree, gomobile
bind with the raced string verified in libgojni.so in all three
ABIs; one lane defect caught by the artifact check itself (the
first assembleDebug packed the STALE libs/younblock.aar — the raced
string absent from the APK — plus 12 MB of incremental-repack zip
slack; the re-bind into android/app/libs/ + a CLEAN assembleDebug
produced the tight artifact); APK 35,233,585 B (SHA-256
817c5e2f7f553afcfc65cc684abfecb610c39c84ffdd875c0e3c169fc21ef68e),
apksigner OK (the pinned cert), badging versionCode 14 /
versionName 1.6.6. D102 stays Ali's ruling, still LIVE (terminals
still cluster under method=strong). Next: M8-x expects the debug.15
report — shell line versionCode 14; the "tunnel first-byte raced"
lines (the save rate) are the candidate count's tuning material,
the terminal ratio the D102 material, and the dns fail counter the
degraded-state confirmation.

## M8-x — the tiered stall watch (ADR-0022): the pooled video
connections stop being murdered at 15 s (2026-10-19, later still;
shipped as `v1.6.6-android-debug.16`, versionCode 15, asset
v1.6.6-debug16.apk)

Ali's seventh pasted report opened with the words that named the
regression himself: "I think the problem is worse now, because
videos used to have lag for starting to play, but after that you
could easily go next (watch next few minutes) instantly without
buffer, but now even going next and back in the same video lags a
lot." The shell line still read versionCode 13 — the debug.14 build
again; the device never ran debug.15 (the D103 accident class is
real: the wrong-build guard this time is the filename itself). The
report's log is the M8-v stall watch's smoking gun: thirteen
"tunnel stall killed idle=15xxx ms" lines in ~70 s, every one a
connection that had ALREADY carried real data — dur=17264 ms
down=2.1 MB, dur=16756 ms down=1.6 MB, 723.6 KB, 633.7 KB,
162.6 KB, plus 20.9/23.4/9.2/7.4/5.3/4.4/3.6/2.8 KB — murdered at
exactly the lean 15 s budget mid-pacing. The player POOLS its video
connections (buffer, quiet past 15 s, next range on the SAME
connection); killing them forced every next/seek through a fresh
~50 % dirty dial (reset terminals ~1.2 s, timeout terminals
~5.4 s; errors 70/122 ≈ 57 %), and each kill ALSO demoted a
proven-good answer set and counted an error — the watch was
poisoning the very origins that had just delivered megabytes. The
method line flipped to balanced (the heal sweep settling after DNS
false-failures — M8-w's ClearNegative attacks the cause); dns
ok 176 / fail 4 (down from 16).

ADR-0022 (doc 08) tiers the watch, scoped exactly as M8-v
(processed hosts only, D46): a flow PROVES itself by payload
(flow.down ≥ StallRichBytes, default 32 KB — past handshake +
headers into real payload; the murdered cohort started at 162.6 KB,
the debug.13 dead cohort topped out ~7 KB with one 121 KB outlier)
or by pacing (the watch observes an activity gap ≥ StallTimeout/5
= 3 s that CLOSED — fresh bytes flowed after it; the shapes overlap
in bytes but differ in RHYTHM: the dead cohort never woke). A
proved flow's budget escalates to StallRichTimeout (default 90 s —
well past every real pacing gap, well under the 300 s idle
timeout) and its close is QUIET: both ends closed, NO demotion, NO
error count (hygiene, not a dead-origin signal). The lean tier
keeps the exact M8-v death (demotion + error + the fast 15 s
close) for the never-proved shape. The stall line now carries
"down=N tier=rich|lean" (the prefix stable — the next report's
primary tuning evidence). Knobs StallRichTimeout (negative or ≤
StallTimeout → no escalation, the exact M8-v shape) and
StallRichBytes (negative → only a survived gap escalates) wired
through Config normalization + the engine.

Proof: stalltier_test.go (4 new tests) BOTH WAYS — with the
tiering stashed, TestStallRichFlowSurvivesLeanBudget FAILS (the
payload flow closed at the 400 ms lean budget — the regression
reproduced in test) and TestStallWakeEscalatesBudget FAILS (the
pacing flow closed at lean right after its second flight); the
lean guards fail pre-fix on the new line format only (behavior
identical both ways); with the fix 4/4 new + 4/4 untouched M8-v
pins PASS. Environment wiped a seventh time and restored per doc 06
§1 (Go 1.27.2, Temurin JDK 21.0.12.1 with javac at ~/sdk/jdk — the
system JRE lacked it, cmdline-tools 12.0 via the versioned URL,
platform-tools 37.0.1, platforms 26+36, build-tools 36.1.0, NDK
28.2.13676358, gomobile/gobind at the pinned x/mobile). Baselines:
vet clean; 18/18 ×2; Windows exit 0; matrix 4/4 + arm64 full
tree; svelte-check 0/0; vite build + dist sync (one tracked
placeholder.txt deleted by an rsync --delete — restored; the lane's
own script copies without --delete). Android lane: versionCode 15
with the full round comment; go clean -cache first; gomobile bind
INTO android/app/libs/younblock.aar (25,178,137 B, the tier=
string verified in libgojni.so in all three ABIs BEFORE gradle —
the M8-w lesson); ./gradlew --stop + CLEAN assembleDebug. APK
35,235,345 B (SHA-256
e107e337245b89e048e7025e1a6daaab083a9da170d8660f98b55874bdddac5f),
apksigner OK (the pinned cert), badging versionCode 15 /
versionName 1.6.6 / minSdk 26 / targetSdk 36; the tier= + raced +
stall strings verified in the APK's libgojni.so in all three ABIs.
The build carries debug.15's never-run cargo unchanged (the
first-dial race + the honest dns probe) — the next report must
carry BOTH signatures. D102 stays Ali's ruling, still LIVE
(terminals cluster under strong AND balanced now). Next: M8-y
expects the debug.16 report — shell line versionCode 15 FIRST
(anything else means the old APK is still installed); the tier=
split on the stall lines is the tuning material, the raced lines
the save rate, the dns fail counter the degraded-state
confirmation.

## M8-y — the negative re-ask + the origin-IP penalty box (ADR-0023): the start lag's two memory failures fixed

Ali's EIGHTH pasted report (2026-10-19T12:53:37Z) — the first from
the right build: the shell line reads versionCode 15, the debug.16
build, carrying both prior signatures (the raced line and the tier=
stall lines) — closed ADR-0022 in his own words ("The original
problem (videos taking time to play) persist, but the problem of
easily going next and back in same video instantly is fixed") and
measured what remains. Two sources, both memory failures:

1. **The DNS negative-cache lockout** — dns ok 142 / fail 50,
   doorFailures=50: every resolver failure a dead door dial, the
   failure lines pairing at the same millisecond (four pairs inside
   22 ms — instant ErrRecentlyFailed hits, not server walks). One
   DoH blip negative-cached a batch of fresh per-video hostnames and
   the 15 s TTL killed every retry wave at the door.
2. **The dead-origin re-gamble** — ~17 timeout terminals at
   2892–4304 ms + ~8 resets through the raced round; the second
   video's start (a NEW rr hostname) paid the full discovery cost
   again because the dead-IP knowledge died with each flow while the
   authoritative answer returned the same blackholed cache-node IPs.

ADR-0023 (doc 08) gives the engine two memories: the NEGATIVE RE-ASK
(a DNS_UNREACHABLE negative re-asks the servers through the
singleflight, rate-limited to one walk per DNSCacheNegativeReask=1 s
per host, the failure's own walk counting as the window's first
probe; server-verdict negatives keep the instant fail; new line
"dns negative re-ask (path failure, not a name verdict)") and the
ORIGIN-IP PENALTY BOX (every classified first-byte death marks its
IP for DeadIPPenalty=60 s; every dial list skips the marked IPs; a
preference never a verdict — an all-penalized list dials in full;
not marked: stall kills, client-side deaths, teardown-canceled
deaths, dial failures; the knowledge crosses flows AND hostnames).
Plus the baked reason on the socks5 door's DNS line (the report's 50
bare lines could not say why). udpDropped=99 is the M8-j QUIC
fast-fail working — no action.

Files: internal/doh/resolver.go, internal/doh/reask_test.go (NEW),
internal/doh/resolver_test.go (TestCacheNegativeTTL rewritten — the
old pin WAS the lockout), internal/proxy/server.go,
internal/proxy/tunnel.go, internal/proxy/socks5.go,
internal/proxy/deadip_test.go (NEW), internal/config/defaults.go,
internal/engine/engine.go. Both-ways proofs executed: the re-ask
neutered → TestNegativeReaskRecoversAfterBlip FAILS with the
report's exact signature ("lookup failed recently:
DNS_UNREACHABLE"); the marking stashed →
TestDeadIPPreferenceSparesTheNextFlow FAILS (flow 2 EOF on the
corpse) and TestRacedDeathsMarkTheIP FAILS (corpse accepts 5 ≠ 3);
all guards pass both ways. Baseline green: vet, 18/18 ×2, Windows
exit 0, matrix 4/4 ABIs (CGO+NDK) + arm64 full tree. The
environment survived this round (no eighth wipe; one detour: the
Edit tool's fuzzy whitespace matching mixed spaces into the
tab-convention files — caught by gofmt -l against HEAD, gofmt -w on
the touched tab files; tunnel.go's space convention verified
untouched). Shipped as v1.6.6-android-debug.17, versionCode 16,
asset v1.6.6-debug17.apk (D103), APK 35,279,565 B, SHA-256
f40d03a09a896a6ea3cfcad4e081ba36e3e62430e55b23b69640b4a9f6577018,
the pinned debug certificate. The next report expects the shell
line versionCode 16 FIRST (anything else means the old APK is still
installed); the re-ask lines count the lockouts absorbed, the dns
fail counter should collapse from 50, the baked reasons name the
failing DoH leg, and the terminal cohort (was 17+8) is the penalty
box's tuning material.

## M8-z — the cold-start round (ADR-0024/0025/0026/0027, debug.18, versionCode 17)

Date: 2026-10-19 (Asia/Tehran), night. Input: Ali's spec
`docs/specs/debug18-cold-start.md` (saved into the repo as instructed)
— the debug.16 evidence named the plain cohort (only 7 domains
fragmented; every other host plain-relayed with NO timeout but the
300 s idle), the DoH burst shape (one handshake per concurrent fresh
name, serial 3 s server walks), and the unmeasured QUIC fallback.

What was done, in the spec's §9 order (A, B, E, C, D):

- **A (ADR-0024)**: settings schema v5 + `fragmentAllRouted`
  (platform-default through `config.LoadFor(dir, platform)`: true on
  Android, false on desktop; a pre-v5 file migrates once on Android,
  a v5 file is the owner's word). `Server.fragmentHost` /
  `Server.budgetedHost` split; both doors pass fragment+budgeted into
  `runBidirectionalCopy`; the budgeted gate is `budgetedHost &&
  FirstByteTimeout > 0`. fragmentall_test.go (5 tests) +
  settings_v5_test.go (5 tests), both ways (fragmentHost neutered →
  the fragmentation test FAILS with one read).
- **B**: `plainGuardConn` — a read-side wrapper arming
  PlainFirstByteTimeout=8 s until the origin's first byte, closing
  both sides + counting + the `tunnel plain first-byte failed
  class=plain reason=timeout` line on timeout; no dead-IP mark, no
  connError. plainnet_test.go (4 tests), both ways. **The soak test
  caught a real defect the same day: the guard's embedded-interface
  method set hid CloseWrite — the client half-close was lost and
  every guarded flow hung to the idle timeout. Fixed by forwarding
  CloseWrite explicitly (the ADR documents it).**
- **E (ADR-0027)**: class= + rtt-class= on every tunnel line;
  stats.HostTable (cap 256, least-recent eviction, memory-only) +
  Top-10 report section (host names redacted unless logDomains);
  plain-relay summary; DNS detail with phases (phaseError: tcp/tls
  from the dial, status/http/parse from queryOne); DoH counters
  (dials/reuse/hedgeWins by server); the one-shot dead-origin probe
  (proxy.PenalizedIPs + ProbeDeadOrigin, while On only, no IPs in
  the report); `Engine.Mark(label)` + MarkJSON + gomobile Mark +
  POST /api/mark (webui) + the Wails binding + the Diagnostics
  screen's "I pressed play"/"First frame" buttons + the timeline
  section with the last pair's elapsed and window deltas;
  `DEBUG BUILD: hostnames included for diagnosis` as the report's
  first line when config.DebugTesterBuild && hostnames (the debug
  lane sets the flag via -ldflags in build-android.sh; the code
  default is the release value, false); marks/probe reset on Stop.
  report_sections_test.go (4 tests), both ways.
- **C (ADR-0025)**: `http2.Transport` with `DialTLSContext`
  (dialServer = the old dialBootstrap TCP+fragment leg; the TLS
  handshake here: ServerName from the server URL's host, ALPN h2,
  cfg.TLS as the test seam); ReadIdleTimeout 15 s / PingTimeout 5 s;
  hedged `askServers` (sticky at t=0, next at +DoHHedgeDelay=300 ms,
  fast failures trigger the next launch immediately, first valid
  answer wins, cancelled losers stay quiet; a hedge win = the winner
  was launched while an earlier server was still pending — the
  serial fallback after a settled failure is not one, pinned by
  test); WarmProbe at goOn/goDegraded + on the 5-minute cadence +
  re-warm within 1 s of a tcp/tls-phase failure (5 s debounce);
  googleVideoCode learning (rr/r→sn-<code>, cap 64, persisted to
  `googlevideo-codes.txt` — the DECISIONS.md privacy check is in the
  ADR: R21 governs logs/report, the file is private app state like
  settings.json and the usage store, and the report renders only its
  count); engine 5-minute prewarm loop (rr1..rr5, ≤3 in flight).
  http2_test.go (7 tests), both ways (per-request transport stash →
  the burst test FAILS with 10 dials; hedge-clock stash → the hedge
  test FAILS at the full timeout). The doh/engine/health/mobile/
  proxy test fakes now negotiate h2 (httptest EnableHTTP2 /
  NextProtos) — the one necessary harness change.
- **D (ADR-0026)**: the ICMP 3/3 reply stays gVisor's — the spec's
  §7.1 "write the host test first" step already exists as
  TestUDP443FastFailICMP (M8-j, green), so per the spec's own rule no
  explicit writer was added; ADDED: `udp443 dropped` line, the
  next-TCP-flow `udp443 fallback elapsed=` line (host only under
  LogHostnames, one-shot memory capped at 128), and `v6Dropped`
  counted at the wire pump (rides the facade's tun line).
  udp443_test.go (4 tests), both ways.
- **Version**: mobile facade `Version: "1.6.6-debug.18"` (the report
  header and appinfo name the round); gradle versionCode 17 with the
  round's comment block; build-android.sh passes
  `-ldflags="-X …/internal/config.DebugTesterBuild=true"` on the bind
  (the debug-tester lane's mark — a release lane omits it).
- **Baseline and verification**: full `go test ./...` green (18
  packages + mobile — after building the frontend bundle this
  source-only checkout lacked). Race detector: internal/doh CLEAN,
  internal/engine, internal/tunstack, internal/stats CLEAN; internal/
  proxy clean of data races — its full-package -race run flakes the
  PRE-EXISTING stall-timing tests in this slow sandbox, verified
  IDENTICAL at HEAD (git stash -u → same failures), and those tests
  pass under -race when not competing with the 5000-tunnel soak for
  CPU. go vet ./... clean. gofmt: tab files normalized; the space-
  convention files (tunnel.go, flowtiming_test.go, settings_test.go,
  health_test.go) kept in their exact expand(gofmt) shape (verified
  by diff against HEAD before and after).
- The build lane and the APK artifact are recorded in the release
  notes (doc 10) and the next section.

### The debug.18 build lane record

Environment built this round (user-space, no system changes): Temurin
JDK 17.0.20.1+1 (~/.local), Android cmdline-tools + platform-tools +
platforms;android-26 + platforms;android-36 + build-tools;36.1.0 +
NDK 29.0.14206865 (~//android-sdk), gomobile@latest + gomobile init.
One lane defect found and fixed at the source: `go tool -X` only sets
STRING variables — `config.DebugTesterBuild` was a bool, so the bind
failed with "cannot set with -X"; the flag is now a string lane
marker ("true" from the debug lane's ldflags, "" the release default)
with an `IsDebugTesterBuild()` accessor, and the tests follow the
string shape.

Steps: frontend `npm ci && npm run build` → bundle copied into
internal/webui/dist (a source-only checkout starts without it — the
mobile lifecycle tests need it) → `gomobile bind` (3 ABIs, min API 26,
the debug-tester ldflags) → the M8-z strings verified in libgojni.so
per ABI BEFORE gradle → CLEAN `./gradlew assembleDebug` (AGP 9.2.0
downloaded build-tools 36.0.0 itself; licenses pre-accepted). Result:
app-debug.apk 55,059,385 B, SHA-256
9bb9707613bac71aa8680c2a3b545c2579145851884d9f321005028f92d4ecd5,
apksigner OK on the pinned debug certificate (c76bb7c6…8551d), badging
versionCode 17 / versionName 1.6.6 / minSdk 26 / targetSdk 36. The
named asset is android/release/v1.6.6-debug18.apk (D103).

Final verification: `go vet ./...` clean; `go test ./...` 18 packages
ok, 0 failures. Race detector: doh/engine/tunstack/stats CLEAN;
proxy: zero data races, with the pre-existing sub-second stall-timing
tests flaking under full-suite -race load in this CPU-starved sandbox
EXACTLY as they do at HEAD (verified by `git stash -u` → identical
failures); they pass under -race when not competing with the soak.

## M8-aa — the dead-origin-pool round (ADR-0028/0029/0030, debug.19, versionCode 18)

Date: 2026-10-20 (Asia/Tehran), after midnight. Input: the tester's
FIRST debug.18 report (versionCode 17, the cold-start build, pasted
2026-10-19T22:55Z) — "The problem persists."

What the report said, read through the cold-start spec's own §12
grid: the plain lane the debug.18 round fixed is HEALTHY
(plain-relay flows=6 firstByteTimeouts=0 worstFirstByte=295ms, zero
class=plain rows in the Top-10 — H1 disproven, A+B held), DoH is
healthy (dials 4 / reuse 48 / hedgeWins 0, dns ok 137 / fail 0 — H2
fixed), the QUIC fallback instant (60 drops, elapsed 0–3 ms mostly),
and the one-shot dead-origin probe answered SILENT on both
sni=www.google.com and sni=none — per the spec's grid, "the IP is
blackholed and the penalty box is right". What remained was ALL
38 errors on 70 conns in class=routed: ~20 first-byte timeout
terminals at exactly one 2.5 s budget (dur≈2.6 s, down=0 B — the
all-dead raced round), ~6 resets, THREE lean-tier stall kills at
exactly idle=15.3 s each carrying 5.0–7.4 KB of half-finished
responses (the mid-response corpse shape), the top host at flows=13
failures=8 (the app's retry waves re-rolling the same mixed
live-and-blackholed answer set faster than the 60 s penalty
lapsed), two hosts at failures=flows with worstFirstByte=0 ms, and
`timeline: no marks` — the tester never pressed the buttons, so the
one number the spec called "the single number that says whether the
work succeeded" was missing. The minute the tester still waited ≈
(the app's serial retry waves × 2.6 s) + (3 × 15 s stalls) — the
measured cold-start cost, in the routed lane alone.

What was done, in the order the evidence demanded:

- **The verified prewarm (ADR-0028, the pool fix).** The 5-minute
  googlevideo prewarm loop (plus one pass the moment the engine
  goes On) now runs an origin-probe pass through the running proxy:
  for each name in priority order — the stable OriginProbeNames
  (youtubei.googleapis.com, www.googleapis.com, www.youtube.com,
  youtube.com, i.ytimg.com, yt3.ggpht.com) first, then the freshest
  learned sn-codes' rr1..rr5 — resolve through the same resolver the
  flows use, then verify every answer IP that is neither
  verified-live nor penalized right now, at most 32 per pass, at
  most 3 in flight, each inside a 2.5 s budget. The probe sends the
  EXACT wire shape a real flow sends: one fragmented ClientHello
  (the live preset, captured once per pass — D102's discipline)
  with the prewarmed name as SNI, through the engine's one netx
  dialer, tls.Client with InsecureSkipVerify — only whether bytes
  come back matters. "answered" → markGoodIP for 15 min (liveIPs
  puts verified-live IPs at the FRONT of every dial list);
  "silent"/"reset" → markDeadIPTTL for 10 min. Android-only (the
  platform gate lives in the engine — the desktops keep the M8-z
  prewarm byte-for-byte, hard rule 5, and the stub platform every
  host test builds on never dials a probe).
- **Strike escalation (ADR-0028, the re-gamble fix).** Every mark on
  an already-known origin IP escalates its penalty TTL to the
  DeadIPStrikeEscalation floor (60 s → 5 min → 30 min): the report's
  13-flows-for-8-failures host stopped being re-learned every 60 s.
  The box stays a PREFERENCE, never a verdict — an all-penalized
  list dials in full (pinned by test).
- **The raced-path salvage round (ADR-0028, the wave fix).** When a
  raced round dies on every candidate it dialed, the flow computes
  the answer set's UNTRIED origins (a fresh resolve minus the IPs
  the round dialed minus everything the box penalizes —
  verified-live first) and, when any exist, races ONE more round
  over exactly those before the terminal: the replay stays
  byte-exact, the client stays frozen (the M8-v proven semantics),
  the app never sees the failure. A set with nothing untried keeps
  the fast M8-w terminal — the report's 100%-failure hosts keep
  their ~2.6 s death. The candidate core was extracted into
  raceIPList and shared with the serial path's own salvage (the
  M8-v machinery, byte-for-byte); raceFirstByte now takes its answer
  set explicitly and reports the IPs every candidate dialed.
- **The Android lean stall budget, 15 s → 8 s (ADR-0029, the stall
  fix).** The three mid-response corpses each held the app's retry
  for the whole 15 s desktop budget — 45 s of the minute. The gate
  lives in the engine's startListener (android →
  StallLeanTimeoutAndroid=8 s; every desktop keeps 15 s
  byte-for-byte); the proxy's tiered watch semantics moved nowhere.
  The report now prints the budget that fired
  (`stall kills: lean=N rich=N leanBudget=8s|15s`).
- **The auto-timeline heuristic + the M8-aa counters (ADR-0030, the
  proof fix).** A 500 ms poller over the counters the flows already
  feed: a routed-flow burst after ≥5 s of routed quiet stamps "auto:
  play-suspected", a ≥100 KB down-burst pairs it with "auto:
  first-data" — both stamps ride the log ring (AUTO-MARK lines), the
  report renders the last pair's elapsed time and count, labeled
  "heuristic; the buttons stay authoritative". The report also gains
  the origin-probe line (passes/ips/answered/silent/reset plus
  good/penalized/strikes — counts only, R21), the tier split with
  the budget, and the racedSaves/salvagedSaves/routedFlows counters.
- **Version:** mobile facade Version "1.6.6-debug.19"; gradle
  versionCode 17 → 18 with the round's comment block; the APK asset
  is v1.6.6-debug19.apk (D103); build-android.sh's debug-tester
  ldflags lane unchanged.

Tests (both-ways, the ADR-0019…0023 discipline):

- internal/proxy/originprobe_test.go (NEW, six tests, four shapes
  plus two guards): the probe-cleaned pool (the corpse never dialed
  by the flow, the record delivered byte-exact, no terminal line);
  the neutered control (DeadIPPenalty=-1, the exact M8-x box-off
  shape — the same fixture gambles the corpse and terminals, the
  report's own pre-fix signature); the raced salvage over the
  untried origin (retry + salvaged lines, no terminal, the live
  origin dialed exactly once); the fast-terminal guard (nothing
  untried → the M8-w terminal, no salvage, no second budget); the
  strike escalation (live=2 escalated=1, the all-penalized list
  dials in full); and the probe line's R21 pin (counts only, no IP,
  no name).
- internal/engine/m8aa_test.go (NEW, three tests): the android 8 s
  vs desktop-shaped 15 s lean budget; the auto-timeline pair from
  the counters (burst → stamp → burst → pair → report → Stop reset);
  and the full M8-aa section render with the R21 no-leak guards.
- Both-ways runs this round: tunnel.go stashed (the debug.18 shape)
  → `TestRacedSalvageCoversTheUntriedOrigin` FAILS with "read echo
  (the salvage must save the flow): EOF" — the exact terminal the
  debug.18 report showed, the live origin never dialed — while
  every guard and the probe tests pass on both sides; the engine's
  M8-aa additions stashed → all three engine tests FAIL on the
  missing sections (the ADR-0027 section-render proof shape); the
  probe-marking's both-ways is the in-test neuter (shapes 1 vs 2 on
  the same fixture, one flip of the box knob).
- Baseline and verification: `go vet ./...` clean; `go test ./...`
  18 packages + mobile ALL GREEN (0 failures). Race detector:
  doh / engine / stats / tunstack CLEAN; the touched proxy surface
  (all new tests + the deadip/firstrace/stalltier/salvage pins)
  CLEAN; the full-package proxy -race run flakes on the SAME
  stall-timing tests that flake at HEAD in this CPU-starved sandbox
  (verified by `git stash -u` → identical failures; ZERO data-race
  reports anywhere).

### The debug.19 build lane record

(Sandbox was wiped between rounds: re-cloned the repo, reinstalled
Go 1.26.3 to /home/z/.local/go, rebuilt the frontend bundle,
reinstalled the Android lane — system JDK 21 (the D74 pin),
cmdline-tools, platform-tools, platforms;android-26/36,
build-tools;36.1.0, NDK 29.0.14206865, gomobile@latest + init.)

The lane: Temurin JDK 21.0.12.1 (the sandbox's system JDK was a
JRE-only install; the D74 pin is JDK 21), cmdline-tools +
platform-tools + platforms;android-26/36 + build-tools;36.1.0 +
NDK 29.0.14206865 at /home/z/android-sdk, gomobile@latest + init.
The bind (3 ABIs — android/arm, android/arm64, android/amd64 —
min API 26, the debug-tester ldflags
`-X …/internal/config.DebugTesterBuild=true`) produced
android/app/libs/younblock.aar, and the M8-aa signatures were
verified present in libgojni.so in ALL THREE ABIs BEFORE gradle (the
house ritual): `origin probe: names=`, `AUTO-MARK play-suspected`,
`stall kills: lean=`, `auto-timeline: no pair yet`,
`1.6.6-debug.19`, `first-byte retry class=`, and
`tunnel first-byte salvaged class=`. Clean `./gradlew
assembleDebug` (the wrapper's exec bit restored to the committed
mode after the fresh clone). APK 55,050,833 B, SHA-256
`6659864ebcc10a54dfe304673ca49db1cd7e3a53d80f866de28f9c74fff051e4`,
apksigner verify OK on the PINNED debug certificate
(`c76bb7c61ec623892eabc37ac90ac1bc0041d39490f2d48073676d6e2ed8551d`
— installs over every earlier debug build, D85), aapt2 badging
versionCode 18 / versionName 1.6.6 / targetSdk 36 (minSdk 26). The
named asset is android/release/v1.6.6-debug19.apk (D103) — kept out
of the tree like every round's APK, uploaded as the release asset.

Final verification: `go vet ./...` clean; `go test ./...` 18
packages ok, 0 failures.

## M8-ab — the escape-the-pool round (ADR-0031/0032/0033, debug.20, versionCode 19)

Date: 2026-10-20 (Asia/Tehran), early morning. Input: the tester's
FIRST debug.19 report (versionCode 18, the dead-origin-pool build,
pasted 2026-10-20T00:54Z) — "The problem persists."

What the report said, read through the round's own instruments: the
dead-origin pool WORKS — racedSaves=9 (racing lands mixed-window
flows on live origins), the box and good table healthy (probe
answered=32/32 on the one pass, box strikes=6, the one-shot
dead-origin probe SILENT on both SNI variants — IP-level blackholing
confirmed again), the plain lane and DoH healthy, the QUIC fallback
instant. But the one number that matters — auto-timeline
play-suspected -> first-data **elapsed=44999ms pairs=5** — says five
app retry waves paid the whole discovery cost anyway, and the code
read says WHY: liveIPs, the raced extras, and the untried salvage all
draw from the SAME hostname's rotating answer set, so the 24
verified-good origins sat in a table no dial list of the failing
hostnames could reach; the video started only when the app itself
fell back to a foreign cache hostname (r5---sn-4g5e6nsy /
r2---sn-nv47lnsr — the only googlevideo hosts with clean first-bytes
in the window). The pass's own name list explained the pool's bias:
stable fronts + rr1..rr5 of the 2 FRESHEST learned codes — early in
a session, the local cluster twice. And markDeadIPTTL never dropped
the good mark its ADR-0028 comment claimed it dropped (the latent
leak: dead IPs resurfacing at the FRONT of dial lists once the box
lapsed).

The round:
- **ADR-0031 (the substitution + the hygiene)**: goodMark{until,gvideo};
  probe verdicts AND flow first-byte successes mark origins good (the
  self-feeding pool); the raced round's substitute candidate (googlevideo
  flows only, pool origins not in the current answer set and not boxed,
  freshest verdict first, SubstitutionPoolDials=3 walked, flat dial
  count, SNI unchanged, knob GoodIPSubstitution); the substitute win
  line + substWins/substAttempts; substitute deaths demote the pool
  origin, never the host's answer set; markDeadIPTTL drops the good
  mark (the demote, actually implemented).
- **ADR-0032 (coverage + cadence, android-gated lanes)**:
  probePassNames round-robin over the code store (one rr name per
  code, rr index rotating, ProbeCodesPerPass=12, PerPass 32→48);
  fast-start cadence (45 s passes while <5 min old AND gvideo pool
  <8); churn kick (gvideoTerminals in failFlow; the gvideo loop's 1 s
  watch kicks an immediate prewarm+pass, ProbeChurnKickDebounce=10 s).
- **ADR-0033 (persistence)**: googlevideo-good-ips.txt (cap 32) —
  seeded at listener start with GoodPoolSeedTTL=5m, written on change
  (15 s check) + Stop-time flush; loadGoodIPPool tolerates
  missing/corrupt files; stale seeds self-correct through the demote.
- **Diagnostics**: `substitution: attempts=N wins=M pool=G (gvideo V)`
  report line; gvideoTerminals on the counters line; "tunnel
  first-byte substituted class=… dials=…" log line (LogHostnames-gated
  host like every tunnel line).
- **Tests**: proxy/substitution_test.go (6: the escape, the neutered
  control, the self-feeding pool, the demote, the churn signal, the
  line's R21 privacy) + engine/m8ab_test.go (3: the rotation pure
  function, the section render with no-leak guards, the persistence
  round-trip). Both-ways: substitution neutered → the escape and the
  self-feeding tests FAIL (the terminal signature / the empty pool);
  the demote neutered → good count 1; the churn counter neutered →
  gvideoTerminals 0. Full suite GREEN (18 packages + mobile);
  race-clean on the touched lanes.
- **Version:** mobile facade Version "1.6.6-debug.20"; gradle
  versionCode 18→19 with the round's comment block; release notes /
  CHANGELOG / README updated; the webui dist rebuilt from frontend
  (the known fresh-checkout gap) before the mobile tests.

## M8-ac — the hostile-network round (ADR-0034/0035/0036, debug.21, versionCode 20)

Date: 2026-10-20 (Asia/Tehran), evening. Input: the tester's FIRST
debug.20 report (versionCode 19, the escape-the-pool build, pasted
2026-10-20T14:31Z, tested at 18:04 Tehran — evening peak; the
debug.19 test was 04:17 — off-peak): "It became way way worse, it
doesn't even play a video after a while!!"

What the report said, read against the 13.5h-earlier debug.19 one:
the NETWORK REGIME FLIPPED between the two tests — the one-shot
dead-origin probe went silent→ANSWERED on both SNI variants (the IP
blackholing is gone), probe reset verdicts 0→35, DoH dials 6→59 (the
resolver's warm connections being killed), down 6.3MB→1.3MB. The
debug.20 machinery itself WORKED under it: substitution attempts=7
wins=5, first bytes 70-524ms everywhere, auto-timeline
play→first-data 2500ms pairs=2 (the 45s cold-start SOLVED early in
the session) — then the session decayed: 7 lean kills each carrying
6-15KB of partial responses, www.youtube.com 17/17 flows failed with
worstFirstByte=282ms (connections opened, answered, then died
mid-flight), gvideoTerminals=17, the DoH servers failing in waves,
dns=579-846ms late in the log. The app can't complete its API calls
→ playback never starts.

The self-harm diagnosis (five interactions of the debug.20 choices
with the new regime): the box held penalized=12 while the dead-origin
probe said those IPs ANSWER (deaths are temporal now, not per-IP);
the flow-fed pool drained to gvideo=1 (17 terminals' all-dead rounds
demoting the very marks the wins wrote — substitutes starved);
liveIPs pinned every flow onto the same verified origin
(www.youtube.com 17/17, the good-front plus raced rotations all
inside the live prefix); every raced death invalidated DNS through
the dying DoH lane (a churn spiral); the 45s fast-start cadence sent
scan-like bursts exactly while the network was hostile (the report's
own decay window correlates with the tick).

The round: (1) ADR-0034 — markDeadIPClass (reset → 20s box, no
demote; else 60s + demote), probe resets → 60s, escalation
[60s,2m,5m], the pending/idle stall demote (pending demotes at both
tiers — dead with work outstanding; idle never demotes), the probe
rehabilitation (4 boxed IPs/pass re-verified, answered → cleared +
good + counted, neutral-SNI marks never gvideo-flagged), and the
front rotation (liveIPs + goodSubstitutes rotate per call through
one server-wide counter); (2) ADR-0035 — the fast-start lane
removed, the churn kick kept (debounced), and the dohBrownout tracker
(≥3 dials/60s pauses the background lanes until 60s of quiet,
counted, injectable clock); (3) ADR-0036 — cause=pending|idle on the
stall lines, the pending/idle counter split, rehabilitated=/goodDrops/
brownouts= on the report.

Tests: proxy/flowkill_test.go (5: the rotating front, the reset-class
mark kept + brief box, the pending demote — the request written
INSIDE the wake gap so the M8-x pacing proof doesn't escalate the
flow to rich first (a sub-tolerance discovery: a write that closes a
≥wakeGap gap wakes the flow; the rich tier's pending kills demote
too, but the lean budget is the fast path), the idle guard, the
rehabilitation) + engine/m8ac_test.go (2: the brownout tracker over
an injected clock, the section render with no-leak guards). Both-ways:
rotation neutered → both rotation tests FAIL; reset-class neutered →
the mark test FAILS; pending demote neutered → the demote test FAILS;
rehab neutered → the rehab test FAILS; brownout neutered → the tracker
test FAILS. Full suite green (18 packages + mobile);
TestStallKillSparesStreamingDownload flaked 1/10 solo with the round's
diff vs 0/10 at HEAD (n=10, same failure family the worklog has
documented flaking under this CPU-starved sandbox for three rounds;
the full package passes; the diff touches nothing in the first-byte
path the failure lives in — recorded honestly here).

Version: mobile facade "1.6.6-debug.21"; gradle versionCode 19→20
with the round's comment block; ADR-0034/0035/0036, release notes
(the time-of-day honesty: the two tests ran 4:17 AM vs 6:04 PM; the
morning re-test ask), CHANGELOG, README, PROGRESS updated. The
sandbox was wiped again before this round: repo re-cloned, Go
1.26.3 reinstalled, the frontend rebuilt, the whole Android lane
(JDK/SDK/NDK/gomobile) rebuilt from scratch.

## M8-ad — the long-lived-connection round (ADR-0037/0038/0039, debug.22, versionCode 21)

Date: 2026-10-20 (Asia/Tehran), night. Input: the tester's
post-debug.21 report — no diagnostics paste this time, three
sentences instead: "the problem is not solved yet (video lags to
begin)"; "I found another weird bug (more in linux, less in
windows), that after a while playing and seeing a video like minute
5 of a 10min video, it goes loading, and as soon as you refresh it
fixes"; and "I think you need to read the backend script again,
maybe it gives you ideas" — pointing at the legacy Python's
backend.py (the app's own proven prototype: 300-fragment hellos at
1 ms, DoH routed through its own fragmented lane, DNS cached
forever, one plain dial per flow, TCP_NODELAY, 21 s timeouts).

What the code read said (tunnel.go's watchStall against the
report's words): the minute-5 wedge is three death shapes of a
PROVEN ("rich") connection the budgets never covered — (a) a
PENDING death: the player's next range request rides the dead
pooled conn, both directions go quiet, and the kill waits the full
90 s StallRichTimeout (the M8-ac round even demoted on pending rich
kills but left the budget untouched); (b) a BLACKHOLED conn: no
tuned keepalive ever notices (Go's default: 9 probes at 15 s ≈ 2 h
to surface), the relay sits blocked in Read; (c) a THROTTLED conn:
bytes keep arriving under the floor, the down stamp keeps
refreshing, and the both-silent clock never even STARTS — the
debug.20 report's 5×-less-down was this shape's fingerprint. In all
three the player buffers, drains, spins — and the user's refresh
closes everything and recovers in ~2 s through the race +
substitution machinery that already works. Plus the two backend.py
ideas the rewrite never adopted: legacy fragmentation was
unreachable from auto (presetFor(auto)=balanced,
HealSweepOrder tried legacy LAST), and the DNS cache got
re-resolved on every stall kill (the churn spiral the debug.20
report measured: dns=579-846 ms late-session, 59 DoH dials).

The round: (1) ADR-0037 — StallRichPendingTimeout (10 s) closes a
rich flow with an outstanding request (demote lands, noise quiet,
tier=rich cause=pending); netx gains KeepAliveIdle/Interval/Count
(net.KeepAliveConfig{15 s, 5 s, 3} applied post-dial on every
origin dial — the engine's dialer AND the DoH lane's own); the
trickle kill (TrickleWindow 12 s / TrickleMinBytes 128 KB /
TrickleFloorBytes 1 MB) closes a rich flow whose rolling window
keeps landing above zero but under the floor — quietly, no demote
(policing follows the CONNECTION; the dead-origin probe already
showed the IPs answer). Considered and rejected: proactive
idle-gvideo rotation (~30 s) — handshake churn under a censor that
punishes scan-like shapes, with keepalive+pending already bounding
the waste. (2) ADR-0038 — the lean stall kill's invalidateDNS
REMOVED (the kill is an origin-conn death; the per-IP demote
carries the evidence; the M8-o dial-failure path keeps the answer
set's refresh). (3) ADR-0039 — presetFor(auto)=fragment.Legacy and
HealSweepOrder = legacy→strong→balanced→light. (4) Diagnostics —
stallRichPending/stallTrickle counters; the stall line reads
"rich=N (pending=Q trickle=T)".

Tests: proxy/liveness_test.go (3: the rich pending fast kill — the
line, the demote, the app-side close, the kept answer set; the
trickle kill — the line, the mark KEPT, the app-side close; the
stall kill keeps the answer set) + netx/keepalive_linux_test.go
(TCP_KEEPIDLE/KEEPINTVL/KEEPCNT read back off the socket) +
engine/m8ad_test.go (3: auto starts at legacy, the sweep leads with
legacy, the report renders the rich cause split with no-leak
guards). Three M8-v/x-era pins updated to the ADR-0038 contract
(stall kills never invalidate): TestStallKillClosesSilentRelayFlow,
TestStallLeanKillStaysLoud, TestStallRichEscalationDisabledByConfig
— their kill/tier/error assertions stand, the invalidate assertions
inverted; the M8-r sweep test's first-method pin updated to legacy.
Both-ways, executed: the pending budget neutered →
TestRichPendingKillAtTheFastBudget FAILS; the trickle ring neutered
→ TestTrickleKillClosesTheThrottledFlow FAILS; invalidateDNS
restored at the kill site → TestStallKillKeepsTheAnswerSet FAILS;
presetFor reverted to balanced → TestAutoStartsAtLegacy FAILS. Full
suite green (18 packages + mobile). Honest flake note (the
first-byte-read family documented for three rounds, living in the
path this diff never touches): TestStallKillClosesSilentRelayFlow
measured 1/20 solo with this diff vs 4+/20 at HEAD in the same
session; TestStallKillSparesStreamingDownload flaked once in ONE of
two full-suite interleaves (first-flight i/o timeout under parallel
package load), then the package passed solo AND the test passed
10/10 solo — pre-existing sandbox CPU starvation, recorded here.

Version: mobile facade "1.6.6-debug.22"; gradle versionCode 20→21
with the round's comment block; ADR-0037/0038/0039, release notes
(the minute-5 story in plain words, the browser-per-OS ask for the
Linux/Windows asymmetry), CHANGELOG, README, PROGRESS updated.


## M8-ae — the wire-discipline round (debug.23, versionCode 22)

The tester's post-debug.22 report ("it doesn't even open video
after a while. But the feed refreshes. this is probably one of the
worst versions") came with the two reference repositories
(gfw_resist_HTTPS_proxy, WebImmortalGuards) and the instruction to
learn how those systems work. They work by emitting almost
nothing: one fragmented hello per connection, DoH through their own
fragmented lane, DNS cached forever, zero background handshakes,
and recovery delegated to the app's hostname rotation. Auditing
our engine against that bar found the compounding regression: the
raced fan-out (primary + 2 extras + 1 substitute, each a legacy
299-cut hello), the 48-job probe pass at start/every 5 min/churn-
kicked, and per-candidate DNS invalidation — ~10-40x the reference
volume, on exactly the SNI class a per-source flag accumulates on.
"Feed refreshes but video never opens" is that flag's signature.

The round (ADR-0040/0041/0042): the good-front single hello + the
once-per-round demotion; the probe trickle (10 per pass, 15-minute
cadence, no start pass, no churn kick) + the emission breaker (3
terminals in 30 s -> ~45 s of zero googlevideo wire at both doors,
non-gvideo untouched, self-extinguishing); the gvideoHellos /
gvideoCooldowns meters and the "gvideo discipline" report line.

Tests: proxy/discipline_test.go (4: the good-front single hello —
the echo origin accepts exactly one flow connection, zero corpse
dials, the meter reads 2 probes + 1 flow; the once-per-round
invalidation — 3 dead candidates demote exactly 2x, never 4x; the
emission breaker — two terminals arm it, the cooldown's
googlevideo CONNECT gets its 502 while the corpses' accept counts
stand perfectly still, the same window's non-gvideo flow echoes
byte-exact, the lapsed cooldown restores the dial; the probe clamp
— 16 reset origins and a 999 budget probe exactly 10, every probe
hello in the meter) + engine/m8ae_test.go (2: the report renders
the discipline line with no-leak guards; the knob contract pinned
— 10 / 15 min / 3-30 s-45 s, the churn-kick knob gone from the
surface). Both-ways, executed: the good-front condition stashed ->
TestGoodFrontSendsOneHello FAILS on the echo count; the
once-per-round hunk stashed -> TestRacedDeathInvalidatesOncePerRound
FAILS at 4 invalidations; the door stashed -> the cooldown's 502
becomes a 200 and the wire speaks; the clamp stashed -> 16 probes;
the report line stashed -> the render test FAILS. Full suite green
(19 packages + mobile). Flake record: one
TestStallRichEscalationDisabledByConfig failure in ONE full-suite
interleave, passing 3/3 solo immediately after; and one
TestPendingStallDemotesTheMark failure ("the origin's first flight
must be delivered: unexpected EOF") in ONE of the final-gate
interleaves, passing 5/5 solo and in both following whole-package
runs — the same first-byte-read family documented for four rounds
(it lives in the read path this diff never touches), recorded
honestly.

Version: mobile facade "1.6.6-debug.23"; gradle versionCode 21->22
with the round's comment block; ADR-0040/0041/0042, release notes
(the wire-discipline story in plain words, the hellos-vs-flows and
cooldowns decision numbers, the fresh-start ask), CHANGELOG,
README, PROGRESS updated.
## M8-af — the prove-before-change round (debug.24, versionCode 23)

Date: 2026-10-21 (Asia/Tehran). Input: the external engineer's
debug.24 spec (younblock-debug24-spec.md, saved at the repo root by
Ali's upload) executing their review of the ENGINEERING-
CONSULTATION document — the debug.23 field report analysis with its
tested/not-tested matrix and ranked assumptions. The spec's own
ruling: the cause is still UNPROVEN (the 2.8 KB corpses are
probably TLS-handshake-only replies, but sizes suggest, they don't
prove), so the round measures before changing — with exactly two
behavior changes, both decided on the consultation's own measured
evidence (A4: the breaker sensor defect, provable from the log
alone; A2/A3: the substitution pool's self-consuming false
positives, measured exactly in the debug.23 counters).

Built, in the spec's priority order (spec §5):

- W1 (ADR-0043): internal/netx/tcpinfo{,_linux,_other}.go — the
  kernel TCP_INFO snapshot (GetsockoptTCPInfo via SyscallConn
  .Control, the M8-ad keepalive pattern) plus
  ClassifyTCPVerdict (the spec's decision rule, pure and
  unit-pinned); internal/proxy/tcpmon.go — the per-flow holders;
  tunnel.go wiring — snapshot A at the request write (the pump's
  hook on the serial path, a write-wrapper on the relay path —
  taken the moment the bytes go out, BEFORE their ACK can land),
  snapshot B at every flow end before the close takes the fd
  (relay unwind, all four stall-kill branches, the terminal), the
  verdict classification into four new stats counters, and the
  `tcp verdicts:` report line. A read failure logs
  `tcpinfo=unavailable reason=…` once per device.
- W2 (ADR-0043): the 5-byte TLS record header parser over the
  RELAYED bytes (first 12 s2c + 8 c2s records as type:length@ms),
  fed by both relay read sides AND the direct deliveries (the
  raced winner's first bytes, the serial first byte) so the
  timeline is whole; `first-app-byte` per the spec's simple rule
  (the first server record after the client's request record),
  with `none`/`no-request` as honest explicit absences.
- W3 (ADR-0043): `win=orig|subst|race` on every googlevideo done
  line, set at every winner site.
- W9 (ADR-0043): fragment.PiecesTotal (one count per written
  piece, process-wide) rendered as `fragPackets=` on the
  discipline line; the network test prints per-method deltas.
  The Android facade gains SetDeviceInfo/SetNetworkInfo
  (permission-free: Build fields; TelephonyManager getters; the
  wifi/cellular GUESS is a shell-owned heuristic — data state
  connected → cellular, else a wlan interface with addresses →
  wifi — recorded as a heuristic in the ADR, because
  ConnectivityManager would need ACCESS_NETWORK_STATE which the
  manifest deliberately does not hold), rendered as `device:` and
  `net:` report lines; no new permissions.
- W4 (ADR-0044): settings v6 (advanced.experimentSubstitution /
  experimentBreaker, both shipping on; the pre-v6 migration flips
  both on — the v5 pattern, absent fields are the shipped default
  never a choice); proxy.Server.SetExperimentFlags applies them
  live (goodSubstitutes and noteGvideoTerminal gate on them;
  disabling the breaker clears an active cooldown); the engine
  wires the knobs (substitution off = the knob's 0 shape, breaker
  off = -1, the debug.22 shape) and applies settings changes
  live; the report's `experiment:` line reads the RUNNING
  server's live flags; the diagnostics screen gains the
  Experiments section (method selector, two toggles, all wired
  through the full-document settings save — a partial save would
  zero fields server-side).
- W7 (ADR-0044): internal/probe/media.go — the media-liveness
  probe (dial through the engine's netx door, fragmented TLS
  handshake with a real SNI and ALPN http/1.1, the deliberately
  bad GET /videoplayback, ANY status line = answered); classes
  answered / silent-after-handshake / hello-silent / reset /
  error. internal/engine/networktest.go — the one-button lane:
  up to 6 current addresses (the freshest learned code's
  rr1..rr5, redirector fallback) × all four methods in the same
  minute, alt-SNI variants (www.google.com, none) under the
  current method, the i.ytimg.com control per method, per-method
  fragment-packet deltas, answered-under-real-SNI →
  MarkMediaVerifiedIP; async with RUNNING/NOT_ON gating, a
  status JSON, an SSE event, and the webui /api/networktest route
  (GET+POST, one registration — two HandleFunc calls on one
  pattern panic the mux). The lane runs outside the breaker's
  door rules (a measurement must not change what it measures)
  but counts every googlevideo-SNI hello in the discipline meter.
  Two spec readings recorded in the ADR: the report build itself
  never probes (a synchronous test would hang the report button
  for minutes; the honest not-run line replaces it), and the
  test's lines survive Stop (the paste is the measurement's whole
  value).
- W8 (ADR-0044): config.ExperimentNodes() embeds
  internal/config/experiment_nodes.txt (empty with a comment
  header; comments/blanks skipped, capped at 4 hostnames); the
  network test probes Ali's hostnames with their own real SNI
  under every method (2 addresses each), foreign-cluster rows
  rendered in the table, foreign answered → media-verified marks.
- W5 (ADR-0045): Server.feedGvideoBreaker (the ring feed
  extracted from failFlow) called from every PENDING stall-kill
  branch of watchStall — lean and rich, never idle, never
  trickle; gvideoTerminals now counts every ring feed (the
  breaker's own view of googlevideo deaths).
- W6 (ADR-0045): markGoodIP's gvideo flag becomes media-verified
  evidence — every first-byte/probe verdict site now passes
  false (preference only); MarkMediaVerifiedIP is the only writer
  of the flag; SeedGoodIPs seeds non-gvideo marks (the old files
  were first-byte-certified — exactly the false-positive class
  W6 removes — and the loader cannot tell them from this build's
  writes; the flip condition, a versioned per-line format, is
  recorded in the ADR).

Tests, all both-ways (executed proofs): the W5 breaker feed
stashed → TestStallPendingFeedsBreaker FAILS at cooldowns=0; the
W6 raced-winner flag stashed → TestFlowSuccessFeedsTheSubstitutionPool
FAILS on the flow-fed pool; the W1 doneSuffix stashed →
TestGvideoFlowEvidenceOnDoneAndStallLines FAILS on the missing
fields; the W9 piece counter stashed → TestFragmentPiecesCounted
FAILS at zero delta; the W2 desync guard stashed →
TestRecordTrackerParsesAcrossChunks FAILS on garbage tails. Full
suite green (19 packages + mobile); race detector green on
internal/proxy + internal/probe + internal/netx.

The round's one detour, recorded honestly: the stalled-flow test
fixtures (stallOriginServer, streamOriginServer,
richThenSilentOrigin) echoed exactly ONE read — but a fragmented
ClientHello arrives in PIECES, so "echo the first flight" silently
depended on the scheduler landing every piece in a single read: a
~10% flake at full speed (documented in the M8-ae and earlier
flake records — the same first-byte-read family, it turns out,
was THIS) and near-deterministic failure under -race once the
evidence wrappers shifted the write timing. The fixtures now echo
the whole first flight byte-budgeted (the real corpse's handshake
completes; the fixture's now does too) — deterministic at any
speed, and the pending-shape tests keep their semantics (the
request after the flight budget is swallowed silently, exactly
the dead-with-work shape they drive). A second detour: one
git-checkout during the both-ways proofs wiped the working
tunnel.go — re-applied from the patch scripts and re-verified
(full proxy suite green) before anything else ran; the both-ways
proofs after it use file-copy stashes only.

Version: mobile facade "1.6.6-debug.24"; gradle versionCode
22→23 with the round's comment block; ADR-0043/0044/0045,
release notes (the prove-before-change story in plain words, the
new decision numbers, the tester protocol), CHANGELOG, README,
PROGRESS updated.

## M8-ag — the breaker stops starving the player (debug.25, versionCode 24)

The tester's debug.24 report, pasted the same night: "Didn't work,
it even failed to play a video. but feed refreshes." The evidence
chain, read in order: (1) the play burst raced googlevideo terminals
from a dead cache cluster (the whole sn-qxau5 set: flows=3 failures=3
each, worstFirstByte=0ms — the terminal never spoke); (2) the W5
sensor fix — the round's own PROVEN defect repair — finally saw those
pending stall kills, three landed inside the 30s window, and the
breaker armed, exactly as designed; (3) ONE 45s+jitter cooldown then
blanket-covered the session (cooldowns=1): 58 "socks5 gvideo
cooldown → door dial failed" pairs, zero wire, the app retrying every
2-7s against the door until +46s when it stopped asking — the
player's whole retry budget spent INSIDE one cooldown; (4) the same
report proved the wire healthy twice: the network test answered 5/5
addresses under the real SNI (media-verifying the substitution pool
to pool=6, gvideo 5) and the one media flow that squeaked through
before the arm delivered 406KB down (win=orig, first-app-byte=77ms,
then 90s idle); (5) substitution: attempts=0 — the door-block
short-circuits BEFORE the raced round where the substitute lane
engages, so the breaker starved the very pool built to route around
dead clusters. The breaker had inverted its own contract (ADR-0041
was written to prevent "video never opens again while the feed keeps
working" — the debug.24 field session WAS that, by the breaker's
own hand).

The round (ADR-0046, four changes, all inside the breaker's scope —
googlevideo hosts at the doors plus the cooldown budget; every other
SNI class and the desktop shapes untouched):
- The pool escape: gvideoDoorBlocked passes a flow when
  goodSubstitutes(nil, 1) finds a fresh, unboxed, media-verified
  origin — answered-under-the-real-SNI evidence outranks the arm's
  guess; the flow rides the existing substitute lane the pass now
  actually reaches. Respects the W4 switches and knob (breaker-on +
  substitution-off keeps the pure M8-ae shape).
- The scout: claimGvideoScout hands each cooldown's FIRST googlevideo
  flow to the wire (reset on every arm) — the app's own request, zero
  synthetic emission, so an armed breaker is never blind; its death
  feeds the ring like any terminal (in-cooldown feeds never re-arm),
  its first byte delivers video.
- The lift: any googlevideo first byte — every path funnels through
  logTunnelFirstByte (serial, raced winner, salvaged, substituted,
  plain relay) — calls liftGvideoBreaker: an active cooldown ends on
  the spot, the ring stays, stalls resume → fresh terminals re-arm.
  Self-correcting in both directions.
- The budget: GvideoBreakerCooldown 45s → 15s (the report measured
  the player dying inside one 45s cooldown; 15s still spans the app's
  2-7s retry cadence for hostname rotation; jitter shape unchanged).
  SetExperimentFlags' breaker-off clear also resets the scout.
- Report: the discipline line reads hellos=… cooldowns=… lifts=…
  scouts=… fragPackets=… (two new counters, gvideoScouts/gvideoLifts)
  — lifts>0 after a play attempt means the wire spoke through an
  armed breaker; scouts>0 with playback recovered means the scout
  carried it; attempts>0 beside a fed pool means the substitute lane
  finally engaged through the pass.

Tests: the two breaker door tests rewired to the new contract —
TestEmissionBreakerQuietsTheWire (corpse fixture: the FIRST post-arm
flow is the scout — dials once, terminal, feeds the ring a third
terminal while cooldowns stays 1; the SECOND gets the door's 502 with
the corpse counts standing still; a mid-cooldown MarkMediaVerifiedIP
lets the NEXT flow through the SAME active cooldown — the pool escape
tail; the non-gvideo feed sails; the lapse serves again; the test's
cooldown raised to 2s so no pass can be mistaken for a lapse) and
TestStallPendingFeedsBreaker (echo fixture: three pending kills arm,
the scout dials the live echo, its first byte LIFTS — the log line,
lifts=1 — and the very next flow passes on the lift, not the lapse).
Both-ways proofs EXECUTED (stash → FAIL → restore): the scout claim
stashed → the corpse test FAILS on the scout's CONNECT getting the
door's 502; the pool check stashed → FAILS on the pool-primed flow's
502; the lift hook stashed → the echo test FAILS on the missing lift
line; the 15s default's both-ways was the knob pin failing on the
unchanged suite (TestWireDisciplineKnobs wanted 45s and said so).
Full suite green (19 packages + mobile); -race green on the house
gate (proxy + probe + netx); gofmt -l identical to HEAD's list (the
edit tool space-ified seven tab-indented files mid-round — caught by
the gofmt-list diff against HEAD and restored with gofmt -w).

One detour, recorded honestly: the engine -race run flagged
TestNetworkTestGatingAndStatus ("the network test never finished",
5.07s) on this tree while the first HEAD runs passed — investigated
before touching anything. The test rides a REAL-network DoH failure
(~3.01s, the stub resolver has no codes and the redirector lookup
fails) against its 5s poll; passing runs measure identically on both
trees (3.00-3.07s) and HEAD flakes at the same rate with more samples
(27 pass / 3 fail over 30 HEAD runs vs 10/12 here) — a pre-existing
marginal test, which is why the house -race gate never included
engine. No code changed for it; the ADR records the numbers.

Version: mobile facade "1.6.6-debug.25"; gradle versionCode
23→24 with the round's comment block; ADR-0046, release notes
(the starved-behind-its-own-door story in plain words, the new
discipline-line numbers), CHANGELOG, README, PROGRESS updated.
Build artifact: commit 0405f1f; release v1.6.6-android-debug.25
(id 418991235, the repo's Latest), APK v1.6.6-debug25.apk
56,068,289 B, SHA-256 b29a3aab77b656a5651bc0cf82222025db65fa5a3c
62d1bf65c4f227bfbdb251, aapt badging versionCode 24 / versionName
1.6.6 / minSdk 26 / targetSdk 36, apksigner pinned cert
c76bb7c6…8551d, the M8-ag lift/scout/version strings verified in
libgojni.so, and the release asset re-downloaded byte-identical
(cmp + SHA-256).

## M8-ah — the dead-cluster memory round (debug.26, versionCode 25)

The tester's debug.25 report: "Feed refreshes, every video takes
initially 1-2 minutes to open, after opening it's smooth and you can
go like 5 min next without hesitation." The report decomposed the
1-2 minutes: the whole sn-qxau5 cluster blackholed (21 flows, all
worstFirstByte=0ms — TCP accepts, TLS never answers; the dead-origin
probe confirmed IP-range blackholing: silent under every SNI), each
attempt burning its 3s budget, interleaved with 7 breaker cooldowns
(7 × ~15s ≈ the whole open latency; one arm blocked a live
r5-aj4g55 dial for ~10s); the M8-ag escapes working as designed
(scouts=6 — the scout flows carried the 1.8 MB and 559 KB videos;
lifts=2) but starved: pool=13 (gvideo 0), attempts=0 — W6 left the
operator-pressed network test as the pool's only food.

The round (ADR-0047): (1) the dead-cluster memory — cluster = the
sn-XXXX token (gvideoClusterKey); 3 distinct hosts silent-terminal
(reason=timeout, the failFlow hook) inside 10min mark the cluster
dead for 10min; the door fast-fails its hosts (instant 502, zero
wire) unless the pool serves or the per-cluster probe is due (1 per
2min, probedAt seeded at mark time so fresh marks are never
immediately probed); an answered OWN dial (serial first byte, raced
own-set winner, salvage — never a substitute) rehabilitates
(noteGvideoClusterAlive at the three winner sites). (2) The
payload-fed pool — logTunnelDone certifies the origin
(ev.originIP(), the new tcpmon accessor) when the flow delivered
PayloadVerifiedMinimum (64KB); MarkMediaVerifiedIP is now fed by the
network test AND payload. (3) The multi-cluster arm gate — the ring's
stamps carry clusters (gvideoTerminalStamp); the arm needs ≥2
distinct clusters; dead-cluster terminals count but never feed
(feedGvideoBreaker's exclusion). (4) The pool file's versioned
format — "<ip> media" lines persist and seed the POOL
(SeedMediaVerifiedIPs via the refactored seedGoodIPs); bare lines
stay preferences; loadGoodIPPool returns (legacy, media);
saveGoodPoolIfChanged writes the marker.

Report: the "gvideo clusters: dead=… fastFails=… probes=…
rehabilitated=… payloadVerified=…" line (five new counters in
stats). Config: GvideoClusterDeadHosts/Window/TTL/ReprobeEvery +
PayloadVerifiedMinimum knobs (defaults 3/10m/10m/2m/64KB).

Tests: TestClusterMemoryFastFailsTheDeadCluster (the contract pin:
mark → instant 502 <500ms with corpse counts still → pool override →
own-dial rehab → 8KB echo payload-feed, pool gvideo ≥2);
TestEmissionBreakerQuietsTheWire and TestStallPendingFeedsBreaker
rewired to arm across two clusters (their single-cluster shapes
failed the moment the gate landed — the natural both-ways); the M8-ab
round-trip extended with a session-2 leg ("192.0.2.55 media" seeds
pool=1 gvideo=1). Both-ways proofs EXECUTED: the door's cluster
branch stashed → the fast-fail flow got a 200 dial; the payload feed
stashed → the media-verified line missing; noteGvideoDeadHost
stashed → the cluster never marked; the media seeding's flag
stashed → session 2 rendered gvideo 0. The ring-feed exclusion has
no isolated pin (the arm gate subsumes it observably) — recorded in
the ADR as the round's honest gap. Full suite green; -race green on
the house gate (proxy 81s + probe + netx); gofmt -l identical to
HEAD's list (the edit-tool space-ification caught twice more —
tcpmon.go joined the list — and was restored both times).
TestNetworkTestGatingAndStatus flaked again at its documented ~10%
HEAD rate; untouched.

Version: mobile facade "1.6.6-debug.26"; gradle versionCode
24→25 with the round's comment block; ADR-0047, release notes,
CHANGELOG, README, PROGRESS updated.
Build artifact: commit 6eddbab; release v1.6.6-android-debug.26
(id 419033462, the repo's Latest), APK v1.6.6-debug26.apk
56,141,329 B, SHA-256 e4148c8bedbe882ecaacdc97b054b14c3399df418
0d509456fc42407330ad3ff, aapt badging versionCode 25 / versionName
1.6.6 / minSdk 26 / targetSdk 36, apksigner pinned cert
c76bb7c6…8551d, the M8-ah cluster/payload/version strings verified
in libgojni.so, and the release asset re-downloaded byte-identical
(cmp + SHA-256).

## M8-ai — the pool keeps its food round (debug.27, versionCode 26)

The tester's debug.26 report: "I think we're closer than ever, this is
probably our best so far… first video still buffers (less than
before), later videos mostly open nearly quickly, some take a little
longer." The M8-ah machinery measured WORKING (dead=1, payloadVerified=6,
cooldowns=1 lifts=1 scouts=1, attempts=14 wins=12) — what remained was
the app's dead-cluster LADDER: fastFails=150, doorFailures=150, the
player grinding its hostname rotation against instant resets for ~40s
per unlucky open while the verdict lane (the door's pool pass) starved
mid-storm — pool gvideo 2 at the end, the two payload-fed marks gone
through the storm's middle with goodDrops=16 and penalized=4 strikes=4
beside them.

The drain had three mouths (ADR-0048): (a) a substitute candidate's
natural death demoted its pool origin (markDeadIPTTL's dropGoodIP —
the M8-ab rule written when pool entries were handshake-certified
guesses); (b) a pending stall kill on a substituted flow demoted the
origin it rode; and (c) — the biggest, found by the round's own test
before the field ever saw it — markGoodIP(ip, false) on every
first-byte WIN REPLACED the winner's good entry, un-flagging
media-certified origins one flow at a time: six certifications in,
the session ended holding two. Two hypotheses retired with readings:
the 12 substituted wins all delivered ~5KB answers (the debug.23
shape on true positives — the cross-cluster substitute is the
fast-verdict lane, not a media carrier; every payloadVerified rode
win=orig), and the udp443 "fallback elapsed" samples (8.3s/23.9s) are
heuristic pairings, not QUIC timeouts (gVisor's ICMP port-unreachable
is already on the wire, M8-j's pin) — no QUIC work.

The round (ADR-0048), one property — a media certification (the
≥64KB payload feed or an answered real-SNI probe verdict, the only
two writers) is revocable ONLY by its 15-minute TTL: (1) the
no-downgrade rule — markGoodIP keeps the flag when a handshake mark
refreshes a certified origin; (2) the demote guard — dropGoodIP
skips gvideo-flagged marks (the strike-escalated penalty box stays
the rotation guard both goodSubstitutes and liveIPs read);
(3) the feed clears the box — MarkMediaVerifiedIP also clearDeadIPs
(a fresh delivery outranks the stale death that boxed it);
(4) the verdict lane made visible — the door's dead-cluster pool
pass counts and logs (GvideoClusterVerdict → verdicts= on the
clusters line, "gvideo cluster verdict" INFO line), every substituted
flow classifies by size at flow end (SubstBig/SubstSmall → big=/
small= on the substitution line, the bar PayloadVerifiedMinimum), and
both doors log their refusal REASON ("cluster-dead" vs
"breaker-cooldown") instead of the M8-ae-era message that blamed 150
refusals on a breaker that armed once.

Tests: m8ai_test.go, five shapes (TestSubstituteLossKeepsTheMediaMark,
TestMediaFeedClearsThePenaltyBox, TestSubstitutedFlowsClassifyBySize,
TestDoorVerdictLaneCountsAndReasons — the fourth carries the
no-downgrade pin inside its flow-B leg). Six both-ways proofs
EXECUTED (stash → FAIL → restore): the dropGoodIP guard, the
clearDeadIP call, the size classification, the no-downgrade guard,
the verdict counter, and the door reason string — every one
FAIL-as-expected / PASS-restored. discipline_test.go's door pin
re-pointed at the honest reason line; m8af_test.go's clusters-line
pin extended with verdicts=0. Full 19-package suite green; -race
green on the house gate (proxy 82s + probe + netx); gofmt -l
identical to HEAD's list (tunnel.go's HEAD space indentation
preserved through a python-inserted hunk; the new m8ai_test.go
gofmt-clean).

What the next report reads: substitution "big=… small=…" (is the
lane ever a media carrier, or purely the fast-verdict lane), clusters
"verdicts=" against fastFails= (did the lane hold through the
storm), and the timeline's "first-byte substituted" lines beside the
app's next move (does a verdict retire the URL set faster than a
reset did — the round's open field question, answered by
verdicts=vs fastFails= timing).

Version: mobile facade "1.6.6-debug.27"; gradle versionCode 25→26
with the round's comment block; ADR-0048, release notes, CHANGELOG,
README, PROGRESS updated.
Build artifact: commit bfc18ed; release v1.6.6-android-debug.27
(id 419074187, the repo's Latest), APK v1.6.6-debug27.apk
56,144,837 B, SHA-256 3c402d83ddbcae8ab352f06a5d8b5f6081012e1b5a
ddd6e0804627c97a0aaa9a, aapt badging versionCode 26 / versionName
1.6.6 / minSdk 26 / targetSdk 36, apksigner pinned cert
c76bb7c6…8551d, the M8-ai verdict-lane/door-reason/version strings
verified in libgojni.so, and the release asset re-downloaded
byte-identical (cmp + SHA-256).

## M8-aj — the lane retires round (debug.28, versionCode 27)

The tester's debug.27 report: "This one did not even open a single
video, only feed refreshed." The cleanest experiment result the
project has had: substitution attempts=40 wins=37 big=0 small=37 —
the fed pool kept the verdict lane engaged on every dead-cluster
attempt, every answer was the ~5KB verdict shape, and the player
NEVER left the dead URL set (rr8-btql alone: 20 flows, one every ~4s;
no live cluster ever dialed; 1.2MB total session data vs debug.26's
23.1MB; payloadVerified=0). And dead=0: the cluster was never marked
dead, because substituted flows never terminal — the lane starved
the very memory that ends the storm. In debug.26 the same regime
recovered in ~40s precisely because the (accidentally drained) lane
let the flows terminal, the cluster got marked, and the door's
fast-fail resets let the player's ladder escalate to a fresh
player-API fetch. The RST is the ladder's escalation signal; the
HTTP verdict is its sedative. Two smaller readings: some substituted
flows hung after TLS (9s, first-app-byte=none — the GFE holds
cross-cluster requests it will not route), and the lane fed the
breaker noise from both sides (armed on the hung flows' pending
kills, lifted by substituted first bytes).

The round (ADR-0049): the four-round experiment concludes — the
cross-origin googlevideo substitute NEVER carried media (debug.19/
23/26/27) — and retires at the SETTINGS layer. (1) Schema v7:
experimentSubstitution defaults OFF; the v7 migration flips it off on
every existing file (a v6 file's true is the shipped default recorded
verbatim, never a choice; a v7 file's explicit true is the owner's
word); the report's experiment line reads substitution=off.
(2) The off position is REAL now — the M8-af engine knob resolved the
switch's off to 0, which the proxy's Config defaulting reads as "use
the default": the switch's off has been a NO-OP at engine start since
debug.24 (the live toggle worked; a settings-off start never did —
the breaker knob got -1 right, substitution's got 0 wrong).
goodSubstKnob now returns -1, the proxy knob's actual disabled shape.
(3) Everything from ADR-0048 stays (the pool fixes, the counters, the
honest door reasons); the door's dead-cluster shape returns to
debug.26's probe → fast-fail.

The knob bug was found by the round's own re-pinned report test:
m8af_test's experiment line expected substitution=off from a default
engine and the report rendered ON — the natural both-ways, observed
twice before the fix was written. Both-ways proofs EXECUTED besides:
the v7 migration stashed → the v6-file leg FAILS; the default flip
stashed → the default shape FAILS; the knob's -1 stashed back to 0 →
the report test FAILS. Full 19-package suite green; -race green on
the house gate (proxy + probe + netx); gofmt -l identical to HEAD's
list (engine.go's HEAD space indentation preserved through a
python-inserted hunk). The environment was rebuilt from scratch this
round (the sandbox was wiped between rounds: Go 1.27.0, JDK 21,
Android SDK build-tools;36.1.0 + platforms 26/36 + NDK 29.0.14206865,
gomobile at the go.mod pseudo-version — the build reproduced from the
repo alone).

What the next report reads: experiment "substitution=off" (the
retirement landed); substitution attempts=0 beside gvideo clusters
"fastFails=" re-owning the recovery; dead=1 with fastFails>0 again
(the memory fed); and the tester's experience back at debug.26's
shape (first video slow-ish on a dead-set, later videos fast) with
the lane's absence the only difference.

Version: mobile facade "1.6.6-debug.28"; gradle versionCode 26→27
with the round's comment block; ADR-0049, release notes, CHANGELOG,
README, PROGRESS updated.
Build artifact: commit 874b768; release v1.6.6-android-debug.28
(id 419378998, the repo's Latest), APK v1.6.6-debug28.apk
57,097,057 B, SHA-256 4fda35bc9559b5144c4b7326cf857863ca9410913
eb74c7a539d1c15135e2116, aapt badging versionCode 27 / versionName
1.6.6 / minSdk 26 / targetSdk 36, apksigner pinned cert
c76bb7c6…8551d, the version/settings/verdict-lane strings verified
in libgojni.so, and the release asset re-downloaded byte-identical
(cmp + SHA-256).

## M8-ak — the method-documentation round (debug.29, versionCode 28)

The tester's debug.28 report: "Congratulations! This is by
faaaaaaaaaaaaar, our best version yet. It worked. 90% of the initial
delay of each video is gone, and it's that much good that it can be
the first official release. But since we came until here, still worth
investigating the remaining 10-5% delay in mobile, which almost
doesn't happen in pc and linux. Document whatever you did to not miss
this accomplishment/method."

The numbers behind the verdict: dead=1 fastFails=90 (the cluster
memory + instant resets owning the recovery), cooldowns=1 lifts=1
scouts=1 (the breaker armed once on redirector pending kills and
lifted in 730ms — the M8-ag escapes working), substitution attempts=0
(the tombstone), auto-timeline 1000ms pairs=5, and a live cluster
(r3---sn-u14g55-5m) delivering 2.1MB mid-session.

The round, per the tester's two asks:
(1) THE METHOD DOCUMENTED — docs/android-port/11-the-video-opening
war.md (indexed in the pack README): the scoreboard (debug.18's 1-2
minute opens to debug.28's ~1 second), the loop method (the report as
the instrument, diagnosis from the report's own evidence, executed
both-ways proofs, pre-registered retreats, the paper trail), the
round-by-round arc (M8-ae through M8-ak), the six semantic
discoveries (the censor's observed classes, the player's ladder
semantics, the cluster as the unit, evidence-class ranks, QUIC as a
non-cost, the app as part of the system), the residual decomposition
(five candidates, each with its measurement path), and the lessons.
(2) THE RESIDUAL INSTRUMENT (ADR-0050) — the auto-timeline re-scopes
to googlevideo: a play is a googlevideo flow after googlevideo quiet
(feed refreshes never touch googlevideo), first-data is a googlevideo
first byte (with the lane retired, that is a live video flow
answering; the old any-down-burst close let the debug.28 report's
1.4MB i.ytimg feed bursts close pairs that carried no video). Two new
counters (GvideoFlows at logTunnelUp, GvideoFirstByte beside the
breaker lift), the burst-bytes knob retired, the report line says
"gvideo-gated" (R21 caught the domain-word leak on the first
attempt — "googlevideo-gated" tripped the release-shape leak guards;
the report's vocabulary is "gvideo").

The residual decomposition (doc 11 §5, from the debug.28 report):
the app's dead-host ladder residue (fastFails=90, the app's own
schedule), the per-flow setup cost (~500-600ms: DoH+ dial + first
byte through the chain), the first-video discovery cost (the cluster
memory is per-session — a persistence candidate for a future round),
stall-kill churn on API hosts (lean=3 rich=3, low), and
first-data→first-frame buffering (the app's policy vs chain
throughput). The next report's gvideo-gated pairs decide the owner.

Tests: proxy m8ak_test.go TestGvideoTimelineSignalsAreVideoScoped
(the video-scoped twins count at the flow's milestones; a routed
feed-class flow moves neither); m8aa_test.go's auto-timeline shape
re-driven on the new counters. Five both-ways proofs EXECUTED (the
two tunnel hooks, the play-stamp gate, the first-data gate, plus the
de-leak guard's natural catch); the play-gate proof needed its second
attempt (the first stash broke the baseline, not the gate — redone
against the condition). Full 19-package suite green; -race green on
the house gate (proxy + probe + netx); gofmt -l identical to HEAD's
list.

Version: mobile facade "1.6.6-debug.29"; gradle versionCode 27→28
with the round's comment block; ADR-0050, doc 11, the pack README
index, release notes, CHANGELOG, README, PROGRESS updated.
Build artifact: commit f84d562; release v1.6.6-android-debug.29
(id 419414818, the repo's Latest), APK v1.6.6-debug29.apk
57,114,657 B, SHA-256 e1a306b4b230f242846ea2d05c12439c4752cf39a
902a07a656405132cfd9904, aapt badging versionCode 28 / versionName
1.6.6 / minSdk 26 / targetSdk 36, apksigner pinned cert
c76bb7c6…8551d, the round's timeline/version strings verified in
libgojni.so, and the release asset re-downloaded byte-identical
(cmp + SHA-256).
