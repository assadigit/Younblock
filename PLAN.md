# Younblock — Delivery Plan

Phases and acceptance criteria come from `01-go-backend-rewrite-spec.md` (file 01) section 11,
scoped by `00-start-here.md`. This file copies each phase's acceptance criteria as checklists
and tracks their completion. Running details (commands, real outputs, blockers) live in
`PROGRESS.md`. Interpretations and deviations live in `DECISIONS.md`.

Working style: one phase at a time, in order. A phase is done only when every box below is
checked with real command output recorded in `PROGRESS.md`. Small commits are preferred;
each phase ends with a commit, a `phase-N` tag, and a push to `main`.

## Global checks (every phase, all must pass)

- [x] `go vet ./...` (green in phases 1-5)
- [x] `go test -race ./...` (green in phases 1-5)
- [x] `GOOS=windows GOARCH=amd64 go build ./...` (green in phases 1-5)
- [x] `scripts/check-platform-neutral` (green in phases 1-5)
- [x] `GOOS=linux GOARCH=arm64 go build ./internal/…` (green in phases 1-5)

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

- [x] R19 — network change and sleep: poll interface addresses every 5 s while On; on change flush the DNS cache, reset the DoH client, run the watchdog check immediately (unit test with a fake interface list) — landed in Phase 2
- [x] R21 — privacy: automated scan of report and log samples for hostnames and IPs; strict CSP (`default-src 'self'`, no inline script, no `eval`) in the shipped frontend; ring cleared on Stop
- [x] Final P1 review: every item R12–R22 (and R1–R11) listed as done with the check that ran (PROGRESS.md Phase 5 tables)
- [x] `build.bat` builds `younblock-core.exe` and the Wails app and prints the exact output paths (+ SHA-256)
- [x] `README.md` in plain English: what the app is, how to install the tools, how to build, how to run, how to uninstall, and Ali's manual Windows check (numbered steps from file 01 §11)
- [x] Size and speed targets: `younblock-core.exe` 6.98 MB (≤ 12 MB) and Wails app 12.13 MB (≤ 20 MB) built with `-trimpath -ldflags "-s -w"` (no UPX); proxy listening within 12 ms of launch (≤ 300 ms)

## Phase 6 — Ali's five post-test requests (v1.1.0, after the real-Windows test)

Scope: five user-requested additions on top of the completed spec work. All default-off or additive; the spec'd behavior stays the default.

