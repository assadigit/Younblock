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
