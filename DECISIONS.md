# Younblock — Decisions Log

Every place where the agent interprets or deviates from the specs, with the reason, plus
one-line justifications for any dependency beyond the allowed list (file 01 §3: standard
library, `golang.org/x/net/dns/dnsmessage`, `golang.org/x/sys`, `golang.org/x/sync/singleflight`,
Wails v2, one tray library). Newest entries at the bottom.

**D1 — Repo layout differs from the expected paths.**
The specs sit at the repo root (not `docs/specs/`), and `backend.py`, `frontend.py` and
`logo.png` were uploaded at the root (not `legacy/` and `assets/`). Adaptation: moved
`backend.py` → `legacy/backend.py`, `frontend.py` → `legacy/frontend.py`, `logo.png` →
`assets/logo.png` with `git mv` (history preserved). The five spec files stay at the root
because they reference each other by bare filename and they are the repo's front page.

**D2 — How the old app made Chrome use the proxy (required by file 01 §2).**
The old app never touched Windows proxy settings. `legacy/frontend.py` launched a **dedicated
Chrome process** on Connect: `chrome --user-data-dir=<app dir>/proxy-profile
--proxy-server=127.0.0.1:4525 --proxy-bypass-list=<-loopback> --no-first-run --disable-gpu
--disable-software-rasterizer https://www.youtube.com/` (1.2 s after the button press), and on
Disconnect it terminated that Chrome process while the proxy listener kept running until app
exit. The separate `--user-data-dir` is what let the flags apply while the user's normal
Chrome kept running untouched.
Comparison with file 01 §8 (the new approach): the new app sets the **system** PAC URL
(`AutoConfigURL` in `HKCU\...\Internet Settings`) so the user's existing Chrome follows it,
with `DIRECT` fallback, backup/restore and crash recovery. Differences Ali will see: the new
app does not launch Chrome, does not open YouTube automatically, and affects the user's normal
Chrome (YouTube hosts only, in the default `youtube` mode) instead of a separate instance. The
spec wins; this entry records the old mechanism as required.
Extra legacy notes: single-instance lock was a bound socket on 127.0.0.1:54321; the old
"Bandwidth" display was random simulated numbers (`simulate_data_usage()`), not real traffic;
the old About text claimed "faster than most VPNs", which the new specs forbid.

**D3 — Reply headers.**
The Python added `Proxy-agent: MyProxy/1.0` to its 200/302/400/502 replies. File 01 specifies
the exact reply lines without it. The new code follows file 01; the header is cosmetic and
browsers ignore it.

**D4 — Unknown HTTP methods.**
The Python returned `400 Bad Request` for anything that was not CONNECT/GET/POST. File 01 §7
says `405` for "anything else" (and §2 extends the redirect to HEAD). File 01 wins: 405.
Unparseable requests (malformed HTTP) still get `400`, which is the standard code for them.

**D5 — What gets fragmented on a tunnel.**
The Python fragmented whatever bytes the first `recv` returned, TLS or not. File 01 §5 says:
only if the first byte is `0x16` (TLS handshake record); otherwise pass through untouched.
File 01 wins (this also fixes non-TLS protocols going through the proxy).

**D6 — DoH transport.**
The Python sent its DoH requests through its own proxy port (a loop through itself). File 01
§2/§6 and R5 say the Go version dials DoH directly through `netx` with the fragmented
ClientHello, `Proxy: nil`. File 01 wins. The v1 DoH request shape is preserved exactly
(GET, `?dns=<base64url, no padding>`, `accept: application/dns-message`, plus the extra
`type=A&ct=application/dns-message` query parameters).

**D7 — Extra DoH query parameters for the fallback providers.**
File 01 §2 says replicate the Python's exact request (including `type=A&ct=…`) and §4 lists
Cloudflare and Google with the same `{dns}` template shape. The extra parameters are appended
for every provider, not only RethinkDNS: they are standard DoH servers that ignore unknown
query parameters, and keeping one request shape keeps the code honest to the v1 behavior.
Verified harmless in Phase 1 tests with a fake server.

