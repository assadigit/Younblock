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

**D62 — v1.6.1: the one-click scripts are .bat, not .ps1, and they
self-heal instead of instructing.**
Ali asked for "a bat file that automatically installs and does this" —
so the developer entry points are two double-clickable batch files:
`setup-dev.bat` (install anything missing, then `wails dev`) and
`setup-build.bat` (the same checks, then `build.bat`, then open
`dist/`). PowerShell was rejected for the entry point because
double-clicking a .ps1 opens Notepad unless the Execution Policy is
loosened first — the exact kind of first-run problem the scripts exist
to remove; PowerShell is used only inside, to reload PATH after winget
installs (so there is no "open a new window and try again" step). The
scripts install Git, Go, Node.js LTS and WebView2 through winget, the
Wails CLI through `go install ...@latest` with `%USERPROFILE%\go\bin`
on PATH, gate Node >= 20 and Go >= 1.26 (the two versions the build
actually requires) with plain fix-it notes, clone or pull the private
repo, and re-run safely: everything already present is skipped. A
`.gitattributes` rule (`*.bat text eol=crlf`) keeps both files
Windows-native in every clone. They are dev-side tools — the Releases
page stays the download for people who just use the app — but from
v0.7.1 the two scripts ride along as release assets, so a fresh PC
needs nothing but a download and a double-click.

