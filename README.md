# Younblock

Younblock is a small Windows app that helps YouTube load in your browser when your
network blocks it. It runs a little relay on your own computer: your browser sends its
traffic to it, it looks up YouTube's address through a secure DNS service, and it sends
the first "hello" message to YouTube in small pieces so name-based filters don't
recognize it.

**It is not a VPN.** It does not hide your IP address and adds no encryption of its
own. YouTube's own HTTPS still protects everything you send. Your internet provider
can still see which servers you connect to. It only changes how the first message is
sent, and it may stop working if the filtering on your network changes.

It works by setting the normal Windows proxy, so every mainstream browser — Edge,
Chrome, Firefox, Opera, Brave and the rest — picks it up with no extra setup (it is
tested most with Chrome). It works with one big button, tests itself, and repairs itself
when YouTube stops responding — and it always tells you what it changed.

## Android test build (debug, not a public release)

The Android port is done and packaged, and it works the same way: the same
engine, the same secure-DNS and record-splitting behavior, but routing **only
the apps on its list** — the YouTube app and the Chrome, Firefox, Opera and
Brave browsers are added by default (D87), and you add or remove apps in the
menu. It is **not a VPN**: it uses Android's
VPN slot as a local traffic-routing interface, hides no IP address and adds no
encryption. Every website ending in **.ir is always left completely alone** —
direct, untouched, exactly as if the app were not running.