**D8 — Legacy fragmentation edge cases.**
The Python's `random.sample(range(1, l-1), 299)` crashes when `l == 300` (it asks for 299
distinct values from a range of 298). File 01 §5 fixes this with `min(299, length-2)` cut
points. File 01 wins. Also: file 01's Phase 1 acceptance says "`legacy` at most 299 pieces"
but the §5 formula (and the Python) produce **299 cut points = up to 300 segments**. The
Python's behavior is kept exactly (299 cut points, `min(299, length-2)` when the range is
smaller); the test asserts at most 300 segments and at most 299 cut points. The "299" in the
acceptance text is read as counting cut points.

**D9 — "Remember the last that worked."**
R4 (try every returned address in order) applies to website IPs with no stickiness; R15's
"remember the last good one" applies to the DoH server list. Implemented that way.

**D10 — `go.mod` Go directive.**
Started at `go 1.22` (file 01's minimum). If a dependency's own `go.mod` demands a higher
version, the directive is bumped to the minimum that works and noted here.

**D11 — Log rotation size.**
File 01 §4 says "file `%APPDATA%\Younblock\logs\app.log` (2 MB x 3)". Interpreted as: the
current file plus two rotated backups (`.1`, `.2`), each capped at 2 MB — three files total.

**D12 — Windows-only code on a Linux sandbox.**
The real registry/proxy/tray code is never run during development (per the non-negotiables);
it is verified by `GOOS=windows GOARCH=amd64 go build ./...` plus unit tests against in-memory
fakes of the registry and WinINet calls. The real check remains Ali's manual Windows step.
Phase 1's Windows factory returns the stub platform; Phase 3 replaces it with the real
`internal/platform/windows` package.

**D13 — Toolchain locations (for README tool-install steps).**
No `sudo` on this machine, so Go, gh and Wails live in user space: `~/sdk/go`, `~/sdk/gh`,
`~/go/bin/wails`. Windows users follow the normal installers instead (documented in
`README.md`).

**D14 — Token security.**
The GitHub token Ali pasted in chat was used once to authenticate `gh` and lives only in
gh's own credential store (`~/.config/gh/hosts.yml`), never in the repo, code, logs or
commits. Ali should revoke/rotate it because it was exposed in plaintext in the conversation.

**D15 — Connection cap test strategy.**
The 1024 default cap is asserted in config tests; the 503-on-overflow behavior is tested with
a small cap override (same code path). The sandbox's file-descriptor limit makes a literal
1025-connection test (≈2100 open fds) unreliable; this is noted in `PROGRESS.md` rather than
weakening anything.

**D16 — Tray library choice (Phase 4, chosen in advance pending verification).**
`energye/systray` (a getlantern/systray fork with an external-loop mode) is the one tray
library file 01 §3 and §9 allow. It was chosen because it is documented to run beside the
Wails v2 event loop via `systray.RunWithExternalLoop`. This is verified against the installed
version in Phase 4 before committing to it; if it cannot coexist with Wails after 3 honest
attempts, that is a STOP condition and Ali gets the options.

**D17 — `timeBeginPeriod(1)` (Windows sleep accuracy).**
File 01 §5 makes it optional ("kept only if measurement shows a benefit"). The real sleep
length is measured and logged at startup; `timeBeginPeriod` is **not** enabled because its
benefit cannot be measured on this Linux sandbox. The measured value ships in the diagnostic
report so Ali can see the real number on Windows.

**D18 — Phase boundary for `--selftest` and R19.**
`cmd/core`'s `--selftest` flag implements the four real checks from Phase 1 (they are needed
by the engine skeleton anyway); Phase 2 formalizes `health.RunSelfTest`, the heal sweep, the
watchdog and notices on top of it. R19 (network-change detection) lands in Phase 2 with the
rest of the health machinery, not Phase 5, because it is the watchdog's trigger. `PLAN.md`
reflects this.