- [x] Route-all switch in Settings mapping the existing "All websites" mode (D43)
- [x] `openBrowserOnConnect` setting + neutral internal/browser package with the exact legacy Chrome command line (D40)
- [x] Windows shell launcher (registry + standard paths), session ended on off/error/quit, every action logged + toasted + emitted as browser:* events (R13) (D40)
- [x] Response-time metric: recorded by every successful health check, in state JSON (`latencyMs`), under the timer, in the diagnostic report; resets on Stop (D41)
- [x] Background texture: two soft brand glows + inline SVG noise grain, no network fetches (D42)
- [x] Ali's icon.ico at assets/icon.ico, installed as build/windows/icon.ico + build/appicon.png; tray keeps state icons (D43)
- [x] Gates re-run green: go vet, go test -race (incl. new browser + latency tests), GOOS=windows build, platform-neutral check, arm64 neutral build, npm run build (87 KB JS)
- [x] All 19 screenshots re-taken; DOM + vision checks of the new UI
- [x] README updated (What's new in 1.1.0; notes on the two new switches)

## Out of scope for this run

- File 04 (Android). Obey only the Android-ready rules (file 01 §3) so the later run stays cheap.
- P2 items R23–R27 from file 03 (proposed only after P0/P1 are accepted).
- The `targeted` fragmentation preset (file 01 §5 says do not build it until everything else is accepted).

## Phase 7 (hotfix, v0.2.1 / app 1.1.1) — the "didn't even connect" report

- [x] Investigate the v0.2.0 report: network stack identical to v0.1.0; live self-test green from the dev box (D44)
- [x] Degraded state carries the failing step + code (`fail`), root-cause ordered; subtitle says secure DNS vs YouTube; diagnostic report line
- [x] Degraded starts the stats loop, watchdog and network monitor (R14): timer runs, self-recovery works
- [x] Changed secure-DNS list swaps the resolver live (atomic pointer, `Server.SetResolver`)
- [x] Advanced secure-DNS list editable in the UI (remove; one-tap add AdGuard/Quad9/dns.sb/defaults; new server tried first)
- [x] Three new engine tests; all gates re-run green; screenshots re-taken and vision-checked

## Phase 8 (v0.3.0 / app 1.2.0) — power-button animation fix + professional polish

- [x] Reproduce the "stuck midway" report: emulated `prefers-reduced-motion`; the blanket base.css kill-rule froze the connecting arc mid-spin (D45)
- [x] Reduced-motion policy rewritten: decoration off per component, functional spinner + color crossfades stay
- [x] Turn-on moment fully orchestrated: spring pop, two staggered echo rings, crossfaded gradient lit plate, glow breath; press can no longer stick (release on click/pointercancel/blur)
- [x] Professional polish: state dot beside the label, warmed degraded subtitle, amber ghost "Fix connection", thicker glyph stroke, resting hover feedback, solid amber degraded rim (replaces the dashed ring), footer badge chip
- [x] Copy: footer "Works with every browser"; first-run/Help/About now say the Windows-proxy truth precisely (tested most with Chrome)
- [x] Version 1.2.0 (config, wails.json, mock); all gates re-run green; all screenshots re-shot and vision-checked, animation verified frame-by-frame with and without reduced motion

## Phase 9 (v0.4.0 / app 1.3.0) — every browser, logo palette

- [x] Diagnose Ali's report: only the dedicated Chrome session (--proxy-server) worked; other profiles and Opera did not — the registry PAC never reached them
- [x] sysproxy rewritten to the manual proxy values (ProxyEnable/ProxyServer/ProxyOverride + bypass list); any AutoConfigURL cleared while active (D46)
- [x] WM_SETTINGCHANGE("Internet Settings") broadcast added to the refresh
- [x] Watchdog anti-clobber: EnsureAttached re-asserts the values every tick; PROXY_REAPPLIED notice; engine + stub + tests
- [x] Legacy cleanup: orphaned pre-1.3.0 PAC URL removed at startup; upgrade path tested (backup preserved, user's original restored)
- [x] Proxy: youtube mode fragments only routed hosts; plain-http to non-routed hosts forwarded as a real forward proxy (origin-form, hop-by-hop stripped, Connection: close)
- [x] Proxy tests: forward path, unfragmented relay, all-mode fragmentation; integration test re-pointed at a routed host
- [x] UI restyled to the logo palette (D47): warm charcoal neutrals, red primary, gold accent; tray icons regenerated
- [x] Version 1.3.0; all gates green; live selftest; real-browser E2E through the proxy (YouTube + https site + plain-http site); screenshots re-shot and vision-checked

## Phase 10 (v0.4.1 / app 1.3.1) — "white space" + "Fix connection is on the edge"

- [x] Diagnose from Ali's two screenshots (VLM + pixel forensics): both show the degraded
      state; the v0.4.0 home screen's fixed height budget overflows the 360x560 window
      whenever the status stack grows (degraded docH 574, error 634) — the footer was
      pushed out of the window and the Fix-connection pill landed on the bottom edge;
      the centered flex with overflow left a dead gap in the middle (the "white space")
- [x] Root cause on the window side: Wails' initial SetSize applies 360x560 logical to
      the OUTER window rect, so the title bar + frame shrink the CSS viewport
      (Ali's measured 336x522) at every DPI
- [x] app startup now corrects the outer size so the client area is exactly 360x560
      logical at the window's DPI (clientsize_windows.go + no-op other; D48)
- [x] CSS re-budget (D48): header 52→44; shell owns the height budget (#app max-height,
      flex:none footer with real margins); Connect scrolls internally as a safety net;
      safe-center; timer/quiet block collapses when its state can't show it (grid-rows);
      error reason text capped with its own scroll; error-state button step-down;
      responsive ladder (button 0.88 below 546 css px; whole-screen zoom 0.82 below 470)
- [x] BackgroundColour matched to the real --neutral-950 (37,33,32)
- [x] Version 1.3.1; gates green; all six states measured at 360x560, 336x522 and
      288x448 (docH = viewport, footer visible, fix pill ≥15px above the footer);
      screenshots re-shot and vision-checked; golden-path E2E with zero console/page
      errors

## Phase 11 (v0.5.0 / app 1.4.0) — the 32-bit and Linux Mint versions

- [x] StatusBlock error-state refactor (D49): the reason is the subtitle, the
      duplicate low paragraph is gone, D48 budget preserved (measured: label→
      reason gap 8 px, docH = viewport at 360x560 and 336x522, footer visible)
- [x] Linux platform adapter (D50): gsettings proxy with verbatim backup/
      restore, autostart .desktop, notify-send; fake-gsettings unit tests;
      gsettings literal forms verified against real glib 2.84 + compiled
      upstream schema (int32 ports, comma+space arrays, quoted enums)
- [x] internal/webui: embedded frontend + the JSON API (same payloads as the
      Wails bindings) + SSE events; loopback-only; 405 method guards; unit
      tests with a real engine over a fake network
- [x] Frontend http adapter with auto-detection (wails → http → mock);
      i18n OS-word swap; tray-copy capability gating (help q2, first-run text,
      separate-Chrome row via the new browserSession capability)
- [x] cmd/core --web: single-executable edition launcher (port fallback,
      connectOnLaunch parity, best-effort xdg-open, Quit exits the process)
- [x] windows/386 target (D51): full GUI via wails + core; parseDword 32-bit
      overflow fixed; linux/386 single executable
- [x] build.bat builds all four Windows files; build.sh cuts all six targets
      from one tree (D52: sync by construction)
- [x] E2E on the real Linux binary from the build machine: first-run →
      connect ON with real YouTube round-trip (86–91 ms) → settings/help/about
      → forced PORT_BUSY error state → recovery → quit; autostart .desktop
      written through the real stack; zero console/page errors
- [x] Gates: go vet ./...; go test -race ./... (14 pkgs incl. webui + linux);
      GOOS/GOARCH builds for windows/amd64+386, linux/amd64+386, arm64
      neutral; check-platform-neutral; npm run build (94.4 KB JS); svelte-check
      (12 pre-existing only); live selftest 1.4.0 all OK (youtube 81 ms, video
      92 ms)
- [x] Docs: README (other versions, Linux how-to, two manual check lists),
      DECISIONS D49–D52, this phase, PROGRESS; screenshots: 6 Linux states +
      re-shot Windows error/about; VLM QA passed (one "About clipping" claim
      disproved by pixel measurement — zero overflow)

## Phase 12 (v0.5.1 / app 1.4.1) — the proper Linux Mint packages

- [x] D54: autostart Exec/home resolution for the packaged editions
      ($APPIMAGE → the .AppImage file, $FLATPAK_ID → "flatpak run <id>",
      real passwd home under Flatpak); 4 new unit tests (7b–7d)
- [x] packaging/ tree (D53): gen-icons.py redraws the logo at 4096 px and
      emits the hicolor set (16–512, both icon names, committed); one desktop
      template rendered per format; build-deb.sh / build-appimage.sh /
      build-flatpak.sh; packaging/README.md
- [x] .deb (amd64 + i386): dpkg-deb --root-owner-group, control/Depends/
      Recommends, postinst/postrm icon-cache refresh, md5sums, copyright;
      E2E: extracted tree, --version 1.4.1, live control page + appinfo
- [x] AppImage (x86_64 + i686): appimagetool fetched to ~/.cache (FUSE-free
      via --appimage-extract-and-run), i686 via --runtime-file; E2E through
      the real AppImage runtime: version, web edition, settings round-trip,
      autostart entry naming the .AppImage file (D54)
- [x] Flatpak (D55): manifest io.github.assadigit.Younblock.json + metainfo;
      user-space flatpak 1.16.6 + flatpak-builder 1.4.4 + GNOME 48 runtime
      installed in-sandbox; flatpak-builder blocked by the container's
      pidns/proc restriction → low-level fallback (same manifest, build
      commands executed with /app redirected, build-finish before export
      collection); bundle installed --user and verified end-to-end (deploy
      metadata, byte-identical binary, desktop, icons, live API, D54 entry);
      flatpak run stays Ali's manual check
- [x] build.sh extended: one run cuts 6 binaries + 2 debs + 2 AppImages (+
      flatpak when the tools exist); npm ci legacy-peer-deps retry (the
      lockfile is content-synced; the svelte plugin just hasn't widened its
      vite peer range)
- [x] Version 1.4.1 in defaults.go / wails.json / mock.ts
- [x] Gates: go vet ./...; go test -race ./... (14 pkgs); ./build.sh all
      green (linux binary reproducible, sha 07ea6373…); live selftest 1.4.1
      (youtube 81 ms, video 95 ms); check-platform-neutral; arm64 neutral;
      svelte-check (12 pre-existing only); PE/ELF format checks for every
      artifact
- [x] Docs: README (which-format guide, download table, package manual
      checks 7–8, Linux uninstall), DECISIONS D53–D55, this phase, PROGRESS
