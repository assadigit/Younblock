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

**D45 — The v1.2.0 power-button fix and visual polish: motion that survives
reduced-motion, and a footer that tells the truth.**
Ali reported the power button animation "stuck midway" and asked for a more
professional look; the footer also said "Works with Chrome" while the app
sets the Windows proxy, which every mainstream browser follows. Root cause of
the freeze (reproduced by emulating `prefers-reduced-motion` in the sandbox
browser): base.css killed *all* animation and transition durations under that
media query with `0.001s !important`, so on Windows with system animations
off (or over RDP, which WebView2 mirrors) the connecting arc — a functional
progress indicator — spun once and then sat frozen mid-circle for the rest of
the start sequence, and the turn-on ripple never appeared. The WCAG-correct
policy replaces the blanket rule: movement-based *decoration* (the echo
rings, the disc spring pop, the glow breath, press scaling, toast slide) is
disabled per component under reduced motion, while the connecting spinner
keeps spinning and color/opacity crossfades keep running — they carry state
information and cause no vestibular motion. On top of the fix the turn-on
moment is now fully orchestrated (spring pop, two staggered echo rings, a
crossfaded gradient disc — gradients cannot tween, so the lit plate is an
opacity-crossfaded layer — and a slow glow breath), the press feedback was
tightened and can no longer stick (release on click/pointercancel/blur), the
degraded rim is a solid amber glow instead of the wireframe-looking dashed
ring (color + label still differ from error), and the status block gains a
state-colored dot, a warmed degraded subtitle and an amber ghost "Fix
connection" action. The footer badge now reads "Works with every browser",
and Help/About/first-run copy says the same truth precisely: the app sets the
Windows proxy, so Edge, Chrome, Firefox, Opera, Brave and the rest all work;
it is simply tested most with Chrome. The optional dedicated-Chrome-session
setting keeps its name — that feature really is Chrome-only. File 02's
section 4.1/7 button look is superseded here by Ali's explicit request
(recorded so the spec and the code don't silently disagree).

**D46 — v1.3.0 switches the Windows system proxy from a PAC script to the
manual proxy values, so every browser actually picks it up.**
Ali reported that only the dedicated Chrome session (launched with
`--proxy-server`) worked; other Chrome profiles and Opera did not. The app
had been setting `AutoConfigURL` to `http://127.0.0.1:<port>/proxy.pac` —
a PAC script several real-world things can quietly break: browsers that
cached their proxy settings keep them until a restart or a
WM_SETTINGCHANGE, other proxy tools overwrite the registry, and a PAC left
behind by anything takes precedence over the manual values in WinINet. The
manual values (ProxyEnable=1, ProxyServer=127.0.0.1:<port>, and a bypass
list for localhost/private ranges) are the mechanism v2rayN, Clash and
friends use on millions of Windows machines — every mainstream browser
follows them. The switch: Enable writes those values, clears any
AutoConfigURL while we are active (ours from an older version, or another
program's — a PAC would win over ProxyServer), keeps the backup/restore
state machine (which already anticipated static proxy), and the refresh now
also broadcasts WM_SETTINGCHANGE("Internet Settings") beside
InternetSetOption(39/37) so even running browsers re-read their settings.
A legacy PAC URL from a crashed pre-1.3.0 run without a backup is removed
at startup (cleanupLegacy) and never recorded as the user's own. The
watchdog re-asserts the values on every tick (EnsureAttached) so another
tool clobbering them is repaired within ~2 minutes and surfaced as a
PROXY_REAPPLIED notice (R13). Consequences inside the proxy: in youtube
mode every host's traffic now passes through the app (the PAC used to keep
non-YouTube traffic out), so the first TLS record is fragmented only for
the routed hosts (unfragmented relay for everything else), and plain-http
requests to non-routed hosts are forwarded like a normal proxy (origin-form
rebuilt, hop-by-hop headers stripped, Connection: close) instead of the
http→https redirect, so plain-http sites keep working. The /proxy.pac
endpoint stays served for tests and anyone who points at it manually.
File 01 §8's PAC approach is superseded by this field report.

**D47 — v1.3.0 restyles the UI to Ali's logo: warm charcoal, padlock red,
lightning gold. The green is gone.**
Ali disliked the color combination, the green in particular, and asked for
a palette harmonized with the logo (red padlock body, dark-charcoal
shackle, white play triangle, gold lightning bolt). The neutrals moved
from cool zinc to warm charcoal; the primary/"on" color is the padlock red
(#DD3233 family) — a "live" red like a recording light, fitting for an
app whose whole job is to be on; the gold of the lightning bolt
(#FFC53D family) is the warn/connecting/accent color and the turn-on echo
rings (the flash), plus the focus rings and the OK checkmarks in the
connection test. The "on" glyph is white (the play triangle's white on the
red padlock). Danger keeps its distinct coral (#FF6A5F) and differs from
the on-state by treatment and label, not just hue. Tray icons regenerated
in the same palette (red on-icon, gold attention dot, warm gray off).
File 02 §2's token table is superseded by the logo-derived one.

**D48 — The home screen's height budget and the window's real size (Ali's
"white space" / "Fix connection is on the edge" report).**
Two defects stacked up. (1) Wails' initial SetSize applies the logical
360x560 to the OUTER window rectangle (winc scales by DPI, then MoveWindow),
so the title bar and frame shrink the CSS viewport to ~336x522 at Ali's
scaling — every state was drawing in a window smaller than designed. The
app now corrects the outer size at startup so the client area is exactly
360x560 logical at the window's DPI (user32 via x/sys/windows in
clientsize_windows.go, a build-tagged shell file like the tray and browser
launchers; app.go/main.go stay tag-free per D35). (2) The Connect screen's
fixed vertical budget had no overflow plan: degraded (574 px) and error
(634 px) stacks pushed the footer out of the window and slammed the
Fix-connection pill onto the bottom edge, while the centered flex left a
dead gap mid-window. The shell now owns the height budget (flex column with
a flex:none footer that can never be pushed out), Connect centers with
`safe center` and carries an internal scroll safety net, the timer/quiet
block collapses in states that can never show it (0fr↔1fr grid-rows), the
error reason text is capped with its own scroll, and a responsive ladder
(button 0.88 below 546 css px; whole-screen zoom 0.82 below 470 —
physically correct on 150%-scaled displays) keeps every state inside any
viewport the fixed window can produce. File 02 §4.1's fixed-metrics table
gains these budget rules; the 52 px header (§3) becomes 44 px.

**D49 — v1.4.0: in the error state the reason is the subtitle.**
The old error screen reserved an always-present subtitle line that sat empty,
then rendered the failure reason in a separate small paragraph below the
collapsed timer block — one message split over two slots with a dead gap
between them. The reason (PORT_BUSY, DNS_…, UNKNOWN wording) now renders in
the subtitle slot, directly under the state label where the eye lands first,
danger-colored, keeping D48's compact budget (280 px line cap, 100 px height
cap with its own scroll) so a long UNKNOWN paragraph still cannot push the
footer out. The error row keeps only the "Open help" link. This also shortens
the tallest status stack, which is what the D48 ladder was fighting.

**D50 — v1.4.0: the Linux edition is the same engine and the same Svelte UI,
delivered as one static executable that serves its own control page.**
Ali asked for "a version working on Linux Mint as a single executable."
Wails-on-Linux needs webkit2gtk system libraries, which breaks the one-file
promise, so the Linux edition is the headless core grown a web door instead:
`internal/webui` embeds the exact frontend bundle (go:embed; build.sh copies
frontend/dist in) and exposes the same JSON API the Wails shell binds, with
engine events pushed as Server-Sent Events; the frontend gained an `http`
adapter (auto-detected when /api/appinfo answers; mock stays the plain-browser
fallback, wails bindings still win in the desktop webview). One engine, one UI,
one copy deck — the Windows-flavored wording is reused with the OS word
swapped in place (`setOsWord`, applied before first render), tray-only copy is
capability-gated (help q2 hidden, first-run text switched, the separate-Chrome
setting row hidden via a new `browserSession` capability). The Linux platform
adapter (internal/platform/linux) sets the system proxy through gsettings
(org.gnome.system.proxy — the schema GLib's own proxy resolver reads, so
GNOME/Cinnamon/MATE browsers follow it; speculative cinnamon/mate schemas are
only touched if present), backs up and restores raw GVariant literals verbatim
(no parsing: gsettings accepts its own printed forms — verified against real
glib 2.84 with the upstream schema compiled in; the dconf persistence path
cannot be exercised in this sandbox and stays Ali's manual check), autostarts
via ~/.config/autostart (Exec carries --web), and notifies via notify-send.
Detach without a backup only clears values that carry our three markers
(manual mode + our loopback host + our exact bypass list) — an unconditional
clear would wipe a manual-mode user's own proxy on every Stop, because the
engine calls Detach regardless of mode and this edition keeps no backup file.
Headless boxes (no gsettings) honestly report systemProxy=false and the UI
shows the manual address. `--web` also works in the Windows core (a browser
tab instead of the window) at no extra cost.

**D51 — v1.4.0: 32-bit builds are first-class targets.**
Ali asked for "a 32bit version." `wails build -platform windows/386` produces
the full GUI app (the Win32 code — tray, toasts, DPI fix, registry — is
syscall-based and compiles unchanged), and the core cross-builds for
windows/386, linux/amd64 and linux/386 (CGO off, statically linked, so the
Linux files run on any Mint regardless of age). One real 32-bit defect surfaced
and was fixed: parseDword's `n > 1<<31` int check does not even compile on
386 (int is 32 bits there); it now accumulates in uint64 and bounds at
MaxUint32, returning uint32. 386 test binaries cannot run in this sandbox
(seccomp blocks 32-bit syscalls) — verified by compile + vet + the amd64 test
suite instead, the same standard the Windows builds always had.

**D52 — v1.4.0: versions stay in sync by construction, not by discipline.**
Ali: "every change we do on the main version must be applied on them as well."
The three editions are not forks or copies that could drift: they are build
targets of one source tree (one engine, one Svelte bundle, one copy deck, one
version number in config/defaults.go). build.sh and build.bat cut every target
from the current tree in one run, and every release page carries all six
files. Nothing can be "applied to the other versions" separately because there
is nothing separate to apply it to.

**D53 — v1.4.1: one packaging tree, every format wraps the same static ELF.**
Ali asked for three Linux Mint formats — .deb ("best for installing software
properly"), AppImage ("a single, portable executable"), Flatpak ("convenient
installation and updates with fewer dependency conflicts"). All three wrap the
exact same younblock-linux binary D50 already produces (static, embedded UI,
--web); nothing format-specific exists inside the app, so D52's sync-by-
construction extends to the packages: build.sh cuts the binaries and then the
deb/AppImage/flatpak wrappers in one run. The format pieces live in packaging/
— one desktop-file template rendered per format (@EXEC@/@ICON@), one icon set
(younblock + io.github.assadigit.Younblock names) generated by gen-icons.py,
and one build script per format. The icons are a high-resolution redraw of
Ali's 32 px logo (the only source) — pixel-measured palette and layout,
checked against the original with a 32 px side-by-side ASCII map; the
generated set is committed so builds need no PIL. Flatpak id:
io.github.assadigit.Younblock (the no-domain GitHub convention, Flathub-ready).

**D54 — v1.4.1: the autostart entry names the edition's launcher, not os.Executable().**
Packaged editions run through a launcher that os.Executable() cannot express:
inside an AppImage the binary path is the runtime's temporary mount
(/tmp/.mount_…, gone after logout), and inside Flatpak it is the sandbox's
/app path, meaningless to the desktop session. SetAutostart now resolves the
Exec= line through autostartExec(): $FLATPAK_ID → "flatpak run <id>";
$APPIMAGE → the .AppImage file (both set by their runtimes, honored only
when present); otherwise the executable, as before. Under Flatpak $HOME
points into the app's private data tree, so autostartHome() targets the
real home from /etc/passwd there — the entry must land in the real
~/.config/autostart. Verified end-to-end in this sandbox through the real
runtimes: the AppImage (via --appimage-extract-and-run, $APPIMAGE set by the
AppImage runtime) wrote Exec=/…/Younblock-1.4.1-x86_64.AppImage --web, and
the flatpak-deployed binary with FLATPAK_ID set wrote
Exec=flatpak run io.github.assadigit.Younblock --web.

**D55 — v1.4.1: the Flatpak edition's permissions and its honest verification limits.**
The manifest (packaging/flatpak/io.github.assadigit.Younblock.json) requests
--share=network (the engine), --socket=session-bus (gsettings/dconf and
notifications reach the REAL desktop — dconf is a session-bus service, so
writes inside the sandbox land in the host's database), --filesystem=host
(the autostart entry and, for the .deb/AppImage editions, ~/.config/Younblock
live on the real home), and --talk-name=org.freedesktop.Notifications. The
Flatpak edition keeps its own settings in its sandbox (~/.var/app/…) by
design — the .deb and AppImage share ~/.config/Younblock; documented in the
README. Build honesty: this sandbox's container blocks the nested PID
namespace bwrap needs (proc-in-pidns mount is denied), so flatpak-builder and
flatpak run cannot execute here. build-flatpak.sh therefore tries
flatpak-builder first (the normal path on any real Mint) and falls back to
flatpak's low-level build-init/build-finish/build-export/build-bundle
commands — executing the manifest's own build-commands with /app redirected,
so the manifest stays the single source of truth. The fallback needed the
build order flatpak-builder itself uses (build-finish BEFORE collecting
export/, which otherwise reads as "already finalized"). The bundle was then
installed with `flatpak --user install` inside the sandbox and verified
end-to-end: deploy metadata, byte-identical /app/bin/younblock, desktop file,
icons, live control-page API, and the D54 start entry — everything except
`flatpak run` itself, which stays Ali's manual check on real Mint (README
list, step 8). appstream-compose (store metadata) is best-effort in the
fallback — Debian's appstreamcli rejects our hicolor icon set with
icon-not-found and mandates an icon policy it also refuses to parse; the
bundle installs and runs identically without the AppStream branch, and the
flatpak-builder path regenerates it on a normal host.

**D56 — v1.5.0: one shared screen chrome (views.css) instead of three
hand-copied variants.**
Settings, Help and About each carried their own copy of the view header,
back button, scroll body and section-title styles — three variants of the
same thing that had already drifted (12 px vs 16 px title sizes, different
paddings). A new global stylesheet (styles/views.css, imported once in
main.ts) now owns `.view`, `.view-header`, `.view-title`, `.back`,
`.view-scroll`, `.overline`, `.card` and `.reveal`, and every screen
mounts on those classes. The type ramp gains the missing steps the
screens were improvising: an 11 px overline (tracked-out, 600, uppercase —
one voice for "Connection test", "Advanced", "How it works" and the home
footer chip), a 17 px view title at 600, a `--text-2xs` size, a mono face
for addresses/URLs, a raised surface step (`--color-surface-2`), card
elevation (`--shadow-card`), and a duration scale (`--dur-fast/med/slow`)
beside the easing ones so every entrance, reveal and hover in the app
moves on the same clock. Content that belongs together (the Help test
block, the rescue actions, the About sections, the Advanced groups) now
sits on cards instead of floating on the grain. File 02's per-screen
fixed metrics keep their D48 budget; the Connect screen's structure is
untouched — only its footer chip and shared classes changed.

**D57 — v1.5.0: every control is drawn to one standard, and the focus
ring stopped reshaping what it lands on.**
Three defects and one gap, fixed together. (1) The global `:focus-visible`
rule set `border-radius: var(--radius-sm)` on the focused *element*, so
keyboard-focusing a pill button (Fix connection, the chips) squared its
corners for as long as the ring showed — the rule now only paints the
outline (2 px gold, offset 2) and lets the element keep its own shape.
(2) The power button's hover selector outranked `.on .disc` and turned
the lit red plate's border gray under the pointer; hover is now scoped to
the resting (non-on) states, where "the plate wakes up" was always the
intent. (3) The Add-provider chips were 36 px and the manual-address copy
button 32 px — under the 44 px touch-target rule every edition follows;
both are 44 px now, with press feedback. And the gap: Advanced's radio
groups were native `accent-color` dots sitting next to the custom switch —
they now render as the brand radio (a ring that fills with the padlock
red, spring-scaled, focus ring on the ring), the FirstRun checkbox the
same way, the port input gets hover/focus border states, the "tried
first" marker became a small gold badge, and DoH URLs and the domain list
set in the mono face.

**D58 — v1.5.0: motion choreography under the D45 policy — and a real
bug it flushed out.**
Entrances and reveals now share one language: every screen mounts with a
240 ms rise-and-fade (the Connect footer and header ride the same
animation), the FirstRun welcome staggers in five steps, the menu unfolds
from its own corner, the close-notice backdrop fades while its sheet
rises, and the two expanders (Settings' Advanced, About's licenses, the
Help accordion) animate with the same 0fr→1fr grid-rows reveal the home
metrics already used — the accordion's text-glyph "›" is finally the same
stroke chevron every other expander uses, rotating the same way. Help's
Run-test button carries a spinner while it works and the running test
shows a quiet "Checking…" row with a turning gold arc; passed steps pop
their check in once. All of it obeys D45: decoration (entrances, pops,
the stagger) is switched off under prefers-reduced-motion while the
functional spinners keep turning. The bug: `animation-fill-mode: both`
on the entrance left a filled *identity-matrix* transform on the header
and screens, and a lingering transform — even an identity one — creates
a stacking context; the menu panel's z-index was therefore scoped inside
the header and the connect section painted over the open menu (the menu
was unclickable in every screen that had just animated in). Every one-
shot animation now fills `backwards` only, which lands on the identical
end state (the `to` keyframe equals the element's specified style)
without keeping a transform alive.

**D59 — v1.5.0: the screens say what they are, from the one copy deck —
and the stopping state finally says anything at all.**
`subtitleFor()` had the state subtitles hard-coded in English while
en.json carried an unused twin of the same table; the function now reads
the deck (like every other word on screen), the deck gains the keys it
was missing (`degradedDns`, a real `stopping` line — "Putting the
internet setting back…" — instead of the empty string the state showed
while Windows' setting was being restored), and the two hard-coded
strings in components ("Open help" in the error row, the menu panel's
aria-label) moved into it. On top of the copy hygiene, three screens got
their overdue design passes: FirstRun is a proper welcome (the wordmark
over a soft brand glow, centered copy, the autostart choice as a card
row, one full-width primary); About leads its How-it-works card with the
three-node diagram (browser → the power glyph on padlock red → the play
badge) — it used to sit under the numbered steps, below the fold where
nobody met it (VLM-verified: diagramTop 560 → 224) — and shows the
version as a pill; and the degraded state now shrinks the power button
the same step the error state always had, because its fix-pill stack was
the one state landing exactly on D48's 15 px minimum gap (now 27 px at
360×560 — the same treatment for both "needs attention" states, so they
read as one family).

**D60 — v1.6.0: the new connect screen is the whole app's face.**
Ali's design package (`younblock-ui`, uploaded to the repo root) replaced
the main screen per its own agent-instructions: `App.svelte`, `app.css`,
`tokens.css` and the seven `lib/` components were copied in as-is, the
window grew to 360×760 for the new stage (the D48 client-size fix applied
to the new height), and the old screens, components, i18n layer and
per-edition adapters were removed — recoverable at tag v0.6.0. The screen
talks to Go only through `ui_bridge.go` (the package's state bridge) and
`engine_adapter.go`, a wrapper that polls `GetState()` until the existing
engine's async Start settles (on/degraded = ready, error = failed, off =
canceled elsewhere) — no proxy logic was touched. The tray's Toggle and
connect-on-launch route through the bridge too, so tray, window and
screen share one truth. The design rules hold: no country flags, no
"Go Premium", no location chips; Sora Variable is the typeface; colors
and motion come only from `tokens.css`. After integration the package's
own six uploaded files (agent-instructions.md, design.md,
design-report.md, App.svelte, reference-vs-younblock.png and the stray
one-byte `younblock-ui` marker) were removed from the repo root — the
integrated copies under `frontend/src/` and `ui_bridge.go` are the
living ones.

**D61 — v1.6.0: the Linux editions keep their real engine behind the new
screen.**
The design package shipped two transports only — Wails bindings and an
offline browser preview — so as integrated, `younblock-linux-*` (and the
.deb / AppImage / Flatpak that embed it) would have served a screen whose
button did nothing real. Before releasing, the localhost JSON +
Server-Sent-Events transport was rebuilt inside the screen's one seam
(`backend.ts`), exactly the adapter the package's own notes anticipated
("only backend.ts needs a new adapter"). The probe is the old one: fetch
`/api/appinfo` with a 1.5 s abort, trust it only when the name is
Younblock, and fall through to the preview in a plain browser. The state
mapping mirrors `ui_bridge.go` + `engine_adapter.go` line for line: on and
degraded both count as connected (the timer keeps its first timestamp
across on↔degraded flaps), stopping is "disconnecting", error shows the
screen's one message ("Could not connect. Try again."), speeds arrive as
bits from the stats events, and — because the engine only pushes state
*changes* — the state is re-read once per second while connected so the
ping stays live, the same cadence as the bridge's RunStats. Verified live
against the Linux core: real connect, ticking timer, live ping and
speeds, disconnect, a forced PORT_BUSY failure (autoPort off) surfacing
as "Can't Connect" with a successful retry, and the preview fallback
(plus its ?fail=1 error mode) on a plain dev server — zero console
errors across all of it.