**D63 — v1.6.2: mirrors per dependency, probed at run time.**
The one-click scripts assumed the usual download sites are reachable,
which a filtered network breaks (proxy.golang.org and go.dev are the
usual casualties). Every dependency now carries mirrors, and the
scripts probe them (a 6 s request, curl.exe first with
Invoke-WebRequest as the fallback) and use the first that answers:
Go modules go through proxy.golang.org / goproxy.io / goproxy.cn (the
chain is written into GOPROXY with fall-through-on-any-error pipes and
remembered with `go env -w`, so a later plain `build.bat` benefits
too); the Go toolchain through winget, then the Aliyun and go.dev zip
mirrors; Node.js through winget, then the nodejs.org and npmmirror
zips; the npm registry through registry.npmjs.org with
registry.npmmirror.com as the automatic retry. golang.ir (an Iranian
mirror) was probed for the Go toolchain and dropped — unreachable.
The private repository itself gets three paths: a normal git clone
(GitHub's browser login); the same clone with a token pasted into
`$GH_TOKEN` at the top of setup.ps1, passed as an http extraheader so
it never lands in `.git/config`; and, when git itself cannot be
installed, a direct zipball download from the GitHub API, with the
script offering the token prompt at that point. The complexity lives
in one setup.ps1; the .bat files stay as the double-clickable face
(D62 refined) — they travel together in the younblock-setup zip,
because a .bat downloaded alone would be a trap.

**D64 — v1.6.2: the offline pack, and the Windows node_modules trick.**
For a network that blocks even the mirrors, the release carries
Younblock-offline: one zip holding the source (with the vendored Go
dependencies, D63), a pre-installed frontend/node_modules, the Go
toolchain zip, the Node.js zip and a cross-compiled Wails CLI — zero
internet needed, `setup-offline.bat` unpacks the tools into
`%LOCALAPPDATA%\YounblockTools` and the CLI into `%USERPROFILE%\go\bin`.
Three details keep it honest: node_modules is installed with
`npm ci --os=win32 --cpu=x64` on the Linux build host, so the native
binding inside it is the Windows one (vite 8 uses rolldown, not
rollup — the binding package is @rolldown/binding-win32-x64-msvc, and
the pack builder fails if it is missing); the wails skip-install
marker (frontend/package.json.md5, which stores the md5 hex twice) is
verified against package.json before packing, so the packed
node_modules really is the one wails would keep; and the Wails CLI is
cross-compiled with `-mod=mod` — `go mod vendor` only vendors what the
app imports, so the CLI (package main, never imported) must come from
the module cache, pinned to the app's own wails/v2 version.

**D65 — v1.6.3: the proportion pass — cut height where it was wasted,
not everywhere by the same percent.**
The window went from a 360×760 client area to 360×628 (−17.4%, the
middle of Ali's 15–20% ask), and the budget came from a measured
audit of the old vertical stack, not a uniform scale. Every font size
is untouched (1rem stays 16px at the design size, so nothing gets
harder to read); only spacing and the two big shapes moved. The
pillars: the header's top padding 3.1→2.375rem and its horizontal
padding 1.5→1.25rem (a uniform 20px); the play illustration −15%
(12→10.2rem wide); the status group pulled 0.5rem closer to it; the
control assembly −12% (track 14.25→12.5rem, thumb 7.75→6.875rem,
travel 5.75→4.875rem — the 82px thumb width and the 40px menu button
are deliberately NOT shrunk, touch targets first); the dotted OFF
ring 7.75→6.625rem so it hugs the shorter track; the five ON arcs
−17%; and the old worst offender, the hint sitting 4.5rem below the
button with 2.9rem of dead space beneath it, now 1.625rem below the
button with 2.3rem of bottom margin. The swipe travel is now read
from the --travel token at runtime (PowerPill measures it on mount
and resize) instead of a second hard-coded copy of the number. Two
side findings fixed on the way: MenuSheet's focus() call now passes
preventScroll (the closed sheet overflows the scaled stage, and a
plain focus made the browser scroll the whole screen up to chase the
still-sliding close button), and the About sheet's version string was
a stale hard-coded 1.5.0.

**D66 — v1.6.4: two editions, one source** — the redesigned screen
(v0.7.0+) and the classic simple UI (v0.6.0) ship side by side on every
platform. Both screens live in the frontend bundle as separate dynamic
chunks; `src/main.ts` picks one at startup from `?ui=` first, then the
build-time `VITE_UI_VARIANT` (the classic desktop editions set it,
everything else defaults to the new screen). The Go shell picks its
window size the same way through `-ldflags "-X main.windowSize=…"`
(360×628 pro, 360×760 classic), so a classic frontend never lands in a
short pro window. The classic tree is restored verbatim from the v0.6.0
tag into `src/classic/` (pure relative imports, so it moved as-is; the
adapter layer talks to the same bindings surface, which is a superset).
The web (single-executable) editions carry both screens switchable by
URL — no second binary needed there. The known 12 svelte-check errors
of the classic tree were fixed instead of tolerated (the `$state`
runes false-positives were a local `state` variable colliding with the
rune in svelte2tsx's legacy-mode view — renamed to `uiState`; the two
intentional-capture warnings carry `svelte-ignore state_referenced_locally`
with a comment saying why the capture is the point).

**D67 — v1.6.4: native Linux windows** — the 64-bit .deb and AppImage
are real GTK + webkit2gtk apps now (the "it opened in a browser"
report). Built with `-tags webkit2_41` against a user-space
Ubuntu 22.04 (jammy) dev stack assembled by
`packaging/native/fetch-stack.sh` (~200 packages resolved from the
jammy indexes, `dpkg -x` into ~/.cache, .pc files rewritten, SONAME
symlinks recreated with `ldconfig -n`). Jammy is the floor that
matters: glibc 2.35 (binary verifies GLIBC_2.34 max) — every Mint
since 21 runs it, and webkit2gtk-4.1 2.50.4 comes from jammy-updates.
The .deb edition uses the SYSTEM webkit (Depends: libwebkit2gtk-4.1-0,
apt keeps it matched — ~10 MB download). The AppImage bundles the whole
webkit NEEDED closure (BFS over `objdump -p` from the binary + the
webkit pair, minus the desktop-guaranteed set: glib/gtk/X/GL/libc),
plus bwrap + xdg-dbus-proxy for the webkit sandbox — ~75 MB, the price
of "runs on anything". WebKit's helper processes live at a compile-time
`/usr/lib/...` path with no 2.50 env override, so the bundled
libwebkit is binary-patched (at build time, in the AppDir copy only) to
look at two fixed `/tmp/.younblock-wk412*` names that AppRun points at
the extraction — the only portable way to relocate them; a fixed /tmp
name is a small, documented symlink race in exchange for working
helpers everywhere. The deb needs no such patch (helpers come with the
system webkit, matched). The window icon rides options.Linux.Icon; the
taskbar groups under StartupWMClass = the binary name. Closing the
window quits on Linux (no tray to hide into; shutdown stops the engine
and restores the proxy first). The 32-bit Linux line stays a browser
edition: old 32-bit Mint's glibc 2.31 predates the floor, and a second
(i386) webkit stack is not worth the cost. WebKit rendering itself
could not be screenshot-verified in the GL-less build sandbox (the
GPU/WebProcess path needs real EGL); everything up to that point —
loader, window geometry, helper spawning through the patched path —
was verified under Xvfb.

**D68 — v1.6.5: `-m` everywhere, or "vendor means vendor".** Every
`wails dev` / `wails build` invocation in the repo's scripts now
carries `-m` (wails' SkipModTidy flag, `cmd/wails/flags/buildcommon.go`).
Reason: wails unconditionally runs `go mod tidy` before compiling, and
tidy does not read `vendor/` — it resolves the module graph through
GOPROXY (proxy.golang.org first). So the v0.7.4 offline pack was never
offline: on the reported machine it printed a wall of `go: downloading`
lines and died with `403 Forbidden` (the proxy is unreachable there)
before the first compile. With `-m` the flow goes straight to
`go build`, which uses `vendor/` automatically (go ≥ 1.14 with a
consistent vendor/modules.txt — the pack stages source and vendor from
the same `git archive` of the tag, so consistency is structural, not
checked). The flag was added to setup-offline.bat's `wails dev`,
build.bat's four builds, build.sh's six, and setup.ps1's dev mode:
tidy adds a network round-trip and zero value when go.mod/go.sum are
complete and vendored. Second fix from the same report: the offline
setup's optional "persist PATH for future windows" step called bare
`powershell` and failed noisily on a machine whose PATH lacks the
WindowsPowerShell directory — now invoked by full path
(`%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`), output
silenced, errorlevel never checked (a convenience step must not be able
to stop a setup). Also corrected the stale "360 x 760" startup text —
the pro screen has opened 360 x 628 since v0.7.3. The immediate
workaround for a v0.7.4 pack, for the record: edit line 103 of its
setup-offline.bat from `call wails dev` to `call wails dev -m`.

**D69 — post-v1.6.5, unreleased: the feel pass — cross-fade the sets,
never re-render them; measure transitions, don't eyeball them.**
Three rules came out of this pass. (1) A tone flip must not swap DOM:
the old DotRings rebuilt its circles on every tone change (one ring ↔
five arcs), which made any CSS transition impossible by construction —
the fix mounts both sets permanently and cross-fades them through
`--dur-swap` (250 ms), with each ring as two nested groups (outer group
owns the fade opacity, inner group owns the pulse animation; CSS
animations override the opacity property they animate, so the fade and
the pulse must live on separate elements to compose). The per-set fill
replaces the tone-scoped `--ring-dot` token so the outgoing color
survives its own fade. (2) A first attempt (dip the whole family to
opacity 0, swap underneath, fade back) flickered — the sets changed at
full opacity before the dip reached them; the lesson is that a swap
hidden by a transition must be ordered AFTER the hide, and the simplest
correct ordering is "never swap at all". (3) VLM frame reviews are a
finder, not a judge: it called a measured 250 ms cross-fade a "pop"
twice (sparse dots, sampled frames) and claimed a spinner was off-center
that measures 0,0 — every visual claim this pass was settled by polling
computed styles through a live transition or by bounding-box
measurement. The same pass wired the About version to the backend
(getAppInfo: the Wails `GetAppInfo` binding / `/api/appinfo`, null in
preview → the line hides), ending the stale-hard-coded-number bug class
the same way D65 ended it the first time; gave the drag an `armed`
preview past halfway (LED previews the outcome, chevron lights and
pauses — `armed ? !on : phase === 'connected'`, because the phase
doesn't change until the release commits); and made the menu sheet trap
Tab/Shift+Tab like the modal it claims to be (aria-modal since v0.7.0,
but focus could leak out the back).

**D70 — v0.7.6: vendor the WebView2 bootstrapper; the tree must build
from a fresh clone, not from the release machine's leftovers.**
Two bugs surfaced while cutting v0.7.6, both the same species: the
release machine's uncommitted local state made builds work there while
every fresh clone failed. (1) `packaging/native/fetch-stack.sh` was
committed 100644, so `build.sh`'s direct call to it silently failed and
the native Linux packages were skipped on any fresh checkout — the
v0.7.5 artifacts only shipped complete because that machine had a
local `chmod`. The bit is in the repo now (as is
`build-offline-pack.sh`'s). (2) Every offline pack since v0.7.4 has
been missing `vendor/…/internal/webview2runtime/
MicrosoftEdgeWebview2Setup.exe`: wails embeds that file into every
Windows build (go:embed), the wails module zip ships it, but our
`*.exe` ignore rule kept it out of the repo — so `wails dev` /
`wails build` died on a fresh machine with `no matching files found`,
defeating the pack's reason to exist. It is vendored now (a negation
in .gitignore un-hides exactly that one path) and build-offline-pack.sh
refuses to zip without it. The rule this settles: if the build needs a
file, the file is committed — mode bits included; "it works on my
machine" is the bug, not the defense.

**D71 — Android port M3: GOOS=android implies the `linux` build tag; the
select wiring correction.** The audit (doc 01 §5) recorded that
`GOOS=android` selects `select_other.go` (the stub). That was wrong:
Go's build rules make the `android` tag imply `linux` (go/build:
"android (implies linux)"), so a naive `//go:build android` selector
collides with `select_linux.go` (duplicate `New` — the compile error
that exposed this). The fix: `select_linux.go` carries
`linux && !android`, `select_other.go` carries
`!windows && !linux && !android`, and the new `select_android.go`
carries `android`. Desktop builds are unchanged in behavior (on real
Linux `linux && !android` is just `linux`); the edit is constraint-only,
no logic touched. The new `internal/platform/android` adapter itself
carries no build tag at all: it is pure Go (the protect hook and the
notice callback arrive as plain funcs from the mobile facade), so its
tests run on any host — which is exactly what M3 required.

**D72 — the facade owns the WebView transport: StartWebUI/StopWebUI on
`mobile.Engine`, with a sink fanout.** Doc 04 §4 listed only the engine's
JSON methods; ADR-0005 needs the in-process webui served for the WebView,
and something must start/stop it from Kotlin. Rather than widen the
gomobile surface with the engine's internal state, the facade grew two
string-friendly methods: `StartWebUI(port int32) (string, error)` and
`StopWebUI()`. Two consequences worth recording. (1) `webui.New`
installs its SSE hub as the engine's *only* sink — on Android that would
silently disconnect the Kotlin EventSink (the shell needs events for the
notification/service state). So while the webui runs, the engine's sink
is a fanout: hub first (the page), bridge second (the shell), restored to
bridge-only after StopWebUI. The regression this prevents is covered by
a dedicated host test (the bridge must survive `webui.New`). (2) The
webui package gained two additive methods: `Events()` (exposes the hub as
an `engine.EventSink` for the fanout) and a real `Close()` — the old
listener-close left keep-alive connections half-alive, which matters for
an app whose StopWebUI must actually stop the page (caught by a failing
host test, fixed in webui, not papered over in the caller). Bridge
callbacks are wrapped with a recover: a throwing Kotlin callback drops
that one event instead of killing the emitting goroutine.

**D73 — gomobile in a vendored module: the tool directive, and
`GOFLAGS=-mod=mod` for the bind.** Modern gomobile requires
`golang.org/x/mobile` in the module graph — recorded the supported way:
`go get -tool golang.org/x/mobile/cmd/gobind` (a `tool` directive in
go.mod; D70's rule "if the build needs a file, the file is committed"
applies to the vendored tree too, so `go mod vendor` carries
x/mobile — the desktop lanes never compile it, verified by the
regression gate). But gomobile also runs `go list -m -json all` on the
repo to build its synthetic bindings module, and **vendor mode cannot
answer that** ("can't compute 'all' using the vendor directory") — the
failure is silent inside gomobile (it writes an empty go.mod, then
`go mod tidy` dies with "missing module declaration"). The bind
therefore runs with `GOFLAGS=-mod=mod`: the module graph is served from
the module cache/proxy, while every normal build of the repo stays
vendored. `build-android.sh` encodes this; the raw `gomobile bind`
invocation in doc 06 §2C carries the flag in a comment.

**D74 — the M3 shell pins and the permission skeleton's honest omissions.**
Toolchain pins (all verified by building, not by documentation):
JDK 21.0.8 (Temurin, with javac), Gradle 9.8.1 via a committed wrapper,
AGP 9.2.0, minSdk 26 / target+compileSdk 36, debug-signed sideload
builds (ADR-0012). Three findings from actually doing it: (1) AGP 9 has
Kotlin support built in — applying the separate `org.jetbrains.kotlin.android`
plugin fails the build with an instruction to remove it; the shell ships
zero AndroidX dependencies (a plain Activity + WebView; the AAR is the
only dependency). (2) `ACCESS_NETWORK_STATE` is deliberately NOT
requested: doc 07 §6 listed it as "to verify at M3"; the Go netmon reads
interfaces in-process (`net.InterfaceAddrs`) and never touches
ConnectivityManager, so requesting it would be a claim the code does not
make — the manifest carries exactly INTERNET, FOREGROUND_SERVICE,
FOREGROUND_SERVICE_CONNECTED_DEVICE and POST_NOTIFICATIONS plus the
`<queries>` block for the two default target apps. (3) The Kotlin-level
bridge call (init/health/stop) is written and compiled into the verified
APK, but this sandbox has no /dev/kvm and no device, so its *runtime*
proof is ⛔ — recorded in doc 05/09 as the standing blocker, never
claimed. The AAR itself is a build output (regenerated by
`build-android.sh`), never committed; the Gradle wrapper jar IS committed
so a fresh clone builds without a local Gradle.

**D75 — ADR-0008's gated selection resolved at M4: gVisor netstack
direct; the tun2socks engine package is unusable as a library here.**
The switch ADR-0008 pre-authorized happened on design-level evidence,
before any device measurement was possible: reading tun2socks v2.7.0's
`engine` package showed it is a CLI-shaped global singleton (config
flags, one `engine` instance for the process) whose UDP model requires
a proxy that supports UDP ASSOCIATE/forwarding — it cannot talk to
this product's CONNECT-only, loopback-only, no-auth SOCKS5 door
(ADR-0009) at all, and its lifecycle model collides with the engine's
per-cycle proxy server. `internal/tunstack` therefore assembles
gVisor netstack directly, following the same production-proven wiring
tun2socks' `core` uses (transport handlers registered before the NIC,
spoofing+promiscuous, tcp/udp forwarders, gonet for the socket side).
gvisor.dev/gvisor is pinned (v0.0.0-20260701204157-69c2d17aea96)
and, per D68, vendored (+6 MB; the desktop lanes never compile it —
the regression gate verifies). ADR-0008's device-measurable criteria
(throughput/CPU/battery within budget) still apply to the gVisor-direct
choice itself and ride with the M5 device checklist; the wire is a
channel endpoint with fdbased as the documented same-seam upgrade if
syscall count matters on device.

**D76 — the facade's TUN contract: non-blocking before os.NewFile,
and the fd crosses the bridge exactly once.** Two findings from
actually running the M4 facade tests (both were real bugs the tests
caught, not review guesses): (1) `os.NewFile` only registers a
foreign fd with the netpoller when O_NONBLOCK is already set on it
(os/file_unix.go: kindNewFile + HasNonblockFlag decide); a blocking
fd produces a file doing plain blocking read(2)s, and **Close from
another thread cannot unblock them** — `StopTun` deadlocked forever
until the facade set O_NONBLOCK (syscall.SetNonblock, the same
production pattern tun2socks uses for Android TUN fds) before
os.NewFile. The nonblock helpers are build-tagged (unix / other) so
`go build ./...` stays green on Windows, where the TUN lifecycle
honestly refuses. (2) fd ownership across the bridge is single-owner:
`StartTun` consumes the fd on EVERY return — success hands it to the
stack (whose error paths release it), every refusal (TUN_ACTIVE,
NOT_RUNNING, BAD_FD, door/stack failures) closes it — so Kotlin never
closes a detached fd and never double-closes a recycled number. The
service holds the ParcelFileDescriptor until the engine is serving
and calls `detachFd()` only at handover. The lifecycle contract
itself: one stack per engine on-phase, the door opened by StartTun is
closed by StopTun only if the same server still runs it, a second
StartTun on a live stack is refused loudly (TUN_ACTIVE — a service
bug must be visible, not silently stacked), and Shutdown always
unwinds a live stack. The engine stopping on its own does NOT
auto-stop the stack: the service owns the TUN lifecycle per doc 04 §6
and reacts to state events — a missed event surfaces as TUN_ACTIVE on
the next connect, which is the honest failure mode.

**D77 — the platform start gate: consent precedes routing, enforced in
the engine's connect run, not in the UI.** The M4 reality had the page
starting the engine directly through the webui (the same Svelte bundle
as every edition, ADR-0005), with the VPN consent checked afterwards
by MainActivity's state listener — so an engine that reached "on"
before consent existed would have been a misleading "Connected", and a
denial would have to stop an already-running engine. Doc 04 §6's
designed ordering (consent → service → TUN → engine) is restored by an
optional platform door instead of UI gymnastics: `engine.StartGater`
(internal/engine/platform.go) is an interface only the Android adapter
implements; the connect run asserts it, and when present waits at the
door BEFORE the listener, holding the phase at "connecting". Three
properties are load-bearing and test-proven: (1) the wait sits OUTSIDE
the start budget — the run's context is split (run-lifetime cancel +
post-gate timeout), because a consent dialog is not a network failure
(the gate test holds past a 200 ms StartTimeout and still reaches
"on"); (2) a cancellation (Stop, a second Start, Shutdown) releases
the waiter QUIETLY — the canceller owns the phase transition, the
abandoned goroutine never touches it; (3) desktop adapters do not
implement the interface, so the type assertion fails and their Start
path is unchanged, byte for byte — no new phase is invented (ADR-0006
holds; "connecting" is the honest page state while the platform
decides). The facade arms the gate closed at NewEngine; Kotlin opens
it (`setVpnReady(true)`) only once consent is granted AND the TUN is
established, and re-arms it closed on every unwind (onRevoke, service
death, teardown) so the next connect re-runs the whole sequence. The
adapter's zero-value gate is OPEN — the no-op form (cmd/core, host
tests) must never hold a run.