**D19 — go.mod language version.**
Started at `go 1.22` (the spec minimum), but `go get` raised the directive to **1.26.0**
because golang.org/x/net v0.61.0 requires it. Still above the spec minimum; the installed
toolchain (1.27.2) builds it.

**D20 — Platform.RestoreProxy.**
The spec's Platform list (file 01 §3) has no restore method, but the API table (§10)
requires `RestoreWindowsProxy()` and the CLI needs `--restore-proxy`. The engine
interface therefore gained `RestoreProxy() error` (no-op on the stub). This is the
documented reason, not a silent deviation.

**D21 — OpenLogsFolder is split in two.**
`engine.OpenLogsFolderJSON()` returns the log folder path (neutral), and the shell
(Wails `app.go`, or the CLI printing it) opens the actual file browser. Opening a window
is shell behavior; the engine stays platform-neutral.

**D22 — Terminating proxy replies.**
File 01 gives bare status lines (`502`, `405`, …). The implementation adds standard
`Content-Length: 0` and `Connection: close` headers on terminating replies so every HTTP
client interprets them correctly. The 200 tunnel reply and 302 Location follow the spec
exactly (no Proxy-agent header, unlike the old Python — D3).

**D23 — Oversized first records.**
A TLS record declaring more than 16 KB + 5 is fragmented only up to the cap; the
remaining bytes flow through the normal copy loop. This is the closest correct reading
of "read exactly that many bytes (cap 16 KB + 5)".

**D24 — Test-only TLS injection in health.**
`health.Config.WebTLS`/`VideoTLS` let tests inject trust pools for local fake servers;
production leaves them nil and uses the system roots, exactly like the real checks.

**D25 — CLI overrides are not persisted.**
`cmd/core --port` and `--preset` override the loaded settings in memory only; they are
never written to settings.json. They are developer/testing knobs.

**D26 — Engine settings application.**
A port change while On applies on the next Start (a listener cannot be re-bound
mid-flight without dropping every tunnel); mode and method changes apply immediately, as
file 01 §10 requires. The engine clears the DNS cache when the secure-DNS list changes so
answers come from the new servers.

**D27 — Notification rate limiting is episode-based.**
File 01 §9 wants the pair "YouTube stopped responding. Trying to fix it…" followed by
"Fixed" or "Couldn't fix it. Open Younblock." with a one-per-10-minutes rate limit.
Limiting every individual message would swallow the outcome of a pair that was just
announced, so the 10-minute window throttles notification EPISODES: a message within
60 s of the previous one belongs to the same episode and is delivered; a new episode
inside the 10-minute window is suppressed. Repeated flapping is throttled; the pair the
spec describes is always shown.

**D28 — A sweep's winner is persisted as the method.**
File 01 §10 says a passing sweep result is "saved as the working method"; the engine
writes it to settings.json immediately (and announces it). R23's remaining part
(persisting the last good DoH server) stays P2.

**D29 — Sweeps only trigger automatically from On (or a Start press).**
From degraded, recovery happens through the watchdog's success path (one later success
returns to on) or the user's Fix connection button (AutoFix), per R14. The watchdog's
failure counter resets on success.

