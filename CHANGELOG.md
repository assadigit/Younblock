# Changelog

All notable changes to Younblock are documented here. Versions are the
GitHub release tags (what you download); the in-app version is noted beside
each one. The full engineering narrative behind every entry lives in
PROGRESS.md, and the reasoning in DECISIONS.md (D-numbers referenced below).

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
dates are Asia/Tehran. Binaries, SHA-256 checksums and upgrade notes for
every release are on the
[Releases page](https://github.com/assadigit/Younblock/releases).

## [Unreleased]

### Added (Android port, M6 — not yet a released build)

- **The per-app allow-list is a setting (schema v2, D79)**:
  `settings.json` gains the `apps` field; v1 files migrate through the
  existing version mechanism (the built-in default list — Chrome +
  YouTube — fills in). Normalize validates package shapes, dedupes,
  caps the list at 32 and restores the built-in defaults when the list
  is empty or all-invalid — an empty allow-list would route EVERY
  app, the opposite of the product scope, so that state can never
  persist. The same normalized list now feeds three doors: the file,
  the settings API the WebView edits, and the new facade binding
  `RoutedApps()` the VPN service reads when it builds the TUN.
  Changes apply on the next connect (the list is baked into the TUN at
  establish; the UI says so).
- **The mobile settings/UX surfaces (D80)**: the Android menu sheet
  gains the Apps-to-unblock row (friendly labels for the known two,
  package names for anything else, add/remove with the engine's saved
  response as the authority), the honest VPN-slot/key-icon About
  lines (still not a VPN: no IP hiding, no added encryption), the
  privacy disclosure (doc 07 §5: no analytics, no Younblock-run
  server, DNS to the user-configured DoH providers, local logs), and
  a diagnostics view over the existing redacted report. All of it is
  gated on the host's reported platform/capabilities — the desktop
  sheet renders its exact prior copy.
- The M6 debug APK (35,194,757 B, SHA-256 5669efd1…02dc) with the
  settings-driven allow-list and the new sheet is recorded in
  docs/android-port/05–06; on-device observation gates (live
  rotation/font-scale/TalkBack, the apps row on a real screen, plus
  the standing M4+M5 checklist) remain blocked by the sandbox (no
  /dev/kvm, no device) and are carried, never claimed.

### Added (Android port, M5 — not yet a released build)

- **The connection lifecycle is host-verified**: the engine's connect
  run now waits at an optional platform start gate (D77) before its
  listener — on Android, the VPN consent plus the established TUN —
  so an unpermitted engine holds at "connecting" (outside the start
  budget) and can never report "on" while nothing routes. The Kotlin
  consent flow completes the M4 wiring: first-run explanation →
  system dialog → grant starts the foreground service and opens the
  gate; denial stops the held engine and shows honest copy. Consent
  and service starts are single-flight; MainActivity re-derives the
  state on resume; a dying service stops the engine (no lying
  "Connected", no zombie connect); onRevoke is a full disconnect with
  re-consent on the next connect.
- **The honest routing report**: `VpnState()` (D78) answers the
  product's "Connected" question in one call — routing requires the
  engine serving AND a TUN stack bound AND that stack's door being
  the live server's door (a stack left on a dead door is reported
  stale for reconciliation, never as routing). The VPN notification's
  text is derived from this same report: active only when actually
  routing, degraded when degraded, connecting otherwise.
- The M5 debug APK (34,711,941 B, SHA-256 c46026a7…bf26) with the
  lifecycle shell is recorded in docs/android-port/05–06; on-device
  observation gates remain blocked by the sandbox (no /dev/kvm, no
  device) and are carried, never claimed.

### Fixed (Android port, M5)

- The facade's SOCKS5-door refusal paths closed the TUN fd leak they
  documented but did not perform (D76's single-owner rule now holds
  on every StartTun return).
- The mobile test harness now writes the product's default port
  behavior (AutoPort=true) instead of the accidental fixed-port
  setting that only held up in single-package runs.

### Added (Android port, M4 — not yet a released build)

- **The routing path exists and is host-verified**: a gVisor-netstack
  userspace stack (`internal/tunstack`) turns the VpnService TUN's IP
  packets back into TCP flows and hands each one to the engine's
  SOCKS5 front door — the same tunnel path (DoH resolve, protect()ed
  dial, first-record fragmentation for routed domains) the browser
  door uses. DNS from routed apps is answered by a virtual resolver
  (fake-pool A records, empty AAAA, other UDP dropped), so apps stay
  IPv4 and domains cross the door as domains. `YounblockVpnService`
  (foreground, per-app allow-list defaulting to Chrome + YouTube,
  fail-closed when no target is installed) plus the facade's
  StartTun/StopTun lifecycle are compiled into a verified debug APK;
  the engine's TUN contract and protocol policy are proven by
  host tests against the real proxy server and engine, including the
  protect() call-counting proof extended through the door. The
  on-device gates (real DoH through the stack, QUIC fallback,
  protect() on real sockets) remain honestly ⛔ — no emulator/device
  in this environment (docs/android-port 05 M4).
- Decisions D75 (gVisor-direct stack selection) and D76 (the TUN fd
  contract: non-blocking before handover, single-owner) in
  DECISIONS.md; gVisor vendored (+6 MB) per the vendor-means-vendor
  rule.

## [v0.7.6] — 2026-10-16 (app 1.6.6)

A feel pass on the redesigned connect screen: the state change is now a
choreography instead of a set of hard cuts, the drag answers "what happens
if I let go" before you let go, and the About sheet can no longer lie about
its version number.

### Fixed