**D78 — VpnState: the honest routing report, and the reconciliation
policy that reads it.** The engine's phase is an ENGINE truth ("the
proxy path works"); the Android product's "Connected" claim is a
ROUTING truth. `mobile/vpnstate.go` answers it in one call:
{"phase", "tun", "routing", "door"} where routing requires the engine
serving (on or degraded — degraded still routes, labeled by phase) AND
a stack bound AND that stack's door being the LIVE server's door. The
third condition is the one the lifecycle suite caught: an engine
restart builds a new server, and a stack still holding the old door is
stale — reporting routing there would have been a lie; the
server-identity check makes the stale state observable (`tun:true,
routing:false`) as D76's "missed event" surface, so the shell can
reconcile (StopTun → fresh bind) instead of guessing. The shell policy
built on the report: the notification text is DERIVED from vpnState
("active" only when it reports routing; "degraded" when it routes
degraded; "connecting" otherwise) — the same JSON the host tests
verify, not a parallel Kotlin truth; MainActivity.onResume re-derives
the world (connecting + no service → restart the service path;
serving + no service → stop the engine, because without the service
nothing routes); YounblockVpnService.onDestroy stops the engine
whenever it is serving or connecting (a dead service must not leave a
lying "Connected" or an invisible zombie — a pending consent result
restarts the engine through the service's own onStartCommand);
MainActivity cleans a gate-held connect when neither UI nor service
survives. The consent UX is honest by construction (doc 04 §6):
explain-once → prepare() → system dialog → grant opens the gate; a
denial stops the held engine (it never reached "on") and shows the
denial copy. Known boundary, recorded: the facade reports what it can
KNOW — a stack bound to a live door whose underlying TUN wire died
system-side still reports tun:true until the service unwinds it; that
detection is the device checklist's job (M7's AND-028-class gates).