For now this is a **debug-signed test build, not a release** ("we're not
releasing yet"): download `v1.6.6-debug21.apk` from the repo's
[Releases page](https://github.com/assadigit/Younblock/releases) — the
`v1.6.6-android-debug.21` release (a regular release in this private
repo, D89, and the repo's Latest; the APK asset's name carries the
build number, D103, so the downloaded file always names its build —
**make sure the filename says debug21**; it installs over any earlier
debug build in place; its notes explain the current test circle —
this build removes the FIRST VIDEO'S remaining start lag the debug.18
report measured: the engine now VERIFIES the video hosts' addresses
in the background (a fragmented hello exactly like a real
connection's, answered→tried-first for 15 min, silent→skipped with
strike escalation), a connection whose racing all failed gets ONE
more in-engine rescue round over the untried addresses before it
gives up, a stalled half-finished response is cut at 8 s on Android
(was 15 — three of those held the app's retry for nearly half the
minute), and the diagnostics report now measures the play→video time
by itself (an "auto-timeline" heuristic — the "I pressed play" /
"First frame" buttons remain the honest measurement)).
Sign in with your
GitHub account, then sideload it:

1. Allow installs from your browser app (Settings → Apps → special access →
   install unknown apps).
2. Open the downloaded APK; Android warns about unknown developers and that the
   app is built for testing — both expected for a debug build (Play
   Protect may show a scan warning too — also expected for a
   sideloaded debug-signed APK).
3. Install (it updates over an earlier debug build in place — same
   certificate), open Younblock, and connect: the YouTube app and the
   installed default browsers are already on the list. Requires
   Android 8.0 or newer.

The release's notes carry the full device test checklist (what to test and what
to report back), and the engineering record lives in
[docs/android-port](docs/android-port/README.md). Successive debug builds
install over this one.

The login gate planned at D94 was **cancelled at D97** (2026-10-18)
before anything was built — Younblock stays one free-for-all app: no
account, no login, no server. The design pack lives on in
[docs/auth](docs/auth/00-start-here.md) as the bannered record of the
cancelled feature (kept for history; the "no account, no data
collection" claims stay literally true, permanently).

**When a public release happens**, the APK gets signed with your own release
keystore — created and backed up locally, never committed. The repo ships a
one-double-click helper for the whole ceremony: `make-release-keystore.bat`
(attached to the `v1.6.6-android-debug.4` release — D90 fixed it after the
first field report: it no longer needs PowerShell on PATH, and it works even
when PowerShell can't be found at all). Put it in a private
folder — not inside the repo, not inside a synced/cloud folder — and
double-click it: it generates the password on your machine with Windows' own
crypto RNG, creates the keystore with every keytool question pre-filled (the
certificate serial number and SHA-256 fingerprint are recorded into the
backup automatically), and writes a `younblock-keystore-backup_<date>.md`
next to it. Never use an online keystore generator instead — the keystore is
your app's signing identity, and whoever holds it and its password can sign
updates as you; it should never pass through anyone else's server. Keep the
keystore and that backup in **two different safe
places** — losing either means the next signed release needs
uninstall-before-update. The full plan:
[docs/android-port/07-security-privacy-and-distribution.md](docs/android-port/07-security-privacy-and-distribution.md) §8.

**What's new in version 1.6.5** (the "offline means offline" release):

- **The offline pack finally keeps its promise.** The v0.7.4 pack said
  "no internet needed" but wails silently tried to download the
  dependency list before building — on a filtered connection that
  ended in `403 Forbidden` and the app never started. The pack now
  skips that step entirely (everything it needs is inside the zip),
  and the setup no longer depends on PowerShell being on PATH.
  If your offline pack printed `403 Forbidden` or
  `'powershell' is not recognized`, download this release's pack.

**What's new in version 1.6.6** (the "feel" pass):

- **The on/off switch is one smooth motion now.** The dotted rings
  around the button cross-fade between red and green instead of
  popping from one to the other, and the big status word
  ("Not Connected" → "Connected") rises in softly.
- **Drag the pill halfway and it shows you what happens next.** The
  thumb's LED lights green while a connect is armed — or dims back to
  red while a disconnect is — so you can see what releasing will do
  before you let go.
- **The About sheet can't show a stale version anymore.** It asks the
  app itself for the number instead of trusting one that could go
  stale between releases.
- The menu now traps Tab like a proper dialog, the error line is
  properly red, and every new motion switches itself off when the
  system asks for reduced motion.
- **Building it yourself, offline?** The pack in this release fixes a
  bug every pack since v0.7.4 had: the WebView2 bootstrapper file was
  missing from `vendor\`, so `wails dev`/`wails build` could not
  compile the Windows apps on a fresh machine. If your old pack
  printed `no matching files found`, this one works.

**What's new in version 1.6.4** (the "two editions, real windows" release):

> The full release history — every version, in plain language — now lives in
> [CHANGELOG.md](CHANGELOG.md). The notes below are the short version.

- **Two editions on every platform.** Every download now comes in two
  flavors: **Younblock** — the redesigned connect screen (the red START
  pill, the dotted rings) — and **Younblock Classic** — the simple,
  familiar UI from before the redesign, back by popular demand in a
  ~8.5 MB download. Same engine, same settings, both installable side
  by side. The one-file Linux edition carries both: add `?ui=classic`
  to its address to switch screens.
- **On Linux Mint the app now opens as a real window.** The 64-bit .deb
  and AppImage used to open a browser tab; they are now proper desktop
  apps with their own window, taskbar icon and menu entry. The .deb
  pulls its display engine from your Mint repositories (small
  download, apt keeps it fresh); the AppImage carries everything
  inside (~75 MB) so it runs on any Linux Mint since 21 with nothing
  installed. The 32-bit Linux files keep the browser style — old
  32-bit Mint cannot run the new one.

**What's new in version 1.5.0** (the "polish" release):

> The full release history — every version, in plain language — now lives in
> [CHANGELOG.md](CHANGELOG.md). The notes below are the short version.

- **The whole app got a look-and-feel pass.** Every screen now shares one
  design system — same headers, same cards, same captions, same motion —
  so Settings, Help, About and the first-run welcome feel like one app
  instead of four cousins. The technical options are drawn in the app's
  own style (the red padlock ring you know from the big button), and
  everything you can tap is comfortably sized.
- **The app moves, quietly.** Screens rise in, the menu unfolds, sections
  expand smoothly instead of snapping, and the connection test shows a
  spinner and a "Checking…" line while it works. If you turn Windows'
  animation effects off, the decoration stops with them — only the
  spinners that tell you something is happening keep turning.
- **The "Turning off…" moment explains itself**: while the app puts the
  internet setting back, it says so instead of showing an empty line.
- **Fixes you'd only notice when they bit you**: the menu could become
  unclickable right after a screen appeared (an animation side-effect —
  fixed); tabbing to the "Fix connection" pill used to square its
  corners while the focus ring showed (fixed); and hovering the On
  button no longer dulls its rim.

**What's new in version 1.4.1** (the "install it properly" release):

> The full release history — every version, in plain language — now lives in
> [CHANGELOG.md](CHANGELOG.md). The notes below are the short version.

- **Three proper Linux Mint downloads, in addition to the single file:** a
  **`.deb` package** (installs with the menu entry, icons and binary in the
  standard places — dpkg tracks it), a **portable `.AppImage`** (one file you
  make executable and launch; delete it to uninstall) and a **Flatpak bundle**
  (installs with `flatpak install`, runs in its own runtime). All of them wrap
  the exact same app — same button, same self-repair, same settings.
- **32-bit Linux downloads too**: the `.deb` and the AppImage each come in a
  32-bit build for an old 32-bit Mint.
- **"Open Younblock when Linux starts" now names the right launcher** for the
  format you installed: the AppImage's start entry points at the `.AppImage`
  file itself, and the Flatpak's points at `flatpak run Younblock` — so
  start-with-OS works from every edition, not just the plain file.

**What's new in version 1.4.0** (the "same app for everyone" release):

- **A Linux version, as one file.** `younblock-linux-amd64` (or `-386` for a
  32-bit Linux) is a single executable with no installer and nothing else to
  install: run it, and the same app — same button, same self-repair, same
  design — opens as a page in your browser. On Linux Mint it also sets the
  system proxy the Linux way, so every mainstream browser picks it up.
- **A 32-bit Windows version.** `younblock-x86.exe` is the full app (window,
  tray, everything) built for 32-bit Windows — for an older PC, or a 32-bit
  VM. It also runs fine on 64-bit Windows.
- **Every version is built from the same source at the same time.** The
  Windows app, the 32-bit app and the Linux app are not separate copies that
  can drift apart: each release is cut from one source tree, so whatever we
  fix or add lands in all of them together.
- **The "Couldn't turn on" screen says its reason right under the title**, in
  red, where the eye lands first — the reason used to sit in a separate small
  paragraph further down with a gap above it. Same message, better place;
  one dead paragraph removed.

**What's new in version 1.3.1** (after the "white space" and "Fix connection is on
the edge" reports):

- **The window finally has the room it was designed for.** The app's window was
  created slightly too small on the inside — the title bar and frame were eating
  part of the space the layout was drawn for. On some screens that left a strange
  empty gap in the middle of the window, and in the "Check needed" state the
  *Fix connection* button sat right on the bottom edge with the footer hidden
  below the window. The app now grows its window at startup so the inside is
  exactly the designed size, at any display scaling.
- **The layout can no longer overflow.** Even if the window is somehow smaller
  than designed (very high display scaling), the home screen re-budgets itself:
  the big button shrinks a step, the tall warning states make room, and the
  footer stays inside the window with breathing room — with a scroll safety net
  under everything so nothing can ever be unreachable.

**What's new in version 1.3.0** (after the "only the dedicated Chrome session worked"
report):

- **Every browser now really uses it — other Chrome profiles, Opera, Edge, Firefox,
  Brave.** Earlier versions told Windows about the app through a proxy *script* (a PAC
  file); some browsers never picked that up unless they were restarted, and other
  proxy tools could quietly take it over. The app now uses the plain Windows proxy
  setting (the same one tools like v2rayN use), which every mainstream browser follows
  immediately — including browsers that are already open.
- **It defends that setting.** If another program changes the Windows proxy while
  Younblock is on, the app notices within a couple of minutes, puts its setting back,
  and tells you it did.
- **A new look that matches the logo**: the greens are gone. The "on" button is the
  padlock red, the turn-on flash and the warning glow are the lightning gold, and the
  whole window uses the warm charcoal of the logo's shackle. The tray icons match too.
- **Browsing through the app got safer**: in YouTube mode your other traffic now just
  passes through the app untouched (it used to depend on the script), plain-http sites
  that don't support https keep working, and local network addresses (like your
  router's page) never go through the app at all.

**What's new in version 1.2.0** (after the "button animation stuck midway" report):

- **The power button's loading spinner no longer freezes when Windows' animation
effects are turned off** (or when Windows itself runs without animations, e.g. over a
  remote-desktop session). That setting used to pause the little amber arc mid-spin,
  which looked like the button was stuck — the arc now keeps turning because it tells
  you the app is working, while purely decorative motion still respects the setting.
- **Turning on is now a fully animated moment**: the button springs, two soft echo rings
  ripple outward, the green plate fades in and then gently breathes. The press itself
  feels snappier and can no longer stay half-pressed.
- **A more polished, professional look throughout the home screen**: a status dot next
  to the state word, a cleaner timer, a softer "Check needed" look (amber glow instead
  of a dashed ring), a warmer failure reason line, and a badge footer.
- **The footer now says "Works with every browser"** — and it's true: the app sets the
  Windows proxy, which Edge, Chrome, Firefox, Opera, Brave and the rest all follow. The
  wording everywhere else was updated to match (it is still tested most with Chrome).

**What's new in version 1.1.0** (asked for after the first real-Windows test):

- **Settings now has two new switches near the top.** "Send all browser traffic
  through Younblock" routes every website — not just YouTube — through the app and
  its secure DNS (this was already possible under Advanced; it is now one switch
  away). "Open a separate Chrome when it turns on" gives you the old app's
  behavior: a dedicated Chrome window with its own session, routed through
  Younblock, that closes again when you turn the app off.
- **A response-time number** ("84 ms · …") appears under the timer while the app
  is on. It is the real round-trip time of the app's own YouTube check — the same
  measurement the connection test uses — refreshed automatically while the app
  runs. It is a response time, not a network ping.
- **A textured background** — soft brand-colored glows and a fine grain instead
  of a flat dark panel.
- **The app icon is now the icon file you provided** (the exe, the window and the
  taskbar use it; the tray keeps its separate on/off/attention icons so it can
  show the state at a glance).

**What's new in version 1.1.1** (after a "won't connect" report on 1.1.0):

- **The "Check needed" screen now says which part failed**: "secure DNS isn't
  answering" (a different secure-DNS provider is the fix — see
  [If it won't connect](#if-it-wont-connect)) or "YouTube isn't responding"
  (the Fix connection button tries every method). The diagnostic report also
  gains a "last failed check" line.
- **While it can't connect, the app keeps checking on its own and recovers by
  itself** as soon as the network allows it, instead of sitting frozen until
  the next button press. The timer keeps running too.
- **The secure-DNS server list in Advanced is now editable**: remove the
  providers that are blocked on your network and add another with one tap
  (AdGuard, Quad9, dns.sb, or the built-in ones back). The change applies
  immediately — no restart, even while the app is on.

---

## Download the ready-made app (no building needed)

The easiest way is to skip building entirely: go to the repo's **Releases** page on
GitHub and download the file for your machine from the latest release:

| File | For | What it is |
|---|---|---|
| `younblock.exe` | 64-bit Windows | the app you use every day (window, tray, button) |
| `younblock-x86.exe` | 32-bit Windows (or 64-bit) | the same full app, built 32-bit |
| `younblock-classic.exe` | 64-bit Windows | **Classic**: the simple pre-redesign UI, same engine |
| `younblock-classic-x86.exe` | 32-bit Windows | the same Classic app, built 32-bit |
| `younblock_…_amd64.deb` | 64-bit Linux Mint | the app as a proper package — opens in its **own window** |
| `younblock-classic_…_amd64.deb` | 64-bit Linux Mint | **Classic** as the same proper package |
| `younblock_…_i386.deb` | 32-bit Linux Mint | the browser-style package, built 32-bit |
| `Younblock-…-x86_64.AppImage` | 64-bit Linux | one portable file — opens in its **own window**, carries everything inside |
| `Younblock-Classic-…-x86_64.AppImage` | 64-bit Linux | **Classic** as the same portable file |
| `Younblock-…-i686.AppImage` | 32-bit Linux | the browser-style portable file, built 32-bit |
| `Younblock-…-x86_64.flatpak` | 64-bit Linux | the app as a Flatpak bundle |
| `younblock-linux-amd64` | 64-bit Linux (Mint 20+) | one file, runs as-is, opens in your browser (both screens: `?ui=classic`) |
| `younblock-linux-386` | 32-bit Linux (old Mint) | the same one file, built 32-bit |

(`younblock-core.exe` / `younblock-core-x86.exe` are the same engines without a
window, for testing and fixing — see below.)

That one file is the whole app — no installer, nothing else to download.

Two things to know about the downloaded file:

- It is **not signed** (a code-signing certificate costs money), so the first time you
  run it Windows may show a blue "Windows protected your PC" screen. Click
  **More info → Run anyway**. If you prefer to check first, the release page lists the
  SHA-256 number for the file; compare it with
  `certutil -hashfile younblock.exe SHA256` in a Command Prompt.
- It needs the WebView2 part of Windows, which Windows 11 (and any Windows 10 with
  Edge installed) already has.

## Building it yourself (optional)

**The easy way:** download the **`younblock-setup`** zip from the
[Releases page](https://github.com/assadigit/Younblock/releases), extract all
of its files into one folder, and double-click `setup-dev.bat`. It checks for
every tool listed below, installs the missing ones automatically — **through
mirrors when the usual download sites are unreachable** — and starts the app
straight from the code (`wails dev`); the app window opens by itself and stays
live while you work. For the everyday `.exe` files instead, double-click
`setup-build.bat`. The repository is private, so the scripts either walk you
through GitHub's browser login or take a token pasted into the `$GH_TOKEN`
line at the top of `setup.ps1`.

**No internet on the build PC at all?** The **`Younblock-offline`** zip on the
Releases page carries everything — the code, the Go dependencies, the window
building blocks, the Go toolchain, Node.js and the Wails CLI. Extract it and
double-click the `setup-offline.bat` inside; nothing is ever downloaded.

The rest of this section explains what happens under the hood, in case you
prefer doing it by hand.

You need three free tools installed on your Windows PC:

1. **Go** (the programming language the engine is written in)
   - Download the installer from <https://go.dev/dl/> (the "Windows" .msi file).
   - Run it, keep every default, and let it finish.
   - Check it: open a Command Prompt and type `go version`. You should see something
     like `go version go1.24.x windows/amd64` or newer.

2. **Node.js** (builds the app's window)
   - Download the "LTS" installer from <https://nodejs.org/>.
   - Run it, keep every default.
   - Check it: type `node --version` in the Command Prompt.

3. **Wails** (the tool that packs the window and the engine together)
   - In the Command Prompt, type: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
   - Wait for it to finish (a minute or two).
   - Check it: type `wails version`. You should see `v2.16.0` or newer.

Also make sure **Git** is installed (it usually already is): type `git --version`.
Get the code onto your PC with:

```
git clone https://github.com/assadigit/Younblock.git
cd Younblock
```

(The repository is private, so GitHub will ask you to log in.)

## How to build the app

In the Command Prompt, inside the `Younblock` folder, run:

```
build.bat
```

The first build takes a few minutes (it downloads the parts it needs once). When it
finishes it prints the two files it made:

- `dist\younblock.exe` — the app you use every day (window, tray icon, button).
- `dist\younblock-core.exe` — the same engine without a window, for testing and
  fixing (see below).

It also prints a long SHA-256 number for each file. If you ever send a copy to a
friend, they can check the number to be sure the file is exactly the one you built.

If you only want the headless engine, you can build just it with:

```
go build -trimpath -ldflags "-s -w" -o dist\younblock-core.exe .\cmd\core
```

## How to run it

Double-click `dist\younblock.exe` (or make a shortcut to it).

- The first time it opens, it welcomes you and asks one question: whether Younblock
  should open when Windows starts. You can change this later in Settings.
- Press the big button to turn it on. The button turns green, the timer starts, and
  YouTube should load in your browser — no other setup.
- Press it again to turn it off. Windows' proxy setting goes back to exactly what it
  was before.
- Closing the window **stops the app**: it shuts down completely, leaves the
  taskbar, and puts Windows' proxy setting back first (exactly like **Quit**).
  If you would rather keep it running as an icon near the clock (the tray),
  turn on **"Keep running in the tray when closed"** in Settings — then the
  tray icon brings the window back on a click, its right-click menu has Turn
  on / Turn off and **Quit**, and only Quit stops it fully.
- If YouTube stops responding while it is on, the app notices, tries to repair
  itself, and tells you what it did. If it cannot fix itself, the button area turns
  amber and shows a **Fix connection** button.

The technical settings live behind the collapsed **Advanced** section in Settings:
which sites go through the app (YouTube only, everything, or manual), the connection
method, the port, and the secure-DNS providers.

Notes on the two new switches:

- "Send all browser traffic" and the Advanced "Unblock mode" are the same setting:
  the switch is on when the mode is "All websites". Every browser that follows the
  Windows proxy setting (Chrome, Edge, …) then sends everything except local
  addresses through Younblock. Websites ending in .ir are the one exception: they
  are always sent DIRECT, never through the app, whatever the mode says.
- The separate Chrome window uses its own profile folder
  (`%LOCALAPPDATA%\Younblock\chrome-profile`), so your normal Chrome — its tabs,
  logins and history — is never touched. That window is hard-wired to Younblock's
  port, so when you turn Younblock off, the window closes with it (the app says so
  with a small note). If Chrome is not found, the app says that too and carries on.

## The Linux version (Linux Mint and friends)

The Linux editions are the **same engine and the same screens**. Since 1.6.4
the 64-bit `.deb` and `.AppImage` are **real desktop apps** — their own
window, their own taskbar icon, a normal menu entry — while the plain
one-file edition keeps the browser-page style (and carries both screens:
add `?ui=classic` to its address for the simple UI). Both editions exist
as **Younblock** (the redesigned screen) and **Younblock Classic** (the
simple pre-redesign UI).

### Which Linux download should I pick?

All of them are the same app underneath; the difference is only how it lands
on your machine:

- **The `.deb` — best for installing Younblock *properly* on Linux Mint.**
  Double-click it (or `sudo apt install ./younblock_…_amd64.deb` in a
  terminal) and Mint's installer puts the app in the menu with its icon, the
  binary in `/usr/bin`, and dpkg knows about it — so it updates cleanly and
  `apt remove younblock` uninstalls it. The `_amd64` one opens in its own
  window and pulls its display engine from Mint's own repositories (apt
  keeps it fresh); the `_i386` one (for a genuine 32-bit Mint) keeps the
  browser-page style. `younblock-classic_…_amd64.deb` is the same package
  with the Classic screen — both can be installed side by side.
- **The `.AppImage` — best if you want a single, portable executable file
  that you download and launch.** Make it runnable once
  (`chmod +x Younblock-…-x86_64.AppImage`) and double-click or run it; a
  normal app window opens — no browser, nothing installed; delete the file
  and it's gone. It carries its whole display engine inside (~75 MB), so it
  runs on any 64-bit Mint since 21 even with nothing else installed.
  `Younblock-Classic-…-x86_64.AppImage` is the same with the Classic screen.
  The `_i686` AppImage (32-bit) keeps the browser-page style.
- **The Flatpak — best for convenient installation with fewer dependency
  conflicts.** `flatpak install --user Younblock-….flatpak` (one-time setup
  below) and it runs inside Flatpak's own runtime, isolated from the rest of
  the system. The Flatpak edition keeps its settings inside its own sandbox
  (`~/.var/app/io.github.assadigit.Younblock`). One-time setup on Mint:
  ```
  sudo apt install flatpak
  flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo
  flatpak install --user Younblock-…-x86_64.flatpak
  flatpak run io.github.assadigit.Younblock
  ```
- **The plain file** (`younblock-linux-amd64` / `-386`) — the original
  one-file edition, unchanged. Download, `chmod +x`, run; the page it opens
  carries both screens (`?ui=classic` to switch).

Whichever you pick, start-with-OS (Settings) writes the right launcher for
that edition, and every edition updates the same way: download the new file,
install/replace it — your settings are kept.

### Running the plain file

1. Download `younblock-linux-amd64` into your Home folder.
2. Open a terminal and make it runnable once (downloaded files are not
   executable by default):
   ```
   chmod +x ~/younblock-linux-amd64
   ```
3. Run it:
   ```
   ~/younblock-linux-amd64
   ```
   It prints one line — `control it at http://127.0.0.1:17500` — and tries to
   open that page in your browser by itself. The terminal window is the app
   now: keep it open while you use Younblock, and press **Ctrl+C** (or choose
   Quit in the page's menu) to stop.

What is the same: the big button, the timer and response time, the self-test
and self-repair, Settings, Help, the diagnostic report — and the upgrade path:
replace the file, your settings are kept.

What is different, honestly:

- **System proxy.** On Mint with the Cinnamon, GNOME or MATE desktop the app
  sets the system proxy through the standard `gsettings` keys, the same
  place the desktop's own network settings write — every browser that follows
  the system proxy (Firefox, Chrome, Chromium, and the rest) picks it up. On
  other desktops (Xfce) there is no such switch, so the app tells you the
  address to enter by hand: set your browser's HTTP proxy to `127.0.0.1` and
  the port shown on the screen.
- **No tray.** Closing the browser tab does not stop the app (the terminal is
  still it). "Open Younblock when Linux starts" works everywhere through the
  standard autostart mechanism (`~/.config/autostart`).
- **The separate-Chrome switch is hidden** — that feature is Windows-only.
- What could only be tested on a real desktop by a human: that a browser
  picks up the proxy change the moment you press the button (see the manual
  check below). Everything else was tested end-to-end from the build machine,
  including a real YouTube round-trip through the Linux build.

## The 32-bit Windows version

`younblock-x86.exe` is the complete app built for 32-bit Windows, for an older
PC or a 32-bit virtual machine. It runs on 64-bit Windows too. It needs 32-bit
WebView2 (Windows 10/11 fetch it automatically the first time). Everything is
the same as the 64-bit app, including the tray and the self-repair.

## The built-in test and the headless engine

The Help page has a **Run test** button that checks four things: that the app is
running, that the secure lookup works, that YouTube answers, and that the video
servers answer. If something fails, it tells you which step and what to try.

`younblock-core.exe` is the same engine without the window. From a Command Prompt:

```
dist\younblock-core.exe --selftest          runs the four checks and prints them
dist\younblock-core.exe --restore-proxy     restores Windows' internet settings
dist\younblock-core.exe --version           prints the version
dist\younblock-core.exe --web               the browser-page edition (Linux style)
```

On Windows too, `--web` serves the same control page in your browser at
`http://127.0.0.1:17500` — handy over a remote session.

The `--selftest` output is exactly what to paste to someone helping you, together
with Help > "Copy diagnostic report".

## If it won't connect

The app now tells you **which part failed** right on the main screen:

- "On, but **secure DNS** isn't answering" — the secure lookup is being blocked
  on this network. Try a different secure-DNS provider: Settings > Advanced >
  Secure DNS servers (the change applies immediately, no restart needed), then
  press **Fix connection**.
- "On, but **YouTube** isn't responding" — the secure lookup works but the
  YouTube tunnel is being blocked. Press **Fix connection**; it tries every
  fragmentation method and keeps the one that works.

While it shows either line, the app keeps re-checking on its own and recovers
by itself as soon as the network allows it. To see the full picture, run the
Help page test and read the failing line, or copy the diagnostic report
(it includes "last failed check: ..."). If nothing helps, paste the diagnostic
report — it says exactly what is blocked on your network.

If you upgrade the app and it suddenly stopped connecting while the previous
version worked, also try the old exe once more: when both fail, your network's
blocking changed (the tunnel code between app versions was the same), and the
diagnostic report will show which part to switch.

## How to uninstall

1. Open Younblock, make sure it is turned **off**, and quit it — closing the
   window is enough now (or **Quit** from the tray menu if you turned on
   "Keep running in the tray when closed"); either path puts Windows' proxy
   setting back.
2. Delete the folder with `younblock.exe` in it.
3. Delete the settings folder: press Windows+R, type `%APPDATA%\Younblock`, press
   Enter, and delete that folder.

On Linux: the AppImage and the plain file — delete the file (and
`~/.config/Younblock` if you want the settings gone too); the `.deb` —
`sudo apt remove younblock`; the Flatpak —
`flatpak uninstall --user io.github.assadigit.Younblock` (its sandboxed
settings go with it).

If you ever lose internet because the app was killed while it was on, just run
`younblock-core.exe --restore-proxy` — or start the app once — and the setting is
cleaned up automatically.

## Your manual check on Windows (Ali's test list)

Do these once, on your own PC:

1. Run `younblock.exe`, press the button, wait for it to turn red (at most 35
   seconds), and open `https://www.youtube.com` in your browser. Videos should load.
2. **The every-browser check**: while it is on, open YouTube in another browser —
   Opera, Edge, Firefox, or another Chrome profile — **even one that was already
   running**. It should load there too now; no restart needed.
3. Press the button again to turn it off, then check Windows' proxy setting is
   back: Settings > Network & internet > Proxy should look like it did before
   ("Use a proxy server" off, or whatever you had before).
4. Turn it on, close the Younblock window, and confirm the app **stopped
   completely**: the tray icon is gone and Windows' proxy setting is back
   (Settings > Network & internet > Proxy) — closing is a real shutdown by
   default now.
5. Turn on **"Keep running in the tray when closed"** in Settings, turn the app
   on, close the window, and confirm the tray icon stays near the clock and
   YouTube still loads. Then use **Quit** from the tray menu and confirm the
   proxy setting is restored again. Turn the setting back off if you prefer
   the close-and-stop behavior.
6. Turn it on, then kill the app from Task Manager. Your internet must still work
   (the browser falls back to direct). Start the app again: it cleans the leftover
   setting by itself.
7. In Settings, turn on "Open Younblock when Windows starts", restart Windows, and
   confirm Younblock appears in the tray (started, but off).
8. While it is on, run Help > Run test and confirm all four steps pass.

If step 1 fails but the test passes, the filtering on your network may need a
different connection method: try Settings > Advanced > Connection method > Strong,
then run the test again.

## Your manual check on Linux Mint (Ali's test list)

Do these once, on the Mint PC (Cinnamon desktop). Steps 1–6 use the plain
file or the AppImage; steps 7–8 cover the packages:

1. Run `./younblock-linux-amd64`. The terminal prints the control address and
   the browser opens the page by itself. Press the button, wait for it to turn
   red, and open `https://www.youtube.com` — in a browser that was **already
   open before you pressed the button**. Videos should load. (This is the one
   thing a build machine cannot verify: that a running browser notices the
   proxy change immediately. If it doesn't, fully close and reopen the browser
   once and say so — the fallback matters.)
2. Press the button again (turn off), then check the system proxy is back:
   `gsettings get org.gnome.system.proxy mode` should print `'none'` (or
   whatever it printed before you started — check that first with the app off).
3. Turn it on, close the browser tab, and confirm the terminal still runs the
   app; open the control page again from the printed address — still on.
4. In Settings, turn on "Open Younblock when Linux starts", log out and back
   in, and confirm Younblock started by itself (a terminal autostarts, or check
   `pgrep -f younblock`).
5. While it is on, run Help > Run test and confirm all four steps pass.
6. Choose Quit from the page's menu: the terminal prints its shutdown lines and
   the proxy setting is restored (`gsettings get ... mode` shows the original).
7. **The .deb**: install it (`sudo apt install ./younblock_…_amd64.deb`),
   launch Younblock from the menu, and confirm the app works exactly as in
   steps 1–6 — including "Open Younblock when Linux starts" (the entry must
   name `/usr/bin/younblock --web`). Uninstall with
   `sudo apt remove younblock`.
8. **The Flatpak**: after the one-time setup above, install and run it
   (`flatpak install --user …; flatpak run io.github.assadigit.Younblock`).
   The build machine verified the bundle's contents end-to-end (install,
   files, live control page, start-with-OS entry naming `flatpak run …`), but
   the sandbox could not launch the Flatpak sandbox itself — so this launch
   on real Mint is the one manual check for this format. While it runs, the
   system-proxy switch and notifications must reach the real desktop (the
   bundle grants the session bus and host files for exactly that).

## Privacy

- No account, no analytics, no telemetry, no automatic updates.
- The only network requests are secure-DNS lookups and the YouTube checks the app
  runs itself.
- Website names are never written to the log unless you turn that on in Advanced.
- The diagnostic report contains no browsing history: version, settings summary,
  test results, counters and the last log lines (names removed).

## What this app does not do

- It is not a VPN and does not hide your IP address.
- It adds no encryption beyond what HTTPS already provides.
- It cannot promise YouTube loads on every network: filters change. The self-test
  and the automatic repair are the answer when they do — nothing in code can
  guarantee it.

## Credits and licenses

Built with Go, Wails (MIT), Svelte (MIT), the Inter font (SIL OFL 1.1), Lucide
icons (ISC), energye/systray (MIT) and go-toast (MIT). The full list is in the
app's About page.