**D30 — Static ProxyServer fallback not implemented.**
File 01 §8 says: if PAC-via-registry is not honored on the test machine, fall back to a
static ProxyServer for all mode and report that youtube mode needs another method. That
fallback is contingent on a real-Windows test result (whether Chrome honors
AutoConfigURL on Ali's machine), which cannot be observed from this sandbox. The primary
PAC path is implemented and tested; if Ali's manual check shows PAC is not honored, the
static fallback is the documented next change. The backup format already records
ProxyEnable/ProxyServer/ProxyOverride so the undo data exists.

**D31 — Windows platform notifications before phase 4.**
Platform.Notify on Windows currently logs only; phase 4 wires the toast through the tray
library (or reports if none works, per file 01 §9). The engine's notification logic is
complete and test-proven; only the toast surface is pending.

**D32 — Platform factory falls back to the stub.**
If the Windows adapter cannot initialize (config dir unavailable), the factory returns
the stub so the app still runs without system-proxy support, and says so on stderr.

**D33 — go-toast for notifications.**
File 01 §9 explicitly allows "the tray library or a toast library". energye/systray has
no notification API, so go-toast (pure Go, drives the Windows toast API via SnoreToast)
was added with that one-line justification. Whether toasts actually render can only be
confirmed on Ali's machine; if they do not, the tray attention icon, the tooltip and
the UI notice line already carry every failure/recovery message, which is the fallback
file 01 §9 sanctions.

**D34 — The frontend bundles a copy of assets/logo.png.**
The Wordmark component loads `/logo.png` from the frontend's public folder; the source
of truth stays `assets/logo.png` (used for the app icon). Keeping a copy avoids coupling
the web build to a path outside `frontend/`.

**D35 — main.go and app.go carry no build tag; only the tray does.**
Wails' binding generation compiles the root package for the host platform, so the entry
points must compile on Linux; `tray_windows.go` (energye/systray pulls Linux GUI
libraries) is windows-only with a no-op `tray_other.go` counterpart for development
builds. The platform-neutral rule from file 01 §3 only constrains the internal/ packages,
and this stays within it.

**D36 — Svelte 5 mount().** The template's legacy `new App()` instantiation breaks
runes-based components ($effect orphan errors); `mount(App, { target })` is the supported
Svelte 5 API and is what ships.

**D37 — Distribution via GitHub Releases; binaries never enter the repo.**
Ali asked for a single testable exe. Release tag `v0.1.0` carries `younblock.exe`
(the app — the one to run) and `younblock-core.exe` (the headless engine with
`--selftest`/`--restore-proxy`). The repo stays source-only (`dist/`, `build/bin/`
git-ignored) so clones stay small and the release page is the one source of truth
for binaries and checksums. The exes are unsigned (no code-signing certificate was
bought), so SmartScreen will show "Windows protected your PC"; the release notes and
README both explain **More info → Run anyway** and how to verify the SHA-256 with
`certutil` instead. Wails apps need the WebView2 runtime, present on Windows 11 and
any Windows 10 with Edge — noted in the release notes.

**D38 — Correction of the Phase 5 core-exe record.**
Rebuilding for the release proved the Phase 5 `dist/younblock-core.exe` was a Linux
ELF binary named `.exe` (built without `GOOS=windows`; a stripped Linux build is
byte-count identical to the old log entry). It would not have run on Windows. The
release build uses the explicit `GOOS=windows GOARCH=amd64` and `file` confirms
PE32+; the correct numbers are recorded in the v0.1.0 PROGRESS entry. Logged openly
here rather than quietly replacing the old entry: the log is append-only.

**D39 — Phase 6 scope: post-spec features requested by Ali after v0.1.0.**
Ali tested v0.1.0 on real Windows ("the backend and frontend both works") and
asked for five additions. The specs (files 01–03) are complete and frozen, so
these are recorded as user-requested extensions, each with its own decision
below, all default-off or additive so the spec'd default behavior is unchanged.

**D40 — The optional dedicated Chrome session (request #2).**
File 01 §8 deliberately replaced the old app's "launch a dedicated Chrome with
--proxy-server" approach with the system PAC setting. Ali asked for the old
behavior as an option. Implementation: a new `openBrowserOnConnect` setting
(default off, so the spec behavior stays the default); the pure command-line
and lookup logic lives in the neutral `internal/browser` package (tested with
injected fakes); the actual spawn lives in the shell (`browser_windows.go`,
registry + standard-path lookup) with a no-op counterpart for development
builds, following the D12 pattern. The flags replicate `legacy/frontend.py`
exactly (user-data-dir under `%LOCALAPPDATA%\Younblock\chrome-profile`,
`--proxy-server=127.0.0.1:<port>`, `--proxy-bypass-list=<-loopback>`,
`--no-first-run`, `--disable-gpu`, `--disable-software-rasterizer`,
youtube.com as the start URL). Because that Chrome is hard-wired to our port,
turning the app off ends the session (the legacy app did the same); the shell
announces every launch/close/failure in the log, the UI notice toasts and a
`browser:*` event (R13). Chrome-not-found is announced, never fatal.

**D41 — The "ping" is a full-chain response time (request #4).**
ICMP ping needs raw sockets and is meaningless for HTTPS through a relay.
The honest number Ali can act on is the round-trip of the app's own YouTube
check (secure lookup + connect + request) — the same measurement the
connection test already reports per step. Every successful health check
(start quick-check, watchdog tick, network-change re-check, full test)
records it; it appears in the state JSON as `latencyMs`, under the timer as
"NN ms · … used", and in the diagnostic report as "response time". It resets
on Stop (a live metric, not history).

**D42 — The background texture (request #3).**
File 02 section 2 specifies the flat `--neutral-950` background. Ali asked
for texture. The change is two soft radial brand glows (red top, green
bottom, ~5% alpha) plus a fine fractal-noise SVG grain at 4.5% opacity,
entirely inline (a data-URI, no fetches — the CSP already allows `img-src
data:`; R21's "frontend loads nothing remote" still holds). Kept subtle so
the file 02 layout, contrast and copy are unchanged; all 17 screenshots were
re-taken and vision-checked.

**D43 — "Route all browsers through this DNS" maps to the existing mode (request #1).**
The app already had the "All websites" mode (PAC: every public host through
the proxy, which resolves via the secure DoH) — it was just buried in
Advanced. Rather than a new mechanism (changing the system's DNS servers
would need admin rights and a local DNS server — a much bigger, riskier
subsystem the specs never ask for), the setting is surfaced as a top-level
switch in Settings that toggles mode `all`/`youtube`. The Advanced radio
group stays in sync and remains the way to pick Manual. Noted in the README.
Ali's `icon.ico` upload moved to `assets/icon.ico` (D1 layout) and is
installed as `build/windows/icon.ico` + `build/appicon.png`; the tray keeps
its state icons per file 01 §9.

**D44 — The v0.2.1 "won't connect" hotfix: say what failed, stay alive, and make the DNS list actually switchable.**
Ali reported v0.2.0 "didn't even connect" (screenshot: degraded, frozen 00:00:00
timer). Investigation first: `git diff v0.1.0..v0.2.0` shows the network stack
(proxy/doh/netx/fragment/health/platform) byte-identical between the release
that worked on his machine and the one that didn't, and the exact shipped code
passes a live self-test against real RethinkDNS + YouTube from the dev box —
so the regression is environmental on his side (his network's blocking most
likely changed; the app's code path did not). What the report DID expose is
that degraded mode was a dead end: the quick check and heal sweep had failed,
the app showed only "On, but YouTube isn't responding" with no hint of WHICH
part was blocked, the stats loop never started (frozen timer, no usage line),
and — the real defect — the watchdog and network monitor only started from
`goOn`, never from `goDegraded`, so a start that ended degraded sat there
forever with no self-recovery, against file 03 R14's "runs while on or
degraded". Fixes, all spec-aligned: (1) the engine records the failing
self-test step and code (root-cause ordered: a dns failure is not overwritten
by the youtube failures that cascade from it), surfaces it in the state JSON
as `fail`, the subtitle ("On, but secure DNS isn't answering" vs the existing
line) and the diagnostic report ("last failed check: …"); (2) `goDegraded`
now starts the stats loop, watchdog and network monitor, so the timer runs
and the app recovers by itself when the network allows; (3) changing the
secure-DNS list in settings now swaps in a resolver bound to the new servers
immediately (atomic pointer; the running proxy gets it via a new
`SetResolver`) — before, the change only cleared the cache while the old
servers kept being queried until an app restart, silently defeating the one
remediation for a DNS-level block; (4) the Advanced list is finally editable
in the UI (remove buttons; one-tap add of AdGuard/Quad9/dns.sb or the
defaults back; a newly added provider goes to the top and is tried first),
because telling a non-coder to hand-edit settings.json is not an honest fix.
No claim is made that any particular provider works on any particular
network; the status line and the test verify what actually answers.
