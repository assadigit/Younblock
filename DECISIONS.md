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

**D90 — the keystore ceremony survives PowerShell-less PATH
machines (2026-10-17, the device-test feedback loop's sixth session,
M8-h).** Ali's first field report from the helper arrived — the
ceremony stopped at password generation: "'powershell' is not
recognized as an internal or external command" — his PATH variable
does not include PowerShell even though Windows ships it (the same
environment class the desktop setup scripts already fixed at v0.7.5:
"the setup no longer depends on PowerShell being on PATH"). THE
FIX: make-release-keystore.bat now locates powershell.exe by its
full standard install path first
(%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe —
setup-offline.bat's shipped pattern), then through where.exe; and if
PowerShell cannot be found at all, a pure-cmd fallback builds the
password from six RANDOM draws (digits and dashes, about 90 bits)
inside the script itself — the step can never dead-end again, and
both the .md backup and the DONE summary record which generator
produced the password. His accompanying question ("can't I use an
online keyring generator instead?") is ruled NO: an online
generator would put the app's signing identity — the private key
and its password — through a third party's server; D88's boundary
stands (signing secrets are generated where they are used, never
transiting any website, chat, agent or repo). A one-line manual
keytool alternative is documented in the release notes for machines
without PowerShell (keytool masks the password itself). DELIVERY:
the fixed script ships as v1.6.6-android-debug.4 — a REGULAR
release (D89) carrying ONLY the .bat, no APK (the app is unchanged
and debug.3's APK remains the current build — D84's
no-identical-re-release rule), tagged at the fix commit cb28507 and
Latest-marked; debug.3's release page gained a one-paragraph pointer
note superseding its older helper asset.

Evidence: the new file on disk — 13,683 B, sha256
ee35e4565dadf2a4a1c6e150208b8749aa6e5c3b8aec6303cc57025c06939558,
pure ASCII, CRLF; structural desk-check clean (even quote counts
per line, block parentheses balanced to zero, the D88 PS one-liner
unchanged apart from the exe resolution); the pattern itself is
proven in production — setup-offline.bat's full-path call has
shipped to real machines since v0.7.5. The release lane verified
live through the GitHub API: debug.4 created regular (draft=false,
prerelease=false) at the fix commit, its asset uploaded and the
served bytes re-downloaded and hashed byte-exact (sha256 match,
13,683 B), /releases/latest → debug.4; debug.3's body 9,017 →
9,650 chars, prefix byte-identical, the note present exactly once.
The Windows execution itself remains ⛔ on this Linux host, as at
D88 — the script self-verifies on Ali's machine (the read-back
round trip), and every failure path carries its own message. Zero
Go/Svelte/app code touched; versionCode stays 3; ~/sdk still wiped
(the reinstall caution stands for the next code-touching session).

**D91 — the first device feedback: root-cause what the host can prove,
instrument what only the device can measure (M8-j, 2026-10-17).**
Ali's first DEVICE report from the debug.3 circle: the app works on
Android, the YouTube app routes and the feed refreshes fast — but
video playback takes 30 s–1 min+ to show; the on-screen numbers sit
"too near each other"; a Play Protect popup appeared at install; and
the release keystore was created (kept private, per D88/D90).
Four rulings:

1. **The spacing complaint is root-caused and FIXED**: the pro
   screen's stage was fixed at 22.5 rem (designed for a 360 px
   viewport at 1× font scale); on narrower screens or with Android's
   font scale it outgrew the viewport and pushed the timer/speed
   numbers against the edges (measured: stage 495 px in a 320 px
   viewport at 1.4× — the two speed values sat nearer to each other,
   24 px, than to the screen edges). The stage is now capped to the
   viewport (min(22.5rem, 100vw)), the speeds row widened to 1.75 rem
   with nowrap, the timer digits given more air (0.66 em boxes,
   tabular-nums). This is the M6 font-scaling gate's first live
   observation — the gate exists precisely for this class.

