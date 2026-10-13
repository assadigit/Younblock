# Younblock

Younblock is a small Windows app that helps YouTube load in Google Chrome when your
network blocks it. It runs a little relay on your own computer: Chrome sends its
traffic to it, it looks up YouTube's address through a secure DNS service, and it sends
the first "hello" message to YouTube in small pieces so name-based filters don't
recognize it.

**It is not a VPN.** It does not hide your IP address and adds no encryption of its
own. YouTube's own HTTPS still protects everything you send. Your internet provider
can still see which servers you connect to. It only changes how the first message is
sent, and it may stop working if the filtering on your network changes.

It is made for Google Chrome and follows the normal Windows proxy setting, so Chrome
needs no extra setup. It works with one big button, tests itself, and repairs itself
when YouTube stops responding — and it always tells you what it changed.

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
  YouTube should load in Chrome — no other setup.
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

Do these once, on your own PC, with Chrome closed first:

1. Run `younblock.exe`, press the button, wait for it to turn green (at most 35
   seconds), and open `https://www.youtube.com` in Chrome. Videos should load.
2. Press the button again to turn it off, then check Windows' proxy setting is back:
   Settings > Network & internet > Proxy should look like it did before (no script
   address pointing at 127.0.0.1).
3. Turn it on, close the Younblock window, and confirm the tray icon stays near the
   clock and YouTube still loads in Chrome.
4. Use **Quit** from the tray menu and confirm the proxy setting is restored again.
5. Turn it on, then kill the app from Task Manager. Your internet must still work
   (Chrome falls back to direct). Start the app again: it cleans the leftover
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