**M5 session note (no new decision number — enforcing D76 and the
product defaults):** building the failure-injection suite found two
real M4-era defects, both fixed at the source: (1) the facade's DOOR
refusal paths in mobile/tun.go returned without closing the fd,
violating D76's own single-owner rule ("every refusal closes it");
(2) the mobile test harness saved settings with AutoPort=false (the
struct zero value) plus the fixed default port — harmless while the
package ran alone, but under `go test ./...`'s concurrent package
execution another package's engine can hold 4525 during the gate's
delayed bind, and the honest PORT_BUSY failure exposed it; the harness
now writes the product default (AutoPort=true, matching
DefaultSettings). Root-caused and fixed, not retried away.

**D79 — the `apps` settings field (schema v2): one list behind three
doors, apply-on-next-connect.** M6 adds the per-app allow-list to
`config.Settings` (`Apps []string`, json `apps`) and bumps
`settingsVersion` to 2 — migration is the existing mechanism, nothing
new: a v1 file simply lacks the field, and `normalizeApps` fills the
built-in defaults (Chrome + YouTube, ADR-0003), so Save persists v2
with the list (apps_test.go proves the v1 file's custom fields survive
verbatim). Validation: trim, Android-package shape (dot-separated
segments, each starting with a letter; case preserved — package names
are case-sensitive), dedupe first-seen, cap 32 (every entry becomes one
addAllowedApplication call at establish). THE LOAD-BEARING RULE: an
empty or all-invalid list RESTORES THE DEFAULTS — a VpnService.Builder
with zero allowed applications routes EVERY app, the exact opposite of
the product scope, so the route-everything state must never persist in
the file; the Kotlin side fails closed independently when none of the
listed packages is installed (zero addAllowedApplication successes →
refuse to establish, stop, honest log). One list, three doors, one
truth: settings.json, the jsonapi GetSettings/SaveSettings surface
(the WebView's Apps-to-unblock row edits through POST /api/settings),
and `mobile.RoutedApps()` (`{"apps":[…]}` from the engine's locked
Settings() copy accessor) which the Kotlin service reads. The semantics
the copy can lean on: the list is read FRESH at TUN establish — a
change saved while the TUN is live applies on the NEXT connect. That
is a choice, not an accident: the allow-list is baked into the
VpnService builder, and rebuilding an established TUN under live
traffic is riskier than the one-cycle delay; the UI says exactly
"Changes apply the next time you connect", and reading at establish
makes the sentence true with zero extra plumbing (no re-establish
machinery, no settings-change listener, no live traffic surgery). The
Kotlin DEFAULT_APPS constant is deleted — a second copy of the default
list could drift from config.DefaultApps, and the honest failure mode
(an unreadable list refuses to route) is better than a silent fallback
to a possibly-stale copy. Desktop unchanged: the field round-trips
through the classic screen's spread-based saves (JSON.parse keeps
unknown fields at runtime, the spread copies them; even a path that
dropped them would hit Normalize's defaults-restore), and desktop
platforms never read it.

**D80 — the M6 WebView UX: capability-gated honesty surfaces, the
saved response as the authority.** The pro screen's menu sheet gains
the Android-only surfaces — the Apps-to-unblock row, the VPN-slot/
key-icon About lines, the doc 07 §5 privacy section, the diagnostics
view — every one gated on what the HOST actually reports
(`AppInfo.platform === "android"`, `capabilities.perAppRouting`), so
the desktop sheet renders its exact prior sections byte for byte and
a preview build (no host) shows no Android-only sections at all (the
M5 discipline extended to UI: no desktop behavior change, no invented
host data). The `Backend` interface grew `getSettings`/`saveSettings`/
`getDiagnosticReport` with real implementations on all three transports
(Wails bindings — app.go already exposed them; the webui's
/api/settings + /api/report; the preview returns null/throws rather
than fabricating). Design rules that keep the row honest: friendly
labels exist ONLY for the two known packages — anything else shows its
package name, never a guessed app identity; the add flow validates
NOTHING client-side beyond non-empty — the engine's Normalize response
IS the authority, and a refused entry surfaces as inline feedback
("not a valid Android package name") instead of a silently dropped
row (the UI duplicates no validation rules that could drift from the
Go side); the remove button disables on the last entry (Normalize
would restore the default anyway, but the UI should not offer a
no-op). The About/How-it-works/Help Android variants replace the
desktop flow sentences with the VPN-slot truth ("uses Android's VPN
slot — the key icon you'll see while it runs — as a local traffic
router for the apps you choose; still not a VPN: no IP hiding, no
added encryption"), per ADR-0007/doc 07 §1 (no overstated VPN/privacy
language) — the disclosure IS the product behavior. The privacy
section is doc 07 §5 rendered as user copy (every sentence verified
against the implementation: no analytics code, no Younblock-run
endpoint, DNS to the user-configured DoH providers, local logs). The
diagnostics view renders the EXISTING redacted GetDiagnosticReport in
a labeled scrollable region with a copy button — no new report content
invented for the UI. Accessibility: the sheet's generic focus-trap
query picks up every new control automatically; aria-labels on
icon-only buttons; 2.75 rem touch targets; rem typography for system
font scaling; role="alert" errors. The live halves (TalkBack, real
rotation, real font-scale) ride with the device checklist — never
claimed host-side.

**M6 session note (recovery + one honest build iteration):** the
session opened after the third recorded full sandbox wipe (repo,
~/sdk, worklog all gone) — re-cloned at 4083e76, toolchain reinstalled
per doc 06's pins, worklog rebuilt from the prompt + repo records,
baseline re-verified green before any M6 code. The first clean
assembleDebug failed at compileDebugKotlin with two honest errors
(Kotlin needs `engine.routedApps()` call syntax — the gomobile binding
is a function, not a Kotlin property — and the org.json.JSONArray
import); both printed, fixed, re-run CLEAN. The compile-matrix evidence
was also sharpened this session: `go build ./...` on android/arm|386|
amd64 with CGO_ENABLED=0 fails ONLY at the three main-package links
("requires external (cgo) linking" — exactly what doc 06 §A's comment
records), so the honest matrix form is `go build` over the 18 non-main
packages per ABI (4/4 green; arm64 links the full tree even without
cgo).

**D81 — the platform refusal door: shell→engine typed errors through
the start gate (NO_TARGET_APPS), the phase machine stays the single
truth.** M6 deferred this question (a "D78-class design decision");
M7 settles it. The shell discovers platform-level impossibilities the
engine cannot know — PackageManager found none of the allow-list's
packages installed — and the honest surface is the ENGINE's error
phase with a typed code, not a parallel Kotlin error UI and not a
silent unwind to "off" (which is what M6 shipped: fail-closed but
unexplained). Mechanism, built on D77's door: `engine.PlatformRefusalError{Code}`
is handed to a gate-held connect run and routed through the ORDINARY
failure path — failStart sets phase "error" with the code, the state
is emitted, the page shows "Can't Connect" plus the mapped reason
("None of the apps on your list is installed. Install one, or edit
the list in the menu."). Three load-bearing properties, all tested:
(1) DELIVERY IS ATOMIC — the waiting run registers its own buffered
channel and `refuse()` hands the error straight to it, so the shell's
immediate re-arm (`stopSelfHonest` → `setVpnReady(false)`, the exact
Kotlin sequence) can never claw back a delivered refusal: a
woken-but-unscheduled waiter still sees it. A refusal nobody has
consumed yet LATCHES for a run that has not reached the gate (the
spawn race), and both open and re-arm clear the latch — an open gate
refuses nothing (NOT_HELD: the shell's honest-stop owns a serving
engine), and a re-arm is a clean slate the shell re-discovers on.
(2) THE VOCABULARY IS WHITELISTED TWICE — the facade accepts only
NO_TARGET_APPS today (BAD_CODE otherwise) and mapErrorCode re-checks
it, so no arbitrary error string can cross the bridge and invent a UI
code. (3) CANCELLATION STILL WINS — the connect run checks its ctx
before honoring a refusal, so Stop/second-Start/Shutdown keep D77's
quiet-exit ownership even when a refusal races them. Desktop is
unaffected: no desktop platform implements StartGater, so the refusal
branch is unreachable there (byte-identical Start path, verified by
the full regression gate). The unreadable-allow-list path (a bridge
bug, not a user state) deliberately does NOT use the door — it stays
log + honest-stop, because claiming NO_TARGET_APPS there would be a
lie about a state the PackageManager never reported.

**D82 — run generations: a cancelled Start can never be resurrected by
its own goroutine.** Building D81's tests exposed a real M5-era hole
in the quiet-cancellation guarantee: `connectSequence` created its
run-lifetime context and registered it at ENTRY — so a connect
goroutine that was spawned but not yet scheduled when its own
Stop/second-Start landed would register a FRESH context nothing could
cancel (and reset `runAborted`), then proceed as a zombie: holding at
the gate, or completing to "on" AFTER the canceller had already set
"off" — the exact phase-resurrection D77 exists to prevent. The proof
it was real, not theoretical: `TestEngineStartIsIdempotent` had been
passing THROUGH the bug for the whole project — its immediate
Start→Start expects "on", the second Start is the documented cancel
(file 01 §10.5), and the only reason the test ever saw "on" was the
unregistered zombie completing after its cancellation. The fix: every
cancel bumps a run generation (`runGen`), Start captures the
generation under the same lock that sets "connecting", and the
goroutine's stale-check and context registration are ONE critical
section — a superseded run exits at its first instruction without
registering anything or touching any phase; a fresh run registers
atomically so every future cancel reaches it through its context.
Desktop behavior is unchanged in practice: the registered-run cancel
path (the overwhelmingly common case — the window is the time between
`go` and the goroutine's first lock) always behaved this way; D82
made the microscopic unscheduled case match it. The test was fixed to
its true intent (wait for "on", then Start again — idempotency while
ON), and the gate suite carries the cancel-semantics coverage.
Evidence: the refusal/cancellation suites in
internal/engine/gate_test.go, the -race lane green through the new
machinery, and the full 17/17 gate twice.

**M7 session note (the first no-wipe session, and one honest
regression iteration):** the sandbox survived a session boundary for
the first time in the port's record — the repo at 57adf1a, the
~/sdk toolchain and the worklog all intact, verified with commands
before work began. The -race lane was added as a standing hardening
gate (gcc present; seven key packages, zero races). One regression
was caught by the gate rather than hidden: D82's honest cancel broke
TestEngineStartIsIdempotent, the failure was root-caused to the test
codifying the zombie (not to the fix), the test was rewritten to its
documented intent, and the suite went green twice — the M5-era
"root-caused and fixed, not retried away" discipline, applied to our
own new code.

**D83 — Ali's two product rulings (2026-10-17, pre-M8): no allow-list
default, and .ir never touches the app.** Two answers arrived for the
two standing product questions (doc 09): (1) "There is no allow-list
defaults" — the apps allow-list starts EMPTY; no package is routed
until the user adds it. (2) "keep every domain with .ir blocked — when
opening any domain with .ir, this app must not be used and its routes
must not work" — every host under the .ir TLD stays outside Younblock's
machinery, in every mode and both editions.

Consequences, implemented as one change set (M8-a):
- THE EMPTY LIST IS NOW A FIRST-CLASS STATE. DefaultApps() returns the
  empty list; Normalize KEEPS an empty or all-invalid list (the M6
  "restore the defaults" rule is gone); mobile.RoutedApps' failure
  direction INVERTED — it fails toward the EMPTY list, because the M7
  refusal door (D81) is what makes empty safe. The Kotlin establish
  now routes the empty list through failStart("NO_TARGET_APPS") too —
  the old path treated empty as a bridge-read bug and stopped
  silently; with no defaults, empty is the NORMAL fresh state, and a
  silent stop would leave a gate-held run hanging. The UI drops the
  last-entry guard (the list may go empty), shows an empty-state line,
  and the NO_TARGET_APPS message honestly covers both "empty" and
  "none installed". Saved files keep their lists — the ruling changes
  the default, never user data.
- THE .ir RULE LIVES AT THE ONE DECISION POINT BOTH DOORS SHARE:
  Server.routedHost = the mode/list rule (D46) MINUS the
  never-processed TLDs (internal/config/direct.go, UnderDirectTLD).
  The rule wins over ModeAll AND over the user's own domain list: a
  .ir host is never fragmented, never http→https-redirected —
  resolved and dialed direct, byte-transparent. The PAC carries the
  same rule as its FIRST line (youtube and all modes), so a desktop
  browser sends .ir DIRECT and the app never even sees it — this
  preserves desktop youtube-mode behavior (the PAC fallthrough already
  kept .ir out) and EXTENDS the guarantee to ModeAll, which previously
  sent .ir through the fragmenting path: a real, ruled behavior change
  on the desktop, disclosed here and in the changelog. The SOCKS5 door
  (the Android TUN path) gets the same decision — on Android the
  routed apps' .ir traffic still transits the TUN (per-app routing is
  all-or-nothing per app), but exits as a plain direct dial: no
  fragmentation, no redirect, no coverage claim.
- ONE HONEST LIMIT, STATED (doc 07 §5): on Android, .ir lookups still
  resolve through the configured DoH providers — the process has no
  other resolver (no /etc/resolv.conf on Android; ConnectivityManager
  would need ACCESS_NETWORK_STATE, deliberately not requested, D74).
  The in-app Privacy copy and doc 07 say so plainly.
- THE <queries> BLOCK HAD TO BROADEN (a real latent bug the ruling
  exposed): it listed only com.android.chrome and
  com.google.android.youtube — the old defaults — so any USER-ADDED
  package would be invisible to PackageManager (Android 11+ package
  filtering) and addAllowedApplication would refuse it as "not
  installed" even when installed. The block is now the launcher-intent
  query (MAIN + LAUNCHER), which makes every launchable app visible
  without QUERY_ALL_PACKAGES; non-launchable packages cannot be
  verified and are refused as not-installed — the honest limit, in
  doc 07 §6 and the manifest comment.

Evidence: internal/config/direct_test.go (the truth table),
apps_test.go's inverted expectations, behavior_test.go's
TestDirectTLDStaysUnfragmentedInAllMode/EvenWhenListed,
socks5_test.go's TestSocks5DirectTLDStaysUnfragmented (the door
proof), pac_test.go's TestPACDirectTLDGuard; the full gates — 17/17
twice, -race 7/7, svelte-check 0/0, vite clean + webui dist synced,
matrix 4/4, gomobile bind (AAR 25,160,550 B; javap shows the nine
bindings), clean assembleDebug (APK 35,567,561 B, sha256
ccf46a2e9ca53346e5454276e1c51a003eb897b976ce0ff6144d58c6bfd37b64;
badging minSdk 26/target 36, 3 ABIs; the dumped manifest carries the
launcher-intent queries and exactly the doc 07 §6 permission
inventory; apksigner OK, debug cert).

**D84 — Ali's keystore/distribution ruling (2026-10-17, at M8): the
debug-APK-only test circle.** The last open product question — the
keystore handling (doc 07 §8), blocking for M8's signed release APK —
is answered verbatim: "yes keep it debug version. we're not releasing
yet. but release in github for me to download and debug. So i guess;
Alternatives: EC key, or no signed release / debug-APK-only test
circle — tell the team and M8 will follow that decision."

Consequences, implemented at M8:
- NO SIGNED RELEASE APK AT THIS STAGE. The alternatives stay
  documented for whenever a public release happens (Ali generates his
  own local keystore per doc 07 §8's README steps, or an EC key);
  M8 follows the debug-only one. Doc 07 §8 now records the test
  circle's distribution model; the public-v1 release notes (doc 10)
  remain a DRAFT — "we're not releasing yet" is recorded, not
  contradicted.
- THE DEBUG APK IS THE DISTRIBUTED ARTIFACT, VIA GITHUB RELEASES on
  the private repo (prerelease-flagged tag `v1.6.6-android-debug.1`),
  so Ali downloads it with his own GitHub credentials and installs it
  on a real device — the first actual path toward closing the shared
  device checklist. The sandbox's own install/launch smoke halves
  remain ⛔ (no /dev/kvm, no device, re-verified this session); the
  release hands those gates to Ali's device together with the test
  circle in doc 10 §"Device test circle" — his observations become the
  recorded evidence in the next session.
- UPDATES INSTALL OVER THE PREVIOUS only while versionCode increases
  across successive debug builds (same debug certificate): every
  future debug build bumps `versionCode` (the rule is carried in doc
  06's M8 record and the handoff). This first artifact keeps
  versionCode 1 / versionName 1.6.6 (the port inherited the desktop
  version line).
- RULING 1 CONFIRMED AS SHIPPED, verbatim: "Ruling 1 — no allow-list
  default: Allow-list starts empty; nothing is routed until apps are
  added. youtube is allow list." D83 already implements exactly this;
  "youtube is allow list" reads as the canonical use case — the user
  adds the YouTube app (the app the product exists for) to the list,
  and the YouTube domains remain the domain-level unblock target. No
  code change; D83 stands.

Evidence: the M8 lane record (doc 05 §M8, doc 06) — the on-disk
artifact re-verified as the M8-a clean-build output before any
distribution (sha256 match ccf46a2e…7b64; apksigner v2-scheme OK,
debug certificate digest c76bb7c6…8551d recorded; badging
io.github.assadigit.younblock 1.6.6 versionCode 1, minSdk 26 /
targetSdk 36, native-code arm64-v8a/armeabi-v7a/x86_64; the dumped
manifest carries exactly doc 07 §6's four permissions and the
launcher-intent <queries>; javap shows the full binding set incl.
failStart(String) and routedApps()); the dependency closure re-run
(`go list -deps ./mobile` — the same audited set as M7); the
platform-requirements recheck against developer.android.com primary
sources; the GitHub release itself, verified through the API after
creation (tag, prerelease flag, asset size and digest match).

**D85 — the pinned debug keystore + the first device observation's
diagnosis (2026-10-17, the device-test feedback loop's first
session).** Ali's first observation from the test circle arrived:
"Android didn't let me install it." Diagnosis before any fix, all with
evidence:

- THE ARTIFACT IS EXONERATED END-TO-END. The bytes GitHub serves were
  downloaded this session and hash to exactly the recorded sha256
  (ccf46a2e…7b64); apksigner v2-verifies the served file with the
  recorded debug certificate (c76bb7c6…8551d); badging shows
  minSdk 26 / target 36 / versionCode 1 / the four permissions; the
  binary manifest carries debuggable=true (normal for a debug build,
  does not block sideloading) and NO testOnly attribute (the flag that
  would make the package uninstallable); the zip central directory is
  intact (20 entries, testzip clean — 4 dex + 3 native libs + manifest/
  resources, exactly the gomobile app's shape).
- THE DECISIVE CLUE: the release asset's download_count is 0. Only
  completed downloads count, so the most probable story is the
  download never finished on his phone — GitHub release assets are
  served from release-assets.githubusercontent.com, which is
  frequently slow or unreliable from Iranian networks — and Android
  refused a truncated/HTML file. The remaining branches are the two
  gates doc 10 already warned about (the install-unknown-apps gate;
  Play Protect's "unsafe app blocked") or a device below Android 8.0.
  The ordered troubleshooting path (size check ≈ 35,567,561 B first,
  PC + USB as the reliable fallback route) is now ON THE LIVE RELEASE
  BODY (PATCHed through the API, HTTP 200) and in doc 10 §1 — the
  help sits exactly where the retry happens, on his phone.
- THE LATENT HOST-SIDE HOLE THE OBSERVATION EXPOSED: doc 10 promised
  "successive debug builds install over this one (same certificate),"
  but that promise rested on ~/.android/debug.keystore — a
  machine-local file REGENERATED on every sandbox wipe (three wipes
  already in this project's record). A post-wipe rebuild would have
  signed with a new debug key and silently broken in-place updates.
  FIX: the exact keystore that signed v1.6.6-android-debug.1 (still
  present; certificate digest verified identical) is now vendored at
  android/app/debug.keystore and wired as the debug signingConfig in
  android/app/build.gradle.kts. Verification: clean assembleDebug →
  apksigner shows the SAME certificate digest c76bb7c6…8551d, and the
  rebuild is byte-identical to the published release (sha256
  ccf46a2e…7b64 — deterministic packaging). The update promise is now
  wipe-proof.
- A DEBUG KEYSTORE IS NOT A SECRET: the store password is the
  well-known "android" and the certificate is the standard Android
  debug identity; committing it carries no credential risk. The
  no-committed-keys rule (doc 09) covers RELEASE keystores, which
  remain out of the repo per D84/doc 07 §8 (Ali's local keystore or
  an EC key when the public release happens).
- No debug.2 was cut this session: nothing about the ARTIFACT changed
  (it is byte-reproducible), so a re-release would churn tags without
  fixing a device-side or network-side cause. The next debug build
  rides on the next observation batch that changes code.

**D86 — Ali's YouTube-default ruling + the first functional device
observation's root cause (2026-10-17, the device-test feedback loop's
second session, M8-c).** Ali's message: "mAKE the youtube default
added list. the first version didnt work, whether in youtube app or
youtube in chrome." Two inputs: a product ruling and the first
functional observation from an installed build (the release asset's
download_count reached 2 — the install completed this time).

The observation was root-caused BEFORE the fix, by reading the whole
routing chain end-to-end (tunstack DNS interception, the SOCKS5 door,
routedHost, the Kotlin establish path — all sound):

- THE ROOT CAUSE IS THE PRODUCT SHELL, NOT THE ENGINE. debug.1's list
  was born EMPTY (D83), and the ONLY entry path was a text field that
  demanded exact Android package names ("Add apps by their Android
  package name"). Typing "youtube" — what any normal user types — was
  REFUSED by package-shape validation (a package name needs
  dot-separated segments), so the product's own namesake app could not
  realistically be added, the list stayed empty, every connect took the
  NO_TARGET_APPS refusal, and nothing ever routed — in the YouTube app
  or anywhere else. Exactly "didn't work, whether in youtube app or
  youtube in chrome" (Chrome was never on the list either, so
  youtube.com in Chrome was untouched by design).
- THE RULING, IMPLEMENTED: DefaultApps() = ["com.google.android.youtube"]
  (D83's empty default superseded). A SAVED EMPTY LIST STILL MEANS
  EMPTY — user agency is preserved (removing the last entry must stick;
  the M7 NO_TARGET_APPS door owns the connect, never a
  restore-to-default). Fresh installs route the YouTube app out of the
  box.
- THE v3 SCHEMA MIGRATION (settingsVersion 2→3, the existing version
  mechanism): a v1/v2 file whose apps list is absent or empty carries
  the SHIPPED default of its era, not a user choice — debug.1 was born
  with apps:[] and its entry path refused friendly names, so "empty" in
  a v2 file almost always means "never successfully configured" (the
  exact state on Ali's device). Load migrates it to the D86 default; a
  v3 file's empty list (the user's own, under this build) is kept; any
  v1/v2 list with entries is kept verbatim. THE UPDATE INSTALLS OVER
  debug.1 (versionCode 2, the pinned D85 certificate) AND ITS SAVED
  SETTINGS SURVIVE — the migration is what makes the ruling reach the
  one device that reported the problem.
- THE ENTRY-PATH FIX: the Apps-to-unblock row gains one-tap quick-add
  chips for the known apps (+ YouTube, + Chrome — rendered when not
  already listed) and the type-in field accepts friendly names
  ("youtube", "chrome", case-insensitive) resolved to their packages
  client-side; the engine's saved response stays the validation
  authority (no rule duplication). Other apps still need the package
  name.
- CHROME IS NOT IN THE DEFAULT (the open question carried): the ruling
  names YouTube only, so the default is exactly [youtube]. youtube.com
  in Chrome needs Chrome on the list — the + Chrome chip makes that one
  tap, and the release notes / row 4 of the test circle teach it. If
  Ali wants Chrome pre-added too, that is a one-line change + a
  versionCode bump away.
- The functional chain stays as audited: UDP:53 to ANY destination is
  answered by the virtual DNS (the YouTube app's hardcoded 8.8.8.8
  queries included); fake-IP destinations reverse to domains at the
  door (fragmentation decisions identical to the browser path); QUIC
  fallback remains the designed device gate (row 9).

Evidence: config tests (the default, the copy-ness, the v1/v2→v3
migration matrix incl. THE debug.1-empty-file case, the v3-agency case,
custom-list survival); mobile tests (the default through RoutedApps,
the debug.1-file migration through a real engine, the emptied-list
refusal state); vet clean; 17/17 ×2; -race green on config/engine/
mobile; svelte-check 0/0; vite build + dist sync (TestWebUILifecycle
re-run green on the new bundle); compile matrix 4/4 ABIs (18 non-main
packages each; arm64 links the full tree — the exact historical form,
re-derived this session); gomobile bind → younblock.aar 25,162,951 B
(javap: the full binding set); clean assembleDebug → app-debug.apk
35,716,265 B, sha256 9cca38c9…811b, versionCode 2, the pinned D85
certificate (c76bb7c6…8551d), badging minSdk 26/target 36/3 ABIs, the
four-permission manifest, no testOnly; the D86 copy verified embedded
in the AAR's native library. Released as v1.6.6-android-debug.2 through
the same lane.

**D87 — Ali's browsers ruling: the default list extended to Chrome,
Firefox, Opera and Brave (2026-10-17, the device-test feedback loop's
third session, M8-d).** Ali's message: "Add chrome to the default
addlist as well, also firefox, opera, brave browser." This closes the
open question D86 carried ("whether Chrome should join the default
list — currently one tap away") with the owner's explicit answer, and
extends it beyond Chrome to the other three mainstream browsers.

- THE RULING, IMPLEMENTED: DefaultApps() = [com.google.android.youtube,
  com.android.chrome, org.mozilla.firefox, com.opera.browser,
  com.brave.browser] — the stable Play Store package names of the four
  browsers' mainline builds (Beta/Nightly/GX/Mini variants are
  deliberate non-choices: one entry per browser, the channel a normal
  user has). A SAVED EMPTY LIST STILL MEANS EMPTY — the agency rule
  D81/D86 kept verbatim.
- MISSING APPS CANNOT BREAK ESTABLISH (audited before shipping): the
  Kotlin establish loop already catches NameNotFoundException per
  entry and skips it, refusing only when ZERO entries resolve — so the
  five-app default is safe on a device with only some of the apps
  installed (Firefox/Opera/Brave are rarely preinstalled; they simply
  skip, and the chip row is how they get added later if installed).
  This was THE risk of a multi-app default and it was already closed
  by the M6 establish design; D87 records the audit.
- THE v4 SCHEMA MIGRATION (settingsVersion 3→4), same one-time
  mechanism as v3, with the era logic made precise: (a) a v1/v2 file
  whose apps are absent or empty still carries the D83 born-empty
  shipped default and fills the D87 default directly; (b) a v3 file
  holding EXACTLY the D86 default [com.google.android.youtube] was
  written by the debug.2 build itself (fresh default or the v2-empty
  migration), never curated, and upgrades to the D87 default — the
  exact requested outcome on the one device carrying debug.2; (c) a v3
  empty list (user agency, D86's ruling) and every custom list —
  including v1/v2 lists with entries, and a v2 file that happens to
  equal [youtube] because its owner hand-typed the full package name
  under debug.1's refusing entry path — survive verbatim. The
  distinction is deliberate: under v3 the build itself wrote
  [youtube]; under v2 only a human hand could.
- CUSTOM LISTS KEEP THE CHIPS: the quick-add row now covers all five
  default apps (+ YouTube / + Chrome / + Firefox / + Opera / + Brave),
  so a curated list (e.g. a debug.2 device that tapped + Chrome during
  row 4's test) regains any default app in one tap; the friendly-name
  aliases extend to "firefox", "opera", "brave" (and "brave browser").
  The engine's saved response stays the validation authority.
- THE HONEST COPY, EVERYWHERE: menu intro ("the YouTube app and the
  Chrome, Firefox, Opera and Brave browsers come already added"), the
  empty state, the note, How-it-works line 1, the NO_TARGET_APPS
  message, README, doc 07 §1, doc 10 — every claim updated to the
  five-app truth in the same breath as the code (the D87 lane rule).

Evidence: config tests (the five-app default + copy-ness + the
normalize rules; the migration matrix: v1→v4, THE debug.1-empty-file
case, THE debug.2-old-default case, v3-empty agency, v3-custom
survival, v2-custom survival, the v2-hand-typed-[youtube] case,
round-trip, JSON shape); mobile tests (the default through RoutedApps,
the debug.1-file and debug.2-file migrations through a real engine,
v3-custom survival, emptied-list refusal, restart survival); vet
clean; 17/17 ×2; -race green on config/engine/mobile; svelte-check
0/0; vite build + dist sync + the webui suite green on the new bundle;
compile matrix 4/4 ABIs (18 non-main packages each; arm64 links the
full tree); gomobile bind → younblock.aar 25,174,619 B (javap: the
full binding set; the D87 WebView copy verified embedded in
libgojni.so); clean assembleDebug → app-debug.apk 35,816,233 B, sha256
81515f0d…d6a1, versionCode 3 (the D84 rule), the pinned D85
certificate (c76bb7c6…8551d — installs over debug.1/debug.2 in place),
badging minSdk 26/target 36/3 ABIs, the four-permission manifest, no
testOnly. Released as v1.6.6-android-debug.3 through the same lane.

**D88 — Ali's script ruling: the local release-keystore ceremony is
a shipped .bat, not manual keytool steps (2026-10-17, the
device-test feedback loop's fourth session, M8-e).** Ali's message:
"Can you fix the serial number and etc issue as well? like giving me
a bat file that auto creates it's password locally on my system and
backs up that password as an md file. if you insist it must be done
locally. at least you can create the script." The "serial number and
etc" is keytool's interactive ceremony (the two password prompts plus
the distinguished-name questions) that D84's deferral left as manual
"README steps" — which, on inspection, did not even exist in the
README yet (doc 07 §8 pointed at them; this session closed that hole
too).

- THE SECURITY BOUNDARY STANDS: a release signing-key password is
  generated WHERE IT IS USED. It never transits the chat, the agent,
  the repo, or any network. That was the standing recommendation since
  M8-a ("he generates his own locally") — Ali's message accepts it and
  asks for the automation ("at least you can create the script").
- THE SCRIPT (make-release-keystore.bat, repo root + an asset on the
  debug.3 release; offline, pure ASCII, CRLF, 253 lines): one
  double-click on his Windows machine — (1) keytool located (PATH,
  JAVA_HOME, the Android Studio JBR, the common JDK directories);
  (2) a 32-character password generated by Windows' own CSPRNG
  (RNGCryptoServiceProvider through an exactly-64-character alphabet
  — each random byte maps to one character with ZERO modulo bias,
  ~192 bits; the alphabet contains no cmd-special characters and no
  confusable 0/1, so it survives batch echo/re-type); (3) the
  keystore created with EVERY keytool prompt pre-filled (-dname,
  RSA 4096, PKCS12, 10950 days = 30 years — comfortably past the
  2033 signing-cert validity floor); (4) the result VERIFIED by
  reading it back with the same password (wrong-password keytool
  exits 1 — proven on the real keytool; that semantic is the
  verification step); (5) younblock-keystore-backup_<date>.md written
  next to it recording the password, the alias, the certificate
  SERIAL NUMBER, the SHA-256 fingerprint, the validity window, the
  machine/user, and the full keytool -list -v record — the "serial
  number and etc" captured automatically instead of fished out by
  hand.
- THE GUARDRAILS BUILT IN: the script REFUSES to overwrite an
  existing release keystore (a second one would be a new identity —
  uninstall-before-update), warns when run inside a git repo, keeps
  everything in the script's own folder, uses no network, and shows
  the password on screen AND in the .md backup before exit. The
  .gitignore now carries both generated patterns
  (younblock-release.keystore, younblock-keystore-backup_*.md) — the
  no-committed-release-keys rule (D84) made mechanical; the debug
  test-circle keystore stays vendored per D85's deliberate
  exception.
- THE HONEST LIMIT: the build host is Linux — the .bat wrapper itself
  is ⛔ there (no Windows, no pwsh). What was verified on the host
  with the real keytool 21.0.12.1 (same version family as the pinned
  Temurin JDK): the full genkeypair flag set runs prompt-free (exit
  0, valid to Oct 2056); -list -v's exact line shapes match the
  script's findstr parsing ("Serial number:", "SHA256:", "Valid
  from:", "Owner:"); a wrong storepass exits 1. The password
  algorithm was simulated end-to-end (alphabet = exactly 64 unique
  characters; sample outputs 32 chars, cmd-safe). The file: pure
  ASCII, 253/253 CRLF lines, sha256
  82e6dded40a8eb415127c6d46b3cb4acb0633ab2cd13ac8b809a8ae8c7c74090,
  byte-identical on every delivery surface. The script's own
  read-back verification is what carries the proof onto Ali's
  machine on first run.
- THE DELIVERY LANE: no new APK, no new tag — the debug.3 release
  gains the script as a second asset (the D84 rule: identical APK
  bytes are never re-released), the repo carries it at the root
  beside setup-build.bat, and the README's Android section now
  documents the actual signed-lane steps (the "README backup steps"
  that doc 07 §8 had referenced into thin air).

Evidence: the host-lane verification list above (real keytool runs,
parsed-shape checks, the alphabet proof, the byte checks); the
served release asset re-downloaded through the API and hashed to the
same sha256; the preview surface serving the same bytes; zero
Go/Svelte/app code touched — the M8-d gates at the unchanged clean
tree 2a8b678 stand as the baseline of record (the ~/sdk toolchain
wipe between M8-d and this session is recorded in doc 06's M8-e
entry; the script-only session needed none of it beyond keytool,
which the system JRE provides).

**D89 — Ali's release-flag ruling: the android circle ships as
regular releases in the private repo (2026-10-17, the device-test
feedback loop's fifth session, M8-g).** Ali's message: "Do not
prelease , just release, the repo is private. Please finish all of
your specs first." The prerelease flag on the android debug releases
carried no public meaning — the repo is private, every release is
collaborator-only — and it kept the repo's Latest pointer off the
current test-circle build. THE CHANGE: all three android releases
(v1.6.6-android-debug.1/.2/.3) were switched to regular releases
through the GitHub API and verified after (tags f61bfae/096e8c0/
2a8b678 unchanged; every asset and sha256 digest unchanged;
/releases/latest → debug.3, set explicitly — a plain flag flip does
not move the Latest marker by itself); the debug.3 body gained
exactly one D89 note paragraph so the non-prerelease flag cannot be
misread as a stability claim — the honest "debug-signed test build,
not a public release" copy stands, and the APK bytes are untouched
(D84's no-identical-re-release rule respected: a flag flip is
metadata, not a new build). THE SPEC AUDIT Ali asked for ran before
the flip: Phase 2 is host-complete end to end (doc 05: M0–M8 all ✅;
every remaining ⛔ is an on-device gate that needs Ali's physical
device), the docs were current through D88, and the M8-f verification
round had re-verified every delivery surface — nothing host-side was
left unfinished, and the docs were then brought current with this
ruling across docs 06/07/09/10, README, PROGRESS, CHANGELOG and the
session prompt. If the repo ever goes public, the flag question
returns with real semantics (update channels, the public Latest
badge); until then the debug circle stays honestly labeled inside a
private repo.

Evidence: the GitHub API state before (three android releases,
prerelease=true each) and after (prerelease=false; /releases/latest
→ v1.6.6-android-debug.3); the debug.3 body re-fetched — the D89
note present exactly once, every other byte identical to the pre-edit
body (8,579 → 9,017 chars); zero Go/Svelte/app code touched — the
M8-d gates at the unchanged clean tree stand as the baseline of
record (~/sdk still wiped; no lane can run without the reinstall,
and none was needed for a metadata change).