2. **The video-start delay is INSTRUMENTED, not speculatively
   "fixed"**: the host-level suspects were each eliminated with
   evidence (the netstack DOES emit ICMP 3/3 port-unreachable for
   non-DNS UDP — the QUIC fast-fail contract, now pinned by
   TestUDP443FastFailICMP; the TCP receive window is 1 MiB; fragment
   pauses are 1–10 ms; DoH is cached/singleflighted/keep-alive; the
   conn cap is 1024; the connect gate already runs a real proxied
   YouTube check before "active" — Ali's "make the connection time
   longer" suggestion is already built, per M5). What remains is
   device/network-side and cannot be measured from this host — so
   every tunnel now logs its phases (tunnel up dns=…ms dial=…ms;
   tunnel first-byte …ms; tunnel done dur=… up=… down=… — values in
   the message, hostnames only behind LogHostnames, riding the log
   ring into the redacted diagnostics report). The next field report
   names the phase that eats the time.

3. **Play Protect warning a sideloaded debug-signed APK is expected
   behavior, not a defect** — documented in the debug.5 release
   notes; nothing to change in the app.

4. **The release keystore needs NOTHING now** — and must NOT be used
   to sign the test circle: the debug builds deliberately share the
   pinned debug certificate (D85) so each installs over the previous
   in place; signing a test build with the release key would break
   that chain. It waits for the signed public release (D88/D90).

Evidence: the probe that became the test (a hand-built UDP/443
datagram through the crossed-wire stack produced ICMP type 3/code 3,
quote carrying the refused dport); the before/after geometry
(stage 495→320 px at the 320 px/1.4× case, value gap 24→39 px, the
canonical 360 px case unchanged at stage 360); the full baseline
green after the changes (vet clean; go test 17/17 ×2; svelte-check
0/0; vite clean; webui dist CLEAN-synced — the accumulated stale
hashed assets purged from the embed for the first time); the build
lane (gomobile bind → younblock.aar 24,914,100 B; assembleDebug →
app-debug.apk 34,831,081 B, sha256 5df2acc3…02b4, versionCode 4,
badging minSdk 26/target 36, 3 ABIs, apksigner cert c76bb7c6…8551d —
the pinned D85 debug certificate); the release lane live through the
API (v1.6.6-android-debug.5 created regular at ad02980, Latest-
marked, the APK asset uploaded and its served bytes re-downloaded
via the API asset endpoint and hashed byte-exact). The first honest
API wrinkle of the round: a release created with a SHORT
target_commitish is rejected 422 ("tag_name is not a valid tag") —
use the full SHA.

**D92 — one UI across every device: Ali's uploaded instruction
rounds (mobile.zip) are applied to THE SHARED SCREEN, and the usage
history is tracked once, in the engine.** Ali uploaded mobile.zip to
the repo carrying the UI instruction documents his Windows/Linux
builds already follow — a round-2 audit (roast-report.md: shrunken
UI, touching digits, hint printed over ring dots, floating red
close, drawn icons) with fix-instructions.md, and the round-3
feature spec (feature-spec.md + agent-instructions.md: per-session
data usage with a local-only history) plus usage-screens.png, and
reported the mobile UI is NOT consistent with the desktops. Three
rulings:

1. **The compliance point is the shared screen, not a mobile
   fork.** The pro screen (frontend/src/App.svelte + lib/) is the
   one UI every edition ships — Wails desktops embed
   frontend/dist, the Linux web edition and the Android WebView
   embed the same bundle through internal/webui — so applying the
   rounds THERE is what makes all devices consistent by
   construction. Round-2 applied: Lucide icons through one file
   (lucide-svelte@1.0.1, src/lib/icons.ts; Power, Ellipsis,
   ChevronsUp/Down, ArrowUp/Down, X — the logo and faint play mark
   stay drawn by design); the menu button is Lucide "…" at the top
   RIGHT (Jakob's law) at 44 px (Fitts); the sheet's close is a
   neutral 44 px X inside a fixed header (title left) while the
   body scrolls underneath; timer digit boxes 0.80 em (the face's
   widest digit is 0.76 em — digits never touch); the connected
   ring family capped (outer arc 6.0 rem around the resting thumb)
   so no dot reaches the hint, which moved to 37.25 rem and grew to
   12 px at 82 % white; ping gets a word (Good ≤ 120 ms, OK ≤
   400 ms, else Slow — thresholds in App.svelte's config block);
   speeds the engine did not measure are hidden (-1), never faked
   as 0; "Created by ChitaCodes" rides the About version line and
   an honest "Other websites" section exists (VERIFIED_SITES empty
   until a site passes site-test-plan.md's two-day rule); the Wails
   window is 360×640, resizable, floored at 320×560. Round-3
   applied: SHOW_SPEEDS=false by default (the total is what
   matters) with the session usage lines under the timer
   ("2.10 GB used" + the ↓/↑ split, 3-significant-digit decimal
   units), "Last session: size · length" under Not Connected, and
   the menu's Data usage section (Today / This month / All time
   cards, the last 10 sessions, the provider-may-count-more
   disclaimer, and a confirmed Clear history: Delete (red) vs
   Keep).

2. **The ENGINE is the one usage tracker — one file, every
   platform.** Both UI transports sit on the engine (the Wails
   bridge and the webui JSON API that serves the Android WebView
   and the Linux edition), so a session tracked anywhere else
   would be a second tracker disagreeing with the first. A session
   is the engine's "on" run (on↔degraded flaps keep the ORIGINAL
   start; a cancelled or failed connect never creates one); its
   bytes are the deltas of the relay counters the tunnels have
   always fed (AddBytesDown/Up in tunnel.go — the first fragmented
   record included; DoH's own direct sockets are engine overhead,
   honestly uncovered by the disclaimer, not browser data). The
   record (startMs/endMs/downBytes/upBytes — times and byte
   totals only, never a website name) lands in
   <ConfigDir>/usage.json (%APPDATA%\Younblock on Windows, the
   app's private storage on Android), capped at 500, autosaved
   every 15 s for crash recovery and recovered on the next engine
   open. GetUsage/ClearUsage ride every transport: the Wails
   binding (app.go), /api/usage + /api/usage/clear (webui), and
   the state carries downBytes/upBytes/lastSession with an
   EXPLICIT null after a clear (the screen's Last-session line
   depends on present-null vs absent). Store writes happen under
   the engine mutex — deliberate, so an in-flight autosave can
   never land after the final record and resurrect a running row.

3. **The reference code folder (younblock-ui/round-3/) was not in
   the upload — the implementation follows the documents.** The
   zip carries the specs and the screenshot, not the tested
   reference; every rule above is traceable to a line in
   fix-instructions.md, feature-spec.md, agent-instructions.md or
   usage-screens.png, and the acceptance items that need the
   missing Python harness were verified the honest way instead:
   agent-browser over vite preview at 320×560, 356×625 and
   360×760 across Not Connected (with Last session), Connected
   (usage ticking, ping verdict), the menu (cards, rows, confirm,
   Keep/Delete, clear dropping the Last-session line) and Error —
   no overlap, no scrollbars; and the byte path was proven on the
   REAL binary: a 949,386-byte YouTube page pulled through the
   engine's own tunnel recorded as session downBytes 980,303 /
   upBytes 1,933, then a stop, a recorded 80-second session in
   usage.json, and todayBytes = allTimeBytes = 982,236 in the
   summary.

**D93 — the close button is a real shutdown by default; keeping the
app in the tray is an explicit opt-in (Ali's ruling, 2026-10-18).**
"By default, when the exe is closed and the app is shut down, the app
must not remain on the taskbar — only keep being in the taskbar if the
user selected this option in the settings." Three rulings:

1. **The default inverts: closing the window QUITS.** Since the first
   Windows build, `closeHidesToTray()` answered a hard-coded `true` on
   Windows — every close hid into the tray, and the app lived on in
   the taskbar/notification area with the proxy engine still running.
   That is now the OPT-IN. `closeHidesToTray(keepInTray)` answers
   exactly the user's setting on Windows (tray_windows.go) and always
   false everywhere else (tray_other.go — no tray exists there, so
   even an opted-in user cannot hide into nothing); `beforeClose`
   composes the two and allows the real close otherwise, and the
   shutdown path it hands control to is the one Linux already ran for
   years: stop the browser session, quit the tray, disconnect through
   the bridge, `eng.Shutdown()` — Stop restores the Windows proxy
   first, so a default close can never strand the user's internet.
   The `--minimized` launch flag and the start-with-Windows autostart
   are untouched: they are explicit launch choices, not the close
   button's semantics.

2. **One field, every edition, no migration owed.** The setting is
   `keepInTrayOnClose` in settings.json (default false — the behavior
   Ali demanded). Every file written before D93 lacks the field and
   loads as exactly that default: the field never recorded a choice
   (the old behavior was unconditional), so settingsVersion stays 4 —
   the D86/D87 migrations existed because a file COULD carry a
   user-made list; this change carries no user state to preserve.
   The opt-in surfaces where its platform can honor it: the pro
   screen's menu sheet gains a SETTINGS section (the sheet the
   default edition ships everywhere — gated on the host's
   capabilities.tray, so Android, the Linux web edition and previews
   never render it), and the classic screen's Settings view gains
   the row beside connect-on-launch (gated the same way, hasTray).
   The copy teaches the contract before anyone flips anything: the
   first-run tray variant now promises "Closing the window stops
   Younblock.", the setting's own hint states what Off and On each
   do, and both Help decks' "window disappeared" entries explain the
   opt-in instead of assuming tray-keeping. The one-time close
   notice (file 02 §199) stays unwired as it has always been: Go
   never emitted "close-requested", the pro screen has no listener
   for it, and the setting's hint + Help copy already carry the
   disclosure — wiring a dormant event into two screens for an
   opt-in-only path would risk a window that can never close-to-tray
   on the default screen.

3. **Desktop-only round: no Android release, the Windows exe is
   rebuilt by Ali.** Nothing in the Android app changes — it has no
   window close and no tray, the row is capability-gated off, and the
   settings field round-trips untouched through the shared screen
   (the same contract `apps` already has on desktops). debug.6
   (versionCode 5) stays the repo's Latest release; Ali rebuilds the
   desktop exes from this tree with build.bat / build.sh. Verified
   here the honest way: the full baseline (vet clean, go test 18/18
   packages twice, race on the touched two, svelte-check 0/0, vite
   build + webui dist CLEAN-synced); `GOOS=windows go build` of the
   whole shell compiles the Windows branch; the contract is pinned
   by tests (close_test.go: the parse table, the no-tray platform
   answer, a real engine's beforeClose quitting on defaults AND on
   opt-in-without-tray, quitting always allowed mid-Quit; config:
   default false, pre-D93 file loads false, opt-in round-trips); and
   the UI was exercised end-to-end on a REAL `cmd/core --web` engine
   using a disclosed scratch build whose Linux platform reported
   tray:true (reverted before commit): both screens' toggles saved
   `keepInTrayOnClose` through /api/settings into the real
   settings.json and back, VLM-checked screenshots of both, and the
   CLEAN rebuild showed the sections hidden (the negative check for
   the capability gate).

**D94 — the account gate opens as the project's next feature lane:
Younblock becomes members-only, and the design lands before the code
(Ali's ruling, 2026-10-18).** "Planning to add login authorization for
this, like a normal VPN app, where users must have been signed up or
input credible and verified user/password... The objective is to make
the app more exclusive, so only people who have username password can
use it. so we need login page etc. plan and design ahead for it's
backend requirements for example user database, admin managing users
etc."

1. **The direction is ACCEPTED and recorded before any code exists.**
   A verified username/password gate in front of the app's usage, on
   every edition (Windows, Linux, Android — implemented once at the
   engine level so all three shells inherit it), backed by a real
   authentication backend: a user database, login/verification
   endpoints, and an admin surface for managing users (create,
   disable, reset, expire). The "plan and design ahead" instruction is
   honored the way this project always has: a spec lane first (the
   auth lane's audit + architecture + ADRs — the M0–M2 pattern the
   Android port followed), THEN implementation. Nothing is built
   before the spec exists and the open questions below carry Ali's
   rulings.

2. **The privacy-claim conflict is named NOW, not discovered later.**
   The standing truthful-claims set says "no account, no data
   collection" and "nothing leaves the device except the routed apps'
   own traffic and their DNS queries" (doc 07 §1, doc 10 §2, the
   in-app Privacy sheet, the README). An account gate REQUIRES a
   remote authentication server, account records, and login traffic
   from every device that uses the app. Those claims change when this
   ships — deliberately, disclosed in the changelog, the About/Privacy
   copy and the release notes, never silently. What does NOT change:
   the proxying itself stays local (no VPN server, no IP hiding, no
   added encryption — the account gate gates USAGE, not traffic; the
   routed data path is untouched by this feature).

3. **The architecture questions the design lane must answer (each
   needs Ali's ruling before implementation):** where the backend
   lives (a small self-hosted VPS service vs a managed backend — the
   repo is deliberately dependency-light, but auth is a new animal);
   who creates accounts (admin-created only vs open signup with
   approval — the "exclusive" objective points at admin-controlled);
   fail-open vs fail-closed when the auth server is unreachable (the
   recommendation is fail-closed with a bounded offline grace, but
   the ruling is Ali's); token/session design (opaque server-side
   sessions vs signed JWTs, expiry, revocation-on-disable); password
   storage (bcrypt/argon2id, never plaintext, never recoverable);
   brute-force protection; and the login surface on the shared screen
   (a signed-out state before the connect control — the same screen
   on every platform, per D92's one-UI rule).

4. **Sequencing: the feedback loop does not stop.** The account-gate
   design runs as the SECOND workstream of the next session; the
   debug.6 device round (the cross-device UI verdict, the data-usage
   check, the still-open slow-video diagnostics report, the D93
   Windows check on the rebuilt exe) stays the FIRST input — field
   evidence keeps flowing while the spec is written.

**D95 — question (b) ruled: admin-only account creation — no open
signup, no approval queue; Ali creates every account (Ali's ruling,
2026-10-18, the second exchange of the A0 day).** "I create account -
admin-only."

1. **RECORDED as ruled; it matches the A0 recommendation (00's table
   row b) and settles the WHO:** an account exists only because the
   admin deliberately creates it, one per member, through the admin
   surface (04 §2's lifecycle flows unchanged). No open registration
   page, no self-signup, no approval queue ever gets built — there is
   nothing to design around. The exclusivity mechanism is admin trust,
   exactly as D94 intended.
2. **What it does NOT settle (no silent narrowing):** the surface FORM
   remains a recommendation riding (f)'s sibling — CLI first (04 §2),
   a web admin only if the member count or the admin's patience later
   warrants it. Questions (a), (c), (d), (e), (f) stay exactly as open
   as before this ruling, and the new (g) below is added on top.
3. Docs updated with it: docs/auth/00 (row b marked RULED), 01 (status
   line; §3 was already admin-shaped), 04 (§2's WHO marked ruled). No
   code exists; nothing was built this round.

**D96 — the two-edition direction recorded BEFORE any code: Younblock
will ship as two Android editions — members (with the account gate)
and free (no gate, no account, no server) (Ali's direction,
2026-10-18).** "Also I want two apk version, one with auth, one
without (free for all)."

1. **The direction is ACCEPTED and recorded before any code exists
   (the D94 pattern).** Two APKs means build-time by construction —
   one source tree produces both; the gate is compiled in only for
   the members edition. The free edition contains no gate code at
   all: no auth client, no token file, no signed-out state, no
   `auth` capability bit, no auth methods in the facade — verifiable
   by absence; its engine is exactly today's engine. The mechanism
   (design, A2 implements only after the rulings): Go build tags on
   the engine gate and the facade (the same mechanism the A0 fix just
   used for `//go:build unix`) + Gradle product flavors on Android.
   A runtime toggle is ruled out by the direction itself — one APK
   with a flag would not be "two apk versions", and it would put the
   gate's code inside the free APK too.
2. **The A0 assumption "every shell inherits the gate" (02 §1,
   AUTH-001) is REVISED accordingly:** every shell *of the members
   edition* inherits the gate by construction; the free edition
   compiles the gate out entirely. The pack's cross-references now
   read that way (dated notes in 00/01/02/05).
3. **The privacy copy splits per edition (D94 ruling 2's discipline,
   never silent):** the free edition keeps today's claims VERBATIM —
   "no account, no data collection" stays literally true of it (it
   talks to nothing); the members edition ships 05 §6's drafted
   account copy. Both are disclosed at the first release that ships
   the split.
4. **The honest bounds, stated now (05's discipline):** the free
   edition is free-for-all BY CONSTRUCTION and keeps working forever
   even if the auth server dies or is never paid for again — a
   feature of the split, and the honest meaning of "free": nothing in
   that edition is exclusive, and nothing in it can be bypassed,
   because there is no gate to bypass. The members edition's bounds
   are EXACTLY 05 §2's (client patching, token extraction, credential
   sharing); the split neither weakens nor strengthens them — the
   gate code is identical.
5. **What it opens — OPEN (g); nothing below is a decision until Ali
   rules:** (g1) package identity — side-by-side install
   (recommended: an applicationId suffix, e.g. `.auth`, two icons on
   one phone) vs one package name that replaces itself on install;
   (g2) distribution channel per edition (free public — GitHub
   Releases / a website / anywhere; members only to the circle?);
   (g3) scope — Android only first (recommended; the same Go tags
   extend the split to the desktop shells later if Ali asks), or
   desktop splits too from day one; (g4) naming for the release notes
   and the two privacy sheets (e.g. "Younblock" vs "Younblock
   (Members)") — needed before any copy is written. The Gradle-flavor
   + build-tag mechanism in (1) is the design recommendation,
   recorded as such.

**D97 — the account gate is CANCELLED outright, before a line of it
was ever built: Younblock stays ONE free-for-all app — no login, no
accounts, no auth server, no editions (Ali's ruling, 2026-10-18, the
third exchange of the A0 day — A0-R3).** "You know I've decided to
defer and cancel this auth version, only work on non-auth version. it
doesnt worth this headache. cancel it altogether, no auth needed
anymore."

1. **RECORDED as ruled — cancellation, not deferral.** The message
   opens with "defer and cancel" and closes with "cancel it
   altogether, no auth needed anymore"; the closing words govern.
   Nothing of the gate will be built: no login page, no signed-out
   state, no user database, no authentication server, no admin
   surface, no invite codes — nothing of AUTH-001…044 (or any
   successor requirement) gets implemented. The implementation ladder
   A1/A2/A3 is cancelled and those Task IDs retire. The exclusivity
   objective of D94 is abandoned with the feature, said plainly:
   the app is free-for-all, and that is now the recorded product
   direction, not an accident.
2. **What it supersedes:** D95 (admin-only accounts) and D96 (the
   two-edition split) are superseded — they remain in this file as
   history, but their subject no longer exists; there will be no
   members edition and no free edition, because there is only one
   app, which was always the free one. OPEN questions (a), (c), (d),
   (e), (f) and (g1)–(g4) all close as MOOT — cancelled by this
   ruling, never ruled on their merits; no recommendation among them
   is promoted to a decision. ADR-AUTH-0001 (doc 08) is RETIRED —
   accepted once as the gate's architecture, now the record of a
   design that will never be implemented. D94 stands as history: the
   design lane it opened ran, delivered `docs/auth/`, and is now
   closed by this cancellation.
3. **What the product keeps, verbatim:** today's truthful-claims set
   — "no account, no data collection" (doc 07 §1, doc 10 §2, the
   in-app Privacy sheet, README) — was always literally true and now
   stays so permanently; the copy change D94 anticipated (05 §6's
   drafted account text) will never ship. Nothing needs removing
   from the code because nothing was ever added: no login code, no
   server, no database, no keys exist anywhere in the repository or
   in production (the A0 state, carried forward verbatim through
   A0-R2 and this round — verified by construction: both rounds were
   docs-only commits).
4. **What survives as record, marked CANCELLED:** `docs/auth/` stays
   in the repository as the design record of a feature that was
   specified, questioned, partially ruled on, and then cancelled
   before implementation — every file carries a dated cancellation
   banner (D97). The Iran-reachability finding in 02 §6 (any future
   server-side feature must be reachable from Iranian ISPs as
   ordinary HTTPS, and that is verifiable only in the field) stays
   on record as a constraint, because it is true of the network, not
   of this feature. If auth is ever re-opened, a NEW decision
   supersedes this one, and the pack is revalidated against the
   then-current tree before anything is trusted — it describes the
   seams as of 6a7bce4, not of whatever the tree becomes.
5. **What this does NOT decide (no silent narrowing, no silent
   widening):** the cancellation governs the account gate only. It
   is not a ruling on a public website or APK distribution channel,
   on the dormant desktop release lane, on public-release
   engineering (signing, CI, updates), or on any future feature —
   those remain exactly as open (or dormant) as before this message.
   The device-test feedback loop (M8-n and successors) is unaffected
   and remains the first workstream. Next decision ID: D98.

**D98 — every release carries the debug APK asset (Ali's ruling,
2026-10-18, the M8-n round — the standing offer from the M8-i/doc-06
era is answered).** "also from now on, add the APK version (debug
version) in the releases."

1. **RECORDED as ruled — every release page from now on carries the
   app's debug APK as a release asset.** This relaxes D84's
   no-duplicate-asset discipline deliberately: the APK (the debug
   test-circle build with the pinned D85 certificate) is attached to
   EVERY release, including helper/docs-only rounds like debug.4 was,
   so a release page never exists without the installable app on it.
   The trigger was observable: v1.6.6-android-debug.4 shipped without
   an APK asset (only make-release-keystore.bat) — Ali noticed, and
   this ruling closes that gap permanently.
2. **Scope:** the ruling covers the APK asset on release pages. It
   does not change D84's versionCode-bump rule (every debug build
   still bumps), D85's pinned debug certificate, D89's
   regular-release discipline (never prerelease), or D88/D90's
   release-keystore ceremony. The release lane (doc 06 §5) gains one
   standing step: upload the APK, verify bytes via the API, record
   asset size + digest.
3. **First application:** v1.6.6-android-debug.7 (the M8-n
   first-video fix build, versionCode 6) — its release page carries
   app-debug.apk, and every release after it does too.
4. Next decision ID: D99.

**D99 — the APK asset carries the version name (Ali's ruling,
2026-10-18, the M8-o round — extends D98 with the naming rule).**
"mention the version name in the apk . like v16.6-debug.apk"

1. **RECORDED as ruled — from now on every release's APK asset is
   NAMED with the version**, in the form `v<versionName>-debug.apk`
   (v1.6.6-debug.apk for the 1.6.6 line; Ali's "v16.6-debug.apk"
   example reads as the quick-typing of the same idea — the version
   name 1.6.6 with the v prefix and the debug suffix). The asset name
   must identify the build without opening the release page.
2. **Scope:** the asset NAME only. The release TAG keeps its
   v<versionName>-android-debug.N form (D84/D89 unchanged — the tag
   still carries the build sequence N, which the filename does not
   need), the versionCode-bump rule is untouched, and the artifact
   itself stays the same debug build with the pinned D85 certificate.
   D98 (every release carries the APK) is unchanged — this ruling
   names what D98 attaches.
3. **First application:** v1.6.6-android-debug.8 (the M8-o build,
   versionCode 7) — its APK asset is named v1.6.6-debug.apk, and
   every release after it follows the form.
4. Next decision ID: D100.

**D100 — the in-app diagnostics surface is the evidence path (Ali's
ruling, 2026-10-18, the M8-p round — his directive after the debug.8
device round).** "The problem persists, there is no diagnostic option
in the apk app setting. add an option to copy paste logs and
diagnostics so i can paste it here."

1. **RECORDED as ruled — the APK carries a first-class diagnostics
   surface**: a Diagnostics screen reachable from BOTH the menu and
   Settings (the two places he looked), with the report text
   selectable on screen and delivered out through Copy (native
   clipboard), Share (the system sheet), or manual selection. The
   surface shipped as ADR-0015 in v1.6.6-android-debug.9. The ruling
   also fixes the loop's evidence discipline: after this round, a
   device report of "still broken" without the pasted report is
   incomplete — the report is one button away and names its own build
   (versionCode identity inside it).
2. **Scope:** the diagnostics surface and its honesty rules (Copy may
   only claim success when a path delivered; the report's R21
   redaction extends to the new TUN/build sections). It does NOT
   decide the stall itself (doc 10 row 3 stays open; no third
   root-cause guess without a pasted report), and it does not change
   QUIC/UDP policy (the tun: counters are observation, not policy).
3. **First application:** v1.6.6-android-debug.9 (the M8-p build,
   versionCode 8) — the round that shipped the surface.
4. Next decision ID: D101.

**D101 — a surface ships in the variant the edition serves (the M8-q
root cause, 2026-10-18 — Ali's round on debug.9: "I still dont see
diagnostic button").** The debug.9 APK DID carry the ADR-0015 surface —
but only in the classic UI tree, while the APK bakes the pro screen
(D66: `build-android.sh` leaves `VITE_UI_VARIANT` unset, so
`main.ts`'s dispatcher defaults to pro; only the desktop wails lanes
bake classic). Ali's phone rendered the pro screen, whose only
diagnostics surface was the M6-era section buried at the very bottom
of the menu sheet behind an android gate, with the raw
`navigator.clipboard` Copy that silently fails in the WebView — the
exact failure ADR-0015 was written to end. The promised "Menu →
Diagnostics → Copy" existed only on screens his device cannot show.

1. **RULED — every user-facing surface ships in the variant(s) the
   edition actually serves.** A feature merged into one UI tree is
   INVISIBLE to every edition that bakes the other tree, and a
   "shipped" claim that holds for only one tree is a false claim. The
   check is part of the ship checklist from M8-q on: name the variant
   each affected edition bakes (Android + web single-file → pro;
   Windows/Linux wails desktops → classic), and verify the surface in
   the served variant through the real webui server (the loopback
   page the WebView loads), not only in the source tree.
2. **First application — the ADR-0015 port (ADR-0016):** the
   diagnostics surface now exists in BOTH trees. The pro screen (what
   the APK serves) carries Diagnostics as the FIRST row of the menu
   sheet — no scrolling, no gate — opening a full-screen report view
   (the same clipboard ladder, Share behind the shell bridge, the
   selectable readonly text). The old buried sheet section is
   removed. The classic tree keeps its M8-p screen unchanged.
3. **Scope:** UI-tree placement discipline and the port itself. It
   does NOT decide which variant the APK SHOULD serve (pro stays the
   Android screen — D66 unchanged; switching variants on Ali's device
   now would change the whole UI under him mid-loop), and it does not
   touch the stall (doc 10 row 3 stays open; a real pasted report is
   still the next evidence).
4. Next decision ID: D102.

**D103 — the APK asset name carries the build number (Ali's ruling,
2026-10-19, the M8-u round — amends D99 after the fourth-report
install miss).** "please add debug13 in the apks name, so such
accidents does not happen."

1. **RECORDED as ruled — every release's APK asset is named
   `v<versionName>-debug<N>.apk`**, where N is the release tag's
   build sequence (the .N in `v<versionName>-android-debug.N`). The
   filename must identify the build WITHOUT opening the release
   page — D99's own stated test, which the shared
   `v<versionName>-debug.apk` form failed in practice: six releases
   (debug.8 through debug.13) carried the identical filename, and
   the fourth diagnostics report round ran the previous build with
   no way to tell the downloaded files apart once they left the
   release page.
2. **First application — the backfill:** all six same-named assets
   renamed IN PLACE through the API (v1.6.6-debug8.apk …
   v1.6.6-debug13.apk; sizes and download counts preserved, bytes
   unchanged — the debug.13 asset re-served and hash-matched after
   the rename:
   18b4ea648b0aa02d7eebf84f2dc596d13894093a1469dfc4f111ddc5b286b518),
   and every affected release body updated so its download
   instructions name the new file and carry the rename note.
3. **Scope:** the asset NAME only — the release TAG keeps its
   v<versionName>-android-debug.N form (D84/D89 unchanged), the
   versionCode-bump rule is untouched, and D98 (every release
   carries the APK) is unchanged; D99's `v<versionName>-debug.apk`
   form is superseded by `v<versionName>-debug<N>.apk` from debug.14
   on. The pre-D99 releases (debug.1 through debug.7, the
   `app-debug.apk` era) keep their historical names: they predate
   the naming rule, and Android refuses same-signature versionCode
   downgrades, so the silent wrong-build install cannot happen
   through them.
4. Next decision ID: D106. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates this one.)

**D104 — the launcher icon is the owner's mark (Ali's ruling,
2026-10-21, the M8-am round — "use this for app icon").** The owner
uploaded `icon.ico` to the repo root (byte-identical to
`assets/icon.ico` — a 32×32 RGBA mark: a padlock whose red body
carries the white play triangle, a gold lightning bolt with a light
outline breaking through the top-right, the dark shackle, two impact
dashes).

1. **RECORDED as ruled — the Android launcher icon is that mark.**
   The 32 px source is too small to upscale, so
   `scripts/gen_android_icons.py` traces it to vector geometry and
   emits the set: `mipmap-anydpi-v26/ic_launcher.xml` (adaptive: the
   brand dark `#1E1F21` background, the mark as the foreground at the
   52/108 safe zone, the same foreground as the Android 13+
   monochrome layer) and the legacy raster tiles (48–192 px, the mark
   on the same dark rounded square the Windows app icon uses). The
   old placeholder (`drawable/ic_launcher.xml`, the pill mark) moves
   to `drawable/ic_notification.xml` — it stays the notification's
   small icon, where Android tints monochrome and the simple mark
   reads as a silhouette.
2. **Fidelity rules, pinned in the script:** five flat colors
   measured from the source's opaque interiors; anti-aliased blend
   pixels resolve to the nearer ENDPOINT of the pair that explains
   them (a red↔white blend must never land on the gold); the bolt's
   light outline is a designed element and survives as its own layer;
   the impact dashes trace at the speckle filter that keeps them
   (100 ate them; 20 keeps every designed element); every layer
   renders against the quantized source at IoU ≥ 0.90 or the script
   refuses to write — and the committed geometry was additionally
   VLM-verified FAITHFUL against the original mark (adaptive circle,
   adaptive square, and legacy tile renders).
3. **Scope:** the launcher and notification surfaces only — the
   in-app wordmark, the tray icons and the packaging icons are
   untouched. The script asserts the two icon.ico copies stay
   byte-identical (a divergence is loud, not silent).

**D105 — the connect-time hand-off: open YouTube, refreshed, when
Younblock connects (Ali's ruling, 2026-10-21, the M8-am round).**
"add this option in setting, checked by default: when app connects
automatically open android's youtube app and refresh it."

1. **RECORDED as ruled — a new setting, checked by default.**
   `autoOpenYouTube` (settings schema v8): TRUE on Android — the
   fresh install and every pre-v8 file migrate to true (the field
   was absent, so its false was the shipped default, never a choice
   — the v5 fragmentAllRouted pattern); false on every desktop,
   where the field round-trips but nothing reads it
   (`openBrowserOnConnect` is the desktop counterpart of the same
   idea). The UI row (the menu sheet's Settings section, gated on
   the new `autoOpenTarget` capability) saves through the ordinary
   settings door.
2. **The behavior:** once per connect run — when the engine reports
   serving AND the TUN is actually bound — the shell drops the
   YouTube app's cached background process
   (KILL_BACKGROUND_PROCESSES, a normal install-time permission:
   the refresh half) and launches it (the open half), so the first
   open is a cold start through the live tunnel instead of a resume
   of the app's pre-connect dead-host ladder. Doc 11 §5.1 measured
   that ladder owning most of the felt 5–6 s first-open load; the
   hand-off is the owner's own sidestep — it does nothing the app's
   player would not do eventually, it just skips the waiting
   (§5.2). The latch fires once per run (a degraded→on flap never
   re-launches over the user); a restart cycle without a fresh
   serving event is the SAME run and never re-fires.
3. **The doors stay one truth:** the capability gates the row
   (Android reports `autoOpenTarget: true`; every other platform
   never shows it), the facade's `AutoOpenYouTube()` is the shell's
   fresh-at-the-moment read, and a change saved mid-connect applies
   on the NEXT connect — the allow-list's rule, verbatim. If the
   refresh proves weak on some OEM (the kill is best-effort), the
   escalation is the launch intent's CLEAR_TASK flag — recorded
   here for the next round, not shipped speculatively.

**D106 — the hand-off waits for a proven video lane (the tester's
ruling, 2026-10-21, the M8-an round — "the better job is to make the
user wait and see connecting, only you're sure that even initial
video is smooth and fast").**

1. **RECORDED as ruled — the connect-time hand-off is GATED on a
   readiness signal, not the first serving event.** The debug.30
   field report ("the app reverted to our old problem… 1-2 minutes…
   I think the app connection time is too short, so when it
   automatically switches to youtube app it's not ready yet")
   convicted ADR-0051's fire moment: second zero of the tunnel's
   life, the earliest instant the stack exists, with the chain cold
   and the cluster memory empty. The engine now owns `warm` (state
   JSON, the UI and the shell both read it): false while the serving
   run settles, true once the run held through a 3 s window and one
   light re-check (a QuickCheck — the watchdog's own blessed
   emission classes, never googlevideo) passed; failing re-checks
   retry every 2 s; a 15 s cap opens the gate regardless (never
   trap). The shell fires kill+launch only on serving AND warm AND
   TUN-bound, still once per run. The connect screen holds its
   connecting look ("Getting the first video ready…") until the flag
   lifts, on Android only. ADR-0052 carries the full record.
2. **The setting itself is unchanged:** `autoOpenYouTube` stays,
   checked by default — the ruling changes WHEN the hand-off fires,
   not whether it does.

**D107 — the dead-cluster marks persist across sessions (doc 11
§5.3's pre-registered fix, shipped on the debug.30 indictment,
2026-10-21, the M8-an round).**

1. **RECORDED — the per-session amnesia ends.** The cluster memory
   (ADR-0047) that fast-fails blackholed clusters at zero wire was
   rebuilt from scratch every connect; the hand-off's second-zero
   cold start made the app's ladder pay the discovery burn (~3 flows
   × ~2.5 s) into a cold chain — the multi-cluster terminal shape
   the breaker's arm gate sums, the 1-2 minute mechanism of the
   debug.18-.25 era. `googlevideo-dead-clusters.txt` (beside the
   pool file) persists the LIVE dead marks; the next session seeds
   them with a 5 min TTL (half the in-session 10 min — the
   GoodPoolSeedTTL precedent); the own-dial rehab clears a stale
   seed at once, the per-cluster re-probe learns a lifted blackhole
   inside one 2-minute window, and a seed never shortens a fresher
   measured mark. The report's cluster line gains its seeded half
   (`dead=N (seeded=M)`), so the tester's fresh-connect paste says
   which world it is in. ADR-0053 carries the full record.
2. **Together with D106** this is what "even initial video is smooth
   and fast" means at the moment YouTube opens: the chain re-proved
   AND the blackholed cluster already marked AND the pool seeded —
   doc 11 §5.2's claimed state, now true at second zero.
3. Next decision ID: D108. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D108 — the owner's logo v1 is the app's icon (the icon.ico mark
retires, 2026-10-21, the M8-ao round — "use this picture for
app's icon").**

1. **RECORDED as ruled — `younblock-logo-v1.png` (his upload,
   commit 35dfcc1, parked for his word in M8-an) is the launcher
   icon's source, superseding D104's icon.ico trace.** D104's
   structure carries: the adaptive mipmap set, the Android 13+
   monochrome layer, the pill glyph as the notification mark. His
   design is a finished full-bleed squircle tile — the wordmark
   sits in the bottom quarter where every Android launcher mask
   would crop it away — so it ships as a faithful RECOMPOSITION,
   never a redraw: his own dark gradient as the full-bleed
   background layer (the launcher's mask is the shape his squircle
   owns elsewhere), his art soft-matted out of it as the foreground
   with every element — padlock, shackle, sparks, all the
   wordmark's letters — inside the 66dp safe circle (z=0.647, the
   content's extent centered; render gates IoU 0.97–0.98 against
   the matte on every density, VLM-verified complete and faithful),
   the same silhouette as the monochrome layer, and the legacy
   tiles carry his squircle exactly as drawn. ADR-0054 holds the
   full record and the gate inventory.
2. **Engine and playback behavior are untouched by this ruling** —
   the warm gate (D106) and the persisted dead-cluster marks
   (D107) ride into debug.32 unchanged.
3. Next decision ID: D110. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D109 — the hand-off stops killing YouTube (the refresh half of
the connect-time hand-off is removed, 2026-10-21, the M8-ap round —
the debug.32 field report's evidence).**

1. **RECORDED as field-convicted — `killBackgroundProcesses` at
   hand-off (D105's refresh half) guaranteed a cold YouTube at
   every connect, and the cold start's costs land on exactly the
   first video the hand-off exists to help:** a fresh process has
   no in-memory "QUIC is dead on this network" knowledge, so the
   first media connection burned its QUIC job timeout into the
   TUN's UDP drops (6920 ms measured on the debug.32 report — the
   M8-j ICMP refusal is correct and the app's QUIC stack ignores
   it), on top of a cold feed (a 45 s thumbnail trickle, a 19 s
   page stall) and a cold terminal ladder. The debug.29 state the
   tester ranked best ("so far v29 was our best") never killed
   anything. ADR-0055 holds the record; the launch half, the warm
   gate (D106) and the persisted dead-cluster marks (D107) ride
   unchanged, and the setting's default (on, D105) is untouched.
2. **The KILL_BACKGROUND_PROCESSES permission leaves the manifest
   with the call** (the honest-inventory rule).
3. Next decision ID: D110. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D110 — one screen for every version (the classic editions retire,
2026-10-23, the one-UI round — the owner's ruling: "update the UI of
all versions, they must be consistent, so when we change UI all
versions change the same").**

1. **RECORDED as owner-ruled — every edition now ships the same UI v3
   main screen:** the APK's WebView, the Windows apps, the Linux
   window and web editions all load the one screen
   (`frontend/src/App.svelte`, tokens imported from
   docs/ui-v3/younblock-tokens.css). The variant dispatcher and its
   `VITE_UI_VARIANT`/`?ui=classic` switch are retired (the URL
   parameter is left inert, not rejected); the classic component tree
   is deleted from the head of the tree (git history keeps it — the
   editions were a standing divergence by design: a second UI no
   rework ever touched, which is exactly the shape the ruling ends).
   The one shared file the classic tree held (lib/clipboard.ts, the
   honest clipboard ladder) moved into the shared lib and stays.
2. **The desktops additionally gain the phone's honest signals:** the
   Wails bridge now folds the engine's real degraded (slow mode) and
   warm-gate states into its snapshots (ui_bridge.go's optional
   HealthSource, engine_adapter.go observing State.Phase/State.Warm) —
   pure observation, zero engine change; the v3 spec §8 audit already
   established these are real engine events. The one-UI ruling is read
   to its end: one screen AND one set of signals on every version.
3. **versionCode 34 / versionName 1.6.7 / mobile string
   1.6.7-debug.1:** the base version moves with the unified line
   (1.6.6 → 1.6.7), so the debug counter restarts at .1 under the new
   base; the versionCode keeps climbing monotonically (34 > 33), so
   the APK installs over debug.34 in place (the pinned D85
   certificate).
4. Next decision ID: D111. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D111 — every version ships in the same release (the unified
release lane, 2026-10-23, the one-UI round — the owner's ruling:
"from now on, release all versions in same release").**

1. **RECORDED as owner-ruled — one GitHub release carries every
   version, from here on:** the APK, the Windows apps (64/32-bit), the
   headless engines, the Linux web editions, the .deb installers, the
   AppImages and the offline pack all ride ONE release (first run:
   tag v0.8.0). The lane is `release-unified.sh`: build everything
   (build.sh + build-android.sh + build-offline-pack.sh, the pack
   best-effort), push the tag, create the release, upload every
   artifact, then re-download and byte-compare each one (the release
   discipline the debug rounds already held, applied to the whole
   set). The token rides GITHUB_TOKEN, never the repo.
2. **The release-shaped divergence this ends:** the desktop downloads
   stood at v0.7.6 (the UI v2 screen) while the APK line ran 34 debug
   releases to UI v3 — the "apk ui differs with other versions" the
   owner saw was five weeks of un-released desktop rounds, not a code
   fork. One release lane makes the gap structurally impossible: the
   same tree, the same screen, the same release.
3. **The separate android-debug.N release series is superseded** by
   the unified series (the APK asset keeps its D103 build-carrying
   name inside the unified release; versionCode keeps the Android
   upgrade order honest).
4. Next decision ID: D112. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D112 — "Quit Younblock": the one deliberate end-of-app action
(2026-10-26, the quit round — the Android lane's requested feature:
"Add a 'Quit Younblock' action… It fully ends the app: connection
stopped, notification removed, app gone from the status bar and from
Recents").**

1. **RECORDED as requested — a separate, confirmed Quit, deliberately
   outside every disconnect semantics:** the lever's tap and swipe
   disconnect exactly as before; the notification's ordinary life is
   untouched (nothing in the quit path weakens or removes the
   persistent-notification behavior — the round only ADDS one door).
   The control sits in the menu sheet's "..." overflow (the screen's
   overflow surface), at the very bottom, Android-only, flat and
   tertiary — never a primary button, never near the power lever. One
   tap opens a confirmation dialog (title "Quit Younblock?", body
   "This stops the connection and removes the notification. You can
   open the app again at any time.", buttons Cancel and Quit — the
   owner's exact copy); only Quit confirmed runs anything.
2. **The ordered sequence (YounblockVpnService.performQuit +
   MainActivity.quitYounblock):** (a) the engine run stops cleanly —
   the same synchronous engine stop the honest paths use (proxy
   listener down, watchdog and self-repair loops down); (b) the TUN
   closes (unbindTun: the Go stack stops, the routes go with the fd)
   and the VpnService stops; (c) foreground ends with
   stopForeground(STOP_FOREGROUND_REMOVE) AND the notification is
   cancelled by its ID (NotificationManager.cancel(1)) — the
   belt-and-braces pair; (d) the re-starters are cancelled: the
   service was already START_NOT_STICKY, the app has no boot
   receiver, no jobs, no alarms and no Quick Settings tile (the
   manifest's inventory is the proof), and the one real re-starter —
   the Activity's connect flow — is gated by a process-wide
   YounblockVpnService.quitting flag (raised before the teardown,
   never cleared for the process's lifetime, checked by
   ensureVpnService/startVpnService/reconcile and by
   onStartCommand's raced-start guard); (e) the activity closes with
   finishAndRemoveTask, taking the app out of Recents; (f) after the
   queued teardown settles (600 ms — the service stop, the activity
   destroy, the engine's trailing state events are all main-thread
   messages ahead of the timer) the process is killed
   (Process.killProcess + exitProcess(0) fallback), because the Go
   runtime's and WebView's background threads never all return.
3. **The bridge door is write-only, like the rest of the page's
   surface:** window.YounblockShell.quitApp() (PageBridge, marshaled
   to the main thread) dispatches the sequence and returns
   immediately; a host without the door (the desktops, the Linux web
   edition, an older APK) keeps the app alive and the dialog says so
   honestly. The Quit row renders only where AppInfo reports the
   android platform — the Wails builds are untouched (no frontend
   behavior change is visible on them beyond the shared bundle's new
   inert component).
4. **Boot auto-start cannot be permanently disabled because none
   exists** (AND-028: nothing auto-starts after reboot — no
   BOOT_COMPLETED receiver, no RECEIVE_BOOT_COMPLETED permission).
   Quit ends the current session only: the next manual launch is a
   fresh process, the engine constructs, the page loads, the
   notification appears on connect — the normal first-open behavior,
   unchanged.
5. Next decision ID: D113. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)

**D113 — the status-bar glyph: the Y mark replaces the opaque square
(2026-10-27, the icon round — the owner's ask: "Currently when the
app is on the android's top bar (notification area) it shows a filled
solid square. can't we show a symbol like Y or play symbol instead?").**

1. **The diagnosis is the rendering model, not a broken asset:**
   Android renders a notification's small icon as an ALPHA MASK —
   every color is discarded and the system tints the surviving shape
   monochrome (white in a dark bar, dark in a light one). The M8-am
   pill glyph (drawable/ic_notification, D104's ruling that "the
   pill glyph stays the notification mark") drew its brand-red pill
   and green LED on an OPAQUE 108x108 background square — so the
   alpha mask was a full solid square, and the bar showed exactly
   that: the "filled solid square" the owner reported. The pill never
   had a voice up there; the status bar never shows color, so the
   red track and the green dot existed only in the color contexts
   (the launcher, the shade header on some OEM builds) — never in
   the bar the owner looks at.
2. **The Y wins over the play triangle:** the owner offered "Y or
   play symbol" — Y is the name's first letter, the mark that reads
   as Younblock rather than as generic media-control chrome (a play
   triangle in a status bar says "something is playing", the wrong
   story for a tunnel that runs silently; the launcher's padlock
   already carries the play motif in full color where color exists).
   The glyph is drawn as geometry, never an image: alpha-only white
   round-cap strokes on transparency — arms meeting at (12, 11.4),
   stem down to (12, 19.2), stroke 2.4dp, round caps and joins — on
   the 24dp status-bar grid with Material-keyline optical margins,
   so the letter survives every vendor's raster size and both bar
   themes. Rendered at bar size and VLM-verified before shipping:
   the preview reads as the letter Y, legible on both dark and light
   strips, strokes-only with no backdrop (the verification preview
   ships as download/y-icon-preview.png).
3. **Scope stays surgical:** one drawable changes
   (ic_notification.xml — the background square path is deleted, the
   pill and LED paths go with it); the Kotlin call site keeps
   R.drawable.ic_notification untouched; the launcher icon (the
   owner's padlock mark, D108's mipmap set), the notification's
   title, text, channel, ongoing life, the D112 Quit teardown and
   every disconnect behavior are untouched; the Go engine is
   untouched. The version line moves to 1.6.10 / versionCode 40 in
   the established lockstep (internal/config AppVersion, gradle, and
   the mobile facade's Version string together).
4. Next decision ID: D114. (D102 remains reserved for Ali's
   method-policy ruling — the per-flow preset retry question, which
   predates these.)
