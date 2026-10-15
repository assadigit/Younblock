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