- **The offline packs could not build the Windows apps.** Since the
  pack was born (v0.7.4), its `vendor\` folder has been missing the
  WebView2 bootstrapper (`MicrosoftEdgeWebview2Setup.exe`, 1.8 MB)
  that the vendored wails embeds into every Windows build — so on a
  fresh machine `wails dev` and `wails build` died with
  `no matching files found` before a single line of the app compiled,
  defeating the pack's whole "no internet" promise. The bootstrapper
  is vendored in the repo now (a `.gitignore` exception un-hides it
  from the `*.exe` rule), the pack builder refuses to zip without it,
  and any fresh clone can build for Windows out of the box.
- **Fresh clones couldn't build the native Linux packages.**
  `packaging/native/fetch-stack.sh` — which `build.sh` calls directly —
  was committed without its executable bit, so on a fresh clone the
  build silently skipped the native-window apps and shipped only the
  web-edition packages (the release machine had a local `chmod`, which
  is why the v0.7.5 artifacts came out fine). The bit is in the repo
  now; `build-offline-pack.sh` gets its bit back too. Mode-only
  change, no script text touched.
- **The About sheet showed a stale version** — "1.6.3" in a 1.6.5 app
  (the same bug the v0.7.3 pass fixed when it was "1.5.0"). The number is
  no longer typed into the frontend at all: the sheet asks the backend for
  it (the `GetAppInfo` binding on Windows, `/api/appinfo` on the Linux
  web edition), and in a plain browser preview — which has no host — the
  version line is simply hidden rather than inventing a number.

### Changed

- **The connected/disconnected flip is one motion now.** The dotted rings
  used to pop (one red ring vanished, five green arcs appeared in the same
  frame); both sets now cross-fade over 250 ms while the family drifts to
  its new center, and the five arcs pulse as an outward ripple instead of
  one flat group pulse. The big status word ("Not Connected" →
  "Connecting…" → "Connected") rises in softly instead of snapping.
- **Dragging the pill past halfway now previews the outcome.** The LED on
  the thumb lights green while a connect is armed (or dims back to red
  while a disconnect is), and the chevron in the direction of travel
  brightens and pauses its idle nudge — release-to-commit is visible
  before you commit. The thumb also presses down (scale 0.98) under a
  finger or mouse, and the chevrons breathe gently toward the swipe
  direction while idle.
- The error detail line ("Could not connect. Try again.") is now the
  approved-for-text red (`--red-400`) instead of secondary grey — red is
  for real errors, per the design language.

### Added

- The menu sheet now behaves like a proper modal: focus moves in when it
  opens (already true), **Tab and Shift+Tab wrap inside it** (previously
  Tab leaked to the dimmed screen behind), and focus returns to the menu
  button on close (already true).

All new motion stops under `prefers-reduced-motion`, verified: every
animation collapses to 0.01 ms and the app remains fully usable.

## [v0.7.5] — 2026-10-16 (app 1.6.5)

The "offline means offline" release: the offline pack from v0.7.4
promised "no internet needed" but died on machines without access to
Go's module proxy — this one keeps the promise.

### Fixed

- **The offline pack now actually works offline.** `wails dev` and
  `wails build` always run `go mod tidy` before compiling, and tidy
  ignores the packed `vendor\` folder — it downloads the dependency
  list from the internet first. On a filtered connection that meant
  `403 Forbidden` from proxy.golang.org and the app never started.
  Every wails call in the setup and build scripts now passes `-m`
  (wails' "skip mod tidy" flag): the dependencies ship inside the
  pack, so there is nothing to download, ever.
- **`'powershell' is not recognized`** in the offline setup: the
  optional "remember the tools for future windows" step called bare
  `powershell`, which fails on systems whose PATH lacks the
  WindowsPowerShell folder. It now calls PowerShell by its full
  Windows path, silently, and can never stop the setup.
- The offline setup's "The app window - 360 x 760" message was stale:
  it now says 360 x 628, the window the redesigned screen actually
  opens since v0.7.3.

### Changed

- `build.bat`, `build.sh` and `setup.ps1` build with `-m` too — the
  tidy step added a network round-trip and zero value (go.mod and
  go.sum are complete and vendored), so builds are faster and never
  surprise-download on filtered networks.

## [v0.7.4] — 2026-10-16 (app 1.6.4)

The two-editions release: every platform now ships **Younblock** (the
redesigned connect screen) and **Younblock Classic** (the simple v0.6.0
UI you knew) side by side — and on Linux the AppImage and .deb finally
**open as their own desktop window** instead of a browser tab.

### Added

- **Younblock Classic editions** (D66): `younblock-classic.exe` (64/32-bit
  Windows) and `younblock-classic_<ver>_amd64.deb` /
  `Younblock-Classic-<ver>-x86_64.AppImage` (Linux). Same engine, same
  settings, ~8.5 MB downloads — the small simple version is back. The
  browser (single-executable) editions carry both screens: add
  `?ui=classic` to the URL to switch.
- **Native window editions for Linux** (D67): the 64-bit .deb and AppImage
  are now real GTK + webkit2gtk desktop apps with their own taskbar icon
  and window — no browser tab. The .deb pulls webkit from your Mint
  repos (apt keeps it updated, ~10 MB download); the AppImage bundles
  the whole webkit runtime so it runs on any glibc ≥ 2.35 desktop even
  with nothing installed. Built against Ubuntu 22.04 webkit2gtk-4.1, so
  every Mint since 21 is covered.
- The web edition's `?ui=classic|pro` switch (both screens are in the
  bundle; the URL picks).

### Changed

- The window's Linux close button now quits the app (stopping the engine
  and restoring the proxy first) — there is no tray to hide into there;
  on Windows the close-to-tray behavior is unchanged.
- Opening the logs folder uses the desktop's file manager on Linux
  (xdg-open) instead of trying Windows Explorer.
- build.bat now produces the classic exes too; build.sh builds the full
  two-edition matrix for both platforms.

### Fixed

- The AppImage on Linux Mint no longer opens in a web browser — it opens
  as an application window (the single-executable 32-bit line keeps the
  browser mode for old 32-bit Mint, where the native stack cannot run).

### Compatibility notes

- The 32-bit Linux editions (.deb i386, i686 AppImage, younblock-linux-386)
  remain browser editions: old 32-bit Mint (glibc 2.31) cannot run the
  native stack. 64-bit Mint 21/22/23 users get native windows.
- The Flatpak stays at 1.6.3 this release (the sandbox lost its flatpak
  stack again; the app code it wraps did not change). It keeps the
  browser mode for now; a native flatpak is on the list.

## [v0.7.3] — 2026-10-16 (app 1.6.3)

The proportion pass: the window is shorter and better balanced — the
same connect screen, the same engine, ~17% less height. Nothing about
how it works changed.

### Changed

- **The window is now 360×628 inside (≈388×650 with the title bar),
  down from 360×760 (≈388×782).** The screen keeps its dark theme,
  red START button, YouTube branding, dotted background and rounded
  shapes — every font size is unchanged; only the spacing and the two
  big shapes were retuned.
- **A deliberate vertical rhythm instead of stacked islands** (D65):
  tighter top padding above the header (50→38 px), the YouTube play
  illustration 15% smaller, the status group pulled closer to it, the
  power-button assembly 12% shorter (track 228→200 px, thumb 124→110
  px — the 82 px width and the touch target are untouched), and the
  dotted ring rescaled to hug it (124→106 px radius; the five
  connected arcs −17%).
- **The bottom hint ("Click or swipe down to connect") moved up** to a
  consistent 26 px below the button instead of floating 72 px lower,
  pushed toward the window edge. Horizontal padding is now a uniform
  20 px.
- `PowerPill` now reads the swipe travel from `tokens.css` (one
  source of truth) instead of hard-coding it beside the token.

### Fixed

- **Opening the menu could shove the whole screen upward for a
  moment.** The sheet lives inside the scaled stage, and focusing its
  close button while the sheet was still sliding up made the browser
  scroll the stage to chase the button. The focus call now explicitly
  forbids that scroll. (Visible at the new window height; barely
  visible before.)
- The About sheet showed "Younblock 1.5.0" since the new screen
  landed — the version string was hard-coded and never bumped with
  the app. It now says 1.6.3.

## [v0.7.2] — 2026-10-15 (app 1.6.2)

The "filtered network" release: the one-click setup survives blocked
download sites, and there is now a fully offline pack (D63, D64). The
app itself is unchanged from 1.6.0 — same connect screen, same engine.

### Added

- **Mirrors for every dependency.** The one-click scripts try several
  mirrors per download and use the first one that answers: three Go
  module proxies (proxy.golang.org, goproxy.io, goproxy.cn — the chain
  is remembered so `build.bat` benefits too), two Go toolchain mirrors
  (Aliyun, go.dev) when winget cannot deliver Go, two Node.js mirrors
  (nodejs.org, npmmirror) when winget cannot deliver Node, and two npm
  registries (npmjs, npmmirror — the mirror is the automatic retry).
  winget is still tried first for everything it can install.
- **The code has three ways to arrive**: a normal git clone (GitHub's
  browser login); the same clone with a GitHub token pasted into the
  `$GH_TOKEN` line at the top of `setup.ps1` (no prompts at all, and
  the token never lands in `.git/config`); or, when git itself cannot
  be installed, a direct zip download from the GitHub API — the script
  offers the token prompt at that point.
- **`Younblock-offline` — the no-internet pack.** One zip carrying the
  app, its Go dependencies, the window building blocks, the Go
  toolchain, Node.js and the Wails CLI. Extract it, double-click
  `setup-offline.bat`, done — nothing is downloaded, ever. It also
  builds the exes (`setup-offline build`).
- The Go dependencies are now **vendored in the repository**
  (`vendor/`), so building and testing never touch a module proxy.

### Changed

- The setup became a pair: the `.bat` files you double-click, and one
  `setup.ps1` holding the whole brain — they travel together in the
  new `younblock-setup` zip on the Releases page (D62 refined).
- The release ships 12 files: the usual ten builds, the
  `younblock-setup` zip and the offline pack.

## [v0.7.1] — 2026-10-15 (app 1.6.1)

The "one-click" release: two scripts that set up a Windows PC by
themselves (D62). The app itself is unchanged from 1.6.0 — same connect
screen, same engine; only its version number moved.

### Added

- **`setup-dev.bat`** — double-click it on any Windows 10 1709+ / 11 PC
  and it installs whatever is missing (Git, Go, Node.js LTS, the
  WebView2 part of Windows, the Wails CLI), fetches the code, and
  starts the app straight from source (`wails dev`) — the 360×760
  window opens by itself. Safe to run again any time: it skips what
  is already installed and updates the rest.
- **`setup-build.bat`** — the same one-click checks, then it builds
  every `.exe` (the app and the headless engine, 64- and 32-bit),
  prints each file's SHA-256 and opens the `dist` folder with the
  finished files.
- Both scripts also fix the usual first-run problems by themselves:
  the search path is reloaded right after an install (no "open a new
  window and try again" step), a too-old Node.js or Go is refused
  with a plain fix-it note, and the private-repo login is explained
  when the code is first downloaded.
- `.gitattributes` now forces Windows (CRLF) line endings on `.bat`
  files, so both scripts always arrive exactly double-clickable.

## [v0.7.0] — 2026-10-15 (app 1.6.0)

The "new face" release: the main screen is replaced by the new connect
screen — one button, one glance (D60, D61).

### Changed

- **The whole app is now the connect screen** (D60): a single power
  pill (START / CANCEL / STOP / WAIT) on a 360×760 stage — a red ring
  while off, five green arcs while on — with the connection timer, live
  download/upload speeds and the ping beside it. The pill can be
  clicked, dragged past halfway to connect or disconnect, or driven
  with ArrowDown / ArrowUp / Enter; pressing it while "Connecting…"
  cancels. The window grew from 360×560 to give the new screen its
  stage (the D48 client-size fix travels with it).
- **The menu is a small sheet now**: About, How it works and Help live
  behind the menu button; Escape closes it. The separate Settings,
  Help, About and First-run screens — and the i18n layer only they
  used — are gone (recoverable at tag v0.6.0). The engine still reads
  the saved settings file, and the Linux JSON API still exposes them;
  only their editing screen is gone.
- **The tray and connect-on-launch now route through the new state
  bridge**, so the tray icon, the window and the screen can never
  disagree about whether the app is on.
- **The Linux editions keep their real engine** (D61): the new screen's
  one backend seam (`backend.ts`) gained back the localhost JSON +
  Server-Sent-Events transport, so `younblock-linux-*`, the .debs, the
  AppImages and the Flatpak drive the same engine as before, with the
  same state mapping as the Wails bridge (degraded counts as connected;
  PORT_BUSY and friends surface as "Can't Connect" with a retry). The
  ping is re-read once a second while connected — the same cadence as
  the Windows side.

### Added

- The Sora variable font ships with the app (the new screen's
  typeface); no flags, no "Go Premium", no location chips anywhere.

## [v0.6.0] — 2026-10-15 (app 1.5.0)

The "polish" release: a dedicated UI/UX and look-and-feel pass over the
whole 360×560 window — every screen and all six states — with no engine
or network changes (D56–D59).

### Changed (the UI/UX and look & feel pass — D56–D59)

- **One screen chrome**: Settings, Help, About and FirstRun mount on a
  shared stylesheet (header, back button, scroll body, section captions,
  cards, reveal) instead of three drifted copies; a real type ramp
  (11 px overline captions, 17 px view titles, mono for addresses and
  URLs) and a duration scale so every motion in the app shares one clock.
- **One control standard**: the Advanced radios and the FirstRun
  checkbox are drawn in the brand style (a ring filling with the
  padlock red) instead of native accent-colored dots beside the custom
  switch; every touch target is ≥44 px (the Add-provider chips and the
  copy-address button were under); the port field gets hover/focus
  states; "tried first" is a small gold badge; secure-DNS URLs and the
  domain list use a mono face.
- **Motion, everywhere, still honest**: screens enter with one
  rise-and-fade, the welcome staggers in, the menu unfolds from its
  corner, the close-notice sheet rises, Advanced / licenses / the Help
  accordion expand with the same smooth reveal, Run-test carries a
  spinner and the running test shows a quiet "Checking…" row. Purely
  decorative motion switches off under reduced-motion settings;
  functional spinners keep turning (D45).
- **The stopping state says what it is doing** ("Putting the internet
  setting back…"), the state subtitles come from the copy deck like
  every other word, and the degraded state's button shrinks the same
  step the error state always had — its "Fix connection" stack now
  keeps comfortable room above the footer instead of landing exactly on
  the minimum.

### Fixed

- **The open menu could be unclickable** right after a screen appeared:
  a filled entrance animation left a lingering (identity) transform on
  the header, which silently repainted the menu panel under the page
  content. One-shot animations now end without keeping transforms.
- **Keyboard focus reshaped pill buttons**: the global focus ring
  carried a border-radius that squared the Fix-connection pill and the
  chips for as long as the ring showed; the ring now only paints.
- **Hovering the On button grayed its rim**: the hover rule outranked
  the lit state's border. Hover now applies to the resting states.
- **The About diagram was below the fold** — under the numbered steps
  at exactly the window's bottom edge, where nobody met it. The picture
  (browser → Younblock → YouTube) now leads the card.

## [v0.5.1] — 2026-10-14 (app 1.4.1)

The "install it properly" release: the three standard Linux Mint package
formats, wrapping the same static binary as the plain-file edition (D53).

### Added

- **.deb packages** (amd64 + i386): `/usr/bin/younblock`, menu entry, nine
  hicolor icon sizes, `Depends: libglib2.0-bin` + `Recommends: libnotify-bin,
  xdg-utils`, postinst/postrm cache refresh, md5sums.
- **AppImages** (x86_64 + i686): single-file, portable, download-and-launch;
  AppRun forwards to the binary with `--web`; built FUSE-free.
- **Flatpak bundle** (x86_64): manifest + metainfo for
  `io.github.assadigit.Younblock` on `org.gnome.Platform/48`.
- Native icon set regenerated from the logo at 4096 px and exported to all
  sizes 16–512 under both icon names.
- `packaging/` tree with one build script per format; `build.sh` now cuts
  all six binaries and all four packages in a single run.
- Autostart resolution per format (D54): `$APPIMAGE` names the AppImage
  file, `$FLATPAK_ID` runs `flatpak run <id>`, passwd home under Flatpak;
  four new unit tests.

### Fixed

- Autostart entries written from inside an AppImage or Flatpak now survive
  the package's own launch mechanism (previously they pointed at the
  extracted temporary path).

### Known limits (honest list)

- `flatpak run` could not be executed in the build container (nested pidns
  restriction); the bundle's contents, install, live API and autostart entry
  were verified, and launching on real Mint is README manual-check step 8.
- Package wrapper files embed build timestamps; only the inner binaries are
  byte-reproducible.

## [v0.5.0] — 2026-10-14 (app 1.4.0)

The "same app for everyone" release: 32-bit Windows and the Linux Mint
edition, all built from the one source tree (D52).

### Added

- **32-bit Windows editions**: `younblock-x86.exe` and
  `younblock-core-x86.exe` (PE32 i386).
- **Linux edition**: `younblock-linux-amd64` / `-386`, static ELF with the
  UI embedded (`go:embed`), loopback-only JSON API + SSE events, `--web`
  mode on port 17500 with free-port fallback; works on Windows too.
- Linux platform adapter: gsettings system proxy with backup/restore,
  EnsureAttached anti-clobber re-assert, autostart `.desktop`, notify-send
  notices, capability probing (D50, D53 groundwork).
- The frontend auto-detects its edition (Wails → HTTP → mock) and adapts
  copy ("Linux" word swap, no-tray first-run text, capability-gated Help
  rows for Chrome-session features).

### Changed

- **StatusBlock error state redesign (D49)**: the failure reason is now the
  subtitle directly under the state label, where the eye lands first; the
  old duplicate low paragraph and its dead gap are gone.

### Fixed

- 32-bit build correctness: `parseDword` 1<<31 signed-int overflow replaced
  with unsigned arithmetic.

## [v0.4.1] — 2026-10-14 (app 1.3.1)

Fixed the "white space" and "Fix connection is on the edge" reports.

### Fixed

- **Window sizing under DPI scaling (D48)**: the outer window is now grown
  at startup so the client area is a true 360x560 logical pixels at any DPI
  (previously ~336x522 at 125%).
- **Overflowing warning states (D48)**: the footer is pinned with real
  margins, the fix-connection button always sits ≥15 px above it, the timer
  block collapses smoothly (0fr↔1fr grid) in states that never show it, and
  long error text gets its own 100 px scroll cap. Header slimmed 52→44 px;
  response-ladder (button scale-down, then zoom) for very small viewports.

## [v0.4.0] — 2026-10-14 (app 1.3.0)

The "every browser really uses it" release, plus the logo palette.

### Changed

- **System proxy rewritten to manual values (D46)**: ProxyEnable /
  ProxyServer / ProxyOverride with a v2rayN-style bypass list instead of the
  registry PAC (AutoConfigURL), which real-world browsers and tools quietly
  defeat. Any AutoConfigURL is cleared while active; legacy orphaned PAC
  entries cleaned at startup; WM_SETTINGCHANGE broadcast; watchdog
  EnsureAttached re-assert with a PROXY_REAPPLIED notice.
- **Logo palette everywhere (D47)**: warm charcoal neutrals, padlock-red
  primary/on-state, lightning-gold accents (warnings, focus, echo rings, OK
  checks); danger keeps its distinct coral. No green anywhere; tray icons
  regenerated to match.
- YouTube mode fragments only routed hosts (unfragmented relay otherwise);
  plain-http to non-routed hosts is forwarded as a real forward proxy so
  http-only sites keep working.

## [v0.3.0] — 2026-10-14 (app 1.2.0)

The power-button animation fix and professional UI polish.

### Fixed

- **Animation freezing under reduced motion (D45)**: Windows mirrors
  "system animations off" (and RDP) into WebView2; the blanket kill-rule
  froze the connecting arc mid-circle. Replaced with a per-component policy
  — decoration off, functional progress indicators and color fades stay.

### Changed

- Orchestrated turn-on: spring pop, staggered echo rings, crossfaded lit
  plate, slow glow breath; press-release hardening; refined connecting arcs.
- StatusBlock: state-colored status dot, typographic rhythm, amber ghost
  "Fix connection" pill (reads as a warning repair, not a second on-button).
- Connect footer badge chip + "works with every browser" micro-caption;
  copy updated across First-run / Help / About.

## [v0.2.1] — 2026-10-14 (app 1.1.1)

Hotfix after a "didn't even connect" report on 1.1.0.

### Fixed

- Degraded-from-start now actually starts the stats loop, watchdog and
  network monitor — the timer runs and the app self-recovers (R14).
- The state JSON carries `fail` (step + code, root-cause ordered); the
  degraded subtitle says which part failed; "last failed check" added to
  the diagnostic report.
- A changed secure-DNS list swaps the resolver live (previously kept until
  restart).

### Added

- Editable Advanced DoH list: remove buttons, one-tap add of
  AdGuard/Quad9/dns.sb/defaults, new provider tried first, keep-at-least-one
  guard.

## [v0.2.0] — 2026-10-14 (app 1.1.0)

Ali's five post-test requests; all default-off or additive.

### Added

- Route-all ("All websites") switch surfaced as a top-level setting.
- Dedicated Chrome session option (`openBrowserOnConnect`, default off) —
  neutral browser package, Windows shell launcher with registry +
  standard-path lookup, session ends on off/error/quit.
- Background texture: two soft brand glows + subtle SVG noise grain, inline.
- Response time ("ping"): YouTube round-trip shown under the timer and in
  the diagnostic report; resets on stop.
- Ali's icon.ico installed as the exe/window icon.

## [v0.1.0] — 2026-10-13 (app 1.0.0)

First Windows test build, published to GitHub Releases.

### Added

- Headless engine: fragmenting CONNECT proxy (routed hosts only), secure
  DNS-over-HTTPS resolver with cache, health self-test with self-healing
  watchdog, ring + file logging, JSON settings.
- Windows adapters: system proxy with backup/restore state machine,
  autostart registry entries, toast notices.
- Wails app: 360x560 window, tray with state icons, First-run, Connect /
  Settings / Help / About screens, i18n-ready copy.
- `younblock-core.exe` headless CLI (`--selftest`, `--version`) and the
  diagnostic report generator.

[Unreleased]: https://github.com/assadigit/Younblock/compare/v0.7.0...HEAD
[v0.7.0]: https://github.com/assadigit/Younblock/releases/tag/v0.7.0
[v0.6.0]: https://github.com/assadigit/Younblock/releases/tag/v0.6.0
[v0.5.1]: https://github.com/assadigit/Younblock/releases/tag/v0.5.1
[v0.5.0]: https://github.com/assadigit/Younblock/releases/tag/v0.5.0
[v0.4.1]: https://github.com/assadigit/Younblock/releases/tag/v0.4.1
[v0.4.0]: https://github.com/assadigit/Younblock/releases/tag/v0.4.0
[v0.3.0]: https://github.com/assadigit/Younblock/releases/tag/v0.3.0
[v0.2.1]: https://github.com/assadigit/Younblock/releases/tag/v0.2.1
[v0.2.0]: https://github.com/assadigit/Younblock/releases/tag/v0.2.0
[v0.1.0]: https://github.com/assadigit/Younblock/releases/tag/v0.1.0

### Added (Android port, M7 — not yet a released build)

- **The NO_TARGET_APPS typed error (D81)**: when none of the
  allow-list's apps is installed, the connect attempt now fails
  honestly instead of silently returning to "Not Connected" — the
  engine lands in its error phase with the typed code and the page
  shows the reason ("None of the apps on your list is installed.
  Install one, or edit the list in the menu."). The refusal travels
  the existing start gate with atomic delivery (the shell's re-arm
  can never claw it back) and a whitelisted code vocabulary.
- **Run generations (D82)** — a hardening fix found by the new tests:
  a connect goroutine that lost the race with its own Stop could
  previously register a fresh uncancellable context and resurrect a
  phase after the cancellation. Every cancel now supersedes spawned
  runs; one existing test had been passing through exactly this bug
  and was corrected to its documented intent.
- **The -race lane**: `go test -race` runs clean on the seven key
  packages (engine, android adapter, mobile, tunstack, proxy, webui,
  config).
- **Audit + attribution**: the AAR's dependency closure verified
  (gVisor netstack Apache-2.0, gomobile BSD-3, golang.org/x BSD-3,
  google/btree Apache-2.0 — all permissive, licenses vendored); the
  About sheet credits the open-source components. The built APK's
  permission set verified to be exactly the documented inventory.
- The v1 release-notes draft (docs/android-port/10-release-notes.md)
  with the truthful-claims review and the open device gates listed.
- The M7 debug APK (35,298,149 B, SHA-256 f27b7eef…491) is recorded in
  docs/android-port/05–06; battery/behavior, reboot/revocation and all
  live-observation gates remain ⛔ (no /dev/kvm, no device) and are
  carried, never claimed.

### Changed (Android port, M8-a — Ali's two product rulings, D83 — not yet a released build)

- **The apps allow-list has NO default (Ali's ruling)**: the list
  starts empty and nothing is routed until the user adds an app. The
  M6 "empty restores the built-in Chrome+YouTube default" rule is
  gone — the M7 NO_TARGET_APPS refusal door is what makes the empty
  list safe, and it now also covers the empty state (the connect
  fails with the typed reason and an honest "add an app" message,
  instead of the old silent stop that treated an empty list as a
  bridge bug). The UI drops the last-entry guard and shows an
  empty-state line. Existing saved lists are kept — the ruling
  changes the default, never user data.
- **Websites ending in .ir never touch the app (Ali's ruling)**: in
  every mode and both editions, .ir hosts are never fragmented, never
  redirected — resolved and dialed direct, byte-transparent. The
  desktop PAC answers DIRECT for them first (youtube mode already
  behaved this way by fallthrough; **"All websites" mode is a real
  behavior change** — it previously sent .ir through the fragmenting
  path). The Android SOCKS5 door makes the same decision, and the
  rule wins even over a .ir entry on the user's own domain list.
  Honest limit, stated in-app: on Android, .ir name lookups still
  resolve through the configured secure-DNS providers (the app has no
  other resolver there).
- **The manifest's `<queries>` block broadened to the launcher-intent
  form** — a latent bug the ruling exposed: the old block listed only
  the two former default packages, so any user-added package would
  have been invisible to PackageManager (Android 11+ package
  filtering) and refused as "not installed" even when installed. Every
  launchable app is now visible; non-launchable packages cannot be
  verified and are refused honestly.
- The M8-a debug APK (35,567,561 B, SHA-256 ccf46a2e…7b64) with both
  rulings applied is recorded in docs/android-port/05–06; the standing
  device gates remain ⛔ (no /dev/kvm, no device) and are carried,
  never claimed.

### Added (Android port, M8 — package, verify, hand off, D84 — the debug test circle)

- **The first distributable Android build** — the debug-APK-only test
  circle (Ali's keystore ruling, D84: "keep it debug version… we're
  not releasing yet. but release in github for me to download and
  debug"): no signed variant exists at this stage; the debug APK is
  published as the GitHub prerelease **v1.6.6-android-debug.1** on the
  private repo, downloadable with Ali's own GitHub account for device
  testing. The artifact is the M8-a clean-build output — M8 changed no
  application code — re-verified on disk before distribution:
  `app-debug.apk`, 35,567,561 B, SHA-256 ccf46a2e…7b64; apksigner
  v2-scheme OK (debug certificate, digest recorded); badging
  io.github.assadigit.younblock 1.6.6 (versionCode 1), minSdk 26 /
  target 36, native-code arm64-v8a/armeabi-v7a/x86_64; the manifest's
  four-permission inventory and launcher-intent `<queries>` verified
  in the shipped file.
- **The release carries the device test circle** (doc 10): the
  checklist of everything the build environment could never verify
  (no /dev/kvm, no device — install/first-launch, the live consent
  dialog, notification behavior, screen-off/Doze, real DoH, the
  YouTube app's QUIC→TCP fallback, protect() on real sockets, the
  .ir direct path on a real radio, stop/reboot paths, battery) —
  handed to Ali's device through the release notes, never claimed
  here. His observations are the feedback loop that closes those
  gates and drives any v1.6.6-android-debug.2 fix build.
- M8's verification record: the baseline gate (vet clean, 17/17
  packages twice, svelte-check 0/0, vite clean, webui dist synced),
  the dependency-closure re-run (the same audited permissive set),
  the platform-requirements recheck against developer.android.com
  primary sources, three of doc 07 §9's release-blocking rows closed,
  and the README's Android sideload section (the unknown-apps + Play
  Protect + built-for-testing warnings, all expected for this circle).
  **Every future debug build bumps versionCode** so it installs over
  the previous one (same debug certificate) — the D84 rule.

### Added (Android port — the device-test feedback loop, session 1, D85)

- **Ali's first device observation ingested**: "Android didn't let me
  install it" from v1.6.6-android-debug.1. Diagnosed before any fix:
  the served release bytes re-downloaded and verified byte-identical
  to the record (sha256, apksigner v2, badging, manifest — no
  `testOnly`, `debuggable` is normal for debug; zip intact), and the
  asset's download_count was 0 — pointing at an interrupted download
  from the Iranian network (release assets are served from
  release-assets.githubusercontent.com) rather than an APK defect.
- **The install-blocked troubleshooting path is now on the live
  release body** (PATCHed via the API) and doc 10 §1: check the file
  size first (≈ 35,567,561 B; a few-KB file is an error page), retry
  on Wi-Fi or download on a PC + USB; then the install-unknown-apps
  gate; then Play Protect's "install anyway"; then the parse-error and
  storage branches.
- **The debug certificate is pinned in the repo (D85)**:
  `android/app/debug.keystore` — the exact key that signed
  v1.6.6-android-debug.1 — is vendored and wired as the debug
  signingConfig, so successive debug builds keep installing over the
  previous one even after sandbox wipes regenerate the machine-local
  ~/.android/debug.keystore. Verified: clean assembleDebug signs with
  the same certificate digest (c76bb7c6…8551d) and is byte-identical
  to the published release. No debug.2 yet — the artifact itself is
  unchanged; the next release rides on the next code-changing
  observation.

### Changed (Android port — the device-test feedback loop, session 2, D86 — v1.6.6-android-debug.2)

- **The YouTube app is the default allow-list entry (D86, Ali's
  ruling)**: a fresh install routes the YouTube app out of the box —
  debug.1's born-empty list (D83) is superseded. A saved empty list
  still means empty: removing the last entry is a user choice that
  persists, and the connect refuses with the honest NO_TARGET_APPS
  reason instead of restoring anything.
- **One-time settings migration (schema v3)**: a v1/v2 settings file
  whose apps list is absent or empty migrates to the YouTube default —
  that empty was the shipped default of the debug.1 era, never a user
  choice (its entry path refused friendly names, so "empty" almost
  always meant "never successfully configured" — the exact state
  behind Ali's "the first version didnt work, whether in youtube app
  or youtube in chrome"). Custom lists survive verbatim; a v3 empty
  list is the user's own and stays. Updating installs over debug.1
  (versionCode 2, same pinned debug certificate) with settings intact.
- **Adding apps is now usable**: the Apps-to-unblock row gains one-tap
  quick-add chips for the known apps (+ YouTube, + Chrome) and the
  type-in field accepts friendly names — "youtube" and "chrome"
  resolve to their packages; other apps still need their Android
  package name. The engine's saved response remains the validation
  authority.
- The honest copy updated wherever it claimed the list "starts empty"
  (menu sheet, How-it-works, the refusal message, README, release
  notes), and the release notes' stale default DoH provider list
  corrected to the code truth (RethinkDNS, Cloudflare, Google).
- Root-cause record: the engine's routing chain (virtual DNS, fake-IP
  reversal, the SOCKS5 door, per-app TUN establish) was audited
  end-to-end and found sound — the failure debug.1 showed on-device was
  the empty list plus the package-name-only entry path, not the engine.

### Changed (Android port — the device-test feedback loop, session 3, D87 — v1.6.6-android-debug.3)

- **The default allow-list is five apps (D87, Ali's ruling — "Add
  chrome to the default addlist as well, also firefox, opera, brave
  browser")**: the YouTube app plus the Chrome, Firefox, Opera and
  Brave browsers (the stable mainline package names:
  com.android.chrome, org.mozilla.firefox, com.opera.browser,
  com.brave.browser). A fresh install routes every installed one of
  them out of the box. Entries whose apps are not installed are
  skipped at TUN establish — a missing browser can never break the
  connect (audited and recorded before shipping; the establish loop
  has caught NameNotFoundException per entry since M6). A saved empty
  list still means empty, as before.
- **One-time settings migration (schema v4)**: a v1/v2 born-empty
  file fills the five-app default directly; a v3 file still holding
  exactly the debug.2 default [com.google.android.youtube] — the
  shipped state of that build, never curated — upgrades to the five
  apps; a v3 empty list (user agency) and every custom list survive
  verbatim (including a v2 file whose [youtube] entry was hand-typed
  under debug.1's refusing entry path). Updating installs over
  debug.1/debug.2 (versionCode 3, same pinned debug certificate) with
  settings intact.
- **The quick-add chips cover all five defaults** (+ YouTube /
  + Chrome / + Firefox / + Opera / + Brave — rendered when not
  already listed), and the friendly-name aliases extend to
  "firefox", "opera", "brave" (and "brave browser"); other apps
  still need their Android package name, and the engine's saved
  response remains the validation authority.
- The honest copy updated everywhere the default list is named
  (menu intro, empty state, note, How-it-works, the NO_TARGET_APPS
  message, README, doc 07 §1, doc 10 / the release notes) — and the
  release guide's expected file size updated to this build's exact
  artifact size.

### Added (Android port — the release-signing helper, D88; no app change, v1.6.6-android-debug.3's APK unchanged)

- **`make-release-keystore.bat` — the local release-keystore ceremony
  in one double-click** (Ali's ask; the security boundary kept: the
  password is generated on his machine with Windows' own crypto RNG
  and never transits chat, agent or repo): locates keytool, generates
  a 32-character zero-bias password, creates the keystore with every
  keytool prompt pre-filled (RSA 4096, PKCS12, 30-year validity — the
  "serial number and etc" interactive ceremony eliminated), verifies
  it by reading it back, and writes a
  `younblock-keystore-backup_<date>.md` recording the password, the
  alias, the certificate serial number, the SHA-256 fingerprint and
  the full keytool record. Refuses to overwrite an existing keystore;
  warns when run inside a git repo; fully offline; the .gitignore
  guards the generated files. The keytool flow was verified against
  the real JDK 21 keytool on the build host; the Windows batch
  execution itself stays ⛔ there (no Windows) — the script
  self-verifies on its first run. Attached to the
  v1.6.6-android-debug.3 release as a second asset and committed at
  the repo root; the README's Android section now documents the
  signed-lane steps it automates.

### Changed (Android port — the release-flag ruling, D89; no app change, the APK bytes untouched)

- The three android debug releases (v1.6.6-android-debug.1/.2/.3)
  are now regular GitHub releases, not prereleases (Ali's ruling: "Do
  not prerelease, just release, the repo is private" — the flag
  carried no public meaning in a private repo, and the Latest pointer
  now lands on the current test-circle build, debug.3). Switched and
  verified via the API; the debug.3 release notes gained one
  clarifying paragraph so the flag cannot be misread as a stability
  claim — it is still the debug-signed device-test build,
  collaborator-visible only; nothing about the artifacts changed.

### Fixed (Android port — the keystore helper survives PowerShell-less PATH machines, D90; no app change, the APK bytes untouched)

- `make-release-keystore.bat` no longer depends on the PATH variable
  finding PowerShell (Ali's first field report: "'powershell' is not
  recognized" — the ceremony stopped at password generation, exit
  code 4). PowerShell is now located by its full standard install
  path first — the same fix the desktop setup scripts took at v0.7.5
  — then through where.exe; and if PowerShell cannot be found at
  all, a pure-cmd fallback generator builds the password (six RANDOM
  draws, digits and dashes, ~90 bits) so the ceremony always
  completes. The .md backup and the final summary record which
  generator produced the password. The fixed script ships on a new
  helper-maintenance release, v1.6.6-android-debug.4 (a regular
  release, D89) — it carries only the .bat; the app is unchanged
  and debug.3's APK remains the current build (D84: identical bytes
  are never re-released), with a pointer note added to debug.3's
  page superseding its older helper asset.
- Also recorded: the answer to "can't I use an online keyring
  generator instead?" — no. An online generator would hand the app's
  signing identity and password to a third party; the D88 boundary
  stands (signing secrets are generated where they are used). A
  one-line manual keytool alternative is documented in the release
  notes for machines without PowerShell.

### Fixed (Android port — the first device feedback answered, D91; v1.6.6-android-debug.5, versionCode 4)

- **The connect screen's numbers get their spacing back** (Ali's
  device report: "the numbers in the app are too near each other, they
  need regular spacings"). Root cause: the redesigned screen's stage
  was a fixed 22.5 rem wide — correct for a 360 px viewport at the
  default font scale, but on narrower screens or with Android's
  font-size setting turned up it grew past the screen edge and pushed
  the timer and the two speed values against it (measured: 495 px of
  stage inside a 320 px viewport at 1.4× scale, the two speed values
  sitting 24 px apart with 29 px to the screen edges). The stage now
  never exceeds the viewport, the two speed values carry wider
  spacing that can never wrap or jam, and the timer's digits get a
  touch more air with tabular figures where the typeface offers
  them. Verified against the rendered app at both the worst case
  (320 px / 1.4×) and the designed case (360 px / 1×) before
  shipping.

### Added (Android port — per-flow tunnel timing in the diagnostics report, D91; same release)

- **Every tunnel now records its own phases in the log** so a field
  report can name WHERE time goes instead of guessing: `tunnel up
  dns=…ms dial=…ms` when a flow is established, `tunnel first-byte
  …ms` when the origin's first response byte arrives (the TLS
  handshake and response start, through the fragment path), and
  `tunnel done dur=…ms up=… down=…` with the flow's own byte totals
  when it ends. These lines ride the in-memory log ring into the
  redacted diagnostics report (Menu → Diagnostics → copy), with
  hostnames appearing only when the log-domains privacy setting is
  on — the same rule the failure-path logs already follow. This is
  the instrument for the round's open question: Ali's device shows
  the YouTube feed refreshing fast while video playback takes
  30 s–1 min+ to first show; the host-side suspects were each
  eliminated with evidence (including a new test pinning that
  non-DNS UDP gets an immediate ICMP port-unreachable — the QUIC
  fast-fail contract routed apps rely on), so the remaining suspects
  are device/network-side and the next diagnostics copy from a
  slow-video moment will name the phase.
- Also in this release: the Play Protect popup observed at install
  is documented as expected behavior for sideloaded debug-signed
  APKs (no defect, no change), and the release keystore Ali created
  is explicitly NOT used for the test circle — signing a test build
  with it would break the installs-over-the-previous update chain;
  it waits for the signed public release.
