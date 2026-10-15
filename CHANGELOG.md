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
