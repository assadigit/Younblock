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
GitHub and download `younblock.exe` from the latest release. That one file is the
whole app — no installer, nothing else to download.

Two things to know about the downloaded file:

- It is **not signed** (a code-signing certificate costs money), so the first time you
  run it Windows may show a blue "Windows protected your PC" screen. Click
  **More info → Run anyway**. If you prefer to check first, the release page lists the
  SHA-256 number for the file; compare it with
  `certutil -hashfile younblock.exe SHA256` in a Command Prompt.
- It needs the WebView2 part of Windows, which Windows 11 (and any Windows 10 with
  Edge installed) already has.

## Building it yourself (optional)

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
- Closing the window does not stop the app: it keeps running as an icon near the
  clock (the tray). Click that icon to bring the window back. Right-click it for
  Turn on / Turn off and **Quit**. Only Quit stops it fully — Quit always restores
  Windows' settings first.
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
  addresses through Younblock.
- The separate Chrome window uses its own profile folder
  (`%LOCALAPPDATA%\Younblock\chrome-profile`), so your normal Chrome — its tabs,
  logins and history — is never touched. That window is hard-wired to Younblock's
  port, so when you turn Younblock off, the window closes with it (the app says so
  with a small note). If Chrome is not found, the app says that too and carries on.

## The built-in test and the headless engine

The Help page has a **Run test** button that checks four things: that the app is
running, that the secure lookup works, that YouTube answers, and that the video
servers answer. If something fails, it tells you which step and what to try.

`younblock-core.exe` is the same engine without the window. From a Command Prompt:

```
dist\younblock-core.exe --selftest          runs the four checks and prints them
dist\younblock-core.exe --restore-proxy     restores Windows' internet settings
dist\younblock-core.exe --version           prints the version
```

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

1. Open Younblock, make sure it is turned **off**, and use **Quit** from the tray
   menu (this puts Windows' proxy setting back).
2. Delete the folder with `younblock.exe` in it.
3. Delete the settings folder: press Windows+R, type `%APPDATA%\Younblock`, press
   Enter, and delete that folder.

If you ever lose internet because the app was killed while it was on, just run
`younblock-core.exe --restore-proxy` — or start the app once — and the setting is
cleaned up automatically.

## Your manual check on Windows (Ali's test list)

Do these once, on your own PC, with your browser closed first:

1. Run `younblock.exe`, press the button, wait for it to turn green (at most 35
   seconds), and open `https://www.youtube.com` in your browser. Videos should load.
2. Press the button again to turn it off, then check Windows' proxy setting is back:
   Settings > Network & internet > Proxy should look like it did before (no script
   address pointing at 127.0.0.1).
3. Turn it on, close the Younblock window, and confirm the tray icon stays near the
   clock and YouTube still loads in your browser.
4. Use **Quit** from the tray menu and confirm the proxy setting is restored again.
5. Turn it on, then kill the app from Task Manager. Your internet must still work
   (the browser falls back to direct). Start the app again: it cleans the leftover
   setting by itself.
6. In Settings, turn on "Open Younblock when Windows starts", restart Windows, and
   confirm Younblock appears in the tray (started, but off).
7. While it is on, run Help > Run test and confirm all four steps pass.

If step 1 fails but the test passes, the filtering on your network may need a
different connection method: try Settings > Advanced > Connection method > Strong,
then run the test again.

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
