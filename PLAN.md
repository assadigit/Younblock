# Younblock — Delivery Plan

Phases and acceptance criteria come from `01-go-backend-rewrite-spec.md` (file 01) section 11,
scoped by `00-start-here.md`. This file copies each phase's acceptance criteria as checklists
and tracks their completion. Running details (commands, real outputs, blockers) live in
`PROGRESS.md`. Interpretations and deviations live in `DECISIONS.md`.

Working style: one phase at a time, in order. A phase is done only when every box below is
checked with real command output recorded in `PROGRESS.md`. Small commits are preferred;
each phase ends with a commit, a `phase-N` tag, and a push to `main`.

## Global checks (every phase, all must pass)

- [ ] `go vet ./...`
- [ ] `go test -race ./...`
- [ ] `GOOS=windows GOARCH=amd64 go build ./...`
- [ ] `scripts/check-platform-neutral` (forbidden-import scan for the neutral packages)
- [ ] `GOOS=linux GOARCH=arm64 go build ./internal/engine/... ./internal/config/... ./internal/netx/... ./internal/fragment/... ./internal/doh/... ./internal/proxy/... ./internal/health/... ./internal/stats/... ./internal/logx/...`

## Phase 1 — Headless engine (`cmd/core`)

Scope (file 01 §11): config, netx, fragment, doh, proxy, stats, logx, engine skeleton with
the stub platform. P0 items R1–R11 and P1 items R15, R20, R22 land here.

Acceptance criteria (copied from file 01 §11):

- [x] `go vet ./...`, `go test -race ./...`, `GOOS=windows GOARCH=amd64 go build ./...` pass
- [x] The platform-neutral check and the `GOOS=linux GOARCH=arm64` build from file 01 §3 pass
- [x] Unit tests: planner (sums, no empty piece, size range, `legacy` at most 299 pieces, deterministic with a seeded source)
- [x] DoH resolver against an `httptest` fake server (A parsing, TTL clamp, cache hit, fallback after first server fails, private-IP filtering, stale serving)
- [x] Request parser including a fuzz test (`go test -fuzz`, 60 s, no panic)
- [x] Integration: fake DoH + local TLS echo server (with `AllowPrivateTargets`); an `http.Client` through the proxy completes an HTTPS request
- [x] A recording TCP listener shows the ClientHello arrived in at least 2 separate reads **and** the handshake still succeeded
- [x] Behavior: idle timeout closes a silent tunnel; the 1025th connection gets `503`; `Stop` returns within 3 s with open tunnels; byte counters match transferred bytes
- [x] Soak (script included): 5,000 short tunnels including failing ones; goroutine and handle counts return to baseline and memory stays flat

Verified for real (see PROGRESS.md for the exact commands and outputs): all checks above,
plus a live end-to-end run of `younblock-core --selftest` against the real RethinkDNS and
YouTube from the development sandbox (all four steps passed), launch-to-listening 13 ms
(target 300 ms), and `younblock-core.exe` at 7.08 MB with `-trimpath -ldflags "-s -w"`
(target 12 MB).

## Phase 2 — Health and self-healing

Scope: self-test, heal sweep, watchdog, cooldowns, notices. P1 items R12, R13, R14, and R19.

Acceptance criteria (copied from file 01 §11):

- [x] Tests use fake networks that only accept one preset
- [x] Assert the announced `notice`
- [x] Assert the 35 s cap
- [x] Assert the cooldown

## Phase 3 — Windows adapters

Scope: sysproxy, PAC, autostart. P1 items R16 and the Run-key half of R18.

Acceptance criteria (copied from file 01 §11):

- [x] PAC tests (YouTube hosts match including subdomains and not look-alikes such as `notyoutube.com`; everything else `DIRECT`)
- [x] Registry code compiles for Windows
- [x] Backup and restore logic is unit-tested against an in-memory fake
- [x] Autostart path-rewrite logic tested
- [x] State clearly which Windows-only behaviors could not be run here (see PROGRESS.md Phase 3)

## Phase 4 — Wails app, tray, first run

Scope: bindings over `jsonapi`, events, single-instance lock, tray and notifications,
first-run flow, and the whole UI from file 02. P1 items R17 (single instance), the tray half
of R18, and the frontend part of R21.

Acceptance criteria (file 01 §11 plus file 02 §10):

- [x] `npm run build` passes (84.7 KB JS + 23.6 KB CSS)
- [x] `wails build` passes for windows/amd64 from this machine: younblock.exe 12.13 MB with -trimpath -ldflags "-s -w"
- [x] Single-instance lock (`SingleInstanceLock` in Wails v2; verified against the installed version)
- [x] Tray: three icons differing by shape (+ light-taskbar variants), tooltip states, left-click show/hide, right-click menu (Turn on/off, Open Younblock, Quit), closing hides to tray, first-time close notice
- [x] Notifications only for failure and recovery, episode-based 10-minute rate limit (go-toast; rendering needs a real Windows session)
- [x] Screenshots from file 02 §10 in `docs/screenshots/` (19 shots, reviewed with a vision model against sections 2 and 4)
- [x] Power button is the largest and most contrasting element in every phase; Off/On distinguishable in grayscale (shape-different icons and discs)
- [x] Keyboard only: Space/Enter toggle; focus ring visible; Esc closes the menu (verified in the browser)
- [x] Reduced-motion removes the ripple and spinner motion (CSS media query)
- [x] No network requests except local (network panel shows only 127.0.0.1 requests); fonts and icons local
- [x] No hard-coded strings (all copy in i18n/en.json); logical CSS properties (inset-inline etc.)

## Phase 5 — Remaining P1 items from file 03

Scope: every P1 item not finished in an earlier phase, plus final deliverables.

- [ ] R19 — network change and sleep: poll interface addresses every 5 s while On; on change flush the DNS cache, reset the DoH client, run the watchdog check immediately (unit test with a fake interface list)
- [ ] R21 — privacy: automated scan of report and log samples for hostnames and IPs; strict CSP (`default-src 'self'`, no inline script, no `eval`) in the shipped frontend; ring cleared on Stop
- [ ] Final P1 review: every item R12–R22 listed as done or not done with the check that ran
- [ ] `build.bat` builds `younblock-core.exe` and the Wails app and prints the exact output paths
- [ ] `README.md` in plain English: what the app is, how to install the tools, how to build, how to run, how to uninstall, and Ali's manual Windows check (numbered steps from file 01 §11)
- [ ] Size and speed targets: `younblock-core.exe` ≤ 12 MB and Wails app ≤ 20 MB built with `-trimpath -ldflags "-s -w"` (no UPX); proxy listening within 300 ms of launch

## Out of scope for this run

- File 04 (Android). Obey only the Android-ready rules (file 01 §3) so the later run stays cheap.
- P2 items R23–R27 from file 03 (proposed only after P0/P1 are accepted).
- The `targeted` fragmentation preset (file 01 §5 says do not build it until everything else is accepted).
